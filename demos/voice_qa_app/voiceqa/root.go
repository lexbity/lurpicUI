package voiceqa

import (
	"fmt"
	"runtime"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// RootFacet composes the Voice UX QA dashboard.
type RootFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	tick   facet.TickRole

	th     theme.Context
	shaper *text.Shaper
	host   *Host

	deviceSelector *DeviceRoutingFacet
	presetSelect   *PresetRoutingFacet

	adder          facetChildAdder
	frameRequester interface{ RequestFrame() }

	panelBounds struct {
		Header gfx.Rect
		Devices gfx.Rect
		Preset  gfx.Rect
	}

	errText string
}

// NewRootFacet builds the QA dashboard root.
func NewRootFacet(th theme.Context, shaper *text.Shaper, host *Host) *RootFacet {
	r := &RootFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
		host:   host,
	}
	r.deviceSelector = NewDeviceRoutingFacet(host, th, shaper)
	r.presetSelect = NewPresetRoutingFacet(host, th, shaper)

	r.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		w := c.MaxSize.W
		if w <= 0 {
			w = c.MinSize.W
		}
		if w <= 0 {
			w = 1
		}
		h := c.MaxSize.H
		if h <= 0 {
			h = c.MinSize.H
		}
		if h <= 0 {
			h = 1
		}
		return gfx.Size{W: w, H: h}
	}
	r.layout.OnArrange = func(bounds gfx.Rect) {
		r.layout.ArrangedBounds = bounds
		r.arrange(bounds)
	}
	r.AddRole(&r.layout)

	r.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		r.renderRoot(list, bounds)
	}
	r.AddRole(&r.render)

	r.tick.OnTick = func(dt time.Duration) {
		r.host.Sync()
		r.tick.RequestTick()
		r.RequestFrame()
	}
	r.AddRole(&r.tick)

	return r
}

func (r *RootFacet) Base() *facet.Facet {
	r.Facet.BindImpl(r)
	return &r.Facet
}

func (r *RootFacet) OnAttach(ctx facet.AttachContext) {
	if adder, ok := ctx.Runtime.(facetChildAdder); ok {
		r.adder = adder
	}
	if req, ok := ctx.Runtime.(interface{ RequestFrame() }); ok {
		r.frameRequester = req
	}
	attachChild(&r.Facet, r, r.deviceSelector, r.adder, layout.ChildAttachment{LayerID: 1})
	attachChild(&r.Facet, r, r.presetSelect, r.adder, layout.ChildAttachment{LayerID: 2})
	r.host.Sync()
}

func (r *RootFacet) OnDetach() {
	if r.host != nil {
		_ = r.host.Stop()
	}
}

func (r *RootFacet) OnActivate() {
	r.tick.RequestTick()
	r.host.Sync()
	r.RequestFrame()
}

func (r *RootFacet) OnDeactivate() {}

func (r *RootFacet) OnLayerSpecs() []layout.LayerSpec {
	bounds := r.layout.ArrangedBounds
	if bounds.IsEmpty() {
		return nil
	}
	r.arrange(bounds)
	return []layout.LayerSpec{
		layerSpec(1, r.panelBounds.Devices),
		layerSpec(2, r.panelBounds.Preset),
	}
}

func (r *RootFacet) arrange(bounds gfx.Rect) {
	margin := float32(18)
	gutter := float32(14)
	headerH := float32(98)
	innerW := bounds.Width() - margin*2
	leftX := bounds.Min.X + margin
	top := bounds.Min.Y + margin
	header := gfx.RectFromXYWH(leftX, top, innerW, headerH)
	contentY := top + headerH + gutter
	contentH := bounds.Height() - margin*2 - headerH - gutter
	if contentH < 0 {
		contentH = 0
	}
	devicesH := float32(220)
	presetH := maxFloat32(contentH-devicesH-gutter, 220)
	r.panelBounds.Header = header
	r.panelBounds.Devices = gfx.RectFromXYWH(leftX, contentY, innerW, devicesH)
	r.panelBounds.Preset = gfx.RectFromXYWH(leftX, contentY+devicesH+gutter, innerW, presetH)
}

func (r *RootFacet) renderRoot(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x0B0F14FF))})
	list.Add(gfx.FillRect{Rect: bounds.Inset(0, 0), Brush: gfx.SolidBrush(gfx.Color{R: 0.06, G: 0.08, B: 0.10, A: 0.35})})
	if r.shaper == nil {
		return
	}
	header := r.panelBounds.Header
	if header.IsEmpty() {
		header = gfx.RectFromXYWH(bounds.Min.X+16, bounds.Min.Y+16, bounds.Width()-32, 72)
	}
	r.drawCard(list, header, "Voice QA", "Select input, output, and preset. Changes apply live to the running pipeline.")
	r.drawText(list, header.Min.X+16, header.Min.Y+55, r.statusLine(), theme.TextBodyS, theme.ColorTextSecondary)
	if r.errText != "" {
		r.drawText(list, header.Min.X+16, header.Min.Y+72, r.errText, theme.TextBodyS, theme.ColorError)
	}
	r.drawSectionLabels(list)
}

