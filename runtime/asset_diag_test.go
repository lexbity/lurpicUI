package runtime

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/facet"
)

// assetDiagFixture implements assets.Manager for testing FrameStats population.
type assetDiagFixture struct {
	stats assets.ManagerStats
}

func (f *assetDiagFixture) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }
func (f *assetDiagFixture) LoadSVG(path string) assets.Handle { return assets.Handle{} }
func (f *assetDiagFixture) LoadImage(path string) assets.Handle { return assets.Handle{} }
func (f *assetDiagFixture) LoadTexture(path string) assets.Handle { return assets.Handle{} }
func (f *assetDiagFixture) LoadFont(path string) assets.Handle { return assets.Handle{} }
func (f *assetDiagFixture) LoadConfig(path string, dst any) assets.Handle { return assets.Handle{} }
func (f *assetDiagFixture) Prefetch(paths ...string) {}
func (f *assetDiagFixture) Invalidate(path string) {}
func (f *assetDiagFixture) Close() error { return nil }
func (f *assetDiagFixture) DrainCompleted() int { return 0 }
func (f *assetDiagFixture) Stats() assets.ManagerStats { return f.stats }

// recordingLog captures log messages for assertion.
type recordingLog struct {
	msgs []string
	args []any
}

func (l *recordingLog) Debug(msg string, args ...any) {
	l.msgs = append(l.msgs, msg)
	l.args = append(l.args, args...)
}
func (l *recordingLog) Info(msg string, args ...any) {
	l.msgs = append(l.msgs, msg)
	l.args = append(l.args, args...)
}
func (l *recordingLog) Warn(msg string, args ...any) {
	l.msgs = append(l.msgs, msg)
	l.args = append(l.args, args...)
}
func (l *recordingLog) Error(msg string, args ...any) {
	l.msgs = append(l.msgs, msg)
	l.args = append(l.args, args...)
}

func TestAssetDiagnostics_FrameStatsPopulated(t *testing.T) {
	fixture := &assetDiagFixture{
		stats: assets.ManagerStats{
			TotalEntries:       10,
			LoadingEntries:     3,
			ReadyEntries:       5,
			PartialEntries:     2,
			FailedEntries:      0,
			CPUUsedBytes:       1_000_000,
			CPUBudgetBytes:     10_000_000,
			GPUUsedBytes:       500_000,
			GPUBudgetBytes:     5_000_000,
			EvictionsThisFrame: 1,
			UploadsThisFrame:   2,
			JobsInFlight:       4,
			CacheHitRate:       0.85,
		},
	}

	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = fixture
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()

	stats := rt.LastFrameStats()
	if stats.AssetTotalEntries != 10 {
		t.Fatalf("AssetTotalEntries = %d, want 10", stats.AssetTotalEntries)
	}
	if stats.AssetLoadingEntries != 3 {
		t.Fatalf("AssetLoadingEntries = %d, want 3", stats.AssetLoadingEntries)
	}
	if stats.AssetReadyEntries != 5 {
		t.Fatalf("AssetReadyEntries = %d, want 5", stats.AssetReadyEntries)
	}
	if stats.AssetPartialEntries != 2 {
		t.Fatalf("AssetPartialEntries = %d, want 2", stats.AssetPartialEntries)
	}
	if stats.AssetFailedEntries != 0 {
		t.Fatalf("AssetFailedEntries = %d, want 0", stats.AssetFailedEntries)
	}
	if stats.AssetCPUUsedBytes != 1_000_000 {
		t.Fatalf("AssetCPUUsedBytes = %d, want 1000000", stats.AssetCPUUsedBytes)
	}
	if stats.AssetCPUBudgetBytes != 10_000_000 {
		t.Fatalf("AssetCPUBudgetBytes = %d, want 10000000", stats.AssetCPUBudgetBytes)
	}
	if stats.AssetGPUUsedBytes != 500_000 {
		t.Fatalf("AssetGPUUsedBytes = %d, want 500000", stats.AssetGPUUsedBytes)
	}
	if stats.AssetGPUBudgetBytes != 5_000_000 {
		t.Fatalf("AssetGPUBudgetBytes = %d, want 5000000", stats.AssetGPUBudgetBytes)
	}
	if stats.AssetEvictionsThisFrame != 1 {
		t.Fatalf("AssetEvictionsThisFrame = %d, want 1", stats.AssetEvictionsThisFrame)
	}
	if stats.AssetUploadsThisFrame != 2 {
		t.Fatalf("AssetUploadsThisFrame = %d, want 2", stats.AssetUploadsThisFrame)
	}
	if stats.AssetJobsInFlight != 4 {
		t.Fatalf("AssetJobsInFlight = %d, want 4", stats.AssetJobsInFlight)
	}
	if stats.AssetCacheHitRate != 0.85 {
		t.Fatalf("AssetCacheHitRate = %f, want 0.85", stats.AssetCacheHitRate)
	}
	rt.Shutdown()
}

