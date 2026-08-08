package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// playFeedbackFamily is the Feedback playground: alert, dialog, notification,
// and tooltip. The alert's message is store-bound (a "trigger fault" button
// flips its content); the dialog is opened by a button action and closes on an
// action or dismissal; the notification and tooltip surface transient status
// from a button action — the feedback family's distinctive behavior: a user
// action surfaces (and then clears) feedback through a store-backed write-back
// loop.
type playFeedbackFamily struct {
	scroll *demoList

	alert    *feedback.Alert
	alertMsg *store.ValueStore[string]

	alertTrigger *action.Button
	alertClear   *action.Button

	dialog            *feedback.Dialog
	dialogOpen        *store.ValueStore[bool]
	dialogOpenTrigger *action.Button

	toast        *feedback.Notification
	toastOpen    *store.ValueStore[bool]
	toastTrigger *action.Button

	tip        *feedback.Tooltip
	tipOpen    *store.ValueStore[bool]
	tipTrigger *action.Button
}

// newPlayFeedbackFamily builds the Feedback family playground.
func newPlayFeedbackFamily() *playFeedbackFamily {
	f := &playFeedbackFamily{
		alertMsg:   store.NewValueStore("All sources are healthy."),
		dialogOpen: store.NewValueStore(false),
		toastOpen:  store.NewValueStore(false),
		tipOpen:    store.NewValueStore(false),
	}

	f.alertTrigger = action.NewButton(marks.Const("Trigger fault"), marks.Const(uiinput.ButtonFilled))
	f.alertClear = action.NewButton(marks.Const("Clear"), marks.Const(uiinput.ButtonOutlined))
	f.alert = feedback.NewAlert("Sources", "All sources are healthy.")
	f.alert.Message = marks.FromStore(f.alertMsg, facet.DirtyProjection)
	f.alert.CloseButtonLabel = marks.Const("Clear")

	f.dialogOpenTrigger = action.NewButton(marks.Const("Open dialog"), marks.Const(uiinput.ButtonFilled))
	f.dialog = feedback.NewDialog("Delete source?", "This removes the selected source row.", []feedback.DialogAction{
		{Label: "Cancel", Variant: uiinput.ButtonOutlined},
		{Label: "Delete", Variant: uiinput.ButtonFilled},
	}, f.dialogOpen)

	f.toastTrigger = action.NewButton(marks.Const("Show notification"), marks.Const(uiinput.ButtonOutlined))
	f.toast = feedback.NewNotification("Export complete", "The chart image was written to disk.", f.toastOpen)

	f.tipTrigger = action.NewButton(marks.Const("Show tooltip"), marks.Const(uiinput.ButtonOutlined))
	f.tip = feedback.NewTooltip("A tooltip passes pointer input through to the control beneath it.", f.tipOpen)

	f.scroll = newDemoList(listGap,
		playgroundCard("alert — trigger and clear feedback", f.alertTrigger, f.alert, f.alertClear),
		playgroundCard("dialog — gate a destructive action", f.dialogOpenTrigger, f.dialog),
		playgroundCard("notification — transient status", f.toastTrigger, f.toast),
		playgroundCard("tooltip — transient hint", f.tipTrigger, f.tip),
	)
	return f
}

func (f *playFeedbackFamily) wire() func() {
	alertID := f.alertTrigger.Activated.Subscribe(func(signal.Unit) {
		f.alertMsg.Set("Source 7 last read failed — check the connection.")
	})
	clearID := f.alertClear.Activated.Subscribe(func(signal.Unit) {
		f.alertMsg.Set("All sources are healthy.")
	})
	openID := f.dialogOpenTrigger.Activated.Subscribe(func(signal.Unit) {
		if !f.dialog.Open.Get() {
			f.dialogOpen.Set(true)
		}
	})
	actionID := f.dialog.Actioned.Subscribe(func(int) {
		f.dialogOpen.Set(false)
	})
	dismissID := f.dialog.Dismissed.Subscribe(func(signal.Unit) {
		f.dialogOpen.Set(false)
	})
	toastID := f.toastTrigger.Activated.Subscribe(func(signal.Unit) {
		f.toastOpen.Set(true)
	})
	toastDismiss := f.toast.Dismissed.Subscribe(func(signal.Unit) {
		f.toastOpen.Set(false)
	})
	tipID := f.tipTrigger.Activated.Subscribe(func(signal.Unit) {
		f.tipOpen.Set(true)
	})
	tipDismiss := f.tip.Dismissed.Subscribe(func(signal.Unit) {
		f.tipOpen.Set(false)
	})
	return func() {
		f.alertTrigger.Activated.Unsubscribe(alertID)
		f.alertClear.Activated.Unsubscribe(clearID)
		f.dialogOpenTrigger.Activated.Unsubscribe(openID)
		f.dialog.Actioned.Unsubscribe(actionID)
		f.dialog.Dismissed.Unsubscribe(dismissID)
		f.toastTrigger.Activated.Unsubscribe(toastID)
		f.toast.Dismissed.Unsubscribe(toastDismiss)
		f.tipTrigger.Activated.Unsubscribe(tipID)
		f.tip.Dismissed.Unsubscribe(tipDismiss)
	}
}

func (f *playFeedbackFamily) AlertMessage() *store.ValueStore[string] { return f.alertMsg }
func (f *playFeedbackFamily) DialogOpen() *store.ValueStore[bool]     { return f.dialogOpen }
func (f *playFeedbackFamily) ToastOpen() *store.ValueStore[bool]      { return f.toastOpen }
func (f *playFeedbackFamily) TipOpen() *store.ValueStore[bool]        { return f.tipOpen }
func (f *playFeedbackFamily) Alert() *feedback.Alert                  { return f.alert }
func (f *playFeedbackFamily) Dialog() *feedback.Dialog                { return f.dialog }
func (f *playFeedbackFamily) Toast() *feedback.Notification           { return f.toast }
func (f *playFeedbackFamily) Tip() *feedback.Tooltip                  { return f.tip }
func (f *playFeedbackFamily) AlertTrigger() *action.Button            { return f.alertTrigger }
func (f *playFeedbackFamily) DialogOpenTrigger() *action.Button       { return f.dialogOpenTrigger }
func (f *playFeedbackFamily) ToastTrigger() *action.Button            { return f.toastTrigger }
func (f *playFeedbackFamily) TipTrigger() *action.Button              { return f.tipTrigger }
