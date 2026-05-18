package theme

import (
	"sort"
)

// VariantKey identifies a named recipe variant.
type VariantKey string

func (k VariantKey) String() string {
	return string(k)
}

// RecipeResolver resolves a typed recipe for a resolved theme context and variant.
type RecipeResolver[T any] interface {
	Resolve(ctx ResolvedContext, variant VariantKey) T
}

// RecipeFunc adapts a function to RecipeResolver.
type RecipeFunc[T any] func(ctx ResolvedContext, variant VariantKey) T

// Resolve implements RecipeResolver.
func (f RecipeFunc[T]) Resolve(ctx ResolvedContext, variant VariantKey) T {
	return f(ctx, variant)
}

// SlotOverride is an optional override value for a slot.
type SlotOverride[T any] struct {
	HasValue bool
	Value    T
}

// Apply implements SlotPatch.
func (o SlotOverride[T]) Apply(base T) T {
	if !o.HasValue {
		return base
	}
	return o.Value
}

// SlotPatch applies an override to a resolved slot value.
type SlotPatch[T any] interface {
	Apply(base T) T
}

// ResolveSlot applies patches in order and returns the final slot value.
func ResolveSlot[T any](root T, patch ...SlotPatch[T]) T {
	return resolveSlot(root, patch...)
}

func resolveSlot[T any](root T, patch ...SlotPatch[T]) T {
	out := root
	for _, p := range patch {
		if p == nil {
			continue
		}
		out = p.Apply(out)
	}
	return out
}

// SlotSource identifies where a resolved slot value came from.
type SlotSource uint8

const (
	SlotSourceRootDefault SlotSource = iota
	SlotSourceSubtreeOverride
	SlotSourceVariantDefault
	SlotSourceInstanceOverride
)

func (s SlotSource) String() string {
	switch s {
	case SlotSourceRootDefault:
		return "root-default"
	case SlotSourceSubtreeOverride:
		return "subtree-override"
	case SlotSourceVariantDefault:
		return "variant-default"
	case SlotSourceInstanceOverride:
		return "instance-override"
	default:
		return "unknown"
	}
}

// RecipeSlotReport describes one slot and where its value came from.
type RecipeSlotReport struct {
	Name   string
	Source SlotSource
}

// RecipeReport describes a fully resolved recipe.
type RecipeReport struct {
	Family      string
	Variant     VariantKey
	slotSources map[string]SlotSource
}

// NewRecipeReport creates a report for one resolved recipe.
func NewRecipeReport(family string, variant VariantKey) RecipeReport {
	return RecipeReport{
		Family:      family,
		Variant:     variant,
		slotSources: make(map[string]SlotSource),
	}
}

// SetSlotSource records provenance for a slot.
func (r *RecipeReport) SetSlotSource(name string, source SlotSource) {
	if r.slotSources == nil {
		r.slotSources = make(map[string]SlotSource)
	}
	r.slotSources[name] = source
}

// SlotSource returns the recorded provenance for a slot.
func (r RecipeReport) SlotSource(name string) (SlotSource, bool) {
	if r.slotSources == nil {
		return 0, false
	}
	source, ok := r.slotSources[name]
	return source, ok
}

// SlotNames returns all recorded slot names in deterministic order.
func (r RecipeReport) SlotNames() []string {
	if len(r.slotSources) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.slotSources))
	for name := range r.slotSources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Slots returns a deterministic slice of slot provenance reports.
func (r RecipeReport) Slots() []RecipeSlotReport {
	names := r.SlotNames()
	out := make([]RecipeSlotReport, 0, len(names))
	for _, name := range names {
		out = append(out, RecipeSlotReport{
			Name:   name,
			Source: r.slotSources[name],
		})
	}
	return out
}
