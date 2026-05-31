package assets

import (
	"fmt"
	"testing"
)

// packTestSource implements AssetSource returning stub data for known IDs.
type packTestSource struct {
	data map[AssetID]map[int][]byte
}

func (s packTestSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	if s.data == nil {
		return nil, errNotFound
	}
	lods, ok := s.data[id]
	if !ok {
		return nil, errNotFound
	}
	data, ok := lods[lod]
	if !ok {
		return nil, errNotFound
	}
	return append([]byte(nil), data...), nil
}

var errNotFound = fmt.Errorf("not found")

func mustID(s string) AssetID {
	id, err := ParseAssetID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func TestPackAssetSource_readsFromFirstMatchingPack(t *testing.T) {
	idA := mustID("01234567-89ab-cdef-0123-456789000001")
	idB := mustID("01234567-89ab-cdef-0123-456789000002")

	install := packTestSource{data: map[AssetID]map[int][]byte{
		idA: {0: []byte("install-a")},
		idB: {0: []byte("install-b")},
	}}
	onDemand := packTestSource{data: map[AssetID]map[int][]byte{
		idB: {0: []byte("ondemand-b")},
	}}

	src := NewPackAssetSource([]PackDescriptor{
		{Name: "install", Delivery: DeliveryInstallTime, Source: install},
		{Name: "ondemand", Delivery: DeliveryOnDemand, Source: onDemand},
	})

	// idA is only in install-time pack.
	data, err := src.ReadLOD(idA, 0)
	if err != nil {
		t.Fatalf("ReadLOD(idA): %v", err)
	}
	if string(data) != "install-a" {
		t.Fatalf("got %q, want %q", data, "install-a")
	}

	// idB is in both packs; should return from install-time (first match).
	data, err = src.ReadLOD(idB, 0)
	if err != nil {
		t.Fatalf("ReadLOD(idB): %v", err)
	}
	if string(data) != "install-b" {
		t.Fatalf("got %q from first pack, want %q", data, "install-b")
	}
}

func TestPackAssetSource_unknownAssetReturnsError(t *testing.T) {
	id := mustID("01234567-89ab-cdef-0123-456789000003")
	src := NewPackAssetSource([]PackDescriptor{
		{Name: "install", Delivery: DeliveryInstallTime, Source: packTestSource{}},
	})
	_, err := src.ReadLOD(id, 0)
	if err == nil {
		t.Fatal("expected error for unknown asset")
	}
}

func TestPackAssetSource_addPack(t *testing.T) {
	id := mustID("01234567-89ab-cdef-0123-456789000004")

	src := NewPackAssetSource(nil)
	// Initially empty — read fails.
	_, err := src.ReadLOD(id, 0)
	if err == nil {
		t.Fatal("expected error on empty source")
	}

	// Add a pack and retry.
	src.AddPack(PackDescriptor{
		Name: "late", Delivery: DeliveryOnDemand,
		Source: packTestSource{data: map[AssetID]map[int][]byte{
			id: {0: []byte("late-data")},
		}},
	})
	data, err := src.ReadLOD(id, 0)
	if err != nil {
		t.Fatalf("ReadLOD after AddPack: %v", err)
	}
	if string(data) != "late-data" {
		t.Fatalf("got %q, want %q", data, "late-data")
	}
}
