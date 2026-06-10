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
	// ButtonSkeuomorphic uses 3D double strokes and tactile materials.
	ButtonSkeuomorphic
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
	case ButtonSkeuomorphic:
		return "skeuomorphic"
	default:
		return "unknown"
	}
}

// IconButtonVariant selects the icon button recipe shape.
type IconButtonVariant uint8

const (
	// IconButtonStandard uses a standard flat container.
	IconButtonStandard IconButtonVariant = iota
	// IconButtonSkeuomorphic uses 3D double strokes and tactile materials.
	IconButtonSkeuomorphic
)

func (v IconButtonVariant) String() string {
	switch v {
	case IconButtonStandard:
		return "standard"
	case IconButtonSkeuomorphic:
		return "skeuomorphic"
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
	// SliderSkeuomorphic uses an engraved track and tactile thumb.
	SliderSkeuomorphic
)

func (v SliderVariant) String() string {
	switch v {
	case SliderStandard:
		return "standard"
	case SliderCompact:
		return "compact"
	case SliderSkeuomorphic:
		return "skeuomorphic"
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
	// SwitchSkeuomorphic uses an engraved track and tactile toggle slider.
	SwitchSkeuomorphic
)

func (v SwitchVariant) String() string {
	switch v {
	case SwitchStandard:
		return "standard"
	case SwitchSkeuomorphic:
		return "skeuomorphic"
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

// ColorPickerVariant selects the color picker recipe shape.
type ColorPickerVariant uint8

const (
	// ColorPickerStandard uses the default hue-wheel picker styling.
	ColorPickerStandard ColorPickerVariant = iota
)

func (v ColorPickerVariant) String() string {
	switch v {
	case ColorPickerStandard:
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

// ListItemVariant selects the list-item recipe shape.
type ListItemVariant uint8

const (
	// ListItemStandard uses the default list-item styling.
	ListItemStandard ListItemVariant = iota
)

func (v ListItemVariant) String() string {
	switch v {
	case ListItemStandard:
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

// ResolveButtonGroupRecipe resolves the button-group slots and provenance.
func ResolveButtonGroupRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.ButtonGroupSlots]) (shared.ButtonGroupSlots, theme.RecipeReport) {
	slots := buttonGroupBase(ctx)
	report := newReport("selection", theme.VariantKey("standard"), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveIconButtonRecipe resolves the icon-button slots and provenance.
func ResolveIconButtonRecipe(ctx theme.StyleContext, variant IconButtonVariant, overrides ...theme.SlotPatch[shared.IconButtonSlots]) (shared.IconButtonSlots, theme.RecipeReport) {
	slots := iconButtonBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

// ResolveColorPickerRecipe resolves the color picker slots and provenance.
func ResolveColorPickerRecipe(ctx theme.StyleContext, variant ColorPickerVariant, overrides ...theme.SlotPatch[shared.ColorPickerSlots]) (shared.ColorPickerSlots, theme.RecipeReport) {
	slots := colorPickerBase(ctx, variant)
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

// ResolveNumberFieldRecipe resolves the number field slots and provenance.
func ResolveNumberFieldRecipe(ctx theme.StyleContext, overrides ...theme.SlotPatch[shared.NumberFieldSlots]) (shared.NumberFieldSlots, theme.RecipeReport) {
	slots := numberFieldBase(ctx)
	report := newReport("uiinput", theme.VariantKey("default"), slots)
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

// ResolveListItemRecipe resolves the list-item slots and provenance.
func ResolveListItemRecipe(ctx theme.StyleContext, variant ListItemVariant, overrides ...theme.SlotPatch[shared.ListItemSlots]) (shared.ListItemSlots, theme.RecipeReport) {
	slots := listItemBase(ctx, variant)
	report := newReport("uiinput", theme.VariantKey(variant.String()), slots)
	resolved := theme.ResolveSlot(slots, overrides...)
	annotateOverrides(&report, slots, resolved)
	return resolved, report
}

func buttonBase(ctx theme.StyleContext, variant ButtonVariant) shared.ButtonSlots {
	tokens := ctx.Tokens
	switch variant {
	case ButtonFilled:
		return shared.ButtonSlots{
			Root:                 transparentStyle(),
			Container:            markStyleFromColor(tokens.Color.Primary),
			Label:                markStyleFromColor(tokens.Color.OnPrimary),
			OptionalLeadingIcon:  markStyleFromColor(tokens.Color.OnPrimary),
			OptionalTrailingIcon: markStyleFromColor(tokens.Color.OnPrimary),
			FocusRing:            strokeStyle(tokens.Color.Primary, 2),
			StateLayer:           stateLayerStyle(tokens.Color.OnPrimary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	case ButtonOutlined:
		return shared.ButtonSlots{
			Root:                 transparentStyle(),
			Container:            markStyleFromColor(tokens.Color.Surface),
			Label:                markStyleFromColor(tokens.Color.Primary),
			OptionalLeadingIcon:  markStyleFromColor(tokens.Color.Primary),
			OptionalTrailingIcon: markStyleFromColor(tokens.Color.Primary),
			FocusRing:            strokeStyle(tokens.Color.Primary, 2),
			StateLayer:           stateLayerStyle(tokens.Color.Primary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	case ButtonText:
		return shared.ButtonSlots{
			Root:                 transparentStyle(),
			Container:            transparentStyle(),
			Label:                markStyleFromColor(tokens.Color.Primary),
			OptionalLeadingIcon:  markStyleFromColor(tokens.Color.Primary),
			OptionalTrailingIcon: markStyleFromColor(tokens.Color.Primary),
			FocusRing:            strokeStyle(tokens.Color.Primary, 2),
			StateLayer:           stateLayerStyle(tokens.Color.Primary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	case ButtonTonal:
		return shared.ButtonSlots{
			Root:                 transparentStyle(),
			Container:            markStyleFromColor(tokens.Color.SecondaryVariant),
			Label:                markStyleFromColor(tokens.Color.OnSecondary),
			OptionalLeadingIcon:  markStyleFromColor(tokens.Color.OnSecondary),
			OptionalTrailingIcon: markStyleFromColor(tokens.Color.OnSecondary),
			FocusRing:            strokeStyle(tokens.Color.Secondary, 2),
			StateLayer:           stateLayerStyle(tokens.Color.OnSecondary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	case ButtonSkeuomorphic:
		return shared.ButtonSlots{
			Root: transparentStyle(),
			Container: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureMicroNoise, Repeat: theme.RepeatTile},
							Opacity: 0.15,
						},
						{
							Type: theme.FillGradient,
							Gradient: theme.Gradient{
								Type:  theme.GradientLinear,
								Start: gfx.Point{X: 0, Y: 0},
								End:   gfx.Point{X: 0, Y: 1},
								Stops: []theme.GradientStop{
									{Position: 0.0, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}},
									{Position: 0.35, Color: gfx.Color{R: 237, G: 243, B: 248, A: 255}},
									{Position: 0.7, Color: gfx.Color{R: 203, G: 216, B: 227, A: 255}},
									{Position: 1.0, Color: gfx.Color{R: 189, G: 204, B: 217, A: 255}},
								},
							},
							Opacity: 1.0,
						},
					},
					Strokes: []theme.MaterialStroke{
						{ // Bottom-right diffuse shadow
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 156, G: 176, B: 196, A: 255}, Opacity: 0.85},
							Width:      0,
							BlurRadius: 12,
							Offset:     gfx.Point{X: 7, Y: 7},
						},
						{ // Top-left specular highlight
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.85},
							Width:      0,
							BlurRadius: 12,
							Offset:     gfx.Point{X: -7, Y: -7},
						},
						{ // Top-left inner bevel
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.85},
							Width:  0.75,
							Offset: gfx.Point{X: -0.75, Y: -0.75},
							Inner:  true,
						},
						{ // Bottom-right inner bevel
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 91, G: 108, B: 124, A: 255}, Opacity: 0.3},
							Width:  0.75,
							Offset: gfx.Point{X: 0.75, Y: 0.75},
							Inner:  true,
						},
					},
					Opacity: 1.0,
				},
			},
			Label:                markStyleFromColor(tokens.Color.OnSurface),
			OptionalLeadingIcon:  markStyleFromColor(tokens.Color.OnSurface),
			OptionalTrailingIcon: markStyleFromColor(tokens.Color.OnSurface),
			FocusRing:            strokeStyle(tokens.Color.Primary, 2),
			StateLayer:           stateLayerStyle(tokens.Color.OnSurface, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	default:
		return shared.ButtonSlots{}
	}
}

func buttonGroupBase(ctx theme.StyleContext) shared.ButtonGroupSlots {
	tokens := ctx.Tokens
	return shared.ButtonGroupSlots{
		Root:              transparentStyle(),
		GroupSurface:      outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
		OptionButtons:     outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
		SelectedIndicator: markStyleFromColor(tokens.Color.Primary),
		FocusRing:         strokeStyle(tokens.Color.Primary, 2),
	}
}

func iconButtonBase(ctx theme.StyleContext, variant IconButtonVariant) shared.IconButtonSlots {
	tokens := ctx.Tokens
	switch variant {
	case IconButtonSkeuomorphic:
		return shared.IconButtonSlots{
			Root: transparentStyle(),
			Container: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureMicroNoise, Repeat: theme.RepeatTile},
							Opacity: 0.15,
						},
						{
							Type: theme.FillGradient,
							Gradient: theme.Gradient{
								Type:  theme.GradientLinear,
								Start: gfx.Point{X: 0, Y: 0},
								End:   gfx.Point{X: 0, Y: 1},
								Stops: []theme.GradientStop{
									{Position: 0.0, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}},
									{Position: 0.35, Color: gfx.Color{R: 237, G: 243, B: 248, A: 255}},
									{Position: 0.7, Color: gfx.Color{R: 203, G: 216, B: 227, A: 255}},
									{Position: 1.0, Color: gfx.Color{R: 189, G: 204, B: 217, A: 255}},
								},
							},
							Opacity: 1.0,
						},
					},
					Strokes: []theme.MaterialStroke{
						{ // Bottom-right diffuse shadow
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 156, G: 176, B: 196, A: 255}, Opacity: 0.85},
							Width:      0,
							BlurRadius: 12,
							Offset:     gfx.Point{X: 7, Y: 7},
						},
						{ // Top-left specular highlight
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.85},
							Width:      0,
							BlurRadius: 12,
							Offset:     gfx.Point{X: -7, Y: -7},
						},
						{ // Top-left inner bevel
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.85},
							Width:  0.75,
							Offset: gfx.Point{X: -0.75, Y: -0.75},
							Inner:  true,
						},
						{ // Bottom-right inner bevel
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 91, G: 108, B: 124, A: 255}, Opacity: 0.3},
							Width:  0.75,
							Offset: gfx.Point{X: 0.75, Y: 0.75},
							Inner:  true,
						},
					},
					Opacity: 1.0,
				},
			},
			Icon:       markStyleFromColor(tokens.Color.Primary),
			FocusRing:  strokeStyle(tokens.Color.Primary, 2),
			StateLayer: stateLayerStyle(tokens.Color.Primary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	default:
		return shared.IconButtonSlots{
			Root:       transparentStyle(),
			Container:  markStyleFromColor(tokens.Color.Primary),
			Icon:       markStyleFromColor(tokens.Color.OnPrimary),
			FocusRing:  strokeStyle(tokens.Color.OnPrimary, 2),
			StateLayer: stateLayerStyle(tokens.Color.OnPrimary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	}
}

func colorPickerBase(ctx theme.StyleContext, variant ColorPickerVariant) shared.ColorPickerSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.ColorPickerSlots{
		Root:      transparentStyle(),
		Wheel:     strokeStyle(tokens.Color.OnSurfaceVariant, 1.5),
		Triangle:  strokeStyle(tokens.Color.OnSurfaceVariant, 1.5),
		Handle:    theme.MarkStyle{Base: theme.SolidMaterial(tokens.Color.Surface, tokens.Color.Primary, 1.5)},
		FocusRing: strokeStyle(tokens.Color.Primary, 2),
	}
}

