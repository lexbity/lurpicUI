package widgets

import (
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

// PresetCard is a reusable presentation helper for a preset entry.
type PresetCard struct {
	ID          voicedsp.PresetID
	Name        string
	Description string
	Tags        []string
	Selected    bool
	Enabled     bool
}

// ActivateCommand converts the card into a preset-selection command.
func (c PresetCard) ActivateCommand() voiceux.VoiceCommand {
	return voiceux.SetPresetCommand{ID: c.ID}
}
