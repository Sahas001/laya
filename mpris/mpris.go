// Package mpris is a small client for the MPRIS D-Bus media player interface.
package mpris

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	busPrefix     = "org.mpris.MediaPlayer2."
	objectPath    = "/org/mpris/MediaPlayer2"
	rootIface     = "org.mpris.MediaPlayer2"
	playerIface   = "org.mpris.MediaPlayer2.Player"
	propsIface    = "org.freedesktop.DBus.Properties"
	playerctldBus = "org.mpris.MediaPlayer2.playerctld"
)

// CallTimeout bounds every D-Bus call so a hung player can never freeze the UI.
var CallTimeout = 2 * time.Second

// Playback status values defined by the MPRIS spec.
const (
	StatusPlaying = "Playing"
	StatusPaused  = "Paused"
	StatusStopped = "Stopped"
)

// Loop status values defined by the MPRIS spec.
const (
	LoopNone     = "None"
	LoopTrack    = "Track"
	LoopPlaylist = "Playlist"
)

// Player represents an MPRIS player client.
type Player struct {
	conn       *dbus.Conn
	busName    string
	dbusObject dbus.BusObject
}

// TrackMetadata holds metadata for the current track.
type TrackMetadata struct {
	TrackID     string
	Length      time.Duration // Duration of track (0 when unknown / live)
	ArtURL      string
	Album       string
	Artist      string // Artist name(s) joined
	Title       string
	AlbumArtist string // Album Artist name(s) joined
	URL         string // track URL (xesam:url)
}

// Key returns a string that uniquely identifies the track for change detection.
func (t TrackMetadata) Key() string {
	return strings.Join([]string{t.TrackID, t.URL, t.Artist, t.Title}, "\x1f")
}

// PlayerState holds the state of the player at a point in time.
type PlayerState struct {
	PlaybackStatus string        // "Playing", "Paused", "Stopped"
	Volume         float64       // 0.0 to 1.0 (may exceed 1.0 on some players)
	HasVolume      bool          // player exposes the Volume property
	Position       time.Duration // Position at LastUpdated
	Rate           float64       // Playback rate (1.0 = normal)
	Metadata       TrackMetadata
	Shuffle        bool
	HasShuffle     bool
	LoopStatus     string // "" when unsupported
	CanPlay        bool
	CanPause       bool
	CanGoNext      bool
	CanGoPrevious  bool
	CanSeek        bool
	CanControl     bool
	LastUpdated    time.Time
}

// PositionAt estimates the playback position at time now, accounting for the
// elapsed time since the state was fetched and the playback rate.
func (s PlayerState) PositionAt(now time.Time) time.Duration {
	pos := s.Position
	if s.PlaybackStatus == StatusPlaying && !s.LastUpdated.IsZero() {
		rate := s.Rate
		if rate <= 0 {
			rate = 1
		}
		pos += time.Duration(float64(now.Sub(s.LastUpdated)) * rate)
	}
	if s.Metadata.Length > 0 && pos > s.Metadata.Length {
		pos = s.Metadata.Length
	}
	if pos < 0 {
		pos = 0
	}
	return pos
}

// NewPlayer creates a new MPRIS player client.
func NewPlayer(conn *dbus.Conn, busName string) *Player {
	return &Player{
		conn:       conn,
		busName:    busName,
		dbusObject: conn.Object(busName, objectPath),
	}
}

// BusName returns the player's D-Bus bus name.
func (p *Player) BusName() string {
	return p.busName
}

func (p *Player) call(method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	return p.dbusObject.CallWithContext(ctx, method, 0, args...)
}

// IsPlayerctld reports whether the bus name belongs to the playerctld proxy daemon.
func IsPlayerctld(busName string) bool {
	return busName == playerctldBus
}

// ListPlayers scans the D-Bus session bus and returns the player bus names, sorted.
func ListPlayers(conn *dbus.Conn) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	obj := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	var names []string
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil, fmt.Errorf("failed to list D-Bus names: %w", err)
	}

	var players []string
	for _, name := range names {
		if strings.HasPrefix(name, busPrefix) {
			players = append(players, name)
		}
	}
	sort.Strings(players)
	return players, nil
}

// ShortName returns the bus-name suffix, e.g. "spotify" or "firefox.instance_1_44".
func ShortName(busName string) string {
	return strings.TrimPrefix(busName, busPrefix)
}

// GetPlayerIdentity retrieves the human-readable identity of the player.
// It never fails: on error it falls back to the bus-name suffix.
func GetPlayerIdentity(conn *dbus.Conn, busName string) (string, error) {
	if IsPlayerctld(busName) {
		return "playerctld (follows active player)", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	obj := conn.Object(busName, objectPath)
	var identity string
	err := obj.CallWithContext(ctx, propsIface+".Get", 0, rootIface, "Identity").Store(&identity)
	if err != nil || strings.TrimSpace(identity) == "" {
		parts := strings.Split(busName, ".")
		return parts[len(parts)-1], nil
	}
	return identity, nil
}

// MatchPlayer returns the first bus name whose identity or bus name contains term
// (case-insensitive), together with its identity.
func MatchPlayer(conn *dbus.Conn, names []string, term string) (busName, identity string, ok bool) {
	term = strings.ToLower(strings.TrimSpace(term))
	for _, name := range names {
		id, _ := GetPlayerIdentity(conn, name)
		if strings.Contains(strings.ToLower(id), term) || strings.Contains(strings.ToLower(name), term) {
			return name, id, true
		}
	}
	return "", "", false
}

// Play starts playback.
func (p *Player) Play() error { return p.call(playerIface + ".Play").Store() }

// Pause pauses playback.
func (p *Player) Pause() error { return p.call(playerIface + ".Pause").Store() }

// PlayPause toggles playback.
func (p *Player) PlayPause() error { return p.call(playerIface + ".PlayPause").Store() }

// Stop stops playback.
func (p *Player) Stop() error { return p.call(playerIface + ".Stop").Store() }

// Next skips to the next track.
func (p *Player) Next() error { return p.call(playerIface + ".Next").Store() }

// Previous skips to the previous track.
func (p *Player) Previous() error { return p.call(playerIface + ".Previous").Store() }

// SetVolume sets the player volume (clamped to 0.0 – 1.0).
func (p *Player) SetVolume(volume float64) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	return p.setProp("Volume", volume)
}

