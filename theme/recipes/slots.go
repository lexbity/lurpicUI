package recipes

import "codeburg.org/lexbit/lurpicui/theme"

// LabelSlots describes styling for a generic label.
type LabelSlots struct {
	Text      theme.MarkStyle
	Icon      theme.MarkStyle
	Container theme.MarkStyle
	Underline theme.MarkStyle
}

// ConnectorSlots describes styling for a connector or link mark.
type ConnectorSlots struct {
	Line      theme.MarkStyle
	StartHead theme.MarkStyle
	EndHead   theme.MarkStyle
	Label     theme.MarkStyle
}

// BadgeSlots describes styling for a badge mark.
type BadgeSlots struct {
	Container theme.MarkStyle
	Text      theme.MarkStyle
	Icon      theme.MarkStyle
}

// CalloutSlots describes styling for a callout mark.
type CalloutSlots struct {
	Container theme.MarkStyle
	Accent    theme.MarkStyle
	Title     theme.MarkStyle
	Body      theme.MarkStyle
}

// HandleSlots describes styling for an interaction handle mark.
type HandleSlots struct {
	Visible   theme.MarkStyle
	Hover     theme.MarkStyle
	Focused   theme.MarkStyle
	DragGhost theme.MarkStyle
}

// ButtonSlots describes styling for a button mark.
type ButtonSlots struct {
	Container theme.MarkStyle
	Label     theme.MarkStyle
	Icon      theme.MarkStyle
	FocusRing theme.MarkStyle
}

// CheckboxSlots describes styling for a checkbox mark.
type CheckboxSlots struct {
	Box       theme.MarkStyle
	Check     theme.MarkStyle
	Label     theme.MarkStyle
	FocusRing theme.MarkStyle
}

// RadioGroupSlots describes styling for a radio group mark.
type RadioGroupSlots struct {
	Option    theme.MarkStyle
	Indicator theme.MarkStyle
	Label     theme.MarkStyle
	FocusRing theme.MarkStyle
}

// SelectSlots describes styling for a select mark.
type SelectSlots struct {
	Field     theme.MarkStyle
	Value     theme.MarkStyle
	Popup     theme.MarkStyle
	Arrow     theme.MarkStyle
	FocusRing theme.MarkStyle
}

// SliderSlots describes styling for a slider mark.
type SliderSlots struct {
	Track     theme.MarkStyle
	Fill      theme.MarkStyle
	Thumb     theme.MarkStyle
	Tick      theme.MarkStyle
	ValueText theme.MarkStyle
	FocusRing theme.MarkStyle
}

// SwitchSlots describes styling for a switch mark.
type SwitchSlots struct {
	Track     theme.MarkStyle
	Thumb     theme.MarkStyle
	Label     theme.MarkStyle
	FocusRing theme.MarkStyle
}

// TextInputSlots describes styling for a text input mark.
type TextInputSlots struct {
	Field         theme.MarkStyle
	Text          theme.MarkStyle
	Placeholder   theme.MarkStyle
	Caret         theme.MarkStyle
	Selection     theme.MarkStyle
	Outline       theme.MarkStyle
	FocusRing     theme.MarkStyle
	AssistiveText theme.MarkStyle
}

// DrawerSlots describes styling for a drawer mark.
type DrawerSlots struct {
	Scrim    theme.MarkStyle
	Surface  theme.MarkStyle
	Title    theme.MarkStyle
	Body     theme.MarkStyle
	Backdrop theme.MarkStyle
}

// BreadcrumbSlots describes styling for a breadcrumbs mark.
type BreadcrumbSlots struct {
	Item      theme.MarkStyle
	Current   theme.MarkStyle
	Separator theme.MarkStyle
}

// MenuSlots describes styling for a menu mark.
type MenuSlots struct {
	Surface   theme.MarkStyle
	Item      theme.MarkStyle
	Icon      theme.MarkStyle
	Shortcut  theme.MarkStyle
	FocusRing theme.MarkStyle
}

// PaginationSlots describes styling for a pagination mark.
type PaginationSlots struct {
	Page      theme.MarkStyle
	Current   theme.MarkStyle
	Nav       theme.MarkStyle
	Separator theme.MarkStyle
}

// SpeedDialSlots describes styling for a speed-dial mark.
type SpeedDialSlots struct {
	Fab      theme.MarkStyle
	Action   theme.MarkStyle
	Label    theme.MarkStyle
	Backdrop theme.MarkStyle
}

// ScrollbarSlots describes styling for a scrollbar mark.
type ScrollbarSlots struct {
	Track  theme.MarkStyle
	Thumb  theme.MarkStyle
	Corner theme.MarkStyle
}

// TabsSlots describes styling for a tabs mark.
type TabsSlots struct {
	Tab       theme.MarkStyle
	Current   theme.MarkStyle
	Indicator theme.MarkStyle
	Panel     theme.MarkStyle
}

// SnackbarSlots describes styling for a snackbar mark.
type SnackbarSlots struct {
	Container theme.MarkStyle
	Text      theme.MarkStyle
	Action    theme.MarkStyle
}

// DialogSlots describes styling for a dialog mark.
type DialogSlots struct {
	Scrim      theme.MarkStyle
	Surface    theme.MarkStyle
	TitleText  theme.MarkStyle
	BodyText   theme.MarkStyle
	ActionArea theme.MarkStyle
	Outline    theme.MarkStyle
	Shadow     theme.MarkStyle
}

// ProgressSlots describes styling for a progress mark.
type ProgressSlots struct {
	Track     theme.MarkStyle
	Indicator theme.MarkStyle
	Label     theme.MarkStyle
}

// AxisSlots describes styling for a chart axis mark.
type AxisSlots struct {
	AxisLine  theme.MarkStyle
	Tick      theme.MarkStyle
	TickLabel theme.MarkStyle
	GridLine  theme.MarkStyle
	Title     theme.MarkStyle
}
