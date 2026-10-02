// Package bindingsources registers every binding element type used by an
// exported binding field in the marks collection with the testkit
// declared-bindings walker (RX-2 FR-1). It lives apart from testkit so the
// walker's home package imports only the marks root package — marks
// subpackage tests use the walker, and testkit must not import the packages
// under their test (import cycle in test). A mark that grows a binding field
// with a new element type needs one more registration here; the conformance
// walk (marks/declared_bindings_test.go) fails closed otherwise.
package bindingsources

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

func register[T any]() {
	testkit.RegisterBindingSource[T](func(flags facet.DirtyFlags) (binding any, write func()) {
		var zero T
		s := store.NewValueStore(zero)
		return marks.FromStore(s, flags), func() { s.Set(zero) }
	})
}

func init() {
	register[string]()
	register[bool]()
	register[int]()
	register[float32]()
	register[float64]()
	register[gfx.Color]()
	register[theme.ColorToken]()
	register[theme.TextToken]()
	register[text.TextAlignment]()
	register[uiinput.ButtonVariant]()
	register[uiinput.IconButtonVariant]()
	register[uiinput.CheckboxVariant]()
	register[uiinput.SwitchVariant]()
	register[uiinput.SliderVariant]()
	register[uiinput.SelectVariant]()
	register[uiinput.RadioGroupVariant]()
	register[uiinput.TextInputVariant]()
	register[uiinput.ListItemVariant]()
	register[uinav.TabsVariant]()
	register[structure.ScrollDirection]()
	register[structure.CrossAlign]()
	register[structure.ContentInsets]()
	register[structure.CardLayoutMode]()
	register[viz.AxisOrientation]()
	register[primitive.TextOverflow]()
	register[primitive.IconDensityBehavior]()
	register[input.TextFieldValidation]()
	register[input.NumberFieldValidation]()
	register[feedback.DialogContentLayoutMode]()
	register[feedback.NotificationContentLayoutMode]()
	register[feedback.DialogAction]()
	register[[]feedback.DialogAction]()
	register[feedback.DialogContentChild]()
	register[[]feedback.DialogContentChild]()
	register[feedback.NotificationContentChild]()
	register[[]feedback.NotificationContentChild]()
	register[selection.ButtonGroupMode]()
	register[selection.DropdownOption]()
	register[[]selection.DropdownOption]()
	register[selection.ButtonGroupOption]()
	register[[]selection.ButtonGroupOption]()
	register[selection.RadioOption]()
	register[[]selection.RadioOption]()
	register[action.ActionBarAction]()
	register[[]action.ActionBarAction]()
	register[action.ActionGroupAction]()
	register[[]action.ActionGroupAction]()
	register[[]action.RadialChild]()
	register[[]action.SplitButtonItem]()
	register[*action.ActionBarAction]()
	register[svg.SVGPreserveAspectRatio]()
}

// Register is a no-op kept for call-site readability: importing this package
// performs the registrations in init; the explicit call documents intent at
// the walk site.
func Register() {}
