package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct{ key, desc string }

type helpSection struct {
	title   string
	entries []helpEntry
}

var helpSections = []helpSection{
	{"Playback", []helpEntry{
		{"space", "play / pause"},
		{"n  →", "next track"},
		{"p  ←", "previous track"},
		{"[  ]", "seek ∓5s"},
		{"{  }", "seek ∓30s"},
		{"0–9", "jump to 0–90%"},
		{"click", "seek on bar"},
	}},
	{"Volume & modes", []helpEntry{
		{"+  -", "volume (↑ ↓)"},
		{"m", "mute / unmute"},
		{"z", "shuffle"},
		{"r", "loop mode"},
	}},
	{"Lyrics", []helpEntry{
		{"l", "show / hide"},
		{"j  k", "scroll"},
		{"g  G", "top / bottom"},
		{"f", "follow song"},
		{"<  >", "timing ∓250ms"},
	}},
	{"Players & app", []helpEntry{
		{"H  L", "prev / next player"},
		{"s", "player picker"},
		{"?  h", "toggle help"},
		{"esc", "close panel"},
		{"q", "quit"},
	}},
	{"Visuals & extras", []helpEntry{
		{"v", "visualizer style"},
		{"V", "fullscreen visualizer"},
		{"A", "live audio on / off"},
		{"a", "album art on / off"},
		{"R", "recently played"},
		{"t", "sleep timer"},
		{"y", "copy artist – title"},
	}},
}

func (m *Model) renderHelpSection(sec helpSection, width int) string {
	s := m.styles
	keyW := 7
	lines := []string{s.HelpTitle.Render(sec.title)}
	for _, e := range sec.entries {
		key := s.HelpKey.Width(keyW).Render(e.key)
		lines = append(lines, key+" "+s.HelpDesc.Render(truncate(e.desc, width-keyW-1)))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

// viewHelp renders the key reference, in two columns when there's room.
func (m *Model) viewHelp(inner int) string {
	if inner < 50 {
		var parts []string
		for _, sec := range helpSections {
			parts = append(parts, m.renderHelpSection(sec, inner))
		}
		return strings.Join(parts, "\n\n")
	}

	colW := (inner - 4) / 2
	left := []helpSection{helpSections[0], helpSections[1], helpSections[3]}
	right := []helpSection{helpSections[4], helpSections[2]}
	render := func(secs []helpSection) string {
		var parts []string
		for _, sec := range secs {
			parts = append(parts, m.renderHelpSection(sec, colW))
		}
		return strings.Join(parts, "\n\n")
	}
	cols := lipgloss.JoinHorizontal(lipgloss.Top, render(left), "    ", render(right))
	return lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).Render(cols)
}
