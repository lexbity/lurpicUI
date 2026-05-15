package facets

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/widgets"
	"codeburg.org/lexbit/voicedsp"
)

type PresetBrowserFacet struct {
	baseVoiceFacet
	filter string
}

func NewPresetBrowserFacet(service voiceux.VoiceService) *PresetBrowserFacet {
	f := &PresetBrowserFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 360, H: 260} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x171C23FF))})
		for i, card := range f.Cards() {
			if i >= 4 {
				break
			}
			_ = card
			list.Add(gfx.FillRect{Rect: presetCardRect(bounds, i), Brush: gfx.SolidBrush(gfx.ColorFromHex(0x242B36FF))})
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
	f.AddRole(&facet.FocusRole{Focusable: func() bool { return true }})
	return f
}

func (f *PresetBrowserFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackReplace(&f.Facet, f.stores.Presets, facet.DirtyAll, "voiceux.preset_browser.presets")
	trackChange(&f.Facet, &f.stores.ActivePreset.OnChange, facet.DirtyProjection, "voiceux.preset_browser.active_preset")
}

func (f *PresetBrowserFacet) OnDetach()     {}
func (f *PresetBrowserFacet) OnActivate()   {}
func (f *PresetBrowserFacet) OnDeactivate() {}

func (f *PresetBrowserFacet) SetFilter(filter string) {
	if f == nil {
		return
	}
	f.filter = strings.TrimSpace(strings.ToLower(filter))
	f.Invalidate(facet.DirtyProjection)
}

func (f *PresetBrowserFacet) Filter() string {
	if f == nil {
		return ""
	}
	return f.filter
}

func (f *PresetBrowserFacet) Cards() []widgets.PresetCard {
	if f == nil || f.stores == nil || f.stores.Presets == nil {
		return nil
	}
	active := voicedsp.PresetID("")
	if f.stores.ActivePreset != nil {
		active = f.stores.ActivePreset.Get()
	}
	out := make([]widgets.PresetCard, 0, f.stores.Presets.Len())
	for _, preset := range f.stores.Presets.All() {
		if f.filter != "" && !presetMatches(preset, f.filter) {
			continue
		}
		out = append(out, widgets.PresetCard{
			ID:          preset.ID,
			Name:        preset.Name,
			Description: preset.Description,
			Tags:        append([]string(nil), preset.Tags...),
			Selected:    preset.Selected || preset.ID == active,
			Enabled:     preset.Enabled,
		})
	}
	return out
}

func (f *PresetBrowserFacet) SelectPreset(id voicedsp.PresetID) error {
	return f.dispatch(voiceux.SetPresetCommand{ID: id})
}

func (f *PresetBrowserFacet) Snapshot() []voiceux.PresetView {
	if f == nil || f.stores == nil || f.stores.Presets == nil {
		return nil
	}
	out := append([]voiceux.PresetView(nil), f.stores.Presets.All()...)
	return out
}

func presetMatches(p voiceux.PresetView, filter string) bool {
	if filter == "" {
		return true
	}
	haystack := strings.ToLower(p.Name + " " + p.Description + " " + strings.Join(p.Tags, " "))
	return strings.Contains(haystack, filter)
}

func presetCardRect(bounds gfx.Rect, index int) gfx.Rect {
	w := bounds.Width() - 16
	h := float32(48)
	y := bounds.Min.Y + 8 + float32(index)*(h+8)
	return gfx.RectFromXYWH(bounds.Min.X+8, y, w, h)
}
