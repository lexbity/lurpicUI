package cook

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
)

func TestAssetIDStringAndJSONRoundTrip(t *testing.T) {
	var id assets.AssetID
	for i := range id {
		id[i] = byte(i)
	}

	want := "00010203-0405-0607-0809-0a0b0c0d0e0f"
	if got := id.String(); got != want {
		t.Fatalf("unexpected string form: got %q want %q", got, want)
	}
	if got := binary.BigEndian.Uint64(id[:8]); got != 0x0001020304050607 {
		t.Fatalf("unexpected big-endian prefix: got %#x", got)
	}

	data, err := json.Marshal(struct {
		ID assets.AssetID `json:"id"`
	}{ID: id})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTrip struct {
		ID assets.AssetID `json:"id"`
	}
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundTrip.ID != id {
		t.Fatalf("json round-trip mismatch: got %v want %v", roundTrip.ID, id)
	}
}

func TestUUIDRegistrySaveLoadRoundTrip(t *testing.T) {
	reg := NewUUIDRegistry()
	ids := []assets.AssetID{
		mustAssign(t, reg, "assets/icons/home.svg"),
		mustAssign(t, reg, "assets/fonts/inter.ttf"),
		mustAssign(t, reg, "assets/config/theme.toml"),
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "asset-ids.json")
	if err := reg.SaveTo(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadUUIDRegistry(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	for i, canonical := range []string{
		"assets/icons/home.svg",
		"assets/fonts/inter.ttf",
		"assets/config/theme.toml",
	} {
		got := loaded.Lookup(canonical)
		if got != ids[i] {
			t.Fatalf("lookup %q mismatch: got %s want %s", canonical, got, ids[i])
		}
	}

	records := loaded.Records()
	if len(records) != 3 {
		t.Fatalf("unexpected record count: %d", len(records))
	}
	for i := 1; i < len(records); i++ {
		if records[i-1].CanonicalPath > records[i].CanonicalPath {
			t.Fatalf("records not sorted: %q before %q", records[i-1].CanonicalPath, records[i].CanonicalPath)
		}
	}
}

func TestUUIDRegistryConcurrentAssign(t *testing.T) {
	var counter atomic.Uint64
	reg := NewUUIDRegistry()
	reg.generator = func() (assets.AssetID, error) {
		n := counter.Add(1)
		var id assets.AssetID
		binary.BigEndian.PutUint64(id[:8], n)
		binary.BigEndian.PutUint64(id[8:], n<<1|1)
		id[6] = (id[6] & 0x0f) | 0x40
		id[8] = (id[8] & 0x3f) | 0x80
		return id, nil
	}

	const paths = 32
	const repeats = 24

	var wg sync.WaitGroup
	errCh := make(chan error, paths*repeats)
	for i := 0; i < paths; i++ {
		path := fmt.Sprintf("assets/images/icon-%02d.svg", i)
		for j := 0; j < repeats; j++ {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				if _, err := reg.Assign(p); err != nil {
					errCh <- err
				}
			}(path)
		}
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("assign failed: %v", err)
	}

	if got := len(reg.Records()); got != paths {
		t.Fatalf("unexpected registry size: got %d want %d", got, paths)
	}

	for i := 0; i < paths; i++ {
		path := fmt.Sprintf("assets/images/icon-%02d.svg", i)
		id1 := reg.Lookup(path)
		id2 := reg.Lookup(path)
		if id1.IsZero() {
			t.Fatalf("missing id for %q", path)
		}
		if id1 != id2 {
			t.Fatalf("lookup not stable for %q: %s vs %s", path, id1, id2)
		}
	}
}

func TestUUIDRegistryCollisionRetry(t *testing.T) {
	reg := NewUUIDRegistry()

	var first assets.AssetID
	for i := range first {
		first[i] = byte(0xaa + i)
	}
	first[6] = (first[6] & 0x0f) | 0x40
	first[8] = (first[8] & 0x3f) | 0x80

	var second assets.AssetID
	for i := range second {
		second[i] = byte(0x10 + i)
	}
	second[6] = (second[6] & 0x0f) | 0x40
	second[8] = (second[8] & 0x3f) | 0x80

	generated := 0
	reg.generator = func() (assets.AssetID, error) {
		generated++
		switch generated {
		case 1:
			return first, nil
		case 2:
			return first, nil
		default:
			return second, nil
		}
	}

	idA, err := reg.Assign("assets/a.svg")
	if err != nil {
		t.Fatalf("assign a: %v", err)
	}
	if idA != first {
		t.Fatalf("unexpected first id: %s", idA)
	}

	idB, err := reg.Assign("assets/b.svg")
	if err != nil {
		t.Fatalf("assign b: %v", err)
	}
	if idB != second {
		t.Fatalf("unexpected collision-resolved id: %s", idB)
	}
	if idB == idA {
		t.Fatal("collision was not resolved")
	}
	if got := reg.Lookup("assets/b.svg"); got != second {
		t.Fatalf("lookup after collision mismatch: got %s want %s", got, second)
	}
}

func mustAssign(t *testing.T, reg *UUIDRegistry, path string) assets.AssetID {
	t.Helper()
	id, err := reg.Assign(path)
	if err != nil {
		t.Fatalf("assign %q: %v", path, err)
	}
	return id
}
