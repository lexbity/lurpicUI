package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/marks/chart"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/marks/uiinput"
	"codeburg.org/lexbit/lurpicui/marks/uinav"
	"codeburg.org/lexbit/lurpicui/marks/uinotification"
	"codeburg.org/lexbit/lurpicui/signal"
	bindingstore "codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	catalogstore "codeburg.org/lexbit/ui_catalog/store"
)

const (
	previewHeaderHeight                = 28
	previewPanelMinH                   = 180
	previewPanelMaxH                   = 320
	previewSceneLayerID layout.LayerID = 1
)

func detailPreviewBounds(bounds gfx.Rect, profile LayoutProfile) gfx.Rect {
	_ = profile
	if bounds.IsEmpty() {
		return gfx.Rect{}
	}
	height := bounds.Height() * 0.42
	if height < previewPanelMinH {
		height = previewPanelMinH
	}
	if height > previewPanelMaxH {
		height = previewPanelMaxH
	}
	if height > bounds.Height() {
		height = bounds.Height()
	}
	if height <= 0 {
		return gfx.Rect{}
	}
	return gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), height)
}

func detailPreviewSceneBounds(bounds gfx.Rect, profile LayoutProfile) gfx.Rect {
	_ = profile
	if bounds.IsEmpty() {
		return gfx.Rect{}
	}
	header := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), previewHeaderHeight)
	top := header.Max.Y + 8
	bottom := bounds.Max.Y - 8
	if bottom <= top {
		return gfx.Rect{}
	}
	return gfx.RectFromXYWH(bounds.Min.X+8, top, bounds.Width()-16, bottom-top)
}

// PreviewFacet shows live authored marks for the currently selected catalog entry.
type PreviewFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	hit    facet.HitRole

	th     theme.Context
	shaper *text.Shaper

	layoutProfile LayoutProfile

	selectionSub signal.SubscriptionID
	filterSub    signal.SubscriptionID
	runtime      previewRuntime

	sceneChildren []facet.FacetImpl
	sceneKey      string
	bounds        gfx.Rect
}

type previewRuntime interface {
	AddFacet(parent, child facet.FacetImpl, attachment layout.ChildAttachment)
	RemoveFacet(child facet.FacetImpl)
	RequestFrame()
}

func NewPreviewFacet(th theme.Context, shaper *text.Shaper) *PreviewFacet {
	p := &PreviewFacet{
		Facet:         facet.NewFacet(),
		th:            th,
		shaper:        shaper,
		layoutProfile: DefaultLayoutProfile(),
	}

	p.layout.OnMeasure = func(cons facet.Constraints) gfx.Size {
		profile := p.layoutProfile
		if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
			profile = DefaultLayoutProfile()
		}
		if cons.MaxSize.W <= 0 {
			cons.MaxSize.W = profile.CardWidth * 2
		}
		if cons.MaxSize.H <= 0 {
			cons.MaxSize.H = previewPanelMinH
		}
		return gfx.Size{W: cons.MaxSize.W, H: cons.MaxSize.H}
	}
	p.layout.OnArrange = func(bounds gfx.Rect) {
		p.bounds = bounds
		p.layout.ArrangedBounds = bounds
		p.syncScene()
	}
	p.AddRole(&p.layout)

	p.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		p.renderPreview(list, bounds)
	}
	p.AddRole(&p.render)

	p.hit.OnHitTest = func(pt gfx.Point) facet.HitResult {
		if p.previewSceneBounds().Contains(pt) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
		}
		return facet.HitResult{}
	}
	p.AddRole(&p.hit)

	return p
}

func (p *PreviewFacet) Base() *facet.Facet {
	p.Facet.BindImpl(p)
	return &p.Facet
}

func (p *PreviewFacet) OnAttach(ctx facet.AttachContext) {
	if rt, ok := ctx.Runtime.(previewRuntime); ok {
		p.runtime = rt
	}
	p.selectionSub = catalogstore.SelectionStore.OnChange.Subscribe(func(change signal.Change[string]) {
		p.syncScene()
		p.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})
	p.filterSub = catalogstore.FilterStore.OnChange.Subscribe(func(change signal.Change[catalogstore.FilterState]) {
		p.syncScene()
		p.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})
	p.syncScene()
}

