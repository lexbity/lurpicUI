package marks

const streamWidgetType TypeName = "voice_stream_widget"

type StreamWidgetMark struct {
	baseMark
	Title string
	Muted bool
}

func NewStreamWidgetMark(id string) *StreamWidgetMark {
	return &StreamWidgetMark{
		baseMark: baseMark{
			id: id,
			desc: Descriptor{
				Family:            FamilyVoice,
				ConstructionClass: ConstructionComposed,
				Type:              streamWidgetType,
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
		Type:              streamWidgetType,
		Focusable:         true,
		HitTestable:       true,
	}
	registerDescriptor(d)
	validateDescriptor(d)
}
