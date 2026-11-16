// Package notify sends "now playing" desktop notifications over D-Bus.
package notify

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// Track describes what to announce.
type Track struct {
	Title, Artist, Album, Player, ArtURL string
}

// Notifier shows one notification that is replaced on every track change.
type Notifier struct {
	conn *dbus.Conn
	mu   sync.Mutex
	id   uint32
}

// New returns a notifier using an existing session-bus connection.
func New(conn *dbus.Conn) *Notifier { return &Notifier{conn: conn} }

// Message builds the summary and body shown for a track.
func Message(t Track) (summary, body string) {
	summary = t.Title
	var parts []string
	if t.Artist != "" {
		parts = append(parts, t.Artist)
	}
	if t.Album != "" {
		parts = append(parts, t.Album)
	}
	body = strings.Join(parts, " — ")
	if t.Player != "" {
		if body != "" {
			body += "\n"
		}
		body += "on " + t.Player
	}
	return summary, body
}

// Notify shows (or replaces) the notification for t.
func (n *Notifier) Notify(t Track) error {
	if n == nil || n.conn == nil || t.Title == "" {
		return nil
	}
	summary, body := Message(t)
	icon := "audio-x-generic"
	hints := map[string]dbus.Variant{
		"category":       dbus.MakeVariant("x-laya.track"),
		"desktop-entry":  dbus.MakeVariant("laya"),
		"suppress-sound": dbus.MakeVariant(true),
	}
	if u, err := url.Parse(t.ArtURL); err == nil && u.Scheme == "file" && u.Path != "" {
		icon = u.Path
		hints["image-path"] = dbus.MakeVariant(u.Path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	n.mu.Lock()
	defer n.mu.Unlock()
	obj := n.conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	var id uint32
	err := obj.CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0,
		"Laya", n.id, icon, summary, body, []string{}, hints, int32(5000)).Store(&id)
	if err == nil {
		n.id = id
	}
	return err
}
