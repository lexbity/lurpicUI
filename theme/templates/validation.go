package templates

import (
	"fmt"
	"strings"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

// ValidationReport summarizes template authoring issues and fallback usage.
type ValidationReport struct {
	ThemeName             string
	Density               DensityMode
	MissingTokens         []string
	MissingRecipes        []string
	InvalidDensityScaling []string
	ChartFallbackFields   []string
}

// OK reports whether the template has no validation issues.
func (r ValidationReport) OK() bool {
	return len(r.MissingTokens) == 0 &&
		len(r.MissingRecipes) == 0 &&
		len(r.InvalidDensityScaling) == 0
}

// Error renders the validation report as a single error.
func (r ValidationReport) Error() error {
	if r.OK() {
		return nil
	}
	parts := make([]string, 0, 4)
	if len(r.MissingTokens) > 0 {
		parts = append(parts, "missing tokens: "+strings.Join(r.MissingTokens, ", "))
	}
	if len(r.MissingRecipes) > 0 {
		parts = append(parts, "missing recipes: "+strings.Join(r.MissingRecipes, ", "))
	}
	if len(r.InvalidDensityScaling) > 0 {
		parts = append(parts, "invalid density scaling: "+strings.Join(r.InvalidDensityScaling, ", "))
	}
	return fmt.Errorf("theme/templates: template %q (%s) failed validation: %s", r.ThemeName, r.Density, strings.Join(parts, "; "))
}

// ValidationReport returns the detailed authoring report for a density mode.
func (t TemplateTheme) ValidationReport(mode DensityMode) ValidationReport {
	return ValidationReport{
		ThemeName:             t.Name,
		Density:               mode,
		MissingTokens:         t.missingTokens(),
		MissingRecipes:        t.missingRecipes(),
		InvalidDensityScaling: t.invalidDensityScaling(),
		ChartFallbackFields:   t.Charts.FallbackFields(),
	}
}

// Validate performs a structural sanity check on the theme record.
func (t TemplateTheme) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("theme/templates: template theme name is required")
	}
	report := t.ValidationReport(t.Metadata.BaselineDensity)
	if !t.Metadata.Supports(t.Metadata.BaselineDensity) {
		report.InvalidDensityScaling = append(report.InvalidDensityScaling,
			fmt.Sprintf("baseline density %s is not supported", t.Metadata.BaselineDensity))
	}
	if err := t.Fonts.Validate(); err != nil {
		report.MissingTokens = append(report.MissingTokens, err.Error())
	}
	if !report.OK() {
		return report.Error()
	}
	return nil
}

// missingTokens returns the canonical missing-token list.
func (t TemplateTheme) missingTokens() []string {
	missing := make([]string, 0, 8)
	missing = append(missing, t.Tokens.Color.MissingRoles()...)
	missing = append(missing, t.Tokens.Typography.MissingRoles()...)
	missing = append(missing, t.Tokens.Metrics.MissingRoles()...)
	return missing
}

// missingRecipes returns the canonical missing-recipe list.
func (t TemplateTheme) missingRecipes() []string {
	return t.Recipes.MissingFamilies()
}

// invalidDensityScaling returns metric fields whose density tables are not monotonic.
func (t TemplateTheme) invalidDensityScaling() []string {
	return t.Tokens.Metrics.InvalidDensityTriples()
}

