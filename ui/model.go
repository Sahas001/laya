// Package ui implements the Bubble Tea interface for Laya.
package ui

import (
	"strings"
	"time"

	"github.com/Sahas001/laya/mpris"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

// ActiveView defines the current screen.
type ActiveView int

const (
	ViewSelectPlayer ActiveView = iota
	ViewDashboard
)

const (
	tickInterval     = 80 * time.Millisecond
	pollInterval     = time.Second
	toastDuration    = 3 * time.Second
	seekSmall        = 5 * time.Second
	seekLarge        = 30 * time.Second
	volumeStep       = 0.05
	lyricLatency     = 250 * time.Millisecond // D-Bus round trip + audio buffer compensation
	lyricOffsetStep  = 250 * time.Millisecond
	manualScrollHold = 5 * time.Second
	numVizBars       = 8
)

// Messages.
type (
	// TickMsg drives animation and position interpolation.
	TickMsg time.Time
	// PollMsg drives the fallback state poll.
	PollMsg time.Time
	// BusEventMsg is sent by the app when an MPRIS signal arrives.
	BusEventMsg struct{ PlayersChanged bool }

	stateMsg struct {
		bus   string
		state mpris.PlayerState
		err   error
	}
	actionMsg struct {
		bus, desc string
		err       error
	}
	playersRefreshedMsg struct {
		players []PlayerInfo
		err     error
	}
	toastExpiredMsg struct{ id int }
)

// hitBox is a clickable horizontal span, relative to the box's top-left corner.
type hitBox struct {
	x0, x1, y int
	ok        bool
}

// lyricsState holds everything about the lyrics of the current song.
type lyricsState struct {
	songID           string
	loading          bool
	err              error
	lines            []LyricLine
	synced           bool
	source           string
	offset           time.Duration
	renderedActive   int
	rowStart         []int
	rowHeight        []int
	totalRows        int
	lastManualScroll time.Time
}

// Model represents the application state.
type Model struct {
	conn *dbus.Conn

	// Active player.
	player       *mpris.Player
	playerName   string
	state        mpris.PlayerState
	hasState     bool
	pos          time.Duration // interpolated position
	fetching     bool
	fetchPending bool
	preMuteVol   float64

	// Player picker.
	players       []PlayerInfo
	playersLoaded bool
	cursor        int

	// UI.
	view        ActiveView
	width       int
	height      int
	styles      Styles
	themeArt    string
	progressBar progress.Model
	showHelp    bool
	showLyrics  bool
	viewport    viewport.Model
	lyrics      lyricsState
	lyricsDirty bool
	viz         []float64
	vizPhase    float64
	toast       string
	toastErr    bool
	toastID     int
	progressHit hitBox
	boxX, boxY  int
}

func newProgressBar(p Palette) progress.Model {
	bar := progress.New(
		progress.WithoutPercentage(),
		progress.WithScaledGradient(p.Primary, p.Tertiary),
		progress.WithFillCharacters('━', '━'),
	)
	bar.EmptyColor = ColorSurface
	return bar
}

// NewModel initialises the Bubble Tea model using an existing session-bus connection.
func NewModel(conn *dbus.Conn) *Model {
	vp := viewport.New(0, 0)
	vp.MouseWheelDelta = 2
	return &Model{
		conn:        conn,
		view:        ViewSelectPlayer,
		styles:      DefaultStyles(),
		progressBar: newProgressBar(DefaultPalette),
		viewport:    vp,
		viz:         make([]float64, numVizBars),
		lyrics:      lyricsState{renderedActive: -2},
	}
}

// SetInitialPlayer configures the model to start directly with the given player.
func (m *Model) SetInitialPlayer(busName, identity string) {
	m.player = mpris.NewPlayer(m.conn, busName)
	m.playerName = identity
	m.view = ViewDashboard
}

// Init starts tickers and loads the player list.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.refreshPlayersCmd(), m.fetchState(), tickCmd(), pollCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return TickMsg(t) })
}

func pollCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg { return PollMsg(t) })
}

// --- async commands ---

// fetchState requests the player's state in the background. Only one fetch is
// in flight at a time; extra requests are coalesced into one follow-up fetch.
func (m *Model) fetchState() tea.Cmd {
	if m.player == nil {
		return nil
	}
	if m.fetching {
		m.fetchPending = true
		return nil
	}
	m.fetching = true
	p := m.player
	return func() tea.Msg {
		st, err := p.GetState()
		return stateMsg{bus: p.BusName(), state: st, err: err}
	}
}

