package chart

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// AxisVariant selects the axis recipe shape.
type AxisVariant uint8

const (
	// AxisStandard uses the default chart axis layout.
	AxisStandard AxisVariant = iota
	// AxisCompact uses denser typography and subtler grid lines.
	AxisCompact
)

func (v AxisVariant) String() string {
	switch v {
	case AxisStandard:
		return "standard"
	case AxisCompact:
		return "compact"
	default:
		return "unknown"
	}
}

// ResolveAxisRecipe resolves axis slots and provenance.
func ResolveAxisRecipe(ctx theme.StyleContext, variant AxisVariant, overrides ...theme.SlotPatch[shared.AxisSlots]) (shared.AxisSlots, theme.RecipeReport) {
	slots := axisBase(ctx, variant)
	report := newReport("chart", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func axisBase(ctx theme.StyleContext, variant AxisVariant) shared.AxisSlots {
	tokens := ctx.Tokens
	switch variant {
	case AxisStandard:
		return shared.AxisSlots{
			AxisLine:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Tick:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			TickLabel: markStyleFromColor(tokens.Color.OnSurface),
			GridLine:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.18),
			Title:     markStyleFromColor(tokens.Color.OnSurface),
		}
	case AxisCompact:
		return shared.AxisSlots{
			AxisLine:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Tick:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			TickLabel: markStyleFromColor(tokens.Color.OnSurfaceVariant),
			GridLine:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.0),
			Title:     markStyleFromColor(tokens.Color.OnSurface),
		}
	default:
		return shared.AxisSlots{}
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
