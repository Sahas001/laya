package ui

import (
	"testing"
	"time"
)

func TestParseSyncedLrc(t *testing.T) {
	input := `[00:12.34]Hello world
[01:23]This is a second line
[invalid]Line with invalid bracket
Just normal line
[02:34.56][02:35.00]Double timestamp line`

	result := ParseSyncedLrc(input)

	// Expected parsed synced lines:
	// 1. "Hello world" at 12.34s
	// 2. "This is a second line" at 1m 23s
	// 3. "Double timestamp line" at 2m 34.56s
	// 4. "Double timestamp line" at 2m 35.00s
	if len(result) != 4 {
		t.Fatalf("Expected 4 parsed lines, got %d", len(result))
	}

	// Line 0
	expectedTime0 := 12*time.Second + 340*time.Millisecond
	if result[0].Time != expectedTime0 || result[0].Text != "Hello world" {
		t.Errorf("Line 0 mismatch. Got time: %v, text: %q", result[0].Time, result[0].Text)
	}

	// Line 1
	expectedTime1 := 1*time.Minute + 23*time.Second
	if result[1].Time != expectedTime1 || result[1].Text != "This is a second line" {
		t.Errorf("Line 1 mismatch. Got time: %v, text: %q", result[1].Time, result[1].Text)
	}

	// Line 2
	expectedTime2 := 2*time.Minute + 34*time.Second + 560*time.Millisecond
	if result[2].Time != expectedTime2 || result[2].Text != "Double timestamp line" {
		t.Errorf("Line 2 mismatch. Got time: %v, text: %q", result[2].Time, result[2].Text)
	}

	// Line 3
	expectedTime3 := 2*time.Minute + 35*time.Second
	if result[3].Time != expectedTime3 || result[3].Text != "Double timestamp line" {
		t.Errorf("Line 3 mismatch. Got time: %v, text: %q", result[3].Time, result[3].Text)
	}
}
