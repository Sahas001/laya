package ui

import (
	"math"
	"strings"
	"time"

	"github.com/Sahas001/laya/internal/audio"
	"github.com/Sahas001/laya/mpris"
	"github.com/charmbracelet/lipgloss"
)

// VizStyle is a visualizer look.
type VizStyle int

const (
	VizBars VizStyle = iota
	VizMirror
	VizScope
	VizBraille
	numVizStyles
)

var vizStyleNames = [...]string{"bars", "mirror", "scope", "braille"}

func (v VizStyle) String() string { return vizStyleNames[v] }

// ParseVizStyle parses a style name; unknown names return VizBars.
func ParseVizStyle(s string) VizStyle {
	for i, n := range vizStyleNames {
		if strings.EqualFold(s, n) {
			return VizStyle(i)
		}
	}
	return VizBars
}

// vizState drives the visualizer from real audio or a synthetic fallback.
type vizState struct {
	style    VizStyle
	capture  *audio.Capture
	audioOn  bool // user wants real audio
	spec     audio.Spectrum
	wave     []float64
	phase    float64
	last     time.Time
	colorKey string
	colors   []string // row colour prefixes, bottom → top
	reset    string
}

var eighths = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// realAudio reports whether the visualizer is fed by captured audio.
func (m *Model) realAudio() bool {
	return m.viz.capture != nil && m.viz.audioOn && m.viz.capture.Running()
}

// vizBandCount returns how many spectrum bands a style needs for width w.
func vizBandCount(style VizStyle, w int) int {
	switch style {
	case VizBraille:
		return w * 2
	case VizScope:
		return 0
	}
	bw, gap := barGeometry(w)
	return (w + gap) / (bw + gap)
}

// barGeometry picks bar and gap widths for a given area width.
func barGeometry(w int) (barW, gap int) {
	switch {
	case w >= 120:
		return 2, 1
	case w >= 24:
		return 1, 1
	}
	return 1, 0
}

// updateViz advances the visualizer by one frame.
func (m *Model) updateViz(now time.Time) {
	dt := 0.04
	if !m.viz.last.IsZero() {
		dt = math.Max(0.01, math.Min(0.2, now.Sub(m.viz.last).Seconds()))
	}
	m.viz.last = now

	w, h := m.vizArea()
	n := vizBandCount(m.viz.style, w)
	if h <= 0 || w <= 0 {
		n = 0
	}
	if n == 0 && m.viz.style != VizScope {
		n = 16
	}
	m.viz.spec.Resize(n)
	playing := m.isPlaying()

	if m.realAudio() {
		f := m.viz.capture.Analyze(m.viz.wave)
		m.viz.wave = f.Wave
		m.viz.spec.Update(f, dt)
		return
	}

	// Synthetic fallback: smooth pseudo-random motion while playing.
	if playing {
		m.viz.phase += dt * 5
	}
	targets := make([]float64, n)
	if playing {
		p := m.viz.phase
		for i := range targets {
			x := float64(i) / math.Max(1, float64(n-1))
			v := 0.5 + 0.22*math.Sin(p+x*5.5) + 0.16*math.Sin(p*1.73+x*13) + 0.1*math.Sin(p*3.1+x*29)
			targets[i] = v * (1 - 0.35*x)
		}
	}
	m.viz.spec.UpdateTargets(targets, dt)

	if len(m.viz.wave) != audio.WaveSize {
		m.viz.wave = make([]float64, audio.WaveSize)
	}
	for i := range m.viz.wave {
		v := 0.0
		if playing {
			t := float64(i) / 60
			v = 0.45*math.Sin(t+m.viz.phase*2) + 0.2*math.Sin(t*2.7+m.viz.phase*3) + 0.08*math.Sin(t*9)
		}
		m.viz.wave[i] = m.viz.wave[i]*0.6 + v*0.4
	}
}

// vizActive reports whether the visualizer still needs fast frames.
func (m *Model) vizActive() bool {
	playing := m.isPlaying()
	return playing || m.fullscreen || !m.viz.spec.Settled()
}

