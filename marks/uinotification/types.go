package uinotification

import uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinotification"

// DialogVariant selects the dialog recipe shape.
type DialogVariant = uirecipe.DialogVariant

const (
	DialogStandard    = uirecipe.DialogStandard
	DialogDestructive = uirecipe.DialogDestructive
	DialogFullscreen  = uirecipe.DialogFullscreen
)

// ButtonAction describes a snackbar or dialog action.
type ButtonAction struct {
	Label    string
	Key      string
	Disabled bool
	OnClick  func()
}