func (p *PreviewFacet) OnDetach() {
	catalogstore.SelectionStore.OnChange.Unsubscribe(p.selectionSub)
	catalogstore.FilterStore.OnChange.Unsubscribe(p.filterSub)
	p.clearScene()
}

func (p *PreviewFacet) OnActivate()   {}
func (p *PreviewFacet) OnDeactivate() {}

func (p *PreviewFacet) SetLayoutProfile(profile LayoutProfile) {
	if p == nil {
		return
	}
	p.layoutProfile = profile
	p.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	p.syncScene()
}

func (p *PreviewFacet) OnLayerSpecs() []layout.LayerSpec {
	scene := p.previewSceneBounds()
	if scene.IsEmpty() {
		return nil
	}
	return []layout.LayerSpec{{
		ID:          1,
		Placement:   layout.PlacementFree,
		Measurement: layout.MeasureStructural,
		CoordSpace:  layout.CoordParentLayout,
		CoordLimits: layout.CoordLimits{Bounds: scene},
		HitPolicy:   layout.HitNormal,
		RenderOrder: 0,
		ClipPolicy:  layout.ClipToParent,
	}}
}

func (p *PreviewFacet) syncScene() {
	if p == nil || p.runtime == nil {
		return
	}
	entry, ok := catalogstore.SelectedEntry(catalogstore.CatalogInstance)
	key := ""
	if ok && entry != nil {
		key = entry.ID
	}
	if key == p.sceneKey {
		return
	}
	p.clearScene()
	p.sceneKey = key
	if entry == nil {
		return
	}
	children := p.buildSceneChildren(entry)
	p.sceneChildren = children
	for i, child := range children {
		if child == nil {
			continue
		}
		p.runtime.AddFacet(p, child, previewChildAttachment(i))
	}
	p.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	p.RequestFrame()
}

func (p *PreviewFacet) clearScene() {
	if p.runtime != nil {
		for _, child := range p.sceneChildren {
			if child != nil {
				p.runtime.RemoveFacet(child)
			}
		}
	}
	p.sceneChildren = nil
}

func (p *PreviewFacet) RequestFrame() {
	if p.runtime != nil {
		p.runtime.RequestFrame()
	}
}

func (p *PreviewFacet) previewHeaderBounds() gfx.Rect {
	return gfx.RectFromXYWH(p.bounds.Min.X, p.bounds.Min.Y, p.bounds.Width(), previewHeaderHeight)
}

func (p *PreviewFacet) previewSceneBounds() gfx.Rect {
	return detailPreviewSceneBounds(p.bounds, p.layoutProfile)
}

func (p *PreviewFacet) renderPreview(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	bg := p.th.Color(theme.ColorSurface)
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(bg)})

	header := p.previewHeaderBounds()
	if !header.IsEmpty() {
		list.Add(gfx.FillRect{Rect: header, Brush: gfx.SolidBrush(p.th.Color(theme.ColorSurfaceVariant))})
		label := p.previewTitle()
		if p.shaper != nil {
			layout := p.shaper.ShapeSimple(label, p.th.TextStyle(theme.TextLabelS))
			if layout != nil && len(layout.Lines) > 0 {
				line := layout.Lines[0]
				drawTextLine(list, header.Min.X+8, header.Min.Y+6, line, p.th.Color(theme.ColorTextSecondary))
			}
		}
	}

	scene := p.previewSceneBounds()
	if !scene.IsEmpty() {
		list.Add(gfx.StrokeRect{Rect: scene, Brush: gfx.SolidBrush(p.th.Color(theme.ColorBorder))})
	}

	entry, ok := catalogstore.SelectedEntry(catalogstore.CatalogInstance)
	if !ok || entry == nil {
		if p.shaper != nil {
			msg := "Select an entry to preview a live mark"
			layout := p.shaper.ShapeSimple(msg, p.th.TextStyle(theme.TextBodyS))
			if layout != nil && len(layout.Lines) > 0 {
				line := layout.Lines[0]
				x := bounds.Min.X + (bounds.Width()-line.Bounds.Width())/2
				y := bounds.Min.Y + bounds.Height()/2
				drawTextLine(list, x, y, line, p.th.Color(theme.ColorTextSecondary))
			}
		}
	}
}

