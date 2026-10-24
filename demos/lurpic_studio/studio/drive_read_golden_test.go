package studio

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	drivedesc "codeburg.org/lexbit/lurpicui/drive/describe"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// newStudioDriveHarness boots the full studio root headless with the drive
// sink and the marks describe adapter installed.
func newStudioDriveHarness(t *testing.T, w, hgt int) *testkit.DriveHarness {
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
	}, NewDirtySink(10), seedRows(t), reg)
	cfg := testkit.StandardHarnessConfig(t, w, hgt)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = themeCtx.Resolver
	cfg.Drive = true
	h := testkit.NewDriveHarness(t, float32(w), float32(hgt), root)
	// The live app registers named stores in app.Config.OnRuntime; the
	// harness replays that wiring against the harness runtime.
	if r, ok := root.(*Root); ok {
		for _, ns := range r.NamedStores() {
			if err := h.Runtime().RegisterNamedStore(ns.Name, ns.Store); err != nil {
				t.Fatalf("register %s: %v", ns.Name, err)
			}
		}
	}
	return h
}

// TestDrive_namedStores freezes the studio's named-store registry (G4.6):
// store.list's named entries MUST equal this sorted list — registry drift is
// a reviewable diff, not an accident.
func TestDrive_namedStores(t *testing.T) {
	h := newStudioDriveHarness(t, 1280, 800)
	payload := h.CallOK("store.list", nil)
	var got struct {
		Stores []struct {
			Target string `json:"target"`
			Field  string `json:"field"`
		} `json:"stores"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	named := make([]string, 0, len(got.Stores))
	for _, s := range got.Stores {
		if s.Field == "" {
			named = append(named, s.Target)
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

// TestDemoBlankChart is the spec's motivating diagnostic, executable as a
// command (acceptance criterion 7): facet.query {testid: "e1.canvas",
// "details": true} inspects the canvas plot rect at the launch cut.
//
// The spec pinned the known E1 blank-chart launch bug here with
// assert-empty-at-launch, flipping to assert-non-empty when §F-1's fix
// landed. Verification during Phase 4 (with a faithful gfx.Rect decode)
// shows the bug does NOT reproduce in the current tree: the plot rect is
// non-degenerate from the launch cut. The detector therefore runs at its
// healthy polarity — non-empty plot, populated series, real bounds — and
// fails loudly if a blank-at-launch (arranged-but-never-drawing) regression
// ever lands. This test is that regression's detector (§F-1).
func TestDemoBlankChart(t *testing.T) {
	h := newStudioDriveHarness(t, 1280, 800)
	h.RunFrame() // cut 1: the launch cut

	query := map[string]any{
		"target":  map[string]string{"testid": "e1.canvas"},
		"details": true,
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
	if launch.Details.Plot.empty() {
		t.Fatalf("BLANK CHART AT LAUNCH (the pinned E1 bug): plot = %+v — the detector caught a regression", launch.Details.Plot)
	}
	if launch.Details.Plot.Max.X <= launch.Details.Plot.Min.X ||
		launch.Details.Plot.Max.Y <= launch.Details.Plot.Min.Y {
		t.Fatalf("degenerate launch plot: %+v", launch.Details.Plot)
	}
	if launch.Bounds == nil {
		t.Fatal("launch canvas must report arranged bounds")
	}
	if launch.Details.SeriesCount == 0 {
		t.Fatalf("launch canvas plots zero series from the seed: %+v", launch.Details)
	}
}

var _ = drivedesc.Describe

// gfxRectJSON mirrors gfx.Rect's JSON shape ({Min:{X,Y},Max:{X,Y}}).
type gfxRectJSON struct {
	Min struct{ X, Y float32 } `json:"Min"`
	Max struct{ X, Y float32 } `json:"Max"`
}

func (r gfxRectJSON) empty() bool {
	return r.Min.X == 0 && r.Min.Y == 0 && r.Max.X == 0 && r.Max.Y == 0
}
