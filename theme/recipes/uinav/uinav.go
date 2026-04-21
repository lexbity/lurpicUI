package uinav

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// TabsVariant selects the tab strip recipe shape.
type TabsVariant uint8

const (
	TabsStandard TabsVariant = iota
	TabsCompact
)

func (v TabsVariant) String() string {
	switch v {
	case TabsStandard:
		return "standard"
	case TabsCompact:
		return "compact"
	default:
		return "unknown"
	}
}

// MenuVariant selects the menu recipe shape.
type MenuVariant uint8

const (
	// MenuStandard uses default spacing.
	MenuStandard MenuVariant = iota
	// MenuDense uses compact spacing.
	MenuDense
)

func (v MenuVariant) String() string {
	switch v {
	case MenuStandard:
		return "standard"
	case MenuDense:
		return "dense"
	default:
		return "unknown"
	}
}

// ResolveTabsRecipe resolves the tabs slots and provenance.
func ResolveTabsRecipe(ctx theme.StyleContext, variant TabsVariant, overrides ...theme.SlotPatch[shared.TabsSlots]) (shared.TabsSlots, theme.RecipeReport) {
	slots := tabsBase(ctx, variant)
	report := newReport("uinav", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveBreadcrumbRecipe resolves breadcrumb styling.
func ResolveBreadcrumbRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.BreadcrumbSlots]) (shared.BreadcrumbSlots, theme.RecipeReport) {
	slots := breadcrumbBase(ctx)
	report := newReport("uinav", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolvePaginationRecipe resolves pagination styling.
func ResolvePaginationRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.PaginationSlots]) (shared.PaginationSlots, theme.RecipeReport) {
	slots := paginationBase(ctx)
	report := newReport("uinav", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveScrollbarRecipe resolves scrollbar styling.
func ResolveScrollbarRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ScrollbarSlots]) (shared.ScrollbarSlots, theme.RecipeReport) {
	slots := scrollbarBase(ctx)
	report := newReport("uinav", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveDrawerRecipe resolves drawer styling.
func ResolveDrawerRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.DrawerSlots]) (shared.DrawerSlots, theme.RecipeReport) {
	slots := drawerBase(ctx)
	report := newReport("uinav", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveSpeedDialRecipe resolves speed-dial styling.
func ResolveSpeedDialRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.SpeedDialSlots]) (shared.SpeedDialSlots, theme.RecipeReport) {
	slots := speedDialBase(ctx)
	report := newReport("uinav", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveMenuRecipe resolves menu styling.
func ResolveMenuRecipe(ctx theme.StyleContext, variant MenuVariant, overrides ...theme.SlotPatch[shared.MenuSlots]) (shared.MenuSlots, theme.RecipeReport) {
	slots := menuBase(ctx, variant)
	report := newReport("uinav", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func tabsBase(ctx theme.StyleContext, variant TabsVariant) shared.TabsSlots {
	tokens := ctx.Tokens
	switch variant {
	case TabsCompact:
		return shared.TabsSlots{
			Tab:       markStyleFromColor(tokens.Color.SurfaceVariant),
			Current:   markStyleFromColor(tokens.Color.Primary),
			Indicator: strokeStyle(tokens.Color.Primary, 2),
			Panel:     markStyleFromColor(tokens.Color.Surface),
		}
	default:
		return shared.TabsSlots{
			Tab:       markStyleFromColor(tokens.Color.Surface),
			Current:   markStyleFromColor(tokens.Color.Primary),
			Indicator: strokeStyle(tokens.Color.Primary, 3),
			Panel:     markStyleFromColor(tokens.Color.Surface),
		}
	}
}

func breadcrumbBase(ctx theme.StyleContext) shared.BreadcrumbSlots {
	tokens := ctx.Tokens
	return shared.BreadcrumbSlots{
		Item:      markStyleFromColor(tokens.Color.OnSurface),
		Current:   markStyleFromColor(tokens.Color.Primary),
		Separator: fadedStyle(tokens.Color.OnSurfaceVariant, 0.6),
	}
}

func paginationBase(ctx theme.StyleContext) shared.PaginationSlots {
	tokens := ctx.Tokens
	return shared.PaginationSlots{
		Page:      markStyleFromColor(tokens.Color.Surface),
		Current:   markStyleFromColor(tokens.Color.Primary),
		Nav:       markStyleFromColor(tokens.Color.OnSurface),
		Separator: fadedStyle(tokens.Color.OnSurfaceVariant, 0.5),
	}
}

func scrollbarBase(ctx theme.StyleContext) shared.ScrollbarSlots {
	tokens := ctx.Tokens
	return shared.ScrollbarSlots{
		Track:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.2),
		Thumb:  markStyleFromColor(tokens.Color.Primary),
		Corner: fadedStyle(tokens.Color.OnSurfaceVariant, 0.3),
	}
}

func menuBase(ctx theme.StyleContext, variant MenuVariant) shared.MenuSlots {
	tokens := ctx.Tokens
	switch variant {
	case MenuDense:
		return shared.MenuSlots{
			Surface:   markStyleFromColor(tokens.Color.SurfaceVariant),
			Item:      markStyleFromColor(tokens.Color.OnSurface),
			Icon:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Shortcut:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.7),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	default:
		return shared.MenuSlots{
			Surface:   markStyleFromColor(tokens.Color.Surface),
			Item:      markStyleFromColor(tokens.Color.OnSurface),
			Icon:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Shortcut:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.55),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	}
}

func drawerBase(ctx theme.StyleContext) shared.DrawerSlots {
	tokens := ctx.Tokens
	return shared.DrawerSlots{
		Scrim:    fadedStyle(tokens.Color.Background, 0.4),
		Surface:  markStyleFromColor(tokens.Color.Surface),
		Title:    markStyleFromColor(tokens.Color.OnSurface),
		Body:     markStyleFromColor(tokens.Color.OnSurface),
		Backdrop: fadedStyle(tokens.Color.Background, 0.1),
	}
}

func speedDialBase(ctx theme.StyleContext) shared.SpeedDialSlots {
	tokens := ctx.Tokens
	return shared.SpeedDialSlots{
		Fab:      markStyleFromColor(tokens.Color.Primary),
		Action:   markStyleFromColor(tokens.Color.Surface),
		Label:    markStyleFromColor(tokens.Color.OnSurface),
		Backdrop: fadedStyle(tokens.Color.Background, 0.22),
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