func textInputBase(ctx theme.StyleContext, variant TextInputVariant) shared.TextInputSlots {
	tokens := ctx.Tokens
	switch variant {
	case TextInputOutlined:
		return shared.TextInputSlots{
			Root:           transparentStyle(),
			FieldContainer: outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
			Label:          markStyleFromColor(tokens.Color.OnSurfaceVariant),
			InputText:      markStyleFromColor(tokens.Color.OnSurface),
			Placeholder:    fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
			HelperText:     markStyleFromColor(tokens.Color.OnSurfaceVariant),
			ErrorText:      markStyleFromColor(tokens.Color.Error),
			Caret:          markStyleFromColor(tokens.Color.Primary),
			SelectionRange: fadedStyle(tokens.Color.Primary, tokens.Color.SelectedOverlay),
			FocusRing:      strokeStyle(tokens.Color.Primary, 2),
		}
	case TextInputFilled:
		return shared.TextInputSlots{
			Root:           transparentStyle(),
			FieldContainer: filledFieldContainer(tokens.Color.SurfaceVariant),
			Label:          markStyleFromColor(tokens.Color.OnSurfaceVariant),
			InputText:      markStyleFromColor(tokens.Color.OnSurface),
			Placeholder:    fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
			HelperText:     markStyleFromColor(tokens.Color.OnSurfaceVariant),
			ErrorText:      markStyleFromColor(tokens.Color.Error),
			Caret:          markStyleFromColor(tokens.Color.Primary),
			SelectionRange: fadedStyle(tokens.Color.Primary, tokens.Color.SelectedOverlay),
			FocusRing:      strokeStyle(tokens.Color.Primary, 2),
		}
	case TextInputUnderlined:
		return shared.TextInputSlots{
			Root:           transparentStyle(),
			FieldContainer: underlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
			Label:          markStyleFromColor(tokens.Color.OnSurfaceVariant),
			InputText:      markStyleFromColor(tokens.Color.OnSurface),
			Placeholder:    fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
			HelperText:     markStyleFromColor(tokens.Color.OnSurfaceVariant),
			ErrorText:      markStyleFromColor(tokens.Color.Error),
			Caret:          markStyleFromColor(tokens.Color.Primary),
			SelectionRange: fadedStyle(tokens.Color.Primary, tokens.Color.SelectedOverlay),
			FocusRing:      strokeStyle(tokens.Color.Primary, 2),
		}
	default:
		return shared.TextInputSlots{}
	}
}

