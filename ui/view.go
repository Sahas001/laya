package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
	"github.com/Sahas001/laya-tui/mpris"
)

// ActiveView defines the current screen view.
type ActiveView int

const (
	ViewSelectPlayer ActiveView = iota
	ViewDashboard
)

// Msg definitions
type TickMsg time.Time
type PollMsg time.Time

type playersRefreshedMsg struct {
	players []PlayerInfo
	err     error
}

// Model represents the application state.
type Model struct {
	// D-Bus
	dbusConn *dbus.Conn

	// MPRIS Player
	player           *mpris.Player
	playerState      mpris.PlayerState
	players          []PlayerInfo
	selectedIdx      int
	activePlayerName string

	// UI State
	currentView ActiveView
	width       int
	height      int
	err         error

	// Sub-components
	progressBar  progress.Model
	viewport     viewport.Model // For lyrics
	showLyrics   bool
	showHelp     bool
	lyricsLoaded bool
	lyricsErr    error
	lyricsSongID string // Unique song key (Title + Artist) we loaded lyrics for
	lyricsLines  []LyricLine
	isSynced     bool
	lastManualScroll time.Time

	styles       Styles

	// Interpolation for smooth progress bar
	interpolatedPos time.Duration
	lastStateUpdate time.Time
}

// NewModel initializes the Bubble Tea model.
func NewModel() (*Model, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("could not connect to D-Bus session bus: %w", err)
	}

	prog := progress.New(
		progress.WithoutPercentage(),
		progress.WithScaledGradient(ColorMauve, ColorSapphire),
	)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))

	return &Model{
		dbusConn:    conn,
		progressBar:  prog,
		viewport:     vp,
		currentView: ViewSelectPlayer,
		styles:       DefaultStyles(),
	}, nil
}

// SetInitialPlayer configures the model to start directly with the given player.
func (m *Model) SetInitialPlayer(busName, identity string) {
	m.player = mpris.NewPlayer(m.dbusConn, busName)
	m.activePlayerName = identity
	m.currentView = ViewDashboard
}

// Init starts tickers and refreshes player list on startup.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshPlayersCmd(),
		tickCmd(),
		pollCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func pollCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return PollMsg(t)
	})
}

func (m *Model) refreshPlayersCmd() tea.Cmd {
	return func() tea.Msg {
		names, err := mpris.ListPlayers(m.dbusConn)
		if err != nil {
			return playersRefreshedMsg{err: err}
		}

		var infos []PlayerInfo
		for _, name := range names {
			var identity string
			if name == "org.mpris.MediaPlayer2.playerctld" {
				identity = "Global Controller (playerctld)"
			} else {
				identity, _ = mpris.GetPlayerIdentity(m.dbusConn, name)
			}
			infos = append(infos, PlayerInfo{
				BusName:  name,
				Identity: identity,
			})
		}
		return playersRefreshedMsg{players: infos}
	}
}

func songKey(artist, title string) string {
	return artist + " // " + title
}

func (m *Model) updateLyricsView() {
	if len(m.lyricsLines) == 0 {
		m.viewport.SetContent("No lyrics text available.")
		return
	}

	styles := m.styles
	var sb strings.Builder

	activeIndex := -1
	if m.isSynced {
		// Compensate for D-Bus roundtrip and audio buffer latency (250ms)
		lookupPos := m.interpolatedPos + 250*time.Millisecond
		for i, line := range m.lyricsLines {
			if lookupPos >= line.Time {
				activeIndex = i
			} else {
				break
			}
		}
	}

	for i, line := range m.lyricsLines {
		if i == activeIndex {
			// Synced highlighted line (indented to align, no arrow)
			sb.WriteString(styles.ActiveLyric.Render("  " + line.Text))
		} else {
			// Normal line (or dimmed if synced is active)
			if m.isSynced {
				sb.WriteString(styles.InactiveLyric.Render("  " + line.Text))
			} else {
				sb.WriteString("  " + line.Text)
			}
		}
		sb.WriteString("\n")
	}

	m.viewport.SetContent(sb.String())

	// Auto-scroll logic: only if lyrics are synced, we have an active line,
	// and the user has not manually scrolled in the last 5 seconds.
	if m.isSynced && activeIndex != -1 && time.Since(m.lastManualScroll) > 5*time.Second {
		targetY := activeIndex - (m.viewport.Height / 2)
		if targetY < 0 {
			targetY = 0
		}
		
		maxScroll := len(m.lyricsLines) - m.viewport.Height
		if maxScroll < 0 {
			maxScroll = 0
		}
		if targetY > maxScroll {
			targetY = maxScroll
		}
		
		m.viewport.YOffset = targetY
	}
}

