package baseline

import "codeburg.org/lexbit/lurpicui/theme/templates"

// Baseline groups the Material-inspired metric tables for the template system.
type Baseline struct {
	UIInput        UIInputBaseline
	UINav          UINavBaseline
	UINotification UINotificationBaseline
}

// UIInputBaseline contains the canonical input-control baseline metrics.
type UIInputBaseline struct {
	Button     ControlBaseline
	Checkbox   SizeBaseline
	RadioGroup SizeBaseline
	Select     FieldBaseline
	Slider     SliderBaseline
	Switch     SwitchBaseline
	TextInput  FieldBaseline
}

// UINavBaseline contains the canonical navigation baseline metrics.
type UINavBaseline struct {
	Drawer      DrawerBaseline
	Breadcrumbs BreadcrumbBaseline
	Menu        MenuBaseline
	Pagination  PaginationBaseline
	SpeedDial   SpeedDialBaseline
	Scrollbar   ScrollbarBaseline
	Tabs        TabsBaseline
}

// UINotificationBaseline contains the canonical notification baseline metrics.
type UINotificationBaseline struct {
	Snackbar SnackbarBaseline
	Dialog   DialogBaseline
	Progress ProgressBaseline
}

// ControlBaseline captures the common control anatomy.
type ControlBaseline struct {
	Height             templates.DensityTriplet
	HorizontalPadding  templates.DensityTriplet
	VerticalPadding    templates.DensityTriplet
	LabelGap           templates.DensityTriplet
	FocusRingInset     templates.DensityTriplet
	FocusRingThickness templates.DensityTriplet
}

// SizeBaseline captures a single density-aware size.
type SizeBaseline struct {
	Size templates.DensityTriplet
}

// FieldBaseline captures text-field-style anatomy.
type FieldBaseline struct {
	MinWidth       templates.DensityTriplet
	PaddingX       templates.DensityTriplet
	PaddingY       templates.DensityTriplet
	CaretThickness templates.DensityTriplet
}

// SliderBaseline captures slider anatomy.
type SliderBaseline struct {
	TrackThickness templates.DensityTriplet
	ThumbSize      templates.DensityTriplet
	TickSize       templates.DensityTriplet
}

// SwitchBaseline captures switch anatomy.
type SwitchBaseline struct {
	TrackWidth  templates.DensityTriplet
	TrackHeight templates.DensityTriplet
	ThumbSize   templates.DensityTriplet
}

// DrawerBaseline captures drawer sizing.
type DrawerBaseline struct {
	MinWidth templates.DensityTriplet
	MaxWidth templates.DensityTriplet
}

// BreadcrumbBaseline captures breadcrumb spacing.
type BreadcrumbBaseline struct {
	Gap templates.DensityTriplet
}

// MenuBaseline captures menu row sizing.
type MenuBaseline struct {
	RowHeight templates.DensityTriplet
	Padding   templates.DensityTriplet
}

// PaginationBaseline captures pagination control sizing.
type PaginationBaseline struct {
	ItemSize templates.DensityTriplet
}

// SpeedDialBaseline captures speed-dial spacing.
type SpeedDialBaseline struct {
	ActionSpacing templates.DensityTriplet
}

// ScrollbarBaseline captures scrollbar thickness.
type ScrollbarBaseline struct {
	Thickness templates.DensityTriplet
}

// TabsBaseline captures tab-strip proportions.
type TabsBaseline struct {
	Height             templates.DensityTriplet
	IndicatorThickness templates.DensityTriplet
}

// SnackbarBaseline captures snackbar sizing.
type SnackbarBaseline struct {
	MinHeight templates.DensityTriplet
	MaxWidth  templates.DensityTriplet
	Padding   templates.DensityTriplet
}

// DialogBaseline captures dialog sizing.
type DialogBaseline struct {
	MinWidth  templates.DensityTriplet
	MaxWidth  templates.DensityTriplet
	Padding   templates.DensityTriplet
	ActionGap templates.DensityTriplet
}

// ProgressBaseline captures progress-indicator sizing.
type ProgressBaseline struct {
	LinearThickness templates.DensityTriplet
	CircularStroke  templates.DensityTriplet
}