// rowColors returns ANSI colour prefixes for h rows (bottom → top) blending
// tertiary → secondary → primary.
func (m *Model) rowColors(h int) ([]string, string) {
	p := m.styles.Palette
	key := p.Primary + p.Secondary + p.Tertiary + string(rune(h))
	if key == m.viz.colorKey {
		return m.viz.colors, m.viz.reset
	}
	cols := make([]string, h)
	reset := ""
	for r := 0; r < h; r++ {
		t := 0.0
		if h > 1 {
			t = float64(r) / float64(h-1)
		}
		var hex string
		if t < 0.5 {
			hex = blend(p.Tertiary, p.Secondary, t*2)
		} else {
			hex = blend(p.Secondary, p.Primary, (t-0.5)*2)
		}
		out := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("x")
		i := strings.Index(out, "x")
		cols[r] = out[:i]
		reset = out[i+1:]
	}
	m.viz.colorKey, m.viz.colors, m.viz.reset = key, cols, reset
	return cols, reset
}

// renderViz draws the current style into exactly w×h cells.
func (m *Model) renderViz(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	var rows []string // top → bottom, uncoloured
	switch m.viz.style {
	case VizMirror:
		rows = renderMirror(m.viz.spec.Values, w, h)
	case VizScope:
		rows = renderScope(m.viz.wave, w, h)
	case VizBraille:
		rows = renderBrailleBars(m.viz.spec.Values, w, h)
	default:
		rows = renderBars(m.viz.spec.Values, m.viz.spec.Peaks, w, h)
	}
	colors, reset := m.rowColors(h)
	var sb strings.Builder
	for i, row := range rows {
		if i > 0 {
			sb.WriteByte('\n')
		}
		c := colors[h-1-i]
		if m.viz.style == VizScope || m.viz.style == VizMirror {
			// Brightest in the middle, fading towards the edges.
			d := math.Abs(float64(i)-float64(h-1)/2) / math.Max(1, float64(h-1)/2)
			c = colors[int(math.Round((1-d)*float64(h-1)))]
		}
		sb.WriteString(c + row + reset)
	}
	return sb.String()
}

// centerPad returns left/right padding to centre content of width used in w.
func centerPad(w, used int) (string, string) {
	if used >= w {
		return "", ""
	}
	l := (w - used) / 2
	return strings.Repeat(" ", l), strings.Repeat(" ", w-used-l)
}

// renderBars draws vertical bars with 1/8-cell resolution and floating peak caps.
func renderBars(vals, peaks []float64, w, h int) []string {
	bw, gap := barGeometry(w)
	n := len(vals)
	used := n*bw + (n-1)*gap
	if n == 0 {
		used = 0
	}
	lpad, rpad := centerPad(w, used)
	rows := make([]string, h)
	for r := 0; r < h; r++ { // r = 0 is the top row
		level := h - 1 - r // cell index from the bottom
		var sb strings.Builder
		sb.WriteString(lpad)
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", gap))
			}
			fill := vals[i]*float64(h*8) - float64(level*8)
			cell := " "
			switch {
			case fill >= 8:
				cell = "█"
			case fill >= 1:
				cell = eighths[int(fill)]
			case i < len(peaks) && peaks[i] > 0.02 && int(peaks[i]*float64(h)-1e-9) == level &&
				level > int(math.Ceil(vals[i]*float64(h)))-1:
				// Peak cap floats in an empty cell above the bar.
				cell = "▔"
			}
			sb.WriteString(strings.Repeat(cell, bw))
		}
		sb.WriteString(rpad)
		rows[r] = sb.String()
	}
	return rows
}

