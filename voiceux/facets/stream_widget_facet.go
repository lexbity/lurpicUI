package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/projection"
)

type StreamWidgetFacet struct {
	baseVoiceFacet
}

func NewStreamWidgetFacet(service voiceux.VoiceService) *StreamWidgetFacet {
	f := &StreamWidgetFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 220, H: 120} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x11151BFF))})
		projection.MeterFromParams(bounds, f.currentParams()).AppendCommands(list)
	}})
	f.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		return f.Bounds().Contains(e.Position)
	}})
	f.AddRole(&facet.TickRole{})
	return f
}

func (f *StreamWidgetFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackChange(&f.Facet, &f.stores.Params.OnChange, facet.DirtyProjection, "voiceux.stream_widget.params")
	trackChange(&f.Facet, &f.stores.PipelineStatus.OnChange, facet.DirtyProjection, "voiceux.stream_widget.pipeline")
	trackChange(&f.Facet, &f.stores.MonitorEnabled.OnChange, facet.DirtyProjection, "voiceux.stream_widget.monitor")
}

func (f *StreamWidgetFacet) OnDetach()     {}
func (f *StreamWidgetFacet) OnActivate()   {}
func (f *StreamWidgetFacet) OnDeactivate() {}

func (f *StreamWidgetFacet) Snapshot(bounds gfx.Rect) projection.MeterSnapshot {
	return projection.MeterFromParams(bounds, f.currentParams())
}

func (f *StreamWidgetFacet) currentParams() voiceux.AudioParamsView {
	if f == nil || f.stores == nil || f.stores.Params == nil {
		return voiceux.AudioParamsView{}
	}
	return f.stores.Params.Get()
}