func (p *PreviewFacet) previewTitle() string {
	entry, ok := catalogstore.SelectedEntry(catalogstore.CatalogInstance)
	if !ok || entry == nil {
		return "Live Preview"
	}
	return fmt.Sprintf("Live Preview: %s", entry.ID)
}

func (p *PreviewFacet) buildSceneChildren(entry *model.CatalogEntry) []facet.FacetImpl {
	if entry == nil {
		return nil
	}
	var roots []facet.FacetImpl
	switch entry.ID {
	case "basic.rect":
		roots = p.buildBasicRectPreview(entry)
	case "basic.ellipse":
		roots = p.buildBasicEllipsePreview(entry)
	case "basic.polygon":
		roots = p.buildBasicPolygonPreview(entry)
	case "basic.polyline":
		roots = p.buildBasicPolylinePreview(entry)
	case "basic.line":
		roots = p.buildBasicLinePreview(entry)
	case "basic.path":
		roots = p.buildBasicPathPreview(entry)
	case "basic.image":
		roots = p.buildBasicImagePreview(entry)
	case "basic.text":
		roots = p.buildBasicTextPreview(entry)
	case "structure.group":
		roots = p.buildStructureGroupPreview(entry)
	case "structure.clip":
		roots = p.buildStructureClipPreview(entry)
	case "structure.transform":
		roots = p.buildStructureTransformPreview(entry)
	case "structure.layer":
		roots = p.buildStructureLayerPreview(entry)
	case "structure.viewport":
		roots = p.buildStructureViewportPreview(entry)
	case "structure.anchor":
		roots = p.buildStructureAnchorPreview(entry)
	case "annotation.area":
		roots = p.buildAnnotationAreaPreview(entry)
	case "annotation.badge":
		roots = p.buildAnnotationBadgePreview(entry)
	case "annotation.callout":
		roots = p.buildAnnotationCalloutPreview(entry)
	case "annotation.connector":
		roots = p.buildAnnotationConnectorPreview(entry)
	case "annotation.handle":
		roots = p.buildAnnotationHandlePreview(entry)
	case "annotation.icon":
		roots = p.buildAnnotationIconPreview(entry)
	case "annotation.label":
		roots = p.buildAnnotationLabelPreview(entry)
	case "annotation.rule":
		roots = p.buildAnnotationRulePreview(entry)
	case "annotation.symbol":
		roots = p.buildAnnotationSymbolPreview(entry)
	case "uiinput.button":
		roots = p.buildUIInputButtonPreview(entry)
	case "uiinput.checkbox":
		roots = p.buildUIInputCheckboxPreview(entry)
	case "uiinput.radiogroup":
		roots = p.buildUIInputRadioGroupPreview(entry)
	case "uiinput.slider":
		roots = p.buildUIInputSliderPreview(entry)
	case "uiinput.select":
		roots = p.buildUIInputSelectPreview(entry)
	case "uiinput.switch":
		roots = p.buildUIInputSwitchPreview(entry)
	case "uiinput.textinput":
		roots = p.buildUIInputTextInputPreview(entry)
	case "uinav.menu":
		roots = p.buildUINavMenuPreview(entry)
	case "uinav.breadcrumbs":
		roots = p.buildUINavBreadcrumbsPreview(entry)
	case "uinav.drawer":
		roots = p.buildUINavDrawerPreview(entry)
	case "uinav.pagination":
		roots = p.buildUINavPaginationPreview(entry)
	case "uinav.scrollbar":
		roots = p.buildUINavScrollbarPreview(entry)
	case "uinav.speeddial":
		roots = p.buildUINavSpeedDialPreview(entry)
	case "uinav.tabs":
		roots = p.buildUINavTabsPreview(entry)
	case "uinotification.dialog":
		roots = p.buildUINotificationDialogPreview(entry)
	case "uinotification.progress":
		roots = p.buildUINotificationProgressPreview(entry)
	case "uinotification.snackbar":
		roots = p.buildUINotificationSnackbarPreview(entry)
	case "chart.axis":
		roots = p.buildChartAxisPreview(entry)
	}
	if len(roots) == 0 {
		switch entry.Family {
		case model.FamilyBasic:
			roots = p.buildBasicPreview(entry)
		case model.FamilyStructure:
			roots = p.buildStructurePreview(entry)
		case model.FamilyAnnotation:
			roots = p.buildAnnotationPreview(entry)
		case model.FamilyUIInput:
			roots = p.buildUIInputPreview(entry)
		case model.FamilyUINav:
			roots = p.buildUINavPreview(entry)
		case model.FamilyUINotification:
			roots = p.buildUINotificationPreview(entry)
		case model.FamilyChart:
			roots = p.buildChartPreview(entry)
		default:
			roots = p.buildBasicPreview(entry)
		}
	}
	return roots
}

