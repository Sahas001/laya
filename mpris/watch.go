package mpris

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

// EventKind describes what changed on the bus.
type EventKind int

const (
	// EventPlayerChanged means a player's properties changed or it seeked.
	EventPlayerChanged EventKind = iota
	// EventPlayersChanged means a player appeared on or vanished from the bus.
	EventPlayersChanged
)

// Event is a coalesced MPRIS bus notification.
type Event struct {
	Kind EventKind
}

// Watch subscribes to MPRIS change signals on conn and delivers coalesced events
// on the returned channel. Events are dropped (not queued) while the consumer is
// busy, because a single pending event is enough to trigger a refresh.
// It uses its own connection so it never steals signals from other users of conn.
func Watch() (<-chan Event, func(), error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, nil, err
	}

	matches := [][]dbus.MatchOption{
		{
			dbus.WithMatchObjectPath(objectPath),
			dbus.WithMatchInterface(propsIface),
			dbus.WithMatchMember("PropertiesChanged"),
		},
		{
			dbus.WithMatchObjectPath(objectPath),
			dbus.WithMatchInterface(playerIface),
			dbus.WithMatchMember("Seeked"),
		},
		{
			dbus.WithMatchInterface("org.freedesktop.DBus"),
			dbus.WithMatchMember("NameOwnerChanged"),
			dbus.WithMatchArg0Namespace("org.mpris.MediaPlayer2"),
		},
	}
	for _, m := range matches {
		if err := conn.AddMatchSignal(m...); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}

	raw := make(chan *dbus.Signal, 32)
	conn.Signal(raw)
	out := make(chan Event, 1)

	go func() {
		defer close(out)
		for sig := range raw {
			ev := Event{Kind: EventPlayerChanged}
			if strings.HasSuffix(sig.Name, ".NameOwnerChanged") {
				ev.Kind = EventPlayersChanged
			}
			select {
			case out <- ev:
			default:
				// A refresh is already pending; upgrade it if this one is broader.
				if ev.Kind == EventPlayersChanged {
					select {
					case <-out:
					default:
					}
					select {
					case out <- ev:
					default:
					}
				}
			}
		}
	}()

	// Closing the connection terminates the signal handler, which closes raw
	// and in turn ends the goroutine above.
	stop := func() { _ = conn.Close() }
	return out, stop, nil
}
