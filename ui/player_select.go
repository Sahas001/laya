package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PlayerInfo holds both the D-Bus bus name and its human-readable identity
type PlayerInfo struct {
	BusName  string
	Identity string
}

func (m *Model) viewSelectPlayer(styles Styles) string {
	boxWidth := 56
	if m.width > 0 && m.width < boxWidth+6 {
		boxWidth = m.width - 6
		if boxWidth < 26 {
			boxWidth = 26
		}
	}
	contentWidth := boxWidth - 6

	var sb strings.Builder

	// Header (Application Name)
	headerText := styles.Header.Margin(0).Render(" L A Y A ")
	centeredHeader := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(headerText)
	sb.WriteString(centeredHeader)
	sb.WriteString("\n\n")

	titleStr := "⚡ Select Media Player"
	centeredTitle := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Bold(true).Render(titleStr)
	sb.WriteString(styles.SelectTitle.Width(contentWidth).Align(lipgloss.Center).Render(centeredTitle))
	sb.WriteString("\n\n")

	if len(m.players) == 0 {
		errorStr := "No active media players detected."
		centeredError := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(errorStr)
		sb.WriteString(styles.LyricsError.Width(contentWidth).Align(lipgloss.Center).Render(centeredError))
		sb.WriteString("\n\n")
		
		descStr := "Ensure a player (e.g. mpv, Spotify, Firefox, Tauon) is open and playing media, then press 'r' to refresh."
		centeredDesc := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(descStr)
		sb.WriteString(styles.HelpText.Width(contentWidth).Align(lipgloss.Center).Render(centeredDesc))
		sb.WriteString("\n\n")
		
		helpKeys := "r: refresh  •  q: quit"
		wrappedHelp := styles.HelpText.Width(contentWidth).Align(lipgloss.Center).Render(helpKeys)
		sb.WriteString(wrappedHelp)
		
		containerStyle := styles.Container.Width(boxWidth)
		return containerStyle.Render(sb.String())
	}

	for i, player := range m.players {
		name := player.Identity
		if name == "" {
			// Fallback to suffix
			parts := strings.Split(player.BusName, ".")
			name = parts[len(parts)-1]
		}

		var line string
		if i == m.selectedIdx {
			line = styles.SelectActive.Render(name)
		} else {
			line = styles.SelectInactive.Render(name)
		}
		// Center each player name line
		centeredLine := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(line)
		sb.WriteString(centeredLine)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	helpKeys := "↑/↓ or j/k: navigate  •  enter: select  •  r: refresh  •  q: quit"
	if m.player != nil {
		helpKeys += "  •  esc: cancel"
	}
	wrappedHelp := styles.HelpText.Width(contentWidth).Align(lipgloss.Center).Render(helpKeys)
	sb.WriteString(wrappedHelp)

	containerStyle := styles.Container.Width(boxWidth)
	return containerStyle.Render(sb.String())
}
