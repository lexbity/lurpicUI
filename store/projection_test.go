package store

import (
	"strings"
	"testing"
)

func TestValueStore_mutation_during_projection_panics(t *testing.T) {
	SetProjectionActiveCheck(func() bool { return true })
	defer SetProjectionActiveCheck(nil)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from projection-phase mutation")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "projection phase") {
			t.Fatalf("panic message %q missing \"projection phase\"", msg)
		}
	}()

	s := NewValueStore(0)
	s.Set(1)
}

func TestMapStore_mutation_during_projection_panics(t *testing.T) {
	SetProjectionActiveCheck(func() bool { return true })
	defer SetProjectionActiveCheck(nil)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from projection-phase mutation")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "projection phase") {
			t.Fatalf("panic message %q missing \"projection phase\"", msg)
		}
	}()

	m := NewMapStore[string, int]()
	m.Set("key", 1)
}

func TestMapStore_delete_during_projection_panics(t *testing.T) {
	SetProjectionActiveCheck(func() bool { return true })
	defer SetProjectionActiveCheck(nil)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from projection-phase mutation")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "projection phase") {
			t.Fatalf("panic message %q missing \"projection phase\"", msg)
		}
	}()

	m := NewMapStore[string, int]()
	m.Set("key", 1)
	m.Delete("key")
}

func TestCollectionStore_insert_during_projection_panics(t *testing.T) {
	SetProjectionActiveCheck(func() bool { return true })
	defer SetProjectionActiveCheck(nil)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from projection-phase mutation")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "projection phase") {
			t.Fatalf("panic message %q missing \"projection phase\"", msg)
		}
	}()

	c := NewCollectionStore(func(v int) ItemID { return ItemID(v) })
	c.Insert(1)
}

func TestCollectionStore_remove_during_projection_panics(t *testing.T) {
	SetProjectionActiveCheck(func() bool { return true })
	defer SetProjectionActiveCheck(nil)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from projection-phase mutation")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "projection phase") {
			t.Fatalf("panic message %q missing \"projection phase\"", msg)
		}
	}()

	c := NewCollectionStore(func(v int) ItemID { return ItemID(v) })
	c.Insert(1)
	c.Remove(ItemID(1))
}
