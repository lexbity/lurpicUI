package readstore

import (
	"encoding/json"
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestRegistryValidatesExplicitDeclarationsAndFreezes(t *testing.T) {
	first := store.ValueExport("a", store.NewValueStore(1), store.DomainState, "integer", 32, store.JSONFreezer[int])
	second := store.ValueExport("b", store.NewValueStore(2), store.ViewState, "integer", 32, store.JSONFreezer[int])
	if _, err := NewRegistry([]store.ReadExport{first, first}); err == nil {
		t.Fatal("duplicate public name must fail")
	}
	registry, err := NewRegistry([]store.ReadExport{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Names(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("sorted names = %v", got)
	}
	if _, ok := registry.Lookup("missing"); ok {
		t.Fatal("undeclared name resolved")
	}
	if err := registry.Register(store.ValueExport("c", store.NewValueStore(3), store.DomainState, "integer", 32, store.JSONFreezer[int])); err != nil {
		t.Fatalf("pre-freeze registration: %v", err)
	}
	registry.Freeze()
	if err := registry.Register(store.ValueExport("d", store.NewValueStore(4), store.DomainState, "integer", 32, store.JSONFreezer[int])); err == nil {
		t.Fatal("frozen catalog accepted a new export")
	}
}

func TestTypedExportCaptureAndCollectionPages(t *testing.T) {
	rows := store.NewCollectionStore(func(s string) store.ItemID {
		switch s {
		case "alpha":
			return 1
		case "beta":
			return 2
		default:
			return 3
		}
	})
	for _, row := range []string{"alpha", "beta", "gamma"} {
		rows.Insert(row)
	}
	export := store.CollectionExport("rows", rows, store.DomainState, "string", 1024, store.JSONFreezer[string])
	page, version, total, _, err := export.Capture(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(page) != `{"items":["beta"],"offset":1,"total":"3"}` {
		t.Fatalf("page=%s", page)
	}
	if version == 0 || total != 3 {
		t.Fatalf("version=%d total=%d", version, total)
	}
	all, allVersion, err := export.CaptureCollection()
	if err != nil || allVersion != version || len(all) != 3 || string(all[0]) != `"alpha"` || string(all[2]) != `"gamma"` {
		t.Fatalf("immutable collection capture = %q version=%d err=%v", all, allVersion, err)
	}
	value := store.ValueExport("label", store.NewValueStore("hello"), store.ViewState, "string", 32, store.JSONFreezer[string])
	payload, _, _, _, err := value.Capture(0, 1)
	if err != nil || string(payload) != `"hello"` {
		t.Fatalf("value=%s err=%v", payload, err)
	}
	if _, _, _, _, err := value.Capture(1, 1); err == nil {
		t.Fatal("scalar export accepted a nonzero offset")
	}
}

func TestExportReadNeverMutatesAndFreezesMutablePayload(t *testing.T) {
	value := store.NewValueStore("stable")
	rows := store.NewCollectionStore(func([]int) store.ItemID { return 1 })
	item := []int{3, 5, 8}
	rows.Insert(item)
	export := store.CollectionExport("rows", rows, store.DomainState, "array<integer>", 1024, store.JSONFreezer[[]int])
	beforeVersion := rows.Version()
	payload, version, _, _, err := export.Capture(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	item[0] = 99
	if string(payload) != `{"items":[[3,5,8]],"offset":0,"total":"1"}` {
		t.Fatalf("captured collection changed or had wrong DTO shape: %s", payload)
	}
	if rows.Version() != beforeVersion || version != uint64(beforeVersion) {
		t.Fatalf("read changed source version: before=%d after=%d captured=%d", beforeVersion, rows.Version(), version)
	}
	valueVersion := value.Version()
	valueExport := store.ValueExport("value", value, store.ViewState, "string", 64, store.JSONFreezer[string])
	if _, _, _, _, err := valueExport.Capture(0, 1); err != nil {
		t.Fatal(err)
	}
	if value.Version() != valueVersion {
		t.Fatalf("value read changed version: before=%d after=%d", valueVersion, value.Version())
	}
}

func TestDerivedExportPeekDoesNotEvaluateOrMutate(t *testing.T) {
	source := store.NewValueStore(2)
	evaluations := 0
	derived := store.NewDerived(func() int { evaluations++; return source.Get() * 3 }, source)
	export := store.DerivedExport("triple", derived, store.ViewState, "integer", 32, store.JSONFreezer[int])
	source.Set(4)
	beforeEvals, beforeRecomputes := store.SnapshotFrameDerivedCounters()
	value, version, _, dirty, err := export.Capture(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	afterEvals, afterRecomputes := store.SnapshotFrameDerivedCounters()
	if string(value) != "0" || version != 0 || !dirty {
		t.Fatalf("peek value=%s version=%d dirty=%v", value, version, dirty)
	}
	if evaluations != 0 || beforeEvals != afterEvals || beforeRecomputes != afterRecomputes {
		t.Fatalf("peek evaluated or changed counters: evaluations=%d before=%d/%d after=%d/%d", evaluations, beforeEvals, beforeRecomputes, afterEvals, afterRecomputes)
	}
	if _, err := json.Marshal(value); err != nil {
		t.Fatal(err)
	}
}
