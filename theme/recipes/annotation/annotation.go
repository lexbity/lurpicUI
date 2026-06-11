package annotation

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// LabelVariant selects the label recipe shape.
type LabelVariant uint8

const (
	LabelStandard LabelVariant = iota
	LabelCompact
)

func (v LabelVariant) String() string {
	switch v {
	case LabelStandard:
		return "standard"
	case LabelCompact:
		return "compact"
	default:
		return "unknown"
	}
}

// ResolveLabelRecipe resolves the label slots and provenance.
func ResolveLabelRecipe(ctx theme.StyleContext, variant LabelVariant, overrides ...theme.SlotPatch[shared.LabelSlots]) (shared.LabelSlots, theme.RecipeReport) {
	slots := labelBase(ctx, variant)
	report := newReport("annotation", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveHandleRecipe resolves the handle slots and provenance.
func ResolveHandleRecipe(ctx theme.StyleContext, variant theme.VariantKey, overrides ...theme.SlotPatch[shared.HandleSlots]) (shared.HandleSlots, theme.RecipeReport) {
	slots := handleBase(ctx, variant)
	report := newReport("annotation", variant, slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func labelBase(ctx theme.StyleContext, variant LabelVariant) shared.LabelSlots {
	tokens := ctx.Tokens
	switch variant {
	case LabelCompact:
		return shared.LabelSlots{
			Text:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Icon:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Container: fadedStyle(tokens.Color.SurfaceVariant, 0.86),
			Underline: strokeStyle(tokens.Color.OnSurfaceVariant, 1),
		}
	default:
		return shared.LabelSlots{
			Text:      markStyleFromColor(tokens.Color.OnSurface),
			Icon:      markStyleFromColor(tokens.Color.OnSurface),
			Container: markStyleFromColor(tokens.Color.Surface),
			Underline: strokeStyle(tokens.Color.Primary, 1.5),
		}
	}
}

func handleBase(ctx theme.StyleContext, variant theme.VariantKey) shared.HandleSlots {
	tokens := ctx.Tokens
	switch variant {
	case theme.VariantKey("compact"):
		return shared.HandleSlots{
			Visible:   markStyleFromColor(tokens.Color.Primary),
			Hover:     fadedStyle(tokens.Color.Primary, 0.8),
			Focused:   strokeStyle(tokens.Color.Primary, 2),
			DragGhost: fadedStyle(tokens.Color.Primary, 0.35),
		}
	default:
		return shared.HandleSlots{
			Visible:   markStyleFromColor(tokens.Color.Primary),
			Hover:     fadedStyle(tokens.Color.Primary, 0.7),
			Focused:   strokeStyle(tokens.Color.Primary, 2),
			DragGhost: fadedStyle(tokens.Color.Primary, 0.3),
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

func strokeStyle(color gfx.Color, width float32) theme.MarkStyle {
	return theme.MarkStyle{
		Base: theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillNone,
				Color:   color,
				Opacity: 0,
			}},
			Strokes: []theme.MaterialStroke{{
				Paint: theme.Fill{
					Type:    theme.FillSolid,
					Color:   color,
					Opacity: 1,
				},
				Width: width,
			}},
			Opacity: 1,
		},
	}
}
