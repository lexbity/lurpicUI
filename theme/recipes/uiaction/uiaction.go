package uiaction

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// ResolveActionBarRecipe resolves the action bar slots and provenance.
func ResolveActionBarRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ActionBarSlots]) (shared.ActionBarSlots, theme.RecipeReport) {
	slots := actionBarBase(ctx)
	report := newReport("uiaction", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveActionGroupRecipe resolves the action group slots and provenance.
func ResolveActionGroupRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ActionGroupSlots]) (shared.ActionGroupSlots, theme.RecipeReport) {
	slots := actionGroupBase(ctx)
	report := newReport("uiaction", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveMenuButtonRecipe resolves the menu button slots and provenance.
func ResolveMenuButtonRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.MenuButtonSlots]) (shared.MenuButtonSlots, theme.RecipeReport) {
	slots := menuButtonBase(ctx)
	report := newReport("uiaction", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveSplitButtonRecipe resolves the split button slots and provenance.
func ResolveSplitButtonRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.SplitButtonSlots]) (shared.SplitButtonSlots, theme.RecipeReport) {
	slots := splitButtonBase(ctx)
	report := newReport("uiaction", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func actionBarBase(ctx theme.StyleContext) shared.ActionBarSlots {
	tokens := ctx.Tokens
	return shared.ActionBarSlots{
		Root:         transparentStyle(),
		BarSurface:   roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
		ContextLabel: markStyleFromColor(tokens.Color.OnSurface),
		ActionItems:  markStyleFromColor(tokens.Color.OnSurface),
		OverflowMenu: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		FocusRing:    strokeStyle(tokens.Color.Primary, 2),
	}
}

func actionGroupBase(ctx theme.StyleContext) shared.ActionGroupSlots {
	tokens := ctx.Tokens
	return shared.ActionGroupSlots{
		Root:         transparentStyle(),
		GroupSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
		ActionItems:  markStyleFromColor(tokens.Color.OnSurface),
		Separators:   strokeStyle(tokens.Color.OnSurfaceVariant, 1),
		FocusRing:    strokeStyle(tokens.Color.Primary, 2),
	}
}

func menuButtonBase(ctx theme.StyleContext) shared.MenuButtonSlots {
	tokens := ctx.Tokens
	return shared.MenuButtonSlots{
		Root:                transparentStyle(),
		Trigger:             roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
		TriggerLabel:        markStyleFromColor(tokens.Color.OnSurface),
		TriggerIcon:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
		Chevron:             markStyleFromColor(tokens.Color.OnSurfaceVariant),
		FloatingMenuSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
		MenuItems:           markStyleFromColor(tokens.Color.OnSurface),
		FocusRing:           strokeStyle(tokens.Color.Primary, 2),
	}
}

func splitButtonBase(ctx theme.StyleContext) shared.SplitButtonSlots {
	tokens := ctx.Tokens
	return shared.SplitButtonSlots{
		Root:                transparentStyle(),
		PrimaryButton:       roundedSurfaceStyle(tokens.Color.Primary, tokens.Color.Primary, 0),
		PrimaryLabel:        markStyleFromColor(tokens.Color.OnPrimary),
		MenuTrigger:         roundedSurfaceStyle(tokens.Color.Primary, tokens.Color.Primary, 0),
		Chevron:             strokeStyle(tokens.Color.OnPrimary, 2),
		FloatingMenuSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
		MenuItems:           markStyleFromColor(tokens.Color.OnSurface),
		FocusRing:           strokeStyle(tokens.Color.Primary, 2),
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

func transparentStyle() theme.MarkStyle {
	return theme.MarkStyle{
		Base: theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillNone,
				Opacity: 0,
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

func roundedSurfaceStyle(fill, stroke gfx.Color, strokeWidth float32) theme.MarkStyle {
	material := theme.Material{
		Fills: []theme.Fill{{
			Type:    theme.FillSolid,
			Color:   fill,
			Opacity: 1,
		}},
		Opacity: 1,
	}
	if strokeWidth > 0 {
		material.Strokes = []theme.MaterialStroke{{
			Paint: theme.Fill{
				Type:    theme.FillSolid,
				Color:   stroke,
				Opacity: 1,
			},
			Width: strokeWidth,
		}}
	}
	return theme.MarkStyle{Base: material}
}
