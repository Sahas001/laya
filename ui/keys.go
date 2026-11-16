package ui

import (
	"fmt"
	"math"
	"time"

	"github.com/Sahas001/laya/mpris"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		return tea.Quit
	}
	if m.view == ViewSelectPlayer {
		return m.handleSelectKey(key)
	}

	// Scrollable panels (help, history).
	if m.showHelp || m.showHistory {
		switch key {
		case "up", "k", "down", "j", "pgup", "pgdown", "ctrl+u", "ctrl+d", "home", "end", "g", "G":
			switch key {
			case "g", "home":
				m.panel.GotoTop()
			case "G", "end":
				m.panel.GotoBottom()
			default:
				var cmd tea.Cmd
				m.panel, cmd = m.panel.Update(msg)
				return cmd
			}
			return nil
		}
	}

	// Lyrics-only keys.
	if m.showLyrics && !m.showHelp && !m.showHistory {
		switch key {
		case "up", "k", "down", "j", "pgup", "pgdown", "ctrl+u", "ctrl+d":
			m.lyrics.lastManualScroll = time.Now()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return cmd
		case "g", "home":
			m.lyrics.lastManualScroll = time.Now()
			m.viewport.GotoTop()
			return nil
		case "G", "end":
			m.lyrics.lastManualScroll = time.Now()
			m.viewport.GotoBottom()
			return nil
		case "f":
			m.lyrics.lastManualScroll = time.Time{}
			m.refreshLyrics()
			return m.setToast("Following lyrics", false)
		case "<", ",":
			return m.shiftLyrics(-lyricOffsetStep)
		case ">", ".":
			return m.shiftLyrics(lyricOffsetStep)
		}
	}

	switch key {
	case "esc":
		switch {
		case m.showHelp:
			m.showHelp = false
		case m.showHistory:
			m.showHistory = false
		case m.fullscreen:
			m.fullscreen = false
		case m.showLyrics:
			m.showLyrics = false
		}
	case "h", "?":
		m.showHelp = !m.showHelp
		m.showHistory = false
		m.fullscreen = false
		m.panel.GotoTop()
	case "R":
		m.showHistory = !m.showHistory
		m.showHelp = false
		m.fullscreen = false
		m.panel.GotoTop()
	case "v":
		m.viz.style = (m.viz.style + 1) % numVizStyles
		return m.setToast("Visualizer: "+m.viz.style.String(), false)
	case "V":
		m.fullscreen = !m.fullscreen
		m.showHelp, m.showHistory = false, false
		return tickCmd(tickFast)
	case "A":
		return m.toggleAudio()
	case "a":
		m.showArt = !m.showArt
		if !m.showArt {
			m.deleteKittyImage()
			return m.setToast("Album art hidden", false)
		}
		if m.art.img == nil {
			return m.setToast("No album art for this track", false)
		}
		return m.setToast("Album art shown", false)
	case "t":
		return m.cycleSleepTimer()
	case "y":
		return m.copyTrack()
	case " ", "enter":
		return m.act("Play/pause", (*mpris.Player).PlayPause)
	case "n", "right":
		return m.act("Next track", (*mpris.Player).Next)
	case "p", "left":
		return m.act("Previous track", (*mpris.Player).Previous)
	case "[":
		return m.seek(-seekSmall)
	case "]":
		return m.seek(seekSmall)
	case "{":
		return m.seek(-seekLarge)
	case "}":
		return m.seek(seekLarge)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return m.jumpTo(float64(key[0]-'0') / 10)
	case "+", "=", "up":
		return m.changeVolume(volumeStep)
	case "-", "_", "down":
		return m.changeVolume(-volumeStep)
	case "m":
		return m.toggleMute()
	case "z":
		return m.toggleShuffle()
	case "r":
		return m.cycleLoop()
	case "l":
		m.showLyrics = !m.showLyrics
		m.showHelp, m.showHistory, m.fullscreen = false, false, false
		if m.showLyrics {
			m.resize()
			return m.ensureLyrics()
		}
	case "H":
		return m.cyclePlayer(false)
	case "L":
		return m.cyclePlayer(true)
	case "s", "tab":
		m.view = ViewSelectPlayer
		m.showHelp = false
		for i, p := range m.players {
			if m.player != nil && p.BusName == m.player.BusName() {
				m.cursor = i
			}
		}
		return m.refreshPlayersCmd()
	}
	return nil
}

func (m *Model) handleSelectKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.players)-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if len(m.players) > 0 {
			m.cursor = len(m.players) - 1
		}
	case "enter", " ", "l":
		if m.cursor < len(m.players) {
			return m.selectPlayer(m.players[m.cursor])
		}
	case "r":
		return tea.Batch(m.refreshPlayersCmd(), m.setToast("Refreshing players…", false))
	case "esc", "s", "tab":
		if m.player != nil {
			m.view = ViewDashboard
		}
	}
	return nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	switch m.view {
	case ViewSelectPlayer:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.MouseButtonWheelDown:
			if m.cursor < len(m.players)-1 {
				m.cursor++
			}
		}
		return nil
	}

	if m.fullscreen {
		return nil
	}
	if m.showHelp || m.showHistory {
		var cmd tea.Cmd
		m.panel, cmd = m.panel.Update(msg)
		return cmd
	}
	if m.showLyrics {
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			m.lyrics.lastManualScroll = time.Now()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return cmd
		}
		return nil
	}

	switch {
	case msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress:
		return m.changeVolume(volumeStep)
	case msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress:
		return m.changeVolume(-volumeStep)
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		hb := m.progressHit
		x, y := msg.X-m.boxX, msg.Y-m.boxY
		if hb.ok && y == hb.y && x >= hb.x0 && x < hb.x1 {
			frac := (float64(x-hb.x0) + 0.5) / float64(hb.x1-hb.x0)
			return m.jumpTo(frac)
		}
	}
	return nil
}

