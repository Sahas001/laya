package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Sahas001/laya/internal/format"
	"github.com/Sahas001/laya/internal/notify"
	"github.com/Sahas001/laya/mpris"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HistoryEntry is one recently played track (kept for this session only).
type HistoryEntry struct {
	Title, Artist, Player string
	At                    time.Time
	key                   string
}

type copyDoneMsg struct{ err error }

// sleepSteps are the sleep-timer presets cycled with `t`.
var sleepSteps = []time.Duration{15 * time.Minute, 30 * time.Minute, 45 * time.Minute, 60 * time.Minute, 90 * time.Minute}

// trackChanged records history and fires a notification when the track changes.
func (m *Model) trackChanged(st mpris.PlayerState) tea.Cmd {
	key := st.Metadata.Key()
	if key == m.lastTrack {
		return nil
	}
	first := !m.trackPrimed
	m.lastTrack, m.trackPrimed = key, true
	meta := st.Metadata
	if meta.Title == "" {
		return nil
	}

	if len(m.history) == 0 || m.history[0].key != key {
		m.history = append([]HistoryEntry{{
			Title: meta.Title, Artist: meta.Artist, Player: m.playerName, At: time.Now(), key: key,
		}}, m.history...)
		if len(m.history) > historyLimit {
			m.history = m.history[:historyLimit]
		}
	}

	if first || m.notifier == nil || st.PlaybackStatus != mpris.StatusPlaying {
		return nil
	}
	n := m.notifier
	t := notify.Track{Title: meta.Title, Artist: meta.Artist, Album: meta.Album, Player: m.playerName, ArtURL: meta.ArtURL}
	return func() tea.Msg {
		_ = n.Notify(t)
		return nil
	}
}

// cycleSleepTimer steps through off → 15 → 30 → 45 → 60 → 90 → off minutes.
func (m *Model) cycleSleepTimer() tea.Cmd {
	next := time.Duration(0)
	if m.sleepAt.IsZero() {
		next = sleepSteps[0]
	} else {
		remaining := time.Until(m.sleepAt)
		for _, s := range sleepSteps {
			if s > remaining+time.Minute {
				next = s
				break
			}
		}
	}
	if next == 0 {
		m.sleepAt = time.Time{}
		return m.setToast("Sleep timer off", false)
	}
	m.sleepAt = time.Now().Add(next)
	return m.setToast(fmt.Sprintf("Sleep timer: pausing in %d min", int(next.Minutes())), false)
}

func (m *Model) checkSleepTimer(now time.Time) tea.Cmd {
	if m.sleepAt.IsZero() || now.Before(m.sleepAt) {
		return nil
	}
	m.sleepAt = time.Time{}
	if m.player == nil || m.state.PlaybackStatus != mpris.StatusPlaying {
		return nil
	}
	return tea.Batch(m.act("Sleep timer", (*mpris.Player).Pause), m.setToast("Sleep timer: paused playback 󰒲", false))
}

// sleepLabel is the header indicator text for an active sleep timer.
func (m *Model) sleepLabel() string {
	if m.sleepAt.IsZero() {
		return ""
	}
	left := time.Until(m.sleepAt)
	if left < 10*time.Minute {
		return "󰒲 " + format.Duration(left)
	}
	return fmt.Sprintf("󰒲 %dm", int(left.Minutes()+0.5))
}

// trackText is "Artist - Title" (or just the title).
func trackText(meta mpris.TrackMetadata) string {
	if meta.Artist == "" {
		return meta.Title
	}
	return meta.Artist + " - " + meta.Title
}

// copyTrack copies "Artist - Title" via OSC 52 and, when available, wl-copy/xclip.
func (m *Model) copyTrack() tea.Cmd {
	text := trackText(m.state.Metadata)
	if m.state.Metadata.Title == "" {
		return m.setToast("Nothing to copy", false)
	}
	m.term.CopyOSC52(text)
	toast := m.setToast("Copied: "+text, false)
	tool := clipboardCommand()
	if tool == nil {
		if m.term == nil {
			return m.setToast("No clipboard available", true)
		}
		return toast
	}
	return tea.Batch(toast, func() tea.Msg {
		cmd := exec.Command(tool[0], tool[1:]...)
		cmd.Stdin = strings.NewReader(text)
		return copyDoneMsg{err: cmd.Run()}
	})
}

func clipboardCommand() []string {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if p, err := exec.LookPath("wl-copy"); err == nil {
			return []string{p}
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if p, err := exec.LookPath("xclip"); err == nil {
			return []string{p, "-selection", "clipboard"}
		}
		if p, err := exec.LookPath("xsel"); err == nil {
			return []string{p, "--clipboard", "--input"}
		}
	}
	return nil
}

