package app

import (
	"strings"
	"testing"
	"time"

	"github.com/Sahas001/laya/mpris"
)

func TestBuildOutputNoPlayer(t *testing.T) {
	out := buildOutput(nil, "", time.Now())
	if out.Class != "stopped" || out.Text != "No active player" {
		t.Fatalf("unexpected: %+v", out)
	}
	if got := renderTemplate("{artist} - {title}", out); got != "No active player" {
		t.Errorf("template with no track = %q", got)
	}
}

func TestBuildOutputPlaying(t *testing.T) {
	now := time.Now()
	st := &mpris.PlayerState{
		PlaybackStatus: mpris.StatusPaused,
		Position:       90 * time.Second,
		Rate:           1,
		LastUpdated:    now,
		Metadata:       mpris.TrackMetadata{Title: "Song", Artist: "Band", Length: 180 * time.Second},
	}
	out := buildOutput(st, "mpv", now)
	if out.Percentage != 50 || out.Class != "paused" || out.Text != "⏸ Band - Song" {
		t.Fatalf("unexpected: %+v", out)
	}
	if !strings.Contains(out.Tooltip, "01:30 / 03:00") || strings.Contains(out.Tooltip, "Album:") {
		t.Errorf("tooltip = %q", out.Tooltip)
	}
	got := renderTemplate("{icon} {artist} — {title} [{position}/{length}] {percentage}% {player}", out)
	if got != "⏸ Band — Song [01:30/03:00] 50% mpv" {
		t.Errorf("template = %q", got)
	}
}

func TestBuildOutputWithoutArtist(t *testing.T) {
	now := time.Now()
	st := &mpris.PlayerState{PlaybackStatus: mpris.StatusPlaying, LastUpdated: now,
		Metadata: mpris.TrackMetadata{Title: "Some Video"}}
	out := buildOutput(st, "Firefox", now)
	if out.Text != "Some Video" || strings.Contains(out.Tooltip, "Artist") {
		t.Fatalf("unexpected: %+v", out)
	}
	if got := renderTemplate("{icon} {artist} - {title}", out); got != "▶ Some Video" {
		t.Errorf("template = %q", got)
	}
}