func TestAssetDiagnostics_JSONPathRegistryLoadsMap(t *testing.T) {
	// Simulate a uuid_registry.json as produced by the cook pipeline.
	jsonData := `{
		"version": 1,
		"records": [
			{"id": "01234567-89ab-cdef-0123-456789000001", "canonical_path": "ui/button.png"},
			{"id": "01234567-89ab-cdef-0123-456789000002", "canonical_path": "fonts/regular.ttf"}
		]
	}`
	reg, err := assets.ParseJSONPathRegistry([]byte(jsonData))
	if err != nil {
		t.Fatalf("ParseJSONPathRegistry: %v", err)
	}

	buttonID := assets.AssetID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0x01, 0x23, 0x45, 0x67, 0x89, 0x00, 0x00, 0x01}
	fontID := assets.AssetID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0x01, 0x23, 0x45, 0x67, 0x89, 0x00, 0x00, 0x02}

	if got := reg.Lookup("ui/button.png"); got != buttonID {
		t.Fatalf("Lookup ui/button.png = %v, want %v", got, buttonID)
	}
	if got := reg.Lookup("fonts/regular.ttf"); got != fontID {
		t.Fatalf("Lookup fonts/regular.ttf = %v, want %v", got, fontID)
	}
	if got := reg.Lookup("missing.png"); !got.IsZero() {
		t.Fatalf("Lookup missing.png = %v, want zero", got)
	}
}

func TestAssetDiagnostics_LoadImageWithPathIDRegistry(t *testing.T) {
	id := assets.AssetID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	path := "ui/button.png"
	idReg := assets.NewMapPathRegistry(map[string]assets.AssetID{path: id})

	reg := assets.NewAssetRegistryStore()
	mgr := assets.NewManager(reg, &passthroughSource{}, assets.BackendSoftware, nil, idReg)

	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr
	cfg.AssetRegistry = reg
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	handle := rt.AssetManager().LoadImage(path)
	if handle.IsZero() {
		t.Fatal("LoadImage returned zero handle — PathIDRegistry not wired")
	}
	if handle.ID != id {
		t.Fatalf("handle.ID = %v, want %v", handle.ID, id)
	}
	if handle.Registry() == nil {
		t.Fatal("handle.Registry() is nil — AssetRegistry not wired")
	}
	rt.Shutdown()
}

// passthroughSource implements AssetSource returning an empty byte slice
// for any LOD. Used in tests where the actual pak contents don't matter.
type passthroughSource struct{}

func (s passthroughSource) ReadLOD(id assets.AssetID, lod int) ([]byte, error) {
	return []byte{}, nil
}

func TestAssetDiagnostics_ProcessDeathHandleRestore(t *testing.T) {
	// Simulate process death: create a manager, load assets, then create
	// a new manager (as happens after process death) and verify the new
	// handles work independently.
	id := assets.AssetID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	// First "lifetime" — create a registry and register an asset.
	reg1 := assets.NewAssetRegistryStore()
	reg1.SetLODReady(id, 0, &assets.DecodedSVGLOD0{Data: []byte("icon")}, 1)

	mgr1 := &assetDiagFixtureWithID{id: id, reg: reg1}
	if rt1 := rtWithManager(t, mgr1, reg1); rt1 != nil {
		handle1 := rt1.AssetManager().LoadSVG("icons/check.svg")
		if handle1.IsZero() {
			t.Fatal("handle1 should be valid in first lifetime")
		}
		rt1.Shutdown()
	}

	// "Process death" — new registry, new manager (simulates restart).
	reg2 := assets.NewAssetRegistryStore()
	reg2.SetLODReady(id, 0, &assets.DecodedSVGLOD0{Data: []byte("icon")}, 1)

	mgr2 := &assetDiagFixtureWithID{id: id, reg: reg2}
	if rt2 := rtWithManager(t, mgr2, reg2); rt2 != nil {
		handle2 := rt2.AssetManager().LoadSVG("icons/check.svg")
		if handle2.IsZero() {
			t.Fatal("handle2 should be valid after process death (new lifetime)")
		}
		// Handle2 must reference reg2 (the new registry), not reg1.
		if handle2.Registry() != reg2 {
			t.Fatal("handle2 must reference the new registry after process death")
		}
		if handle2.Registry() == reg1 {
			t.Fatal("handle2 must NOT reference the old registry (process death)")
		}
		rt2.Shutdown()
	}
}