// MissingRoles returns the semantic color roles that were left unset.
func (c ColorTokens) MissingRoles() []string {
	missing := make([]string, 0, 4)
	check := func(name string, color gfx.Color) {
		if isUnsetColor(color) {
			missing = append(missing, name)
		}
	}
	check("Color.Background", c.Background)
	check("Color.Surface", c.Surface)
	check("Color.SurfaceVariant", c.SurfaceVariant)
	check("Color.SurfaceContainerLow", c.SurfaceContainerLow)
	check("Color.SurfaceContainer", c.SurfaceContainer)
	check("Color.SurfaceContainerHigh", c.SurfaceContainerHigh)
	check("Color.SurfaceInverse", c.SurfaceInverse)
	check("Color.OnBackground", c.OnBackground)
	check("Color.OnSurface", c.OnSurface)
	check("Color.OnSurfaceVariant", c.OnSurfaceVariant)
	check("Color.Outline", c.Outline)
	check("Color.OutlineVariant", c.OutlineVariant)
	check("Color.Primary", c.Primary)
	check("Color.OnPrimary", c.OnPrimary)
	check("Color.PrimaryContainer", c.PrimaryContainer)
	check("Color.OnPrimaryContainer", c.OnPrimaryContainer)
	check("Color.Secondary", c.Secondary)
	check("Color.OnSecondary", c.OnSecondary)
	check("Color.SecondaryContainer", c.SecondaryContainer)
	check("Color.OnSecondaryContainer", c.OnSecondaryContainer)
	check("Color.Tertiary", c.Tertiary)
	check("Color.OnTertiary", c.OnTertiary)
	check("Color.TertiaryContainer", c.TertiaryContainer)
	check("Color.OnTertiaryContainer", c.OnTertiaryContainer)
	check("Color.Error", c.Error)
	check("Color.OnError", c.OnError)
	check("Color.Warning", c.Warning)
	check("Color.OnWarning", c.OnWarning)
	check("Color.Success", c.Success)
	check("Color.OnSuccess", c.OnSuccess)
	check("Color.Info", c.Info)
	check("Color.OnInfo", c.OnInfo)
	check("Color.AxisStrong", c.AxisStrong)
	check("Color.AxisSubtle", c.AxisSubtle)
	check("Color.GridStrong", c.GridStrong)
	check("Color.GridSubtle", c.GridSubtle)
	if len(c.DataPalette) == 0 {
		missing = append(missing, "Color.DataPalette")
	}
	return missing
}

func isUnsetColor(c gfx.Color) bool {
	return c.R == 0 && c.G == 0 && c.B == 0 && c.A == 0
}

// MissingRoles returns the semantic typography roles that were left unset.
func (t TypographyTokens) MissingRoles() []string {
	missing := make([]string, 0, 4)
	check := func(name string, style text.TextStyle) {
		if style.Size <= 0 || style.LineHeight <= 0 {
			missing = append(missing, name)
		}
	}
	check("Typography.DisplayLarge", t.DisplayLarge)
	check("Typography.DisplayMedium", t.DisplayMedium)
	check("Typography.DisplaySmall", t.DisplaySmall)
	check("Typography.HeadlineLarge", t.HeadlineLarge)
	check("Typography.HeadlineMedium", t.HeadlineMedium)
	check("Typography.HeadlineSmall", t.HeadlineSmall)
	check("Typography.TitleLarge", t.TitleLarge)
	check("Typography.TitleMedium", t.TitleMedium)
	check("Typography.TitleSmall", t.TitleSmall)
	check("Typography.LabelLarge", t.LabelLarge)
	check("Typography.LabelMedium", t.LabelMedium)
	check("Typography.LabelSmall", t.LabelSmall)
	check("Typography.BodyLarge", t.BodyLarge)
	check("Typography.BodyMedium", t.BodyMedium)
	check("Typography.BodySmall", t.BodySmall)
	check("Typography.MonoLarge", t.MonoLarge)
	check("Typography.MonoMedium", t.MonoMedium)
	check("Typography.MonoSmall", t.MonoSmall)
	return missing
}

// MissingRoles returns the metric families that are missing or invalid.
func (m MetricTokens) MissingRoles() []string {
	missing := make([]string, 0, 8)
	missing = append(missing, m.Control.MissingRoles()...)
	missing = append(missing, m.Input.MissingRoles()...)
	missing = append(missing, m.Navigation.MissingRoles()...)
	missing = append(missing, m.Notification.MissingRoles()...)
	missing = append(missing, m.Annotation.MissingRoles()...)
	missing = append(missing, m.Chart.MissingRoles()...)
	return missing
}

// InvalidDensityTriples returns metric fields whose density values are not usable.
func (m MetricTokens) InvalidDensityTriples() []string {
	invalid := make([]string, 0, 8)
	invalid = append(invalid, m.Control.InvalidDensityTriples()...)
	invalid = append(invalid, m.Input.InvalidDensityTriples()...)
	invalid = append(invalid, m.Navigation.InvalidDensityTriples()...)
	invalid = append(invalid, m.Notification.InvalidDensityTriples()...)
	invalid = append(invalid, m.Annotation.InvalidDensityTriples()...)
	invalid = append(invalid, m.Chart.InvalidDensityTriples()...)
	return invalid
}

func (t ControlMetricTokens) MissingRoles() []string {
	return namedTriplesAsMissing(
		namedTriplet{"Metric.Control.Height", t.Height},
		namedTriplet{"Metric.Control.HorizontalPadding", t.HorizontalPadding},
		namedTriplet{"Metric.Control.VerticalPadding", t.VerticalPadding},
		namedTriplet{"Metric.Control.LabelGap", t.LabelGap},
		namedTriplet{"Metric.Control.FocusRingInset", t.FocusRingInset},
		namedTriplet{"Metric.Control.FocusRingThickness", t.FocusRingThickness},
	)
}

