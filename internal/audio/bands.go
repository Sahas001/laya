package audio

import "math"

const (
	minFreq = 40.0
	maxFreq = 16000.0
	dbRange = 42.0 // dynamic range shown, in dB
)

// Spectrum turns FFT frames into smoothed, auto-gained bar heights in [0, 1].
// It keeps per-bar state (falloff, peaks), so use one Spectrum per visualizer.
type Spectrum struct {
	Values []float64 // current bar heights
	Peaks  []float64 // slowly falling peak markers

	ceiling  float64 // adaptive loudest level in dB
	primed   bool
	peakHold []int
}

// Resize sets the number of bars, keeping state when the count is unchanged.
func (s *Spectrum) Resize(n int) {
	if n == len(s.Values) {
		return
	}
	s.Values = make([]float64, n)
	s.Peaks = make([]float64, n)
	s.peakHold = make([]int, n)
}

// BandEdges returns n+1 log-spaced frequency edges between 40 Hz and 16 kHz.
func BandEdges(n int) []float64 {
	edges := make([]float64, n+1)
	ratio := maxFreq / minFreq
	for i := range edges {
		edges[i] = minFreq * math.Pow(ratio, float64(i)/float64(n))
	}
	return edges
}

// bandLevels reduces an amplitude spectrum to n log-spaced bands, in dB, with a
// +3 dB/octave tilt so typical music (which falls off with frequency) looks even.
func bandLevels(mags []float64, binHz float64, n int, out []float64) []float64 {
	if cap(out) < n {
		out = make([]float64, n)
	}
	out = out[:n]
	edges := BandEdges(n)
	last := len(mags) - 1
	for i := 0; i < n; i++ {
		lo, hi := edges[i]/binHz, edges[i+1]/binHz
		var amp float64
		if int(hi) <= int(lo)+1 {
			// Narrower than a bin: interpolate at the band centre.
			c := (lo + hi) / 2
			k := int(c)
			if k >= last {
				k = last - 1
			}
			f := c - float64(k)
			amp = mags[k]*(1-f) + mags[k+1]*f
		} else {
			for k := int(math.Ceil(lo)); k <= int(hi) && k <= last; k++ {
				if mags[k] > amp {
					amp = mags[k]
				}
			}
		}
		centre := math.Sqrt(edges[i] * edges[i+1])
		tilt := 3 * math.Log2(centre/200)
		out[i] = 20*math.Log10(amp+1e-9) + tilt
	}
	return out
}

// Update feeds one frame. When active is false (silence / no capture) bars fall to rest.
func (s *Spectrum) Update(f Frame, dt float64) {
	n := len(s.Values)
	if n == 0 {
		return
	}
	var levels []float64
	if f.Active && len(f.Mags) > 1 {
		levels = bandLevels(f.Mags, f.BinHz, n, nil)
		loudest := -200.0
		for _, l := range levels {
			loudest = math.Max(loudest, l)
		}
		// Adaptive gain: jump up quickly, relax slowly, never amplify near-silence.
		switch {
		case !s.primed:
			s.ceiling, s.primed = loudest, true
		case loudest > s.ceiling:
			s.ceiling += (loudest - s.ceiling) * 0.5
		default:
			s.ceiling -= 10 * dt
		}
		if s.ceiling < -42 {
			s.ceiling = -42
		}
	}
	s.applyTargets(func(i int) float64 {
		if levels == nil {
			return 0
		}
		return (levels[i] - (s.ceiling - dbRange)) / dbRange
	}, dt)
}

// UpdateTargets feeds pre-computed targets in [0, 1] (used for the synthetic fallback).
func (s *Spectrum) UpdateTargets(targets []float64, dt float64) {
	s.applyTargets(func(i int) float64 {
		if i < len(targets) {
			return targets[i]
		}
		return 0
	}, dt)
}

func (s *Spectrum) applyTargets(target func(int) float64, dt float64) {
	fall := 2.2 * dt // full height falls in ~0.45 s
	for i := range s.Values {
		t := math.Max(0, math.Min(1, target(i)))
		v := s.Values[i]
		if t > v {
			v += (t - v) * 0.7
		} else {
			v = math.Max(t, v-fall*(0.35+v))
		}
		s.Values[i] = v

		if v >= s.Peaks[i] {
			s.Peaks[i] = v
			s.peakHold[i] = 8
		} else if s.peakHold[i] > 0 {
			s.peakHold[i]--
		} else {
			s.Peaks[i] = math.Max(v, s.Peaks[i]-0.9*dt)
		}
	}
}

// Settled reports whether every bar is at rest.
func (s *Spectrum) Settled() bool {
	for i := range s.Values {
		if s.Values[i] > 0.005 || s.Peaks[i] > 0.005 {
			return false
		}
	}
	return true
}