// act runs a player command in the background and reports the result.
func (m *Model) act(desc string, fn func(p *mpris.Player) error) tea.Cmd {
	if m.player == nil {
		return nil
	}
	p := m.player
	return func() tea.Msg {
		return actionMsg{bus: p.BusName(), desc: desc, err: fn(p)}
	}
}

func (m *Model) refreshPlayersCmd() tea.Cmd {
	conn := m.conn
	return func() tea.Msg {
		players, err := loadPlayers(conn)
		return playersRefreshedMsg{players: players, err: err}
	}
}

func (m *Model) setToast(text string, isErr bool) tea.Cmd {
	m.toastID++
	m.toast, m.toastErr = text, isErr
	id := m.toastID
	return tea.Tick(toastDuration, func(time.Time) tea.Msg { return toastExpiredMsg{id: id} })
}

// --- player lifecycle ---

func (m *Model) selectPlayer(info PlayerInfo) tea.Cmd {
	m.player = mpris.NewPlayer(m.conn, info.BusName)
	m.playerName = info.Identity
	m.view = ViewDashboard
	m.state = mpris.PlayerState{}
	m.hasState = false
	m.pos = 0
	m.preMuteVol = 0
	m.lyrics = lyricsState{renderedActive: -2}
	m.lyricsDirty = true
	return m.fetchState()
}

func (m *Model) playerLost() tea.Cmd {
	name := m.playerName
	m.player = nil
	m.playerName = ""
	m.state = mpris.PlayerState{}
	m.hasState = false
	m.showLyrics = false
	m.view = ViewSelectPlayer
	m.themeArt = ""
	m.applyTheme(nil)
	if name == "" {
		name = "Player"
	}
	return tea.Batch(m.setToast(name+" disconnected", true), m.refreshPlayersCmd())
}

func (m *Model) cyclePlayer(forward bool) tea.Cmd {
	var real []PlayerInfo
	for _, p := range m.players {
		if !mpris.IsPlayerctld(p.BusName) {
			real = append(real, p)
		}
	}
	if len(real) < 2 {
		return m.setToast("No other players to switch to", false)
	}
	idx := -1
	for i, p := range real {
		if m.player != nil && p.BusName == m.player.BusName() {
			idx = i
		}
	}
	next := 0
	if idx >= 0 {
		if forward {
			next = (idx + 1) % len(real)
		} else {
			next = (idx - 1 + len(real)) % len(real)
		}
	}
	return tea.Batch(m.selectPlayer(real[next]), m.setToast("Switched to "+real[next].Label, false))
}

// applyState stores a freshly fetched state and reacts to track/art changes.
func (m *Model) applyState(st mpris.PlayerState) tea.Cmd {
	var cmds []tea.Cmd
	m.state = st
	m.hasState = true
	m.pos = st.PositionAt(time.Now())

	if st.Metadata.ArtURL != m.themeArt {
		m.themeArt = st.Metadata.ArtURL
		if m.themeArt == "" {
			m.applyTheme(nil)
		} else {
			cmds = append(cmds, ExtractColorsCmd(m.themeArt))
		}
	}

	if m.showLyrics {
		cmds = append(cmds, m.ensureLyrics())
		m.lyricsDirty = true
		m.refreshLyrics()
	}
	return tea.Batch(cmds...)
}

func (m *Model) handlePlayers(msg playersRefreshedMsg) tea.Cmd {
	if msg.err != nil {
		return m.setToast("Could not list players: "+msg.err.Error(), true)
	}
	first := !m.playersLoaded
	m.playersLoaded = true
	m.players = msg.players

	if m.player != nil {
		for _, p := range m.players {
			if p.BusName == m.player.BusName() {
				if m.playerName == "" || m.playerName == mpris.ShortName(p.BusName) {
					m.playerName = p.Identity
				}
				return nil
			}
		}
		return m.playerLost()
	}

	var real []PlayerInfo
	for _, p := range m.players {
		if !mpris.IsPlayerctld(p.BusName) {
			real = append(real, p)
		}
	}
	switch {
	case len(real) == 1:
		return m.selectPlayer(real[0])
	case len(real) == 0 && len(m.players) == 1:
		return m.selectPlayer(m.players[0])
	}

	if first || m.cursor >= len(m.players) {
		m.cursor = 0
		for i, p := range m.players {
			if p.Status == mpris.StatusPlaying && !mpris.IsPlayerctld(p.BusName) {
				m.cursor = i
				break
			}
		}
	}
	return nil
}

