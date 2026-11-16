package ui

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func songKey(artist, title string) string {
	return strings.ToLower(strings.TrimSpace(artist)) + "\x1f" + strings.ToLower(strings.TrimSpace(title))
}

// ensureLyrics starts a lyrics lookup if the current song's lyrics aren't loaded.
func (m *Model) ensureLyrics() tea.Cmd {
	meta := m.state.Metadata
	key := songKey(meta.Artist, meta.Title)
	if key == m.lyrics.songID {
		return nil
	}
	m.lyrics = lyricsState{songID: key, renderedActive: -2}
	m.lyricsDirty = true
	m.viewport.SetContent("")
	m.viewport.GotoTop()
	if !m.hasState || meta.Title == "" {
		m.lyrics.err = errors.New("nothing is playing")
		return nil
	}
	m.lyrics.loading = true
	return FetchLyricsCmd(key, lyricsQuery{
		Artist:   meta.Artist,
		Title:    meta.Title,
		Album:    meta.Album,
		TrackURL: meta.URL,
		Duration: meta.Length,
	})
}

// activeLyric returns the index of the line being sung, or -1.
func (m *Model) activeLyric() int {
	if !m.lyrics.synced || len(m.lyrics.lines) == 0 {
		return -1
	}
	lookup := m.pos + lyricLatency + m.lyrics.offset
	lines := m.lyrics.lines
	return sort.Search(len(lines), func(i int) bool { return lines[i].Time > lookup }) - 1
}

// refreshLyrics re-renders the lyrics viewport when the active line changes and
// keeps the active line centred unless the user scrolled recently.
func (m *Model) refreshLyrics() {
	if len(m.lyrics.lines) == 0 {
		return
	}
	active := m.activeLyric()
	fill := m.karaokeFill(active)
	if active != m.lyrics.renderedActive || fill != m.lyrics.renderedFill || m.lyricsDirty {
		m.renderLyrics(active)
		m.lyrics.renderedActive = active
		m.lyrics.renderedFill = fill
		m.lyricsDirty = false
	}

	if active < 0 || time.Since(m.lyrics.lastManualScroll) < manualScrollHold {
		return
	}
	center := m.lyrics.rowStart[active] + m.lyrics.rowHeight[active]/2
	y := center - m.viewport.Height/2
	maxY := m.lyrics.totalRows - m.viewport.Height
	if y > maxY {
		y = maxY
	}
	if y < 0 {
		y = 0
	}
	m.viewport.SetYOffset(y)
}

func (m *Model) renderLyrics(active int) {
	s := m.styles
	width := m.viewport.Width
	if width <= 0 {
		_, width = m.layout()
	}
	n := len(m.lyrics.lines)
	m.lyrics.rowStart = make([]int, n)
	m.lyrics.rowHeight = make([]int, n)

	var sb strings.Builder
	rows := 0
	for i, line := range m.lyrics.lines {
		text := line.Text
		var st lipgloss.Style
		switch {
		case !m.lyrics.synced:
			st = s.PlainLyric
		case i == active:
			st = s.ActiveLyric
		case i < active:
			st = s.PastLyric
		default:
			st = s.UpcomingLyric
		}
		if text == "" && m.lyrics.synced {
			text = "♪"
		}
		var rendered string
		if i == active && line.Text != "" {
			// Karaoke: sweep the highlight across the line as it's sung.
			done, rest := splitAtCells(text, m.karaokeFill(active))
			rendered = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).
				Render(s.ActiveLyric.Render(done) + s.KaraokeRest.Render(rest))
		} else {
			rendered = st.Width(width).Align(lipgloss.Center).Render(text)
		}
		h := lipgloss.Height(rendered)
		m.lyrics.rowStart[i] = rows
		m.lyrics.rowHeight[i] = h
		rows += h
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(rendered)
	}
	m.lyrics.totalRows = rows
	m.viewport.SetContent(sb.String())
}

// viewLyrics renders the lyrics panel (header + viewport or status message).
func (m *Model) viewLyrics(inner int) string {
	s := m.styles
	meta := m.state.Metadata
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)

	title := meta.Title
	if title == "" {
		title = "Unknown track"
	}
	header := center.Render(s.LyricsTitle.Render(truncate("󰎆  "+title, inner)))

	var parts []string
	if meta.Artist != "" {
		parts = append(parts, meta.Artist)
	}
	if len(m.lyrics.lines) > 0 {
		if m.lyrics.synced {
			parts = append(parts, "synced")
		} else {
			parts = append(parts, "unsynced")
		}
		if m.lyrics.source != "" {
			parts = append(parts, m.lyrics.source)
		}
		if m.lyrics.offset != 0 {
			parts = append(parts, formatOffset(m.lyrics.offset))
		}
	}
	sub := center.Render(s.LyricsMeta.Render(truncate(strings.Join(parts, " · "), inner)))

	var body string
	switch {
	case m.lyrics.loading:
		body = s.LyricsLoading.Render("Searching for lyrics…")
	case errors.Is(m.lyrics.err, ErrInstrumental):
		body = s.LyricsLoading.Render("♪  Instrumental  ♪")
	case errors.Is(m.lyrics.err, ErrNoLyrics):
		body = s.LyricsMeta.Render("No lyrics found for this track.")
	case m.lyrics.err != nil:
		body = s.LyricsError.Render(m.lyrics.err.Error())
	case len(m.lyrics.lines) == 0:
		body = s.LyricsMeta.Render("No lyrics text available.")
	}

	if body != "" {
		// Keep the panel the same height as the viewport so the box doesn't jump.
		body = lipgloss.Place(inner, m.viewport.Height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).Render(body))
	} else {
		body = m.viewport.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, sub, "", body)
}

func formatOffset(d time.Duration) string {
	return fmt.Sprintf("%+.2fs", d.Seconds())
}

// karaokeFill returns how many cells of the active line have been sung.
func (m *Model) karaokeFill(active int) int {
	if active < 0 || active >= len(m.lyrics.lines) {
		return -1
	}
	lines := m.lyrics.lines
	start := lines[active].Time
	end := start + 4*time.Second
	if active+1 < len(lines) && lines[active+1].Time > start {
		end = lines[active+1].Time
	}
	// Most of the line is sung well before the next one starts.
	span := time.Duration(float64(end-start) * 0.85)
	if span <= 0 {
		return -1
	}
	pos := m.pos + lyricLatency + m.lyrics.offset - start
	frac := math.Max(0, math.Min(1, float64(pos)/float64(span)))
	return int(math.Round(frac * float64(ansi.StringWidth(lines[active].Text))))
}

// splitAtCells splits plain text after n display cells.
func splitAtCells(s string, n int) (string, string) {
	if n <= 0 {
		return "", s
	}
	w := 0
	for i, r := range s {
		rw := ansi.StringWidth(string(r))
		if w+rw > n {
			return s[:i], s[i:]
		}
		w += rw
	}
	return s, ""
}
