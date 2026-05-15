package voiceux

import "codeburg.org/lexbit/voicedsp"

// VoiceService is the host-facing contract used by the reusable Voice UX package.
type VoiceService interface {
	Stores() *VoiceStores
	DispatchVoiceCommand(cmd VoiceCommand) error
	DispatchAction(actionID string, args map[string]any) error
	DescriptorRegistry() DescriptorRegistry
}

// VoiceCommand is the typed control surface dispatched by Voice UX.
type VoiceCommand interface {
	voiceCommand()
}

// SetPresetCommand requests a preset change.
type SetPresetCommand struct {
	ID voicedsp.PresetID
}

// SetBypassCommand toggles the FX bypass state.
type SetBypassCommand struct {
	Enabled bool
}

// SetMonitorCommand toggles monitor output routing.
type SetMonitorCommand struct {
	Enabled bool
}

// SetInputGainCommand adjusts the capture gain.
type SetInputGainCommand struct {
	Gain float32
}

// SetBusGainCommand adjusts one bus gain.
type SetBusGainCommand struct {
	Bus  voicedsp.BusName
	Gain float32
}

// SetEffectParamCommand updates a single effect parameter.
type SetEffectParamCommand struct {
	Effect voicedsp.EffectID
	Param  voicedsp.ParameterID
	Value  float32
}

// PlaySoundCommand starts an SFX cue.
type PlaySoundCommand struct {
	Cue voicedsp.SoundCue
}

// StopSoundCommand stops an SFX cue by ID.
type StopSoundCommand struct {
	ID voicedsp.SoundID
}

// StartCalibrationCommand begins a calibration session.
type StartCalibrationCommand struct {
	Config voicedsp.CalibrationConfig
}

// CancelCalibrationCommand cancels the current calibration session.
type CancelCalibrationCommand struct{}

// CommitCalibrationCommand persists a calibration profile.
type CommitCalibrationCommand struct {
	Calibration voicedsp.VowelCalibration
}

func (SetPresetCommand) voiceCommand()         {}
func (SetBypassCommand) voiceCommand()         {}
func (SetMonitorCommand) voiceCommand()        {}
func (SetInputGainCommand) voiceCommand()      {}
func (SetBusGainCommand) voiceCommand()        {}
func (SetEffectParamCommand) voiceCommand()    {}
func (PlaySoundCommand) voiceCommand()         {}
func (StopSoundCommand) voiceCommand()         {}
func (StartCalibrationCommand) voiceCommand()  {}
func (CancelCalibrationCommand) voiceCommand() {}
func (CommitCalibrationCommand) voiceCommand() {}
