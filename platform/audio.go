package platform

// AudioFocusChange represents the type of audio focus change event.
type AudioFocusChange int

const (
	AudioFocusGain        AudioFocusChange = 1  // AUDIOFOCUS_GAIN
	AudioFocusLoss        AudioFocusChange = 2  // AUDIOFOCUS_LOSS
	AudioFocusLossTransient     AudioFocusChange = 3  // AUDIOFOCUS_LOSS_TRANSIENT
	AudioFocusLossTransientCanDuck AudioFocusChange = 4  // AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK
	AudioFocusGainTransient      AudioFocusChange = 5  // AUDIOFOCUS_GAIN_TRANSIENT
)

// AudioState reflects the current state of the audio subsystem.
type AudioState uint8

const (
	AudioStateActive   AudioState = iota // normal playback
	AudioStateDucked                     // volume reduced (transient loss can duck)
	AudioStatePaused                     // paused due to transient loss
	AudioStateIdle                       // stopped, no audio playing
)

// AudioConfig configures an audio output stream.
type AudioConfig struct {
	SampleRate    int // Hz (e.g. 44100, 48000)
	ChannelCount  int // 1 = mono, 2 = stereo
	BitsPerSample int // 16 or 32
	BufferSizeMs  int // target buffer size in milliseconds
	LowLatency    bool // request AAudio performance mode
}

// AudioOutput is a handle to an active audio output stream.
type AudioOutput interface {
	// Write enqueues interleaved PCM samples. Returns the number of frames
	// written, or an error if the stream is in a bad state.
	Write(samples []int16) (frames int, err error)
	// Pause suspends playback without closing the stream.
	Pause() error
	// Resume restarts playback after a pause.
	Resume() error
	// Close releases all stream resources.
	Close() error
	// State returns the current stream state.
	State() AudioState
	// Latency returns the estimated stream latency in milliseconds.
	Latency() int
}

// AudioFocusCallback is called when audio focus changes occur.
type AudioFocusCallback func(change AudioFocusChange)

// Audio provides platform audio output and focus management.
type Audio interface {
	// OpenOutput creates an audio output stream with the given config.
	OpenOutput(cfg AudioConfig) (AudioOutput, error)
	// OnFocusChange registers a callback for audio focus events.
	OnFocusChange(cb AudioFocusCallback)
	// SetVolume adjusts the master output volume (0.0 = silent, 1.0 = max).
	SetVolume(v float32) error
}

// AudioCapableOf returns the platform's Audio implementation when available.
func AudioCapableOf(app App) Audio {
	if c, ok := app.(interface{ Audio() Audio }); ok {
		return c.Audio()
	}
	return nil
}
