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

// DeviceRoutingFacet exposes standard select dropdowns for input/output selection.
type DeviceRoutingFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	host   *Host
	th     theme.Context
	shaper *text.Shaper

	inputBinding  store.Binding[string]
	outputBinding store.Binding[string]
	inputSelect   *uiinput.Select
	outputSelect  *uiinput.Select

	adder          facetChildAdder
	bounds         gfx.Rect
	syncing        bool
	inputSub       signal.SubscriptionID
	outputSub      signal.SubscriptionID
	devicesSub     signal.SubscriptionID
	selectedInput  signal.SubscriptionID
	selectedOutput signal.SubscriptionID
}

func NewDeviceRoutingFacet(host *Host, th theme.Context, shaper *text.Shaper) *DeviceRoutingFacet {
	f := &DeviceRoutingFacet{
		Facet:         facet.NewFacet(),
		host:          host,
		th:            th,
		shaper:        shaper,
		inputBinding:  store.NewBinding(""),
		outputBinding: store.NewBinding(""),
	}
	f.inputSelect = &uiinput.Select{
		ID:       "voiceqa-input-select",
		Variant:  uiinput.SelectStandard,
		Selected: f.inputBinding,
		Theme:    th,
		Shaper:   shaper,
	}
	f.outputSelect = &uiinput.Select{
		ID:       "voiceqa-output-select",
		Variant:  uiinput.SelectStandard,
		Selected: f.outputBinding,
		Theme:    th,
		Shaper:   shaper,
	}
	f.layout.OnMeasure = func(facet.Constraints) gfx.Size { return gfx.Size{W: 420, H: 300} }
	f.layout.OnArrange = func(bounds gfx.Rect) {
		f.bounds = bounds
	}
	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		f.renderPanel(list, bounds)
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	return f
}

func (f *DeviceRoutingFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func (f *DeviceRoutingFacet) OnAttach(ctx facet.AttachContext) {
	if adder, ok := ctx.Runtime.(facetChildAdder); ok {
		f.adder = adder
	}
	attachChild(&f.Facet, f, f.inputSelect, f.adder, layout.ChildAttachment{LayerID: 1})
	attachChild(&f.Facet, f, f.outputSelect, f.adder, layout.ChildAttachment{LayerID: 2})
	f.syncFromHost()
	if f.inputBinding.Store() != nil {
		f.inputSub = f.inputBinding.Store().OnChange.Subscribe(func(change signal.Change[string]) {
			if f.syncing {
				return
			}
			_ = f.host.DispatchAction("select_input", map[string]any{"id": change.New})
		})
	}
	if f.outputBinding.Store() != nil {
		f.outputSub = f.outputBinding.Store().OnChange.Subscribe(func(change signal.Change[string]) {
			if f.syncing {
				return
			}
			_ = f.host.DispatchAction("select_output", map[string]any{"id": change.New})
		})
	}
	if f.host != nil && f.host.Stores() != nil {
		f.devicesSub = f.host.Stores().Devices.OnChange.Subscribe(func(change signal.Change[voiceux.AudioDeviceSnapshot]) {
			f.syncFromHost()
		})
		f.selectedInput = f.host.Stores().SelectedInput.OnChange.Subscribe(func(change signal.Change[voicedsp.DeviceID]) {
			f.syncFromHost()
		})
		f.selectedOutput = f.host.Stores().SelectedOutput.OnChange.Subscribe(func(change signal.Change[voicedsp.DeviceID]) {
			f.syncFromHost()
		})
	}
}

func (f *DeviceRoutingFacet) OnDetach() {
	if f.host != nil && f.host.Stores() != nil {
		f.host.Stores().Devices.OnChange.Unsubscribe(f.devicesSub)
		f.host.Stores().SelectedInput.OnChange.Unsubscribe(f.selectedInput)
		f.host.Stores().SelectedOutput.OnChange.Unsubscribe(f.selectedOutput)
	}
	if f.inputBinding.Store() != nil {
		f.inputBinding.Store().OnChange.Unsubscribe(f.inputSub)
	}
	if f.outputBinding.Store() != nil {
		f.outputBinding.Store().OnChange.Unsubscribe(f.outputSub)
	}
}

func (f *DeviceRoutingFacet) OnActivate()   {}
func (f *DeviceRoutingFacet) OnDeactivate() {}

func (f *DeviceRoutingFacet) OnLayerSpecs() []layout.LayerSpec {
	bounds := f.bounds
	if bounds.IsEmpty() {
		return nil
	}
	padding := float32(12)
	titleH := float32(24)
	selectH := float32(56)
	inputLayerBounds := f.inputSelect.LayerBounds()
	outputLayerBounds := f.outputSelect.LayerBounds()
	wide := bounds.Width() - padding*2
	inputBounds := gfx.RectFromXYWH(bounds.Min.X+padding, bounds.Min.Y+padding+titleH, wide, maxFloat32(selectH, inputLayerBounds.Height()))
	outputY := inputBounds.Max.Y + 16
	outputBounds := gfx.RectFromXYWH(bounds.Min.X+padding, outputY, wide, maxFloat32(selectH, outputLayerBounds.Height()))
	return []layout.LayerSpec{
		{
			ID:          1,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: inputBounds},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 1,
			ClipPolicy:  layout.ClipToParent,
		},
		{
			ID:          2,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: outputBounds},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 2,
			ClipPolicy:  layout.ClipToParent,
		},
	}
}