func numberFieldBase(ctx theme.StyleContext) shared.NumberFieldSlots {
	tokens := ctx.Tokens
	return shared.NumberFieldSlots{
		Root:           transparentStyle(),
		FieldContainer: outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
		Label:          markStyleFromColor(tokens.Color.OnSurfaceVariant),
		InputText:      markStyleFromColor(tokens.Color.OnSurface),
		Placeholder:    fadedStyle(tokens.Color.OnSurfaceVariant, 0.58),
		StepperUp:      markStyleFromColor(tokens.Color.Primary),
		StepperDown:    markStyleFromColor(tokens.Color.Primary),
		HelperText:     markStyleFromColor(tokens.Color.OnSurfaceVariant),
		ErrorText:      markStyleFromColor(tokens.Color.Error),
		Caret:          markStyleFromColor(tokens.Color.Primary),
		SelectionRange: fadedStyle(tokens.Color.Primary, tokens.Color.SelectedOverlay),
		FocusRing:      strokeStyle(tokens.Color.Primary, 2),
	}
}

func sliderBase(ctx theme.StyleContext, variant SliderVariant) shared.SliderSlots {
	tokens := ctx.Tokens
	switch variant {
	case SliderStandard:
		return shared.SliderSlots{
			Root:        transparentStyle(),
			Track:       fadedStyle(tokens.Color.OnSurfaceVariant, 0.24),
			ActiveTrack: markStyleFromColor(tokens.Color.Primary),
			Thumb:       markStyleFromColor(tokens.Color.Primary),
			TickMarks:   fadedStyle(tokens.Color.OnSurfaceVariant, 0.4),
			ValueLabel:  markStyleFromColor(tokens.Color.OnSurface),
			FocusRing:   strokeStyle(tokens.Color.Primary, 2),
		}
	case SliderCompact:
		return shared.SliderSlots{
			Root:        transparentStyle(),
			Track:       fadedStyle(tokens.Color.Primary, 0.32),
			ActiveTrack: markStyleFromColor(tokens.Color.PrimaryVariant),
			Thumb:       markStyleFromColor(tokens.Color.OnPrimary),
			TickMarks:   fadedStyle(tokens.Color.OnSurfaceVariant, 0.5),
			ValueLabel:  markStyleFromColor(tokens.Color.OnSurfaceVariant),
			FocusRing:   strokeStyle(tokens.Color.Primary, 2),
		}
	case SliderSkeuomorphic:
		return shared.SliderSlots{
			Root: transparentStyle(),
			Track: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureMicroNoise, Repeat: theme.RepeatTile},
							Opacity: 0.15,
						},
						{
							Type:    theme.FillSolid,
							Color:   gfx.Color{R: 18, G: 20, B: 24, A: 255},
							Opacity: 1,
						},
					},
					Strokes: []theme.MaterialStroke{
						{ // Inset bevel shadow: Black (#000000, opacity 0.75, blur 4, offset {1, 1}, Inner: true)
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.75},
							Width:      1.5,
							BlurRadius: 4,
							Offset:     gfx.Point{X: 1, Y: 1},
							Inner:      true,
						},
						{ // Inset bevel reflection: Soft highlight (#ffffff, opacity 0.25, blur 2, offset {-1, -1}, Inner: true)
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.25},
							Width:      1.0,
							BlurRadius: 2,
							Offset:     gfx.Point{X: -1, Y: -1},
							Inner:      true,
						},
					},
					Opacity: 1,
				},
			},
			ActiveTrack: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.Primary,
						Opacity: 1,
					}},
					Strokes: []theme.MaterialStroke{
						{
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.4},
							Width:      1.0,
							BlurRadius: 2,
							Offset:     gfx.Point{X: 0.5, Y: 0.5},
							Inner:      true,
						},
					},
					Opacity: 1,
				},
			},
			Thumb: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureBrushedMetal, Repeat: theme.RepeatTile},
							Opacity: 0.9,
						},
						{
							Type: theme.FillGradient,
							Gradient: theme.Gradient{
								Type:  theme.GradientLinear,
								Start: gfx.Point{X: 0, Y: 0},
								End:   gfx.Point{X: 0, Y: 1},
								Stops: []theme.GradientStop{
									{Position: 0.0, Color: gfx.Color{R: 240, G: 240, B: 240, A: 255}},
									{Position: 0.5, Color: gfx.Color{R: 200, G: 200, B: 200, A: 255}},
									{Position: 1.0, Color: gfx.Color{R: 160, G: 160, B: 160, A: 255}},
								},
							},
							Opacity: 1,
						},
					},
					Strokes: []theme.MaterialStroke{
						{
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.4},
							Width:      0,
							BlurRadius: 5,
							Offset:     gfx.Point{X: 1.5, Y: 1.5},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.8},
							Width:  1.5,
							Offset: gfx.Point{X: -1, Y: -1},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.3},
							Width:  1.5,
							Offset: gfx.Point{X: 1, Y: 1},
						},
					},
					Opacity: 1,
				},
				Pressed: &theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureBrushedMetal, Repeat: theme.RepeatTile},
							Opacity: 0.9,
						},
						{
							Type: theme.FillGradient,
							Gradient: theme.Gradient{
								Type:  theme.GradientLinear,
								Start: gfx.Point{X: 0, Y: 0},
								End:   gfx.Point{X: 0, Y: 1},
								Stops: []theme.GradientStop{
									{Position: 0.0, Color: gfx.Color{R: 210, G: 210, B: 210, A: 255}},
									{Position: 0.5, Color: gfx.Color{R: 180, G: 180, B: 180, A: 255}},
									{Position: 1.0, Color: gfx.Color{R: 140, G: 140, B: 140, A: 255}},
								},
							},
							Opacity: 1,
						},
					},
					Strokes: []theme.MaterialStroke{
						{
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.45},
							Width:      0,
							BlurRadius: 3,
							Offset:     gfx.Point{X: 0.8, Y: 0.8},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.7},
							Width:  1.5,
							Offset: gfx.Point{X: -0.8, Y: -0.8},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.35},
							Width:  1.5,
							Offset: gfx.Point{X: 0.8, Y: 0.8},
						},
					},
					Opacity: 1,
				},
			},
			TickMarks:  fadedStyle(tokens.Color.OnSurfaceVariant, 0.4),
			ValueLabel: markStyleFromColor(tokens.Color.OnSurface),
			FocusRing:  strokeStyle(tokens.Color.Primary, 2),
		}
	default:
		return shared.SliderSlots{}
	}
}

