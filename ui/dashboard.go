package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/Sahas001/laya/internal/format"
	"github.com/Sahas001/laya/mpris"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// truncate shortens s to fit within width terminal cells.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// frame accumulates rendered sections and tracks the current row so that
// clickable regions can be located later.
type frame struct {
	parts []string
	rows  int
}

func (f *frame) add(s string) {
	f.parts = append(f.parts, s)
	f.rows += lipgloss.Height(s)
}

func (f *frame) blank() { f.add("") }

func (f *frame) String() string { return strings.Join(f.parts, "\n") }

// boxOffsetX is the box chrome before content: rounded border (1) + left padding (2).
const boxOffsetX = 3

// compact reports whether the terminal is too short for the roomy layout.
func (m *Model) compact() bool { return m.height > 0 && m.height < 22 }

// container returns the outer box style, without vertical padding in compact mode.
func (m *Model) container(box int) lipgloss.Style {
	st := m.styles.Container.Width(box)
	if m.compact() {
		st = st.Padding(0, 2)
	}
	return st
}

// boxOffsetY is the box chrome above content: border (1) + top padding.
func (m *Model) boxOffsetY() int {
	if m.compact() {
		return 1
	}
	return 2
}

func (m *Model) viewHeader(f *frame, inner int) {
	s := m.styles
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)
	f.add(center.Render(s.Header.Render("L A Y A")))

	if m.playerName == "" {
		return
	}
	dot := s.DotStopped.Render("●")
	switch m.state.PlaybackStatus {
	case mpris.StatusPlaying:
		dot = s.DotPlaying.Render("●")
	case mpris.StatusPaused:
		dot = s.DotPaused.Render("●")
	}

	var ind []string
	if m.hasState {
		if m.state.HasShuffle {
			st := s.IndicatorOff
			if m.state.Shuffle {
				st = s.IndicatorOn
			}
			ind = append(ind, st.Render("󰒟"))
		}
		switch m.state.LoopStatus {
		case mpris.LoopPlaylist:
			ind = append(ind, s.IndicatorOn.Render("󰑖"))
		case mpris.LoopTrack:
			ind = append(ind, s.IndicatorOn.Render("󰑘"))
		case mpris.LoopNone:
			ind = append(ind, s.IndicatorOff.Render("󰑗"))
		}
		if r := m.state.Rate; r > 0 && math.Abs(r-1) > 0.01 {
			ind = append(ind, s.IndicatorOn.Render(fmt.Sprintf("%.2g×", r)))
		}
	}
	if l := m.sleepLabel(); l != "" {
		ind = append(ind, s.IndicatorOn.Render(l))
	}
	indicators := ""
	if len(ind) > 0 {
		indicators = "   " + strings.Join(ind, " ")
	}
	name := truncate(m.playerName, inner-4-lipgloss.Width(indicators))
	f.add(center.Render(dot + " " + s.PlayerName.Render(name) + indicators))
}

func (m *Model) viewFooter(f *frame, inner int) {
	s := m.styles
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)

	// Toast line is always reserved so the layout doesn't jump.
	toast := ""
	if m.toast != "" {
		st := s.Toast
		if m.toastErr {
			st = s.ToastErr
		}
		toast = st.Render(truncate(m.toast, inner))
	}
	f.add(center.Render(toast))

	var hints []hint
	switch {
	case m.showHelp || m.showHistory:
		hints = []hint{{"esc", "close", 0}, {"q", "quit", 1}}
		if m.panelOverflows() {
			hints = append([]hint{{"j/k", "scroll", 0}}, hints...)
		}
	case m.showLyrics:
		hints = []hint{{"␣", "play/pause", 1}, {"j/k", "scroll", 2}, {"</>", "timing", 4},
			{"l", "back", 0}, {"?", "help", 0}, {"q", "quit", 3}}
	default:
		hints = []hint{{"␣", "play/pause", 1}, {"l", "lyrics", 2}, {"V", "visualizer", 3},
			{"s", "players", 4}, {"?", "help", 0}, {"q", "quit", 2}}
	}
	f.add(center.Render(m.renderHints(hints, inner)))
}

// hint is a footer key hint; lower prio values are kept longest when space is short.
type hint struct {
	key, desc string
	prio      int
}

// renderHints joins hints with separators, dropping the least important ones
// until the line fits within width (display order is preserved).
func (m *Model) renderHints(hints []hint, width int) string {
	s := m.styles
	sep := s.HelpText.Render(" · ")
	render := func(hs []hint) string {
		parts := make([]string, len(hs))
		for i, h := range hs {
			parts[i] = s.HelpKey.Render(h.key) + " " + s.HelpText.Render(h.desc)
		}
		return strings.Join(parts, sep)
	}
	hs := append([]hint(nil), hints...)
	for len(hs) > 1 && lipgloss.Width(render(hs)) > width {
		worst := 0
		for i, h := range hs {
			if h.prio >= hs[worst].prio {
				worst = i
			}
		}
		hs = append(hs[:worst], hs[worst+1:]...)
	}
	return render(hs)
}

func (m *Model) viewDashboard() string {
	box, inner := m.layout()
	m.progressHit = hitBox{}

	var f frame
	m.viewHeader(&f, inner)
	f.blank()

	switch {
	case m.showHelp:
		m.panel.SetContent(m.viewHelp(inner))
		f.add(m.panelView())
	case m.showHistory:
		m.panel.SetContent(m.viewHistory(inner))
		f.add(m.panelView())
	case m.showLyrics:
		f.add(m.viewLyrics(inner))
	default:
		m.viewNowPlaying(&f, inner)
	}

	if !m.compact() {
		f.blank()
	}
	m.viewFooter(&f, inner)
	return m.container(box).Render(f.String())
}

