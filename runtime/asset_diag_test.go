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
