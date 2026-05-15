package marks

const vowelSpaceType TypeName = "voice_vowel_space"

type VowelSpaceMark struct {
	baseMark
	Title  string
	LiveF1 float32
	LiveF2 float32
}

func NewVowelSpaceMark(id string) *VowelSpaceMark {
	return &VowelSpaceMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              vowelSpaceType,
				HitTestable:       true,
			},
		},
	}
}

func init() {
	d := Descriptor{
		Family:            FamilyVoice,
		ConstructionClass: ConstructionComposed,
		Type:              vowelSpaceType,
		HitTestable:       true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
