package voiceux

import (
	"hash/fnv"
	"time"

	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/voicedsp"
)

// AudioDeviceSnapshot captures the current host-visible device lists.
type AudioDeviceSnapshot struct {
	Inputs    []voicedsp.DeviceInfo
	Outputs   []voicedsp.DeviceInfo
	UpdatedAt time.Time
}

// PipelineStatusView mirrors the host's current pipeline state for UI consumption.
type PipelineStatusView struct {
	State           voicedsp.PipelineState
	Running         bool
	Message         string
	LastError       string
	LastWarning     string
	ParamsPublished uint64
	ProcessedBlocks uint64
}

// AudioParamsView is the UI-facing copy of voiceDSP audio measurements.
type AudioParamsView = voicedsp.AudioParams

// PresetView summarizes one preset entry for the browser.
type PresetView struct {
	ID          voicedsp.PresetID
	Name        string
	Description string
	Tags        []string
	Enabled     bool
	Selected    bool
}

// FXSlotView summarizes one effect node in the current chain.
type FXSlotView struct {
	ID          voicedsp.EffectID
	Kind        voicedsp.EffectKind
	Name        string
	Description string
	Enabled     bool
	Wet         float32
	Params      map[voicedsp.ParameterID]float32
	Tags        []string
}

// CalibrationStateView summarizes the live calibration workflow.
type CalibrationStateView struct {
	SessionID    string
	Progress     voicedsp.CalibrationProgress
	Phase        voicedsp.CalibrationPhase
	CurrentVowel voicedsp.Vowel
	StableFrames int
	CanCommit    bool
	CanCancel    bool
}

// VoiceDiagnosticView represents one host-visible diagnostic record.
type VoiceDiagnosticView struct {
	ID         string
	Code       string
	Severity   voicedsp.Severity
	Target     string
	Message    string
	Detail     string
	Suggestion string
}

// MixerBusView summarizes one mixer bus.
type MixerBusView struct {
	Bus   voicedsp.BusName
	Gain  float32
	Level float32
	Muted bool
	Solo  bool
}

// MixerStateView exposes mixer controls and bus meters.
type MixerStateView struct {
	Buses          []MixerBusView
	MasterGain     float32
	MonitorEnabled bool
	VoiceGain      float32
	SFXGain        float32
}

// VoiceStores contains the reusable UI state used by Voice UX facets.
type VoiceStores struct {
	Devices         *store.ValueStore[AudioDeviceSnapshot]
	SelectedInput   *store.ValueStore[voicedsp.DeviceID]
	SelectedOutput  *store.ValueStore[voicedsp.DeviceID]
	SelectedMonitor *store.ValueStore[voicedsp.DeviceID]

	Params         *store.ValueStore[AudioParamsView]
	PipelineStatus *store.ValueStore[PipelineStatusView]
	ActivePreset   *store.ValueStore[voicedsp.PresetID]
	Presets        *store.CollectionStore[PresetView]
	FXChain        *store.CollectionStore[FXSlotView]

	Calibration *store.ValueStore[CalibrationStateView]
	Diagnostics *store.CollectionStore[VoiceDiagnosticView]

	MonitorEnabled *store.ValueStore[bool]
	FXBypassed     *store.ValueStore[bool]
	Mixer          *store.ValueStore[MixerStateView]
}

// NewVoiceStores constructs Voice UX stores with deterministic collection IDs.
func NewVoiceStores() *VoiceStores {
	return &VoiceStores{
		Devices:         store.NewValueStore(AudioDeviceSnapshot{}),
		SelectedInput:   store.NewValueStore(voicedsp.DeviceID("")),
		SelectedOutput:  store.NewValueStore(voicedsp.DeviceID("")),
		SelectedMonitor: store.NewValueStore(voicedsp.DeviceID("")),
		Params:          store.NewValueStore(AudioParamsView{}),
		PipelineStatus:  store.NewValueStore(PipelineStatusView{}),
		ActivePreset:    store.NewValueStore(voicedsp.PresetID("")),
		Presets:         newIDCollection(func(v PresetView) string { return string(v.ID) }),
		FXChain:         newIDCollection(func(v FXSlotView) string { return string(v.ID) }),
		Calibration:     store.NewValueStore(CalibrationStateView{}),
		Diagnostics:     newIDCollection(func(v VoiceDiagnosticView) string { return v.ID }),
		MonitorEnabled:  store.NewValueStore(false),
		FXBypassed:      store.NewValueStore(false),
		Mixer:           store.NewValueStore(MixerStateView{}),
	}
}

func newIDCollection[T any](keyFn func(T) string) *store.CollectionStore[T] {
	return store.NewCollectionStore(func(v T) store.ItemID {
		return stableItemID(keyFn(v))
	})
}

func stableItemID(s string) store.ItemID {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return store.ItemID(h.Sum64())
}
