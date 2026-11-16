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
	case m.showHelp:
		hints = []hint{{"esc", "close help", 0}, {"q", "quit", 1}}
	case m.showLyrics:
		hints = []hint{{"␣", "play/pause", 1}, {"j/k", "scroll", 2}, {"</>", "timing", 4},
			{"l", "back", 0}, {"?", "help", 0}, {"q", "quit", 3}}
	default:
		hints = []hint{{"␣", "play/pause", 1}, {"←/→", "skip", 4}, {"l", "lyrics", 2},
			{"s", "players", 3}, {"?", "help", 0}, {"q", "quit", 2}}
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
		f.add(m.viewHelp(inner))
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

func (m *Model) viewNowPlaying(f *frame, inner int) {
	s := m.styles
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)
	meta := m.state.Metadata

	// Track card.
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

	showViz := inner >= 46
	vizWidth := 0
	if showViz {
		vizWidth = numVizBars*2 - 1 + 4 // bars + gaps + padding + border
	}
	cardWidth := inner
	if showViz {
		cardWidth = inner - vizWidth - 2
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

	if showViz {
		f.add(lipgloss.JoinHorizontal(lipgloss.Center, trackCard, "  ", m.viewVisualizer()))
	} else {
		f.add(trackCard)
	}
	f.blank()

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

// --- visualizer ---

var vizLevels = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// animateViz moves the bars towards a pseudo-random wave while playing and
// lets them settle when paused.
func (m *Model) animateViz() {
	playing := m.player != nil && m.state.PlaybackStatus == mpris.StatusPlaying
	if playing {
		m.vizPhase += 0.21
	}
	for i := range m.viz {
		target := 1.0
		ease := 0.12
		if playing {
			fi := float64(i)
			p := m.vizPhase
			target = 12 + 6.5*math.Sin(p+fi*0.9) + 4*math.Sin(p*1.73+fi*2.3) + 2.5*math.Sin(p*3.1+fi*0.55)
			target = math.Max(2, math.Min(24, target))
			ease = 0.45
		}
		m.viz[i] += (target - m.viz[i]) * ease
	}
}

func (m *Model) viewVisualizer() string {
	s := m.styles
	rowsStyles := []struct {
		st   lipgloss.Style
		base int
	}{{s.VizTop, 16}, {s.VizMid, 8}, {s.VizBot, 0}}

	lines := make([]string, len(rowsStyles))
	for r, rs := range rowsStyles {
		var sb strings.Builder
		for i, h := range m.viz {
			if i > 0 {
				sb.WriteByte(' ')
			}
			lvl := int(math.Round(h)) - rs.base
			if lvl < 0 {
				lvl = 0
			}
			if lvl > 8 {
				lvl = 8
			}
			sb.WriteString(vizLevels[lvl])
		}
		lines[r] = rs.st.Render(sb.String())
	}
	return s.VizBorder.Render(strings.Join(lines, "\n"))
}