func checkboxBase(ctx theme.StyleContext, variant CheckboxVariant) shared.CheckboxSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.CheckboxSlots{
		Root: transparentStyle(),
		ControlBox: theme.MarkStyle{
			Base: theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.Surface,
					Opacity: 1,
				}},
				Strokes: []theme.MaterialStroke{{
					Paint: theme.Fill{
						Type:    theme.FillSolid,
						Color:   tokens.Color.OnSurfaceVariant,
						Opacity: 1,
					},
					Width: 1,
					Cap:   theme.CapRound,
					Join:  theme.JoinRound,
				}},
				Opacity: 1,
			},
			Selected: &theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.Primary,
					Opacity: 1,
				}},
				Strokes: []theme.MaterialStroke{{
					Paint: theme.Fill{
						Type:    theme.FillSolid,
						Color:   tokens.Color.Primary,
						Opacity: 1,
					},
					Width: 1,
					Cap:   theme.CapRound,
					Join:  theme.JoinRound,
				}},
				Opacity: 1,
			},
		},
		Checkmark:  strokeStyle(tokens.Color.OnPrimary, 2.25),
		Label:      markStyleFromColor(tokens.Color.OnSurface),
		HelperText: markStyleFromColor(tokens.Color.OnSurfaceVariant),
		FocusRing:  strokeStyle(tokens.Color.Primary, 2),
		StateLayer: stateLayerStyle(tokens.Color.Primary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
	}
}

