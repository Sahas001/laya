package mpris

import (
	"testing"
	"time"
)

func TestPositionAt(t *testing.T) {
	now := time.Now()
	st := PlayerState{
		PlaybackStatus: StatusPlaying,
		Position:       10 * time.Second,
		Rate:           1.5,
		LastUpdated:    now.Add(-2 * time.Second),
		Metadata:       TrackMetadata{Length: 12 * time.Second},
	}
	if got := st.PositionAt(now); got != 12*time.Second {
		t.Errorf("rate/clamp: got %v, want 12s", got)
	}
	st.Metadata.Length = 0
	if got := st.PositionAt(now); got != 13*time.Second {
		t.Errorf("rate: got %v, want 13s", got)
	}
	st.PlaybackStatus = StatusPaused
	if got := st.PositionAt(now); got != 10*time.Second {
		t.Errorf("paused: got %v, want 10s", got)
	}
}

func TestAsInt64(t *testing.T) {
	for _, v := range []any{int64(5), uint64(5), int32(5), uint32(5), float64(5)} {
		if n, ok := asInt64(v); !ok || n != 5 {
			t.Errorf("asInt64(%T) = %d, %v", v, n, ok)
		}
	}
	if _, ok := asInt64("5"); ok {
		t.Error("string should not convert")
	}
}

func TestShortNameAndPlayerctld(t *testing.T) {
	if got := ShortName("org.mpris.MediaPlayer2.firefox.instance_1_44"); got != "firefox.instance_1_44" {
		t.Errorf("ShortName = %q", got)
	}
	if !IsPlayerctld("org.mpris.MediaPlayer2.playerctld") || IsPlayerctld("org.mpris.MediaPlayer2.mpv") {
		t.Error("IsPlayerctld mismatch")
	}
}