func (t ControlMetricTokens) InvalidDensityTriples() []string {
	return namedTriplesInvalid(
		namedTriplet{"Metric.Control.Height", t.Height},
		namedTriplet{"Metric.Control.HorizontalPadding", t.HorizontalPadding},
		namedTriplet{"Metric.Control.VerticalPadding", t.VerticalPadding},
		namedTriplet{"Metric.Control.LabelGap", t.LabelGap},
		namedTriplet{"Metric.Control.FocusRingInset", t.FocusRingInset},
		namedTriplet{"Metric.Control.FocusRingThickness", t.FocusRingThickness},
	)
}

func (t InputMetricTokens) MissingRoles() []string {
	return namedTriplesAsMissing(
		namedTriplet{"Metric.Input.CheckboxSize", t.CheckboxSize},
		namedTriplet{"Metric.Input.RadioSize", t.RadioSize},
		namedTriplet{"Metric.Input.SwitchTrackWidth", t.SwitchTrackWidth},
		namedTriplet{"Metric.Input.SwitchTrackHeight", t.SwitchTrackHeight},
		namedTriplet{"Metric.Input.SwitchThumbSize", t.SwitchThumbSize},
		namedTriplet{"Metric.Input.SliderTrackThickness", t.SliderTrackThickness},
		namedTriplet{"Metric.Input.SliderThumbSize", t.SliderThumbSize},
		namedTriplet{"Metric.Input.SliderTickSize", t.SliderTickSize},
		namedTriplet{"Metric.Input.TextFieldMinWidth", t.TextFieldMinWidth},
		namedTriplet{"Metric.Input.TextFieldPaddingX", t.TextFieldPaddingX},
		namedTriplet{"Metric.Input.TextFieldPaddingY", t.TextFieldPaddingY},
		namedTriplet{"Metric.Input.CaretThickness", t.CaretThickness},
	)
}

func (t InputMetricTokens) InvalidDensityTriples() []string {
	return namedTriplesInvalid(
		namedTriplet{"Metric.Input.CheckboxSize", t.CheckboxSize},
		namedTriplet{"Metric.Input.RadioSize", t.RadioSize},
		namedTriplet{"Metric.Input.SwitchTrackWidth", t.SwitchTrackWidth},
		namedTriplet{"Metric.Input.SwitchTrackHeight", t.SwitchTrackHeight},
		namedTriplet{"Metric.Input.SwitchThumbSize", t.SwitchThumbSize},
		namedTriplet{"Metric.Input.SliderTrackThickness", t.SliderTrackThickness},
		namedTriplet{"Metric.Input.SliderThumbSize", t.SliderThumbSize},
		namedTriplet{"Metric.Input.SliderTickSize", t.SliderTickSize},
		namedTriplet{"Metric.Input.TextFieldMinWidth", t.TextFieldMinWidth},
		namedTriplet{"Metric.Input.TextFieldPaddingX", t.TextFieldPaddingX},
		namedTriplet{"Metric.Input.TextFieldPaddingY", t.TextFieldPaddingY},
		namedTriplet{"Metric.Input.CaretThickness", t.CaretThickness},
	)
}

func (t NavigationMetricTokens) MissingRoles() []string {
	return namedTriplesAsMissing(
		namedTriplet{"Metric.Navigation.TabHeight", t.TabHeight},
		namedTriplet{"Metric.Navigation.TabIndicatorThickness", t.TabIndicatorThickness},
		namedTriplet{"Metric.Navigation.MenuRowHeight", t.MenuRowHeight},
		namedTriplet{"Metric.Navigation.MenuPadding", t.MenuPadding},
		namedTriplet{"Metric.Navigation.DrawerMinWidth", t.DrawerMinWidth},
		namedTriplet{"Metric.Navigation.DrawerMaxWidth", t.DrawerMaxWidth},
		namedTriplet{"Metric.Navigation.ScrollbarThickness", t.ScrollbarThickness},
		namedTriplet{"Metric.Navigation.PaginationItemSize", t.PaginationItemSize},
		namedTriplet{"Metric.Navigation.BreadcrumbGap", t.BreadcrumbGap},
		namedTriplet{"Metric.Navigation.SpeedDialActionSpacing", t.SpeedDialActionSpacing},
	)
}