func (p *PreviewFacet) buildBasicRectPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&basic.Rect{
			ID:     sceneID(entry.ID, "rect"),
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 144, H: 92},
			Radius: 14,
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorPrimary), p.th.Color(theme.ColorBorder), 0),
				Stroke:  theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorBorder), Opacity: 1}, Width: 1},
				Opacity: 1,
				Visible: true,
			},
		},
	}
}

func (p *PreviewFacet) buildBasicEllipsePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&basic.Ellipse{
			ID:     sceneID(entry.ID, "ellipse"),
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 128, H: 128},
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorPrimary), p.th.Color(theme.ColorBorder), 0),
				Stroke:  theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorBorder), Opacity: 1}, Width: 1},
				Opacity: 1,
				Visible: true,
			},
		},
	}
}

func (p *PreviewFacet) buildBasicPolygonPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&basic.Polygon{
			ID: sceneID(entry.ID, "polygon"),
			Points: []gfx.Point{
				{X: 12, Y: 88},
				{X: 64, Y: 12},
				{X: 116, Y: 88},
				{X: 84, Y: 116},
				{X: 44, Y: 116},
			},
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorSurfaceVariant), p.th.Color(theme.ColorBorder), 0),
				Stroke:  theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorBorder), Opacity: 1}, Width: 2},
				Opacity: 1,
				Visible: true,
			},
		},
	}
}

func (p *PreviewFacet) buildBasicPolylinePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&basic.Polyline{
			ID: sceneID(entry.ID, "polyline"),
			Points: []gfx.Point{
				{X: 12, Y: 18},
				{X: 34, Y: 96},
				{X: 68, Y: 44},
				{X: 112, Y: 104},
			},
			Stroke: theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorPrimary), Opacity: 1}, Width: 3},
		},
	}
}

func (p *PreviewFacet) buildBasicLinePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&basic.Line{
			ID:     sceneID(entry.ID, "line"),
			Start:  gfx.Point{X: 12, Y: 100},
			End:    gfx.Point{X: 136, Y: 20},
			Stroke: theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorPrimary), Opacity: 1}, Width: 4},
		},
	}
}

func (p *PreviewFacet) buildBasicPathPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	path := gfx.NewPath().
		MoveTo(gfx.Point{X: 8, Y: 92}).
		CubicTo(gfx.Point{X: 28, Y: 8}, gfx.Point{X: 92, Y: 8}, gfx.Point{X: 112, Y: 68}).
		CubicTo(gfx.Point{X: 124, Y: 104}, gfx.Point{X: 156, Y: 112}, gfx.Point{X: 168, Y: 72}).
		Close().Build()
	return []facet.FacetImpl{
		&basic.Path{
			ID:   sceneID(entry.ID, "path"),
			Path: path,
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorSurfaceVariant), p.th.Color(theme.ColorBorder), 0),
				Stroke:  theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorPrimary), Opacity: 1}, Width: 2},
				Opacity: 1,
				Visible: true,
			},
		},
	}
}