// panelView renders the scrollable panel, shrinking it to its content.
func (m *Model) panelView() string {
	h := m.panel.Height
	if total := m.panel.TotalLineCount(); total < h {
		m.panel.Height = total
		defer func() { m.panel.Height = h }()
	}
	return m.panel.View()
}

// panelOverflows reports whether the open panel needs scrolling.
func (m *Model) panelOverflows() bool {
	return m.panel.TotalLineCount() > m.panel.Height
}

// fixedDashboardRows is the dashboard height without the card and spectrum.
func (m *Model) fixedDashboardRows() int {
	// header 2 + blank + blank(after card) + progress + blank + status + toast + footer + border 2
	rows := 12
	if !m.compact() {
		rows += 3 // vertical padding + blank before the footer
	}
	return rows
}

// nowPlayingLayout decides how many rows album art and the spectrum get.
func (m *Model) nowPlayingLayout(inner int) (artRows, specRows int) {
	textRows := 4
	budget := 40
	if m.height > 0 {
		budget = m.height - 1 - m.fixedDashboardRows()
	}
	if budget-textRows >= 2 {
		specRows = min(5, budget-textRows)
	}
	if m.artAvailable() && inner >= 44 {
		a := min(8, budget-specRows)
		if a < 5 && specRows > 2 {
			steal := min(specRows-2, 5-a)
			specRows -= steal
			a += steal
		}
		if a >= 5 {
			artRows = min(a, maxArtRows)
		}
	}
	return artRows, specRows
}

func (m *Model) viewNowPlaying(f *frame, inner int) {
	s := m.styles
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)
	meta := m.state.Metadata
	artRows, specRows := m.nowPlayingLayout(inner)

	// Track card, with album art on the left when available.
	title, artist, album := meta.Title, meta.Artist, meta.Album
	if title == "" {
		if !m.hasState {
			title, artist = "Connecting…", ""
		} else {
			title = "Nothing playing"
			artist = "Start something in " + m.playerName
		}
		album = ""
	}
	art := ""
	if artRows > 0 {
		art = m.viewArt(artRows*2, artRows)
	}
	cardWidth := inner
	if art != "" {
		cardWidth = inner - lipgloss.Width(art) - 2
	}
	textWidth := cardWidth - 3 // left border + padding

	var card []string
	card = append(card, s.SongTitle.Width(textWidth).MaxHeight(2).Render(title))
	if artist != "" {
		card = append(card, s.Artist.Render(truncate("󰠃  "+artist, textWidth)))
	}
	if album != "" {
		card = append(card, s.Album.Render(truncate("󰀥  "+album, textWidth)))
	}
	trackCard := s.TrackCard.Width(cardWidth - 1).Render(strings.Join(card, "\n"))
	if art != "" {
		f.add(lipgloss.JoinHorizontal(lipgloss.Center, art, "  ", trackCard))
	} else {
		f.add(trackCard)
	}
	f.blank()

	// Spectrum sits directly on top of the progress bar.
	if specRows > 0 {
		f.add(m.renderViz(inner, specRows))
	}

	// Progress row.
	length := meta.Length
	posStr := format.Duration(m.pos)
	var durStr string
	var percent float64
	live := false
	switch {
	case length > 0:
		durStr = format.Duration(length)
		percent = format.Clamp01(float64(m.pos) / float64(length))
	case m.hasState && meta.Title != "" && m.state.PlaybackStatus != mpris.StatusStopped:
		live = true
		durStr = s.LiveBadge.Render("● LIVE")
	default:
		durStr = "--:--"
	}
	if !m.hasState || meta.Title == "" {
		posStr = "--:--"
	}
	posR := s.ProgressTime.Render(posStr)
	durR := durStr
	if !live {
		durR = s.ProgressTime.Render(durStr)
	}
	barWidth := inner - lipgloss.Width(posR) - lipgloss.Width(durR) - 4
	if barWidth < 4 {
		barWidth = 4
	}
	m.progressBar.Width = barWidth
	bar := m.progressBar.ViewAs(percent)
	barX := lipgloss.Width(posR) + 2
	if length > 0 && m.state.CanSeek {
		m.progressHit = hitBox{
			x0: boxOffsetX + barX,
			x1: boxOffsetX + barX + barWidth,
			y:  m.boxOffsetY() + f.rows,
			ok: true,
		}
	}
	f.add(posR + "  " + bar + "  " + durR)
	f.blank()

	// Status + volume row.
	var badge string
	switch m.state.PlaybackStatus {
	case mpris.StatusPlaying:
		badge = s.StatusPlaying.Render("▶ PLAYING")
	case mpris.StatusPaused:
		badge = s.StatusPaused.Render("⏸ PAUSED")
	default:
		badge = s.StatusStopped.Render("⏹ STOPPED")
	}
	row := badge
	if m.state.HasVolume {
		row = lipgloss.JoinHorizontal(lipgloss.Center, badge, "     ", m.viewVolume(inner))
	}
	f.add(center.Render(row))
}

func (m *Model) viewVolume(inner int) string {
	s := m.styles
	vol := format.Clamp01(m.state.Volume)
	icon := "󰕾"
	switch {
	case vol <= 0.001:
		icon = "󰖁"
	case vol < 0.34:
		icon = "󰕿"
	case vol < 0.67:
		icon = "󰖀"
	}
	width := 12
	if inner < 46 {
		width = 6
	}
	filled := format.FineBar(vol, width, "")
	empty := strings.Repeat("─", width-lipgloss.Width(filled))
	return s.VolumeLabel.Render(fmt.Sprintf("%s %3d%%", icon, int(math.Round(vol*100)))) + " " +
		s.VolumeValue.Render(filled) + s.VolumeTrack.Render(empty)
}
