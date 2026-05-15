package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/projection"
)

type VowelSpaceFacet struct {
	baseVoiceFacet
}

func NewVowelSpaceFacet(service voiceux.VoiceService) *VowelSpaceFacet {
	f := &VowelSpaceFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 360, H: 300} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		projection.VowelSpaceFromState(bounds, f.currentParams(), f.calibrationState()).AppendCommands(list)
	}})
	f.AddRole(&facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
		if f.Bounds().Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorCrosshair}
		}
		return facet.HitResult{}
	}})
	f.AddRole(&facet.ViewportRole{})
	f.AddRole(&facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
		var list gfx.CommandList
		projection.VowelSpaceFromState(ctx.Bounds, f.currentParams(), f.calibrationState()).AppendCommands(&list)
		return &list
	}})
	return f
}

func (f *VowelSpaceFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackChange(&f.Facet, &f.stores.Calibration.OnChange, facet.DirtyProjection, "voiceux.vowel_space.calibration")
	trackChange(&f.Facet, &f.stores.Params.OnChange, facet.DirtyProjection, "voiceux.vowel_space.params")
}

func (f *VowelSpaceFacet) OnDetach()     {}
func (f *VowelSpaceFacet) OnActivate()   {}
func (f *VowelSpaceFacet) OnDeactivate() {}

func (f *VowelSpaceFacet) Projection(bounds gfx.Rect) projection.VowelSpaceSnapshot {
	return projection.VowelSpaceFromState(bounds, f.currentParams(), f.calibrationState())
}

func (f *VowelSpaceFacet) currentParams() voiceux.AudioParamsView {
	if f == nil || f.stores == nil || f.stores.Params == nil {
		return voiceux.AudioParamsView{}
	}
	return f.stores.Params.Get()
}

func (f *VowelSpaceFacet) calibrationState() voiceux.CalibrationStateView {
	if f == nil || f.stores == nil || f.stores.Calibration == nil {
		return voiceux.CalibrationStateView{}
	}
	return f.stores.Calibration.Get()
}
