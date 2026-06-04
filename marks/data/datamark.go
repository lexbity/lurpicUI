package data

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

// DataMark is the base for data-bound marks that map collection items
// through scales to positioned child facets.
//
// Concrete marks (Point, Bar, Line) embed DataMark and supply visual
// encoding; DataMark owns the data lifecycle, scale mapping, region feed,
// and coordinate narrowing.
type DataMark[T any] struct {
	marks.Core

	Store  *store.CollectionStore[T]
	Binder *CollectionBinder[T]
	Scales map[marks.Channel]*reactive.ReactiveScale

	regionStore         *store.ValueStore[[2]float64]
	childPositions      []gfx.Point
	childPositionsDirty bool
}

// NewDataMark constructs a DataMark with the given collection store,
// child factory, and channel→scale bindings.
func NewDataMark[T any](
	parent *facet.Facet,
	store *store.CollectionStore[T],
	factory func(T) facet.FacetImpl,
	scales map[marks.Channel]*reactive.ReactiveScale,
	region *store.ValueStore[[2]float64],
) *DataMark[T] {
	m := &DataMark[T]{
		Store:               store,
		Scales:              scales,
		regionStore:         region,
		childPositionsDirty: true,
	}
	m.Core.Facet = facet.NewFacet()
	m.Binder = NewCollectionBinder(parent, store, factory)
	return m
}

// Descriptor returns a base descriptor for data-bound marks.
// Concrete marks should override Family and TypeName.
func (m *DataMark[T]) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "datamark"}
}

// BoundData returns the underlying data store, satisfying marks.DataBound.
func (m *DataMark[T]) BoundData() any {
	return m.Store
}

// MapPosition converts a domain value through the named channel's scale
// into a pixel position. Returns 0 if the channel or scale is missing.
func (m *DataMark[T]) MapPosition(ch marks.Channel, value float64) float64 {
	rs, ok := m.Scales[ch]
	if !ok {
		return 0
	}
	return rs.Get().Map(value)
}

// MapPositions converts domain coordinates through x/y scales into
// gfx.Point pixel positions. Returns (0,0) if either scale is missing.
func (m *DataMark[T]) MapPositions(xChan, yChan marks.Channel, x, y float64) gfx.Point {
	return Pt(m.MapPosition(xChan, x), m.MapPosition(yChan, y))
}

// InvertPosition converts a pixel position back through the named channel's
// scale to a domain value. Returns 0 if the channel or scale is missing
// or the scale does not support Invert.
func (m *DataMark[T]) InvertPosition(ch marks.Channel, pixel float64) float64 {
	rs, ok := m.Scales[ch]
	if !ok {
		return 0
	}
	if inv, ok := rs.Get().(scale.InvertibleScale); ok {
		return inv.Invert(pixel)
	}
	return 0
}
