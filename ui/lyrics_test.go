package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestParseSyncedLrcExtended(t *testing.T) {
	input := "[ar:Someone]\n[ti:Song]\n[offset:+500]\n[00:01:50]Colon centiseconds\n[00:03.250]<00:03.25>Word <00:03.80>timed\n[00:05.00]"
	got := ParseSyncedLrc(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 lines, got %d: %+v", len(got), got)
	}
	// offset +500ms shifts lines earlier
	if got[0].Time != 1*time.Second || got[0].Text != "Colon centiseconds" {
		t.Errorf("line 0: %+v", got[0])
	}
	if got[1].Time != 2750*time.Millisecond || got[1].Text != "Word timed" {
		t.Errorf("line 1: %+v", got[1])
	}
	if got[2].Text != "" {
		t.Errorf("line 2 should be an empty (instrumental) line: %+v", got[2])
	}
}

func TestParseTimestampRejectsGarbage(t *testing.T) {
	for _, s := range []string{"ar:Artist", "1:2:3:4", "00:75.00", "-1:00", "xx:10"} {
		if _, err := parseTimestamp(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestParseLyricsFallsBackToPlain(t *testing.T) {
	// A plain .lrc (no timestamps) used to parse to zero lines.
	lines, synced := parseLyrics("Line one\nLine two", "")
	if synced || len(lines) != 2 {
		t.Fatalf("expected 2 plain lines, got synced=%v %+v", synced, lines)
	}
	lines, synced = parseLyrics("[00:01.00]Hi", "ignored")
	if !synced || len(lines) != 1 {
		t.Fatalf("expected synced lyrics, got %v %+v", synced, lines)
	}
}

func TestParsePlainLrcKeepsStanzaBreaks(t *testing.T) {
	got := ParsePlainLrc("\n\nA\nB\n\n\nC\n\n")
	want := []string{"A", "B", "", "C"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Text != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i].Text, want[i])
		}
	}
}

func TestPickBestResult(t *testing.T) {
	results := []LyricResponse{
		{ID: 1, TrackName: "Song", Duration: 300, PlainLyrics: "plain"},
		{ID: 2, TrackName: "Song", Duration: 181, SyncedLyrics: "[00:01.00]x"},
		{ID: 3, TrackName: "Song", Duration: 180, Instrumental: true},
		{ID: 4, TrackName: "Song (Live)", Duration: 240, SyncedLyrics: "[00:01.00]x"},
	}
	best := pickBestResult(results, "Song", 180*time.Second)
	if best == nil || best.ID != 2 {
		t.Fatalf("expected ID 2, got %+v", best)
	}
	if pickBestResult(nil, "x", 0) != nil {
		t.Error("expected nil for no results")
	}
	if pickBestResult([]LyricResponse{{ID: 9}}, "x", 0) != nil {
		t.Error("expected nil when the only result has no lyrics")
	}
}

func TestSearchVariants(t *testing.T) {
	got := searchVariants("Artist - Topic", "Song Name (Official Music Video) [HD]")
	if len(got) < 2 || got[1] != (searchQuery{"Artist", "Song Name"}) {
		t.Fatalf("unexpected variants: %+v", got)
	}
	got = searchVariants("SomeChannel", "Band - Track (Lyrics)")
	last := got[len(got)-1]
	if last != (searchQuery{"Band", "Track"}) {
		t.Fatalf("expected Artist - Title split, got %+v", got)
	}
	if c := cleanTitle("Song - 2011 Remaster"); c != "Song" {
		t.Errorf("cleanTitle remaster = %q", c)
	}
	if c := cleanTitle("Shadow (Childhood Memories)"); c != "Shadow (Childhood Memories)" {
		t.Errorf("cleanTitle should keep ordinary brackets, got %q", c)
	}
}

func TestFileURLPath(t *testing.T) {
	p, ok := fileURLPath("file:///music/AC+DC/My%20Song.flac")
	if !ok || p != "/music/AC+DC/My Song.flac" {
		t.Fatalf("got %q %v", p, ok)
	}
	if _, ok := fileURLPath("https://example.com/a.mp3"); ok {
		t.Error("http URL should not be a file path")
	}
}

func TestLookupLyricsSidecarAndCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	// Sidecar .lrc next to the track wins.
	track := filepath.Join(dir, "My Song.flac")
	if err := os.WriteFile(filepath.Join(dir, "My Song.lrc"), []byte("[00:01.00]local"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, synced, src, err := lookupLyrics(lyricsQuery{Title: "x", TrackURL: "file://" + strings.ReplaceAll(track, " ", "%20")})
	if err != nil || !synced || src != "local file" || lines[0].Text != "local" {
		t.Fatalf("sidecar: %v %v %q %+v", err, synced, src, lines)
	}

	// LRCLIB result is cached; second lookup doesn't hit the network.
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		switch {
		case strings.Contains(r.URL.RawQuery, "Missing"):
			_, _ = w.Write([]byte("[]"))
		case r.URL.Path == "/api/search":
			_ = json.NewEncoder(w).Encode([]LyricResponse{{TrackName: "Song", Duration: 200, SyncedLyrics: "[00:02.00]remote"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := lrclibBase
	lrclibBase = srv.URL
	defer func() { lrclibBase = old }()

	q := lyricsQuery{Artist: "Band", Title: "Song", Album: "LP", Duration: 200 * time.Second}
	for i, wantSrc := range []string{"LRCLIB", "cache"} {
		lines, synced, src, err := lookupLyrics(q)
		if err != nil || !synced || src != wantSrc || lines[0].Text != "remote" {
			t.Fatalf("lookup %d: %v %v %q %+v", i, err, synced, src, lines)
		}
	}
	if hits != 2 { // get-cached 404 + search, then nothing
		t.Errorf("expected 2 HTTP hits, got %d", hits)
	}

	// Misses are cached too.
	miss := lyricsQuery{Artist: "Band", Title: "Missing"}
	for i := 0; i < 2; i++ {
		if _, _, _, err := lookupLyrics(miss); err != ErrNoLyrics {
			t.Fatalf("miss %d: expected ErrNoLyrics, got %v", i, err)
		}
	}
	if hits != 3 {
		t.Errorf("expected miss to be cached, got %d hits", hits)
	}
}
