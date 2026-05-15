package marks

const deviceSelectorType TypeName = "voice_device_selector"

type DeviceSelectorMark struct {
	baseMark
	Title    string
	Devices  []string
	Selected string
}

func NewDeviceSelectorMark(id string) *DeviceSelectorMark {
	return &DeviceSelectorMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              deviceSelectorType,
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
		Type:              deviceSelectorType,
		Focusable:         true,
		HitTestable:       true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
