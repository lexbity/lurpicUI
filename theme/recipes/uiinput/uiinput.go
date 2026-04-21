package uiinput

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

// ButtonVariant selects the button recipe shape.
type ButtonVariant uint8

const (
	// ButtonFilled uses a filled container.
	ButtonFilled ButtonVariant = iota
	// ButtonOutlined uses a surface container with emphasized content.
	ButtonOutlined
	// ButtonText uses a flat, low-emphasis container.
	ButtonText
	// ButtonTonal uses a softer semantic container.
	ButtonTonal
)

func (v ButtonVariant) String() string {
	switch v {
	case ButtonFilled:
		return "filled"
	case ButtonOutlined:
		return "outlined"
	case ButtonText:
		return "text"
	case ButtonTonal:
		return "tonal"
	default:
		return "unknown"
	}
}

// TextInputVariant selects the text input recipe shape.
type TextInputVariant uint8

const (
	// TextInputOutlined uses an outlined field.
	TextInputOutlined TextInputVariant = iota
	// TextInputFilled uses a filled field.
	TextInputFilled
	// TextInputUnderlined uses an underlined field.
	TextInputUnderlined
)

func (v TextInputVariant) String() string {
	switch v {
	case TextInputOutlined:
		return "outlined"
	case TextInputFilled:
		return "filled"
	case TextInputUnderlined:
		return "underlined"
	default:
		return "unknown"
	}
}

// SliderVariant selects the slider recipe shape.
type SliderVariant uint8

const (
	// SliderStandard uses a comfortable semantic layout.
	SliderStandard SliderVariant = iota
	// SliderCompact uses denser feedback and a smaller thumb.
	SliderCompact
)

func (v SliderVariant) String() string {
	switch v {
	case SliderStandard:
		return "standard"
	case SliderCompact:
		return "compact"
	default:
		return "unknown"
	}
}

// CheckboxVariant selects the checkbox recipe shape.
type CheckboxVariant uint8

const (
	// CheckboxStandard uses the default checkbox styling.
	CheckboxStandard CheckboxVariant = iota
)

func (v CheckboxVariant) String() string {
	switch v {
	case CheckboxStandard:
		return "standard"
	default:
		return "unknown"
	}
}

// SwitchVariant selects the switch recipe shape.
type SwitchVariant uint8

const (
	// SwitchStandard uses the default switch styling.
	SwitchStandard SwitchVariant = iota
)

func (v SwitchVariant) String() string {
	switch v {
	case SwitchStandard:
		return "standard"
	default:
		return "unknown"
	}
}

// RadioGroupVariant selects the radio-group recipe shape.
type RadioGroupVariant uint8

const (
	// RadioGroupStandard uses the default radio-group styling.
	RadioGroupStandard RadioGroupVariant = iota
)

func (v RadioGroupVariant) String() string {
	switch v {
	case RadioGroupStandard:
		return "standard"
	default:
		return "unknown"
	}
}

// SelectVariant selects the select recipe shape.
type SelectVariant uint8

const (
	// SelectStandard uses the default trigger and popup styling.
SelectStandard SelectVariant = iota
)

func (v SelectVariant) String() string {
	switch v {
	case SelectStandard:
		return "standard"
	default:
		return "unknown"
	}
}