// viewHistory renders the recently played list.
func (m *Model) viewHistory(inner int) string {
	s := m.styles
	if len(m.history) == 0 {
		return s.LyricsMeta.Render("Nothing played yet this session.")
	}
	var lines []string
	for i, h := range m.history {
		marker := s.HelpText.Render("  ")
		titleStyle := s.SelectInactive
		if i == 0 && h.key == m.state.Metadata.Key() {
			marker = s.DotPlaying.Render("▶ ")
			titleStyle = s.SelectTitle
		}
		when := s.HelpText.Render(h.At.Format("15:04") + "  ")
		player := ""
		if h.Player != "" {
			player = s.HelpText.Render("  " + h.Player)
		}
		room := inner - lipgloss.Width(marker) - lipgloss.Width(when) - lipgloss.Width(player)
		title := h.Title
		if h.Artist != "" {
			title += " — " + h.Artist
		}
		if room < 12 {
			player, room = "", inner-lipgloss.Width(marker)-lipgloss.Width(when)
		}
		lines = append(lines, marker+when+titleStyle.Render(truncate(title, room))+player)
	}
	return strings.Join(lines, "\n")
}

// --- fullscreen visualizer ---

// vizArea returns the size the visualizer is drawn at in the current view.
func (m *Model) vizArea() (int, int) {
	if m.fullscreen {
		w, h, _ := m.fullscreenLayout()
		return w, h
	}
	_, inner := m.layout()
	_, spec := m.nowPlayingLayout(inner)
	return inner, spec
}

// fullscreenLayout returns the visualizer size and whether a lyric line fits.
func (m *Model) fullscreenLayout() (w, h int, lyric bool) {
	w = m.width - 4
	h = m.height - 6 // title, blank, …, blank, progress, hints
	lyric = m.lyrics.synced && len(m.lyrics.lines) > 0 && h >= 10
	if lyric {
		h -= 2
	}
	return max(w, 1), max(h, 1), lyric
}

func (m *Model) viewFullscreen() string {
	s := m.styles
	w, h, lyric := m.fullscreenLayout()
	meta := m.state.Metadata
	pad := "  "

	// Title row: track on the left, player + style on the right.
	icon := s.DotStopped.Render("⏹")
	switch m.state.PlaybackStatus {
	case mpris.StatusPlaying:
		icon = s.DotPlaying.Render("▶")
	case mpris.StatusPaused:
		icon = s.DotPaused.Render("⏸")
	}
	src := "synthetic"
	if m.realAudio() {
		src = "live audio"
	}
	right := s.HelpText.Render(strings.Join(nonEmpty(m.playerName, m.viz.style.String(), src, m.sleepLabel()), " · "))
	title := meta.Title
	if title == "" {
		title = "Nothing playing"
	}
	left := icon + " " + s.SongTitle.Render(title)
	if meta.Artist != "" {
		left += s.Artist.Render("  " + meta.Artist)
	}
	left = truncate(left, w-lipgloss.Width(right)-2)
	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	lines := []string{"", pad + left + strings.Repeat(" ", gap) + right, ""}

	for _, row := range strings.Split(m.renderViz(w, h), "\n") {
		lines = append(lines, pad+row)
	}

	if lyric {
		text := ""
		if i := m.activeLyric(); i >= 0 {
			text = m.lyrics.lines[i].Text
		}
		lines = append(lines, "", pad+lipgloss.NewStyle().Width(w).Align(lipgloss.Center).
			Render(s.ActiveLyric.Render(truncate(text, w))))
	}

	// Progress row.
	posStr, durStr, pct := format.Duration(m.pos), "--:--", 0.0
	if l := meta.Length; l > 0 {
		durStr = format.Duration(l)
		pct = format.Clamp01(float64(m.pos) / float64(l))
	}
	bw := max(4, w-len(posStr)-len(durStr)-4)
	m.progressBar.Width = bw
	lines = append(lines, pad+s.ProgressTime.Render(posStr)+"  "+m.progressBar.ViewAs(pct)+"  "+s.ProgressTime.Render(durStr))

	hint := m.renderHints([]hint{{"v", "style", 1}, {"V/esc", "exit", 0}, {"␣", "play/pause", 1}, {"A", "audio", 2}, {"q", "quit", 1}}, w)
	if m.toast != "" {
		st := s.Toast
		if m.toastErr {
			st = s.ToastErr
		}
		hint = st.Render(truncate(m.toast, w))
	}
	lines = append(lines, pad+lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(hint))

	out := strings.Join(lines, "\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, out)
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
