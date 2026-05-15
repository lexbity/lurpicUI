package voiceqa

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
	"codeburg.org/lexbit/voicedsp/audio"
	"codeburg.org/lexbit/voicedsp/audio/fake"
	"codeburg.org/lexbit/voicedsp/audio/linux"
	"codeburg.org/lexbit/voicedsp/pipeline"
	"codeburg.org/lexbit/voicedsp/preset"
)

// HostOptions configures the QA host.
type HostOptions struct {
	FakeAudio       bool
	InputDeviceID   string
	OutputDeviceID  string
	MonitorDeviceID string
}

// Host owns the voicedsp pipeline and keeps the Voice UX stores fresh.
type Host struct {
	mu sync.Mutex

	opts     HostOptions
	backend  voicedsp.AudioBackend
	engine   *pipeline.Engine
	cfg      voicedsp.Config
	stores   *voiceux.VoiceStores
	registry voiceux.StaticDescriptorRegistry

	presetRegistry preset.Registry

	started bool
	stopped bool

	deviceSnapshot  voiceux.AudioDeviceSnapshot
	events          []voiceux.VoiceDiagnosticView
	nextDiagID      uint64
	processedBlocks uint64

	currentPreset     voicedsp.PresetID
	currentBypass     bool
	currentMonitor    bool
	inputGain         float32
	busGains          map[voicedsp.BusName]float32
	activeCalibration voiceux.CalibrationStateView
}

type backendAdapter struct {
	backend audio.AudioBackend
}

func (b *backendAdapter) ListInputs() ([]voicedsp.DeviceInfo, error) {
	if b == nil || b.backend == nil {
		return nil, nil
	}
	inputs, err := b.backend.ListInputs()
	return cloneRootDevices(inputs), err
}

func (b *backendAdapter) ListOutputs() ([]voicedsp.DeviceInfo, error) {
	if b == nil || b.backend == nil {
		return nil, nil
	}
	outputs, err := b.backend.ListOutputs()
	return cloneRootDevices(outputs), err
}

func (b *backendAdapter) OpenInput(device voicedsp.DeviceInfo, cfg voicedsp.StreamConfig) (voicedsp.InputStream, error) {
	if b == nil || b.backend == nil {
		return nil, fmt.Errorf("voiceqa: backend unavailable")
	}
	stream, err := b.backend.OpenInput(audio.DeviceInfo(device), audio.StreamConfig(cfg))
	if err != nil {
		return nil, err
	}
	return inputStreamAdapter{stream: stream}, nil
}

func (b *backendAdapter) OpenOutput(device voicedsp.DeviceInfo, cfg voicedsp.StreamConfig) (voicedsp.OutputStream, error) {
	if b == nil || b.backend == nil {
		return nil, fmt.Errorf("voiceqa: backend unavailable")
	}
	stream, err := b.backend.OpenOutput(audio.DeviceInfo(device), audio.StreamConfig(cfg))
	if err != nil {
		return nil, err
	}
	return outputStreamAdapter{stream: stream}, nil
}

func (b *backendAdapter) Refresh() (audio.DeviceSnapshot, error) {
	if b == nil || b.backend == nil {
		return audio.DeviceSnapshot{}, nil
	}
	if refresher, ok := b.backend.(interface {
		Refresh() (audio.DeviceSnapshot, error)
	}); ok {
		return refresher.Refresh()
	}
	return audio.DeviceSnapshot{}, nil
}

func (b *backendAdapter) Events() <-chan audio.DeviceEvent {
	if b == nil || b.backend == nil {
		return nil
	}
	if hotplug, ok := b.backend.(interface {
		Events() <-chan audio.DeviceEvent
	}); ok {
		return hotplug.Events()
	}
	return nil
}

type inputStreamAdapter struct {
	stream audio.InputStream
}

func (s inputStreamAdapter) Stop() error {
	if s.stream == nil {
		return nil
	}
	return s.stream.Stop()
}

func (s inputStreamAdapter) Read(dst []float32) (int, error) {
	if s.stream == nil {
		return 0, nil
	}
	return s.stream.Read(dst)
}

type outputStreamAdapter struct {
	stream audio.OutputStream
}

func (s outputStreamAdapter) Stop() error {
	if s.stream == nil {
		return nil
	}
	return s.stream.Stop()
}

func (s outputStreamAdapter) Write(samples []float32) (int, error) {
	if s.stream == nil {
		return 0, nil
	}
	return s.stream.Write(samples)
}

