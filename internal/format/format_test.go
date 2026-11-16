package format

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	cases := map[time.Duration]string{
		-time.Second:                          "00:00",
		0:                                     "00:00",
		59*time.Second + 600*time.Millisecond: "01:00",
		83 * time.Second:                      "01:23",
		time.Hour + 2*time.Minute + 3*time.Second: "01:02:03",
	}
	for in, want := range cases {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestBars(t *testing.T) {
	if got := Bar(0.5, 10, "#", "-"); got != "#####-----" {
		t.Errorf("Bar = %q", got)
	}
	if got := Bar(2, 4, "#", "-"); got != "####" {
		t.Errorf("Bar clamp = %q", got)
	}
	if got := FineBar(0.5625, 2, "-"); got != "█▏" {
		t.Errorf("FineBar = %q", got)
	}
	if got := FineBar(0, 3, "-"); got != "---" {
		t.Errorf("FineBar empty = %q", got)
	}
	if got := Truncate("héllo world", 6); got != "héllo…" {
		t.Errorf("Truncate = %q", got)
	}
}
