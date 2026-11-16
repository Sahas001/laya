package ui

import (
	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// Base colours (Catppuccin Mocha).
const (
	ColorBg       = "#1E1E2E"
	ColorBase     = "#313244"
	ColorSurface  = "#45475A"
	ColorLavender = "#B4BEFE"
	ColorMauve    = "#CBA6F7"
	ColorBlue     = "#89B4FA"
	ColorSapphire = "#74C7EC"
	ColorGreen    = "#A6E3A1"
	ColorYellow   = "#F9E2AF"
	ColorPeach    = "#FAB387"
	ColorRed      = "#F38BA8"
	ColorOverlay  = "#6C7086"
	ColorText     = "#CDD6F4"
	ColorSubtext  = "#A6ADC8"
)

// Palette holds the accent colours every themed style is derived from.
type Palette struct {
	Primary   string
	Secondary string
	Tertiary  string
}

// DefaultPalette is used when no album art is available.
var DefaultPalette = Palette{Primary: ColorMauve, Secondary: ColorSapphire, Tertiary: ColorBlue}

// Styles holds all the lipgloss styles used by the UI.
type Styles struct {
	Palette Palette

	Header       lipgloss.Style
	PlayerName   lipgloss.Style
	DotPlaying   lipgloss.Style
	DotPaused    lipgloss.Style
	DotStopped   lipgloss.Style
	IndicatorOn  lipgloss.Style
	IndicatorOff lipgloss.Style

	SongTitle lipgloss.Style
	Artist    lipgloss.Style
	Album     lipgloss.Style
	TrackCard lipgloss.Style

	ProgressTime  lipgloss.Style
	LiveBadge     lipgloss.Style
	StatusPlaying lipgloss.Style
	StatusPaused  lipgloss.Style
	StatusStopped lipgloss.Style
	VolumeLabel   lipgloss.Style
	VolumeValue   lipgloss.Style
	VolumeTrack   lipgloss.Style

	Container lipgloss.Style
	HelpText  lipgloss.Style
	HelpKey   lipgloss.Style
	HelpDesc  lipgloss.Style
	HelpTitle lipgloss.Style
	Toast     lipgloss.Style
	ToastErr  lipgloss.Style

	SelectTitle    lipgloss.Style
	SelectActive   lipgloss.Style
	SelectInactive lipgloss.Style
	SelectMeta     lipgloss.Style
	SelectCurrent  lipgloss.Style

	LyricsTitle   lipgloss.Style
	LyricsMeta    lipgloss.Style
	LyricsError   lipgloss.Style
	LyricsLoading lipgloss.Style
	ActiveLyric   lipgloss.Style
	KaraokeRest   lipgloss.Style
	PastLyric     lipgloss.Style
	UpcomingLyric lipgloss.Style
	PlainLyric    lipgloss.Style

	VizTop    lipgloss.Style
	VizMid    lipgloss.Style
	VizBot    lipgloss.Style
	VizBorder lipgloss.Style
}

// blend mixes hex colour a towards b by t (0 = a, 1 = b), in Lab space.
func blend(a, b string, t float64) string {
	ca, err1 := colorful.Hex(a)
	cb, err2 := colorful.Hex(b)
	if err1 != nil || err2 != nil {
		return a
	}
	return ca.BlendLab(cb, t).Clamped().Hex()
}

// DefaultStyles returns the styles for the default palette.
func DefaultStyles() Styles { return NewStyles(DefaultPalette) }

// NewStyles builds the full style set from an accent palette.
func NewStyles(p Palette) Styles {
	c := func(s string) lipgloss.Color { return lipgloss.Color(s) }
	border := blend(p.Primary, ColorBg, 0.62)

	s := Styles{Palette: p}

	s.Header = lipgloss.NewStyle().Bold(true).
		Foreground(c(ColorBg)).Background(c(p.Primary)).Padding(0, 1)
	s.PlayerName = lipgloss.NewStyle().Foreground(c(p.Secondary)).Italic(true)
	s.DotPlaying = lipgloss.NewStyle().Foreground(c(ColorGreen))
	s.DotPaused = lipgloss.NewStyle().Foreground(c(ColorYellow))
	s.DotStopped = lipgloss.NewStyle().Foreground(c(ColorRed))
	s.IndicatorOn = lipgloss.NewStyle().Foreground(c(p.Primary)).Bold(true)
	s.IndicatorOff = lipgloss.NewStyle().Foreground(c(ColorSurface))

	s.SongTitle = lipgloss.NewStyle().Bold(true).Foreground(c(p.Primary))
	s.Artist = lipgloss.NewStyle().Foreground(c(p.Secondary))
	s.Album = lipgloss.NewStyle().Foreground(c(ColorOverlay)).Italic(true)
	s.TrackCard = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(c(p.Secondary)).
		PaddingLeft(2)

	s.ProgressTime = lipgloss.NewStyle().Foreground(c(ColorSubtext))
	s.LiveBadge = lipgloss.NewStyle().Bold(true).Foreground(c(ColorRed))

	badge := lipgloss.NewStyle().Bold(true).Foreground(c(ColorBg)).Padding(0, 1)
	s.StatusPlaying = badge.Background(c(ColorGreen))
	s.StatusPaused = badge.Background(c(ColorYellow))
	s.StatusStopped = badge.Background(c(ColorRed))

	s.VolumeLabel = lipgloss.NewStyle().Foreground(c(p.Tertiary))
	s.VolumeValue = lipgloss.NewStyle().Foreground(c(p.Secondary))
	s.VolumeTrack = lipgloss.NewStyle().Foreground(c(ColorSurface))

	s.Container = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c(border)).
		Padding(1, 2)
	s.HelpText = lipgloss.NewStyle().Foreground(c(ColorOverlay))
	s.HelpKey = lipgloss.NewStyle().Foreground(c(p.Primary)).Bold(true)
	s.HelpDesc = lipgloss.NewStyle().Foreground(c(ColorSubtext))
	s.HelpTitle = lipgloss.NewStyle().Foreground(c(p.Secondary)).Bold(true)
	s.Toast = lipgloss.NewStyle().Foreground(c(p.Tertiary)).Italic(true)
	s.ToastErr = lipgloss.NewStyle().Foreground(c(ColorRed)).Italic(true)

	s.SelectTitle = lipgloss.NewStyle().Bold(true).Foreground(c(p.Primary))
	s.SelectActive = lipgloss.NewStyle().Bold(true).Foreground(c(ColorText)).
		Background(c(blend(p.Primary, ColorBg, 0.75)))
	s.SelectInactive = lipgloss.NewStyle().Foreground(c(ColorSubtext))
	s.SelectMeta = lipgloss.NewStyle().Foreground(c(ColorOverlay))
	s.SelectCurrent = lipgloss.NewStyle().Foreground(c(p.Secondary))

	s.LyricsTitle = lipgloss.NewStyle().Bold(true).Foreground(c(p.Primary))
	s.LyricsMeta = lipgloss.NewStyle().Foreground(c(ColorOverlay))
	s.LyricsError = lipgloss.NewStyle().Foreground(c(ColorRed)).Italic(true)
	s.LyricsLoading = lipgloss.NewStyle().Foreground(c(p.Tertiary)).Italic(true)
	s.ActiveLyric = lipgloss.NewStyle().Bold(true).Foreground(c(p.Primary))
	s.KaraokeRest = lipgloss.NewStyle().Bold(true).Foreground(c(blend(p.Primary, ColorBg, 0.55)))
	s.PastLyric = lipgloss.NewStyle().Foreground(c(ColorSurface))
	s.UpcomingLyric = lipgloss.NewStyle().Foreground(c(ColorSubtext))
	s.PlainLyric = lipgloss.NewStyle().Foreground(c(ColorText))

	s.VizTop = lipgloss.NewStyle().Foreground(c(p.Primary))
	s.VizMid = lipgloss.NewStyle().Foreground(c(p.Secondary))
	s.VizBot = lipgloss.NewStyle().Foreground(c(p.Tertiary))
	s.VizBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c(border)).
		Padding(0, 1)

	return s
}
