package ui

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const userAgent = "laya-tui (https://github.com/Sahas001/laya)"

var (
	// ErrNoLyrics means no lyrics could be found for the track.
	ErrNoLyrics = errors.New("no lyrics found for this track")
	// ErrInstrumental means the track is known to be instrumental.
	ErrInstrumental = errors.New("instrumental track")
)

// notFoundTTL is how long a "no lyrics" result is cached before retrying LRCLIB.
const notFoundTTL = 72 * time.Hour

var lyricsClient = &http.Client{Timeout: 15 * time.Second}

// lrclibBase can be overridden in tests.
var lrclibBase = "https://lrclib.net"

// LyricsMsg is sent when a lyrics lookup finishes.
type LyricsMsg struct {
	SongID string
	Lines  []LyricLine
	Synced bool
	Source string // "local file", "cache" or "LRCLIB"
	Err    error
}

// LyricLine represents a single line of lyrics with its timestamp (0 for plain lyrics).
type LyricLine struct {
	Time time.Duration
	Text string
}

// LyricResponse represents a track record returned by LRCLIB.
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

// lyricsQuery describes the track we are looking up.
type lyricsQuery struct {
	Artist   string
	Title    string
	Album    string
	TrackURL string
	Duration time.Duration
}

// cachedLyrics is the on-disk cache record.
type cachedLyrics struct {
	Artist       string    `json:"artist"`
	Title        string    `json:"title"`
	Synced       string    `json:"synced,omitempty"`
	Plain        string    `json:"plain,omitempty"`
	Instrumental bool      `json:"instrumental,omitempty"`
	NotFound     bool      `json:"not_found,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// FetchLyricsCmd returns a tea.Cmd that looks up lyrics in the background.
func FetchLyricsCmd(songID string, q lyricsQuery) tea.Cmd {
	return func() tea.Msg {
		lines, synced, source, err := lookupLyrics(q)
		return LyricsMsg{SongID: songID, Lines: lines, Synced: synced, Source: source, Err: err}
	}
}

// lookupLyrics tries, in order: a sidecar .lrc next to a local track, the
// on-disk cache, then LRCLIB. Results from LRCLIB (including misses) are cached.
func lookupLyrics(q lyricsQuery) ([]LyricLine, bool, string, error) {
	q.Artist = strings.TrimSpace(q.Artist)
	q.Title = strings.TrimSpace(q.Title)
	q.Album = strings.TrimSpace(q.Album)

	if text, err := readSidecarLrc(q.TrackURL); err == nil {
		if lines, synced := parseLyrics(text, ""); len(lines) > 0 {
			return lines, synced, "local file", nil
		}
	}

	if q.Title == "" {
		return nil, false, "", fmt.Errorf("the player did not report a track title")
	}

	cachePath := lyricsCachePath(q.Artist, q.Title)
	if c, ok := readCache(cachePath); ok {
		switch {
		case c.Instrumental:
			return nil, false, "cache", ErrInstrumental
		case c.NotFound && time.Since(c.FetchedAt) < notFoundTTL:
			return nil, false, "cache", ErrNoLyrics
		case !c.NotFound:
			if lines, synced := parseLyrics(c.Synced, c.Plain); len(lines) > 0 {
				return lines, synced, "cache", nil
			}
		}
	}

	rec, err := fetchFromLRCLIB(q)
	switch {
	case errors.Is(err, ErrNoLyrics):
		writeCache(cachePath, cachedLyrics{Artist: q.Artist, Title: q.Title, NotFound: true, FetchedAt: time.Now()})
		return nil, false, "LRCLIB", err
	case err != nil:
		return nil, false, "LRCLIB", err
	}

	writeCache(cachePath, cachedLyrics{
		Artist: q.Artist, Title: q.Title,
		Synced: rec.SyncedLyrics, Plain: rec.PlainLyrics,
		Instrumental: rec.Instrumental, FetchedAt: time.Now(),
	})
	if rec.Instrumental {
		return nil, false, "LRCLIB", ErrInstrumental
	}
	lines, synced := parseLyrics(rec.SyncedLyrics, rec.PlainLyrics)
	if len(lines) == 0 {
		return nil, false, "LRCLIB", ErrNoLyrics
	}
	return lines, synced, "LRCLIB", nil
}

// parseLyrics prefers synced LRC and falls back to plain text. If the "synced"
// text has no timestamps (e.g. a plain .lrc file) it is treated as plain.
func parseLyrics(synced, plain string) ([]LyricLine, bool) {
	if strings.TrimSpace(synced) != "" {
		if lines := ParseSyncedLrc(synced); len(lines) > 0 {
			return lines, true
		}
		if plain == "" {
			plain = synced
		}
	}
	return ParsePlainLrc(plain), false
}

// fetchFromLRCLIB queries the cached signature endpoint first, then falls back
// to a search (with cleaned-up title/artist for browser playback).
func fetchFromLRCLIB(q lyricsQuery) (*LyricResponse, error) {
	durationSec := int(math.Round(q.Duration.Seconds()))

	if q.Artist != "" && q.Album != "" && durationSec > 0 {
		params := url.Values{
			"track_name":  {q.Title},
			"artist_name": {q.Artist},
			"album_name":  {q.Album},
			"duration":    {strconv.Itoa(durationSec)},
		}
		var rec LyricResponse
		if status, err := getJSON(lrclibBase+"/api/get-cached?"+params.Encode(), &rec, 4*time.Second); err == nil && status == http.StatusOK {
			return &rec, nil
		}
	}

	var lastErr error = ErrNoLyrics
	for _, sq := range searchVariants(q.Artist, q.Title) {
		params := url.Values{"track_name": {sq.title}}
		if sq.artist != "" {
			params.Set("artist_name", sq.artist)
		}
		var results []LyricResponse
		status, err := getJSON(lrclibBase+"/api/search?"+params.Encode(), &results, 0)
		if err != nil {
			lastErr = fmt.Errorf("could not reach LRCLIB: %w", err)
			continue
		}
		if status != http.StatusOK {
			lastErr = fmt.Errorf("LRCLIB returned HTTP %d", status)
			continue
		}
		if best := pickBestResult(results, sq.title, q.Duration); best != nil {
			return best, nil
		}
	}
	return nil, lastErr
}

func getJSON(u string, v any, timeout time.Duration) (int, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	client := lyricsClient
	if timeout > 0 {
		c := *lyricsClient
		c.Timeout = timeout
		client = &c
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(v)
}

// pickBestResult ranks search results: synced lyrics, close duration and an
// exact title match are preferred. Instrumental records only win if nothing else exists.
func pickBestResult(results []LyricResponse, title string, duration time.Duration) *LyricResponse {
	if len(results) == 0 {
		return nil
	}
	score := func(r LyricResponse) float64 {
		s := 0.0
		if r.Instrumental {
			s -= 100
		}
		if r.SyncedLyrics != "" {
			s += 10
		} else if r.PlainLyrics != "" {
			s += 4
		}
		if strings.EqualFold(strings.TrimSpace(r.TrackName), strings.TrimSpace(title)) {
			s += 3
		}
		if duration > 0 && r.Duration > 0 {
			diff := math.Abs(r.Duration - duration.Seconds())
			switch {
			case diff <= 2:
				s += 8
			case diff <= 5:
				s += 4
			case diff > 20:
				s -= 6
			}
		}
		return s
	}
	best := 0
	for i := 1; i < len(results); i++ {
		if score(results[i]) > score(results[best]) {
			best = i
		}
	}
	r := results[best]
	if !r.Instrumental && r.SyncedLyrics == "" && r.PlainLyrics == "" {
		return nil
	}
	return &r
}

type searchQuery struct{ artist, title string }

var (
	noiseBrackets = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*\b(official|video|audio|lyrics?|visuali[sz]er|remaster(ed)?|hd|4k|mv|live|explicit|clean)\b[^\)\]]*[\)\]]`)
	featSuffix    = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.?|featuring)\s+.*$`)
	remasterTail  = regexp.MustCompile(`(?i)\s+-\s+(\d{4}\s+)?remaster(ed)?.*$`)
)

// cleanTitle strips decorations that browsers/YouTube add to track titles.
func cleanTitle(s string) string {
	s = noiseBrackets.ReplaceAllString(s, "")
	s = remasterTail.ReplaceAllString(s, "")
	s = featSuffix.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// cleanArtist strips YouTube "- Topic" / "VEVO" channel suffixes.
func cleanArtist(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " - Topic")
	s = strings.TrimSuffix(s, "VEVO")
	return strings.TrimSpace(s)
}

// searchVariants returns de-duplicated (artist, title) pairs to try in order.
func searchVariants(artist, title string) []searchQuery {
	var out []searchQuery
	seen := map[searchQuery]bool{}
	add := func(a, t string) {
		q := searchQuery{strings.TrimSpace(a), strings.TrimSpace(t)}
		if q.title == "" || seen[q] {
			return
		}
		seen[q] = true
		out = append(out, q)
	}
	add(artist, title)
	ca, ct := cleanArtist(artist), cleanTitle(title)
	add(ca, ct)
	// "Artist - Title" style titles (YouTube) when the artist is missing or a channel name.
	if parts := strings.SplitN(ct, " - ", 2); len(parts) == 2 {
		add(parts[0], cleanTitle(parts[1]))
	}
	return out
}

// --- local files & cache ---

// fileURLPath converts a file:// URL to a filesystem path.
func fileURLPath(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "file://") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return "", false
	}
	return u.Path, true
}

func hasHTTPScheme(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

// readSidecarLrc reads "<track>.lrc" next to a local audio file.
func readSidecarLrc(trackURL string) (string, error) {
	path, ok := fileURLPath(trackURL)
	if !ok {
		return "", fmt.Errorf("not a local file")
	}
	ext := filepath.Ext(path)
	if ext == "" {
		return "", fmt.Errorf("no extension found in path")
	}
	b, err := os.ReadFile(strings.TrimSuffix(path, ext) + ".lrc")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func lyricsCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "laya", "lyrics")
}

func lyricsCachePath(artist, title string) string {
	dir := lyricsCacheDir()
	if dir == "" {
		return ""
	}
	sum := sha1.Sum([]byte(strings.ToLower(artist) + "\x1f" + strings.ToLower(title)))
	return filepath.Join(dir, hex.EncodeToString(sum[:10])+".json")
}

func readCache(path string) (cachedLyrics, bool) {
	var c cachedLyrics
	if path == "" {
		return c, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c, false
	}
	if json.Unmarshal(b, &c) != nil {
		return c, false
	}
	return c, true
}

func writeCache(path string, c cachedLyrics) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// --- LRC parsing ---

var wordTimestamp = regexp.MustCompile(`<\d+:\d+(?:[.:]\d+)?>`)

// ParseSyncedLrc parses LRC synced lyrics into LyricLines sorted chronologically.
// It supports multiple timestamps per line, [mm:ss], [mm:ss.xx], [mm:ss:xx],
// the [offset:±ms] tag and strips enhanced-LRC word timestamps.
func ParseSyncedLrc(lrc string) []LyricLine {
	var lines []LyricLine
	var offset time.Duration

	for _, raw := range strings.Split(lrc, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		temp := raw
		var stamps []time.Duration
		for strings.HasPrefix(temp, "[") {
			end := strings.Index(temp, "]")
			if end == -1 {
				break
			}
			tag := temp[1:end]
			if d, err := parseTimestamp(tag); err == nil {
				stamps = append(stamps, d)
			} else if v, ok := strings.CutPrefix(strings.ToLower(tag), "offset:"); ok {
				if ms, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					// Positive offset means lyrics should appear sooner.
					offset = time.Duration(ms) * time.Millisecond
				}
			}
			temp = temp[end+1:]
		}
		text := strings.TrimSpace(wordTimestamp.ReplaceAllString(temp, ""))
		text = strings.Join(strings.Fields(text), " ")
		for _, ts := range stamps {
			lines = append(lines, LyricLine{Time: ts, Text: text})
		}
	}

	if offset != 0 {
		for i := range lines {
			lines[i].Time -= offset
			if lines[i].Time < 0 {
				lines[i].Time = 0
			}
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Time < lines[j].Time })
	return lines
}

// parseTimestamp parses mm:ss, mm:ss.xx(x) or mm:ss:xx.
func parseTimestamp(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}
	min, err := strconv.Atoi(parts[0])
	if err != nil || min < 0 {
		return 0, fmt.Errorf("invalid minutes in %q", s)
	}
	secStr := parts[1]
	if len(parts) == 3 {
		secStr += "." + parts[2]
	}
	sec, err := strconv.ParseFloat(secStr, 64)
	if err != nil || sec < 0 || sec >= 60 {
		return 0, fmt.Errorf("invalid seconds in %q", s)
	}
	ms := math.Round(sec * 1000)
	return time.Duration(min)*time.Minute + time.Duration(ms)*time.Millisecond, nil
}

// ParsePlainLrc converts plain lyrics text into LyricLines without timestamps.
// Single blank lines are kept as stanza breaks.
func ParsePlainLrc(plain string) []LyricLine {
	var lines []LyricLine
	blank := false
	for _, raw := range strings.Split(plain, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			blank = len(lines) > 0
			continue
		}
		if blank {
			lines = append(lines, LyricLine{})
			blank = false
		}
		lines = append(lines, LyricLine{Text: raw})
	}
	return lines
}
