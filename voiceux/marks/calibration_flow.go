package marks

const calibrationFlowType TypeName = "voice_calibration_flow"

type CalibrationFlowMark struct {
	baseMark
	Phase    string
	Progress float32
	Title    string
}

func NewCalibrationFlowMark(id string) *CalibrationFlowMark {
	return &CalibrationFlowMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              calibrationFlowType,
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
		Type:              calibrationFlowType,
		Focusable:         true,
		HitTestable:       true,
		ChildHosting:      true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