// renderMirror draws bars growing up and down from the centre line using half blocks.
func renderMirror(vals []float64, w, h int) []string {
	bw, gap := barGeometry(w)
	n := len(vals)
	used := 0
	if n > 0 {
		used = n*bw + (n-1)*gap
	}
	lpad, rpad := centerPad(w, used)
	top := (h + 1) / 2
	bot := h - top
	rows := make([]string, h)
	for r := 0; r < h; r++ {
		var sb strings.Builder
		sb.WriteString(lpad)
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", gap))
			}
			cell := " "
			if r < top {
				halves := int(math.Round(vals[i] * float64(top*2)))
				k := top - 1 - r // 0 = row nearest the centre
				switch {
				case halves >= 2*(k+1):
					cell = "█"
				case halves == 2*k+1:
					cell = "▄"
				}
			} else {
				halves := int(math.Round(vals[i] * 0.75 * float64(bot*2)))
				k := r - top
				switch {
				case halves >= 2*(k+1):
					cell = "█"
				case halves == 2*k+1:
					cell = "▀"
				}
			}
			sb.WriteString(strings.Repeat(cell, bw))
		}
		sb.WriteString(rpad)
		rows[r] = sb.String()
	}
	return rows
}

// braille dot bits, bottom → top, for the left and right columns of a cell.
var (
	brailleLeft  = [4]rune{0x40, 0x04, 0x02, 0x01}
	brailleRight = [4]rune{0x80, 0x20, 0x10, 0x08}
)

// renderBrailleBars draws two bands per cell with 4 dot rows per cell.
func renderBrailleBars(vals []float64, w, h int) []string {
	rows := make([]string, h)
	for r := 0; r < h; r++ {
		level := h - 1 - r
		var sb strings.Builder
		for c := 0; c < w; c++ {
			var bits rune
			for side, dots := range [2][4]rune{brailleLeft, brailleRight} {
				i := c*2 + side
				if i >= len(vals) {
					continue
				}
				n := int(math.Round(vals[i]*float64(h*4))) - level*4
				for d := 0; d < 4 && d < n; d++ {
					bits |= dots[d]
				}
			}
			if bits == 0 {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(0x2800 + bits)
			}
		}
		rows[r] = sb.String()
	}
	return rows
}

// renderScope plots the waveform as a connected braille line.
func renderScope(wave []float64, w, h int) []string {
	W, H := w*2, h*4
	grid := make([][]bool, H)
	for i := range grid {
		grid[i] = make([]bool, W)
	}
	if len(wave) > 0 {
		// Trigger on a rising zero crossing so the trace holds still.
		start := 0
		for i := 1; i < len(wave)/2; i++ {
			if wave[i-1] < 0 && wave[i] >= 0 {
				start = i
				break
			}
		}
		seg := wave[start:]
		span := len(seg)
		if span > W*8 {
			span = W * 8
		}
		seg = seg[:span]
		// Remove DC / very low frequencies so the trace stays centred.
		mean := 0.0
		for _, v := range seg {
			mean += v
		}
		mean /= float64(len(seg))
		peak := 0.0
		for _, v := range seg {
			peak = math.Max(peak, math.Abs(v-mean))
		}
		gain := 1.0
		if peak > 1e-4 {
			gain = math.Min(8, 0.9/peak)
		}
		prev := -1
		for x := 0; x < W; x++ {
			v := (seg[x*span/W] - mean) * gain
			y := int(math.Round((1 - (v+1)/2) * float64(H-1)))
			y = max(0, min(H-1, y))
			lo, hi := y, y
			if prev >= 0 {
				lo, hi = min(prev, y), max(prev, y)
			}
			for yy := lo; yy <= hi; yy++ {
				grid[yy][x] = true
			}
			prev = y
		}
	}
	rows := make([]string, h)
	for r := 0; r < h; r++ {
		var sb strings.Builder
		for c := 0; c < w; c++ {
			var bits rune
			for d := 0; d < 4; d++ {
				y := r*4 + (3 - d)
				if grid[y][c*2] {
					bits |= brailleLeft[d]
				}
				if grid[y][c*2+1] {
					bits |= brailleRight[d]
				}
			}
			if bits == 0 {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(0x2800 + bits)
			}
		}
		rows[r] = sb.String()
	}
	return rows
}

// isPlaying reports whether the active player is playing.
func (m *Model) isPlaying() bool {
	return m.hasState && m.state.PlaybackStatus == mpris.StatusPlaying
}