// --- Update ---

// Update handles messages and updates the state.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case BusEventMsg:
		cmds := []tea.Cmd{m.fetchState()}
		if msg.PlayersChanged {
			cmds = append(cmds, m.refreshPlayersCmd())
		}
		return m, tea.Batch(cmds...)

	case PollMsg:
		cmds := []tea.Cmd{m.fetchState(), pollCmd()}
		if m.view == ViewSelectPlayer {
			cmds = append(cmds, m.refreshPlayersCmd())
		}
		return m, tea.Batch(cmds...)

	case stateMsg:
		m.fetching = false
		var follow tea.Cmd
		if m.fetchPending {
			m.fetchPending = false
			follow = m.fetchState()
		}
		if m.player == nil || msg.bus != m.player.BusName() {
			return m, follow
		}
		if msg.err != nil {
			return m, m.playerLost()
		}
		return m, tea.Batch(m.applyState(msg.state), follow)

	case actionMsg:
		var cmds []tea.Cmd
		if msg.err != nil {
			cmds = append(cmds, m.setToast(msg.desc+" failed: "+shortErr(msg.err), true))
		}
		if m.player != nil && msg.bus == m.player.BusName() {
			cmds = append(cmds, m.fetchState())
		}
		return m, tea.Batch(cmds...)

	case playersRefreshedMsg:
		return m, m.handlePlayers(msg)

	case ThemeUpdateMsg:
		if msg.ArtURL == m.themeArt {
			m.applyTheme(msg.Colors)
		}
		return m, nil

	case LyricsMsg:
		if msg.SongID != m.lyrics.songID {
			return m, nil
		}
		m.lyrics.loading = false
		m.lyrics.err = msg.Err
		m.lyrics.lines = msg.Lines
		m.lyrics.synced = msg.Synced
		m.lyrics.source = msg.Source
		m.lyrics.renderedActive = -2
		m.lyricsDirty = true
		m.viewport.GotoTop()
		m.refreshLyrics()
		return m, nil

	case toastExpiredMsg:
		if msg.id == m.toastID {
			m.toast = ""
		}
		return m, nil

	case TickMsg:
		if m.hasState {
			m.pos = m.state.PositionAt(time.Time(msg))
		}
		m.animateViz()
		if m.showLyrics {
			m.refreshLyrics()
		}
		return m, tickCmd()
	}
	return m, nil
}

// shortErr returns the last, most specific part of a D-Bus error message.
func shortErr(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 && i+2 < len(s) {
		s = s[i+2:]
	}
	if len(s) > 60 {
		s = s[:59] + "…"
	}
	return s
}

// --- View ---

// layout returns the outer box width (excluding border) and the usable content width.
func (m *Model) layout() (box, inner int) {
	box = 62
	if m.width > 0 && m.width-2 < box {
		box = m.width - 2
	}
	if box < 34 {
		box = 34
	}
	return box, box - 4
}

// View renders the TUI screen.
func (m *Model) View() string {
	var content string
	switch m.view {
	case ViewSelectPlayer:
		content = m.viewSelectPlayer()
	default:
		content = m.viewDashboard()
	}

	if m.width <= 0 || m.height <= 0 {
		return content
	}

	w, h := lipgloss.Width(content), lipgloss.Height(content)
	hAlign, vAlign := lipgloss.Center, lipgloss.Center
	m.boxX, m.boxY = (m.width-w)/2, (m.height-h)/2
	if w > m.width {
		hAlign, m.boxX = lipgloss.Left, 0
	}
	if h > m.height {
		vAlign, m.boxY = lipgloss.Top, 0
		lines := strings.Split(content, "\n")
		content = strings.Join(lines[:m.height], "\n")
	}
	return lipgloss.Place(m.width, m.height, hAlign, vAlign, content)
}

// resize updates size-dependent components.
func (m *Model) resize() {
	_, inner := m.layout()
	m.viewport.Width = inner
	// Chrome: border 2 + padding 2 + header 3 + lyric header 3 + gap/toast/footer 3 + margin 1.
	vh := m.height - 14
	if m.compact() {
		vh += 3 // no vertical padding, no gap before the footer
	}
	if vh < 4 {
		vh = 4
	}
	m.viewport.Height = vh
	m.lyricsDirty = true
	if m.showLyrics {
		m.refreshLyrics()
	}
}
