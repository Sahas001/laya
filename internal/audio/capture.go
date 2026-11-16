package audio

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const (
	// SampleRate is the capture rate in Hz.
	SampleRate = 44100
	// FFTSize is the number of samples analysed per frame (~46 ms).
	FFTSize = 2048
	// WaveSize is the number of samples returned for the oscilloscope.
	WaveSize = 1024
)

// commandFactory builds the capture process; overridable in tests.
type commandFactory func() (*exec.Cmd, error)

// backends lists capture commands in order of preference. Each reads the
// default output's monitor as signed 16-bit little-endian mono PCM.
var backends = []func() (*exec.Cmd, error){
	func() (*exec.Cmd, error) {
		path, err := exec.LookPath("parec")
		if err != nil {
			return nil, err
		}
		return exec.Command(path,
			"--device=@DEFAULT_MONITOR@", "--raw",
			"--format=s16le", "--rate=44100", "--channels=1",
			"--latency-msec=25",
			"--client-name=laya", "--stream-name=laya visualizer"), nil
	},
	func() (*exec.Cmd, error) {
		path, err := exec.LookPath("pw-record")
		if err != nil {
			return nil, err
		}
		return exec.Command(path,
			"-P", "{ stream.capture.sink=true node.name=laya-visualizer }",
			"--rate=44100", "--channels=1", "--format=s16", "--latency=25ms", "-"), nil
	},
}

// ErrNoBackend means neither parec nor pw-record is installed.
var ErrNoBackend = errors.New("no audio capture tool found (install pipewire-pulse or pipewire)")

// Capture reads system audio in the background and keeps the latest samples.
type Capture struct {
	mu       sync.Mutex
	ring     []float64
	pos      int
	lastData time.Time
	cmd      *exec.Cmd
	running  bool
	err      error
	factory  commandFactory

	window []float64
	frame  []float64
	buf    []complex128
	mags   []float64
}

// New returns a capture using the first available backend. It isn't started.
func New() *Capture {
	return &Capture{
		ring:   make([]float64, FFTSize*2),
		window: hann(FFTSize),
		frame:  make([]float64, FFTSize),
		buf:    make([]complex128, FFTSize),
	}
}

// Start launches the capture process. Calling Start on a running capture is a no-op.
func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil
	}
	var cmd *exec.Cmd
	var err error
	if c.factory != nil {
		cmd, err = c.factory()
	} else {
		err = ErrNoBackend
		for _, b := range backends {
			if cmd, err = b(); err == nil {
				break
			}
		}
		if err != nil {
			err = ErrNoBackend
		}
	}
	if err != nil {
		c.err = err
		return err
	}
	// Make sure the helper dies with us even if we crash.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	out, err := cmd.StdoutPipe()
	if err != nil {
		c.err = err
		return err
	}
	if err := cmd.Start(); err != nil {
		c.err = err
		return err
	}
	c.cmd, c.running, c.err = cmd, true, nil
	go c.read(cmd, out)
	return nil
}

func (c *Capture) read(cmd *exec.Cmd, r io.Reader) {
	br := bufio.NewReaderSize(r, 8192)
	chunk := make([]byte, 2048)
	for {
		n, err := io.ReadFull(br, chunk)
		n -= n % 2
		if n > 0 {
			c.mu.Lock()
			for i := 0; i < n; i += 2 {
				s := int16(binary.LittleEndian.Uint16(chunk[i:]))
				c.ring[c.pos] = float64(s) / 32768
				c.pos = (c.pos + 1) % len(c.ring)
			}
			c.lastData = time.Now()
			c.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	waitErr := cmd.Wait()
	c.mu.Lock()
	if c.cmd == cmd {
		c.running = false
		if c.err == nil && waitErr != nil {
			c.err = waitErr
		}
	}
	c.mu.Unlock()
}

// Stop terminates the capture process.
func (c *Capture) Stop() {
	c.mu.Lock()
	cmd := c.cmd
	c.cmd, c.running = nil, false
	for i := range c.ring {
		c.ring[i] = 0
	}
	c.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// Running reports whether audio is being captured.
func (c *Capture) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Err returns the last capture error, if any.
func (c *Capture) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Frame is one analysis snapshot.
type Frame struct {
	Mags   []float64 // amplitude spectrum, FFTSize/2+1 bins
	Wave   []float64 // most recent WaveSize samples in [-1, 1]
	BinHz  float64
	Active bool // data arrived recently
}

// Analyze computes a spectrum from the latest samples. The returned slices are
// reused between calls and must not be retained.
func (c *Capture) Analyze(wave []float64) Frame {
	c.mu.Lock()
	start := (c.pos - FFTSize + len(c.ring)) % len(c.ring)
	for i := 0; i < FFTSize; i++ {
		c.frame[i] = c.ring[(start+i)%len(c.ring)]
	}
	active := c.running && time.Since(c.lastData) < 500*time.Millisecond
	c.mu.Unlock()

	if cap(wave) < WaveSize {
		wave = make([]float64, WaveSize)
	}
	wave = wave[:WaveSize]
	copy(wave, c.frame[FFTSize-WaveSize:])

	c.mags = magnitudes(c.frame, c.window, c.buf, c.mags)
	return Frame{Mags: c.mags, Wave: wave, BinHz: float64(SampleRate) / FFTSize, Active: active}
}
