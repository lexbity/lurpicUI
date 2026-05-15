package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/projection"
)

type MeterFacet struct {
	baseVoiceFacet
}

func NewMeterFacet(service voiceux.VoiceService) *MeterFacet {
	f := &MeterFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 240, H: 96} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		projection.MeterFromParams(bounds, f.currentParams()).AppendCommands(list)
	}})
	f.AddRole(&facet.TickRole{})
	return f
}

func (f *MeterFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackChange(&f.Facet, &f.stores.Params.OnChange, facet.DirtyProjection, "voiceux.meter.params")
	trackChange(&f.Facet, &f.stores.PipelineStatus.OnChange, facet.DirtyProjection, "voiceux.meter.pipeline")
	trackReplace(&f.Facet, f.stores.Diagnostics, facet.DirtyProjection, "voiceux.meter.diagnostics")
}

func (f *MeterFacet) OnDetach()     {}
func (f *MeterFacet) OnActivate()   {}
func (f *MeterFacet) OnDeactivate() {}

// Snapshot returns the current meter projection for tests and higher-level UI composition.
func (f *MeterFacet) Snapshot(bounds gfx.Rect) projection.MeterSnapshot {
	return projection.MeterFromParams(bounds, f.currentParams())
}

func (f *MeterFacet) currentParams() voiceux.AudioParamsView {
	if f == nil || f.stores == nil || f.stores.Params == nil {
		return voiceux.AudioParamsView{}
	}
	return f.stores.Params.Get()
}