// Default returns the phase-1 baseline tables.
func Default() Baseline {
	return Baseline{
		UIInput: UIInputBaseline{
			Button: ControlBaseline{
				Height:             templates.DensityTriplet{Compact: 32, Regular: 40, Touchspread: 48},
				HorizontalPadding:  templates.DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
				VerticalPadding:    templates.DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
				LabelGap:           templates.DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
				FocusRingInset:     templates.DensityTriplet{Compact: 2, Regular: 2, Touchspread: 3},
				FocusRingThickness: templates.DensityTriplet{Compact: 1.5, Regular: 2, Touchspread: 2.5},
			},
			Checkbox:   SizeBaseline{Size: templates.DensityTriplet{Compact: 16, Regular: 18, Touchspread: 22}},
			RadioGroup: SizeBaseline{Size: templates.DensityTriplet{Compact: 16, Regular: 18, Touchspread: 22}},
			Select: FieldBaseline{
				MinWidth:       templates.DensityTriplet{Compact: 120, Regular: 140, Touchspread: 160},
				PaddingX:       templates.DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
				PaddingY:       templates.DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
				CaretThickness: templates.DensityTriplet{Compact: 1, Regular: 1.5, Touchspread: 2},
			},
			Slider: SliderBaseline{
				TrackThickness: templates.DensityTriplet{Compact: 3, Regular: 4, Touchspread: 6},
				ThumbSize:      templates.DensityTriplet{Compact: 14, Regular: 16, Touchspread: 20},
				TickSize:       templates.DensityTriplet{Compact: 2, Regular: 3, Touchspread: 4},
			},
			Switch: SwitchBaseline{
				TrackWidth:  templates.DensityTriplet{Compact: 32, Regular: 36, Touchspread: 44},
				TrackHeight: templates.DensityTriplet{Compact: 18, Regular: 20, Touchspread: 24},
				ThumbSize:   templates.DensityTriplet{Compact: 14, Regular: 16, Touchspread: 20},
			},
			TextInput: FieldBaseline{
				MinWidth:       templates.DensityTriplet{Compact: 120, Regular: 140, Touchspread: 160},
				PaddingX:       templates.DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
				PaddingY:       templates.DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
				CaretThickness: templates.DensityTriplet{Compact: 1, Regular: 1.5, Touchspread: 2},
			},
		},
		UINav: UINavBaseline{
			Drawer: DrawerBaseline{
				MinWidth: templates.DensityTriplet{Compact: 220, Regular: 256, Touchspread: 288},
				MaxWidth: templates.DensityTriplet{Compact: 320, Regular: 360, Touchspread: 420},
			},
			Breadcrumbs: BreadcrumbBaseline{
				Gap: templates.DensityTriplet{Compact: 4, Regular: 6, Touchspread: 8},
			},
			Menu: MenuBaseline{
				RowHeight: templates.DensityTriplet{Compact: 28, Regular: 36, Touchspread: 44},
				Padding:   templates.DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
			},
			Pagination: PaginationBaseline{
				ItemSize: templates.DensityTriplet{Compact: 28, Regular: 32, Touchspread: 40},
			},
			SpeedDial: SpeedDialBaseline{
				ActionSpacing: templates.DensityTriplet{Compact: 8, Regular: 12, Touchspread: 16},
			},
			Scrollbar: ScrollbarBaseline{
				Thickness: templates.DensityTriplet{Compact: 8, Regular: 10, Touchspread: 14},
			},
			Tabs: TabsBaseline{
				Height:             templates.DensityTriplet{Compact: 32, Regular: 40, Touchspread: 48},
				IndicatorThickness: templates.DensityTriplet{Compact: 2, Regular: 3, Touchspread: 4},
			},
		},
		UINotification: UINotificationBaseline{
			Snackbar: SnackbarBaseline{
				MinHeight: templates.DensityTriplet{Compact: 36, Regular: 44, Touchspread: 52},
				MaxWidth:  templates.DensityTriplet{Compact: 420, Regular: 480, Touchspread: 560},
				Padding:   templates.DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
			},
			Dialog: DialogBaseline{
				MinWidth:  templates.DensityTriplet{Compact: 280, Regular: 320, Touchspread: 360},
				MaxWidth:  templates.DensityTriplet{Compact: 560, Regular: 640, Touchspread: 760},
				Padding:   templates.DensityTriplet{Compact: 16, Regular: 20, Touchspread: 24},
				ActionGap: templates.DensityTriplet{Compact: 8, Regular: 12, Touchspread: 16},
			},
			Progress: ProgressBaseline{
				LinearThickness: templates.DensityTriplet{Compact: 3, Regular: 4, Touchspread: 6},
				CircularStroke:  templates.DensityTriplet{Compact: 3, Regular: 4, Touchspread: 6},
			},
		},
	}
}
