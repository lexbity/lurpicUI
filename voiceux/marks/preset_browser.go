package marks

const presetBrowserType TypeName = "voice_preset_browser"

type PresetBrowserMark struct {
	baseMark
	Title   string
	Presets []string
	Active  string
	Filter  string
}

func NewPresetBrowserMark(id string) *PresetBrowserMark {
	return &PresetBrowserMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              presetBrowserType,
				Focusable:         true,
				HitTestable:       true,
			},
		},
	}
}

func init() {
	d := Descriptor{
		Family:            FamilyVoice,
		ConstructionClass: ConstructionComposed,
		Type:              presetBrowserType,
		Focusable:         true,
		HitTestable:       true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
