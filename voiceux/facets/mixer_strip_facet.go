package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

type MixerStripFacet struct {
	baseVoiceFacet
}

func NewMixerStripFacet(service voiceux.VoiceService) *MixerStripFacet {
	f := &MixerStripFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 300, H: 220} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x1A2028FF))})
		for i, bus := range f.snapshot().Buses {
			_ = bus
			list.Add(gfx.FillRect{Rect: mixerBusRect(bounds, i), Brush: gfx.SolidBrush(gfx.ColorFromHex(0x2D3440FF))})
		}
	}})
	f.AddRole(&facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
		if f.Bounds().Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{}
	}})
	f.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		return f.Bounds().Contains(e.Position)
	}})
	return f
}

func (f *MixerStripFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackChange(&f.Facet, &f.stores.Mixer.OnChange, facet.DirtyProjection, "voiceux.mixer_strip.mixer")
	trackChange(&f.Facet, &f.stores.MonitorEnabled.OnChange, facet.DirtyProjection, "voiceux.mixer_strip.monitor")
}

func (f *MixerStripFacet) OnDetach()     {}
func (f *MixerStripFacet) OnActivate()   {}
func (f *MixerStripFacet) OnDeactivate() {}

func (f *MixerStripFacet) SetBusGain(bus voicedsp.BusName, gain float32) error {
	return f.dispatch(voiceux.SetBusGainCommand{Bus: bus, Gain: gain})
}

func (f *MixerStripFacet) SetMonitorEnabled(enabled bool) error {
	return f.dispatch(voiceux.SetMonitorCommand{Enabled: enabled})
}

func (f *MixerStripFacet) snapshot() voiceux.MixerStateView {
	if f == nil || f.stores == nil || f.stores.Mixer == nil {
		return voiceux.MixerStateView{}
	}
	return f.stores.Mixer.Get()
}

func mixerBusRect(bounds gfx.Rect, index int) gfx.Rect {
	width := bounds.Width() - 16
	height := float32(36)
	y := bounds.Min.Y + 8 + float32(index)*(height+6)
	return gfx.RectFromXYWH(bounds.Min.X+8, y, width, height)
}