// NewHost constructs a QA host with either a real Pulse/PipeWire backend or a fake backend.
func NewHost(opts HostOptions) *Host {
	h := &Host{
		opts:           opts,
		stores:         voiceux.NewVoiceStores(),
		registry:       voiceux.DefaultDescriptorRegistry(),
		presetRegistry: preset.BuiltinRegistry(),
		currentPreset:  voicedsp.PresetID("natural"),
		busGains: map[voicedsp.BusName]float32{
			voicedsp.BusMicRaw:         1,
			voicedsp.BusVoiceProcessed: 1,
			voicedsp.BusSFX:            1,
			voicedsp.BusMonitor:        1,
			voicedsp.BusMaster:         1,
		},
		inputGain: 1,
	}
	if opts.FakeAudio {
		h.backend = newFakeBackend()
	} else {
		h.backend = &backendAdapter{backend: linux.NewBackend()}
	}
	h.cfg = voicedsp.DefaultConfig()
	h.cfg.CommandBufferSize = 128
	h.cfg.EventBufferSize = 128
	h.cfg.InputDeviceID = voicedsp.DeviceID(opts.InputDeviceID)
	h.cfg.OutputDeviceID = voicedsp.DeviceID(opts.OutputDeviceID)
	h.cfg.MonitorDeviceID = voicedsp.DeviceID(opts.MonitorDeviceID)
	return h
}

func newFakeBackend() voicedsp.AudioBackend {
	backend := fake.NewBackend()
	cfg := voicedsp.DefaultConfig()
	cfg.InputDeviceID = ""
	cfg.OutputDeviceID = ""
	backend.SetInputs(audio.DeviceInfo{
		ID:         "fake-mic",
		Name:       "Fake Voice Mic",
		Kind:       "microphone",
		Input:      true,
		Output:     false,
		Default:    true,
		Hotplug:    true,
		Channels:   cfg.Channels,
		SampleRate: cfg.SampleRate,
	})
	backend.SetOutputs(audio.DeviceInfo{
		ID:         "fake-speakers",
		Name:       "Fake Speakers",
		Kind:       "speaker",
		Input:      false,
		Output:     true,
		Default:    true,
		Hotplug:    true,
		Channels:   2,
		SampleRate: cfg.SampleRate,
	})
	backend.SetInputStream("fake-mic", fake.NewSineInput(audio.DeviceInfo{ID: "fake-mic"}, audio.StreamConfig{
		SampleRate: cfg.SampleRate,
		Channels:   cfg.Channels,
		FrameSize:  cfg.FrameSize,
		HopSize:    cfg.HopSize,
	}, 110, 0.2))
	return &backendAdapter{backend: backend}
}

func cloneRootDevices(in []audio.DeviceInfo) []voicedsp.DeviceInfo {
	if len(in) == 0 {
		return nil
	}
	out := make([]voicedsp.DeviceInfo, len(in))
	for i := range in {
		out[i] = voicedsp.DeviceInfo(in[i])
	}
	return out
}

// Stores returns the shared Voice UX stores.
func (h *Host) Stores() *voiceux.VoiceStores {
	if h == nil {
		return nil
	}
	return h.stores
}

// DescriptorRegistry returns the host descriptor registry.
func (h *Host) DescriptorRegistry() voiceux.DescriptorRegistry {
	if h == nil {
		return voiceux.DefaultDescriptorRegistry()
	}
	return h.registry
}

// Start initializes the pipeline if possible and caches an initial device snapshot.
func (h *Host) Start() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if refresher, ok := h.backend.(interface {
		Refresh() (audio.DeviceSnapshot, error)
	}); ok {
		if snapshot, err := refresher.Refresh(); err == nil {
			h.deviceSnapshot = voiceux.AudioDeviceSnapshot{
				Inputs:    snapshot.Inputs,
				Outputs:   snapshot.Outputs,
				UpdatedAt: time.Now(),
			}
		}
	}
	if h.engine == nil {
		h.engine = pipeline.New(h.backend)
	}
	cfg := h.cfg
	cfg.InputDeviceID = voicedsp.DeviceID(h.opts.InputDeviceID)
	cfg.OutputDeviceID = voicedsp.DeviceID(h.opts.OutputDeviceID)
	cfg.MonitorDeviceID = voicedsp.DeviceID(h.opts.MonitorDeviceID)
	if err := h.engine.Start(context.Background(), cfg); err != nil {
		return err
	}
	h.started = true
	return nil
}

