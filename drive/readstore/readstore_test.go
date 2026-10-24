package readstore

import (
	"encoding/json"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestValidateStore_AcceptsTheThreeShapes(t *testing.T) {
	vs := store.NewValueStore(1)
	cs := store.NewCollectionStore(func(s string) store.ItemID { return 0 })
	dv := store.NewDerived(func() int { return 0 }, vs)
	for name, s := range map[string]any{"value": vs, "collection": cs, "derived": dv} {
		if err := ValidateStore(s); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if Kind(s) != name {
			t.Errorf("Kind = %q, want %q", Kind(s), name)
		}
	}
	for _, bad := range []any{nil, 42, "x", struct{ x int }{}, &struct{ x int }{}} {
		if err := ValidateStore(bad); err == nil {
			t.Errorf("ValidateStore(%T) must fail", bad)
		}
	}
}

func TestRegistry_CollisionsAndNames(t *testing.T) {
	if _, err := NewRegistry([]NamedStore{
		{Name: "a", Store: store.NewValueStore(1)},
		{Name: "a", Store: store.NewValueStore(2)},
	}); err == nil {
		t.Fatal("duplicate name must be a startup error")
	}
	if _, err := NewRegistry([]NamedStore{{Name: "", Store: store.NewValueStore(1)}}); err == nil {
		t.Fatal("empty name must be a startup error")
	}
	if _, err := NewRegistry([]NamedStore{{Name: "bad", Store: 42}}); err == nil {
		t.Fatal("non-store must be a startup error")
	}
	r, err := NewRegistry([]NamedStore{
		{Name: "b", Store: store.NewValueStore(2)},
		{Name: "a", Store: store.NewValueStore(1)},
	})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if names := r.Names(); names[0] != "a" || names[1] != "b" {
		t.Fatalf("Names = %v, want sorted", names)
	}
	if err := r.Register("a", store.NewValueStore(9)); err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if s, _ := r.Lookup("a"); Kind(s) != "value" {
		t.Fatalf("lookup after re-register: %T", s)
	}
}

func TestReadValue_AllShapes(t *testing.T) {
	vs := store.NewValueStore(map[string]int{"a": 1})
	vPayload, err := ReadValue(vs)
	if err != nil {
		t.Fatalf("value read: %v", err)
	}
	if string(vPayload) != `{"a":1}` {
		t.Fatalf("value payload = %s", vPayload)
	}

	cs := store.NewCollectionStore(func(s string) store.ItemID { return store.ItemID(len(s)) })
	cs.Insert("aa")
	cs.Insert("bbb")
	cPayload, err := ReadValue(cs)
	if err != nil {
		t.Fatalf("collection read: %v", err)
	}
	if string(cPayload) != `["aa","bbb"]` {
		t.Fatalf("collection payload = %s", cPayload)
	}

	dv := store.NewDerived(func() int { return cs.Len() * 10 }, cs)
	dv.Get() // prime
	dPayload, err := ReadValue(dv)
	if err != nil {
		t.Fatalf("derived read: %v", err)
	}
	if string(dPayload) != "20" {
		t.Fatalf("derived payload = %s", dPayload)
	}
}

func TestReadDerivedPeek_PeekOnly(t *testing.T) {
	src := store.NewValueStore(1)
	dv := store.NewDerived(func() int { return src.Get() * 2 }, src)
	src.Set(5) // dirty, never computed
	value, dirty, err := ReadDerivedPeek(dv)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if !dirty || string(value) != "0" {
		t.Fatalf("peek = (%s, %v), want (0, true) — Peek must not recompute", value, dirty)
	}
	// A value store is a typed error, never a silent duplicate of store.read.
	if _, _, err := ReadDerivedPeek(src); err == nil || !strings.Contains(err.Error(), "not a *store.Derived") {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteValue_TypedDecode(t *testing.T) {
	vs := store.NewValueStore(0.5)
	if err := WriteValue(vs, json.RawMessage("0.25")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if vs.Get() != 0.25 {
		t.Fatalf("value = %v", vs.Get())
	}
	if err := WriteValue(vs, json.RawMessage(`"text"`)); err == nil {
		t.Fatal("type-mismatched payload must fail")
	}
	if vs.Get() != 0.25 {
		t.Fatalf("failed write mutated: %v", vs.Get())
	}
	// Collections refuse (Phase 5) without mutating.
	cs := store.NewCollectionStore(func(s string) store.ItemID { return 0 })
	if err := WriteValue(cs, json.RawMessage(`["x"]`)); err == nil {
		t.Fatal("collection write must refuse")
	}
	if cs.Len() != 0 {
		t.Fatalf("refused write mutated: %d items", cs.Len())
	}
	// Derived is read-only by construction.
	dv := store.NewDerived(func() int { return 1 })
	if err := WriteValue(dv, json.RawMessage("2")); err == nil {
		t.Fatal("derived write must refuse")
	}
}

type fieldProbe struct {
	Opacity *store.ValueStore[float64]
	Hidden  string
	// private exists to prove unexported fields fail closed; the static
	// analyzer's unused-field check is satisfied by the assertion below
	// reading it through FieldStore only.
	private *store.ValueStore[int]
}

func TestStructFields_AndFailClosedFieldStore(t *testing.T) {
	fields := StructFields(&fieldProbe{})
	if len(fields) != 1 || fields[0].Name != "Opacity" || fields[0].Kind != "value" {
		t.Fatalf("fields = %+v, want only Opacity/value", fields)
	}
	// Pointer implementations are dereferenced.
	if fields := StructFields((*fieldProbe)(nil)); len(fields) != 1 {
		t.Fatalf("pointer impl fields = %+v", fields)
	}

	p := &fieldProbe{Opacity: store.NewValueStore(0.5)}
	if p.private != nil {
		t.Fatal("fixture invariant: private starts nil")
	}
	s, err := FieldStore(p, "Opacity")
	if err != nil {
		t.Fatalf("field store: %v", err)
	}
	if Kind(s) != "value" {
		t.Fatalf("kind = %q", Kind(s))
	}
	for _, tc := range []struct {
		field, wantErr string
	}{
		{"Missing", "has no field"},
		{"Hidden", "not a store pointer"},
		{"private", "unexported"},
	} {
		if _, err := FieldStore(p, tc.field); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("FieldStore(%q) = %v, want %q failure", tc.field, err, tc.wantErr)
		}
	}
}