func (p *PreviewFacet) buildBasicImagePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	img := image.NewRGBA(image.Rect(0, 0, 96, 72))
	for y := 0; y < 72; y++ {
		for x := 0; x < 96; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(30 + x*2), G: uint8(60 + y*2), B: uint8(120), A: 255})
		}
	}
	return []facet.FacetImpl{
		&basic.Image{
			ID:      sceneID(entry.ID, "image"),
			Source:  basic.RGBAImageSource{ImageRef: img, Key: entry.ID},
			Bounds:  basic.BoundsProps{X: 0, Y: 0, W: 144, H: 108},
			Fit:     basic.FitContain,
			Clip:    true,
			Opacity: 1,
		},
	}
}

func (p *PreviewFacet) buildBasicTextPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&basic.Text{
			ID: entry.ID,
			Paragraph: text.Paragraph{Spans: []text.TextSpan{
				{Text: "The quick brown fox jumps over the lazy dog.", Style: p.th.TextStyle(theme.TextBodyM)},
				{Text: "\n", Style: p.th.TextStyle(theme.TextBodyM)},
				{Text: shortLabel(entry.DisplayName), Style: p.th.TextStyle(theme.TextLabelM)},
			}},
			Style:      p.th.TextStyle(theme.TextBodyM),
			MaxWidth:   220,
			Align:      text.AlignLeft,
			Selectable: true,
		},
	}
}

func (p *PreviewFacet) buildBasicPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	rectID := sceneID(entry.ID, "rect")
	textID := sceneID(entry.ID, "text")
	return []facet.FacetImpl{
		&basic.Rect{
			ID:     rectID,
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 140, H: 88},
			Radius: 14,
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorPrimary), p.th.Color(theme.ColorBorder), 0),
				Stroke:  theme.MaterialStroke{Paint: theme.Fill{Type: theme.FillSolid, Color: p.th.Color(theme.ColorBorder), Opacity: 1}, Width: 1},
				Opacity: 1,
				Visible: true,
			},
		},
		&basic.Text{
			ID: textID,
			Paragraph: text.Paragraph{Spans: []text.TextSpan{{
				Text:  shortLabel(entry.DisplayName),
				Style: p.th.TextStyle(theme.TextBodyM),
			}}},
			Style:    p.th.TextStyle(theme.TextBodyM),
			MaxWidth: 180,
		},
	}
}

func (p *PreviewFacet) buildStructurePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	rectID := sceneID(entry.ID, "structure-rect")
	labelID := sceneID(entry.ID, "structure-label")
	mountID := sceneID(entry.ID, "structure-mount")
	return []facet.FacetImpl{
		&structure.Group{
			ID: rectID,
			Children: []marks.Mark{
				&basic.Rect{
					ID:     sceneID(entry.ID, "group-rect"),
					Bounds: basic.BoundsProps{X: 0, Y: 0, W: 130, H: 74},
					Radius: 12,
					Style: basic.PrimitiveStyleProps{
						Fill:    theme.SolidMaterial(p.th.Color(theme.ColorSurfaceVariant), p.th.Color(theme.ColorBorder), 0),
						Opacity: 1,
						Visible: true,
					},
				},
				&basic.Text{
					ID: labelID,
					Paragraph: text.Paragraph{Spans: []text.TextSpan{{
						Text:  "structure.group",
						Style: p.th.TextStyle(theme.TextLabelS),
					}}},
					Style:    p.th.TextStyle(theme.TextLabelS),
					MaxWidth: 160,
				},
			},
		},
		&structure.LayerMount{
			ID:          mountID,
			TargetLayer: 2,
			Child: &basic.Rect{
				ID:     sceneID(entry.ID, "mount-rect"),
				Bounds: basic.BoundsProps{X: 0, Y: 0, W: 96, H: 52},
				Radius: 10,
				Style: basic.PrimitiveStyleProps{
					Fill:    theme.SolidMaterial(p.th.Color(theme.ColorPrimary), p.th.Color(theme.ColorBorder), 0),
					Opacity: 1,
					Visible: true,
				},
			},
		},
	}
}