// ResolveButtonRecipe resolves the button slots and provenance.
func ResolveButtonRecipe(ctx theme.StyleContext, variant ButtonVariant, overrides ...theme.SlotPatch[shared.ButtonSlots]) (shared.ButtonSlots, theme.RecipeReport) {
	slots := buttonBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveTextInputRecipe resolves the text input slots and provenance.
func ResolveTextInputRecipe(ctx theme.StyleContext, variant TextInputVariant, overrides ...theme.SlotPatch[shared.TextInputSlots]) (shared.TextInputSlots, theme.RecipeReport) {
	slots := textInputBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveSliderRecipe resolves the slider slots and provenance.
func ResolveSliderRecipe(ctx theme.StyleContext, variant SliderVariant, overrides ...theme.SlotPatch[shared.SliderSlots]) (shared.SliderSlots, theme.RecipeReport) {
	slots := sliderBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveCheckboxRecipe resolves the checkbox slots and provenance.
func ResolveCheckboxRecipe(ctx theme.StyleContext, variant CheckboxVariant, overrides ...theme.SlotPatch[shared.CheckboxSlots]) (shared.CheckboxSlots, theme.RecipeReport) {
	slots := checkboxBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveSwitchRecipe resolves the switch slots and provenance.
func ResolveSwitchRecipe(ctx theme.StyleContext, variant SwitchVariant, overrides ...theme.SlotPatch[shared.SwitchSlots]) (shared.SwitchSlots, theme.RecipeReport) {
	slots := switchBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveRadioGroupRecipe resolves the radio-group slots and provenance.
func ResolveRadioGroupRecipe(ctx theme.StyleContext, variant RadioGroupVariant, overrides ...theme.SlotPatch[shared.RadioGroupSlots]) (shared.RadioGroupSlots, theme.RecipeReport) {
	slots := radioGroupBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveSelectRecipe resolves the select slots and provenance.
func ResolveSelectRecipe(ctx theme.StyleContext, variant SelectVariant, overrides ...theme.SlotPatch[shared.SelectSlots]) (shared.SelectSlots, theme.RecipeReport) {
	slots := selectBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func buttonBase(ctx theme.StyleContext, variant ButtonVariant) shared.ButtonSlots {
	tokens := ctx.Tokens
	rootSource := sourceForContext(ctx)
	switch variant {
	case ButtonFilled:
		return shared.ButtonSlots{
			Container: markStyleFromColor(tokens.Color.Primary),
			Label:     markStyleFromColor(tokens.Color.OnPrimary),
			Icon:      markStyleFromColor(tokens.Color.OnPrimary),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	case ButtonOutlined:
		return shared.ButtonSlots{
			Container: markStyleFromColor(tokens.Color.Surface),
			Label:     markStyleFromColor(tokens.Color.Primary),
			Icon:      markStyleFromColor(tokens.Color.Primary),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	case ButtonText:
		return shared.ButtonSlots{
			Container: transparentStyle(),
			Label:     markStyleFromColor(tokens.Color.Primary),
			Icon:      markStyleFromColor(tokens.Color.Primary),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	case ButtonTonal:
		return shared.ButtonSlots{
			Container: markStyleFromColor(tokens.Color.SecondaryVariant),
			Label:     markStyleFromColor(tokens.Color.OnSecondary),
			Icon:      markStyleFromColor(tokens.Color.OnSecondary),
			FocusRing: strokeStyle(tokens.Color.Secondary, 2),
		}
	default:
		_ = rootSource
		return shared.ButtonSlots{}
	}
}

func textInputBase(ctx theme.StyleContext, variant TextInputVariant) shared.TextInputSlots {
	tokens := ctx.Tokens
	switch variant {
	case TextInputOutlined:
		return shared.TextInputSlots{
			Field:         markStyleFromColor(tokens.Color.Surface),
			Text:          markStyleFromColor(tokens.Color.OnSurface),
			Placeholder:   fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
			Caret:         markStyleFromColor(tokens.Color.Primary),
			Selection:     fadedStyle(tokens.Color.Primary, 0.2),
			Outline:       markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:     strokeStyle(tokens.Color.Primary, 2),
			AssistiveText: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	case TextInputFilled:
		return shared.TextInputSlots{
			Field:         markStyleFromColor(tokens.Color.SurfaceVariant),
			Text:          markStyleFromColor(tokens.Color.OnSurface),
			Placeholder:   fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
			Caret:         markStyleFromColor(tokens.Color.Primary),
			Selection:     fadedStyle(tokens.Color.Primary, 0.2),
			Outline:       transparentStyle(),
			FocusRing:     strokeStyle(tokens.Color.Primary, 2),
			AssistiveText: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	case TextInputUnderlined:
		return shared.TextInputSlots{
			Field:         transparentStyle(),
			Text:          markStyleFromColor(tokens.Color.OnSurface),
			Placeholder:   fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
			Caret:         markStyleFromColor(tokens.Color.Primary),
			Selection:     fadedStyle(tokens.Color.Primary, 0.2),
			Outline:       markStyleFromColor(tokens.Color.Primary),
			FocusRing:     strokeStyle(tokens.Color.Primary, 2),
			AssistiveText: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		}
	default:
		return shared.TextInputSlots{}
	}
}

func sliderBase(ctx theme.StyleContext, variant SliderVariant) shared.SliderSlots {
	tokens := ctx.Tokens
	switch variant {
	case SliderStandard:
		return shared.SliderSlots{
			Track:     fadedStyle(tokens.Color.OnSurfaceVariant, 0.24),
			Fill:      markStyleFromColor(tokens.Color.Primary),
			Thumb:     markStyleFromColor(tokens.Color.Primary),
			Tick:      fadedStyle(tokens.Color.OnSurfaceVariant, 0.4),
			ValueText: markStyleFromColor(tokens.Color.OnSurface),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	case SliderCompact:
		return shared.SliderSlots{
			Track:     fadedStyle(tokens.Color.Primary, 0.32),
			Fill:      markStyleFromColor(tokens.Color.PrimaryVariant),
			Thumb:     markStyleFromColor(tokens.Color.OnPrimary),
			Tick:      fadedStyle(tokens.Color.OnSurfaceVariant, 0.5),
			ValueText: markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing: strokeStyle(tokens.Color.Primary, 2),
		}
	default:
		return shared.SliderSlots{}
	}
}

func checkboxBase(ctx theme.StyleContext, variant CheckboxVariant) shared.CheckboxSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.CheckboxSlots{
		Box:       markStyleFromColor(tokens.Color.Surface),
		Check:     markStyleFromColor(tokens.Color.Primary),
		Label:     markStyleFromColor(tokens.Color.OnSurface),
		FocusRing: strokeStyle(tokens.Color.Primary, 2),
	}
}

func switchBase(ctx theme.StyleContext, variant SwitchVariant) shared.SwitchSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.SwitchSlots{
		Track:     fadedStyle(tokens.Color.OnSurfaceVariant, 0.32),
		Thumb:     markStyleFromColor(tokens.Color.Primary),
		Label:     markStyleFromColor(tokens.Color.OnSurface),
		FocusRing: strokeStyle(tokens.Color.Primary, 2),
	}
}

func radioGroupBase(ctx theme.StyleContext, variant RadioGroupVariant) shared.RadioGroupSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.RadioGroupSlots{
		Option:    markStyleFromColor(tokens.Color.Surface),
		Indicator: markStyleFromColor(tokens.Color.Primary),
		Label:     markStyleFromColor(tokens.Color.OnSurface),
		FocusRing: strokeStyle(tokens.Color.Primary, 2),
	}
}

func selectBase(ctx theme.StyleContext, variant SelectVariant) shared.SelectSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.SelectSlots{
		Field:     markStyleFromColor(tokens.Color.Surface),
		Value:     markStyleFromColor(tokens.Color.OnSurface),
		Popup:     markStyleFromColor(tokens.Color.Surface),
		Arrow:     markStyleFromColor(tokens.Color.OnSurfaceVariant),
		FocusRing: strokeStyle(tokens.Color.Primary, 2),
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

func sourceForContext(ctx theme.StyleContext) theme.SlotSource {
	if ctx.Depth > 0 {
		return theme.SlotSourceSubtreeOverride
	}
	return theme.SlotSourceRootDefault
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
	return theme.MarkStyle{
		Base: theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillNone,
				Opacity: 1,
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
