package store

import (
	"testing"
)

func TestValueStore_mutation_during_projection_panics(t *testing.T) {
	withActiveProjectionCheck(t)
	defer expectProjectionPanic(t)
	s := NewValueStore(0)
	s.Set(1)
}

func TestMapStore_mutation_during_projection_panics(t *testing.T) {
	withActiveProjectionCheck(t)
	defer expectProjectionPanic(t)
	m := NewMapStore[string, int]()
	m.Set("key", 1)
}

func TestMapStore_delete_during_projection_panics(t *testing.T) {
	m := NewMapStore[string, int]()
	m.Set("key", 1)
	withActiveProjectionCheck(t)
	defer expectProjectionPanic(t)
	m.Delete("key")
}

func TestCollectionStore_insert_during_projection_panics(t *testing.T) {
	withActiveProjectionCheck(t)
	defer expectProjectionPanic(t)
	c := NewCollectionStore(func(v int) ItemID { return ItemID(v) })
	c.Insert(1)
}

func TestCollectionStore_remove_during_projection_panics(t *testing.T) {
	c := NewCollectionStore(func(v int) ItemID { return ItemID(v) })
	c.Insert(1)
	withActiveProjectionCheck(t)
	defer expectProjectionPanic(t)
	c.Remove(ItemID(1))
}
