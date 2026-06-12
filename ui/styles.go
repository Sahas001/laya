package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
const (
	ColorBg         = "#1E1E2E" // Catppuccin Mocha Crust/Base
	ColorBase       = "#313244"
	ColorLavender   = "#B4BEFE"
	ColorMauve      = "#CBA6F7"
	ColorBlue       = "#89B4FA"
	ColorSapphire   = "#74C7EC"
	ColorGreen      = "#A6E3A1"
	ColorYellow     = "#F9E2AF"
	ColorRed        = "#F38BA8"
	ColorOverlay    = "#6C7086"
	ColorText       = "#CDD6F4"
	ColorSubtext    = "#A6ADC8"
)

// Styles struct holds all the pre-defined lipgloss styles.
type Styles struct {
	Header         lipgloss.Style
	PlayerName     lipgloss.Style
	SongTitle      lipgloss.Style
	Artist         lipgloss.Style
	Album          lipgloss.Style
	ProgressTime   lipgloss.Style
	StatusPlaying  lipgloss.Style
	StatusPaused   lipgloss.Style
	StatusStopped  lipgloss.Style
	VolumeLabel    lipgloss.Style
	VolumeValue    lipgloss.Style
	Container      lipgloss.Style
	Border         lipgloss.Style
	HelpText       lipgloss.Style
	KeyStyle       lipgloss.Style
	SelectTitle    lipgloss.Style
	SelectActive   lipgloss.Style
	SelectInactive lipgloss.Style
	LyricsTitle    lipgloss.Style
	LyricsContent  lipgloss.Style
	LyricsError    lipgloss.Style
	LyricsLoading  lipgloss.Style
	ActiveLyric    lipgloss.Style
	TrackCard      lipgloss.Style
}

// DefaultStyles returns the default set of styles.
func DefaultStyles() Styles {
	s := Styles{}

	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorMauve)).
		Background(lipgloss.Color(ColorBase)).
		Padding(0, 2).
		MarginRight(1)

	s.PlayerName = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSapphire)).
		Italic(true)

	s.SongTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorMauve))

	s.Artist = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText))

	s.Album = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorOverlay)).
		Italic(true)

	s.TrackCard = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(ColorSapphire)).
		PaddingLeft(2).
		MarginLeft(2).
		MarginBottom(1)

	s.ProgressTime = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorOverlay))

	s.StatusPlaying = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorGreen)).
		Padding(0, 1)

	s.StatusPaused = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorYellow)).
		Padding(0, 1)

	s.StatusStopped = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorRed)).
		Padding(0, 1)

	s.VolumeLabel = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBlue))

	s.VolumeValue = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText))

	s.Container = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBase)).
		Padding(1, 2)

	s.HelpText = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorOverlay))

	s.KeyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMauve)).
		Bold(true)

	s.SelectTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorMauve)).
		MarginBottom(1)

	s.SelectActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorGreen)).
		SetString("> ")

	s.SelectInactive = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorOverlay)).
		SetString("  ")

	s.LyricsTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorMauve)).
		Align(lipgloss.Center)

	s.LyricsContent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText))

	s.LyricsError = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorRed)).
		Italic(true)

	s.LyricsLoading = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBlue)).
		Italic(true)

	s.ActiveLyric = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorGreen))

	return s
}
