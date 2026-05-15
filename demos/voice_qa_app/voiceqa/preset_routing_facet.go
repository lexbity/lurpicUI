package voiceqa

import (
	"fmt"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/uiinput"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

// PresetRoutingFacet exposes a standard select dropdown for preset selection.
type PresetRoutingFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	host   *Host
	th     theme.Context
	shaper *text.Shaper

	presetBinding store.Binding[string]
	presetSelect  *uiinput.Select

	adder           facetChildAdder
	bounds          gfx.Rect
	syncing         bool
	presetsSub      signal.SubscriptionID
	activePresetSub signal.SubscriptionID
}

func NewPresetRoutingFacet(host *Host, th theme.Context, shaper *text.Shaper) *PresetRoutingFacet {
	f := &PresetRoutingFacet{
		Facet:         facet.NewFacet(),
		host:          host,
		th:            th,
		shaper:        shaper,
		presetBinding: store.NewBinding(""),
	}
	f.presetSelect = &uiinput.Select{
		ID:       "voiceqa-preset-select",
		Variant:  uiinput.SelectStandard,
		Selected: f.presetBinding,
		Theme:    th,
		Shaper:   shaper,
	}
	f.layout.OnMeasure = func(facet.Constraints) gfx.Size { return gfx.Size{W: 420, H: 220} }
	f.layout.OnArrange = func(bounds gfx.Rect) { f.bounds = bounds }
	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) { f.renderPanel(list, bounds) }
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	return f
}

func (f *PresetRoutingFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func (f *PresetRoutingFacet) OnAttach(ctx facet.AttachContext) {
	if adder, ok := ctx.Runtime.(facetChildAdder); ok {
		f.adder = adder
	}
	attachChild(&f.Facet, f, f.presetSelect, f.adder, layout.ChildAttachment{LayerID: 10})
	f.syncFromHost()
	if f.presetBinding.Store() != nil {
		f.presetsSub = f.presetBinding.Store().OnChange.Subscribe(func(change signal.Change[string]) {
			if f.syncing {
				return
			}
			if change.New == "" {
				return
			}
			_ = f.host.DispatchVoiceCommand(voiceux.SetPresetCommand{ID: voicedsp.PresetID(change.New)})
		})
	}
	if f.host != nil && f.host.Stores() != nil {
		f.activePresetSub = f.host.Stores().ActivePreset.OnChange.Subscribe(func(change signal.Change[voicedsp.PresetID]) {
			f.syncFromHost()
		})
	}
}

func (f *PresetRoutingFacet) OnDetach() {
	if f.host != nil && f.host.Stores() != nil {
		f.host.Stores().ActivePreset.OnChange.Unsubscribe(f.activePresetSub)
	}
	if f.presetBinding.Store() != nil {
		f.presetBinding.Store().OnChange.Unsubscribe(f.presetsSub)
	}
}

func (f *PresetRoutingFacet) OnActivate()   {}
func (f *PresetRoutingFacet) OnDeactivate() {}

func (f *PresetRoutingFacet) OnLayerSpecs() []layout.LayerSpec {
	bounds := f.bounds
	if bounds.IsEmpty() {
		return nil
	}
	padding := float32(12)
	titleH := float32(24)
	selectH := float32(56)
	wide := bounds.Width() - padding*2
	selectBounds := gfx.RectFromXYWH(bounds.Min.X+padding, bounds.Min.Y+padding+titleH, wide, maxFloat32(selectH, f.presetSelect.LayerBounds().Height()))
	return []layout.LayerSpec{
		{
			ID:          10,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: selectBounds},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 10,
			ClipPolicy:  layout.ClipToParent,
		},
	}
}

func (f *PresetRoutingFacet) renderPanel(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	if f.shaper == nil {
		return
	}
	y := bounds.Min.Y + 10
	f.drawText(list, bounds.Min.X+12, y, "Preset to Test", theme.TextLabelM, theme.ColorText)
	list.Add(gfx.StrokeRect{Rect: bounds, Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorderStrong))})
}

func (f *PresetRoutingFacet) drawText(list *gfx.CommandList, x, y float32, label string, style theme.TextToken, color theme.ColorToken) {
	if f.shaper == nil || label == "" {
		return
	}
	layout := f.shaper.ShapeSimple(label, f.th.TextStyle(style))
	if layout == nil || len(layout.Lines) == 0 {
		return
	}
	line := layout.Lines[0]
	origin := gfx.Point{X: x, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{Run: run, Origin: origin, Brush: gfx.SolidBrush(f.th.Color(color))})
	}
}

func (f *PresetRoutingFacet) syncFromHost() {
	if f == nil || f.host == nil || f.host.Stores() == nil {
		return
	}
	f.syncing = true
	defer func() { f.syncing = false }()
	presets := f.host.Stores().Presets.All()
	options := presetOptions(presets)
	f.presetSelect.Options = options
	if selected := string(f.host.Stores().ActivePreset.Get()); selected != "" {
		f.presetBinding.Set(selected)
	} else if len(options) > 0 {
		f.presetBinding.Set(options[0].Key)
	}
	f.presetSelect.Disabled = len(options) == 0
	if f.presetSelect.Selected.Store() != nil && f.presetSelect.Selected.Get() == "" && len(options) > 0 {
		f.presetSelect.Selected.Set(options[0].Key)
	}
}

func presetOptions(presets []voiceux.PresetView) []uiinput.SelectOption {
	if len(presets) == 0 {
		return nil
	}
	out := make([]uiinput.SelectOption, 0, len(presets))
	for _, preset := range presets {
		label := strings.TrimSpace(preset.Name)
		if label == "" {
			label = string(preset.ID)
		}
		if preset.Selected {
			label = fmt.Sprintf("%s (active)", label)
		}
		out = append(out, uiinput.SelectOption{
			Key:   string(preset.ID),
			Label: label,
		})
	}
	return out
}
