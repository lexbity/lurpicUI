package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type OverlayHost struct {
	facet.Facet
	layout       facet.LayoutRole
	appState     *state.AppState

	dialog       *feedback.Dialog
	notification *feedback.Notification
	commandPalette *action.CommandPalette
	navDrawer    *navigation.NavDrawer
	bottomSheet   *structure.ScrollRegion
	popupPalette  *action.PopupPalette
	tooltip       *feedback.Tooltip
	radialMenu    *action.RadialMenu
	alert         *feedback.Alert

	bottomSheetOpen bool
	alertOpen    bool
	dialogSub    signal.SubscriptionID
	notifSub     signal.SubscriptionID
	paletteSub   signal.SubscriptionID
}

func NewOverlayHost(appState *state.AppState, reg *runtime.CommandRegistry) *OverlayHost {
	o := &OverlayHost{appState: appState}
	o.Facet = facet.NewFacet()

	o.dialog = feedback.NewDialog("Delete Source", "Are you sure you want to delete this data source?", []feedback.DialogAction{
		{Label: "Cancel", Variant: uiinput.ButtonText},
		{Label: "Delete", Variant: uiinput.ButtonTonal},
	})
	o.dialog.Dismissed.Subscribe(func(signal.Unit) {
		appState.OverlayState.Set(state.OverlayNone)
	})
	o.dialog.Actioned.Subscribe(func(index int) {
		if index == 1 {
			appState.SelectedSource.Set("")
		}
		appState.OverlayState.Set(state.OverlayNone)
	})

	o.notification = feedback.NewNotification("Export Complete", "Your data has been exported successfully.")
	o.notification.Open = marks.Const(false)
	o.notification.Dismissed.Subscribe(func(signal.Unit) {
		appState.OverlayState.Set(state.OverlayNone)
	})

	o.commandPalette = action.NewCommandPalette(marks.Const("Commands"), reg)
	o.commandPalette.Open = false

	o.navDrawer = navigation.NewNavDrawer("Navigation", []navigation.NavDrawerSection{
		{
			Label: "Sources",
			Items: []navigation.NavDrawerItem{
				{Key: "all", Label: "All Data"},
				{Key: "na", Label: "North America"},
				{Key: "eu", Label: "Europe"},
				{Key: "apac", Label: "APAC"},
				{Key: "latam", Label: "LATAM"},
			},
		},
	})
	o.navDrawer.Open = marks.Const(false)

	o.bottomSheet = structure.NewScrollRegion("Inspector")
	o.bottomSheetOpen = false

	o.popupPalette = action.NewPopupPalette("Quick Insert", []action.PopupPaletteTool{
		{Key: "line", Label: "Line", IconRef: "show_chart", Color: gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1}},
		{Key: "bar", Label: "Bar", IconRef: "bar_chart", Color: gfx.Color{R: 0.8, G: 0.3, B: 0.2, A: 1}},
		{Key: "area", Label: "Area", IconRef: "area_chart", Color: gfx.Color{R: 0.2, G: 0.7, B: 0.4, A: 1}},
	})
	o.popupPalette.Open = marks.Const(false)

	o.tooltip = feedback.NewTooltip("Control help")

	o.radialMenu = action.NewRadialMenu("Chart Actions", nil, []action.RadialChild{})
	o.radialMenu.Open = false

	o.alert = feedback.NewAlert("Invalid Value", "Please enter a valid number.")

	o.Facet.AddChild(o.dialog.Base())
	o.Facet.AddChild(o.notification.Base())
	o.Facet.AddChild(o.commandPalette.Base())
	o.Facet.AddChild(o.navDrawer.Base())
	o.Facet.AddChild(o.bottomSheet.Base())
	o.Facet.AddChild(o.popupPalette.Base())
	o.Facet.AddChild(o.tooltip.Base())
	o.Facet.AddChild(o.radialMenu.Base())
	o.Facet.AddChild(o.alert.Base())

	o.dialogSub = appState.OverlayState.OnChange.Subscribe(func(c signal.Change[state.OverlayKind]) {
		o.dialog.Open = marks.Const(c.New == state.OverlayDialog)
		o.dialog.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})

	o.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			w := constraints.MaxSize.W
			h := constraints.MaxSize.H
			if w <= 0 { w = 1280 }
			if h <= 0 { h = 800 }

			o.dialog.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: h}})
			o.notification.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 80}})
			o.commandPalette.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w * 0.6, H: h * 0.5}})
			o.navDrawer.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 300, H: h}})
			o.radialMenu.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}})
			o.bottomSheet.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: h * 0.4}})
			o.alert.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: h}})

			return facet.MeasureResult{Size: gfx.Size{W: w, H: h}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			w := bounds.Width()
			h := bounds.Height()
			os := o.appState.OverlayState.Get()

			// Only the currently-active overlay receives visible bounds.
			// Everything else is collapsed to an empty rect so projection
			// never paints a closed overlay over the app.
			place := func(f facet.FacetImpl, visible bool, r gfx.Rect) {
				lr := f.Base().LayoutRole()
				if visible {
					lr.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, r)
					lr.ArrangedBounds = r
				} else {
					lr.ArrangedBounds = gfx.Rect{}
				}
			}

			place(o.dialog, os == state.OverlayDialog, gfx.RectFromXYWH(0, 0, w, h))
			place(o.notification, os == state.OverlayNotification, gfx.RectFromXYWH(w-380, h-100, 360, 80))
			place(o.commandPalette, os == state.OverlayCommandPalette || o.commandPalette.Open, gfx.RectFromXYWH(w*0.2, h*0.25, w*0.6, h*0.5))
			place(o.popupPalette, os == state.OverlayPopupPalette, gfx.RectFromXYWH(w*0.3, h*0.3, w*0.4, h*0.4))
			place(o.navDrawer, os == state.OverlayNavDrawer || o.navDrawer.Open.Get(), gfx.RectFromXYWH(0, 0, 300, h))
			place(o.radialMenu, o.radialMenu.Open, gfx.RectFromXYWH(w*0.5-100, h*0.5-100, 200, 200))
			place(o.bottomSheet, o.bottomSheetOpen, gfx.RectFromXYWH(0, h-h*0.4, w, h*0.4))
			place(o.alert, o.alertOpen, gfx.RectFromXYWH(w*0.25, 16, w*0.5, 64))
		},
	}
	o.AddRole(&o.layout)
	return o
}

func (o *OverlayHost) Base() *facet.Facet { o.Facet.BindImpl(o); return &o.Facet }
func (o *OverlayHost) OnAttach(ctx facet.AttachContext)   {}
func (o *OverlayHost) OnDetach()                          { o.appState.OverlayState.OnChange.Unsubscribe(o.dialogSub) }
func (o *OverlayHost) OnActivate()                        {}
func (o *OverlayHost) OnDeactivate()                      {}

func (o *OverlayHost) Dialog() *feedback.Dialog               { return o.dialog }
func (o *OverlayHost) Notification() *feedback.Notification    { return o.notification }
func (o *OverlayHost) CommandPalette() *action.CommandPalette  { return o.commandPalette }
func (o *OverlayHost) NavDrawer() *navigation.NavDrawer        { return o.navDrawer }
func (o *OverlayHost) BottomSheet() *structure.ScrollRegion    { return o.bottomSheet }
func (o *OverlayHost) RadialMenu() *action.RadialMenu          { return o.radialMenu }
func (o *OverlayHost) Alert() *feedback.Alert                  { return o.alert }
func (o *OverlayHost) BottomSheetOpen() bool                   { return o.bottomSheetOpen }

func (o *OverlayHost) ToggleBottomSheet() {
	o.bottomSheetOpen = !o.bottomSheetOpen
	o.bottomSheet.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

func (o *OverlayHost) SetBottomSheetOpen(v bool) {
	o.bottomSheetOpen = v
	o.bottomSheet.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

func (o *OverlayHost) CloseNavDrawer() {
	o.navDrawer.Open = marks.Const(false)
	o.navDrawer.Base().Invalidate(facet.DirtyProjection)
}

func (o *OverlayHost) ShowNotification() {
	o.notification.Open = marks.Const(true)
	o.notification.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	appState := o.appState
	appState.OverlayState.Set(state.OverlayNotification)
}

func (o *OverlayHost) ToggleNavDrawer() {
	o.navDrawer.Open = marks.Const(!o.navDrawer.Open.Get())
	o.navDrawer.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}