func (m *Model) updateState() tea.Cmd {
	if m.player == nil {
		return nil
	}
	state, err := m.player.GetState()
	if err != nil {
		// Player likely exited or disconnected
		m.player = nil
		m.currentView = ViewSelectPlayer
		m.err = err
		m.activePlayerName = ""
		return m.refreshPlayersCmd()
	}

	var cmds []tea.Cmd

	// Track change detection
	trackChanged := state.Metadata.Title != m.playerState.Metadata.Title || state.Metadata.Artist != m.playerState.Metadata.Artist
	if trackChanged {
		// Reset lyrics state to avoid showing stale lyrics
		m.lyricsLoaded = false
		m.lyricsErr = nil
		m.lyricsLines = nil
		m.isSynced = false
		m.lyricsSongID = "" // clear it to force a reload when toggled

		// Asynchronously extract colors from new album art
		if state.Metadata.ArtURL != "" {
			cmds = append(cmds, ExtractColorsCmd(state.Metadata.ArtURL))
		}
		
		// If lyrics view is active, trigger lyrics fetch immediately
		if m.showLyrics {
			m.lyricsSongID = songKey(state.Metadata.Artist, state.Metadata.Title)
			cmds = append(cmds, FetchLyricsCmd(
				m.lyricsSongID,
				state.Metadata.Artist,
				state.Metadata.Title,
				state.Metadata.Album,
				state.Metadata.URL,
				state.Metadata.Length,
			))
		}
	} else if m.showLyrics && songKey(state.Metadata.Artist, state.Metadata.Title) != m.lyricsSongID {
		// Fallback check if lyrics view was just toggled
		m.lyricsLoaded = false
		m.lyricsErr = nil
		m.lyricsSongID = songKey(state.Metadata.Artist, state.Metadata.Title)
		cmds = append(cmds, FetchLyricsCmd(
			m.lyricsSongID,
			state.Metadata.Artist,
			state.Metadata.Title,
			state.Metadata.Album,
			state.Metadata.URL,
			state.Metadata.Length,
		))
	}

	m.playerState = state
	m.lastStateUpdate = time.Now()
	m.interpolatedPos = state.Position

	return tea.Batch(cmds...)
}

func (m *Model) cyclePlayer(forward bool) tea.Cmd {
	names, err := mpris.ListPlayers(m.dbusConn)
	if err != nil || len(names) <= 1 {
		return nil
	}

	var infos []PlayerInfo
	currentIdx := -1
	for i, name := range names {
		identity, _ := mpris.GetPlayerIdentity(m.dbusConn, name)
		infos = append(infos, PlayerInfo{
			BusName:  name,
			Identity: identity,
		})
		if m.player != nil && name == m.player.BusName() {
			currentIdx = i
		}
	}

	if len(infos) == 0 {
		return nil
	}

	var nextIdx int
	if currentIdx == -1 {
		nextIdx = 0
	} else {
		if forward {
			nextIdx = (currentIdx + 1) % len(infos)
		} else {
			nextIdx = (currentIdx - 1 + len(infos)) % len(infos)
		}
	}

	selected := infos[nextIdx]
	m.player = mpris.NewPlayer(m.dbusConn, selected.BusName)
	m.activePlayerName = selected.Identity
	m.currentView = ViewDashboard
	return m.updateState()
}