// Stop shuts down the pipeline.
func (h *Host) Stop() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
	if h.engine == nil {
		return nil
	}
	return h.engine.Stop()
}

// Sync drains engine state and publishes host-visible stores.
func (h *Host) Sync() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.syncLocked()
}

func (h *Host) syncLocked() {
	if h.stores == nil {
		return
	}
	h.publishDeviceSnapshotLocked()
	h.publishPresetStateLocked()
	h.publishMixerStateLocked()
	h.publishCalibrationStateLocked()
	h.publishPipelineStateLocked()
	h.drainEngineLocked()
	if h.engine != nil {
		h.publishEngineDiagnosticsLocked()
	}
}

func (h *Host) publishDeviceSnapshotLocked() {
	if h.stores.Devices != nil {
		h.stores.Devices.Set(h.deviceSnapshot)
	}
	if h.stores.SelectedInput != nil && h.stores.SelectedInput.Get() == "" {
		if id := firstDeviceID(h.deviceSnapshot.Inputs); id != "" {
			h.stores.SelectedInput.Set(id)
		}
	}
	if h.stores.SelectedOutput != nil && h.stores.SelectedOutput.Get() == "" {
		if id := firstDeviceID(h.deviceSnapshot.Outputs); id != "" {
			h.stores.SelectedOutput.Set(id)
		}
	}
	if h.stores.SelectedMonitor != nil && h.stores.SelectedMonitor.Get() == "" {
		if id := firstDeviceID(h.deviceSnapshot.Outputs); id != "" {
			h.stores.SelectedMonitor.Set(id)
		}
	}
}

func (h *Host) publishPresetStateLocked() {
	if h.stores.Presets != nil && h.stores.Presets.Len() == 0 {
		h.stores.Presets.Replace(presetViews(h.presetRegistry.All(), h.currentPreset))
	}
	if h.stores.ActivePreset != nil && h.currentPreset != "" {
		h.stores.ActivePreset.Set(h.currentPreset)
	}
	if h.stores.FXChain != nil && h.stores.FXChain.Len() == 0 && h.currentPreset != "" {
		if desc, ok := h.presetRegistry.Get(h.currentPreset); ok {
			h.stores.FXChain.Replace(effectViews(desc.Effects))
		}
	}
}

func (h *Host) publishMixerStateLocked() {
	if h.stores.Mixer == nil {
		return
	}
	params := voicedsp.AudioParams{}
	if h.stores.Params != nil {
		params = h.stores.Params.Get()
	}
	level := params.Peak
	if level < params.RMS {
		level = params.RMS
	}
	h.stores.Mixer.Set(voiceux.MixerStateView{
		Buses: []voiceux.MixerBusView{
			{Bus: voicedsp.BusMicRaw, Gain: h.busGains[voicedsp.BusMicRaw], Level: params.RMS},
			{Bus: voicedsp.BusVoiceProcessed, Gain: h.busGains[voicedsp.BusVoiceProcessed], Level: params.Peak},
			{Bus: voicedsp.BusSFX, Gain: h.busGains[voicedsp.BusSFX], Level: 0},
			{Bus: voicedsp.BusMonitor, Gain: h.busGains[voicedsp.BusMonitor], Level: level, Muted: !h.currentMonitor},
			{Bus: voicedsp.BusMaster, Gain: h.busGains[voicedsp.BusMaster], Level: level},
		},
		MasterGain:     h.busGains[voicedsp.BusMaster],
		MonitorEnabled: h.currentMonitor,
		VoiceGain:      h.busGains[voicedsp.BusVoiceProcessed],
		SFXGain:        h.busGains[voicedsp.BusSFX],
	})
}

func (h *Host) publishCalibrationStateLocked() {
	if h.stores.Calibration == nil {
		return
	}
	state := h.activeCalibration
	if state.Phase == "" {
		state.Phase = voicedsp.CalibrationIdle
	}
	h.stores.Calibration.Set(state)
}

func (h *Host) publishPipelineStateLocked() {
	if h.stores.PipelineStatus == nil || h.engine == nil {
		return
	}
	diag := h.engine.Diagnostics()
	h.stores.PipelineStatus.Set(voiceux.PipelineStatusView{
		State:           diag.State,
		Running:         diag.Running,
		Message:         h.statusMessageLocked(diag.State),
		LastError:       h.lastErrorLocked(),
		LastWarning:     h.lastWarningLocked(),
		ParamsPublished: diag.ParamsPublished,
		ProcessedBlocks: h.processedBlocks,
	})
}

