package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/input"
	"codeburg.org/lexbit/voicedsp"
)

type FXChainFacet struct {
	baseVoiceFacet
	drag input.FXDragState
}

func NewFXChainFacet(service voiceux.VoiceService) *FXChainFacet {
	f := &FXChainFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 380, H: 280} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x1A1F27FF))})
		for i, slot := range f.slots() {
			list.Add(gfx.FillRect{Rect: fxSlotRect(bounds, i), Brush: fxSlotBrush(slot.Enabled)})
		}
	}})
	f.AddRole(&facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
		if f.Bounds().Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorGrab}
		}
		return facet.HitResult{}
	}})
	f.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		return f.Bounds().Contains(e.Position)
	}})
	f.AddRole(&facet.FocusRole{Focusable: func() bool { return true }})
	return f
}

func (f *FXChainFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackReplace(&f.Facet, f.stores.FXChain, facet.DirtyAll, "voiceux.fx_chain.chain")
	trackChange(&f.Facet, &f.stores.FXBypassed.OnChange, facet.DirtyProjection, "voiceux.fx_chain.bypass")
}

func (f *FXChainFacet) OnDetach()     {}
func (f *FXChainFacet) OnActivate()   {}
func (f *FXChainFacet) OnDeactivate() {}

func (f *FXChainFacet) SetEffectParam(effect voicedsp.EffectID, param voicedsp.ParameterID, value float32) error {
	return f.dispatch(voiceux.SetEffectParamCommand{Effect: effect, Param: param, Value: value})
}

func (f *FXChainFacet) ToggleBypass(enabled bool) error {
	return f.dispatch(voiceux.SetBypassCommand{Enabled: enabled})
}

func (f *FXChainFacet) Reorder(from, to int) error {
	if f.stores != nil && f.stores.FXChain != nil {
		items := f.stores.FXChain.All()
		items = input.Reorder(items, from, to)
		f.stores.FXChain.Replace(items)
	}
	f.drag = input.FXDragState{From: from, To: to}
	return nil
}

func (f *FXChainFacet) slots() []voiceux.FXSlotView {
	if f == nil || f.stores == nil || f.stores.FXChain == nil {
		return nil
	}
	return f.stores.FXChain.All()
}

func fxSlotRect(bounds gfx.Rect, index int) gfx.Rect {
	width := bounds.Width() - 16
	height := float32(44)
	y := bounds.Min.Y + 8 + float32(index)*(height+6)
	return gfx.RectFromXYWH(bounds.Min.X+8, y, width, height)
}

func fxSlotBrush(enabled bool) gfx.Brush {
	if enabled {
		return gfx.SolidBrush(gfx.ColorFromHex(0x394B5DFF))
	}
	return gfx.SolidBrush(gfx.ColorFromHex(0x222831FF))
}
