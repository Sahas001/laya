package notify

import "testing"

func TestMessage(t *testing.T) {
	s, b := Message(Track{Title: "Song", Artist: "Band", Album: "LP", Player: "mpv"})
	if s != "Song" || b != "Band — LP\non mpv" {
		t.Errorf("got %q / %q", s, b)
	}
	s, b = Message(Track{Title: "Video"})
	if s != "Video" || b != "" {
		t.Errorf("got %q / %q", s, b)
	}
}

func TestNilNotifierIsSafe(t *testing.T) {
	var n *Notifier
	if err := n.Notify(Track{Title: "x"}); err != nil {
		t.Fatal(err)
	}
}
