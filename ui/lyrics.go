package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LyricsMsg is sent when lyrics fetch finishes
type LyricsMsg struct {
	SongID       string
	PlainLyrics  string
	SyncedLyrics string
	Err          error
}

// LyricLine represents a single line of synced lyrics with its timestamp
type LyricLine struct {
	Time time.Duration
	Text string
}

// LyricResponse represents the response format from LRCLIB
type LyricResponse struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// FetchLyricsCmd returns a tea.Cmd to fetch lyrics in the background
func FetchLyricsCmd(songID, artist, title, album, trackURL string, duration time.Duration) tea.Cmd {
	return func() tea.Msg {
		plain, synced, err := fetchLyrics(artist, title, album, trackURL, duration)
		return LyricsMsg{SongID: songID, PlainLyrics: plain, SyncedLyrics: synced, Err: err}
	}
}

func findLocalLrc(trackURL string) (string, error) {
	if !strings.HasPrefix(trackURL, "file://") {
		return "", fmt.Errorf("not a local file URL")
	}
	path := strings.TrimPrefix(trackURL, "file://")
	// Unescape path (e.g. %20 -> space)
	unescaped, err := url.QueryUnescape(path)
	if err == nil {
		path = unescaped
	}

	// Change extension to .lrc
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 {
		return "", fmt.Errorf("no extension found in path")
	}
	lrcPath := path[:lastDot] + ".lrc"

	if _, err := os.Stat(lrcPath); err != nil {
		return "", err
	}

	content, err := os.ReadFile(lrcPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func fetchLyrics(artist, title, album, trackURL string, duration time.Duration) (string, string, error) {
	artist = strings.TrimSpace(artist)
	title = strings.TrimSpace(title)
	album = strings.TrimSpace(album)

	// Stage 0: Try local .lrc file
	if trackURL != "" {
		localLrc, err := findLocalLrc(trackURL)
		if err == nil && localLrc != "" {
			return "", localLrc, nil
		}
	}

	if artist == "" || title == "" {
		return "", "", fmt.Errorf("artist and track title are required to find lyrics")
	}

	// Stage 1: Try /api/get-cached (Fast, database-only cached search)
	durationSec := int(duration.Seconds())
	if album != "" && durationSec > 0 {
		cachedURL := fmt.Sprintf("https://lrclib.net/api/get-cached?track_name=%s&artist_name=%s&album_name=%s&duration=%d",
			url.QueryEscape(title),
			url.QueryEscape(artist),
			url.QueryEscape(album),
			durationSec)
		
		req, err := http.NewRequest("GET", cachedURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "LayaTUI/1.0.0 (https://github.com/Sahas001/laya-tui)")
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				var result LyricResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					resp.Body.Close()
					if result.Instrumental {
						return "", "", fmt.Errorf("instrumental track")
					}
					return result.PlainLyrics, result.SyncedLyrics, nil
				}
				resp.Body.Close()
			}
		}
	}

	// Stage 2: Fallback to /api/search (Patient, full database search)
	queryURL := fmt.Sprintf("https://lrclib.net/api/search?track_name=%s&artist_name=%s",
		url.QueryEscape(title),
		url.QueryEscape(artist))

	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "LayaTUI/1.0.0 (https://github.com/Sahas001/laya-tui)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to reach lyrics database (timeout): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("lyrics database returned status %d", resp.StatusCode)
	}

	var results []LyricResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", "", fmt.Errorf("failed to parse lyrics: %w", err)
	}

	if len(results) == 0 {
		return "", "", fmt.Errorf("no lyrics found for this track")
	}

	best := results[0]
	if best.Instrumental {
		return "", "", fmt.Errorf("instrumental track")
	}
	return best.PlainLyrics, best.SyncedLyrics, nil
}

// ParseSyncedLrc parses LRC synced lyrics into a structured slice of LyricLines sorted chronologically
func ParseSyncedLrc(lrc string) []LyricLine {
	var lines []LyricLine
	rawLines := strings.Split(lrc, "\n")
	for _, rawLine := range rawLines {
		rawLine = strings.TrimSpace(rawLine)
		if rawLine == "" {
			continue
		}

		// A line can have multiple timestamps, e.g. [00:12.34][00:15.00]Text
		temp := rawLine
		var timestamps []time.Duration
		for {
			if !strings.HasPrefix(temp, "[") {
				break
			}
			endIdx := strings.Index(temp, "]")
			if endIdx == -1 {
				break
			}

			tsStr := temp[1:endIdx]
			d, err := parseTimestamp(tsStr)
			if err == nil {
				timestamps = append(timestamps, d)
			}
			temp = temp[endIdx+1:]
		}

		text := strings.TrimSpace(temp)
		for _, ts := range timestamps {
			lines = append(lines, LyricLine{
				Time: ts,
				Text: text,
			})
		}
	}

	// Sort lines by timestamp
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Time < lines[j].Time
	})

	return lines
}

func parseTimestamp(s string) (time.Duration, error) {
	// LRC timestamps can be MM:SS.xx or MM:SS
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid timestamp format")
	}

	minStr := parts[0]
	secStr := parts[1]

	var min int
	if _, err := fmt.Sscanf(minStr, "%d", &min); err != nil {
		return 0, err
	}

	var sec float64
	if _, err := fmt.Sscanf(secStr, "%f", &sec); err != nil {
		return 0, err
	}

	totalSec := float64(min)*60 + sec
	return time.Duration(totalSec * float64(time.Second)), nil
}

// ParsePlainLrc converts plain lyrics text into a structured slice of LyricLines without timestamps
func ParsePlainLrc(plain string) []LyricLine {
	var lines []LyricLine
	rawLines := strings.Split(plain, "\n")
	for _, rawLine := range rawLines {
		rawLine = strings.TrimSpace(rawLine)
		if rawLine != "" {
			lines = append(lines, LyricLine{
				Time: 0,
				Text: rawLine,
			})
		}
	}
	return lines
}
