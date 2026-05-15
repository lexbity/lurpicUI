package marks

const fxChainType TypeName = "voice_fx_chain"

type FXChainMark struct {
	baseMark
	Title string
	Slots []string
}

func NewFXChainMark(id string) *FXChainMark {
	return &FXChainMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              fxChainType,
				Focusable:         true,
				HitTestable:       true,
				ChildHosting:      true,
			},
		},
	}
}

func init() {
	d := Descriptor{
		Family:            FamilyVoice,
		ConstructionClass: ConstructionComposed,
		Type:              fxChainType,
		Focusable:         true,
		HitTestable:       true,
		ChildHosting:      true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