func (f *DeviceRoutingFacet) renderPanel(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	if f.shaper == nil {
		return
	}
	y := bounds.Min.Y + 10
	f.drawText(list, bounds.Min.X+12, y, "Device Routing", theme.TextLabelM, theme.ColorText)
	y += 20
	f.drawText(list, bounds.Min.X+12, y, "Input", theme.TextBodyS, theme.ColorTextSecondary)
	y += 68
	f.drawText(list, bounds.Min.X+12, y, "Output", theme.TextBodyS, theme.ColorTextSecondary)
	list.Add(gfx.StrokeRect{Rect: bounds, Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorderStrong))})
}

func (f *DeviceRoutingFacet) drawText(list *gfx.CommandList, x, y float32, label string, style theme.TextToken, color theme.ColorToken) {
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

func (f *DeviceRoutingFacet) syncFromHost() {
	if f == nil || f.host == nil || f.host.Stores() == nil {
		return
	}
	f.syncing = true
	defer func() { f.syncing = false }()
	snap := f.host.Stores().Devices.Get()
	inputs := deviceOptions(snap.Inputs)
	outputs := deviceOptions(snap.Outputs)
	f.inputSelect.Options = inputs
	f.outputSelect.Options = outputs
	if selected := string(f.host.Stores().SelectedInput.Get()); selected != "" {
		f.inputBinding.Set(selected)
	} else if len(inputs) > 0 {
		f.inputBinding.Set(inputs[0].Key)
	}
	if selected := string(f.host.Stores().SelectedOutput.Get()); selected != "" {
		f.outputBinding.Set(selected)
	} else if len(outputs) > 0 {
		f.outputBinding.Set(outputs[0].Key)
	}
	f.inputSelect.Disabled = len(inputs) == 0
	f.outputSelect.Disabled = len(outputs) == 0
	if f.inputSelect.Selected.Store() != nil && f.inputSelect.Selected.Get() == "" && len(inputs) > 0 {
		f.inputSelect.Selected.Set(inputs[0].Key)
	}
	if f.outputSelect.Selected.Store() != nil && f.outputSelect.Selected.Get() == "" && len(outputs) > 0 {
		f.outputSelect.Selected.Set(outputs[0].Key)
	}
}

func deviceOptions(devices []voicedsp.DeviceInfo) []uiinput.SelectOption {
	if len(devices) == 0 {
		return nil
	}
	out := make([]uiinput.SelectOption, 0, len(devices))
	for _, device := range devices {
		label := strings.TrimSpace(device.Name)
		if label == "" {
			label = string(device.ID)
		}
		if device.Default {
			label = fmt.Sprintf("%s (default)", label)
		}
		out = append(out, uiinput.SelectOption{
			Key:   string(device.ID),
			Label: label,
		})
	}
	return out
}
