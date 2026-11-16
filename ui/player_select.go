package ui

import (
	"strings"

	"github.com/Sahas001/laya/mpris"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

// PlayerInfo describes a player on the bus for the picker.
type PlayerInfo struct {
	BusName  string
	Identity string
	Label    string // Identity, disambiguated when several players share it
	Status   string
	Title    string
	Artist   string
}

// loadPlayers lists players together with their current status and track.
func loadPlayers(conn *dbus.Conn) ([]PlayerInfo, error) {
	names, err := mpris.ListPlayers(conn)
	if err != nil {
		return nil, err
	}
	infos := make([]PlayerInfo, 0, len(names))
	counts := map[string]int{}
	for _, name := range names {
		identity, _ := mpris.GetPlayerIdentity(conn, name)
		info := PlayerInfo{BusName: name, Identity: identity, Label: identity}
		if st, err := mpris.NewPlayer(conn, name).GetState(); err == nil {
			info.Status = st.PlaybackStatus
			info.Title = st.Metadata.Title
			info.Artist = st.Metadata.Artist
		}
		counts[identity]++
		infos = append(infos, info)
	}
	for i := range infos {
		if counts[infos[i].Identity] > 1 && !mpris.IsPlayerctld(infos[i].BusName) {
			infos[i].Label = infos[i].Identity + " (" + mpris.ShortName(infos[i].BusName) + ")"
		}
	}
	return infos, nil
}

func (m *Model) statusIcon(status string) string {
	s := m.styles
	switch status {
	case mpris.StatusPlaying:
		return s.DotPlaying.Render("▶")
	case mpris.StatusPaused:
		return s.DotPaused.Render("⏸")
	case mpris.StatusStopped:
		return s.DotStopped.Render("⏹")
	}
	return s.SelectMeta.Render("·")
}

func (m *Model) viewSelectPlayer() string {
	box, inner := m.layout()
	s := m.styles
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)

	var f frame
	m.viewHeader(&f, inner)
	f.blank()
	f.add(center.Render(s.SelectTitle.Render("Choose a media player")))
	f.blank()

	switch {
	case !m.playersLoaded:
		f.add(center.Render(s.LyricsLoading.Render("Looking for players…")))
	case len(m.players) == 0:
		f.add(center.Render(s.LyricsError.Render("No active media players detected.")))
		f.blank()
		f.add(center.Render(s.HelpText.Render(
			"Open a player (mpv, Spotify, Firefox, VLC, …) and start playing something. " +
				"The list refreshes automatically.")))
	default:
		for i, p := range m.players {
			f.add(m.viewPlayerRow(p, i == m.cursor, inner))
		}
	}

	f.blank()
	toast := ""
	if m.toast != "" {
		st := s.Toast
		if m.toastErr {
			st = s.ToastErr
		}
		toast = st.Render(truncate(m.toast, inner))
	}
	f.add(center.Render(toast))

	hints := []hint{{"↑/↓", "move", 2}, {"enter", "select", 0}, {"r", "refresh", 3}}
	if m.player != nil {
		hints = append(hints, hint{"esc", "back", 1})
	}
	hints = append(hints, hint{"q", "quit", 1})
	f.add(center.Render(m.renderHints(hints, inner)))

	return m.container(box).Render(f.String())
}

func (m *Model) viewPlayerRow(p PlayerInfo, selected bool, inner int) string {
	s := m.styles
	cursor := "  "
	nameStyle := s.SelectInactive
	if selected {
		cursor = s.SelectTitle.Render("❯ ")
		nameStyle = s.SelectTitle
	}
	current := ""
	if m.player != nil && p.BusName == m.player.BusName() {
		current = s.SelectCurrent.Render("  (current)")
	}

	label := p.Label
	if label == "" {
		label = mpris.ShortName(p.BusName)
	}
	nameWidth := inner - 4 - lipgloss.Width(current)
	line1 := cursor + m.statusIcon(p.Status) + " " + nameStyle.Render(truncate(label, nameWidth)) + current

	var track []string
	if p.Title != "" {
		track = append(track, p.Title)
	}
	if p.Artist != "" {
		track = append(track, p.Artist)
	}
	detail := "nothing playing"
	if len(track) > 0 {
		detail = strings.Join(track, " · ")
	}
	line2 := "    " + s.SelectMeta.Render(truncate(detail, inner-4))

	row := line1 + "\n" + line2
	if selected {
		return s.SelectActive.Width(inner).Render(row)
	}
	return lipgloss.NewStyle().Width(inner).Render(row)
}
