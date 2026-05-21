package theme

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/text"
)

// DensityID identifies an app-defined density scale.
type DensityID string

const (
	// DensityIDComfortable is the default density scale.
	DensityIDComfortable DensityID = "comfortable"
	// DensityIDCompact is the compact density scale.
	DensityIDCompact DensityID = "compact"
	// DensityIDTouch is the touch-oriented density scale.
	DensityIDTouch DensityID = "touch"
)

// DensityScale resolves density-sensitive layout and typography values.
type DensityScale struct {
	ID      DensityID
	Factor  float32
	Spacing SpacingTokens
	Type    TypographyTokens
}

// Normalize returns a density scale with a valid factor.
func (s DensityScale) Normalize() DensityScale {
	if s.Factor <= 0 {
		s.Factor = 1
	}
	return s
}

// Scale multiplies a raw value by the density factor.
func (s DensityScale) Scale(value float32) float32 {
	if s.Factor <= 0 {
		return value
	}
	return value * s.Factor
}

// ResolveSpacing returns the density-adjusted spacing value for a token.
func (s DensityScale) ResolveSpacing(token SpacingToken) float32 {
	switch token {
	case SpacingXS:
		return s.Spacing.XS
	case SpacingS:
		return s.Spacing.SM
	case SpacingM:
		return s.Spacing.MD
	case SpacingL:
		return s.Spacing.LG
	case SpacingXL:
		return s.Spacing.XL
	case SpacingXXL:
		return s.Spacing.XXL
	default:
		return 0
	}
}

// ResolveTextStyle returns the density-adjusted text style for a token.
func (s DensityScale) ResolveTextStyle(token TextToken) text.TextStyle {
	switch token {
	case TextBodyS:
		return s.Type.BodySmall
	case TextLabelM:
		return s.Type.LabelMedium
	case TextLabelS:
		return s.Type.LabelSmall
	case TextHeadingS:
		return s.Type.HeadlineSmall
	case TextMonoM:
		return s.Type.DataLabel
	case TextMonoS:
		return s.Type.DataAnnotation
	case TextBodyM:
		fallthrough
	default:
		return s.Type.BodyMedium
	}
}

// DefaultDensityScale constructs a density scale from the canonical token set.
func DefaultDensityScale(id DensityID, tokens Tokens) DensityScale {
	factor := float32(1)
	switch id {
	case DensityIDCompact:
		factor = 0.93
	case DensityIDTouch:
		factor = 1.08
	case DensityIDComfortable:
		fallthrough
	default:
		factor = 1
	}
	return DensityScale{
		ID:      id,
		Factor:  factor,
		Spacing: scaleSpacingTokens(tokens.Spacing, factor),
		Type:    scaleTypographyTokens(tokens.Typography, factor),
	}.Normalize()
}

// DefaultDensityScales returns the shipped density scale map.
func DefaultDensityScales(tokens Tokens) map[DensityID]DensityScale {
	return map[DensityID]DensityScale{
		DensityIDComfortable: DefaultDensityScale(DensityIDComfortable, tokens),
		DensityIDCompact:     DefaultDensityScale(DensityIDCompact, tokens),
		DensityIDTouch:       DefaultDensityScale(DensityIDTouch, tokens),
	}
}

// ResolvedContext is the concrete, runtime-resolved theme context.
type ResolvedContext struct {
	defaultContext

	Materials    *MaterialRegistry
	Density      DensityScale
	Viewport     gfx.Size
	FontRegistry *text.FontRegistry

	ContentScale     float32
	WritingDirection layout.WritingDirection
	Depth            int
	Resolver         *ThemeResolver
}

// DefaultResolvedContext returns the canonical resolved theme context.
func DefaultResolvedContext() ResolvedContext {
	tokens := DefaultTokens()
	scales := DefaultDensityScales(tokens)
	materials := NewMaterialRegistry()
	return ResolvedContext{
		defaultContext:   defaultContext{tokens: tokens},
		Materials:        materials,
		Density:          scales[DensityIDComfortable],
		FontRegistry:     nil,
		ContentScale:     1,
		WritingDirection: layout.WritingDirectionLTR,
		Resolver:         DefaultThemeResolver(tokens),
	}
}

