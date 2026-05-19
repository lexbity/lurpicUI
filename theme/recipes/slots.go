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
	Root                 theme.MarkStyle
	Container            theme.MarkStyle
	Label                theme.MarkStyle
	OptionalLeadingIcon  theme.MarkStyle
	OptionalTrailingIcon theme.MarkStyle
	FocusRing            theme.MarkStyle
	StateLayer           theme.MarkStyle
}

// IconButtonSlots describes styling for an icon button mark.
type IconButtonSlots struct {
	Root       theme.MarkStyle
	Container  theme.MarkStyle
	Icon       theme.MarkStyle
	FocusRing  theme.MarkStyle
	StateLayer theme.MarkStyle
}

// CheckboxSlots describes styling for a checkbox mark.
type CheckboxSlots struct {
	Root       theme.MarkStyle
	ControlBox theme.MarkStyle
	Checkmark  theme.MarkStyle
	Label      theme.MarkStyle
	HelperText theme.MarkStyle
	FocusRing  theme.MarkStyle
	StateLayer theme.MarkStyle
}

// RadioGroupSlots describes styling for a radio group mark.
type RadioGroupSlots struct {
	Root         theme.MarkStyle
	GroupLabel   theme.MarkStyle
	RadioItems   theme.MarkStyle
	RadioControl theme.MarkStyle
	ItemLabel    theme.MarkStyle
	FocusRing    theme.MarkStyle
}

// SelectSlots describes styling for a select mark.
type SelectSlots struct {
	Root               theme.MarkStyle
	Trigger            theme.MarkStyle
	SelectedValueLabel theme.MarkStyle
	Chevron            theme.MarkStyle
	FloatingListbox    theme.MarkStyle
	OptionItems        theme.MarkStyle
	FocusRing          theme.MarkStyle
}

// ListItemSlots describes styling for a selectable list item mark.
type ListItemSlots struct {
	Root               theme.MarkStyle
	ItemContainer      theme.MarkStyle
	LeadingIcon        theme.MarkStyle
	Label              theme.MarkStyle
	SupportingText     theme.MarkStyle
	SelectionIndicator theme.MarkStyle
	FocusRing          theme.MarkStyle
}

// SliderSlots describes styling for a slider mark.
type SliderSlots struct {
	Root        theme.MarkStyle
	Track       theme.MarkStyle
	ActiveTrack theme.MarkStyle
	Thumb       theme.MarkStyle
	TickMarks   theme.MarkStyle
	ValueLabel  theme.MarkStyle
	FocusRing   theme.MarkStyle
}

// SwitchSlots describes styling for a switch mark.
type SwitchSlots struct {
	Root       theme.MarkStyle
	Track      theme.MarkStyle
	Thumb      theme.MarkStyle
	Label      theme.MarkStyle
	FocusRing  theme.MarkStyle
	StateLayer theme.MarkStyle
}

// TextInputSlots describes styling for a text input mark.
type TextInputSlots struct {
	Root           theme.MarkStyle
	FieldContainer theme.MarkStyle
	Label          theme.MarkStyle
	InputText      theme.MarkStyle
	Placeholder    theme.MarkStyle
	HelperText     theme.MarkStyle
	ErrorText      theme.MarkStyle
	Caret          theme.MarkStyle
	SelectionRange theme.MarkStyle
	FocusRing      theme.MarkStyle
}

// NumberFieldSlots describes styling for a number field mark.
type NumberFieldSlots struct {
	Root           theme.MarkStyle
	FieldContainer theme.MarkStyle
	Label          theme.MarkStyle
	InputText      theme.MarkStyle
	Placeholder    theme.MarkStyle
	StepperUp      theme.MarkStyle
	StepperDown    theme.MarkStyle
	HelperText     theme.MarkStyle
	ErrorText      theme.MarkStyle
	Caret          theme.MarkStyle
	SelectionRange theme.MarkStyle
	FocusRing      theme.MarkStyle
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
	Root           theme.MarkStyle
	SegmentList    theme.MarkStyle
	SegmentLink    theme.MarkStyle
	Separator      theme.MarkStyle
	CurrentSegment theme.MarkStyle
	FocusRing      theme.MarkStyle
}

// NavDrawerSlots describes styling for a navigation drawer mark.
type NavDrawerSlots struct {
	Root          theme.MarkStyle
	ScrimOptional theme.MarkStyle
	DrawerSurface theme.MarkStyle
	Header        theme.MarkStyle
	NavItems      theme.MarkStyle
	SectionLabels theme.MarkStyle
	FocusRing     theme.MarkStyle
}

// NavRailSlots describes styling for a navigation rail mark.
type NavRailSlots struct {
	Root            theme.MarkStyle
	RailSurface     theme.MarkStyle
	NavItems        theme.MarkStyle
	ActiveIndicator theme.MarkStyle
	Icon            theme.MarkStyle
	Label           theme.MarkStyle
	FocusRing       theme.MarkStyle
}

// TreeNavigatorSlots describes styling for a tree navigator mark.
type TreeNavigatorSlots struct {
	Root               theme.MarkStyle
	Tree               theme.MarkStyle
	TreeItem           theme.MarkStyle
	Disclosure         theme.MarkStyle
	Icon               theme.MarkStyle
	Label              theme.MarkStyle
	SelectionIndicator theme.MarkStyle
	FocusRing          theme.MarkStyle
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
	Root            theme.MarkStyle
	TabList         theme.MarkStyle
	Tab             theme.MarkStyle
	TabLabel        theme.MarkStyle
	ActiveIndicator theme.MarkStyle
	PanelAnchor     theme.MarkStyle
	FocusRing       theme.MarkStyle
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