// Update handles messages and updates the state.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global Quit
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Mode-Specific Keybindings
		if m.currentView == ViewSelectPlayer {
			switch msg.String() {
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
			case "down", "j":
				if m.selectedIdx < len(m.players)-1 {
					m.selectedIdx++
				}
			case "enter":
				if len(m.players) > 0 {
					selected := m.players[m.selectedIdx]
					m.player = mpris.NewPlayer(m.dbusConn, selected.BusName)
					m.activePlayerName = selected.Identity
					m.currentView = ViewDashboard
					cmds = append(cmds, m.updateState())
				}
			case "r":
				cmds = append(cmds, m.refreshPlayersCmd())
			case "esc":
				// If player is already selected, allow canceling back to dashboard
				if m.player != nil {
					m.currentView = ViewDashboard
				}
			}
		} else if m.currentView == ViewDashboard {
			// Dashboard View Keybindings
			if m.showLyrics {
				// While in lyrics mode
				switch msg.String() {
				case "esc", "l":
					m.showLyrics = false
				case "h":
					m.showHelp = !m.showHelp
				case " ":
					if m.player != nil {
						_ = m.player.PlayPause()
						cmds = append(cmds, m.updateState())
					}
				case "n", "right":
					if m.player != nil {
						_ = m.player.Next()
						cmds = append(cmds, m.updateState())
					}
				case "p", "left":
					if m.player != nil {
						_ = m.player.Previous()
						cmds = append(cmds, m.updateState())
					}
				case "+", "=":
					if m.player != nil {
						_ = m.player.SetVolume(m.playerState.Volume + 0.05)
						cmds = append(cmds, m.updateState())
					}
				case "-":
					if m.player != nil {
						_ = m.player.SetVolume(m.playerState.Volume - 0.05)
						cmds = append(cmds, m.updateState())
					}
				case "[":
					if m.player != nil {
						_ = m.player.Seek(-5000000)
						cmds = append(cmds, m.updateState())
					}
				case "]":
					if m.player != nil {
						_ = m.player.Seek(5000000)
						cmds = append(cmds, m.updateState())
					}
				case "H":
					cmds = append(cmds, m.cyclePlayer(false))
				case "L":
					cmds = append(cmds, m.cyclePlayer(true))
				case "up", "down", "j", "k", "pgup", "pgdown":
					m.lastManualScroll = time.Now()
					var vpCmd tea.Cmd
					m.viewport, vpCmd = m.viewport.Update(msg)
					cmds = append(cmds, vpCmd)
				default:
					// Pass other keys (scrolling) to viewport
					var vpCmd tea.Cmd
					m.viewport, vpCmd = m.viewport.Update(msg)
					cmds = append(cmds, vpCmd)
				}
			} else {
				// Normal dashboard view
				switch msg.String() {
				case " ":
					if m.player != nil {
						_ = m.player.PlayPause()
						cmds = append(cmds, m.updateState())
					}
				case "n", "right":
					if m.player != nil {
						_ = m.player.Next()
						cmds = append(cmds, m.updateState())
					}
				case "p", "left":
					if m.player != nil {
						_ = m.player.Previous()
						cmds = append(cmds, m.updateState())
					}
				case "up", "+", "=":
					if m.player != nil {
						_ = m.player.SetVolume(m.playerState.Volume + 0.05)
						cmds = append(cmds, m.updateState())
					}
				case "down", "-":
					if m.player != nil {
						_ = m.player.SetVolume(m.playerState.Volume - 0.05)
						cmds = append(cmds, m.updateState())
					}
				case "[":
					if m.player != nil {
						_ = m.player.Seek(-5000000)
						cmds = append(cmds, m.updateState())
					}
				case "]":
					if m.player != nil {
						_ = m.player.Seek(5000000)
						cmds = append(cmds, m.updateState())
					}
				case "l":
					m.showLyrics = !m.showLyrics
					if m.showLyrics && (songKey(m.playerState.Metadata.Artist, m.playerState.Metadata.Title) != m.lyricsSongID || !m.lyricsLoaded) {
						m.lyricsLoaded = false
						m.lyricsErr = nil
						m.lyricsSongID = songKey(m.playerState.Metadata.Artist, m.playerState.Metadata.Title)
						cmds = append(cmds, FetchLyricsCmd(
							m.lyricsSongID,
							m.playerState.Metadata.Artist,
							m.playerState.Metadata.Title,
							m.playerState.Metadata.Album,
							m.playerState.Metadata.URL,
							m.playerState.Metadata.Length,
						))
					}
				case "H":
					cmds = append(cmds, m.cyclePlayer(false))
				case "L":
					cmds = append(cmds, m.cyclePlayer(true))
				case "s":
					m.currentView = ViewSelectPlayer
					cmds = append(cmds, m.refreshPlayersCmd())
				case "h":
					m.showHelp = !m.showHelp
				}
			}
		}

	case tea.MouseMsg:
		if m.currentView == ViewDashboard && m.showLyrics {
			if msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown {
				m.lastManualScroll = time.Now()
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Resize viewport for lyrics view. Height budget:
		// Total Height - (Header 3 + Lyric Header 3 + Footer 3 + Pad 3) = -12
		vpHeight := msg.Height - 12
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = msg.Width - 8
		m.viewport.Height = vpHeight

	case playersRefreshedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.players = msg.players
		
		// If only one player is active and we don't have a player connected yet, auto-select it!
		if m.player == nil && len(m.players) == 1 {
			selected := m.players[0]
			m.player = mpris.NewPlayer(m.dbusConn, selected.BusName)
			m.activePlayerName = selected.Identity
			m.currentView = ViewDashboard
			cmds = append(cmds, m.updateState())
		} else if len(m.players) > 0 {
			if m.selectedIdx >= len(m.players) {
				m.selectedIdx = 0
			}
		} else {
			m.selectedIdx = 0
		}

	case ThemeUpdateMsg:
		m.UpdateThemeStyles(msg.Colors)

	case LyricsMsg:
		if msg.SongID != m.lyricsSongID {
			return m, nil
		}
		if msg.Err != nil {
			m.lyricsLoaded = false
			m.lyricsErr = msg.Err
			m.lyricsLines = nil
			m.isSynced = false
		} else {
			m.lyricsLoaded = true
			m.lyricsErr = nil
			if msg.SyncedLyrics != "" {
				m.lyricsLines = ParseSyncedLrc(msg.SyncedLyrics)
				m.isSynced = true
			} else {
				m.lyricsLines = ParsePlainLrc(msg.PlainLyrics)
				m.isSynced = false
			}
			m.updateLyricsView()
			m.viewport.GotoTop()
		}

	case TickMsg:
		if m.player != nil && m.playerState.PlaybackStatus == "Playing" {
			now := time.Now()
			elapsed := now.Sub(m.lastStateUpdate)
			m.interpolatedPos = m.playerState.Position + elapsed
			
			// Cap at track length if known
			totalLen := m.playerState.Metadata.Length
			if totalLen > 0 && m.interpolatedPos > totalLen {
				m.interpolatedPos = totalLen
			}

			// Update synced lyrics display on each tick
			if m.showLyrics && m.lyricsLoaded {
				m.updateLyricsView()
			}
		}
		cmds = append(cmds, tickCmd())

	case PollMsg:
		if m.player != nil {
			cmds = append(cmds, m.updateState())
		}
		cmds = append(cmds, pollCmd())
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI screen.
func (m *Model) View() string {
	var content string
	switch m.currentView {
	case ViewSelectPlayer:
		content = m.viewSelectPlayer(m.styles)
	case ViewDashboard:
		content = m.viewDashboard(m.styles)
	default:
		content = "Laya TUI: Unknown View State"
	}

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	return content
}
