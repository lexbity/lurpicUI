package marks

const meterType TypeName = "voice_meter"

type MeterMark struct {
	baseMark
	Label      string
	Value      float32
	Peak       float32
	Confidence float32
	Clipping   bool
	Muted      bool
}

func NewMeterMark(id string) *MeterMark {
	return &MeterMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionPrimitive,
				Type:              meterType,
			},
		},
	}
}

func init() {
	d := Descriptor{
		Family:            FamilyVoice,
		ConstructionClass: ConstructionPrimitive,
		Type:              meterType,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
