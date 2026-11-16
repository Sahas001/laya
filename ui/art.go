package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/nfnt/resize"
)

// ArtMode selects how album art is drawn.
type ArtMode int

const (
	ArtAuto   ArtMode = iota // kitty images when supported, else blocks
	ArtKitty                 // force kitty graphics
	ArtBlocks                // true-colour half blocks
	ArtOff                   // never draw art
)

// ParseArtMode parses the --art flag value.
func ParseArtMode(s string) (ArtMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ArtAuto, nil
	case "kitty":
		return ArtKitty, nil
	case "blocks", "block":
		return ArtBlocks, nil
	case "off", "none", "false":
		return ArtOff, nil
	}
	return ArtAuto, fmt.Errorf("unknown art mode %q (want auto, kitty, blocks or off)", s)
}

// kittyDiacritics are the first row/column index marks from kitty's
// rowcolumn-diacritics.txt, used by Unicode image placeholders.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
}

const (
	kittyPlaceholder = '\U0010EEEE'
	maxArtRows       = 12
)

// artState holds the decoded album art and render caches.
type artState struct {
	url   string
	img   image.Image
	cache map[[2]int]string // block renders keyed by cols×rows

	kittyID   uint32
	kittyKey  string // url + size of the image currently transmitted
	kittySeq  uint32
	kittyBase uint32
}

func (a *artState) set(url string, img image.Image) {
	a.url, a.img = url, img
	a.cache = nil
}

func (a *artState) clear() { a.set("", nil) }

// useKitty decides between kitty images and block art.
func (m *Model) useKitty() bool {
	switch m.artMode {
	case ArtKitty:
		return m.term != nil
	case ArtAuto:
		return m.term != nil && m.term.Kitty
	}
	return false
}

// artAvailable reports whether there is art to show.
func (m *Model) artAvailable() bool {
	return m.showArt && m.artMode != ArtOff && m.art.img != nil
}

// viewArt renders album art in cols×rows cells, or "" if unavailable.
func (m *Model) viewArt(cols, rows int) string {
	if !m.artAvailable() || cols <= 0 || rows <= 0 {
		return ""
	}
	if m.useKitty() && rows <= len(kittyDiacritics) {
		return m.kittyArt(cols, rows)
	}
	return m.blockArt(cols, rows)
}

// blockArt draws the image with "▀" cells: foreground = top pixel, background = bottom pixel.
func (m *Model) blockArt(cols, rows int) string {
	key := [2]int{cols, rows}
	if s, ok := m.art.cache[key]; ok {
		return s
	}
	s := renderBlockArt(m.art.img, cols, rows, lipgloss.ColorProfile())
	if m.art.cache == nil {
		m.art.cache = map[[2]int]string{}
	}
	m.art.cache[key] = s
	return s
}

func renderBlockArt(img image.Image, cols, rows int, profile termenv.Profile) string {
	if profile == termenv.Ascii {
		return ""
	}
	small := resize.Resize(uint(cols), uint(rows*2), img, resize.Lanczos3)
	b := small.Bounds()
	hex := func(x, y int) string {
		r, g, bl, _ := small.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, bl>>8)
	}
	var sb strings.Builder
	for y := 0; y < rows; y++ {
		if y > 0 {
			sb.WriteByte('\n')
		}
		prevFg, prevBg := "", ""
		for x := 0; x < cols; x++ {
			fg, bg := hex(x, 2*y), hex(x, 2*y+1)
			if fg != prevFg {
				sb.WriteString("\x1b[" + profile.Color(fg).Sequence(false) + "m")
				prevFg = fg
			}
			if bg != prevBg {
				sb.WriteString("\x1b[" + profile.Color(bg).Sequence(true) + "m")
				prevBg = bg
			}
			sb.WriteString("▀")
		}
		sb.WriteString("\x1b[0m")
	}
	return sb.String()
}

// kittyArt transmits the image once per art/size and returns placeholder cells
// that kitty replaces with the picture.
func (m *Model) kittyArt(cols, rows int) string {
	key := fmt.Sprintf("%s|%dx%d", m.art.url, cols, rows)
	if key != m.art.kittyKey {
		if m.art.kittyBase == 0 {
			m.art.kittyBase = uint32(os.Getpid()&0xFFFF) << 8
		}
		m.art.kittySeq++
		id := m.art.kittyBase | (m.art.kittySeq & 0xFF)
		if id == 0 {
			id = 1
		}
		payload, err := encodeKittyImage(m.art.img, cols, rows)
		if err != nil {
			return m.blockArt(cols, rows)
		}
		m.deleteKittyImage()
		m.term.WriteRaw(kittyTransmit(id, cols, rows, payload))
		m.art.kittyID, m.art.kittyKey = id, key
	}
	return kittyPlaceholders(m.art.kittyID, cols, rows)
}

func (m *Model) deleteKittyImage() {
	if m.art.kittyID != 0 && m.term != nil {
		m.term.WriteRaw(fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", m.art.kittyID))
		m.art.kittyID, m.art.kittyKey = 0, ""
	}
}

// Close releases terminal resources (kitty images). Call after the program exits.
func (m *Model) Close() {
	m.deleteKittyImage()
	if m.viz.capture != nil {
		m.viz.capture.Stop()
	}
}

func encodeKittyImage(img image.Image, cols, rows int) (string, error) {
	// ~12×24 px per cell keeps it crisp without sending huge payloads.
	w, h := uint(cols*12), uint(rows*24)
	if w > 600 {
		w, h = 600, 600*h/w
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, resize.Resize(w, h, img, resize.Lanczos3)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// kittyTransmit builds the chunked "transmit and create virtual placement" command.
func kittyTransmit(id uint32, cols, rows int, payload string) string {
	const chunk = 4096
	var sb strings.Builder
	for i := 0; i < len(payload); i += chunk {
		end := i + chunk
		if end > len(payload) {
			end = len(payload)
		}
		more := 1
		if end == len(payload) {
			more = 0
		}
		if i == 0 {
			fmt.Fprintf(&sb, "\x1b_Ga=T,U=1,f=100,t=d,i=%d,c=%d,r=%d,q=2,m=%d;", id, cols, rows, more)
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;", more)
		}
		sb.WriteString(payload[i:end])
		sb.WriteString("\x1b\\")
	}
	return sb.String()
}

// kittyPlaceholders returns rows of U+10EEEE cells whose foreground colour
// encodes the image id; the first cell of each row carries row/column marks.
func kittyPlaceholders(id uint32, cols, rows int) string {
	color := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", (id>>16)&0xFF, (id>>8)&0xFF, id&0xFF)
	var sb strings.Builder
	for y := 0; y < rows; y++ {
		if y > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(color)
		sb.WriteRune(kittyPlaceholder)
		sb.WriteRune(kittyDiacritics[y])
		sb.WriteRune(kittyDiacritics[0])
		sb.WriteString(strings.Repeat(string(kittyPlaceholder), cols-1))
		sb.WriteString("\x1b[39m")
	}
	return sb.String()
}