func (h *Host) publishEngineDiagnosticsLocked() {
	if h.stores.Diagnostics == nil {
		return
	}
	diag := h.engine.Diagnostics()
	items := []voiceux.VoiceDiagnosticView{
		{
			ID:       "engine-summary",
			Code:     "pipeline.summary",
			Severity: voicedsp.SeverityInfo,
			Target:   "pipeline",
			Message:  fmt.Sprintf("state=%s params=%d processed=%d", diag.State, diag.ParamsPublished, h.processedBlocks),
		},
	}
	items = append(items, h.events...)
	h.stores.Diagnostics.Replace(items)
}

func (h *Host) drainEngineLocked() {
	if h.engine == nil {
		return
	}
	drainParams := true
	for drainParams {
		select {
		case params := <-h.engine.Params():
			h.processedBlocks++
			if h.stores.Params != nil {
				h.stores.Params.Set(params)
			}
		default:
			drainParams = false
		}
	}
	drainEvents := true
	for drainEvents {
		select {
		case event := <-h.engine.Events():
			h.applyEventLocked(event)
		default:
			drainEvents = false
		}
	}
}

func (h *Host) applyEventLocked(event voicedsp.Event) {
	switch ev := event.(type) {
	case voicedsp.DeviceChanged:
		h.deviceSnapshot = splitDevices(ev.Devices)
	case voicedsp.PipelineStateChanged:
		if h.stores.PipelineStatus != nil {
			current := h.stores.PipelineStatus.Get()
			current.State = ev.To
			current.Running = ev.To == voicedsp.PipelineStateRunning || ev.To == voicedsp.PipelineStateStarting
			current.Message = h.statusMessageLocked(ev.To)
			current.ProcessedBlocks = h.processedBlocks
			h.stores.PipelineStatus.Set(current)
		}
	case voicedsp.PresetChanged:
		h.currentPreset = ev.ID
		if h.stores.ActivePreset != nil {
			h.stores.ActivePreset.Set(ev.ID)
		}
		if desc, ok := h.presetRegistry.Get(ev.ID); ok && h.stores.FXChain != nil {
			h.stores.FXChain.Replace(effectViews(desc.Effects))
		}
	case voicedsp.EffectParamChanged:
		if h.stores.FXChain != nil {
			items := h.stores.FXChain.All()
			for i := range items {
				if items[i].ID == ev.Effect {
					if items[i].Params == nil {
						items[i].Params = make(map[voicedsp.ParameterID]float32)
					}
					items[i].Params[ev.Param] = ev.Value
					h.stores.FXChain.Update(items[i])
					break
				}
			}
		}
	case voicedsp.CalibrationProgressEvent:
		h.activeCalibration = calibrationStateFromProgress(ev.Progress)
		if h.stores.Calibration != nil {
			h.stores.Calibration.Set(h.activeCalibration)
		}
	case voicedsp.CalibrationCompletedEvent:
		h.activeCalibration.Phase = voicedsp.CalibrationComplete
		h.activeCalibration.CanCommit = true
		if h.stores.Calibration != nil {
			h.stores.Calibration.Set(h.activeCalibration)
		}
	case voicedsp.AudioWarning:
		h.appendDiagnosticLocked("warning", ev.Code, ev.Message, ev.Severity)
		h.setWarningLocked(ev.Message)
	case voicedsp.AudioError:
		h.appendDiagnosticLocked("error", ev.Code, ev.Message, voicedsp.SeverityError)
		h.setErrorLocked(ev.Message)
	}
}

func (h *Host) appendDiagnosticLocked(kind, code, message string, severity voicedsp.Severity) {
	h.nextDiagID++
	entry := voiceux.VoiceDiagnosticView{
		ID:       fmt.Sprintf("diag-%d", h.nextDiagID),
		Code:     code,
		Severity: severity,
		Target:   kind,
		Message:  message,
	}
	h.events = append(h.events, entry)
	if len(h.events) > 64 {
		h.events = h.events[len(h.events)-64:]
	}
}

func (h *Host) setErrorLocked(msg string) {
	if h.stores.PipelineStatus == nil {
		return
	}
	current := h.stores.PipelineStatus.Get()
	current.LastError = msg
	current.Message = msg
	h.stores.PipelineStatus.Set(current)
}

func (h *Host) setWarningLocked(msg string) {
	if h.stores.PipelineStatus == nil {
		return
	}
	current := h.stores.PipelineStatus.Get()
	current.LastWarning = msg
	if current.Message == "" {
		current.Message = msg
	}
	h.stores.PipelineStatus.Set(current)
}

