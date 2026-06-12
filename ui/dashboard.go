package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// formatDuration converts a time.Duration to a MM:SS or HH:MM:SS string.
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "00:00"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// renderVolumeBar returns a character-based volume bar.
func renderVolumeBar(vol float64, width int) string {
	filled := int(vol * float64(width))
	if filled > width {
		filled = width
	} else if filled < 0 {
		filled = 0
	}
	empty := width - filled
	return strings.Repeat("■", filled) + strings.Repeat("□", empty)
}

func (m *Model) viewDashboard(styles Styles) string {
	boxWidth := 56
	if m.width > 0 && m.width < boxWidth+6 {
		boxWidth = m.width - 6
		if boxWidth < 26 {
			boxWidth = 26
		}
	}
	contentWidth := boxWidth - 6

	var sb strings.Builder

	// 1. Header (Application Name & Selected Player)
	headerText := styles.Header.Margin(0).Render(" L A Y A ")
	centeredHeader := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(headerText)
	sb.WriteString(centeredHeader)
	sb.WriteString("\n")

	playerText := styles.PlayerName.Render(fmt.Sprintf("● %s", m.activePlayerName))
	centeredPlayer := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(playerText)
	sb.WriteString(centeredPlayer)
	sb.WriteString("\n\n")

	// 2. Main content area (either Track Details or Lyrics)
	if m.showLyrics {
		// --- LYRICS VIEW ---
		title := m.playerState.Metadata.Title
		artist := m.playerState.Metadata.Artist
		if title == "" {
			title = "Unknown Track"
		}
		
		lyricHeader := fmt.Sprintf("󰎆 Lyrics: %s - %s", title, artist)
		lyricHeaderStyle := styles.LyricsTitle.Width(contentWidth).Align(lipgloss.Center)
		sb.WriteString(lyricHeaderStyle.Render(lyricHeader))
		sb.WriteString("\n\n")

		if !m.lyricsLoaded {
			var loadingStr string
			if m.lyricsErr != nil {
				loadingStr = styles.LyricsError.Render(fmt.Sprintf("Error: %v", m.lyricsErr))
			} else {
				loadingStr = styles.LyricsLoading.Render("Loading lyrics from LRCLIB...")
			}
			centeredLoading := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(loadingStr)
			sb.WriteString(centeredLoading)
		} else {
			sb.WriteString(m.viewport.View())
		}
		sb.WriteString("\n\n")
	} else {
		// --- NOW PLAYING DASHBOARD ---
		title := m.playerState.Metadata.Title
		artist := m.playerState.Metadata.Artist
		album := m.playerState.Metadata.Album

		if title == "" {
			title = "No media detected"
			artist = "Start playing something on your media player"
			album = ""
		}

		// Track Info Card (wrapped to fit contentWidth)
		var cardSb strings.Builder
		cardSb.WriteString(styles.SongTitle.Render(title))
		cardSb.WriteString("\n")
		if artist != "" {
			cardSb.WriteString(styles.Artist.Render("  " + artist))
			cardSb.WriteString("\n")
		}
		if album != "" {
			cardSb.WriteString(styles.Album.Render("󰀥  " + album))
			cardSb.WriteString("\n")
		}
		trackCardStyle := styles.TrackCard.Width(contentWidth - 2)
		sb.WriteString(trackCardStyle.Render(cardSb.String()))
		sb.WriteString("\n")

		// 3. Progress Bar Row
		duration := m.playerState.Metadata.Length
		posStr := formatDuration(m.interpolatedPos)
		durStr := "--:--"
		
		var percent float64
		if duration > 0 {
			durStr = formatDuration(duration)
			percent = float64(m.interpolatedPos) / float64(duration)
			if percent > 1.0 {
				percent = 1.0
			} else if percent < 0.0 {
				percent = 0.0
			}
		} else {
			// Live stream or unknown duration
			if m.playerState.PlaybackStatus == "Playing" {
				percent = 0.5 // Static centered progress indicator
				durStr = "Live"
			}
		}

		// Progress bar width adjusted based on contentWidth
		progWidth := contentWidth - len(posStr) - len(durStr) - 4
		if progWidth < 6 {
			progWidth = 6
		}
		m.progressBar.Width = progWidth
		progressBarStr := m.progressBar.ViewAs(percent)

		progRow := fmt.Sprintf("%s  %s  %s", 
			styles.ProgressTime.Render(posStr),
			progressBarStr,
			styles.ProgressTime.Render(durStr),
		)
		centeredProg := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(progRow)
		sb.WriteString(centeredProg)
		sb.WriteString("\n\n")

		// 4. Status Bar & Volume Row
		var statusBadge string
		switch m.playerState.PlaybackStatus {
		case "Playing":
			statusBadge = styles.StatusPlaying.Render(" PLAYING")
		case "Paused":
			statusBadge = styles.StatusPaused.Render("⏸ PAUSED")
		default:
			statusBadge = styles.StatusStopped.Render("⏹ STOPPED")
		}

		volPercent := m.playerState.Volume
		volStr := fmt.Sprintf("󰕾 %3d%% ", int(volPercent*100))
		volBarWidth := 10
		if contentWidth < 40 {
			volBarWidth = 5
		}
		volBar := renderVolumeBar(volPercent, volBarWidth)
		volRow := fmt.Sprintf("%s%s", 
			styles.VolumeLabel.Render(volStr),
			styles.VolumeValue.Render(volBar),
		)

		// Join status and volume side-by-side with padding
		statusBar := lipgloss.JoinHorizontal(lipgloss.Center, statusBadge, "    ", volRow)
		centeredStatus := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(statusBar)
		sb.WriteString(centeredStatus)
		sb.WriteString("\n")
	}

	// 5. Help Hint Footer
	sb.WriteString("\n")
	var helpKeys string
	if m.showLyrics {
		helpKeys = "j/k: scroll  •  space: play/pause  •  [ and ]: seek  •  H/L: cycle players  •  l/esc: close lyrics"
	} else {
		helpKeys = "space: play/pause  •  [ and ]: seek  •  H/L: cycle players  •  l: lyrics  •  s: select player"
	}
	wrappedHelp := styles.HelpText.Width(contentWidth).Align(lipgloss.Center).Render(helpKeys)
	sb.WriteString(wrappedHelp)

	containerStyle := styles.Container.Width(boxWidth)
	return containerStyle.Render(sb.String())
}