// SetShuffle enables or disables shuffle.
func (p *Player) SetShuffle(on bool) error { return p.setProp("Shuffle", on) }

// SetLoopStatus sets the loop mode ("None", "Track" or "Playlist").
func (p *Player) SetLoopStatus(status string) error { return p.setProp("LoopStatus", status) }

func (p *Player) setProp(name string, value any) error {
	return p.call(propsIface+".Set", playerIface, name, dbus.MakeVariant(value)).Store()
}

// SeekBy seeks relative to the current position.
func (p *Player) SeekBy(offset time.Duration) error {
	return p.call(playerIface+".Seek", offset.Microseconds()).Store()
}

// SetPosition seeks to an absolute position in the given track.
func (p *Player) SetPosition(trackID string, pos time.Duration) error {
	if trackID == "" {
		return fmt.Errorf("player did not report a track id; absolute seek unsupported")
	}
	if pos < 0 {
		pos = 0
	}
	return p.call(playerIface+".SetPosition", dbus.ObjectPath(trackID), pos.Microseconds()).Store()
}

// GetPosition fetches the current position of the player (Position doesn't emit change signals).
func (p *Player) GetPosition() (time.Duration, error) {
	var v dbus.Variant
	if err := p.call(propsIface+".Get", playerIface, "Position").Store(&v); err != nil {
		return 0, err
	}
	us, ok := asInt64(v.Value())
	if !ok {
		return 0, fmt.Errorf("unexpected Position type %T", v.Value())
	}
	return time.Duration(us) * time.Microsecond, nil
}

// GetState fetches the complete PlayerState.
func (p *Player) GetState() (PlayerState, error) {
	state := PlayerState{Rate: 1}

	var props map[string]dbus.Variant
	if err := p.call(propsIface+".GetAll", playerIface).Store(&props); err != nil {
		return state, fmt.Errorf("failed to get properties: %w", err)
	}

	state.PlaybackStatus = propString(props, "PlaybackStatus")
	if state.PlaybackStatus == "" {
		state.PlaybackStatus = StatusStopped
	}
	if v, ok := props["Volume"]; ok {
		state.Volume, state.HasVolume = asFloat64(v.Value())
	}
	if v, ok := props["Rate"]; ok {
		if r, ok := asFloat64(v.Value()); ok && r > 0 {
			state.Rate = r
		}
	}
	if v, ok := props["Shuffle"]; ok {
		state.Shuffle, state.HasShuffle = v.Value().(bool)
	}
	state.LoopStatus = propString(props, "LoopStatus")

	// Fetch position directly, as some players omit it from GetAll or don't keep it updated there.
	if pos, err := p.GetPosition(); err == nil {
		state.Position = pos
	} else if v, ok := props["Position"]; ok {
		if us, ok := asInt64(v.Value()); ok {
			state.Position = time.Duration(us) * time.Microsecond
		}
	}
	state.LastUpdated = time.Now()

	if v, ok := props["Metadata"]; ok {
		if meta, ok := v.Value().(map[string]dbus.Variant); ok {
			state.Metadata = parseMetadata(meta)
		}
	}

	state.CanPlay = propBool(props, "CanPlay")
	state.CanPause = propBool(props, "CanPause")
	state.CanGoNext = propBool(props, "CanGoNext")
	state.CanGoPrevious = propBool(props, "CanGoPrevious")
	state.CanSeek = propBool(props, "CanSeek")
	state.CanControl = propBool(props, "CanControl")
	if _, ok := props["CanControl"]; !ok {
		state.CanControl = true
	}

	return state, nil
}

func parseMetadata(m map[string]dbus.Variant) TrackMetadata {
	var meta TrackMetadata
	if v, ok := m["mpris:trackid"]; ok {
		switch id := v.Value().(type) {
		case dbus.ObjectPath:
			meta.TrackID = string(id)
		case string:
			meta.TrackID = id
		}
	}
	if v, ok := m["mpris:length"]; ok {
		if us, ok := asInt64(v.Value()); ok && us > 0 {
			meta.Length = time.Duration(us) * time.Microsecond
		}
	}
	meta.ArtURL = metaString(m, "mpris:artUrl")
	meta.Album = metaString(m, "xesam:album")
	meta.URL = metaString(m, "xesam:url")
	meta.Title = metaString(m, "xesam:title")
	meta.Artist = metaString(m, "xesam:artist")
	meta.AlbumArtist = metaString(m, "xesam:albumArtist")
	return meta
}

func metaString(m map[string]dbus.Variant, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch s := v.Value().(type) {
	case string:
		return strings.TrimSpace(s)
	case []string:
		return strings.TrimSpace(strings.Join(s, ", "))
	}
	return ""
}

func propString(props map[string]dbus.Variant, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

func propBool(props map[string]dbus.Variant, key string) bool {
	if v, ok := props[key]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}

// asInt64 normalises the integer types players use for microsecond values.
func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		return int64(n), true
	case int32:
		return int64(n), true
	case uint32:
		return int64(n), true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func asFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case uint32:
		return float64(n), true
	}
	return 0, false
}
