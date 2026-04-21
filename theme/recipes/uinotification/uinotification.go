package uinotification

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// DialogVariant selects the dialog recipe shape.
type DialogVariant uint8

const (
	DialogStandard DialogVariant = iota
	DialogDestructive
	DialogFullscreen
)

func (v DialogVariant) String() string {
	switch v {
	case DialogStandard:
		return "standard"
	case DialogDestructive:
		return "destructive"
	case DialogFullscreen:
		return "fullscreen"
	default:
		return "unknown"
	}
}

// ResolveSnackbarRecipe resolves snackbar styling.
func ResolveSnackbarRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.SnackbarSlots]) (shared.SnackbarSlots, theme.RecipeReport) {
	slots := snackbarBase(ctx)
	report := newReport("uinotification", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveDialogRecipe resolves dialog styling.
func ResolveDialogRecipe(ctx theme.StyleContext, variant DialogVariant, overrides ...theme.SlotPatch[shared.DialogSlots]) (shared.DialogSlots, theme.RecipeReport) {
	slots := dialogBase(ctx, variant)
	report := newReport("uinotification", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveProgressRecipe resolves progress styling.
func ResolveProgressRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ProgressSlots]) (shared.ProgressSlots, theme.RecipeReport) {
	slots := progressBase(ctx)
	report := newReport("uinotification", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func snackbarBase(ctx theme.StyleContext) shared.SnackbarSlots {
	tokens := ctx.Tokens
	return shared.SnackbarSlots{
		Container: markStyleFromColor(tokens.Color.SurfaceVariant),
		Text:      markStyleFromColor(tokens.Color.OnSurface),
		Action:    markStyleFromColor(tokens.Color.Primary),
	}
}

func dialogBase(ctx theme.StyleContext, variant DialogVariant) shared.DialogSlots {
	tokens := ctx.Tokens
	switch variant {
	case DialogDestructive:
		return shared.DialogSlots{
			Scrim:      fadedStyle(tokens.Color.Error, 0.32),
			Surface:    markStyleFromColor(tokens.Color.Surface),
			TitleText:  markStyleFromColor(tokens.Color.Error),
			BodyText:   markStyleFromColor(tokens.Color.OnSurface),
			ActionArea: markStyleFromColor(tokens.Color.SurfaceVariant),
			Outline:    strokeStyle(tokens.Color.Error, 2),
			Shadow:     fadedStyle(tokens.Color.Error, 0.18),
		}
	case DialogFullscreen:
		return shared.DialogSlots{
			Scrim:      fadedStyle(tokens.Color.Background, 0.5),
			Surface:    markStyleFromColor(tokens.Color.Background),
			TitleText:  markStyleFromColor(tokens.Color.OnSurface),
			BodyText:   markStyleFromColor(tokens.Color.OnSurface),
			ActionArea: markStyleFromColor(tokens.Color.SurfaceVariant),
			Outline:    strokeStyle(tokens.Color.Primary, 1),
			Shadow:     fadedStyle(tokens.Color.Background, 0.12),
		}
	default:
		return shared.DialogSlots{
			Scrim:      fadedStyle(tokens.Color.Background, 0.44),
			Surface:    markStyleFromColor(tokens.Color.Surface),
			TitleText:  markStyleFromColor(tokens.Color.OnSurface),
			BodyText:   markStyleFromColor(tokens.Color.OnSurface),
			ActionArea: markStyleFromColor(tokens.Color.SurfaceVariant),
			Outline:    strokeStyle(tokens.Color.Primary, 1),
			Shadow:     fadedStyle(tokens.Color.Background, 0.12),
		}
	}
}

func progressBase(ctx theme.StyleContext) shared.ProgressSlots {
	tokens := ctx.Tokens
	return shared.ProgressSlots{
		Track:     fadedStyle(tokens.Color.OnSurfaceVariant, 0.24),
		Indicator: markStyleFromColor(tokens.Color.Primary),
		Label:     markStyleFromColor(tokens.Color.OnSurface),
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
			Strokes: []theme.MaterialStroke{{
				Paint: theme.Fill{
					Type:    theme.FillSolid,
					Color:   color,
					Opacity: 1,
				},
				Width: width,
				Cap:   theme.CapRound,
				Join:  theme.JoinRound,
			}},
			Opacity: 1,
		},
	}
}
