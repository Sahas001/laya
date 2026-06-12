package mpris

import (
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
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
	Length      time.Duration // Duration of track
	ArtURL      string
	Album       string
	Artist      string // Artist name(s) joined
	Title       string
	AlbumArtist string // Album Artist name(s) joined
}

// PlayerState holds the state of the player at a point in time.
type PlayerState struct {
	PlaybackStatus string        // "Playing", "Paused", "Stopped"
	Volume         float64       // 0.0 to 1.0
	Position       time.Duration // Current position
	Metadata       TrackMetadata
	CanPlay        bool
	CanPause       bool
	CanGoNext      bool
	CanGoPrevious  bool
	LastUpdated    time.Time
}

// NewPlayer creates a new MPRIS player client.
func NewPlayer(conn *dbus.Conn, busName string) *Player {
	return &Player{
		conn:       conn,
		busName:    busName,
		dbusObject: conn.Object(busName, "/org/mpris/MediaPlayer2"),
	}
}

// ListPlayers scans the D-Bus session bus and returns list of player bus names.
func ListPlayers(conn *dbus.Conn) ([]string, error) {
	obj := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	var names []string
	err := obj.Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return nil, fmt.Errorf("failed to list D-Bus names: %w", err)
	}

	var players []string
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			players = append(players, name)
		}
	}
	return players, nil
}

// GetPlayerIdentity retrieves the human-readable identity of the player.
func GetPlayerIdentity(conn *dbus.Conn, busName string) (string, error) {
	obj := conn.Object(busName, "/org/mpris/MediaPlayer2")
	var identity string
	err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.mpris.MediaPlayer2", "Identity").Store(&identity)
	if err != nil {
		// Fallback to name suffix
		parts := strings.Split(busName, ".")
		return parts[len(parts)-1], nil
	}
	return identity, nil
}

// Play starts playback.
func (p *Player) Play() error {
	return p.dbusObject.Call("org.mpris.MediaPlayer2.Player.Play", 0).Store()
}

// Pause pauses playback.
func (p *Player) Pause() error {
	return p.dbusObject.Call("org.mpris.MediaPlayer2.Player.Pause", 0).Store()
}

// PlayPause toggles playback.
func (p *Player) PlayPause() error {
	return p.dbusObject.Call("org.mpris.MediaPlayer2.Player.PlayPause", 0).Store()
}

// Stop stops playback.
func (p *Player) Stop() error {
	return p.dbusObject.Call("org.mpris.MediaPlayer2.Player.Stop", 0).Store()
}

// Next skips to the next track.
func (p *Player) Next() error {
	return p.dbusObject.Call("org.mpris.MediaPlayer2.Player.Next", 0).Store()
}

// Previous skips to the previous track.
func (p *Player) Previous() error {
	return p.dbusObject.Call("org.mpris.MediaPlayer2.Player.Previous", 0).Store()
}

// SetVolume sets the player volume (0.0 to 1.0).
func (p *Player) SetVolume(volume float64) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	return p.dbusObject.Call("org.freedesktop.DBus.Properties.Set", 0, "org.mpris.MediaPlayer2.Player", "Volume", dbus.MakeVariant(volume)).Store()
}

// GetPosition fetches the current position of the player (useful since Position doesn't emit signals).
func (p *Player) GetPosition() (time.Duration, error) {
	var pos int64
	err := p.dbusObject.Call("org.freedesktop.DBus.Properties.Get", 0, "org.mpris.MediaPlayer2.Player", "Position").Store(&pos)
	if err != nil {
		return 0, err
	}
	return time.Duration(pos) * time.Microsecond, nil
}

// GetState fetches the complete PlayerState.
func (p *Player) GetState() (PlayerState, error) {
	var state PlayerState
	state.LastUpdated = time.Now()

	var props map[string]dbus.Variant
	err := p.dbusObject.Call("org.freedesktop.DBus.Properties.GetAll", 0, "org.mpris.MediaPlayer2.Player").Store(&props)
	if err != nil {
		return state, fmt.Errorf("failed to get properties: %w", err)
	}

	if val, ok := props["PlaybackStatus"]; ok {
		if s, ok := val.Value().(string); ok {
			state.PlaybackStatus = s
		}
	}

	if val, ok := props["Volume"]; ok {
		if v, ok := val.Value().(float64); ok {
			state.Volume = v
		}
	}

	// Fetch position directly, as some players omit it from GetAll or don't keep it updated there
	if pos, err := p.GetPosition(); err == nil {
		state.Position = pos
	} else {
		if val, ok := props["Position"]; ok {
			if posMicro, ok := val.Value().(int64); ok {
				state.Position = time.Duration(posMicro) * time.Microsecond
			}
		}
	}

	if val, ok := props["Metadata"]; ok {
		if metaMap, ok := val.Value().(map[string]dbus.Variant); ok {
			var meta TrackMetadata
			if idVal, ok := metaMap["mpris:trackid"]; ok {
				if id, ok := idVal.Value().(dbus.ObjectPath); ok {
					meta.TrackID = string(id)
				}
			}
			if lenVal, ok := metaMap["mpris:length"]; ok {
				if length, ok := lenVal.Value().(int64); ok {
					meta.Length = time.Duration(length) * time.Microsecond
				}
			}
			if artVal, ok := metaMap["mpris:artUrl"]; ok {
				if art, ok := artVal.Value().(string); ok {
					meta.ArtURL = art
				}
			}
			if albumVal, ok := metaMap["xesam:album"]; ok {
				if album, ok := albumVal.Value().(string); ok {
					meta.Album = album
				}
			}
			if artistVal, ok := metaMap["xesam:artist"]; ok {
				switch a := artistVal.Value().(type) {
				case []string:
					meta.Artist = strings.Join(a, ", ")
				case string:
					meta.Artist = a
				}
			}
			if titleVal, ok := metaMap["xesam:title"]; ok {
				if title, ok := titleVal.Value().(string); ok {
					meta.Title = title
				}
			}
			if albumArtistVal, ok := metaMap["xesam:albumArtist"]; ok {
				switch aa := albumArtistVal.Value().(type) {
				case []string:
					meta.AlbumArtist = strings.Join(aa, ", ")
				case string:
					meta.AlbumArtist = aa
				}
			}
			state.Metadata = meta
		}
	}

	if val, ok := props["CanPlay"]; ok {
		if can, ok := val.Value().(bool); ok {
			state.CanPlay = can
		}
	}
	if val, ok := props["CanPause"]; ok {
		if can, ok := val.Value().(bool); ok {
			state.CanPause = can
		}
	}
	if val, ok := props["CanGoNext"]; ok {
		if can, ok := val.Value().(bool); ok {
			state.CanGoNext = can
		}
	}
	if val, ok := props["CanGoPrevious"]; ok {
		if can, ok := val.Value().(bool); ok {
			state.CanGoPrevious = can
		}
	}

	return state, nil
}
