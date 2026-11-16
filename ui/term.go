package ui

import (
	"encoding/base64"
	"os"
	"strings"
	"sync"
)

// Terminal wraps the program's output so that out-of-band escape sequences
// (kitty images, clipboard) can be written without interleaving with frames
// drawn by Bubble Tea's renderer. Pass it to tea.WithOutput.
type Terminal struct {
	mu sync.Mutex
	f  *os.File

	// Kitty reports whether the terminal supports kitty graphics with
	// Unicode placeholders (kitty, ghostty) and we're not inside tmux.
	Kitty bool
}

// NewTerminal wraps f (normally os.Stdout) and detects terminal capabilities.
func NewTerminal(f *os.File) *Terminal {
	return &Terminal{f: f, Kitty: detectKitty()}
}

func detectKitty() bool {
	if os.Getenv("TMUX") != "" || strings.HasPrefix(os.Getenv("TERM"), "screen") {
		return false
	}
	return os.Getenv("KITTY_WINDOW_ID") != "" ||
		os.Getenv("TERM") == "xterm-kitty" ||
		strings.EqualFold(os.Getenv("TERM_PROGRAM"), "ghostty") ||
		os.Getenv("TERM") == "xterm-ghostty"
}

// Write implements io.Writer for the renderer.
func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.f.Write(p)
}

// Read implements io.Reader (required by term.File; never used for input).
func (t *Terminal) Read(p []byte) (int, error) { return t.f.Read(p) }

// Close implements io.Closer without closing stdout.
func (t *Terminal) Close() error { return nil }

// Fd exposes the descriptor so Bubble Tea can detect the TTY and its size.
func (t *Terminal) Fd() uintptr { return t.f.Fd() }

// WriteRaw writes an escape sequence atomically with respect to frame output.
func (t *Terminal) WriteRaw(s string) {
	if t == nil || s == "" {
		return
	}
	_, _ = t.Write([]byte(s))
}

// CopyOSC52 asks the terminal to put text on the system clipboard.
func (t *Terminal) CopyOSC52(text string) {
	t.WriteRaw("\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a")
}
