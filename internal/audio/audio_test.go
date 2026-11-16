package audio

import (
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func sine(freq, amp float64, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = amp * math.Sin(2*math.Pi*freq*float64(i)/SampleRate)
	}
	return s
}

func TestMagnitudesFindSinePeak(t *testing.T) {
	const freq = 1000.0
	mags := magnitudes(sine(freq, 0.5, FFTSize), hann(FFTSize), make([]complex128, FFTSize), nil)
	best := 0
	for k := range mags {
		if mags[k] > mags[best] {
			best = k
		}
	}
	binHz := float64(SampleRate) / FFTSize
	if got := float64(best) * binHz; math.Abs(got-freq) > binHz {
		t.Fatalf("peak at %.0f Hz, want %.0f", got, freq)
	}
	if mags[best] < 0.35 || mags[best] > 0.55 {
		t.Errorf("peak amplitude %.3f, want ≈0.5", mags[best])
	}
}

func TestBandEdgesAreLogSpaced(t *testing.T) {
	e := BandEdges(10)
	if len(e) != 11 || math.Abs(e[0]-minFreq) > 1e-9 || math.Abs(e[10]-maxFreq) > 1e-6 {
		t.Fatalf("edges %v", e)
	}
	r := e[1] / e[0]
	for i := 1; i < 10; i++ {
		if math.Abs(e[i+1]/e[i]-r) > 1e-9 {
			t.Fatalf("not log spaced at %d", i)
		}
	}
}

func TestSpectrumRisesOnToneAndFallsInSilence(t *testing.T) {
	mags := magnitudes(sine(1000, 0.5, FFTSize), hann(FFTSize), make([]complex128, FFTSize), nil)
	var s Spectrum
	s.Resize(24)
	for i := 0; i < 10; i++ {
		s.Update(Frame{Mags: mags, BinHz: float64(SampleRate) / FFTSize, Active: true}, 0.04)
	}
	edges := BandEdges(24)
	peakBand := 0
	for i := range s.Values {
		if s.Values[i] > s.Values[peakBand] {
			peakBand = i
		}
	}
	if !(edges[peakBand] <= 1000 && 1000 <= edges[peakBand+1]) {
		t.Errorf("loudest band %d (%.0f–%.0f Hz) doesn't contain 1 kHz", peakBand, edges[peakBand], edges[peakBand+1])
	}
	if s.Values[peakBand] < 0.8 {
		t.Errorf("tone band only reached %.2f", s.Values[peakBand])
	}
	for i := 0; i < 40; i++ {
		s.Update(Frame{}, 0.04)
	}
	if !s.Settled() {
		t.Errorf("bars did not settle in silence: %v", s.Values)
	}
}

func TestCaptureReadsPCM(t *testing.T) {
	samples := sine(440, 0.8, SampleRate/2)
	raw := make([]byte, len(samples)*2)
	for i, v := range samples {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(int16(v*32767)))
	}
	path := filepath.Join(t.TempDir(), "tone.raw")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// Emit the tone, then keep the pipe open briefly like a live stream.
	c := New()
	c.factory = func() (*exec.Cmd, error) {
		return exec.Command("sh", "-c", `cat "$1"; sleep 1`, "sh", path), nil
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	deadline := time.Now().Add(2 * time.Second)
	var f Frame
	for time.Now().Before(deadline) {
		f = c.Analyze(nil)
		if f.Active {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !f.Active {
		t.Fatal("capture never became active")
	}
	best := 0
	for k := range f.Mags {
		if f.Mags[k] > f.Mags[best] {
			best = k
		}
	}
	if got := float64(best) * f.BinHz; math.Abs(got-440) > f.BinHz {
		t.Errorf("captured peak at %.0f Hz, want 440", got)
	}
	if len(f.Wave) != WaveSize {
		t.Errorf("wave has %d samples", len(f.Wave))
	}
	c.Stop()
	if c.Running() {
		t.Error("still running after Stop")
	}
}
