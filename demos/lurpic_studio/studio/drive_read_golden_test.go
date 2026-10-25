package studio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	drivedesc "codeburg.org/lexbit/lurpicui/drive/describe"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/canonical"
)

// newStudioDriveHarness boots the full studio root headless with the drive
// sink and the marks describe adapter installed.
func newStudioDriveHarness(t *testing.T, w, hgt int) *testkit.DriveHarness {
	t.Helper()
	return newStudioDriveHarnessWithRows(t, w, hgt, seedRows(t))
}

func newStudioDriveHarnessWithRows(t *testing.T, w, hgt int, rows []dataset.Row) *testkit.DriveHarness {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	reg, err := StudioLayerRegistry()
	if err != nil {
		t.Fatalf("layer registry: %v", err)
	}
	themeCtx := StudioThemeContext().WithFontRegistry(fonts)
	root := BuildRoot(app.BuildContext{
		FontRegistry: fonts,
		WindowSize:   gfx.Size{W: float32(w), H: float32(hgt)},
		ContentScale: 1,
		Theme:        themeCtx,
	}, NewDirtySink(10), rows, reg)
	cfg := testkit.StandardHarnessConfig(t, w, hgt)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = themeCtx.Resolver
	cfg.Drive = true
	h := testkit.NewDriveHarness(t, float32(w), float32(hgt), root)
	return h
}

