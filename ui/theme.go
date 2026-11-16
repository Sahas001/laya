package ui

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"  // album art decoders
	_ "image/jpeg" // album art decoders
	_ "image/png"  // album art decoders
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/EdlinOrg/prominentcolor"
	tea "github.com/charmbracelet/bubbletea"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// maxArtBytes caps remote album-art downloads.
const maxArtBytes = 8 << 20

var artClient = &http.Client{Timeout: 4 * time.Second}

// ThemeUpdateMsg is sent when album-art colour extraction finishes.
// A nil Colors slice means "reset to the default palette".
type ThemeUpdateMsg struct {
	ArtURL string
	Colors []string    // hex colours, most prominent first
	Image  image.Image // decoded album art, nil on failure
}

// ExtractColorsCmd returns a tea.Cmd that runs colour extraction in the background.
func ExtractColorsCmd(artURL string) tea.Cmd {
	return func() tea.Msg {
		msg := ThemeUpdateMsg{ArtURL: artURL}
		img, err := loadArtImage(artURL)
		if err != nil {
			return msg
		}
		msg.Image = img
		centroids, err := prominentcolor.KmeansWithAll(4, img, prominentcolor.ArgumentDefault,
			prominentcolor.DefaultSize, prominentcolor.GetDefaultMasks())
		if err != nil || len(centroids) == 0 {
			// Fall back to no background masking (e.g. art that is mostly black/white).
			centroids, err = prominentcolor.KmeansWithAll(4, img, prominentcolor.ArgumentNoCropping,
				prominentcolor.DefaultSize, nil)
			if err != nil || len(centroids) == 0 {
				return msg
			}
		}
		for _, c := range centroids {
			msg.Colors = append(msg.Colors, fmt.Sprintf("#%02x%02x%02x", c.Color.R, c.Color.G, c.Color.B))
		}
		return msg
	}
}

func loadArtImage(artURL string) (image.Image, error) {
	var r io.Reader
	if path, ok := fileURLPath(artURL); ok {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	} else if strings.HasPrefix(artURL, "data:") {
		// data:image/png;base64,… (mpv-mpris, some browsers)
		comma := strings.IndexByte(artURL, ',')
		if comma < 0 || !strings.Contains(artURL[:comma], ";base64") {
			return nil, fmt.Errorf("unsupported data URL")
		}
		r = base64.NewDecoder(base64.StdEncoding, strings.NewReader(artURL[comma+1:]))
	} else if hasHTTPScheme(artURL) {
		req, err := http.NewRequest(http.MethodGet, artURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := artClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP error %d", resp.StatusCode)
		}
		r = io.LimitReader(resp.Body, maxArtBytes)
	} else {
		return nil, fmt.Errorf("unsupported art URL scheme: %s", artURL)
	}
	img, _, err := image.Decode(r)
	return img, err
}

// boostHexColor makes a colour vivid and bright enough to read on a dark background.
func boostHexColor(hex string) string {
	c, err := colorful.Hex(hex)
	if err != nil {
		return hex
	}
	h, s, l := c.Hsl()
	if s < 0.08 {
		// Near-greyscale art: keep it neutral but light.
		return colorful.Hsl(h, s, clampF(l, 0.72, 0.85)).Clamped().Hex()
	}
	s = clampF(s, 0.45, 0.9)
	l = clampF(l, 0.62, 0.8)
	return colorful.Hsl(h, s, l).Clamped().Hex()
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// paletteFromColors picks three accents from the extracted colours, preferring
// colourful ones and keeping primary and secondary visually distinct.
func paletteFromColors(colors []string) (Palette, bool) {
	type cc struct {
		hex string
		c   colorful.Color
		sat float64
	}
	var cs []cc
	for _, h := range colors {
		boosted := boostHexColor(h)
		c, err := colorful.Hex(boosted)
		if err != nil {
			continue
		}
		_, s, _ := c.Hsl()
		cs = append(cs, cc{boosted, c, s})
	}
	if len(cs) == 0 {
		return Palette{}, false
	}
	// Most saturated first, stable so prominence breaks ties.
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].sat > cs[j].sat+0.15 })

	primary := cs[0]
	secondary := primary
	for _, c := range cs[1:] {
		if c.c.DistanceLab(primary.c) > 0.2 {
			secondary = c
			break
		}
	}
	if secondary.hex == primary.hex {
		// Monochrome art: derive the secondary by rotating the hue.
		h, s, l := primary.c.Hsl()
		secondary.c = colorful.Hsl(h+40, s, l).Clamped()
		secondary.hex = secondary.c.Hex()
	}
	tertiary := secondary.hex
	for _, c := range cs {
		if c.hex != primary.hex && c.hex != secondary.hex {
			tertiary = c.hex
			break
		}
	}
	return Palette{Primary: primary.hex, Secondary: secondary.hex, Tertiary: tertiary}, true
}

// applyTheme rebuilds styles from extracted colours (or the default palette when nil).
func (m *Model) applyTheme(colors []string) {
	p, ok := paletteFromColors(colors)
	if !ok {
		p = DefaultPalette
	}
	if p == m.styles.Palette {
		return
	}
	m.styles = NewStyles(p)
	m.progressBar = newProgressBar(p)
	m.lyricsDirty = true
}