// Default returns the canonical resolved theme context.
func Default() ResolvedContext {
	return DefaultResolvedContext()
}

// TokenSet returns the underlying token table.
func (c ResolvedContext) TokenSet() Tokens {
	return c.defaultContext.tokens
}

// Color resolves a semantic color token.
func (c ResolvedContext) Color(t ColorToken) gfx.Color {
	return c.defaultContext.Color(t)
}

// Radius resolves a radius token.
func (c ResolvedContext) Radius(t RadiusToken) layout.ResolvedScalar {
	return c.defaultContext.Radius(t)
}

// Spacing resolves a spacing token through the selected density scale.
func (c ResolvedContext) Spacing(t SpacingToken) layout.ResolvedScalar {
	if c.Density.Spacing != (SpacingTokens{}) {
		return layout.ResolvedScalar(c.Density.ResolveSpacing(t))
	}
	return c.defaultContext.Spacing(t)
}

// TextStyle resolves a text token through the selected density scale.
func (c ResolvedContext) TextStyle(t TextToken) text.TextStyle {
	if c.Density.Type != (TypographyTokens{}) {
		style := c.Density.ResolveTextStyle(t)
		return c.defaultContext.tokens.Fonts.ResolveTextStyle(t, style, c.FontRegistry)
	}
	return c.defaultContext.TextStyle(t)
}

// WithDensity returns a copy with a different density scale.
func (c ResolvedContext) WithDensity(scale DensityScale) ResolvedContext {
	c.Density = scale.Normalize()
	return c
}

// WithResolver returns a copy with a different recipe resolver.
func (c ResolvedContext) WithResolver(resolver *ThemeResolver) ResolvedContext {
	c.Resolver = resolver
	return c
}

// WithViewport returns a copy with a different viewport size.
func (c ResolvedContext) WithViewport(size gfx.Size) ResolvedContext {
	c.Viewport = size
	return c
}

// WithWritingDirection returns a copy with a different writing direction.
func (c ResolvedContext) WithWritingDirection(direction layout.WritingDirection) ResolvedContext {
	c.WritingDirection = direction
	return c
}

// WithMaterials returns a copy with a different material registry.
func (c ResolvedContext) WithMaterials(materials *MaterialRegistry) ResolvedContext {
	c.Materials = materials
	return c
}

// WithFontRegistry returns a copy with a different font registry.
func (c ResolvedContext) WithFontRegistry(reg *text.FontRegistry) ResolvedContext {
	c.FontRegistry = reg
	return c
}

// ResolveLayerLayoutRecipe looks up a layer layout recipe through the attached resolver.
func (c ResolvedContext) ResolveLayerLayoutRecipe(ref layout.LayerLayoutRecipeRef) (layout.ResolvedLayerLayoutRecipe, bool) {
	if c.Resolver == nil {
		return layout.ResolvedLayerLayoutRecipe{}, false
	}
	return c.Resolver.ResolveLayerLayoutRecipe(ref, c)
}

// ResolveGroupLayoutRecipe looks up a group layout recipe through the attached resolver.
func (c ResolvedContext) ResolveGroupLayoutRecipe(ref layout.GroupLayoutRecipeRef) (layout.ResolvedGroupLayoutRecipe, bool) {
	if c.Resolver == nil {
		return layout.ResolvedGroupLayoutRecipe{}, false
	}
	return c.Resolver.ResolveGroupLayoutRecipe(ref, c)
}

// RecipeKey identifies one resolved recipe by family and name.
type RecipeKey struct {
	Family string
	Name   string
}

// RecipeState identifies a state-specific recipe variant.
type RecipeState string

