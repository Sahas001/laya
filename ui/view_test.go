package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sahas001/laya/mpris"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func fakeModel(w, h int) *Model {
	m := NewModel(nil)
	m.view = ViewDashboard
	m.playerName = "Spotify"
	m.hasState = true
	m.state = mpris.PlayerState{
		PlaybackStatus: mpris.StatusPlaying,
		Volume:         0.65,
		HasVolume:      true,
		HasShuffle:     true,
		Shuffle:        true,
		LoopStatus:     mpris.LoopPlaylist,
		Rate:           1,
		CanSeek:        true,
		Position:       83 * time.Second,
		LastUpdated:    time.Now(),
		Metadata: mpris.TrackMetadata{
			Title:  "A Very Long Song Title That Keeps Going (Remastered Deluxe Edition)",
			Artist: "Some Artist feat. Another Artist",
			Album:  "Greatest Hits Vol. 2",
			Length: 225 * time.Second,
		},
	}
	m.pos = m.state.Position
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	now := time.Now()
	for i := 0; i < 10; i++ {
		now = now.Add(40 * time.Millisecond)
		m.updateViz(now)
	}
	return m
}

func checkFits(t *testing.T, name, out string, w, h int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("%s: line %d is %d wide (> %d): %q", name, i, lw, w, line)
		}
	}
	if lh := lipgloss.Height(out); lh > h {
		t.Errorf("%s: %d lines tall (> %d)", name, lh, h)
	}
	if os.Getenv("LAYA_DUMP") != "" {
		t.Logf("%s (%dx%d):\n%s", name, w, h, out)
	}
}

func TestViewsFitTerminal(t *testing.T) {
	sizes := [][2]int{{120, 40}, {80, 24}, {50, 20}, {36, 16}}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]

		m := fakeModel(w, h)
		checkFits(t, "dashboard", m.View(), w, h)

		m.showHelp = true
		checkFits(t, "help", m.View(), w, h)
		m.showHelp = false

		m.showLyrics = true
		m.lyrics = lyricsState{songID: "x", synced: true, source: "LRCLIB", renderedActive: -2,
			lines: ParseSyncedLrc("[00:10.00]First line of the song\n[01:20.00]This is the line that is currently being sung and it is quite long indeed\n[01:30.00]\n[01:40.00]Next line")}
		m.resize()
		checkFits(t, "lyrics", m.View(), w, h)
		m.showLyrics = false

		m.view = ViewSelectPlayer
		m.playersLoaded = true
		m.players = []PlayerInfo{
			{BusName: "org.mpris.MediaPlayer2.spotify", Identity: "Spotify", Label: "Spotify", Status: mpris.StatusPlaying, Title: "Song", Artist: "Artist"},
			{BusName: "org.mpris.MediaPlayer2.firefox.instance_1_44", Identity: "Mozilla Firefox", Label: "Mozilla Firefox (firefox.instance_1_44)", Status: mpris.StatusPaused},
		}
		checkFits(t, "select", m.View(), w, h)
	}
}

func TestProgressHitBoxMatchesRenderedBar(t *testing.T) {
	for _, sz := range [][2]int{{90, 26}, {60, 18}} {
		m := fakeModel(sz[0], sz[1])
		out := m.View()
		hb := m.progressHit
		if !hb.ok {
			t.Fatalf("%v: no hitbox", sz)
		}
		lines := strings.Split(ansi.Strip(out), "\n")
		row := []rune(lines[m.boxY+hb.y])
		for x := m.boxX + hb.x0; x < m.boxX+hb.x1; x++ {
			if row[x] != '━' {
				t.Fatalf("%v: column %d is %q, not part of the bar: %q", sz, x, row[x], string(row))
			}
		}
		if row[m.boxX+hb.x0-1] == '━' || row[m.boxX+hb.x1] == '━' {
			t.Errorf("%v: hitbox narrower than the bar", sz)
		}
	}
}
