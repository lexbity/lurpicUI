package uistruct

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// ResolveCardRecipe resolves the card slots and provenance.
func ResolveCardRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.CardSlots]) (shared.CardSlots, theme.RecipeReport) {
	slots := cardBase(ctx)
	report := newReport("structure", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveListRecipe resolves the list slots and provenance.
func ResolveListRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ListSlots]) (shared.ListSlots, theme.RecipeReport) {
	slots := listBase(ctx)
	report := newReport("structure", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveTableRecipe resolves the table slots and provenance.
func ResolveTableRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.TableSlots]) (shared.TableSlots, theme.RecipeReport) {
	slots := tableBase(ctx)
	report := newReport("structure", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveScrollRegionRecipe resolves the scroll-region slots and provenance.
func ResolveScrollRegionRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ScrollRegionSlots]) (shared.ScrollRegionSlots, theme.RecipeReport) {
	slots := scrollRegionBase(ctx)
	report := newReport("structure", theme.VariantKey("default"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func cardBase(ctx theme.StyleContext) shared.CardSlots {
	tokens := ctx.Tokens
	return shared.CardSlots{
		Root:              transparentStyle(),
		CardSurface:       roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
		HeaderOptional:    transparentStyle(),
		MediaOptional:     fadedStyle(tokens.Color.SurfaceVariant, 0.92),
		Body:              transparentStyle(),
		ActionsOptional:   markStyleFromColor(tokens.Color.Primary),
		FocusRingOptional: strokeStyle(tokens.Color.Primary, 2),
	}
}

func listBase(ctx theme.StyleContext) shared.ListSlots {
	tokens := ctx.Tokens
	return shared.ListSlots{
		Root:                  transparentStyle(),
		ListContainer:         markStyleFromColor(tokens.Color.Surface),
		ListItems:             transparentStyle(),
		SectionHeaderOptional: transparentStyle(),
		EmptyStateOptional:    transparentStyle(),
	}
}

func tableBase(ctx theme.StyleContext) shared.TableSlots {
	tokens := ctx.Tokens
	return shared.TableSlots{
		Root:                    transparentStyle(),
		TableSurface:            markStyleFromColor(tokens.Color.Surface),
		HeaderRow:               markStyleFromColor(tokens.Color.SurfaceVariant),
		HeaderCell:              transparentStyle(),
		BodyRows:                transparentStyle(),
		BodyCell:                transparentStyle(),
		SelectionColumnOptional: markStyleFromColor(tokens.Color.Primary),
		SortIndicator:           markStyleFromColor(tokens.Color.Primary),
		FocusRing:               strokeStyle(tokens.Color.Primary, 2),
	}
}

func scrollRegionBase(ctx theme.StyleContext) shared.ScrollRegionSlots {
	tokens := ctx.Tokens
	return shared.ScrollRegionSlots{
		Root:                        transparentStyle(),
		Viewport:                    transparentStyle(),
		Content:                     transparentStyle(),
		ScrollbarVerticalOptional:   markStyleFromColor(tokens.Color.OnSurfaceVariant),
		ScrollbarHorizontalOptional: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		ScrollShadowsOptional:       fadedStyle(tokens.Color.OnSurfaceVariant, 0.18),
		FocusRingOptional:           strokeStyle(tokens.Color.Primary, 2),
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

func roundedSurfaceStyle(fill, stroke gfx.Color, width float32) theme.MarkStyle {
	return theme.MarkStyle{
		Base: theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillSolid,
				Color:   fill,
				Opacity: 1,
			}},
			Strokes: []theme.MaterialStroke{{
				Paint: theme.Fill{
					Type:    theme.FillSolid,
					Color:   stroke,
					Opacity: 0.35,
				},
				Width: width,
			}},
			Opacity: 1,
		},
	}
}
