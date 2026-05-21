package uistatus

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// BadgeVariant selects the badge recipe shape.
type BadgeVariant uint8

const (
	// BadgeDefault uses the standard badge styling.
	BadgeDefault BadgeVariant = iota
	// BadgeDisabled uses the muted disabled styling.
	BadgeDisabled
)

func (v BadgeVariant) String() string {
	switch v {
	case BadgeDefault:
		return "default"
	case BadgeDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ResolveBadgeRecipe resolves the badge slots and provenance.
func ResolveBadgeRecipe(ctx theme.StyleContext, variant BadgeVariant, overrides ...theme.SlotPatch[shared.BadgeSlots]) (shared.BadgeSlots, theme.RecipeReport) {
	slots := badgeBase(ctx, variant)
	report := newReport("status", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// StatusLightVariant selects the status-light recipe shape.
type StatusLightVariant uint8

const (
	// StatusLightDefault uses the standard status-light styling.
	StatusLightDefault StatusLightVariant = iota
	// StatusLightDisabled uses the muted disabled styling.
	StatusLightDisabled
)

func (v StatusLightVariant) String() string {
	switch v {
	case StatusLightDefault:
		return "default"
	case StatusLightDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ResolveStatusLightRecipe resolves the status-light slots and provenance.
func ResolveStatusLightRecipe(ctx theme.StyleContext, variant StatusLightVariant, overrides ...theme.SlotPatch[shared.StatusLightSlots]) (shared.StatusLightSlots, theme.RecipeReport) {
	slots := statusLightBase(ctx, variant)
	report := newReport("status", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ProgressBarVariant selects the progress-bar recipe shape.
type ProgressBarVariant uint8

const (
	// ProgressBarDefault uses the standard progress-bar styling.
	ProgressBarDefault ProgressBarVariant = iota
	// ProgressBarDisabled uses the muted disabled styling.
	ProgressBarDisabled
)

func (v ProgressBarVariant) String() string {
	switch v {
	case ProgressBarDefault:
		return "default"
	case ProgressBarDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ResolveProgressBarRecipe resolves the progress-bar slots and provenance.
func ResolveProgressBarRecipe(ctx theme.StyleContext, variant ProgressBarVariant, overrides ...theme.SlotPatch[shared.StatusProgressBarSlots]) (shared.StatusProgressBarSlots, theme.RecipeReport) {
	slots := progressBarBase(ctx, variant)
	report := newReport("status", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ProgressRingVariant selects the progress-ring recipe shape.
type ProgressRingVariant uint8

const (
	// ProgressRingDefault uses the standard progress-ring styling.
	ProgressRingDefault ProgressRingVariant = iota
	// ProgressRingDisabled uses the muted disabled styling.
	ProgressRingDisabled
)

func (v ProgressRingVariant) String() string {
	switch v {
	case ProgressRingDefault:
		return "default"
	case ProgressRingDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ResolveProgressRingRecipe resolves the progress-ring slots and provenance.
func ResolveProgressRingRecipe(ctx theme.StyleContext, variant ProgressRingVariant, overrides ...theme.SlotPatch[shared.StatusProgressRingSlots]) (shared.StatusProgressRingSlots, theme.RecipeReport) {
	slots := progressRingBase(ctx, variant)
	report := newReport("status", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func badgeBase(ctx theme.StyleContext, variant BadgeVariant) shared.BadgeSlots {
	tokens := ctx.Tokens
	switch variant {
	case BadgeDisabled:
		return shared.BadgeSlots{
			Root:           transparentStyle(),
			BadgeContainer: fadedStyle(tokens.Color.OnSurfaceVariant, 0.14),
			Label:          markStyleFromColor(tokens.Color.OnSurfaceVariant),
			OptionalIcon:   fadedStyle(tokens.Color.OnSurfaceVariant, 0.18),
		}
	default:
		return shared.BadgeSlots{
			Root:           transparentStyle(),
			BadgeContainer: markStyleFromColor(tokens.Color.Primary),
			Label:          markStyleFromColor(tokens.Color.OnPrimary),
			OptionalIcon:   fadedStyle(tokens.Color.OnPrimary, 0.16),
		}
	}
}

func statusLightBase(ctx theme.StyleContext, variant StatusLightVariant) shared.StatusLightSlots {
	tokens := ctx.Tokens
	switch variant {
	case StatusLightDisabled:
		return shared.StatusLightSlots{
			Root:          transparentStyle(),
			Indicator:     fadedStyle(tokens.Color.OnSurfaceVariant, 0.18),
			LabelOptional: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	default:
		return shared.StatusLightSlots{
			Root:          transparentStyle(),
			Indicator:     markStyleFromColor(tokens.Color.Success),
			LabelOptional: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	}
}

func progressBarBase(ctx theme.StyleContext, variant ProgressBarVariant) shared.StatusProgressBarSlots {
	tokens := ctx.Tokens
	switch variant {
	case ProgressBarDisabled:
		return shared.StatusProgressBarSlots{
			Root:          fadedStyle(tokens.Color.OnSurfaceVariant, 0.12),
			Track:         fadedStyle(tokens.Color.OnSurfaceVariant, 0.08),
			Indicator:     fadedStyle(tokens.Color.OnSurfaceVariant, 0.4),
			OptionalLabel: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	default:
		return shared.StatusProgressBarSlots{
			Root:          fadedStyle(tokens.Color.SurfaceVariant, 0.96),
			Track:         fadedStyle(tokens.Color.OnSurfaceVariant, 0.18),
			Indicator:     markStyleFromColor(tokens.Color.Primary),
			OptionalLabel: markStyleFromColor(tokens.Color.OnSurface),
		}
	}
}

func progressRingBase(ctx theme.StyleContext, variant ProgressRingVariant) shared.StatusProgressRingSlots {
	tokens := ctx.Tokens
	switch variant {
	case ProgressRingDisabled:
		return shared.StatusProgressRingSlots{
			Root:          fadedStyle(tokens.Color.OnSurfaceVariant, 0.1),
			TrackArc:      fadedStyle(tokens.Color.OnSurfaceVariant, 0.08),
			IndicatorArc:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.36),
			OptionalLabel: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	default:
		return shared.StatusProgressRingSlots{
			Root:          fadedStyle(tokens.Color.SurfaceVariant, 0.12),
			TrackArc:      fadedStyle(tokens.Color.OnSurfaceVariant, 0.18),
			IndicatorArc:  markStyleFromColor(tokens.Color.Primary),
			OptionalLabel: markStyleFromColor(tokens.Color.OnSurface),
		}
	}
}

func newReport(family string, variant theme.VariantKey, slots any) theme.RecipeReport {
	report := theme.NewRecipeReport(family, variant)
	recordSlotSources(&report, slots, theme.SlotSourceVariantDefault)
	return report
}

func recordSlotSources(report *theme.RecipeReport, slots any, source theme.SlotSource) {
	value := reflect.ValueOf(slots)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return
	}
	typeOf := value.Type()
	for i := 0; i < value.NumField(); i++ {
		report.SetSlotSource(typeOf.Field(i).Name, source)
	}
}

func annotateOverrides[T any](report *theme.RecipeReport, base, resolved T) {
	baseValue := reflect.ValueOf(base)
	resolvedValue := reflect.ValueOf(resolved)
	if baseValue.Kind() == reflect.Pointer {
		baseValue = baseValue.Elem()
	}
	if resolvedValue.Kind() == reflect.Pointer {
		resolvedValue = resolvedValue.Elem()
	}
	if baseValue.Kind() != reflect.Struct || resolvedValue.Kind() != reflect.Struct {
		return
	}
	typeOf := baseValue.Type()
	for i := 0; i < baseValue.NumField() && i < resolvedValue.NumField(); i++ {
		if !reflect.DeepEqual(baseValue.Field(i).Interface(), resolvedValue.Field(i).Interface()) {
			report.SetSlotSource(typeOf.Field(i).Name, theme.SlotSourceInstanceOverride)
		}
	}
}

func markStyleFromColor(color gfx.Color) theme.MarkStyle {
	return theme.MarkStyle{Base: theme.FromToken(color)}
}

func fadedStyle(color gfx.Color, opacity float32) theme.MarkStyle {
	return theme.MarkStyle{
		Base: theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillSolid,
				Color:   color,
				Opacity: opacity,
			}},
			Opacity: 1,
		},
	}
}

func transparentStyle() theme.MarkStyle {
	return theme.MarkStyle{Base: theme.Material{Opacity: 0}}
}
