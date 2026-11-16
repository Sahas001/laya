// Package audio captures system audio output and turns it into spectrum and
// waveform data for the visualizer. Everything stays in memory; nothing is
// recorded to disk.
package audio

import (
	"math"
	"math/cmplx"
)

// fft computes an in-place radix-2 FFT. len(x) must be a power of two.
func fft(x []complex128) {
	n := len(x)
	// Bit-reversal permutation.
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		step := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			half := size / 2
			for k := 0; k < half; k++ {
				a, b := x[start+k], x[start+k+half]*w
				x[start+k], x[start+k+half] = a+b, a-b
				w *= step
			}
		}
	}
}

// hann returns a Hann window of length n.
func hann(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// magnitudes returns the single-sided amplitude spectrum of the windowed samples,
// scaled so a full-scale sine reads ≈1.0 at its bin.
func magnitudes(samples, window []float64, buf []complex128, out []float64) []float64 {
	n := len(samples)
	for i, s := range samples {
		buf[i] = complex(s*window[i], 0)
	}
	fft(buf[:n])
	half := n / 2
	if cap(out) < half+1 {
		out = make([]float64, half+1)
	}
	out = out[:half+1]
	scale := 4.0 / float64(n) // ×2 single-sided, ×2 Hann coherent gain
	for k := 0; k <= half; k++ {
		out[k] = cmplx.Abs(buf[k]) * scale
	}
	return out
}