func (p *PreviewFacet) buildAnnotationPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	leftID := sceneID(entry.ID, "anchor-left")
	rightID := sceneID(entry.ID, "anchor-right")
	return []facet.FacetImpl{
		&basic.Rect{
			ID:     leftID,
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 88, H: 56},
			Radius: 12,
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorPrimary), p.th.Color(theme.ColorBorder), 0),
				Opacity: 1,
				Visible: true,
			},
		},
		&basic.Rect{
			ID:     rightID,
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 88, H: 56},
			Radius: 12,
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorPrimary), p.th.Color(theme.ColorBorder), 0),
				Opacity: 1,
				Visible: true,
			},
		},
		&annotation.Label{
			ID:         sceneID(entry.ID, "label"),
			Text:       simpleTextMark("annotation.label", p.th),
			Placement:  annotation.LabelAnchorAttached,
			AnchorRef:  &annotation.AnchorSourceRef{MarkID: leftID, Anchor: "center"},
			Offset:     gfx.Point{X: 0, Y: -30},
			Background: true,
		},
		&annotation.Connector{
			ID:        sceneID(entry.ID, "connector"),
			Mode:      annotation.ConnectorStraight,
			From:      annotation.ConnectorEndpoint{Source: annotation.AnchorSourceRef{MarkID: leftID, Anchor: "center"}},
			To:        annotation.ConnectorEndpoint{Source: annotation.AnchorSourceRef{MarkID: rightID, Anchor: "center"}},
			ArrowHead: true,
		},
	}
}

func (p *PreviewFacet) buildUIInputPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	checked := bindingstore.NewBinding(true)
	value := bindingstore.NewBinding("edit me")
	selected := bindingstore.NewBinding("alpha")
	return []facet.FacetImpl{
		&uiinput.Button{
			ID:      sceneID(entry.ID, "button"),
			Label:   "Button",
			Theme:   p.th,
			Shaper:  p.shaper,
			OnPress: func() { checked.Set(!checked.Get()) },
		},
		&uiinput.Checkbox{
			ID:      sceneID(entry.ID, "checkbox"),
			Label:   "Checkbox",
			Checked: checked,
		},
		&uiinput.TextInput{
			ID:          sceneID(entry.ID, "textinput"),
			Value:       value,
			Placeholder: "Type here",
			Theme:       p.th,
			Shaper:      p.shaper,
		},
		&uiinput.Select{
			ID: sceneID(entry.ID, "select"),
			Options: []uiinput.SelectOption{
				{Key: "alpha", Label: "Alpha"},
				{Key: "beta", Label: "Beta"},
				{Key: "gamma", Label: "Gamma"},
			},
			Selected: selected,
			Theme:    p.th,
			Shaper:   p.shaper,
		},
	}
}

func (p *PreviewFacet) buildUINavPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	anchorID := sceneID(entry.ID, "anchor")
	return []facet.FacetImpl{
		&basic.Rect{
			ID:     anchorID,
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 120, H: 72},
			Radius: 14,
			Style: basic.PrimitiveStyleProps{
				Fill:    theme.SolidMaterial(p.th.Color(theme.ColorSurfaceVariant), p.th.Color(theme.ColorBorder), 0),
				Opacity: 1,
				Visible: true,
			},
		},
		&uinav.Menu{
			ID:     sceneID(entry.ID, "menu"),
			Anchor: uinav.AnchorSourceRef{MarkID: anchorID, Anchor: "bounds-center"},
			Open:   bindingstore.NewBinding(true),
			Items: []uinav.MenuItem{
				{Key: "overview", Label: "Overview"},
				{Key: "details", Label: "Details"},
				{Key: "settings", Label: "Settings"},
			},
		},
		&uinav.SpeedDial{
			ID:     sceneID(entry.ID, "speeddial"),
			Anchor: uinav.AnchorSourceRef{MarkID: anchorID, Anchor: "bottom-right"},
			Open:   bindingstore.NewBinding(true),
			Actions: []uinav.SpeedDialAction{
				{Key: "add", Label: "Add"},
				{Key: "edit", Label: "Edit"},
			},
			OnAction: func(string) {},
		},
		&uinav.Tabs{
			ID: sceneID(entry.ID, "tabs"),
			Items: []uinav.TabItem{
				{Key: "one", Label: "One"},
				{Key: "two", Label: "Two"},
				{Key: "three", Label: "Three"},
			},
			Selected: bindingstore.NewBinding("one"),
			Theme:    p.th,
			Shaper:   p.shaper,
		},
	}
}

