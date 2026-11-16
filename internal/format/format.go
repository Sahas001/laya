// Package format holds small text helpers shared by the TUI and stream modes.
package format

import (
	"fmt"
	"strings"
	"time"
)

// Duration renders d as MM:SS, or HH:MM:SS for durations of an hour or more.
func Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// Clamp01 limits f to the range [0, 1].
func Clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// Bar renders a fixed-width bar of filled/empty runes for percent in [0, 1].
func Bar(percent float64, width int, filled, empty string) string {
	if width <= 0 {
		return ""
	}
	n := int(Clamp01(percent)*float64(width) + 0.5)
	return strings.Repeat(filled, n) + strings.Repeat(empty, width-n)
}

// fineBlocks are the eighth-width left blocks used for sub-cell precision.
var fineBlocks = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// FineBar renders a bar with 1/8-cell precision. The empty part is filled with empty.
func FineBar(percent float64, width int, empty string) string {
	if width <= 0 {
		return ""
	}
	eighths := int(Clamp01(percent)*float64(width*8) + 0.5)
	full := eighths / 8
	rem := eighths % 8
	var sb strings.Builder
	sb.WriteString(strings.Repeat("█", full))
	used := full
	if rem > 0 && used < width {
		sb.WriteString(fineBlocks[rem])
		used++
	}
	sb.WriteString(strings.Repeat(empty, width-used))
	return sb.String()
}

// Truncate shortens s to at most max display columns, adding an ellipsis.
func Truncate(s string, max int) string {
	r := []rune(s)
	if max <= 0 {
		return ""
	}
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