func (t NavigationMetricTokens) InvalidDensityTriples() []string {
	return namedTriplesInvalid(
		namedTriplet{"Metric.Navigation.TabHeight", t.TabHeight},
		namedTriplet{"Metric.Navigation.TabIndicatorThickness", t.TabIndicatorThickness},
		namedTriplet{"Metric.Navigation.MenuRowHeight", t.MenuRowHeight},
		namedTriplet{"Metric.Navigation.MenuPadding", t.MenuPadding},
		namedTriplet{"Metric.Navigation.DrawerMinWidth", t.DrawerMinWidth},
		namedTriplet{"Metric.Navigation.DrawerMaxWidth", t.DrawerMaxWidth},
		namedTriplet{"Metric.Navigation.ScrollbarThickness", t.ScrollbarThickness},
		namedTriplet{"Metric.Navigation.PaginationItemSize", t.PaginationItemSize},
		namedTriplet{"Metric.Navigation.BreadcrumbGap", t.BreadcrumbGap},
		namedTriplet{"Metric.Navigation.SpeedDialActionSpacing", t.SpeedDialActionSpacing},
	)
}

func (t NotificationMetricTokens) MissingRoles() []string {
	return namedTriplesAsMissing(
		namedTriplet{"Metric.Notification.DialogMinWidth", t.DialogMinWidth},
		namedTriplet{"Metric.Notification.DialogMaxWidth", t.DialogMaxWidth},
		namedTriplet{"Metric.Notification.DialogPadding", t.DialogPadding},
		namedTriplet{"Metric.Notification.DialogActionGap", t.DialogActionGap},
		namedTriplet{"Metric.Notification.SnackbarMinHeight", t.SnackbarMinHeight},
		namedTriplet{"Metric.Notification.SnackbarMaxWidth", t.SnackbarMaxWidth},
		namedTriplet{"Metric.Notification.SnackbarPadding", t.SnackbarPadding},
		namedTriplet{"Metric.Notification.ProgressLinearThickness", t.ProgressLinearThickness},
		namedTriplet{"Metric.Notification.ProgressCircularStroke", t.ProgressCircularStroke},
	)
}

func (t NotificationMetricTokens) InvalidDensityTriples() []string {
	return namedTriplesInvalid(
		namedTriplet{"Metric.Notification.DialogMinWidth", t.DialogMinWidth},
		namedTriplet{"Metric.Notification.DialogMaxWidth", t.DialogMaxWidth},
		namedTriplet{"Metric.Notification.DialogPadding", t.DialogPadding},
		namedTriplet{"Metric.Notification.DialogActionGap", t.DialogActionGap},
		namedTriplet{"Metric.Notification.SnackbarMinHeight", t.SnackbarMinHeight},
		namedTriplet{"Metric.Notification.SnackbarMaxWidth", t.SnackbarMaxWidth},
		namedTriplet{"Metric.Notification.SnackbarPadding", t.SnackbarPadding},
		namedTriplet{"Metric.Notification.ProgressLinearThickness", t.ProgressLinearThickness},
		namedTriplet{"Metric.Notification.ProgressCircularStroke", t.ProgressCircularStroke},
	)
}

func (t AnnotationMetricTokens) MissingRoles() []string {
	return namedTriplesAsMissing(
		namedTriplet{"Metric.Annotation.BadgeMinHeight", t.BadgeMinHeight},
		namedTriplet{"Metric.Annotation.BadgePaddingX", t.BadgePaddingX},
		namedTriplet{"Metric.Annotation.LabelPadding", t.LabelPadding},
		namedTriplet{"Metric.Annotation.HandleVisualSize", t.HandleVisualSize},
		namedTriplet{"Metric.Annotation.HandleHitExpansion", t.HandleHitExpansion},
		namedTriplet{"Metric.Annotation.CalloutGap", t.CalloutGap},
		namedTriplet{"Metric.Annotation.ConnectorStrokeDefault", t.ConnectorStrokeDefault},
		namedTriplet{"Metric.Annotation.RuleStrokeDefault", t.RuleStrokeDefault},
	)
}

func (t AnnotationMetricTokens) InvalidDensityTriples() []string {
	return namedTriplesInvalid(
		namedTriplet{"Metric.Annotation.BadgeMinHeight", t.BadgeMinHeight},
		namedTriplet{"Metric.Annotation.BadgePaddingX", t.BadgePaddingX},
		namedTriplet{"Metric.Annotation.LabelPadding", t.LabelPadding},
		namedTriplet{"Metric.Annotation.HandleVisualSize", t.HandleVisualSize},
		namedTriplet{"Metric.Annotation.HandleHitExpansion", t.HandleHitExpansion},
		namedTriplet{"Metric.Annotation.CalloutGap", t.CalloutGap},
		namedTriplet{"Metric.Annotation.ConnectorStrokeDefault", t.ConnectorStrokeDefault},
		namedTriplet{"Metric.Annotation.RuleStrokeDefault", t.RuleStrokeDefault},
	)
}