// --- player actions ---

func (m *Model) unsupported(what string) tea.Cmd {
	name := m.playerName
	if name == "" {
		name = "this player"
	}
	return m.setToast(fmt.Sprintf("%s isn't supported by %s", what, name), true)
}

// optimisticPosition updates the local position immediately so the UI feels instant.
func (m *Model) optimisticPosition(pos time.Duration) {
	if l := m.state.Metadata.Length; l > 0 && pos > l {
		pos = l
	}
	if pos < 0 {
		pos = 0
	}
	m.state.Position = pos
	m.state.LastUpdated = time.Now()
	m.pos = pos
	if m.showLyrics {
		m.refreshLyrics()
	}
}

func (m *Model) seek(delta time.Duration) tea.Cmd {
	if m.player == nil || !m.hasState {
		return nil
	}
	if !m.state.CanSeek {
		return m.unsupported("Seeking")
	}
	m.optimisticPosition(m.pos + delta)
	return m.act("Seek", func(p *mpris.Player) error { return p.SeekBy(delta) })
}

func (m *Model) jumpTo(frac float64) tea.Cmd {
	if m.player == nil || !m.hasState {
		return nil
	}
	length := m.state.Metadata.Length
	if !m.state.CanSeek || length <= 0 {
		return m.unsupported("Seeking")
	}
	target := time.Duration(frac * float64(length))
	trackID := m.state.Metadata.TrackID
	m.optimisticPosition(target)
	return m.act("Seek", func(p *mpris.Player) error {
		if err := p.SetPosition(trackID, target); err != nil {
			// Fallback for players without track ids: relative seek.
			cur, perr := p.GetPosition()
			if perr != nil {
				return err
			}
			return p.SeekBy(target - cur)
		}
		return nil
	})
}

func (m *Model) setVolume(v float64) tea.Cmd {
	if m.player == nil || !m.hasState {
		return nil
	}
	if !m.state.HasVolume {
		return m.unsupported("Volume control")
	}
	v = math.Round(math.Max(0, math.Min(1, v))*100) / 100
	m.state.Volume = v
	return m.act("Volume", func(p *mpris.Player) error { return p.SetVolume(v) })
}

func (m *Model) changeVolume(delta float64) tea.Cmd {
	// Snap to the step grid so repeated presses land on round numbers.
	v := math.Round((m.state.Volume+delta)/volumeStep) * volumeStep
	return m.setVolume(v)
}

func (m *Model) toggleMute() tea.Cmd {
	if m.state.Volume > 0.001 {
		m.preMuteVol = m.state.Volume
		return m.setVolume(0)
	}
	restore := m.preMuteVol
	if restore <= 0 {
		restore = 0.5
	}
	return m.setVolume(restore)
}

func (m *Model) toggleShuffle() tea.Cmd {
	if m.player == nil || !m.hasState {
		return nil
	}
	if !m.state.HasShuffle {
		return m.unsupported("Shuffle")
	}
	on := !m.state.Shuffle
	m.state.Shuffle = on
	label := "Shuffle off"
	if on {
		label = "Shuffle on"
	}
	return tea.Batch(
		m.act("Shuffle", func(p *mpris.Player) error { return p.SetShuffle(on) }),
		m.setToast(label, false),
	)
}

func (m *Model) cycleLoop() tea.Cmd {
	if m.player == nil || !m.hasState {
		return nil
	}
	if m.state.LoopStatus == "" {
		return m.unsupported("Loop mode")
	}
	next := map[string]string{
		mpris.LoopNone:     mpris.LoopPlaylist,
		mpris.LoopPlaylist: mpris.LoopTrack,
		mpris.LoopTrack:    mpris.LoopNone,
	}[m.state.LoopStatus]
	if next == "" {
		next = mpris.LoopNone
	}
	m.state.LoopStatus = next
	return tea.Batch(
		m.act("Loop mode", func(p *mpris.Player) error { return p.SetLoopStatus(next) }),
		m.setToast("Loop: "+next, false),
	)
}

func (m *Model) shiftLyrics(d time.Duration) tea.Cmd {
	if !m.lyrics.synced {
		return m.setToast("Lyrics aren't synced", false)
	}
	m.lyrics.offset += d
	m.lyricsDirty = true
	m.refreshLyrics()
	return m.setToast("Lyrics offset "+formatOffset(m.lyrics.offset), false)
}

// toggleAudio turns live audio capture on or off.
func (m *Model) toggleAudio() tea.Cmd {
	if m.viz.capture == nil {
		return m.setToast("Live audio is disabled (--no-audio)", false)
	}
	if m.realAudio() {
		m.viz.audioOn = false
		m.viz.capture.Stop()
		return m.setToast("Visualizer: synthetic (audio capture off)", false)
	}
	m.viz.audioOn = true
	if err := m.viz.capture.Start(); err != nil {
		m.viz.audioOn = false
		return m.setToast("Audio capture failed: "+shortErr(err), true)
	}
	return m.setToast("Visualizer: live audio", false)
}