func switchBase(ctx theme.StyleContext, variant SwitchVariant) shared.SwitchSlots {
	tokens := ctx.Tokens
	switch variant {
	case SwitchSkeuomorphic:
		return shared.SwitchSlots{
			Root: transparentStyle(),
			Track: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureMicroNoise, Repeat: theme.RepeatTile},
							Opacity: 0.15,
						},
						{
							Type:    theme.FillSolid,
							Color:   gfx.Color{R: 18, G: 20, B: 24, A: 255},
							Opacity: 1,
						},
					},
					Strokes: []theme.MaterialStroke{
						{ // Inset bevel shadow: Black (#000000, opacity 0.75, blur 4, offset {1, 1}, Inner: true)
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.75},
							Width:      1.5,
							BlurRadius: 4,
							Offset:     gfx.Point{X: 1, Y: 1},
							Inner:      true,
						},
						{ // Inset bevel reflection: Soft highlight (#ffffff, opacity 0.25, blur 2, offset {-1, -1}, Inner: true)
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.25},
							Width:      1.0,
							BlurRadius: 2,
							Offset:     gfx.Point{X: -1, Y: -1},
							Inner:      true,
						},
					},
					Opacity: 1,
				},
				Selected: &theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.Primary,
						Opacity: 0.85,
					}},
					Strokes: []theme.MaterialStroke{
						{
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.75},
							Width:      1.5,
							BlurRadius: 4,
							Offset:     gfx.Point{X: 1, Y: 1},
							Inner:      true,
						},
					},
					Opacity: 1,
				},
			},
			Thumb: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureBrushedMetal, Repeat: theme.RepeatTile},
							Opacity: 0.95,
						},
						{
							Type: theme.FillGradient,
							Gradient: theme.Gradient{
								Type:  theme.GradientLinear,
								Start: gfx.Point{X: 0, Y: 0},
								End:   gfx.Point{X: 0, Y: 1},
								Stops: []theme.GradientStop{
									{Position: 0.0, Color: gfx.Color{R: 245, G: 245, B: 245, A: 255}},
									{Position: 1.0, Color: gfx.Color{R: 170, G: 170, B: 170, A: 255}},
								},
							},
							Opacity: 1.0,
						},
					},
					Strokes: []theme.MaterialStroke{
						{
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.45},
							Width:      0,
							BlurRadius: 5,
							Offset:     gfx.Point{X: 1.5, Y: 1.5},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.85},
							Width:  1.5,
							Offset: gfx.Point{X: -1, Y: -1},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.35},
							Width:  1.5,
							Offset: gfx.Point{X: 1, Y: 1},
						},
					},
					Opacity: 1,
				},
				Selected: &theme.Material{
					Fills: []theme.Fill{
						{
							Type:    theme.FillTexture,
							Texture: theme.TextureFill{Ref: theme.TextureBrushedMetal, Repeat: theme.RepeatTile},
							Opacity: 0.95,
						},
						{
							Type: theme.FillGradient,
							Gradient: theme.Gradient{
								Type:  theme.GradientLinear,
								Start: gfx.Point{X: 0, Y: 0},
								End:   gfx.Point{X: 0, Y: 1},
								Stops: []theme.GradientStop{
									{Position: 0.0, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}},
									{Position: 1.0, Color: gfx.Color{R: 190, G: 190, B: 190, A: 255}},
								},
							},
							Opacity: 1.0,
						},
					},
					Strokes: []theme.MaterialStroke{
						{
							Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.45},
							Width:      0,
							BlurRadius: 5,
							Offset:     gfx.Point{X: 1.5, Y: 1.5},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: 0.85},
							Width:  1.5,
							Offset: gfx.Point{X: -1, Y: -1},
						},
						{
							Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.Color{R: 0, G: 0, B: 0, A: 255}, Opacity: 0.35},
							Width:  1.5,
							Offset: gfx.Point{X: 1, Y: 1},
						},
					},
					Opacity: 1,
				},
			},
			Label:      markStyleFromColor(tokens.Color.OnSurface),
			FocusRing:  strokeStyle(tokens.Color.Primary, 2),
			StateLayer: stateLayerStyle(tokens.Color.Primary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	default:
		return shared.SwitchSlots{
			Root: transparentStyle(),
			Track: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.OnSurfaceVariant,
						Opacity: 1,
					}},
					Opacity: 1,
				},
				Selected: &theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.Primary,
						Opacity: 1,
					}},
					Opacity: 1,
				},
				Disabled: &theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.OnSurfaceVariant,
						Opacity: tokens.Color.DisabledOpacity,
					}},
					Opacity: tokens.Color.DisabledOpacity,
				},
			},
			Thumb: theme.MarkStyle{
				Base: theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.Surface,
						Opacity: 1,
					}},
					Opacity: 1,
				},
				Selected: &theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.OnPrimary,
						Opacity: 1,
					}},
					Opacity: 1,
				},
				Disabled: &theme.Material{
					Fills: []theme.Fill{{
						Type:    theme.FillSolid,
						Color:   tokens.Color.SurfaceVariant,
						Opacity: tokens.Color.DisabledOpacity,
					}},
					Opacity: tokens.Color.DisabledOpacity,
				},
			},
			Label:      markStyleFromColor(tokens.Color.OnSurface),
			FocusRing:  strokeStyle(tokens.Color.Primary, 2),
			StateLayer: stateLayerStyle(tokens.Color.Primary, tokens.Color.HoverLighten, tokens.Color.PressedDarken, tokens.Color.SelectedOverlay, tokens.Color.DisabledOpacity),
		}
	}
}