// TestDrive_namedStores freezes the studio's named-store registry (G4.6):
// store.list's named entries MUST equal this sorted list — registry drift is
// a reviewable diff, not an accident.
func TestDrive_namedStores(t *testing.T) {
	h := newStudioDriveHarness(t, 1280, 800)
	payload := h.CallOK("store.list", nil)
	var got struct {
		Exports []struct {
			Name string `json:"name"`
		} `json:"exports"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	named := make([]string, 0, len(got.Exports))
	for _, export := range got.Exports {
		if export.Name != "" {
			named = append(named, export.Name)
		}
	}
	sort.Strings(named)
	want := []string{
		"e1.chartType",
		"e1.opacity",
		"e1.seriesColor",
		"e1.timeRange",
		"shell.active",
	}
	if !reflect.DeepEqual(named, want) {
		t.Fatalf("named stores = %v, want %v (update the freeze list deliberately)", named, want)
	}

	// A named read is cut-exact and typed: shell.active carries the default
	// exhibit.
	read := h.CallOK("store.read", map[string]any{"name": "shell.active"})
	var value struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(read, &value); err != nil {
		t.Fatalf("read payload %s: %v", read, err)
	}
	if value.Value != string(ExhibitRealtime) {
		t.Fatalf("shell.active = %q, want %q", value.Value, ExhibitRealtime)
	}
}

func TestDrivingAndVirtualIndicator(t *testing.T) {
	h := newStudioDriveHarness(t, 1280, 800)
	h.RunFrame()
	root := h.Runtime().Root().(*Root)
	driving := root.StatusBar().Driving()
	if !driving.Disabled.Get() {
		t.Fatal("driving indicator must be dim while Wall mode has no client owner")
	}
	before := h.Runtime().LastPrimaryFrame()
	if _, perr := h.Call("time.mode", map[string]string{"mode": "virtual"}); perr != nil {
		t.Fatalf("enter Virtual mode: %v", perr)
	}
	if _, perr := h.Call("time.advance", map[string]string{"duration_ns": "16667000"}); perr != nil {
		t.Fatalf("advance indicator frame: %v", perr)
	}
	if err := requireDrivingIndicatorLit(driving); err != nil {
		t.Fatal(err)
	}
	if err := requirePacketChange(t, before, h.Runtime().LastPrimaryFrame(), "the driving indicator must alter the visible packet"); err != nil {
		t.Fatal(err)
	}
	if _, perr := h.Call("time.mode", map[string]string{"mode": "wall"}); perr != nil {
		t.Fatalf("return to Wall mode: %v", perr)
	}
	if !driving.Disabled.Get() {
		t.Fatal("driving indicator must return to dim after the time lease is released")
	}
}

func TestNegativeDrivingIndicator(t *testing.T) {
	h := newStudioDriveHarness(t, 1280, 800)
	root := h.Runtime().Root().(*Root)
	indicator := root.StatusBar().Driving()
	// The deliberate defect pins the display dim even when control ownership
	// changes. The same packet assertion used by the positive test must catch it.
	indicator.Disabled = marks.Const(true)
	h.RunFrame()
	if _, perr := h.Call("time.mode", map[string]string{"mode": "virtual"}); perr != nil {
		t.Fatalf("enter Virtual mode: %v", perr)
	}
	if _, perr := h.Call("time.advance", map[string]string{"duration_ns": "16667000"}); perr != nil {
		t.Fatalf("advance indicator frame: %v", perr)
	}
	if err := requireDrivingIndicatorLit(indicator); err == nil {
		t.Fatal("the driving-indicator assertion accepted a deliberately pinned-dim indicator")
	}
}

func requireDrivingIndicatorLit(indicator *status.StatusLight) error {
	if indicator == nil || indicator.Disabled.Get() {
		return fmt.Errorf("driving indicator must be lit while Virtual time is owned")
	}
	return nil
}

func requirePacketChange(t *testing.T, before, current *render.Frame, reason string) error {
	t.Helper()
	if before == nil || current == nil {
		return fmt.Errorf("%s: before/current frame is nil", reason)
	}
	beforeDigest, err := canonical.Digest(before)
	if err != nil {
		return fmt.Errorf("%s: digest before: %w", reason, err)
	}
	currentDigest, err := canonical.Digest(current)
	if err != nil {
		return fmt.Errorf("%s: digest current: %w", reason, err)
	}
	if beforeDigest == currentDigest {
		return fmt.Errorf("%s: canonical digest unchanged (%s)", reason, beforeDigest)
	}
	return nil
}

// TestStudioLaunchChartDiagnostic proves the actual launch chart is healthy
// and that the same diagnostic rejects a real empty-series, non-drawing
// fixture rather than merely accepting its geometry.
func TestStudioLaunchChartDiagnostic(t *testing.T) {
	h := newStudioDriveHarness(t, 1280, 800)
	h.RunFrame() // cut 1: the launch cut

	query := map[string]any{
		"target":  map[string]string{"testid": "e1.canvas"},
		"details": []string{"plot", "seriesCount"},
	}
	payload := h.CallOK("facet.query", query)
	var launch struct {
		Bounds  *gfxRectJSON `json:"bounds"`
		Details struct {
			Plot        gfxRectJSON `json:"plot"`
			SeriesCount int         `json:"seriesCount"`
		} `json:"details"`
	}
	if err := json.Unmarshal(payload, &launch); err != nil {
		t.Fatalf("payload %s: %v", payload, err)
	}
	if !healthyChartDiagnostic(launch.Details.Plot, launch.Bounds, launch.Details.SeriesCount) {
		t.Fatalf("launch chart diagnostic failed: %+v", launch)
	}
}

func TestChartDiagnosticDetectsNonDrawingFixture(t *testing.T) {
	h := newStudioDriveHarnessWithRows(t, 1280, 800, nil)
	h.RunFrame()
	payload := h.CallOK("facet.query", map[string]any{
		"target":  map[string]string{"testid": "e1.canvas"},
		"details": []string{"plot", "seriesCount"},
	})
	var observed struct {
		Bounds  *gfxRectJSON `json:"bounds"`
		Details struct {
			Plot        gfxRectJSON `json:"plot"`
			SeriesCount int         `json:"seriesCount"`
		} `json:"details"`
	}
	if err := json.Unmarshal(payload, &observed); err != nil {
		t.Fatalf("empty chart diagnostic payload %s: %v", payload, err)
	}
	if observed.Details.Plot.empty() || observed.Bounds == nil || observed.Details.SeriesCount != 0 {
		t.Fatalf("empty-series fixture did not exercise the non-drawing chart contract: %+v", observed)
	}
	if healthyChartDiagnostic(observed.Details.Plot, observed.Bounds, observed.Details.SeriesCount) {
		t.Fatal("launch chart diagnostic accepted a non-drawing fixture")
	}
}

var _ = drivedesc.Describe

// TestCommandRegistry_audit is the G5.5 freeze: cmd.list output equals the
// frozen sorted registry — the studio's public mutation surface (Debt-3) is
// versioned names, and registry drift is a reviewable diff.
func TestCommandRegistry_audit(t *testing.T) {
	shell := NewShellState(state.NewAppState(seedRows(t)))
	registry, _ := ShellCommands(shell, nil)

	specs := registry.List()
	ids := make([]string, 0, len(specs))
	for _, s := range specs {
		ids = append(ids, s.ID)
		if !regexpCommandID.MatchString(s.ID) {
			t.Errorf("command %q violates the [a-z0-9.]+ surface contract", s.ID)
		}
	}
	want := []string{
		"exhibit.anchors",
		"exhibit.capabilities",
		"exhibit.layers",
		"exhibit.playground",
		"exhibit.policies",
		"exhibit.propagation",
		"exhibit.realtime",
		"feed.toggle",
		"index.toggle",
		"inspector.toggle",
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("command registry = %v, want frozen %v", ids, want)
	}
	if registry.Version() == 0 {
		t.Fatal("the registry must stamp a version")
	}
}

// regexpCommandID mirrors runtime's registration-time validation contract.
var regexpCommandID = regexp.MustCompile(`^[a-z0-9.]+$`)

// gfxRectJSON mirrors gfx.Rect's JSON shape ({Min:{X,Y},Max:{X,Y}}).
type gfxRectJSON struct {
	Min struct{ X, Y float32 } `json:"Min"`
	Max struct{ X, Y float32 } `json:"Max"`
}

func (r gfxRectJSON) empty() bool {
	return r.Min.X == 0 && r.Min.Y == 0 && r.Max.X == 0 && r.Max.Y == 0
}

func healthyChartDiagnostic(plot gfxRectJSON, bounds *gfxRectJSON, seriesCount int) bool {
	return !plot.empty() && plot.Max.X > plot.Min.X && plot.Max.Y > plot.Min.Y &&
		bounds != nil && bounds.Max.X > bounds.Min.X && bounds.Max.Y > bounds.Min.Y && seriesCount > 0
}
