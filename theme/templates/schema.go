package templates

import (
	"fmt"
	"sort"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// DensityMode identifies the template density profile.
type DensityMode uint8

const (
	DensityCompact DensityMode = iota
	DensityRegular
	DensityTouchspread
)

func (m DensityMode) String() string {
	switch m {
	case DensityCompact:
		return "compact"
	case DensityRegular:
		return "regular"
	case DensityTouchspread:
		return "touchspread"
	default:
		return "unknown"
	}
}

// TypographyScale returns the conservative typography multiplier for a density mode.
func (m DensityMode) TypographyScale() float32 {
	switch m {
	case DensityCompact:
		return 0.93
	case DensityRegular:
		return 1.0
	case DensityTouchspread:
		return 1.08
	default:
		return 1.0
	}
}

// DensityTriplet stores the compact/regular/touchspread values for a metric.
type DensityTriplet struct {
	Compact     float32
	Regular     float32
	Touchspread float32
}

// For returns the value for the supplied density mode.
func (t DensityTriplet) For(mode DensityMode) float32 {
	switch mode {
	case DensityCompact:
		return t.Compact
	case DensityTouchspread:
		return t.Touchspread
	case DensityRegular:
		fallthrough
	default:
		return t.Regular
	}
}

// ScaleTypographySize applies the density typography scale to a size value.
func ScaleTypographySize(size float32, mode DensityMode) float32 {
	return size * mode.TypographyScale()
}

// ScaleTypographyLineHeight scales a line-height value using the same density rule.
func ScaleTypographyLineHeight(lineHeight float32, mode DensityMode) float32 {
	return lineHeight * mode.TypographyScale()
}

// ColorTokens groups canonical semantic color roles.
type ColorTokens struct {
	Background           gfx.Color
	Surface              gfx.Color
	SurfaceVariant       gfx.Color
	SurfaceContainerLow  gfx.Color
	SurfaceContainer     gfx.Color
	SurfaceContainerHigh gfx.Color
	SurfaceInverse       gfx.Color
	OnBackground         gfx.Color
	OnSurface            gfx.Color
	OnSurfaceVariant     gfx.Color
	Outline              gfx.Color
	OutlineVariant       gfx.Color
	Primary              gfx.Color
	OnPrimary            gfx.Color
	PrimaryContainer     gfx.Color
	OnPrimaryContainer   gfx.Color
	Secondary            gfx.Color
	OnSecondary          gfx.Color
	SecondaryContainer   gfx.Color
	OnSecondaryContainer gfx.Color
	Tertiary             gfx.Color
	OnTertiary           gfx.Color
	TertiaryContainer    gfx.Color
	OnTertiaryContainer  gfx.Color
	Error                gfx.Color
	OnError              gfx.Color
	Warning              gfx.Color
	OnWarning            gfx.Color
	Success              gfx.Color
	OnSuccess            gfx.Color
	Info                 gfx.Color
	OnInfo               gfx.Color
	HoverOpacity         float32
	PressedOpacity       float32
	FocusOpacity         float32
	DisabledOpacity      float32
	SelectionOpacity     float32
	DataPalette          []gfx.Color
	AxisStrong           gfx.Color
	AxisSubtle           gfx.Color
	GridStrong           gfx.Color
	GridSubtle           gfx.Color
}

// TypographyTokens groups canonical semantic text roles.
type TypographyTokens struct {
	DisplayLarge   text.TextStyle
	DisplayMedium  text.TextStyle
	DisplaySmall   text.TextStyle
	HeadlineLarge  text.TextStyle
	HeadlineMedium text.TextStyle
	HeadlineSmall  text.TextStyle
	TitleLarge     text.TextStyle
	TitleMedium    text.TextStyle
	TitleSmall     text.TextStyle
	LabelLarge     text.TextStyle
	LabelMedium    text.TextStyle
	LabelSmall     text.TextStyle
	BodyLarge      text.TextStyle
	BodyMedium     text.TextStyle
	BodySmall      text.TextStyle
	MonoLarge      text.TextStyle
	MonoMedium     text.TextStyle
	MonoSmall      text.TextStyle
}

// MetricTokens groups density-aware metric tables.
type MetricTokens struct {
	Control      ControlMetricTokens
	Input        InputMetricTokens
	Navigation   NavigationMetricTokens
	Notification NotificationMetricTokens
	Annotation   AnnotationMetricTokens
	Chart        ChartMetricTokens
}

// ControlMetricTokens covers the control-height baseline.
type ControlMetricTokens struct {
	Height             DensityTriplet
	HorizontalPadding  DensityTriplet
	VerticalPadding    DensityTriplet
	LabelGap           DensityTriplet
	FocusRingInset     DensityTriplet
	FocusRingThickness DensityTriplet
}

// InputMetricTokens covers the interactive input anatomy.
type InputMetricTokens struct {
	CheckboxSize         DensityTriplet
	RadioSize            DensityTriplet
	SwitchTrackWidth     DensityTriplet
	SwitchTrackHeight    DensityTriplet
	SwitchThumbSize      DensityTriplet
	SliderTrackThickness DensityTriplet
	SliderThumbSize      DensityTriplet
	SliderTickSize       DensityTriplet
	TextFieldMinWidth    DensityTriplet
	TextFieldPaddingX    DensityTriplet
	TextFieldPaddingY    DensityTriplet
	CaretThickness       DensityTriplet
}

// NavigationMetricTokens covers menu/drawer/tab anatomy.
type NavigationMetricTokens struct {
	TabHeight              DensityTriplet
	TabIndicatorThickness  DensityTriplet
	MenuRowHeight          DensityTriplet
	MenuPadding            DensityTriplet
	DrawerMinWidth         DensityTriplet
	DrawerMaxWidth         DensityTriplet
	ScrollbarThickness     DensityTriplet
	PaginationItemSize     DensityTriplet
	BreadcrumbGap          DensityTriplet
	SpeedDialActionSpacing DensityTriplet
}

// NotificationMetricTokens covers dialog, snackbar, and progress anatomy.
type NotificationMetricTokens struct {
	DialogMinWidth          DensityTriplet
	DialogMaxWidth          DensityTriplet
	DialogPadding           DensityTriplet
	DialogActionGap         DensityTriplet
	SnackbarMinHeight       DensityTriplet
	SnackbarMaxWidth        DensityTriplet
	SnackbarPadding         DensityTriplet
	ProgressLinearThickness DensityTriplet
	ProgressCircularStroke  DensityTriplet
}

// AnnotationMetricTokens covers annotation/control helper anatomy.
type AnnotationMetricTokens struct {
	BadgeMinHeight         DensityTriplet
	BadgePaddingX          DensityTriplet
	LabelPadding           DensityTriplet
	HandleVisualSize       DensityTriplet
	HandleHitExpansion     DensityTriplet
	CalloutGap             DensityTriplet
	ConnectorStrokeDefault DensityTriplet
	RuleStrokeDefault      DensityTriplet
}

// ChartMetricTokens covers axis and chart-adjacent spacing.
type ChartMetricTokens struct {
	AxisTickLength    DensityTriplet
	AxisLabelGap      DensityTriplet
	AxisTitleGap      DensityTriplet
	GridStrokeThin    DensityTriplet
	GridStrokeStrong  DensityTriplet
	SymbolDefaultSize DensityTriplet
}

// ShapeTokens defines the radius scale.
type ShapeTokens struct {
	RadiusNone float32
	RadiusXS   float32
	RadiusSM   float32
	RadiusMD   float32
	RadiusLG   float32
	RadiusXL   float32
	RadiusFull float32
}

// SpringToken describes a spring timing profile.
type SpringToken struct {
	Tension  float32
	Friction float32
}

// MotionTokens defines timing and easing tokens.
type MotionTokens struct {
	DurationFast     time.Duration
	DurationMedium   time.Duration
	DurationSlow     time.Duration
	EasingStandard   string
	EasingEmphasized string
	EasingExit       string
	SpringLight      SpringToken
	SpringMedium     SpringToken
}

// Tokens is the canonical template-theme token set.
type Tokens struct {
	Color      ColorTokens
	Typography TypographyTokens
	Metrics    MetricTokens
	Shape      ShapeTokens
	Motion     MotionTokens
}

// ThemeMetadata carries template-level flags.
type ThemeMetadata struct {
	Dark                bool
	BaselineDensity     DensityMode
	SupportsCompact     bool
	SupportsRegular     bool
	SupportsTouchspread bool
}

// Supports reports whether a density mode is advertised by this template.
func (m ThemeMetadata) Supports(mode DensityMode) bool {
	switch mode {
	case DensityCompact:
		return m.SupportsCompact
	case DensityTouchspread:
		return m.SupportsTouchspread
	case DensityRegular:
		fallthrough
	default:
		return m.SupportsRegular
	}
}

// FamilyRecipeBundle records the authored variants available for one family.
type FamilyRecipeBundle struct {
	Family   string
	Variants []string
}

// NewFamilyRecipeBundle constructs a family bundle with a copied variant list.
func NewFamilyRecipeBundle(family string, variants ...string) FamilyRecipeBundle {
	out := FamilyRecipeBundle{Family: family}
	if len(variants) > 0 {
		out.Variants = append([]string(nil), variants...)
	}
	return out
}

// Validate checks that the family bundle contains a family name and variants.
func (b FamilyRecipeBundle) Validate() error {
	if b.Family == "" {
		return fmt.Errorf("recipe family bundle is missing a family name")
	}
	if len(b.Variants) == 0 {
		return fmt.Errorf("recipe family bundle %q has no variants", b.Family)
	}
	return nil
}

// RecipeBundle groups the family-level recipe declarations.
type RecipeBundle struct {
	Annotation     FamilyRecipeBundle
	UIInput        FamilyRecipeBundle
	UINav          FamilyRecipeBundle
	UINotification FamilyRecipeBundle
	Chart          FamilyRecipeBundle
}

// NewRecipeBundle constructs a bundle from the canonical family slots.
func NewRecipeBundle(annotation, uiinput, uinav, uinotification, chart FamilyRecipeBundle) RecipeBundle {
	return RecipeBundle{
		Annotation:     annotation,
		UIInput:        uiinput,
		UINav:          uinav,
		UINotification: uinotification,
		Chart:          chart,
	}
}

// Families returns the declared family bundles in deterministic order.
func (b RecipeBundle) Families() []FamilyRecipeBundle {
	out := make([]FamilyRecipeBundle, 0, 5)
	if b.Annotation.Family != "" || len(b.Annotation.Variants) > 0 {
		out = append(out, b.Annotation)
	}
	if b.UIInput.Family != "" || len(b.UIInput.Variants) > 0 {
		out = append(out, b.UIInput)
	}
	if b.UINav.Family != "" || len(b.UINav.Variants) > 0 {
		out = append(out, b.UINav)
	}
	if b.UINotification.Family != "" || len(b.UINotification.Variants) > 0 {
		out = append(out, b.UINotification)
	}
	if b.Chart.Family != "" || len(b.Chart.Variants) > 0 {
		out = append(out, b.Chart)
	}
	return out
}

// Lookup returns the named family bundle if present.
func (b RecipeBundle) Lookup(family string) (FamilyRecipeBundle, bool) {
	for _, candidate := range b.Families() {
		if candidate.Family == family {
			return candidate, true
		}
	}
	return FamilyRecipeBundle{}, false
}

// BundleNames returns the declared family names in deterministic order.
func (b RecipeBundle) BundleNames() []string {
	families := b.Families()
	if len(families) == 0 {
		return nil
	}
	out := make([]string, 0, len(families))
	for _, family := range families {
		out = append(out, family.Family)
	}
	sort.Strings(out)
	return out
}

// Validate checks that the bundle has usable family declarations.
func (b RecipeBundle) Validate() error {
	families := b.Families()
	if len(families) == 0 {
		return fmt.Errorf("recipe bundle is empty")
	}
	for _, family := range families {
		if err := family.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ChartInheritance defines the fallback contract for chart presentation.
type ChartInheritance struct {
	InheritTypography bool
	InheritSurface    bool
	InheritDensity    bool
	InheritMotion     bool
	InheritShape      bool
	DataPalette       []gfx.Color
	AxisStrong        *gfx.Color
	AxisSubtle        *gfx.Color
	GridStrong        *gfx.Color
	GridSubtle        *gfx.Color
}

// ChartTokens is the resolved chart-specific presentation contract.
type ChartTokens struct {
	DataPalette []gfx.Color
	AxisStrong  gfx.Color
	AxisSubtle  gfx.Color
	GridStrong  gfx.Color
	GridSubtle  gfx.Color
}

// Resolve returns the chart presentation contract with root-theme fallback.
func (c ChartInheritance) Resolve(root ColorTokens) ChartTokens {
	out := ChartTokens{
		DataPalette: append([]gfx.Color(nil), c.DataPalette...),
		AxisStrong:  root.AxisStrong,
		AxisSubtle:  root.AxisSubtle,
		GridStrong:  root.GridStrong,
		GridSubtle:  root.GridSubtle,
	}
	if len(out.DataPalette) == 0 {
		out.DataPalette = append([]gfx.Color(nil), root.DataPalette...)
	}
	if c.AxisStrong != nil {
		out.AxisStrong = *c.AxisStrong
	}
	if c.AxisSubtle != nil {
		out.AxisSubtle = *c.AxisSubtle
	}
	if c.GridStrong != nil {
		out.GridStrong = *c.GridStrong
	}
	if c.GridSubtle != nil {
		out.GridSubtle = *c.GridSubtle
	}
	return out
}

// HasOverrides reports whether any chart-specific values were supplied.
func (c ChartInheritance) HasOverrides() bool {
	return len(c.DataPalette) > 0 ||
		c.AxisStrong != nil ||
		c.AxisSubtle != nil ||
		c.GridStrong != nil ||
		c.GridSubtle != nil
}

// TemplateTheme is the canonical template-theme record.
type TemplateTheme struct {
	Name      string
	Tokens    Tokens
	Fonts     theme.FontRoles
	Materials *theme.MaterialRegistry
	Recipes   RecipeBundle
	Metadata  ThemeMetadata
	Charts    ChartInheritance
}

// DeclaredVariants returns a sorted copy of the provided bundle variant names.
func DeclaredVariants(bundle FamilyRecipeBundle) []string {
	if len(bundle.Variants) == 0 {
		return nil
	}
	out := append([]string(nil), bundle.Variants...)
	sort.Strings(out)
	return out
}