func (r *RootFacet) statusLine() string {
	if r.host == nil || r.host.Stores() == nil {
		return runtime.GOOS + "/" + runtime.GOARCH
	}
	stores := r.host.Stores()
	mode := "real-audio"
	if r.host != nil && r.host.opts.FakeAudio {
		mode = "fake-audio"
	}
	status := mode
	if stores.PipelineStatus != nil {
		status = fmt.Sprintf("pipeline=%s", stores.PipelineStatus.Get().State)
	}
	if stores.ActivePreset != nil {
		status += fmt.Sprintf(" preset=%s", stores.ActivePreset.Get())
	}
	if stores.Devices != nil {
		snap := stores.Devices.Get()
		status += fmt.Sprintf(" inputs=%d outputs=%d", len(snap.Inputs), len(snap.Outputs))
	}
	status += fmt.Sprintf(" marks=%d facets=%d themes=%d", len(r.host.DescriptorRegistry().Marks()), len(r.host.DescriptorRegistry().Facets()), len(r.host.DescriptorRegistry().ThemeSlots()))
	return status
}

func (r *RootFacet) drawSectionLabels(list *gfx.CommandList) {
	sections := []struct {
		bounds gfx.Rect
		title  string
		sub    string
	}{
		{r.panelBounds.Devices, "Device Routing", "input and output devices"},
		{r.panelBounds.Preset, "Preset to Test", "select the active DSP preset"},
	}
	for _, section := range sections {
		if section.bounds.IsEmpty() {
			continue
		}
		r.drawCardHeader(list, section.bounds, section.title, section.sub)
	}
}

func (r *RootFacet) drawCard(list *gfx.CommandList, bounds gfx.Rect, title, subtitle string) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x10151DCC))})
	list.Add(gfx.StrokeRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x283140FF))})
	r.drawText(list, bounds.Min.X+16, bounds.Min.Y+24, title, theme.TextHeadingS, theme.ColorText)
	if subtitle != "" {
		r.drawText(list, bounds.Min.X+16, bounds.Min.Y+46, subtitle, theme.TextBodyS, theme.ColorTextSecondary)
	}
}

func (r *RootFacet) drawCardHeader(list *gfx.CommandList, bounds gfx.Rect, title, subtitle string) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	// Small label band so each area reads like a dashboard section even when the child facet is sparse.
	band := gfx.RectFromXYWH(bounds.Min.X+1, bounds.Min.Y+1, maxFloat32(bounds.Width()-2, 0), 38)
	list.Add(gfx.FillRect{Rect: band, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x131A23CC))})
	list.Add(gfx.StrokeRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x263240FF))})
	r.drawText(list, bounds.Min.X+12, bounds.Min.Y+12, title, theme.TextLabelM, theme.ColorText)
	if subtitle != "" {
		r.drawText(list, bounds.Min.X+12, bounds.Min.Y+28, subtitle, theme.TextBodyS, theme.ColorTextSecondary)
	}
}

func (r *RootFacet) drawText(list *gfx.CommandList, x, y float32, text string, token theme.TextToken, color theme.ColorToken) {
	if r.shaper == nil || text == "" {
		return
	}
	layout := r.shaper.ShapeSimple(text, r.th.TextStyle(token))
	if layout == nil || len(layout.Lines) == 0 {
		return
	}
	line := layout.Lines[0]
	origin := gfx.Point{X: x, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{Run: run, Origin: origin, Brush: gfx.SolidBrush(r.th.Color(color))})
	}
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func (r *RootFacet) RequestFrame() {
	if r == nil || r.frameRequester == nil {
		return
	}
	r.frameRequester.RequestFrame()
}

func layerSpec(id int, bounds gfx.Rect) layout.LayerSpec {
	return layout.LayerSpec{
		ID:          layout.LayerID(id),
		Placement:   layout.PlacementFree,
		Measurement: layout.MeasureNonStructural,
		CoordSpace:  layout.CoordParentLayout,
		CoordLimits: layout.CoordLimits{Bounds: bounds},
		HitPolicy:   layout.HitNormal,
		RenderOrder: id,
		ClipPolicy:  layout.ClipToParent,
	}
}

type facetChildAdder interface {
	AddFacet(parent, child facet.FacetImpl, attachment layout.ChildAttachment)
}

func attachChild(parent *facet.Facet, parentImpl facet.FacetImpl, child facet.FacetImpl, adder facetChildAdder, attachment layout.ChildAttachment) {
	if parent == nil || child == nil {
		return
	}
	if adder != nil {
		adder.AddFacet(parentImpl, child, attachment)
		return
	}
	parent.AddChildRuntime(child.Base())
}
