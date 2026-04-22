package templates

// ResolvedMetricTokens stores density-resolved metric values.
type ResolvedMetricTokens struct {
	Control      ResolvedControlMetricTokens
	Input        ResolvedInputMetricTokens
	Navigation   ResolvedNavigationMetricTokens
	Notification ResolvedNotificationMetricTokens
	Annotation   ResolvedAnnotationMetricTokens
	Chart        ResolvedChartMetricTokens
}

// ResolvedControlMetricTokens is the resolved control baseline.
type ResolvedControlMetricTokens struct {
	Height             float32
	HorizontalPadding  float32
	VerticalPadding    float32
	LabelGap           float32
	FocusRingInset     float32
	FocusRingThickness float32
}

// ResolvedInputMetricTokens is the resolved input baseline.
type ResolvedInputMetricTokens struct {
	CheckboxSize         float32
	RadioSize            float32
	SwitchTrackWidth     float32
	SwitchTrackHeight    float32
	SwitchThumbSize      float32
	SliderTrackThickness float32
	SliderThumbSize      float32
	SliderTickSize       float32
	TextFieldMinWidth    float32
	TextFieldPaddingX    float32
	TextFieldPaddingY    float32
	CaretThickness       float32
}

// ResolvedNavigationMetricTokens is the resolved navigation baseline.
type ResolvedNavigationMetricTokens struct {
	TabHeight              float32
	TabIndicatorThickness  float32
	MenuRowHeight          float32
	MenuPadding            float32
	DrawerMinWidth         float32
	DrawerMaxWidth         float32
	ScrollbarThickness     float32
	PaginationItemSize     float32
	BreadcrumbGap          float32
	SpeedDialActionSpacing float32
}

// ResolvedNotificationMetricTokens is the resolved notification baseline.
type ResolvedNotificationMetricTokens struct {
	DialogMinWidth          float32
	DialogMaxWidth          float32
	DialogPadding           float32
	DialogActionGap         float32
	SnackbarMinHeight       float32
	SnackbarMaxWidth        float32
	SnackbarPadding         float32
	ProgressLinearThickness float32
	ProgressCircularStroke  float32
}

// ResolvedAnnotationMetricTokens is the resolved annotation baseline.
type ResolvedAnnotationMetricTokens struct {
	BadgeMinHeight         float32
	BadgePaddingX          float32
	LabelPadding           float32
	HandleVisualSize       float32
	HandleHitExpansion     float32
	CalloutGap             float32
	ConnectorStrokeDefault float32
	RuleStrokeDefault      float32
}

// ResolvedChartMetricTokens is the resolved chart baseline.
type ResolvedChartMetricTokens struct {
	AxisTickLength    float32
	AxisLabelGap      float32
	AxisTitleGap      float32
	GridStrokeThin    float32
	GridStrokeStrong  float32
	SymbolDefaultSize float32
}

