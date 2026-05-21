package theme

import (
	"fmt"
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

// InteractionState describes the current interaction state for a mark.
type InteractionState uint8

const (
	StateDefault InteractionState = iota
	StateHover
	StatePressed
	StateDisabled
	StateSelected
	StateFocused
)

// MarkStyle defines visual appearance across interaction states.
type MarkStyle struct {
	Base     Material
	Hover    *Material
	Pressed  *Material
	Disabled *Material
	Selected *Material
	Focused  *Material
}

// Resolve returns the material for a given interaction state.
func (s MarkStyle) Resolve(state InteractionState, tokens Tokens) Material {
	switch state {
	case StateHover:
		if s.Hover != nil {
			return *s.Hover
		}
		return deriveHover(s.Base, tokens)
	case StatePressed:
		if s.Pressed != nil {
			return *s.Pressed
		}
		return derivePressed(s.Base, tokens)
	case StateDisabled:
		if s.Disabled != nil {
			return *s.Disabled
		}
		return deriveDisabled(s.Base, tokens)
	case StateSelected:
		if s.Selected != nil {
			return *s.Selected
		}
		return deriveSelected(s.Base, tokens)
	case StateFocused:
		if s.Focused != nil {
			return *s.Focused
		}
		return s.Base
	case StateDefault:
		fallthrough
	default:
		return s.Base
	}
}

// StyleContext is the resolved style environment for a facet subtree.
type StyleContext struct {
	Tokens    Tokens
	Materials *MaterialRegistry
	Depth     int
}

// StyleContextOverride selectively overrides fields in a style context.
type StyleContextOverride struct {
	Colors     *ColorTokens
	Typography *TypographyTokens
	Fonts      *FontRoles
	Spacing    *SpacingTokens
	Motion     *MotionTokens
	Density    *DensityTokens
	Materials  *MaterialRegistry
}

// StyleContextStore stores a style context value.
type StyleContextStore = store.ValueStore[StyleContext]

// Derive returns a new style context with the supplied overrides applied.
func (ctx StyleContext) Derive(overrides StyleContextOverride) StyleContext {
	next := ctx
	changed := false
	if overrides.Colors != nil {
		next.Tokens.Color = *overrides.Colors
		changed = true
	}
	if overrides.Typography != nil {
		next.Tokens.Typography = *overrides.Typography
		changed = true
	}
	if overrides.Fonts != nil {
		next.Tokens.Fonts = *overrides.Fonts
		changed = true
	}
	if overrides.Spacing != nil {
		next.Tokens.Spacing = *overrides.Spacing
		changed = true
	}
	if overrides.Motion != nil {
		next.Tokens.Motion = *overrides.Motion
		changed = true
	}
	if overrides.Density != nil {
		next.Tokens.Density = *overrides.Density
		changed = true
	}
	if overrides.Materials != nil {
		next.Materials = overrides.Materials
		changed = true
	}
	if changed {
		next.Depth = ctx.Depth + 1
	}
	return next
}

// NewRootStyleContext constructs the root style context store.
func NewRootStyleContext(rt any, tokens Tokens, materials *MaterialRegistry) *StyleContextStore {
	if materials == nil {
		materials = NewMaterialRegistry()
	}
	store := store.NewValueStore(StyleContext{
		Tokens:    tokens,
		Materials: materials,
		Depth:     0,
	})
	if rt != nil {
		callMethod(rt, "SetRootStyleContext", store)
	}
	return store
}

type styleContextProvider interface {
	StyleContextStore() *StyleContextStore
}

// NearestStyleContext walks upward from id and returns the nearest style context store.
func NearestStyleContext(tree any, id any) *StyleContextStore {
	if tree == nil {
		return nil
	}
	root := styleContextStoreFromAny(callMethod(tree, "RootStyleContext"))
	if node := callMethod(tree, "FacetByID", id); node != nil {
		for current := node; current != nil; {
			if isNilValue(current) {
				break
			}
			if provider, ok := current.(styleContextProvider); ok {
				if store := provider.StyleContextStore(); store != nil {
					return store
				}
			}
			base := callMethod(current, "Base")
			if base == nil {
				break
			}
			parent := callMethod(base, "Parent")
			if parent == nil {
				break
			}
			current = callMethod(parent, "Impl")
			if current == nil {
				current = parent
			}
		}
	}
	return root
}

func styleContextStoreFromAny(v any) *StyleContextStore {
	if v == nil {
		return nil
	}
	if store, ok := v.(*StyleContextStore); ok {
		return store
	}
	return nil
}

func deriveHover(base Material, tokens Tokens) Material {
	out := cloneMaterial(base)
	overlay := false
	for i := range out.Fills {
		if out.Fills[i].Type == FillSolid {
			out.Fills[i].Color = lightenColor(out.Fills[i].Color, tokens.Color.HoverLighten)
			continue
		}
		overlay = true
	}
	for i := range out.Strokes {
		if out.Strokes[i].Paint.Type == FillSolid {
			out.Strokes[i].Paint.Color = lightenColor(out.Strokes[i].Paint.Color, tokens.Color.HoverLighten)
			continue
		}
		overlay = true
	}
	if overlay {
		out.Fills = append(out.Fills, Fill{
			Type:    FillSolid,
			Color:   whiteColor(),
			Opacity: tokens.Color.HoverLighten,
		})
	}
	return out
}

func derivePressed(base Material, tokens Tokens) Material {
	out := cloneMaterial(base)
	overlay := false
	for i := range out.Fills {
		if out.Fills[i].Type == FillSolid {
			out.Fills[i].Color = darkenColor(out.Fills[i].Color, tokens.Color.PressedDarken)
			continue
		}
		overlay = true
	}
	for i := range out.Strokes {
		if out.Strokes[i].Paint.Type == FillSolid {
			out.Strokes[i].Paint.Color = darkenColor(out.Strokes[i].Paint.Color, tokens.Color.PressedDarken)
			continue
		}
		overlay = true
	}
	if overlay {
		out.Fills = append(out.Fills, Fill{
			Type:    FillSolid,
			Color:   blackColor(),
			Opacity: tokens.Color.PressedDarken,
		})
	}
	return out
}

func deriveDisabled(base Material, tokens Tokens) Material {
	out := cloneMaterial(base)
	out.Opacity = tokens.Color.DisabledOpacity
	if out.Opacity < 0.1 {
		out.Opacity = 0.1
	}
	return out
}

func deriveSelected(base Material, tokens Tokens) Material {
	out := cloneMaterial(base)
	out.Fills = append(out.Fills, Fill{
		Type:    FillSolid,
		Color:   tokens.Color.Primary,
		Opacity: tokens.Color.SelectedOverlay,
	})
	return out
}

func cloneMaterial(base Material) Material {
	out := Material{
		Opacity: base.Opacity,
	}
	if len(base.Fills) > 0 {
		out.Fills = append([]Fill(nil), base.Fills...)
	}
	if len(base.Strokes) > 0 {
		out.Strokes = append([]MaterialStroke(nil), base.Strokes...)
	}
	return out
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

func whiteColor() gfx.Color {
	return gfx.ColorFromRGBA8(255, 255, 255, 255)
}

func blackColor() gfx.Color {
	return gfx.ColorFromRGBA8(0, 0, 0, 255)
}

func clampByte(v float32) uint8 {
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v + 0.5)
}

func (s StyleContext) String() string {
	return fmt.Sprintf("StyleContext{Depth:%d}", s.Depth)
}

func callMethod(target any, name string, args ...any) any {
	if target == nil {
		return nil
	}
	v := reflect.ValueOf(target)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}
	m := v.MethodByName(name)
	if !m.IsValid() {
		return nil
	}
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}
	out := m.Call(in)
	if len(out) == 0 {
		return nil
	}
	return out[0].Interface()
}

func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