func radioGroupBase(ctx theme.StyleContext, variant RadioGroupVariant) shared.RadioGroupSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.RadioGroupSlots{
		Root:       transparentStyle(),
		GroupLabel: markStyleFromColor(tokens.Color.OnSurface),
		RadioItems: transparentStyle(),
		RadioControl: theme.MarkStyle{
			Base: theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.Surface,
					Opacity: 1,
				}},
				Strokes: []theme.MaterialStroke{{
					Paint: theme.Fill{
						Type:    theme.FillSolid,
						Color:   tokens.Color.OnSurfaceVariant,
						Opacity: 1,
					},
					Width: 1,
					Cap:   theme.CapRound,
					Join:  theme.JoinRound,
				}},
				Opacity: 1,
			},
			Selected: &theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.Primary,
					Opacity: 1,
				}},
				Opacity: 1,
			},
		},
		ItemLabel: markStyleFromColor(tokens.Color.OnSurface),
		FocusRing: strokeStyle(tokens.Color.Primary, 2),
	}
}

func selectBase(ctx theme.StyleContext, variant SelectVariant) shared.SelectSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.SelectSlots{
		Root:               transparentStyle(),
		Trigger:            outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
		SelectedValueLabel: markStyleFromColor(tokens.Color.OnSurface),
		Chevron:            markStyleFromColor(tokens.Color.OnSurfaceVariant),
		FloatingListbox:    outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
		OptionItems:        markStyleFromColor(tokens.Color.OnSurface),
		FocusRing:          strokeStyle(tokens.Color.Primary, 2),
	}
}