// DefaultMetricTokens returns the canonical density-aware metric tables.
func DefaultMetricTokens() MetricTokens {
	return MetricTokens{
		Control: ControlMetricTokens{
			Height:             DensityTriplet{Compact: 32, Regular: 40, Touchspread: 48},
			HorizontalPadding:  DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
			VerticalPadding:    DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
			LabelGap:           DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
			FocusRingInset:     DensityTriplet{Compact: 2, Regular: 2, Touchspread: 3},
			FocusRingThickness: DensityTriplet{Compact: 1.5, Regular: 2, Touchspread: 2.5},
		},
		Input: InputMetricTokens{
			CheckboxSize:         DensityTriplet{Compact: 16, Regular: 18, Touchspread: 22},
			RadioSize:            DensityTriplet{Compact: 16, Regular: 18, Touchspread: 22},
			SwitchTrackWidth:     DensityTriplet{Compact: 32, Regular: 36, Touchspread: 44},
			SwitchTrackHeight:    DensityTriplet{Compact: 18, Regular: 20, Touchspread: 24},
			SwitchThumbSize:      DensityTriplet{Compact: 14, Regular: 16, Touchspread: 20},
			SliderTrackThickness: DensityTriplet{Compact: 3, Regular: 4, Touchspread: 6},
			SliderThumbSize:      DensityTriplet{Compact: 14, Regular: 16, Touchspread: 20},
			SliderTickSize:       DensityTriplet{Compact: 2, Regular: 3, Touchspread: 4},
			TextFieldMinWidth:    DensityTriplet{Compact: 120, Regular: 140, Touchspread: 160},
			TextFieldPaddingX:    DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
			TextFieldPaddingY:    DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
			CaretThickness:       DensityTriplet{Compact: 1, Regular: 1.5, Touchspread: 2},
		},
		Navigation: NavigationMetricTokens{
			TabHeight:              DensityTriplet{Compact: 32, Regular: 40, Touchspread: 48},
			TabIndicatorThickness:  DensityTriplet{Compact: 2, Regular: 3, Touchspread: 4},
			MenuRowHeight:          DensityTriplet{Compact: 28, Regular: 36, Touchspread: 44},
			MenuPadding:            DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
			DrawerMinWidth:         DensityTriplet{Compact: 220, Regular: 256, Touchspread: 288},
			DrawerMaxWidth:         DensityTriplet{Compact: 320, Regular: 360, Touchspread: 420},
			ScrollbarThickness:     DensityTriplet{Compact: 8, Regular: 10, Touchspread: 14},
			PaginationItemSize:     DensityTriplet{Compact: 28, Regular: 32, Touchspread: 40},
			BreadcrumbGap:          DensityTriplet{Compact: 4, Regular: 6, Touchspread: 8},
			SpeedDialActionSpacing: DensityTriplet{Compact: 8, Regular: 12, Touchspread: 16},
		},
		Notification: NotificationMetricTokens{
			DialogMinWidth:          DensityTriplet{Compact: 280, Regular: 320, Touchspread: 360},
			DialogMaxWidth:          DensityTriplet{Compact: 560, Regular: 640, Touchspread: 760},
			DialogPadding:           DensityTriplet{Compact: 16, Regular: 20, Touchspread: 24},
			DialogActionGap:         DensityTriplet{Compact: 8, Regular: 12, Touchspread: 16},
			SnackbarMinHeight:       DensityTriplet{Compact: 36, Regular: 44, Touchspread: 52},
			SnackbarMaxWidth:        DensityTriplet{Compact: 420, Regular: 480, Touchspread: 560},
			SnackbarPadding:         DensityTriplet{Compact: 10, Regular: 12, Touchspread: 16},
			ProgressLinearThickness: DensityTriplet{Compact: 3, Regular: 4, Touchspread: 6},
			ProgressCircularStroke:  DensityTriplet{Compact: 3, Regular: 4, Touchspread: 6},
		},
		Annotation: AnnotationMetricTokens{
			BadgeMinHeight:         DensityTriplet{Compact: 16, Regular: 18, Touchspread: 22},
			BadgePaddingX:          DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
			LabelPadding:           DensityTriplet{Compact: 4, Regular: 6, Touchspread: 8},
			HandleVisualSize:       DensityTriplet{Compact: 8, Regular: 10, Touchspread: 14},
			HandleHitExpansion:     DensityTriplet{Compact: 4, Regular: 6, Touchspread: 8},
			CalloutGap:             DensityTriplet{Compact: 6, Regular: 8, Touchspread: 12},
			ConnectorStrokeDefault: DensityTriplet{Compact: 1.5, Regular: 2, Touchspread: 2.5},
			RuleStrokeDefault:      DensityTriplet{Compact: 1, Regular: 1.5, Touchspread: 2},
		},
		Chart: ChartMetricTokens{
			AxisTickLength:    DensityTriplet{Compact: 4, Regular: 6, Touchspread: 8},
			AxisLabelGap:      DensityTriplet{Compact: 4, Regular: 6, Touchspread: 8},
			AxisTitleGap:      DensityTriplet{Compact: 8, Regular: 10, Touchspread: 12},
			GridStrokeThin:    DensityTriplet{Compact: 0.5, Regular: 1, Touchspread: 1},
			GridStrokeStrong:  DensityTriplet{Compact: 1, Regular: 1.5, Touchspread: 2},
			SymbolDefaultSize: DensityTriplet{Compact: 6, Regular: 8, Touchspread: 10},
		},
	}
}

