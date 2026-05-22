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
	Root           theme.MarkStyle
	BadgeContainer theme.MarkStyle
	Label          theme.MarkStyle
	OptionalIcon   theme.MarkStyle
}

// StatusLightSlots describes styling for a status light mark.
type StatusLightSlots struct {
	Root          theme.MarkStyle
	Indicator     theme.MarkStyle
	LabelOptional theme.MarkStyle
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

// ActionBarSlots describes styling for an action bar mark.
type ActionBarSlots struct {
	Root         theme.MarkStyle
	BarSurface   theme.MarkStyle
	ContextLabel theme.MarkStyle
	ActionItems  theme.MarkStyle
	OverflowMenu theme.MarkStyle
	FocusRing    theme.MarkStyle
}

// ActionGroupSlots describes styling for an action group mark.
type ActionGroupSlots struct {
	Root         theme.MarkStyle
	GroupSurface theme.MarkStyle
	ActionItems  theme.MarkStyle
	Separators   theme.MarkStyle
	FocusRing    theme.MarkStyle
}

// ToolbarSlots describes styling for an action toolbar mark.
type ToolbarSlots struct {
	Root           theme.MarkStyle
	ToolbarSurface theme.MarkStyle
	ActionItems    theme.MarkStyle
	Groups         theme.MarkStyle
	Separators     theme.MarkStyle
	OverflowMenu   theme.MarkStyle
	FocusRing      theme.MarkStyle
}

// RibbonSlots describes styling for an action ribbon mark.
type RibbonSlots struct {
	Root             theme.MarkStyle
	RibbonSurface    theme.MarkStyle
	Groups           theme.MarkStyle
	GroupLabels      theme.MarkStyle
	ActionItems      theme.MarkStyle
	OverflowControls theme.MarkStyle
	FocusRing        theme.MarkStyle
}

// MenuButtonSlots describes styling for a menu button mark.
type MenuButtonSlots struct {
	Root                theme.MarkStyle
	Trigger             theme.MarkStyle
	TriggerLabel        theme.MarkStyle
	TriggerIcon         theme.MarkStyle
	Chevron             theme.MarkStyle
	FloatingMenuSurface theme.MarkStyle
	MenuItems           theme.MarkStyle
	FocusRing           theme.MarkStyle
}

// SplitButtonSlots describes styling for a split button mark.
type SplitButtonSlots struct {
	Root                theme.MarkStyle
	PrimaryButton       theme.MarkStyle
	PrimaryLabel        theme.MarkStyle
	MenuTrigger         theme.MarkStyle
	Chevron             theme.MarkStyle
	FloatingMenuSurface theme.MarkStyle
	MenuItems           theme.MarkStyle
	FocusRing           theme.MarkStyle
}

// CommandPaletteSlots describes styling for a command palette mark.
type CommandPaletteSlots struct {
	Root          theme.MarkStyle
	Backdrop      theme.MarkStyle
	ModalSurface  theme.MarkStyle
	SearchField   theme.MarkStyle
	ResultsList   theme.MarkStyle
	ResultItem    theme.MarkStyle
	ShortcutLabel theme.MarkStyle
	EmptyState    theme.MarkStyle
	FocusRing     theme.MarkStyle
}

// PopupPaletteSlots describes styling for an anchored popup palette mark.
type PopupPaletteSlots struct {
	Root           theme.MarkStyle
	PaletteSurface theme.MarkStyle
	ToolItems      theme.MarkStyle
	ToolGroup      theme.MarkStyle
	AnchorArrow    theme.MarkStyle
	FocusRing      theme.MarkStyle
}

// RadialMenuSlots describes styling for a composed radial menu mark.
type RadialMenuSlots struct {
	Root        theme.MarkStyle
	Surface     theme.MarkStyle
	CenterSlot  theme.MarkStyle
	RadialTrack theme.MarkStyle
	AnchorArrow theme.MarkStyle
	FocusRing   theme.MarkStyle
}

// ButtonGroupSlots describes styling for a segmented button-group mark.
type ButtonGroupSlots struct {
	Root              theme.MarkStyle
	GroupSurface      theme.MarkStyle
	OptionButtons     theme.MarkStyle
	SelectedIndicator theme.MarkStyle
	FocusRing         theme.MarkStyle
}

// IconButtonSlots describes styling for an icon button mark.
type IconButtonSlots struct {
	Root       theme.MarkStyle
	Container  theme.MarkStyle
	Icon       theme.MarkStyle
	FocusRing  theme.MarkStyle
	StateLayer theme.MarkStyle
}

// ColorPickerSlots describes styling for a color picker mark.
type ColorPickerSlots struct {
	Root      theme.MarkStyle
	Wheel     theme.MarkStyle
	Triangle  theme.MarkStyle
	Handle    theme.MarkStyle
	FocusRing theme.MarkStyle
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

// CardSlots describes styling for a structure card mark.
type CardSlots struct {
	Root              theme.MarkStyle
	CardSurface       theme.MarkStyle
	HeaderOptional    theme.MarkStyle
	MediaOptional     theme.MarkStyle
	Body              theme.MarkStyle
	ActionsOptional   theme.MarkStyle
	FocusRingOptional theme.MarkStyle
}

// ListSlots describes styling for a structure list mark.
type ListSlots struct {
	Root                  theme.MarkStyle
	ListContainer         theme.MarkStyle
	ListItems             theme.MarkStyle
	SectionHeaderOptional theme.MarkStyle
	EmptyStateOptional    theme.MarkStyle
}

// TableSlots describes styling for a structure table mark.
type TableSlots struct {
	Root                    theme.MarkStyle
	TableSurface            theme.MarkStyle
	HeaderRow               theme.MarkStyle
	HeaderCell              theme.MarkStyle
	BodyRows                theme.MarkStyle
	BodyCell                theme.MarkStyle
	SelectionColumnOptional theme.MarkStyle
	SortIndicator           theme.MarkStyle
	FocusRing               theme.MarkStyle
}

// ScrollRegionSlots describes styling for a scroll region mark.
type ScrollRegionSlots struct {
	Root                        theme.MarkStyle
	Viewport                    theme.MarkStyle
	Content                     theme.MarkStyle
	ScrollbarVerticalOptional   theme.MarkStyle
	ScrollbarHorizontalOptional theme.MarkStyle
	ScrollShadowsOptional       theme.MarkStyle
	FocusRingOptional           theme.MarkStyle
}

// PaginationSlots describes styling for a pagination mark.
type PaginationSlots struct {
	Root      theme.MarkStyle
	Page      theme.MarkStyle
	Current   theme.MarkStyle
	Nav       theme.MarkStyle
	Separator theme.MarkStyle
	FocusRing theme.MarkStyle
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

// FeedbackAlertSlots describes styling for a feedback.alert mark.
type FeedbackAlertSlots struct {
	Root         theme.MarkStyle
	AlertSurface theme.MarkStyle
	Icon         theme.MarkStyle
	Title        theme.MarkStyle
	Message      theme.MarkStyle
	Action       theme.MarkStyle
	CloseButton  theme.MarkStyle
}

// FeedbackTooltipSlots describes styling for a feedback.tooltip mark.
type FeedbackTooltipSlots struct {
	Root           theme.MarkStyle
	TooltipSurface theme.MarkStyle
	Content        theme.MarkStyle
	AnchorArrow    theme.MarkStyle
}

// FeedbackNotificationSlots describes styling for a feedback.notification mark.
type FeedbackNotificationSlots struct {
	Root          theme.MarkStyle
	StatusSurface theme.MarkStyle
	Icon          theme.MarkStyle
	Title         theme.MarkStyle
	Message       theme.MarkStyle
	Action        theme.MarkStyle
	CloseButton   theme.MarkStyle
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

// FeedbackDialogSlots describes styling for a feedback.dialog mark.
type FeedbackDialogSlots struct {
	Root         theme.MarkStyle
	Backdrop     theme.MarkStyle
	ModalSurface theme.MarkStyle
	Title        theme.MarkStyle
	Body         theme.MarkStyle
	Actions      theme.MarkStyle
	CloseButton  theme.MarkStyle
	FocusRing    theme.MarkStyle
}

// ProgressSlots describes styling for a progress mark.
type ProgressSlots struct {
	Track     theme.MarkStyle
	Indicator theme.MarkStyle
	Label     theme.MarkStyle
}

// StatusProgressBarSlots describes styling for a status.progress_bar mark.
type StatusProgressBarSlots struct {
	Root          theme.MarkStyle
	Track         theme.MarkStyle
	Indicator     theme.MarkStyle
	OptionalLabel theme.MarkStyle
}

// StatusProgressRingSlots describes styling for a status.progress_ring mark.
type StatusProgressRingSlots struct {
	Root          theme.MarkStyle
	TrackArc      theme.MarkStyle
	IndicatorArc  theme.MarkStyle
	OptionalLabel theme.MarkStyle
}

// AxisSlots describes styling for a chart axis mark.
type AxisSlots struct {
	AxisLine  theme.MarkStyle
	Tick      theme.MarkStyle
	TickLabel theme.MarkStyle
	GridLine  theme.MarkStyle
	Title     theme.MarkStyle
}
