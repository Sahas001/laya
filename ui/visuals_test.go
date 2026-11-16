package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/Sahas001/laya/mpris"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestVizRenderersFillExactArea(t *testing.T) {
	m := fakeModel(100, 40)
	for _, size := range [][2]int{{58, 5}, {33, 3}, {150, 30}, {10, 1}} {
		w, h := size[0], size[1]
		for st := VizStyle(0); st < numVizStyles; st++ {
			m.viz.style = st
			m.viz.spec.Resize(vizBandCount(st, w))
			for i := range m.viz.spec.Values {
				m.viz.spec.Values[i] = float64(i%7) / 6
				m.viz.spec.Peaks[i] = 1
			}
			out := m.renderViz(w, h)
			lines := strings.Split(out, "\n")
			if len(lines) != h {
				t.Fatalf("%s %dx%d: %d rows", st, w, h, len(lines))
			}
			for i, l := range lines {
				if lw := lipgloss.Width(l); lw != w {
					t.Fatalf("%s %dx%d: row %d is %d wide", st, w, h, i, lw)
				}
			}
		}
	}
}

func TestBarsShowLevelsAndPeaks(t *testing.T) {
	rows := renderBars([]float64{1, 0.5, 0}, []float64{1, 1, 0}, 5, 2)
	want := []string{" █▔  ", " ██  "} // <24 cols: no gaps, centred
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, rows[i], want[i])
		}
	}
}

func TestScopeDrawsAContinuousLine(t *testing.T) {
	wave := make([]float64, 1024)
	for i := range wave {
		wave[i] = float64(i%200)/100 - 1 // sawtooth
	}
	rows := renderScope(wave, 20, 4)
	for c := 0; c < 20; c++ {
		lit := false
		for _, r := range rows {
			if []rune(r)[c] != ' ' {
				lit = true
			}
		}
		if !lit {
			t.Fatalf("column %d has no trace: %q", c, rows)
		}
	}
}

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 200, 255})
		}
	}
	return img
}

func TestBlockArtSize(t *testing.T) {
	out := renderBlockArt(testImage(), 16, 8, termenv.TrueColor)
	lines := strings.Split(out, "\n")
	if len(lines) != 8 {
		t.Fatalf("%d rows", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 16 {
			t.Fatalf("row %d is %d wide", i, w)
		}
	}
	if renderBlockArt(testImage(), 4, 2, termenv.Ascii) != "" {
		t.Error("no art expected without colour support")
	}
}

func TestKittyPlaceholders(t *testing.T) {
	out := kittyPlaceholders(0x010203, 6, 3)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("%d rows", len(lines))
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, "\x1b[38;2;1;2;3m") {
			t.Errorf("row %d missing id colour: %q", i, l)
		}
		if w := ansi.StringWidth(l); w != 6 {
			t.Errorf("row %d is %d wide", i, w)
		}
		if !strings.Contains(l, string([]rune{kittyPlaceholder, kittyDiacritics[i], kittyDiacritics[0]})) {
			t.Errorf("row %d lacks row/column marks", i)
		}
	}
	tx := kittyTransmit(7, 6, 3, strings.Repeat("A", 9000))
	if strings.Count(tx, "\x1b_G") != 3 || !strings.Contains(tx, "a=T,U=1,f=100") || !strings.Contains(tx, "m=0;") {
		t.Errorf("unexpected transmit framing: %q…", tx[:80])
	}
}

func TestParseArtMode(t *testing.T) {
	for in, want := range map[string]ArtMode{"": ArtAuto, "kitty": ArtKitty, "Blocks": ArtBlocks, "off": ArtOff} {
		if got, err := ParseArtMode(in); err != nil || got != want {
			t.Errorf("%q → %v, %v", in, got, err)
		}
	}
	if _, err := ParseArtMode("sixel"); err == nil {
		t.Error("expected error")
	}
}

func TestKaraokeFill(t *testing.T) {
	m := fakeModel(80, 24)
	m.lyrics.synced = true
	m.lyrics.lines = []LyricLine{{Time: 10 * time.Second, Text: "abcdefghij"}, {Time: 20 * time.Second, Text: "next"}}
	m.pos = 10*time.Second - lyricLatency
	if f := m.karaokeFill(0); f != 0 {
		t.Errorf("start fill = %d", f)
	}
	m.pos = 10*time.Second - lyricLatency + 4250*time.Millisecond // half of the 8.5s sweep
	if f := m.karaokeFill(0); f != 5 {
		t.Errorf("mid fill = %d", f)
	}
	if a, b := splitAtCells("héllo wörld", 3); a != "hél" || b != "lo wörld" {
		t.Errorf("split = %q %q", a, b)
	}
}

func TestSleepTimerCycles(t *testing.T) {
	m := fakeModel(80, 24)
	var got []int
	for i := 0; i < 6; i++ {
		m.cycleSleepTimer()
		if m.sleepAt.IsZero() {
			got = append(got, 0)
		} else {
			got = append(got, int(time.Until(m.sleepAt).Round(time.Minute).Minutes()))
		}
	}
	want := []int{15, 30, 45, 60, 90, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestHistoryRecordsDistinctTracks(t *testing.T) {
	m := fakeModel(80, 24)
	st := m.state
	m.trackChanged(st)
	m.trackChanged(st)
	st.Metadata.Title = "Second"
	m.trackChanged(st)
	if len(m.history) != 2 || m.history[0].Title != "Second" {
		t.Fatalf("history = %+v", m.history)
	}
}

func TestFullscreenAndPanelsFit(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)
	for _, sz := range [][2]int{{120, 40}, {80, 24}, {40, 14}} {
		w, h := sz[0], sz[1]
		m := fakeModel(w, h)
		m.art.set("file:///x.png", testImage())
		m.artMode = ArtBlocks
		checkFits(t, "dashboard+art", m.View(), w, h)

		m.fullscreen = true
		for i := 0; i < 8; i++ {
			m.updateViz(time.Now().Add(time.Duration(i) * 40 * time.Millisecond))
		}
		checkFits(t, "fullscreen", m.View(), w, h)
		m.fullscreen = false

		m.showHistory = true
		m.history = []HistoryEntry{{Title: "A long title for history", Artist: "Artist", Player: "mpv", At: time.Now()}}
		checkFits(t, "history", m.View(), w, h)
		m.showHistory = false
	}
	_ = mpris.StatusPlaying
}
