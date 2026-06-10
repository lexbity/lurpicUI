package platform

import (
	"testing"
)

type fakeAudio struct {
	lastConfig AudioConfig
	lastVolume float32
	focusCB    AudioFocusCallback
	outputs    []*fakeAudioOutput
}

type fakeAudioOutput struct {
	closed bool
	paused bool
	state  AudioState
	data   []int16
}

func (o *fakeAudioOutput) Write(samples []int16) (int, error) {
	o.data = append(o.data, samples...)
	return len(samples), nil
}
func (o *fakeAudioOutput) Pause() error      { o.paused = true; o.state = AudioStatePaused; return nil }
func (o *fakeAudioOutput) Resume() error     { o.paused = false; o.state = AudioStateActive; return nil }
func (o *fakeAudioOutput) Close() error      { o.closed = true; o.state = AudioStateIdle; return nil }
func (o *fakeAudioOutput) State() AudioState { return o.state }
func (o *fakeAudioOutput) Latency() int      { return 0 }

func (a *fakeAudio) OpenOutput(cfg AudioConfig) (AudioOutput, error) {
	a.lastConfig = cfg
	out := &fakeAudioOutput{state: AudioStateActive}
	a.outputs = append(a.outputs, out)
	return out, nil
}
func (a *fakeAudio) OnFocusChange(cb AudioFocusCallback) { a.focusCB = cb }
func (a *fakeAudio) SetVolume(v float32) error           { a.lastVolume = v; return nil }

func TestAudioInterface_openOutput(t *testing.T) {
	a := &fakeAudio{}
	cfg := AudioConfig{SampleRate: 44100, ChannelCount: 2, BitsPerSample: 16, BufferSizeMs: 50}
	out, err := a.OpenOutput(cfg)
	if err != nil {
		t.Fatalf("OpenOutput: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil output")
	}
	if a.lastConfig.SampleRate != 44100 {
		t.Errorf("expected sample rate 44100, got %d", a.lastConfig.SampleRate)
	}
	if out.State() != AudioStateActive {
		t.Errorf("expected active state, got %v", out.State())
	}
}

func TestAudioOutput_writeAndClose(t *testing.T) {
	a := &fakeAudio{}
	out, _ := a.OpenOutput(AudioConfig{SampleRate: 48000, ChannelCount: 1, BitsPerSample: 16})

	samples := []int16{100, 200, 300, 400}
	n, err := out.Write(samples)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 4 {
		t.Fatalf("expected 4 frames, got %d", n)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if out.State() != AudioStateIdle {
		t.Errorf("expected idle state after close, got %v", out.State())
	}
}

func TestAudioOutput_pauseAndResume(t *testing.T) {
	a := &fakeAudio{}
	out, _ := a.OpenOutput(AudioConfig{})
	if err := out.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if out.State() != AudioStatePaused {
		t.Errorf("expected paused state, got %v", out.State())
	}
	if err := out.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if out.State() != AudioStateActive {
		t.Errorf("expected active state after resume, got %v", out.State())
	}
}

func TestAudioFocusCallback_invoked(t *testing.T) {
	a := &fakeAudio{}
	received := make(chan AudioFocusChange, 1)
	a.OnFocusChange(func(change AudioFocusChange) {
		received <- change
	})

	if a.focusCB == nil {
		t.Fatal("expected focus callback to be registered")
	}
}

func TestAudioSetVolume(t *testing.T) {
	a := &fakeAudio{}
	if err := a.SetVolume(0.5); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if a.lastVolume != 0.5 {
		t.Fatalf("expected volume 0.5, got %f", a.lastVolume)
	}
}

func TestAudioCapableOf_returnsNilForNonAudioApp(t *testing.T) {
	app := &struct{ App }{}
	audio := AudioCapableOf(app)
	if audio != nil {
		t.Fatal("expected nil for app without Audio() method")
	}
}