func listItemBase(ctx theme.StyleContext, variant ListItemVariant) shared.ListItemSlots {
	tokens := ctx.Tokens
	_ = variant
	return shared.ListItemSlots{
		Root:               transparentStyle(),
		ItemContainer:      outlinedFieldContainer(tokens.Color.Surface, tokens.Color.OnSurfaceVariant),
		LeadingIcon:        markStyleFromColor(tokens.Color.OnSurfaceVariant),
		Label:              markStyleFromColor(tokens.Color.OnSurface),
		SupportingText:     markStyleFromColor(tokens.Color.OnSurfaceVariant),
		SelectionIndicator: markStyleFromColor(tokens.Color.Primary),
		FocusRing:          strokeStyle(tokens.Color.Primary, 2),
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

func stateLayerStyle(color gfx.Color, hoverOpacity, pressedOpacity, focusOpacity, disabledOpacity float32) theme.MarkStyle {
	mk := func(opacity float32) *theme.Material {
		if opacity <= 0 {
			return nil
		}
		m := theme.FromToken(color)
		m.Opacity = opacity
		return &m
	}
	return theme.MarkStyle{
		Base:     theme.Material{Opacity: 0},
		Hover:    mk(hoverOpacity),
		Pressed:  mk(pressedOpacity),
		Focused:  mk(focusOpacity),
		Disabled: mk(disabledOpacity),
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

func outlinedFieldContainer(fill, outline gfx.Color) theme.MarkStyle {
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
					Color:   outline,
					Opacity: 1,
				},
				Width: 1,
				Cap:   theme.CapRound,
				Join:  theme.JoinRound,
			}},
			Opacity: 1,
		},
	}
}

