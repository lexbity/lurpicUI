package cook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCookCacheSaveLoadRoundTrip(t *testing.T) {
	cache := NewCookCache()
	manifestHash := HashBytes([]byte("manifest"))
	cache.EnsureInvalidation("v1", "linux", manifestHash)

	sourceHash := HashBytes([]byte("source-a"))
	cookedHash := HashBytes([]byte("cooked-a"))
	cache.Record("assets/a.svg", sourceHash, cookedHash)

	dir := t.TempDir()
	path := filepath.Join(dir, ".lurpic-cache", "cook-cache.json")
	if err := cache.Save(path); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	loaded, err := LoadCookCache(path)
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if loaded.CookerVersion != "v1" {
		t.Fatalf("unexpected cooker version: %q", loaded.CookerVersion)
	}
	if loaded.Target != "linux" {
		t.Fatalf("unexpected target: %q", loaded.Target)
	}
	if loaded.manifestHash != manifestHash {
		t.Fatalf("unexpected manifest hash: %x", loaded.manifestHash)
	}
	if got := loaded.SourceHashes["assets/a.svg"]; got != sourceHash {
		t.Fatalf("unexpected source hash: %x", got)
	}
	if got := loaded.CookedHashes["assets/a.svg"]; got != cookedHash {
		t.Fatalf("unexpected cooked hash: %x", got)
	}
}

func TestIncrementalCookerReusesUnchangedSources(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.svg")
	bPath := filepath.Join(dir, "b.svg")
	if err := os.WriteFile(aPath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(bPath, []byte("bravo"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	counts := map[string]int{}
	makeSource := func(path string) IncrementalSource {
		return IncrementalSource{
			Path: path,
			Read: func() ([]byte, error) {
				return os.ReadFile(path)
			},
			Compile: func(src []byte) ([]CompiledLOD, error) {
				counts[path]++
				out := append([]byte(nil), src...)
				return []CompiledLOD{{Level: 0, Data: out}}, nil
			},
		}
	}

	cache := NewCookCache()
	cooker := NewIncrementalCooker(cache)
	manifestHash := HashBytes([]byte("manifest-v1"))

	first, err := cooker.Cook("v1", PlatformLinux, manifestHash, []IncrementalSource{makeSource(aPath), makeSource(bPath)})
	if err != nil {
		t.Fatalf("first cook: %v", err)
	}
	if counts[aPath] != 1 || counts[bPath] != 1 {
		t.Fatalf("unexpected compile counts after first run: %#v", counts)
	}
	if got := string(first[aPath][0].Data); got != "alpha" {
		t.Fatalf("unexpected a output: %q", got)
	}

	if err := os.WriteFile(bPath, []byte("bravo-two"), 0o644); err != nil {
		t.Fatalf("rewrite b: %v", err)
	}

	second, err := cooker.Cook("v1", PlatformLinux, manifestHash, []IncrementalSource{makeSource(aPath), makeSource(bPath)})
	if err != nil {
		t.Fatalf("second cook: %v", err)
	}
	if counts[aPath] != 1 {
		t.Fatalf("expected a to be reused, got compile count %d", counts[aPath])
	}
	if counts[bPath] != 2 {
		t.Fatalf("expected b to recompile, got compile count %d", counts[bPath])
	}
	if got := string(second[aPath][0].Data); got != "alpha" {
		t.Fatalf("unexpected reused a output: %q", got)
	}
	if got := string(second[bPath][0].Data); got != "bravo-two" {
		t.Fatalf("unexpected updated b output: %q", got)
	}
}

func TestCookCacheInvalidatesOnVersionTargetAndManifest(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.svg")
	bPath := filepath.Join(dir, "b.svg")
	if err := os.WriteFile(aPath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(bPath, []byte("bravo"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	counts := map[string]int{}
	makeSource := func(path string) IncrementalSource {
		return IncrementalSource{
			Path: path,
			Read: func() ([]byte, error) {
				return os.ReadFile(path)
			},
			Compile: func(src []byte) ([]CompiledLOD, error) {
				counts[path]++
				return []CompiledLOD{{Level: 0, Data: append([]byte(nil), src...)}}, nil
			},
		}
	}

	type scenario struct {
		name         string
		version      string
		target       Platform
		manifestHash [32]byte
	}

	manifest1 := HashBytes([]byte("manifest-1"))
	manifest2 := HashBytes([]byte("manifest-2"))
	cases := []scenario{
		{name: "version", version: "v2", target: PlatformLinux, manifestHash: manifest1},
		{name: "target", version: "v1", target: PlatformWindows, manifestHash: manifest1},
		{name: "manifest", version: "v1", target: PlatformLinux, manifestHash: manifest2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			counts[aPath] = 0
			counts[bPath] = 0

			cache := NewCookCache()
			cooker := NewIncrementalCooker(cache)
			if _, err := cooker.Cook("v1", PlatformLinux, manifest1, []IncrementalSource{makeSource(aPath), makeSource(bPath)}); err != nil {
				t.Fatalf("seed cook: %v", err)
			}
			if counts[aPath] != 1 || counts[bPath] != 1 {
				t.Fatalf("unexpected counts after seed: %#v", counts)
			}

			if _, err := cooker.Cook(tc.version, tc.target, tc.manifestHash, []IncrementalSource{makeSource(aPath), makeSource(bPath)}); err != nil {
				t.Fatalf("second cook: %v", err)
			}
			if counts[aPath] != 2 || counts[bPath] != 2 {
				t.Fatalf("expected full invalidation for %s, got %#v", tc.name, counts)
			}
		})
	}
}