const (
	RecipeStateDefault  RecipeState = "default"
	RecipeStateHover    RecipeState = "hover"
	RecipeStateActive   RecipeState = "active"
	RecipeStateDisabled RecipeState = "disabled"
	RecipeStateFocused  RecipeState = "focused"
	RecipeStateSelected RecipeState = "selected"
	RecipeStateInvalid  RecipeState = "invalid"
)

// StateRecipeCatalog resolves state-specific recipes with explicit fallback order.
type StateRecipeCatalog[T any] struct {
	exact         map[RecipeKey]map[RecipeState]T
	familyDefault map[string]map[RecipeState]T
	globalDefault map[RecipeState]T
}

// NewStateRecipeCatalog constructs an empty state recipe catalog.
func NewStateRecipeCatalog[T any]() *StateRecipeCatalog[T] {
	return &StateRecipeCatalog[T]{
		exact:         make(map[RecipeKey]map[RecipeState]T),
		familyDefault: make(map[string]map[RecipeState]T),
		globalDefault: make(map[RecipeState]T),
	}
}

// RegisterExact records an exact family/name/state recipe.
func (c *StateRecipeCatalog[T]) RegisterExact(family, name string, state RecipeState, value T) {
	if c == nil {
		return
	}
	key := RecipeKey{Family: family, Name: name}
	states, ok := c.exact[key]
	if !ok {
		states = make(map[RecipeState]T)
		c.exact[key] = states
	}
	states[state] = value
}

// RegisterFamilyDefault records a family-level fallback recipe.
func (c *StateRecipeCatalog[T]) RegisterFamilyDefault(family string, state RecipeState, value T) {
	if c == nil {
		return
	}
	states, ok := c.familyDefault[family]
	if !ok {
		states = make(map[RecipeState]T)
		c.familyDefault[family] = states
	}
	states[state] = value
}

// RegisterGlobalDefault records a global fallback recipe.
func (c *StateRecipeCatalog[T]) RegisterGlobalDefault(state RecipeState, value T) {
	if c == nil {
		return
	}
	c.globalDefault[state] = value
}

// Resolve returns the first matching recipe according to the fallback order.
func (c *StateRecipeCatalog[T]) Resolve(family, name string, state RecipeState) (T, bool) {
	var zero T
	if c == nil {
		return zero, false
	}
	if states, ok := c.exact[RecipeKey{Family: family, Name: name}]; ok {
		if value, ok := states[state]; ok {
			return value, true
		}
		if value, ok := states[RecipeStateDefault]; ok {
			return value, true
		}
	}
	if states, ok := c.familyDefault[family]; ok {
		if value, ok := states[state]; ok {
			return value, true
		}
		if value, ok := states[RecipeStateDefault]; ok {
			return value, true
		}
	}
	if value, ok := c.globalDefault[state]; ok {
		return value, true
	}
	if value, ok := c.globalDefault[RecipeStateDefault]; ok {
		return value, true
	}
	return zero, false
}

// MustResolve returns the resolved recipe or panics if the fallback chain is exhausted.
func (c *StateRecipeCatalog[T]) MustResolve(family, name string, state RecipeState) T {
	if value, ok := c.Resolve(family, name, state); ok {
		return value
	}
	panic(fmt.Sprintf("theme: no recipe fallback for family %q name %q state %q", family, name, state))
}

// LayerLayoutRecipeFunc resolves a concrete layer layout recipe for a context.
type LayerLayoutRecipeFunc func(ctx ResolvedContext) layout.ResolvedLayerLayoutRecipe

// GroupLayoutRecipeFunc resolves a concrete group layout recipe for a context.
type GroupLayoutRecipeFunc func(ctx ResolvedContext) layout.ResolvedGroupLayoutRecipe

// ThemeResolver resolves density and layout recipes for a resolved theme context.
type ThemeResolver struct {
	densities    map[DensityID]DensityScale
	layerRecipes map[layout.LayerLayoutRecipeRef]LayerLayoutRecipeFunc
	groupRecipes map[layout.GroupLayoutRecipeRef]GroupLayoutRecipeFunc
}

