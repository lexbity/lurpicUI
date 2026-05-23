package assets

import "testing"

func TestAssetRegistrySetLODReadyAndClearLOD(t *testing.T) {
	reg := NewAssetRegistryStore()
	id, err := ParseAssetID("01234567-89ab-cdef-0123-456789abcdef")
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}

	if got := reg.Get(id); got != nil {
		t.Fatalf("expected nil entry, got %+v", got)
	}

	entry := reg.GetOrCreate(id)
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.State != AssetStateAbsent {
		t.Fatalf("unexpected initial state: %v", entry.State)
	}
	if entry.HighestReadyLOD != -1 {
		t.Fatalf("unexpected initial highest lod: %d", entry.HighestReadyLOD)
	}

	reg.SetLODReady(id, 2, []byte("lod2"), 12)
	entry = reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry after set")
	}
	if entry.State != AssetStateLoading {
		t.Fatalf("expected loading after lod2, got %v", entry.State)
	}
	if entry.HighestReadyLOD != 2 {
		t.Fatalf("unexpected highest lod after lod2: %d", entry.HighestReadyLOD)
	}
	if entry.EntryVersion != 1 {
		t.Fatalf("unexpected entry version after lod2: %d", entry.EntryVersion)
	}
	if got := reg.GlobalVersion(); got != 1 {
		t.Fatalf("unexpected global version after lod2: %d", got)
	}
	if got := entry.LODHandles[2]; got == nil {
		t.Fatal("expected lod2 handle")
	}

	reg.SetLODReady(id, 0, []byte("lod0"), 34)
	entry = reg.Get(id)
	if entry.State != AssetStateReady {
		t.Fatalf("expected ready after lod0, got %v", entry.State)
	}
	if entry.HighestReadyLOD != 0 {
		t.Fatalf("unexpected highest lod after lod0: %d", entry.HighestReadyLOD)
	}
	if entry.EntryVersion != 2 {
		t.Fatalf("unexpected entry version after lod0: %d", entry.EntryVersion)
	}
	if got := reg.GlobalVersion(); got != 2 {
		t.Fatalf("unexpected global version after lod0: %d", got)
	}

	reg.ClearLOD(id, 0)
	entry = reg.Get(id)
	if entry.State != AssetStateLoading {
		t.Fatalf("expected loading after clearing lod0, got %v", entry.State)
	}
	if entry.HighestReadyLOD != 2 {
		t.Fatalf("unexpected highest lod after clearing lod0: %d", entry.HighestReadyLOD)
	}
	if entry.EntryVersion != 3 {
		t.Fatalf("unexpected entry version after clear: %d", entry.EntryVersion)
	}
	if got := reg.GlobalVersion(); got != 3 {
		t.Fatalf("unexpected global version after clear: %d", got)
	}
	if entry.LODHandles[0] != nil {
		t.Fatal("expected lod0 to be cleared")
	}

	reg.ClearLOD(id, 2)
	entry = reg.Get(id)
	if entry.State != AssetStateAbsent {
		t.Fatalf("expected absent after clearing all lods, got %v", entry.State)
	}
	if entry.HighestReadyLOD != -1 {
		t.Fatalf("unexpected highest lod after clearing all: %d", entry.HighestReadyLOD)
	}
	if entry.EntryVersion != 4 {
		t.Fatalf("unexpected entry version after second clear: %d", entry.EntryVersion)
	}
	if got := reg.GlobalVersion(); got != 4 {
		t.Fatalf("unexpected global version after second clear: %d", got)
	}
}

func TestAssetRegistryGetOrCreateReturnsStableEntry(t *testing.T) {
	reg := NewAssetRegistryStore()
	id, err := ParseAssetID("01234567-89ab-cdef-0123-456789abcdee")
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}

	a := reg.GetOrCreate(id)
	b := reg.GetOrCreate(id)
	if a != b {
		t.Fatal("expected stable entry pointer")
	}
}
