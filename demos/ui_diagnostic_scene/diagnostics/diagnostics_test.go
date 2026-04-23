package diagnostics

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
)

func TestOverlayKind_String(t *testing.T) {
	tests := []struct {
		kind OverlayKind
		want string
	}{
		{OverlayOff, "Off"},
		{OverlayBounds, "Bounds"},
		{OverlayDirty, "Dirty"},
		{OverlayHitRegions, "HitRegions"},
		{OverlayAnchors, "Anchors"},
		{OverlayFocus, "Focus"},
		{OverlayLayers, "Layers"},
		{OverlayTiming, "Timing"},
		{OverlayAll, "All"},
		{OverlayKind(99), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.kind.String()
		if got != tt.want {
			t.Errorf("OverlayKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestOverlayKind_IsValid(t *testing.T) {
	if !OverlayOff.IsValid() {
		t.Error("OverlayOff should be valid")
	}
	if !OverlayAll.IsValid() {
		t.Error("OverlayAll should be valid")
	}
	if OverlayKind(99).IsValid() {
		t.Error("OverlayKind(99) should not be valid")
	}
}

func TestOverlayKind_CanCombine(t *testing.T) {
	if OverlayOff.CanCombine() {
		t.Error("OverlayOff should not be combinable")
	}
	if OverlayAll.CanCombine() {
		t.Error("OverlayAll should not be combinable")
	}
	if !OverlayBounds.CanCombine() {
		t.Error("OverlayBounds should be combinable")
	}
}

func TestEventCategory_String(t *testing.T) {
	if EventPointer.String() != "Pointer" {
		t.Error("wrong string for EventPointer")
	}
	if EventKeyboard.String() != "Keyboard" {
		t.Error("wrong string for EventKeyboard")
	}
}

func TestSeverity_String(t *testing.T) {
	if SeverityDebug.String() != "Debug" {
		t.Error("wrong string for SeverityDebug")
	}
	if SeverityError.String() != "Error" {
		t.Error("wrong string for SeverityError")
	}
}

func TestNewEvent(t *testing.T) {
	e := NewEvent(EventScene, SeverityWarning, "test message", "test-source")

	if e.Category != EventScene {
		t.Errorf("expected category EventScene, got %v", e.Category)
	}
	if e.Severity != SeverityWarning {
		t.Errorf("expected severity Warning, got %v", e.Severity)
	}
	if e.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", e.Message)
	}
	if e.Source != "test-source" {
		t.Errorf("expected source 'test-source', got %s", e.Source)
	}
	if e.Data != nil {
		t.Error("expected nil data")
	}
	if time.Since(e.Timestamp) > time.Second {
		t.Error("event timestamp seems wrong")
	}
}

func TestNewEventWithData(t *testing.T) {
	data := map[string]any{"key": "value"}
	e := NewEventWithData(EventStore, SeverityInfo, "msg", "src", data)

	if !e.HasData() {
		t.Error("expected HasData() to be true")
	}
	if e.Data["key"] != "value" {
		t.Error("data not preserved correctly")
	}
}

func TestEvent_IsImportant(t *testing.T) {
	debugEvent := NewEvent(EventScene, SeverityDebug, "debug", "src")
	if debugEvent.IsImportant() {
		t.Error("debug event should not be important")
	}

	warningEvent := NewEvent(EventScene, SeverityWarning, "warn", "src")
	if !warningEvent.IsImportant() {
		t.Error("warning event should be important")
	}

	errorEvent := NewEvent(EventScene, SeverityError, "err", "src")
	if !errorEvent.IsImportant() {
		t.Error("error event should be important")
	}
}

func TestFrameStatsView_FPS(t *testing.T) {
	// 16.67ms = 60fps
	stats := FrameStatsView{
		TotalDuration: 16_670_000, // nanoseconds
	}
	fps := stats.FPS()
	if fps < 59 || fps > 61 {
		t.Errorf("expected ~60fps, got %f", fps)
	}
}

func TestFrameStatsView_IsBudgetHealthy(t *testing.T) {
	healthy := FrameStatsView{TotalDuration: 10_000_000}
	if !healthy.IsBudgetHealthy() {
		t.Error("10ms should be healthy")
	}

	unhealthy := FrameStatsView{TotalDuration: 20_000_000}
	if unhealthy.IsBudgetHealthy() {
		t.Error("20ms should not be healthy")
	}
}

func TestFrameStatsHistory(t *testing.T) {
	h := NewFrameStatsHistory(5)

	// Add 3 entries
	for i := 0; i < 3; i++ {
		h.Add(FrameStatsView{FrameNumber: uint64(i)})
	}

	if h.Latest().FrameNumber != 2 {
		t.Error("latest should be frame 2")
	}

	all := h.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}

	// Add more to wrap around
	for i := 3; i < 7; i++ {
		h.Add(FrameStatsView{FrameNumber: uint64(i)})
	}

	// Should now have 5 entries (2, 3, 4, 5, 6)
	all = h.GetAll()
	if len(all) != 5 {
		t.Errorf("expected 5 entries after wrap, got %d", len(all))
	}
}

func TestFrameStatsHistory_AverageFPS(t *testing.T) {
	h := NewFrameStatsHistory(10)

	// Add 10 frames at exactly 16.67ms (60fps)
	for i := 0; i < 10; i++ {
		h.Add(FrameStatsView{
			TotalDuration: 16_670_000,
		})
	}

	avg := h.AverageFPS()
	if avg < 59 || avg > 61 {
		t.Errorf("expected average ~60fps, got %f", avg)
	}
}

func TestHitSummary_IsEmpty(t *testing.T) {
	empty := HitSummary{TotalRegions: 0}
	if !empty.IsEmpty() {
		t.Error("should be empty")
	}

	nonEmpty := HitSummary{TotalRegions: 5}
	if nonEmpty.IsEmpty() {
		t.Error("should not be empty")
	}
}

func TestFocusSummary_FocusDepth(t *testing.T) {
	empty := FocusSummary{}
	if empty.FocusDepth() != 0 {
		t.Error("empty focus should have depth 0")
	}

	withChain := FocusSummary{
		FocusChain: []facet.FacetID{1, 2, 3},
	}
	if withChain.FocusDepth() != 3 {
		t.Error("focus chain of 3 should have depth 3")
	}
}

func TestFocusSummary_IsInChain(t *testing.T) {
	f := FocusSummary{
		FocusChain: []facet.FacetID{1, 2, 3},
	}

	if !f.IsInChain(1) {
		t.Error("1 should be in chain")
	}
	if !f.IsInChain(2) {
		t.Error("2 should be in chain")
	}
	if f.IsInChain(4) {
		t.Error("4 should not be in chain")
	}
}

func TestInvalidationSummary_IsClean(t *testing.T) {
	clean := InvalidationSummary{TotalDirtyFacets: 0}
	if !clean.IsClean() {
		t.Error("should be clean")
	}

	dirty := InvalidationSummary{TotalDirtyFacets: 5}
	if dirty.IsClean() {
		t.Error("should not be clean")
	}
}

func TestInvalidationSummary_DirtyFlagNames(t *testing.T) {
	summary := InvalidationSummary{}

	// None
	names := summary.DirtyFlagNames(0)
	if len(names) != 1 || names[0] != "None" {
		t.Errorf("expected [None], got %v", names)
	}

	// Layout only
	names = summary.DirtyFlagNames(facet.DirtyLayout)
	if len(names) != 1 || names[0] != "Layout" {
		t.Errorf("expected [Layout], got %v", names)
	}

	// Multiple flags
	names = summary.DirtyFlagNames(facet.DirtyLayout | facet.DirtyProjection)
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %v", names)
	}
}

func TestRenderBatchSummary_IsEmpty(t *testing.T) {
	empty := RenderBatchSummary{TotalBatches: 0}
	if !empty.IsEmpty() {
		t.Error("should be empty")
	}

	nonEmpty := RenderBatchSummary{TotalBatches: 10}
	if nonEmpty.IsEmpty() {
		t.Error("should not be empty")
	}
}

func TestAnchorSummary_IsEmpty(t *testing.T) {
	empty := AnchorSummary{TotalAnchors: 0}
	if !empty.IsEmpty() {
		t.Error("should be empty")
	}

	nonEmpty := AnchorSummary{TotalAnchors: 5}
	if nonEmpty.IsEmpty() {
		t.Error("should not be empty")
	}
}

func TestSceneCapabilitySummary_HasFamily(t *testing.T) {
	s := SceneCapabilitySummary{
		Families: []string{"basic", "structure"},
	}

	if !s.HasFamily("basic") {
		t.Error("should have 'basic' family")
	}
	if !s.HasFamily("structure") {
		t.Error("should have 'structure' family")
	}
	if s.HasFamily("chart") {
		t.Error("should not have 'chart' family")
	}
}

func TestActiveOverlays(t *testing.T) {
	a := NewActiveOverlays("test-scene")

	if a.AnyEnabled() {
		t.Error("new overlays should have nothing enabled")
	}

	// Enable bounds
	a.SetEnabled(OverlayBounds, true)
	if !a.IsEnabled(OverlayBounds) {
		t.Error("bounds should be enabled")
	}
	if !a.AnyEnabled() {
		t.Error("should have some overlays enabled")
	}

	// Toggle off
	off := a.Toggle(OverlayBounds)
	if off {
		t.Error("toggle should return false when disabling")
	}
	if a.IsEnabled(OverlayBounds) {
		t.Error("bounds should be toggled off")
	}

	// Enable multiple
	a.SetEnabled(OverlayBounds, true)
	a.SetEnabled(OverlayDirty, true)
	list := a.EnabledList()
	if len(list) != 2 {
		t.Errorf("expected 2 enabled overlays, got %d", len(list))
	}

	// Clear
	a.Clear()
	if a.AnyEnabled() {
		t.Error("should have no overlays after clear")
	}
}

func TestNewAdapter(t *testing.T) {
	a := NewAdapter()
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if a.frameStats == nil {
		t.Fatal("expected frameStats to be initialized")
	}
	if a.frameStats.capacity != 120 {
		t.Fatalf("expected capacity 120, got %d", a.frameStats.capacity)
	}
}

func TestAdapter_IsAvailable(t *testing.T) {
	a := NewAdapter()
	if a.IsAvailable() {
		t.Error("new adapter should not be available")
	}

	// Even with inspector, it's "available"
	// (though specific features may not be)
	a.SetInspector(diagnostics.NewInspector(nil))
	if !a.IsAvailable() {
		t.Error("adapter with inspector should be available")
	}
}

func TestAdapter_UpdateFrameStats(t *testing.T) {
	a := NewAdapter()

	engineStats := diagnostics.FrameStats{
		FrameNumber:      1,
		DirtyFacets:      5,
		ProjectedFacets:  10,
		RenderBatchCount: 3,
		LayoutDuration:   5_000_000, // 5ms
		ProjectDuration:  3_000_000, // 3ms
		RenderDuration:   8_000_000, // 8ms
	}

	a.UpdateFrameStats(engineStats)

	latest := a.GetFrameStats().Latest()
	if latest.FrameNumber != 1 {
		t.Errorf("expected frame 1, got %d", latest.FrameNumber)
	}
	if latest.DirtyFacetCount != 5 {
		t.Errorf("expected 5 dirty facets, got %d", latest.DirtyFacetCount)
	}

	// Total duration should be sum of all phases
	expectedTotal := time.Duration(16_000_000) // 5+3+8ms
	if latest.TotalDuration != expectedTotal {
		t.Errorf("expected total duration %v, got %v", expectedTotal, latest.TotalDuration)
	}
}

func TestAdapter_GetFrameStats_empty(t *testing.T) {
	a := NewAdapter()
	stats := a.GetFrameStats()
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	latest := stats.Latest()
	if latest.FrameNumber != 0 {
		t.Error("expected empty latest stats")
	}
}

func TestAdapter_GetInvalidationSummary_noInspector(t *testing.T) {
	a := NewAdapter()
	summary := a.GetInvalidationSummary()

	if summary.TotalDirtyFacets != 0 {
		t.Error("expected 0 dirty facets without inspector")
	}
	if summary.ByFlag == nil {
		t.Error("expected non-nil ByFlag map")
	}
	if summary.BySource == nil {
		t.Error("expected non-nil BySource map")
	}
}

func TestAdapter_GetHitSummary_noProbe(t *testing.T) {
	a := NewAdapter()
	summary := a.GetHitSummary()

	if !summary.IsEmpty() {
		t.Error("expected empty summary without probe")
	}
}

func TestAdapter_GetAnchorSummary_noInspector(t *testing.T) {
	a := NewAdapter()
	summary := a.GetAnchorSummary()

	if !summary.IsEmpty() {
		t.Error("expected empty summary without inspector")
	}
	if summary.ByParent == nil {
		t.Error("expected non-nil ByParent map")
	}
}
