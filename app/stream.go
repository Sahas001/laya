package app

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Sahas001/laya/internal/format"
	"github.com/Sahas001/laya/mpris"
	"github.com/godbus/dbus/v5"
)

// WaybarOutput represents the structured JSON output for Waybar.
type WaybarOutput struct {
	Artist           string `json:"artist"`
	Title            string `json:"title"`
	Album            string `json:"album"`
	Player           string `json:"player"`
	Status           string `json:"status"`
	Position         string `json:"position"`
	Length           string `json:"length"`
	ProgressBarChars string `json:"progress_bar_chars"`
	Text             string `json:"text"`
	Alt              string `json:"alt"`
	Tooltip          string `json:"tooltip"`
	Class            string `json:"class"`
	Percentage       int    `json:"percentage"`
}

// StreamOptions configures the headless stream mode.
type StreamOptions struct {
	PlayerSearch string
	Format       string // when non-empty, emit plain text from this template instead of JSON
	Interval     time.Duration
}

func statusIcon(status string) string {
	switch status {
	case mpris.StatusPlaying:
		return "▶"
	case mpris.StatusPaused:
		return "⏸"
	default:
		return "⏹"
	}
}

// buildOutput converts a player state into the status-bar payload.
func buildOutput(state *mpris.PlayerState, identity string, now time.Time) WaybarOutput {
	if state == nil {
		return WaybarOutput{
			Status: mpris.StatusStopped,
			Text:   "No active player",
			Alt:    "stopped",
			Class:  "stopped",
		}
	}

	pos := state.PositionAt(now)
	length := state.Metadata.Length

	var percent float64
	if length > 0 {
		percent = format.Clamp01(float64(pos) / float64(length))
	}
	pct := int(percent * 100)
	bar := format.Bar(percent, 10, "█", "░")

	title := state.Metadata.Title
	if title == "" {
		title = "Unknown Track"
	}
	// Browsers and video sites often report no artist; show just the title then.
	artist := state.Metadata.Artist
	text := title
	if artist != "" {
		text = artist + " - " + title
	}
	if state.PlaybackStatus != mpris.StatusPlaying {
		text = statusIcon(state.PlaybackStatus) + " " + text
	}

	var tt strings.Builder
	fmt.Fprintf(&tt, "Title: %s", title)
	if artist != "" {
		fmt.Fprintf(&tt, "\nArtist: %s", artist)
	}
	if state.Metadata.Album != "" {
		fmt.Fprintf(&tt, "\nAlbum: %s", state.Metadata.Album)
	}
	fmt.Fprintf(&tt, "\nStatus: %s\nPlayer: %s", state.PlaybackStatus, identity)
	posStr, lenStr := format.Duration(pos), "live"
	if length > 0 {
		lenStr = format.Duration(length)
		fmt.Fprintf(&tt, "\nProgress: %s / %s (%d%%)\n%s", posStr, lenStr, pct, bar)
	}

	status := strings.ToLower(state.PlaybackStatus)
	return WaybarOutput{
		Artist:           artist,
		Title:            title,
		Album:            state.Metadata.Album,
		Player:           identity,
		Status:           state.PlaybackStatus,
		Position:         posStr,
		Length:           lenStr,
		ProgressBarChars: bar,
		Text:             text,
		Alt:              status,
		Tooltip:          tt.String(),
		Class:            status,
		Percentage:       pct,
	}
}

// renderTemplate expands {placeholders} in tmpl using values from out.
func renderTemplate(tmpl string, out WaybarOutput) string {
	if out.Title == "" && out.Artist == "" {
		return out.Text
	}
	if out.Artist == "" {
		// Drop the separator that would dangle in "{artist} - {title}".
		for _, sep := range []string{" - ", " — ", " – ", " · "} {
			tmpl = strings.ReplaceAll(tmpl, "{artist}"+sep, "")
			tmpl = strings.ReplaceAll(tmpl, sep+"{artist}", "")
		}
	}
	r := strings.NewReplacer(
		"{artist}", out.Artist,
		"{title}", out.Title,
		"{album}", out.Album,
		"{player}", out.Player,
		"{status}", out.Status,
		"{icon}", statusIcon(out.Status),
		"{position}", out.Position,
		"{length}", out.Length,
		"{percentage}", strconv.Itoa(out.Percentage),
		"{bar}", out.ProgressBarChars,
		"{text}", out.Text,
	)
	return strings.TrimSpace(r.Replace(tmpl))
}

type streamTarget struct {
	player   *mpris.Player
	identity string
}

// pickStreamPlayer chooses which player to report. With a search term it uses the
// first matching player that responds. Otherwise it prefers a playing player,
// then sticks with the current one, then falls back to the first responsive one.
// playerctld is used only when nothing else is available.
func pickStreamPlayer(conn *dbus.Conn, search string, current *streamTarget) (*streamTarget, *mpris.PlayerState) {
	names, err := mpris.ListPlayers(conn)
	if err != nil || len(names) == 0 {
		return nil, nil
	}

	search = strings.ToLower(strings.TrimSpace(search))
	type cand struct {
		t     *streamTarget
		state mpris.PlayerState
	}
	var cands []cand
	for _, name := range names {
		identity, _ := mpris.GetPlayerIdentity(conn, name)
		if search != "" && !strings.Contains(strings.ToLower(identity), search) &&
			!strings.Contains(strings.ToLower(name), search) {
			continue
		}
		p := mpris.NewPlayer(conn, name)
		if current != nil && current.player.BusName() == name {
			p = current.player
		}
		st, err := p.GetState()
		if err != nil {
			continue
		}
		cands = append(cands, cand{&streamTarget{p, identity}, st})
	}
	if len(cands) == 0 {
		return nil, nil
	}

	score := func(c cand) int {
		s := 0
		if c.state.PlaybackStatus == mpris.StatusPlaying {
			s += 4
		}
		if current != nil && c.t.player.BusName() == current.player.BusName() {
			s += 2
		}
		if !mpris.IsPlayerctld(c.t.player.BusName()) {
			s++
		}
		return s
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if score(c) > score(best) {
			best = c
		}
	}
	return best.t, &best.state
}

func runStream(conn *dbus.Conn, opts StreamOptions, w io.Writer) {
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	var events <-chan mpris.Event
	if ev, stop, err := mpris.Watch(); err == nil {
		events = ev
		defer stop()
	}

	var target *streamTarget
	var last string

	emit := func() {
		var state *mpris.PlayerState
		if target != nil && opts.PlayerSearch == "" {
			// Re-evaluate when the current player isn't playing, so a player that
			// starts playing elsewhere takes over the status bar.
			if st, err := target.player.GetState(); err == nil && st.PlaybackStatus == mpris.StatusPlaying {
				state = &st
			}
		} else if target != nil {
			if st, err := target.player.GetState(); err == nil {
				state = &st
			}
		}
		if state == nil {
			target, state = pickStreamPlayer(conn, opts.PlayerSearch, target)
		}

		identity := ""
		if target != nil {
			identity = target.identity
		}
		out := buildOutput(state, identity, time.Now())

		var line string
		if opts.Format != "" {
			line = renderTemplate(opts.Format, out)
		} else {
			b, err := json.Marshal(out)
			if err != nil {
				return
			}
			line = string(b)
		}
		if line != last {
			fmt.Fprintln(w, line)
			last = line
		}
	}

	emit()
	for {
		select {
		case <-ticker.C:
		case _, ok := <-events:
			if !ok {
				events = nil
				continue
			}
		}
		emit()
	}
}
