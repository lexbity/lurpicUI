//go:build android

// Package audio provides the Android audio subsystem implementation using AAudio
// (primary) and OpenSL ES (fallback) for low-latency audio playback with focus
// and interruption handling.
//
// AAudio is the preferred path (API 27+, low-latency). When AAudio is not
// available or the device does not support the requested configuration, the
// implementation falls back to OpenSL ES.
//
// Audio-focus changes are routed from the Java AudioManager.OnAudioFocusChangeListener
// through the C bridge to the Go event queue.
package audio

import (
	"errors"
	"sync"

	"codeburg.org/lexbit/lurpicui/platform"
)

// ErrAudioNotAvailable is returned when no audio backend can be initialized.
var ErrAudioNotAvailable = errors.New("android audio: no audio backend available")

// BackendKind indicates which audio API is being used.
type BackendKind int

const (
	BackendAAudio  BackendKind = iota // AAudio (primary)
	BackendOpenSL                     // OpenSL ES (fallback)
)

// Backend is the singleton audio backend manager.
type Backend struct {
	mu            sync.Mutex
	streams       map[int]*stream
	nextStreamID  int
	focusCB       func(platform.AudioFocusChange)
	currentState  platform.AudioState
	backendKind   BackendKind
	volume        float32
}

// stream wraps an active audio output stream.
type stream struct {
	id            int
	config        platform.AudioConfig
	backend       BackendKind
	state         platform.AudioState
	closeFn       func() error
	pauseFn       func() error
	resumeFn      func() error
	writeFn       func([]int16) (int, error)
	latencyFn     func() int
}

func (s *stream) Write(samples []int16) (int, error)    { return s.writeFn(samples) }
func (s *stream) Pause() error                          { return s.pauseFn() }
func (s *stream) Resume() error                         { return s.resumeFn() }
func (s *stream) Close() error                          { return s.closeFn() }
func (s *stream) State() platform.AudioState             { return s.state }
func (s *stream) Latency() int                          { return s.latencyFn() }

// NewBackend creates the audio backend, attempting AAudio first and falling
// back to OpenSL ES.
func NewBackend() *Backend {
	b := &Backend{
		streams:      make(map[int]*stream),
		currentState: platform.AudioStateIdle,
		volume:       1.0,
	}
	b.backendKind = detectBackend()
	return b
}

// detectBackend checks if AAudio is available on this device.
func detectBackend() BackendKind {
	// AAudio is available on API 27+ (Android 8.1).
	// The C-side initialization checks for the presence of the AAudio shared
	// library at runtime. If unavailable, we fall back to OpenSL ES.
	if aaudioAvailable() {
		return BackendAAudio
	}
	return BackendOpenSL
}

// aaudioAvailable returns true if the AAudio shared library can be loaded.
// Implemented in aaudio_android.c via cgo.
func aaudioAvailable() bool {
	return cAAudioAvailable()
}

// OpenOutput creates an audio output stream.
func (b *Backend) OpenOutput(cfg platform.AudioConfig) (platform.AudioOutput, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextStreamID
	b.nextStreamID++

	s := &stream{
		id:      id,
		config:  cfg,
		backend: b.backendKind,
		state:   platform.AudioStateActive,
	}

	var err error
	if b.backendKind == BackendAAudio {
		err = b.initAAudioStream(s, cfg)
	} else {
		err = b.initOpenSLStream(s, cfg)
	}
	if err != nil {
		return nil, err
	}

	b.streams[id] = s
	b.currentState = platform.AudioStateActive
	return s, nil
}

// initAAudioStream initialises an AAudio stream via C.
func (b *Backend) initAAudioStream(s *stream, cfg platform.AudioConfig) error {
	// AAudio stream handle returned from C.
	handle, err := cAAudioStreamOpen(cfg.SampleRate, cfg.ChannelCount, cfg.BitsPerSample, cfg.LowLatency)
	if err != nil {
		return err
	}

	s.closeFn = func() error {
		return cAAudioStreamClose(handle)
	}
	s.pauseFn = func() error {
		s.state = platform.AudioStatePaused
		return cAAudioStreamPause(handle)
	}
	s.resumeFn = func() error {
		s.state = platform.AudioStateActive
		return cAAudioStreamResume(handle)
	}
	s.writeFn = func(samples []int16) (int, error) {
		return cAAudioStreamWrite(handle, samples)
	}
	s.latencyFn = func() int {
		return cAAudioStreamLatency(handle)
	}
	return nil
}

// initOpenSLStream initialises an OpenSL ES stream via C.
func (b *Backend) initOpenSLStream(s *stream, cfg platform.AudioConfig) error {
	handle, err := cOpenSLStreamOpen(cfg.SampleRate, cfg.ChannelCount, cfg.BitsPerSample)
	if err != nil {
		return err
	}

	s.closeFn = func() error {
		return cOpenSLStreamClose(handle)
	}
	s.pauseFn = func() error {
		s.state = platform.AudioStatePaused
		return cOpenSLStreamPause(handle)
	}
	s.resumeFn = func() error {
		s.state = platform.AudioStateActive
		return cOpenSLStreamResume(handle)
	}
	s.writeFn = func(samples []int16) (int, error) {
		return cOpenSLStreamWrite(handle, samples)
	}
	s.latencyFn = func() int {
		return 0 // OpenSL ES does not provide latency query
	}
	return nil
}

// OnFocusChange registers a callback for audio focus events from Java.
func (b *Backend) OnFocusChange(cb platform.AudioFocusCallback) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.focusCB = cb
}

// HandleFocusChange is called from the Java audio-focus listener via the bridge.
func (b *Backend) HandleFocusChange(change platform.AudioFocusChange) {
	b.mu.Lock()
	cb := b.focusCB
	state := b.currentState
	b.mu.Unlock()

	// Update internal state based on focus change.
	switch change {
	case platform.AudioFocusLoss:
		state = platform.AudioStateIdle
	case platform.AudioFocusLossTransient:
		state = platform.AudioStatePaused
	case platform.AudioFocusLossTransientCanDuck:
		state = platform.AudioStateDucked
	case platform.AudioFocusGain, platform.AudioFocusGainTransient:
		state = platform.AudioStateActive
	}

	b.mu.Lock()
	b.currentState = state
	b.mu.Unlock()

	if cb != nil {
		cb(change)
	}
}

// SetVolume adjusts the master volume.
func (b *Backend) SetVolume(v float32) error {
	if v < 0 || v > 1 {
		return errors.New("android audio: volume out of range (0.0–1.0)")
	}
	b.mu.Lock()
	b.volume = v
	b.mu.Unlock()
	// Volume is applied per-stream on the C side.
	return nil
}
