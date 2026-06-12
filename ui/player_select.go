package ui

import (
	"strings"
)

// PlayerInfo holds both the D-Bus bus name and its human-readable identity
type PlayerInfo struct {
	BusName  string
	Identity string
}

func (m *Model) viewSelectPlayer(styles Styles) string {
	var sb strings.Builder

	sb.WriteString(styles.SelectTitle.Render("⚡ Select Media Player to Control"))
	sb.WriteString("\n\n")

	if len(m.players) == 0 {
		sb.WriteString(styles.LyricsError.Render("No active media players detected."))
		sb.WriteString("\n\nEnsure a player (e.g. mpv, Spotify, Firefox, Tauon) is open\nand playing media, then press 'r' to refresh.\n")
		sb.WriteString("\n")
		sb.WriteString(styles.HelpText.Render("r: refresh  •  q: quit"))
		return styles.Container.Render(sb.String())
	}

	for i, player := range m.players {
		name := player.Identity
		if name == "" {
			// Fallback to suffix
			parts := strings.Split(player.BusName, ".")
			name = parts[len(parts)-1]
		}

		if i == m.selectedIdx {
			sb.WriteString(styles.SelectActive.Render(name))
		} else {
			sb.WriteString(styles.SelectInactive.Render(name))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(styles.HelpText.Render("↑/↓ or j/k: navigate  •  enter: select  •  r: refresh  •  q: quit"))
	if m.player != nil {
		sb.WriteString(styles.HelpText.Render("  •  esc: cancel"))
	}

	return styles.Container.Render(sb.String())
}