func filledFieldContainer(fill gfx.Color) theme.MarkStyle {
	return theme.MarkStyle{
		Base: theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillSolid,
				Color:   fill,
				Opacity: 1,
			}},
			Opacity: 1,
		},
	}
}

func underlinedFieldContainer(fill, underline gfx.Color) theme.MarkStyle {
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
					Color:   underline,
					Opacity: 1,
				},
				Width: 1,
				Cap:   theme.CapSquare,
				Join:  theme.JoinMiter,
			}},
			Opacity: 1,
		},
	}
}

func darkenColor(c gfx.Color, factor float32) gfx.Color {
	r, g, b, a := c.ToRGBA8()
	if a == 0 {
		return c
	}
	scale := 1 - factor
	return gfx.ColorFromRGBA8(
		clampByte(float32(r)*scale),
		clampByte(float32(g)*scale),
		clampByte(float32(b)*scale),
		a,
	)
}

func lightenColor(c gfx.Color, factor float32) gfx.Color {
	r, g, b, a := c.ToRGBA8()
	if a == 0 {
		return c
	}
	return gfx.ColorFromRGBA8(
		clampByte(float32(r)+(255-float32(r))*factor),
		clampByte(float32(g)+(255-float32(g))*factor),
		clampByte(float32(b)+(255-float32(b))*factor),
		a,
	)
}

func clampByte(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