func (p *PreviewFacet) buildUINotificationPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return []facet.FacetImpl{
		&uinotification.Dialog{
			ID:    sceneID(entry.ID, "dialog"),
			Open:  bindingstore.NewBinding(true),
			Title: "Dialog",
			Body: []marks.Mark{
				&basic.Text{
					ID: sceneID(entry.ID, "dialog-body"),
					Paragraph: text.Paragraph{Spans: []text.TextSpan{{
						Text:  "This is a live dialog mark.",
						Style: p.th.TextStyle(theme.TextBodyS),
					}}},
					Style:    p.th.TextStyle(theme.TextBodyS),
					MaxWidth: 240,
				},
			},
			Actions: []marks.Mark{
				&uiinput.Button{
					ID:     sceneID(entry.ID, "dialog-action"),
					Label:  "Close",
					Theme:  p.th,
					Shaper: p.shaper,
				},
			},
		},
		&uinotification.Snackbar{
			ID:      sceneID(entry.ID, "snackbar"),
			Message: "Snackbar",
			Open:    bindingstore.NewBinding(true),
		},
	}
}

func (p *PreviewFacet) buildChartPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	axis := &chart.Axis{
		ID:          sceneID(entry.ID, "axis"),
		Orientation: chart.AxisBottom,
		ShowGrid:    true,
		Title:       "chart.axis",
	}
	return []facet.FacetImpl{
		&structure.Group{
			ID: sceneID(entry.ID, "group"),
			Children: []marks.Mark{
				axis,
			},
		},
	}
}

func (p *PreviewFacet) buildStructureGroupPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildStructurePreview(entry)
}

func (p *PreviewFacet) buildStructureClipPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildStructurePreview(entry)
}

func (p *PreviewFacet) buildStructureTransformPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildStructurePreview(entry)
}

func (p *PreviewFacet) buildStructureLayerPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildStructurePreview(entry)
}

func (p *PreviewFacet) buildStructureViewportPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildStructurePreview(entry)
}

func (p *PreviewFacet) buildStructureAnchorPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildStructurePreview(entry)
}

func (p *PreviewFacet) buildAnnotationAreaPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationBadgePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationCalloutPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationConnectorPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationHandlePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationIconPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationLabelPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationRulePreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildAnnotationSymbolPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildAnnotationPreview(entry)
}

func (p *PreviewFacet) buildUIInputButtonPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUIInputCheckboxPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUIInputRadioGroupPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUIInputSliderPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUIInputSelectPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUIInputSwitchPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUIInputTextInputPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUIInputPreview(entry)
}

func (p *PreviewFacet) buildUINavMenuPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINavBreadcrumbsPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINavDrawerPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINavPaginationPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINavScrollbarPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINavSpeedDialPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINavTabsPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINavPreview(entry)
}

func (p *PreviewFacet) buildUINotificationDialogPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINotificationPreview(entry)
}

func (p *PreviewFacet) buildUINotificationProgressPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINotificationPreview(entry)
}

func (p *PreviewFacet) buildUINotificationSnackbarPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildUINotificationPreview(entry)
}

func (p *PreviewFacet) buildChartAxisPreview(entry *model.CatalogEntry) []facet.FacetImpl {
	return p.buildChartPreview(entry)
}

func previewChildAttachment(index int) layout.ChildAttachment {
	_ = index
	return layout.ChildAttachment{LayerID: previewSceneLayerID}
}

func sceneID(parts ...string) string {
	return strings.Join(parts, ":")
}

func shortLabel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "preview"
	}
	return s
}

func simpleTextMark(label string, th theme.Context) basic.Text {
	return basic.Text{
		Paragraph: text.Paragraph{Spans: []text.TextSpan{{Text: label, Style: th.TextStyle(theme.TextBodyS)}}},
		Style:     th.TextStyle(theme.TextBodyS),
	}
}