// ScaleMetricsForDensity resolves density triplets into concrete values.
func ScaleMetricsForDensity(base MetricTokens, mode DensityMode) ResolvedMetricTokens {
	return ResolvedMetricTokens{
		Control: ResolvedControlMetricTokens{
			Height:             base.Control.Height.For(mode),
			HorizontalPadding:  base.Control.HorizontalPadding.For(mode),
			VerticalPadding:    base.Control.VerticalPadding.For(mode),
			LabelGap:           base.Control.LabelGap.For(mode),
			FocusRingInset:     base.Control.FocusRingInset.For(mode),
			FocusRingThickness: base.Control.FocusRingThickness.For(mode),
		},
		Input: ResolvedInputMetricTokens{
			CheckboxSize:         base.Input.CheckboxSize.For(mode),
			RadioSize:            base.Input.RadioSize.For(mode),
			SwitchTrackWidth:     base.Input.SwitchTrackWidth.For(mode),
			SwitchTrackHeight:    base.Input.SwitchTrackHeight.For(mode),
			SwitchThumbSize:      base.Input.SwitchThumbSize.For(mode),
			SliderTrackThickness: base.Input.SliderTrackThickness.For(mode),
			SliderThumbSize:      base.Input.SliderThumbSize.For(mode),
			SliderTickSize:       base.Input.SliderTickSize.For(mode),
			TextFieldMinWidth:    base.Input.TextFieldMinWidth.For(mode),
			TextFieldPaddingX:    base.Input.TextFieldPaddingX.For(mode),
			TextFieldPaddingY:    base.Input.TextFieldPaddingY.For(mode),
			CaretThickness:       base.Input.CaretThickness.For(mode),
		},
		Navigation: ResolvedNavigationMetricTokens{
			TabHeight:              base.Navigation.TabHeight.For(mode),
			TabIndicatorThickness:  base.Navigation.TabIndicatorThickness.For(mode),
			MenuRowHeight:          base.Navigation.MenuRowHeight.For(mode),
			MenuPadding:            base.Navigation.MenuPadding.For(mode),
			DrawerMinWidth:         base.Navigation.DrawerMinWidth.For(mode),
			DrawerMaxWidth:         base.Navigation.DrawerMaxWidth.For(mode),
			ScrollbarThickness:     base.Navigation.ScrollbarThickness.For(mode),
			PaginationItemSize:     base.Navigation.PaginationItemSize.For(mode),
			BreadcrumbGap:          base.Navigation.BreadcrumbGap.For(mode),
			SpeedDialActionSpacing: base.Navigation.SpeedDialActionSpacing.For(mode),
		},
		Notification: ResolvedNotificationMetricTokens{
			DialogMinWidth:          base.Notification.DialogMinWidth.For(mode),
			DialogMaxWidth:          base.Notification.DialogMaxWidth.For(mode),
			DialogPadding:           base.Notification.DialogPadding.For(mode),
			DialogActionGap:         base.Notification.DialogActionGap.For(mode),
			SnackbarMinHeight:       base.Notification.SnackbarMinHeight.For(mode),
			SnackbarMaxWidth:        base.Notification.SnackbarMaxWidth.For(mode),
			SnackbarPadding:         base.Notification.SnackbarPadding.For(mode),
			ProgressLinearThickness: base.Notification.ProgressLinearThickness.For(mode),
			ProgressCircularStroke:  base.Notification.ProgressCircularStroke.For(mode),
		},
		Annotation: ResolvedAnnotationMetricTokens{
			BadgeMinHeight:         base.Annotation.BadgeMinHeight.For(mode),
			BadgePaddingX:          base.Annotation.BadgePaddingX.For(mode),
			LabelPadding:           base.Annotation.LabelPadding.For(mode),
			HandleVisualSize:       base.Annotation.HandleVisualSize.For(mode),
			HandleHitExpansion:     base.Annotation.HandleHitExpansion.For(mode),
			CalloutGap:             base.Annotation.CalloutGap.For(mode),
			ConnectorStrokeDefault: base.Annotation.ConnectorStrokeDefault.For(mode),
			RuleStrokeDefault:      base.Annotation.RuleStrokeDefault.For(mode),
		},
		Chart: ResolvedChartMetricTokens{
			AxisTickLength:    base.Chart.AxisTickLength.For(mode),
			AxisLabelGap:      base.Chart.AxisLabelGap.For(mode),
			AxisTitleGap:      base.Chart.AxisTitleGap.For(mode),
			GridStrokeThin:    base.Chart.GridStrokeThin.For(mode),
			GridStrokeStrong:  base.Chart.GridStrokeStrong.For(mode),
			SymbolDefaultSize: base.Chart.SymbolDefaultSize.For(mode),
		},
	}
}
