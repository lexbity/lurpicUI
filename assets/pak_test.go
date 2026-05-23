package assets

import (
	"encoding/binary"
	"sort"
	"testing"
	"unsafe"
)

var (
	sinkIndex int
	sinkOK    bool
)

func TestPakStructSizes(t *testing.T) {
	if got := unsafe.Sizeof(PakHeader{}); got != 32 {
		t.Fatalf("unexpected PakHeader size: got %d want 32", got)
	}
	if got := unsafe.Sizeof(PakTOCEntry{}); got != 48 {
		t.Fatalf("unexpected PakTOCEntry size: got %d want 48", got)
	}
}

func TestTOCSearchExactMatch(t *testing.T) {
	toc := makeTOC(8)
	id := toc[5].ID
	idx, ok := tocSearch(toc, id)
	if !ok {
		t.Fatal("expected match")
	}
	if idx != 5 {
		t.Fatalf("unexpected index: got %d want 5", idx)
	}
	if got := toc[idx].ID; got != id {
		t.Fatalf("unexpected id at index: got %s want %s", got, id)
	}
}

func TestTOCSearchSingleItem(t *testing.T) {
	toc := []PakTOCEntry{{ID: makeAssetID(0x1122334455667788, 0x99)}}
	if idx, ok := tocSearch(toc, toc[0].ID); !ok || idx != 0 {
		t.Fatalf("expected single item match, got idx=%d ok=%v", idx, ok)
	}
	if _, ok := tocSearch(toc, makeAssetID(0x1122334455667788, 0x98)); ok {
		t.Fatal("expected absent match to fail")
	}
}

func TestTOCSearchPrefixCollision(t *testing.T) {
	prefix := uint64(0x0123456789abcdef)
	toc := []PakTOCEntry{
		{ID: makeAssetID(prefix, 0x01)},
		{ID: makeAssetID(prefix, 0x02)},
		{ID: makeAssetID(prefix, 0x03)},
		{ID: makeAssetID(prefix+1, 0x04)},
		{ID: makeAssetID(prefix+2, 0x05)},
	}
	sortTOC(toc)

	idx, ok := tocSearch(toc, makeAssetID(prefix, 0x03))
	if !ok {
		t.Fatal("expected prefix collision target to be found")
	}
	if got := toc[idx].ID; got != makeAssetID(prefix, 0x03) {
		t.Fatalf("unexpected id: got %s", got)
	}

	if idx, ok := tocSearch(toc, makeAssetID(prefix, 0xff)); ok {
		t.Fatalf("expected absent colliding id to fail, got idx=%d", idx)
	}
}

func TestTOCSearchDuplicateEntries(t *testing.T) {
	id := makeAssetID(0xfeedfacecafebeef, 0x42)
	toc := []PakTOCEntry{
		{ID: id},
		{ID: id},
		{ID: makeAssetID(0xffeeddccbbaa9988, 0x01)},
	}
	sortTOC(toc)

	idx, ok := tocSearch(toc, id)
	if !ok {
		t.Fatal("expected duplicate id to be found")
	}
	if got := toc[idx].ID; got != id {
		t.Fatalf("unexpected duplicate match: got %s want %s", got, id)
	}
	if idx != 0 {
		t.Fatalf("expected leftmost duplicate match at index 0, got %d", idx)
	}
}

func TestTOCSearchAbsent(t *testing.T) {
	toc := makeTOC(16)
	if idx, ok := tocSearch(toc, makeAssetID(0x9999999999999999, 0x12)); ok {
		t.Fatalf("expected absent match to fail, got idx=%d", idx)
	}
}

func BenchmarkTOCSearch100(b *testing.B)   { benchmarkTOCSearch(b, 100) }
func BenchmarkTOCSearch1000(b *testing.B)  { benchmarkTOCSearch(b, 1000) }
func BenchmarkTOCSearch10000(b *testing.B) { benchmarkTOCSearch(b, 10000) }

func benchmarkTOCSearch(b *testing.B, n int) {
	b.Helper()
	toc := makeTOC(n)
	target := toc[n/2].ID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkIndex, sinkOK = tocSearch(toc, target)
	}
}

func makeTOC(n int) []PakTOCEntry {
	toc := make([]PakTOCEntry, n)
	for i := 0; i < n; i++ {
		prefix := uint64(i*2 + 1)
		toc[i] = PakTOCEntry{
			ID: makeAssetID(prefix, uint64(i)),
		}
	}
	sortTOC(toc)
	return toc
}

func sortTOC(toc []PakTOCEntry) {
	sort.Slice(toc, func(i, j int) bool {
		ki := binary.BigEndian.Uint64(toc[i].ID[:8])
		kj := binary.BigEndian.Uint64(toc[j].ID[:8])
		if ki != kj {
			return ki < kj
		}
		return toc[i].ID.String() < toc[j].ID.String()
	})
}

func makeAssetID(prefix uint64, tail uint64) AssetID {
	var id AssetID
	binary.BigEndian.PutUint64(id[:8], prefix)
	binary.BigEndian.PutUint64(id[8:], tail)
	return id
}
