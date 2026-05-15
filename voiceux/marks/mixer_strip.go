package marks

const mixerStripType TypeName = "voice_mixer_strip"

type MixerStripMark struct {
	baseMark
	Title string
	Buses []string
}

func NewMixerStripMark(id string) *MixerStripMark {
	return &MixerStripMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              mixerStripType,
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
		Type:              mixerStripType,
		Focusable:         true,
		HitTestable:       true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
