package ui

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EdlinOrg/prominentcolor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/progress"
)

// ThemeUpdateMsg is sent when dominant colors are successfully extracted
type ThemeUpdateMsg struct {
	Colors []string // Hex colors
}

// ExtractColorsCmd returns a tea.Cmd that runs color extraction in the background
func ExtractColorsCmd(artURL string) tea.Cmd {
	return func() tea.Msg {
		if artURL == "" {
			return nil
		}
		img, err := loadArtImage(artURL)
		if err != nil {
			return nil // Keep current theme on error
		}
		
		// Extract colors using K-means
		centroids, err := prominentcolor.Kmeans(img)
		if err != nil || len(centroids) < 2 {
			return nil
		}

		colors := make([]string, 0, len(centroids))
		for _, c := range centroids {
			colors = append(colors, fmt.Sprintf("#%02x%02x%02x", c.Color.R, c.Color.G, c.Color.B))
		}
		return ThemeUpdateMsg{Colors: colors}
	}
}

func loadArtImage(artURL string) (image.Image, error) {
	if strings.HasPrefix(artURL, "file://") {
		path := strings.TrimPrefix(artURL, "file://")
		// Unescape url path (e.g. %20 -> space)
		unescaped, err := url.QueryUnescape(path)
		if err == nil {
			path = unescaped
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		return img, err
	} else if strings.HasPrefix(artURL, "http://") || strings.HasPrefix(artURL, "https://") {
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(artURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP error %d", resp.StatusCode)
		}
		img, _, err := image.Decode(resp.Body)
		return img, err
	}
	return nil, fmt.Errorf("unsupported art URL scheme: %s", artURL)
}

// boostHexColor takes a hex color string and ensures it is highly visible on dark/transparent backgrounds.
func boostHexColor(hexStr string) string {
	if !strings.HasPrefix(hexStr, "#") || len(hexStr) != 7 {
		return hexStr
	}
	r64, err1 := strconv.ParseUint(hexStr[1:3], 16, 8)
	g64, err2 := strconv.ParseUint(hexStr[3:5], 16, 8)
	b64, err3 := strconv.ParseUint(hexStr[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return hexStr
	}

	r, g, b := uint8(r64), uint8(g64), uint8(b64)

	// Find max channel
	maxChan := r
	if g > maxChan {
		maxChan = g
	}
	if b > maxChan {
		maxChan = b
	}

	if maxChan == 0 {
		return "#B4BEFE" // default light lavender
	}

	// Boost max channel to at least 200 (extra prominent!) for high contrast
	if maxChan < 200 {
		scale := 200.0 / float64(maxChan)
		r = uint8(float64(r) * scale)
		g = uint8(float64(g) * scale)
		b = uint8(float64(b) * scale)
	}

	// Check luminance
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if lum < 140 {
		// Mix with white by 30% to guarantee readability on dark/transparent backgrounds
		r = uint8(float64(r) + (255-float64(r))*0.3)
		g = uint8(float64(g) + (255-float64(g))*0.3)
		b = uint8(float64(b) + (255-float64(b))*0.3)
	}

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// UpdateThemeStyles dynamically applies a color palette to Laya's Lipgloss styles and progress bar
func (m *Model) UpdateThemeStyles(colors []string) {
	if len(colors) < 2 {
		return
	}

	primary := boostHexColor(colors[0])
	secondary := boostHexColor(colors[1])
	
	// Tertiary fallback
	tertiary := secondary
	if len(colors) > 2 {
		tertiary = boostHexColor(colors[2])
	}

	// Update Lipgloss styles dynamically
	m.styles.Header = m.styles.Header.Background(lipgloss.Color(primary)).Foreground(lipgloss.Color(ColorBg))
	m.styles.PlayerName = m.styles.PlayerName.Foreground(lipgloss.Color(secondary))
	m.styles.SongTitle = m.styles.SongTitle.Foreground(lipgloss.Color(primary))
	m.styles.TrackCard = m.styles.TrackCard.
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(secondary))
	m.styles.Artist = m.styles.Artist.Foreground(lipgloss.Color(secondary))
	m.styles.VolumeLabel = m.styles.VolumeLabel.Foreground(lipgloss.Color(secondary))
	
	m.styles.HelpText = m.styles.HelpText.Foreground(lipgloss.Color(secondary))
	m.styles.KeyStyle = m.styles.KeyStyle.Foreground(lipgloss.Color(primary))
	m.styles.SelectTitle = m.styles.SelectTitle.Foreground(lipgloss.Color(primary))
	m.styles.SelectActive = m.styles.SelectActive.Foreground(lipgloss.Color(primary))
	m.styles.LyricsTitle = m.styles.LyricsTitle.Foreground(lipgloss.Color(primary))
	m.styles.ActiveLyric = m.styles.ActiveLyric.Foreground(lipgloss.Color(primary))

	// Recreate progress bar with new color gradient
	m.progressBar = progress.New(
		progress.WithoutPercentage(),
		progress.WithScaledGradient(primary, tertiary),
	)
}