func (h *Host) lastErrorLocked() string {
	if h.stores.PipelineStatus == nil {
		return ""
	}
	return h.stores.PipelineStatus.Get().LastError
}

func (h *Host) lastWarningLocked() string {
	if h.stores.PipelineStatus == nil {
		return ""
	}
	return h.stores.PipelineStatus.Get().LastWarning
}

func (h *Host) statusMessageLocked(state voicedsp.PipelineState) string {
	switch state {
	case voicedsp.PipelineStateRunning:
		return "voice pipeline running"
	case voicedsp.PipelineStateStarting:
		return "starting voice pipeline"
	case voicedsp.PipelineStateStopping:
		return "stopping voice pipeline"
	case voicedsp.PipelineStateFailed:
		return "voice pipeline failed"
	default:
		return "voice pipeline stopped"
	}
}

// VoiceService methods.
func (h *Host) DispatchVoiceCommand(cmd voiceux.VoiceCommand) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch v := cmd.(type) {
	case voiceux.SetPresetCommand:
		h.currentPreset = v.ID
		if h.stores.ActivePreset != nil {
			h.stores.ActivePreset.Set(v.ID)
		}
		if desc, ok := h.presetRegistry.Get(v.ID); ok && h.stores.FXChain != nil {
			h.stores.FXChain.Replace(effectViews(desc.Effects))
		}
		return h.send(voicedsp.SetPreset{ID: v.ID})
	case voiceux.SetBypassCommand:
		h.currentBypass = v.Enabled
		if h.stores.FXBypassed != nil {
			h.stores.FXBypassed.Set(v.Enabled)
		}
		return h.send(voicedsp.SetBypass{Enabled: v.Enabled})
	case voiceux.SetMonitorCommand:
		h.currentMonitor = v.Enabled
		if h.stores.MonitorEnabled != nil {
			h.stores.MonitorEnabled.Set(v.Enabled)
		}
		if h.stores.Mixer != nil {
			h.publishMixerStateLocked()
		}
		return h.send(voicedsp.SetMonitor{Enabled: v.Enabled})
	case voiceux.SetInputGainCommand:
		h.inputGain = v.Gain
		return h.send(voicedsp.SetInputGain{Gain: v.Gain})
	case voiceux.SetBusGainCommand:
		h.busGains[v.Bus] = v.Gain
		h.publishMixerStateLocked()
		return h.send(voicedsp.SetBusGain{Bus: v.Bus, Gain: v.Gain})
	case voiceux.SetEffectParamCommand:
		return h.send(voicedsp.SetEffectParam{Effect: v.Effect, Param: v.Param, Value: v.Value})
	case voiceux.PlaySoundCommand:
		return h.send(voicedsp.PlaySound{Cue: v.Cue})
	case voiceux.StopSoundCommand:
		return h.send(voicedsp.StopSound{ID: v.ID})
	case voiceux.StartCalibrationCommand:
		h.activeCalibration = voiceux.CalibrationStateView{
			Phase:     voicedsp.CalibrationExplain,
			CanCancel: true,
		}
		if h.stores.Calibration != nil {
			h.stores.Calibration.Set(h.activeCalibration)
		}
		return h.send(voicedsp.StartCalibration{Config: v.Config})
	case voiceux.CancelCalibrationCommand:
		h.activeCalibration.Phase = voicedsp.CalibrationCancelled
		h.activeCalibration.CanCancel = false
		if h.stores.Calibration != nil {
			h.stores.Calibration.Set(h.activeCalibration)
		}
		return h.send(voicedsp.CancelCalibration{})
	case voiceux.CommitCalibrationCommand:
		h.activeCalibration.Phase = voicedsp.CalibrationComplete
		h.activeCalibration.CanCommit = true
		if h.stores.Calibration != nil {
			h.stores.Calibration.Set(h.activeCalibration)
		}
		return h.send(voicedsp.CommitCalibration{Calibration: v.Calibration})
	default:
		return nil
	}
}