// NewThemeResolver constructs an empty theme resolver.
func NewThemeResolver() *ThemeResolver {
	return &ThemeResolver{
		densities:    make(map[DensityID]DensityScale),
		layerRecipes: make(map[layout.LayerLayoutRecipeRef]LayerLayoutRecipeFunc),
		groupRecipes: make(map[layout.GroupLayoutRecipeRef]GroupLayoutRecipeFunc),
	}
}

// DefaultThemeResolver constructs a resolver with the canonical density scales.
func DefaultThemeResolver(tokens Tokens) *ThemeResolver {
	resolver := NewThemeResolver()
	for _, scale := range DefaultDensityScales(tokens) {
		_ = resolver.RegisterDensityScale(scale)
	}
	return resolver
}

// RegisterDensityScale stores a density scale for later resolution.
func (r *ThemeResolver) RegisterDensityScale(scale DensityScale) error {
	if r == nil {
		return fmt.Errorf("theme: nil resolver")
	}
	scale = scale.Normalize()
	if scale.ID == "" {
		return fmt.Errorf("theme: cannot register density scale with empty id")
	}
	if _, ok := r.densities[scale.ID]; ok {
		return fmt.Errorf("theme: duplicate density scale %q", scale.ID)
	}
	r.densities[scale.ID] = scale
	return nil
}

// DensityScale returns a registered density scale if present.
func (r *ThemeResolver) DensityScale(id DensityID) (DensityScale, bool) {
	if r == nil {
		return DensityScale{}, false
	}
	scale, ok := r.densities[id]
	return scale, ok
}

// MustDensityScale returns a registered density scale or panics.
func (r *ThemeResolver) MustDensityScale(id DensityID) DensityScale {
	if scale, ok := r.DensityScale(id); ok {
		return scale
	}
	panic(fmt.Sprintf("theme: unknown density scale %q", id))
}

// RegisterLayerLayoutRecipe stores a layer recipe resolver.
func (r *ThemeResolver) RegisterLayerLayoutRecipe(ref layout.LayerLayoutRecipeRef, fn LayerLayoutRecipeFunc) error {
	if r == nil {
		return fmt.Errorf("theme: nil resolver")
	}
	if ref.Family == "" || ref.Name == "" {
		return fmt.Errorf("theme: cannot register layer layout recipe with empty reference %q/%q", ref.Family, ref.Name)
	}
	if fn == nil {
		return fmt.Errorf("theme: cannot register layer layout recipe %q/%q with nil resolver", ref.Family, ref.Name)
	}
	if _, ok := r.layerRecipes[ref]; ok {
		return fmt.Errorf("theme: duplicate layer layout recipe %q/%q", ref.Family, ref.Name)
	}
	r.layerRecipes[ref] = fn
	return nil
}

// RegisterGroupLayoutRecipe stores a group recipe resolver.
func (r *ThemeResolver) RegisterGroupLayoutRecipe(ref layout.GroupLayoutRecipeRef, fn GroupLayoutRecipeFunc) error {
	if r == nil {
		return fmt.Errorf("theme: nil resolver")
	}
	if ref.Family == "" || ref.Name == "" {
		return fmt.Errorf("theme: cannot register group layout recipe with empty reference %q/%q", ref.Family, ref.Name)
	}
	if fn == nil {
		return fmt.Errorf("theme: cannot register group layout recipe %q/%q with nil resolver", ref.Family, ref.Name)
	}
	if _, ok := r.groupRecipes[ref]; ok {
		return fmt.Errorf("theme: duplicate group layout recipe %q/%q", ref.Family, ref.Name)
	}
	r.groupRecipes[ref] = fn
	return nil
}

