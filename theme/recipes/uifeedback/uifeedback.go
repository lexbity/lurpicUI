package uifeedback

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// AlertVariant selects the alert recipe shape.
type AlertVariant uint8

const (
	AlertDefault AlertVariant = iota
	AlertHover
	AlertActive
	AlertDisabled
)

func (v AlertVariant) String() string {
	switch v {
	case AlertDefault:
		return "default"
	case AlertHover:
		return "hover"
	case AlertActive:
		return "active"
	case AlertDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ResolveAlertRecipe resolves alert styling.
func ResolveAlertRecipe(ctx theme.StyleContext, variant AlertVariant, overrides ...theme.SlotPatch[shared.FeedbackAlertSlots]) (shared.FeedbackAlertSlots, theme.RecipeReport) {
	slots := alertBase(ctx, variant)
	report := newReport("uifeedback", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// TooltipVariant selects the tooltip recipe shape.
type TooltipVariant uint8

const (
	TooltipDefault TooltipVariant = iota
	TooltipHover
	TooltipActive
	TooltipDisabled
	TooltipOpen
)

func (v TooltipVariant) String() string {
	switch v {
	case TooltipDefault:
		return "default"
	case TooltipHover:
		return "hover"
	case TooltipActive:
		return "active"
	case TooltipDisabled:
		return "disabled"
	case TooltipOpen:
		return "open"
	default:
		return "unknown"
	}
}

// ResolveTooltipRecipe resolves tooltip styling.
func ResolveTooltipRecipe(ctx theme.StyleContext, variant TooltipVariant, overrides ...theme.SlotPatch[shared.FeedbackTooltipSlots]) (shared.FeedbackTooltipSlots, theme.RecipeReport) {
	slots := tooltipBase(ctx, variant)
	report := newReport("uifeedback", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// NotificationVariant selects the notification recipe shape.
type NotificationVariant uint8

const (
	NotificationDefault NotificationVariant = iota
	NotificationHover
	NotificationActive
	NotificationDisabled
	NotificationOpen
)

func (v NotificationVariant) String() string {
	switch v {
	case NotificationDefault:
		return "default"
	case NotificationHover:
		return "hover"
	case NotificationActive:
		return "active"
	case NotificationDisabled:
		return "disabled"
	case NotificationOpen:
		return "open"
	default:
		return "unknown"
	}
}

// ResolveNotificationRecipe resolves notification styling.
func ResolveNotificationRecipe(ctx theme.StyleContext, variant NotificationVariant, overrides ...theme.SlotPatch[shared.FeedbackNotificationSlots]) (shared.FeedbackNotificationSlots, theme.RecipeReport) {
	slots := notificationBase(ctx, variant)
	report := newReport("uifeedback", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// DialogVariant selects the dialog recipe shape.
type DialogVariant uint8

const (
	DialogDefault DialogVariant = iota
	DialogHover
	DialogActive
	DialogFocused
	DialogDisabled
	DialogOpen
)

func (v DialogVariant) String() string {
	switch v {
	case DialogDefault:
		return "default"
	case DialogHover:
		return "hover"
	case DialogActive:
		return "active"
	case DialogFocused:
		return "focused"
	case DialogDisabled:
		return "disabled"
	case DialogOpen:
		return "open"
	default:
		return "unknown"
	}
}

// ResolveDialogRecipe resolves dialog styling.
func ResolveDialogRecipe(ctx theme.StyleContext, variant DialogVariant, overrides ...theme.SlotPatch[shared.FeedbackDialogSlots]) (shared.FeedbackDialogSlots, theme.RecipeReport) {
	slots := dialogBase(ctx, variant)
	report := newReport("uifeedback", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func alertBase(ctx theme.StyleContext, variant AlertVariant) shared.FeedbackAlertSlots {
	tokens := ctx.Tokens
	switch variant {
	case AlertHover:
		return shared.FeedbackAlertSlots{
			Root:         transparentStyle(),
			AlertSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.Primary, 1),
			Icon:         markStyleFromColor(tokens.Color.Primary),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Message:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:       markStyleFromColor(tokens.Color.Primary),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	case AlertActive:
		return shared.FeedbackAlertSlots{
			Root:         transparentStyle(),
			AlertSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.Primary, 2),
			Icon:         markStyleFromColor(tokens.Color.Primary),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Message:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:       markStyleFromColor(tokens.Color.Primary),
			CloseButton:  markStyleFromColor(tokens.Color.Primary),
		}
	case AlertDisabled:
		return shared.FeedbackAlertSlots{
			Root:         transparentStyle(),
			AlertSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Icon:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Title:        markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Message:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:       markStyleFromColor(tokens.Color.OnSurfaceVariant),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	default:
		return shared.FeedbackAlertSlots{
			Root:         transparentStyle(),
			AlertSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Icon:         markStyleFromColor(tokens.Color.Primary),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Message:      markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:       markStyleFromColor(tokens.Color.Primary),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	}
}

func tooltipBase(ctx theme.StyleContext, variant TooltipVariant) shared.FeedbackTooltipSlots {
	tokens := ctx.Tokens
	switch variant {
	case TooltipHover:
		return shared.FeedbackTooltipSlots{
			Root:           transparentStyle(),
			TooltipSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.OnSurfaceVariant, 1),
			Content:        markStyleFromColor(tokens.Color.OnSurface),
			AnchorArrow:    markStyleFromColor(tokens.Color.SurfaceVariant),
		}
	case TooltipActive, TooltipOpen:
		return shared.FeedbackTooltipSlots{
			Root:           transparentStyle(),
			TooltipSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.Primary, 2),
			Content:        markStyleFromColor(tokens.Color.OnSurface),
			AnchorArrow:    markStyleFromColor(tokens.Color.SurfaceVariant),
		}
	case TooltipDisabled:
		return shared.FeedbackTooltipSlots{
			Root:           transparentStyle(),
			TooltipSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Content:        markStyleFromColor(tokens.Color.OnSurfaceVariant),
			AnchorArrow:    markStyleFromColor(tokens.Color.Surface),
		}
	default:
		return shared.FeedbackTooltipSlots{
			Root:           transparentStyle(),
			TooltipSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.OnSurfaceVariant, 1),
			Content:        markStyleFromColor(tokens.Color.OnSurface),
			AnchorArrow:    markStyleFromColor(tokens.Color.SurfaceVariant),
		}
	}
}

func notificationBase(ctx theme.StyleContext, variant NotificationVariant) shared.FeedbackNotificationSlots {
	tokens := ctx.Tokens
	switch variant {
	case NotificationHover:
		return shared.FeedbackNotificationSlots{
			Root:          transparentStyle(),
			StatusSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.Primary, 1),
			Icon:          markStyleFromColor(tokens.Color.Primary),
			Title:         markStyleFromColor(tokens.Color.OnSurface),
			Message:       markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:        markStyleFromColor(tokens.Color.Primary),
			CloseButton:   markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	case NotificationActive:
		return shared.FeedbackNotificationSlots{
			Root:          transparentStyle(),
			StatusSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.Primary, 2),
			Icon:          markStyleFromColor(tokens.Color.Primary),
			Title:         markStyleFromColor(tokens.Color.OnSurface),
			Message:       markStyleFromColor(tokens.Color.OnSurface),
			Action:        markStyleFromColor(tokens.Color.Primary),
			CloseButton:   markStyleFromColor(tokens.Color.OnSurface),
		}
	case NotificationDisabled:
		return shared.FeedbackNotificationSlots{
			Root:          transparentStyle(),
			StatusSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Icon:          markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Title:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Message:       markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:        markStyleFromColor(tokens.Color.OnSurfaceVariant),
			CloseButton:   markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	case NotificationOpen:
		return shared.FeedbackNotificationSlots{
			Root:          transparentStyle(),
			StatusSurface: roundedSurfaceStyle(tokens.Color.SurfaceVariant, tokens.Color.OnSurfaceVariant, 1),
			Icon:          markStyleFromColor(tokens.Color.Primary),
			Title:         markStyleFromColor(tokens.Color.OnSurface),
			Message:       markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:        markStyleFromColor(tokens.Color.Primary),
			CloseButton:   markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	default:
		return shared.FeedbackNotificationSlots{
			Root:          transparentStyle(),
			StatusSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Icon:          markStyleFromColor(tokens.Color.Primary),
			Title:         markStyleFromColor(tokens.Color.OnSurface),
			Message:       markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Action:        markStyleFromColor(tokens.Color.Primary),
			CloseButton:   markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	}
}

func dialogBase(ctx theme.StyleContext, variant DialogVariant) shared.FeedbackDialogSlots {
	tokens := ctx.Tokens
	switch variant {
	case DialogHover:
		return shared.FeedbackDialogSlots{
			Root:         transparentStyle(),
			Backdrop:     fadedStyle(tokens.Color.Background, 0.48),
			ModalSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.Primary, 1),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Body:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Actions:      markStyleFromColor(tokens.Color.SurfaceVariant),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:    strokeStyle(tokens.Color.Primary, 1),
		}
	case DialogActive:
		return shared.FeedbackDialogSlots{
			Root:         transparentStyle(),
			Backdrop:     fadedStyle(tokens.Color.Background, 0.54),
			ModalSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.Primary, 2),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Body:         markStyleFromColor(tokens.Color.OnSurface),
			Actions:      markStyleFromColor(tokens.Color.SurfaceVariant),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurface),
			FocusRing:    strokeStyle(tokens.Color.Primary, 2),
		}
	case DialogFocused:
		return shared.FeedbackDialogSlots{
			Root:         transparentStyle(),
			Backdrop:     fadedStyle(tokens.Color.Background, 0.5),
			ModalSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.Primary, 2),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Body:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Actions:      markStyleFromColor(tokens.Color.SurfaceVariant),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:    strokeStyle(tokens.Color.Primary, 2),
		}
	case DialogDisabled:
		return shared.FeedbackDialogSlots{
			Root:         transparentStyle(),
			Backdrop:     fadedStyle(tokens.Color.Background, 0.36),
			ModalSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Title:        markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Body:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Actions:      markStyleFromColor(tokens.Color.Surface),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:    strokeStyle(tokens.Color.OnSurfaceVariant, 1),
		}
	case DialogOpen:
		return shared.FeedbackDialogSlots{
			Root:         transparentStyle(),
			Backdrop:     fadedStyle(tokens.Color.Background, 0.5),
			ModalSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Body:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Actions:      markStyleFromColor(tokens.Color.SurfaceVariant),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:    strokeStyle(tokens.Color.Primary, 1),
		}
	default:
		return shared.FeedbackDialogSlots{
			Root:         transparentStyle(),
			Backdrop:     fadedStyle(tokens.Color.Background, 0.44),
			ModalSurface: roundedSurfaceStyle(tokens.Color.Surface, tokens.Color.OnSurfaceVariant, 1),
			Title:        markStyleFromColor(tokens.Color.OnSurface),
			Body:         markStyleFromColor(tokens.Color.OnSurfaceVariant),
			Actions:      markStyleFromColor(tokens.Color.SurfaceVariant),
			CloseButton:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:    strokeStyle(tokens.Color.Primary, 1),
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

func transparentStyle() theme.MarkStyle {
	return theme.MarkStyle{Base: theme.Material{Opacity: 0}}
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
					Opacity: 0.28,
				},
				Width: width,
			}},
			Opacity: 1,
		},
	}
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