func (h *Host) DispatchAction(actionID string, args map[string]any) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch actionID {
	case "refresh_devices":
		if refresher, ok := h.backend.(interface {
			Refresh() (audio.DeviceSnapshot, error)
		}); ok {
			snapshot, err := refresher.Refresh()
			if err != nil {
				return err
			}
			h.deviceSnapshot = voiceux.AudioDeviceSnapshot{
				Inputs:    snapshot.Inputs,
				Outputs:   snapshot.Outputs,
				UpdatedAt: time.Now(),
			}
			h.publishDeviceSnapshotLocked()
		}
		return nil
	case "select_input":
		if id, ok := argString(args, "id"); ok {
			h.cfg.InputDeviceID = voicedsp.DeviceID(id)
			if h.stores.SelectedInput != nil {
				h.stores.SelectedInput.Set(voicedsp.DeviceID(id))
			}
			return h.restartLocked()
		}
	case "select_output":
		if id, ok := argString(args, "id"); ok {
			h.cfg.OutputDeviceID = voicedsp.DeviceID(id)
			if h.stores.SelectedOutput != nil {
				h.stores.SelectedOutput.Set(voicedsp.DeviceID(id))
			}
			return h.restartLocked()
		}
	case "select_monitor":
		if id, ok := argString(args, "id"); ok {
			h.cfg.MonitorDeviceID = voicedsp.DeviceID(id)
			if h.stores.SelectedMonitor != nil {
				h.stores.SelectedMonitor.Set(voicedsp.DeviceID(id))
			}
			return h.restartLocked()
		}
	}
	return nil
}

func (h *Host) restartLocked() error {
	if h.engine == nil {
		h.engine = pipeline.New(h.backend)
	}
	cfg := h.cfg
	return h.engine.Restart(context.Background(), cfg)
}

func (h *Host) send(msg voicedsp.ControlMessage) error {
	if h.engine == nil {
		return nil
	}
	select {
	case h.engine.Commands() <- msg:
		return nil
	default:
		return fmt.Errorf("voiceqa: command queue full")
	}
}

func presetViews(in []voicedsp.PresetDescriptor, active voicedsp.PresetID) []voiceux.PresetView {
	out := make([]voiceux.PresetView, 0, len(in))
	for _, preset := range in {
		out = append(out, voiceux.PresetView{
			ID:          preset.ID,
			Name:        preset.Name,
			Description: preset.Description,
			Tags:        append([]string(nil), preset.Tags...),
			Enabled:     true,
			Selected:    preset.ID == active,
		})
	}
	return out
}

func effectViews(in []voicedsp.EffectConfig) []voiceux.FXSlotView {
	out := make([]voiceux.FXSlotView, 0, len(in))
	for _, effect := range in {
		params := make(map[voicedsp.ParameterID]float32, len(effect.Params))
		for key, value := range effect.Params {
			params[key] = value
		}
		out = append(out, voiceux.FXSlotView{
			ID:          effect.ID,
			Kind:        effect.Kind,
			Name:        string(effect.ID),
			Description: string(effect.Kind),
			Enabled:     effect.Enabled,
			Wet:         effect.Wet,
			Params:      params,
		})
	}
	return out
}

func splitDevices(devices []voicedsp.DeviceInfo) voiceux.AudioDeviceSnapshot {
	snapshot := voiceux.AudioDeviceSnapshot{}
	for _, device := range devices {
		if device.Input {
			snapshot.Inputs = append(snapshot.Inputs, device)
		}
		if device.Output {
			snapshot.Outputs = append(snapshot.Outputs, device)
		}
	}
	snapshot.UpdatedAt = time.Now()
	return snapshot
}

func calibrationStateFromProgress(progress voicedsp.CalibrationProgress) voiceux.CalibrationStateView {
	state := voiceux.CalibrationStateView{
		Phase:     progress.Phase,
		Progress:  progress,
		CanCancel: progress.Phase != voicedsp.CalibrationComplete && progress.Phase != voicedsp.CalibrationCancelled,
		CanCommit: progress.Phase == voicedsp.CalibrationReview || progress.Phase == voicedsp.CalibrationComplete,
	}
	switch progress.Phase {
	case voicedsp.CalibrationPromptA:
		state.CurrentVowel = voicedsp.VowelA
	case voicedsp.CalibrationPromptE:
		state.CurrentVowel = voicedsp.VowelE
	case voicedsp.CalibrationPromptI:
		state.CurrentVowel = voicedsp.VowelI
	case voicedsp.CalibrationPromptO:
		state.CurrentVowel = voicedsp.VowelO
	case voicedsp.CalibrationPromptU:
		state.CurrentVowel = voicedsp.VowelU
	}
	return state
}

func firstDeviceID(devices []voicedsp.DeviceInfo) voicedsp.DeviceID {
	for _, device := range devices {
		if device.ID != "" {
			return device.ID
		}
	}
	return ""
}

func argString(args map[string]any, key string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	value, ok := args[key]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok && str != ""
}