// ResolveLayerLayoutRecipe resolves a layer layout recipe if one is registered.
func (r *ThemeResolver) ResolveLayerLayoutRecipe(ref layout.LayerLayoutRecipeRef, ctx ResolvedContext) (layout.ResolvedLayerLayoutRecipe, bool) {
	if r == nil {
		return layout.ResolvedLayerLayoutRecipe{}, false
	}
	fn, ok := r.layerRecipes[ref]
	if !ok || fn == nil {
		return layout.ResolvedLayerLayoutRecipe{}, false
	}
	return fn(ctx), true
}

// MustResolveLayerLayoutRecipe resolves a layer layout recipe or panics.
func (r *ThemeResolver) MustResolveLayerLayoutRecipe(ref layout.LayerLayoutRecipeRef, ctx ResolvedContext) layout.ResolvedLayerLayoutRecipe {
	if recipe, ok := r.ResolveLayerLayoutRecipe(ref, ctx); ok {
		return recipe
	}
	panic(fmt.Sprintf("theme: no layer layout recipe registered for %q/%q", ref.Family, ref.Name))
}

// ResolveGroupLayoutRecipe resolves a group layout recipe if one is registered.
func (r *ThemeResolver) ResolveGroupLayoutRecipe(ref layout.GroupLayoutRecipeRef, ctx ResolvedContext) (layout.ResolvedGroupLayoutRecipe, bool) {
	if r == nil {
		return layout.ResolvedGroupLayoutRecipe{}, false
	}
	fn, ok := r.groupRecipes[ref]
	if !ok || fn == nil {
		return layout.ResolvedGroupLayoutRecipe{}, false
	}
	return fn(ctx), true
}

// MustResolveGroupLayoutRecipe resolves a group layout recipe or panics.
func (r *ThemeResolver) MustResolveGroupLayoutRecipe(ref layout.GroupLayoutRecipeRef, ctx ResolvedContext) layout.ResolvedGroupLayoutRecipe {
	if recipe, ok := r.ResolveGroupLayoutRecipe(ref, ctx); ok {
		return recipe
	}
	panic(fmt.Sprintf("theme: no group layout recipe registered for %q/%q", ref.Family, ref.Name))
}

func scaleSpacingTokens(base SpacingTokens, factor float32) SpacingTokens {
	return SpacingTokens{
		XXS:           base.XXS * factor,
		XS:            base.XS * factor,
		SM:            base.SM * factor,
		MD:            base.MD * factor,
		LG:            base.LG * factor,
		XL:            base.XL * factor,
		XXL:           base.XXL * factor,
		IconSize:      base.IconSize * factor,
		TouchTarget:   base.TouchTarget * factor,
		DividerWeight: base.DividerWeight * factor,
		BorderWeight:  base.BorderWeight * factor,
	}
}

func scaleTypographyTokens(base TypographyTokens, factor float32) TypographyTokens {
	scale := func(style text.TextStyle) text.TextStyle {
		style.Size *= factor
		style.LineHeight *= factor
		style.LetterSpacing *= factor
		return style
	}
	return TypographyTokens{
		DisplayLarge:   scale(base.DisplayLarge),
		DisplayMedium:  scale(base.DisplayMedium),
		DisplaySmall:   scale(base.DisplaySmall),
		HeadlineLarge:  scale(base.HeadlineLarge),
		HeadlineMedium: scale(base.HeadlineMedium),
		HeadlineSmall:  scale(base.HeadlineSmall),
		TitleLarge:     scale(base.TitleLarge),
		TitleMedium:    scale(base.TitleMedium),
		TitleSmall:     scale(base.TitleSmall),
		BodyLarge:      scale(base.BodyLarge),
		BodyMedium:     scale(base.BodyMedium),
		BodySmall:      scale(base.BodySmall),
		LabelLarge:     scale(base.LabelLarge),
		LabelMedium:    scale(base.LabelMedium),
		LabelSmall:     scale(base.LabelSmall),
		DataLabel:      scale(base.DataLabel),
		DataAnnotation: scale(base.DataAnnotation),
		ChartTitle:     scale(base.ChartTitle),
		ChartSubtitle:  scale(base.ChartSubtitle),
	}
}