func (t ChartMetricTokens) MissingRoles() []string {
	return namedTriplesAsMissing(
		namedTriplet{"Metric.Chart.AxisTickLength", t.AxisTickLength},
		namedTriplet{"Metric.Chart.AxisLabelGap", t.AxisLabelGap},
		namedTriplet{"Metric.Chart.AxisTitleGap", t.AxisTitleGap},
		namedTriplet{"Metric.Chart.GridStrokeThin", t.GridStrokeThin},
		namedTriplet{"Metric.Chart.GridStrokeStrong", t.GridStrokeStrong},
		namedTriplet{"Metric.Chart.SymbolDefaultSize", t.SymbolDefaultSize},
	)
}

func (t ChartMetricTokens) InvalidDensityTriples() []string {
	return namedTriplesInvalid(
		namedTriplet{"Metric.Chart.AxisTickLength", t.AxisTickLength},
		namedTriplet{"Metric.Chart.AxisLabelGap", t.AxisLabelGap},
		namedTriplet{"Metric.Chart.AxisTitleGap", t.AxisTitleGap},
		namedTriplet{"Metric.Chart.GridStrokeThin", t.GridStrokeThin},
		namedTriplet{"Metric.Chart.GridStrokeStrong", t.GridStrokeStrong},
		namedTriplet{"Metric.Chart.SymbolDefaultSize", t.SymbolDefaultSize},
	)
}

type namedTriplet struct {
	Name    string
	Triplet DensityTriplet
}

func namedTriplesInvalid(triples ...namedTriplet) []string {
	invalid := make([]string, 0, len(triples))
	for _, named := range triples {
		triplet := named.Triplet
		if triplet.Compact <= 0 || triplet.Regular <= 0 || triplet.Touchspread <= 0 {
			invalid = append(invalid, named.Name+" has unset density values")
			continue
		}
		if triplet.Compact > triplet.Regular || triplet.Regular > triplet.Touchspread {
			invalid = append(invalid, named.Name+" is not monotonic compact<=regular<=touchspread")
		}
	}
	return invalid
}

func namedTriplesAsMissing(triples ...namedTriplet) []string {
	missing := make([]string, 0, len(triples))
	for _, named := range triples {
		triplet := named.Triplet
		if triplet.Compact <= 0 || triplet.Regular <= 0 || triplet.Touchspread <= 0 {
			missing = append(missing, named.Name)
		}
	}
	return missing
}

// MissingFamilies returns family names with empty or invalid declarations.
func (b RecipeBundle) MissingFamilies() []string {
	missing := make([]string, 0, 5)
	check := func(name string, bundle FamilyRecipeBundle) {
		if bundle.Family == "" || len(bundle.Variants) == 0 {
			missing = append(missing, name)
		}
	}
	check("annotation", b.Annotation)
	check("uiinput", b.UIInput)
	check("uinav", b.UINav)
	check("uinotification", b.UINotification)
	check("chart", b.Chart)
	return missing
}

// FallbackFields returns the chart fields resolved from the root theme.
func (c ChartInheritance) FallbackFields() []string {
	fields := make([]string, 0, 5)
	if len(c.DataPalette) == 0 {
		fields = append(fields, "Chart.DataPalette")
	}
	if c.AxisStrong == nil {
		fields = append(fields, "Chart.AxisStrong")
	}
	if c.AxisSubtle == nil {
		fields = append(fields, "Chart.AxisSubtle")
	}
	if c.GridStrong == nil {
		fields = append(fields, "Chart.GridStrong")
	}
	if c.GridSubtle == nil {
		fields = append(fields, "Chart.GridSubtle")
	}
	return fields
}

// ChartUsesFallback reports whether any chart values inherit from the root theme.
func (c ChartInheritance) ChartUsesFallback() bool {
	return len(c.FallbackFields()) > 0
}

// IsSupportedDensity reports whether the density mode is one of the canonical modes.
func (m DensityMode) IsSupportedDensity() bool {
	switch m {
	case DensityCompact, DensityRegular, DensityTouchspread:
		return true
	default:
		return false
	}
}