// assetDiagFixtureWithID implements assets.Manager returning a fixed handle.
type assetDiagFixtureWithID struct {
	id  assets.AssetID
	reg *assets.AssetRegistryStore
}

func (f *assetDiagFixtureWithID) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }
func (f *assetDiagFixtureWithID) LoadSVG(path string) assets.Handle {
	return assets.NewHandle(f.id, f.reg)
}
func (f *assetDiagFixtureWithID) LoadImage(path string) assets.Handle {
	return assets.NewHandle(f.id, f.reg)
}
func (f *assetDiagFixtureWithID) LoadTexture(path string) assets.Handle {
	return assets.NewHandle(f.id, f.reg)
}
func (f *assetDiagFixtureWithID) LoadFont(path string) assets.Handle {
	return assets.NewHandle(f.id, f.reg)
}
func (f *assetDiagFixtureWithID) LoadConfig(path string, dst any) assets.Handle {
	return assets.NewHandle(f.id, f.reg)
}
func (f *assetDiagFixtureWithID) Prefetch(paths ...string) {}
func (f *assetDiagFixtureWithID) Invalidate(path string)   {}
func (f *assetDiagFixtureWithID) Close() error             { return nil }
func (f *assetDiagFixtureWithID) DrainCompleted() int      { return 0 }
func (f *assetDiagFixtureWithID) Stats() assets.ManagerStats { return assets.ManagerStats{} }

func rtWithManager(t *testing.T, mgr assets.Manager, reg *assets.AssetRegistryStore) *Runtime {
	t.Helper()
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr
	cfg.AssetRegistry = reg
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

func TestAssetDiagnostics_ManagerAccessors(t *testing.T) {
	fixture := &assetDiagFixture{}
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = fixture
	cfg.AssetRegistry = assets.NewAssetRegistryStore()
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if rt.AssetManager() == nil {
		t.Fatal("AssetManager() returned nil")
	}
	if rt.AssetRegistry() == nil {
		t.Fatal("AssetRegistry() returned nil")
	}
	rt.Shutdown()
}

func TestAssetDiagnostics_StatsAreZeroWithoutManager(t *testing.T) {
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = nil
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()

	stats := rt.LastFrameStats()
	if stats.AssetTotalEntries != 0 {
		t.Fatalf("AssetTotalEntries = %d, want 0 (no asset manager)", stats.AssetTotalEntries)
	}
	if stats.AssetCPUUsedBytes != 0 {
		t.Fatalf("AssetCPUUsedBytes = %d, want 0", stats.AssetCPUUsedBytes)
	}
	if stats.AssetCacheHitRate != 0 {
		t.Fatalf("AssetCacheHitRate = %f, want 0", stats.AssetCacheHitRate)
	}
	rt.Shutdown()
}

func TestAssetDiagnostics_EventLogMethods(t *testing.T) {
	log := &recordingLog{}
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Logger = log
	cfg.AssetDiagnosticsEnabled = true
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	id, _ := assets.ParseAssetID("01234567-89ab-cdef-0123-456789abc001")

	rt.LogAssetMount("/data/assets.pak", 1024, "abc123def")
	rt.LogAssetExtract(0.5, "copy")
	rt.LogAssetStream(id, assets.AssetTypeImage, 0)
	rt.LogAssetEvict(id, 1, "budget")

	if len(log.msgs) != 4 {
		t.Fatalf("expected 4 log messages, got %d: %v", len(log.msgs), log.msgs)
	}
	if log.msgs[0] != "asset.mount" {
		t.Fatalf("first msg = %q, want %q", log.msgs[0], "asset.mount")
	}
	if log.msgs[1] != "asset.extract" {
		t.Fatalf("second msg = %q, want %q", log.msgs[1], "asset.extract")
	}
	if log.msgs[2] != "asset.stream" {
		t.Fatalf("third msg = %q, want %q", log.msgs[2], "asset.stream")
	}
	if log.msgs[3] != "asset.evict" {
		t.Fatalf("fourth msg = %q, want %q", log.msgs[3], "asset.evict")
	}

	// Verify key-value pairs in args.
	allArgs := fmtArgs(log.args)
	if !strings.Contains(allArgs, "abc123def") {
		t.Fatalf("mount args missing buildHash: %s", allArgs)
	}
	if !strings.Contains(allArgs, "01234567-89ab-cdef-0123-456789abc001") {
		t.Fatalf("stream args missing asset id: %s", allArgs)
	}
	if !strings.Contains(allArgs, "budget") {
		t.Fatalf("evict args missing reason: %s", allArgs)
	}

	rt.Shutdown()
}

func fmtArgs(args []any) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("%v", a))
	}
	return b.String()
}
