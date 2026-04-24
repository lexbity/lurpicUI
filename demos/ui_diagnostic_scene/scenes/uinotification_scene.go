package scenes

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/marks/uinotification"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type UINotificationScene struct {
	BaseScene
	th          theme.Context
	snackOpen   store.Binding[bool]
	dialogOpen  store.Binding[bool]
	linear      store.Binding[float64]
	circular    store.Binding[float64]
	snackbar    *uinotification.Snackbar
	dialog      *uinotification.Dialog
	linearBar   *uinotification.Progress
	circularBar *uinotification.Progress
	snackLabel  *basic.Text
}

func NewUINotificationScene() *UINotificationScene {
	s := &UINotificationScene{
		BaseScene: NewBaseScene(
			"uinotification",
			"UI Notification",
			"Validates snackbar, dialog, and progress notification marks",
			[]string{"uinotification"},
		),
		th:         theme.Default(),
		snackOpen:  store.NewBinding(true),
		dialogOpen: store.NewBinding(true),
		linear:     store.NewBinding(0.72),
		circular:   store.NewBinding(0.35),
	}
	return s
}

func (s *UINotificationScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	stack := layout.NewStackLayout(layout.AlignStart)
	s.root = stack

	title := newTextMark("notification-title", "Notification surfaces", 18)
	stack.AddChild(title.Base())

	s.snackbar = &uinotification.Snackbar{
		ID:       "notification-snackbar",
		Message:  "Saved draft",
		Action:   &uinotification.ButtonAction{Label: "Undo", Key: "undo"},
		Open:     s.snackOpen,
		Duration: 0,
	}
	stack.AddChild(s.snackbar.Base())

	s.dialog = &uinotification.Dialog{
		ID:                "notification-dialog",
		Open:              s.dialogOpen,
		Title:             "Delete item?",
		Body:              []marks.Mark{newTextMark("notification-dialog-body", "This dialog stays open for snapshotting.", 12)},
		Actions:           []marks.Mark{newTextMark("notification-dialog-ok", "Confirm", 12), newTextMark("notification-dialog-cancel", "Cancel", 12)},
		Variant:           uinotification.DialogStandard,
		DismissOnEscape:   true,
		DismissOnBackdrop: true,
	}
	stack.AddChild(s.dialog.Base())

	s.linearBar = &uinotification.Progress{
		ID:    "notification-linear",
		Mode:  uinotification.ProgressDeterminate,
		Shape: uinotification.ProgressLinear,
		Value: s.linear,
	}
	stack.AddChild(s.linearBar.Base())

	s.circularBar = &uinotification.Progress{
		ID:    "notification-circular",
		Mode:  uinotification.ProgressDeterminate,
		Shape: uinotification.ProgressCircular,
		Value: s.circular,
	}
	stack.AddChild(s.circularBar.Base())

	s.snackLabel = newTextMark("notification-status", "", 13)
	stack.AddChild(s.snackLabel.Base())
	s.refreshStatus()
	return stack
}

func (s *UINotificationScene) refreshStatus() {
	if s.snackLabel == nil {
		return
	}
	updateTextValue(s.snackLabel, fmt.Sprintf("Snackbar=%t Dialog=%t Linear=%.2f Circular=%.2f", s.snackOpen.Get(), s.dialogOpen.Get(), s.linear.Get(), s.circular.Get()), 13)
}

func (s *UINotificationScene) ApplyTheme(th theme.Context) {
	s.th = th
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *UINotificationScene) ApplyDensity(scale float32) {
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (s *UINotificationScene) Reset() {
	s.snackOpen.Set(true)
	s.dialogOpen.Set(true)
	s.linear.Set(0.72)
	s.circular.Set(0.35)
	s.refreshStatus()
	s.BaseScene.Reset()
}

func (s *UINotificationScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id": s.id,
		"snackbar": s.snackOpen.Get(),
		"dialog":   s.dialogOpen.Get(),
		"linear":   s.linear.Get(),
		"circular": s.circular.Get(),
	}
}

func (s *UINotificationScene) ImportState(state map[string]any) {
	if v, ok := state["snackbar"].(bool); ok {
		s.snackOpen.Set(v)
	}
	if v, ok := state["dialog"].(bool); ok {
		s.dialogOpen.Set(v)
	}
	if v, ok := state["linear"].(float64); ok {
		s.linear.Set(v)
	}
	if v, ok := state["circular"].(float64); ok {
		s.circular.Set(v)
	}
	s.refreshStatus()
}

var _ scene.Scene = (*UINotificationScene)(nil)
