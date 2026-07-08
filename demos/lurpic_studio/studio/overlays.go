package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
)

type overlays struct {
	dialog         *feedback.Dialog
	exportToast    *feedback.Notification
	tooltip        *feedback.Tooltip
	commandPalette *action.CommandPalette
	popupPalette   *action.PopupPalette
	navDrawer      *navigation.NavDrawer
	commandReg     *runtime.CommandRegistry
}

func newOverlays(as *state.AppState) *overlays {
	o := &overlays{}

	o.commandReg = runtime.NewCommandRegistry()
	registerCommands(o.commandReg, as)

	o.commandPalette = action.NewCommandPalette(marks.Const("Commands"), o.commandReg)
	o.commandPalette.Open = false

	o.dialog = feedback.NewDialog("Delete Source", "Are you sure you want to delete this source?", []feedback.DialogAction{
		{Label: "Cancel"},
		{Label: "Delete"},
	})
	o.dialog.Open = marks.Const(false)
	o.dialog.Actioned.Subscribe(func(idx int) {
		if idx == 1 {
			as.SelectedSource.Set("")
		}
		as.OverlayState.Set(state.OverlayNone)
	})

	o.exportToast = feedback.NewNotification("Export Complete", "Your data has been exported successfully.")
	o.exportToast.Open = marks.Const(false)

	o.tooltip = feedback.NewTooltip("")
	o.tooltip.Open = marks.Const(false)

	o.popupPalette = action.NewPopupPalette("Quick Insert", []action.PopupPaletteTool{
		{Key: "csv", Label: "CSV", IconRef: "csv"},
		{Key: "json", Label: "JSON", IconRef: "json"},
	})
	o.popupPalette.Open = marks.Const(false)

	o.navDrawer = navigation.NewNavDrawer("Navigation", []navigation.NavDrawerSection{
		{
			Label: labelSources,
			Items: []navigation.NavDrawerItem{
				{Key: "NA", Label: "NA", IconRef: "globe"},
				{Key: "EU", Label: "EU", IconRef: "globe"},
				{Key: "APAC", Label: "APAC", IconRef: "globe"},
				{Key: labelLATAM, Label: labelLATAM, IconRef: "globe"},
			},
		},
	})
	o.navDrawer.Open = marks.Const(false)
	o.navDrawer.Activated.Subscribe(func(idx int) {
		if idx >= 0 && idx < len(o.navDrawer.Sections[0].Items) {
			item := o.navDrawer.Sections[0].Items[idx]
			as.SelectedSource.Set(item.Key)
		}
		as.OverlayState.Set(state.OverlayNone)
	})

	driveOverlayOpen(as, o)

	return o
}

func driveOverlayOpen(as *state.AppState, o *overlays) {
	kind := as.OverlayState
	o.dialog.Open = marks.FromDerived(
		store.NewDerived(func() bool { return kind.Get() == state.OverlayDialog }, kind),
		facetDirtyFlags,
	)
	o.navDrawer.Open = marks.FromDerived(
		store.NewDerived(func() bool { return kind.Get() == state.OverlayNavDrawer }, kind),
		facetDirtyFlags,
	)
}

var facetDirtyFlags = facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit
