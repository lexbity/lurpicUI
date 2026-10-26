package drive_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
)

// probeScript mirrors demos/lurpic_studio/scripts/launch_probe.json: the
// studio reference driver script (LDP Phase 9). CI runs it headless through
// the in-process sink (budget-gated); the live variant runs the same steps
// over the socket with `lurpic drive`.
type probeScript struct {
	Steps []struct {
		Op     string            `json:"op"`
		From   map[string]string `json:"from"`
		Depth  int               `json:"depth"`
		Name   string            `json:"name"`
		Target map[string]string `json:"target"`
		X      float32           `json:"x"`
		Y      float32           `json:"y"`
	} `json:"steps"`
}

// TestStudioProbe is the reference driver script executed headless (G9.3):
// hello → tree.dump → store.read → input.click → facet.query verification →
// packet hashes. It proves the studio is drivable end to end and self-reports
// its frame packet hashes.
func TestStudioProbe(t *testing.T) {
	scope :=
		// Load the committed script (a schema for the probe, reviewed like code).
		store.NewScope()
	t.Cleanup(scope.Close)

	raw, err := os.ReadFile(filepath.Join("..", "demos", "lurpic_studio", "scripts", "launch_probe.json"))
	if err != nil {
		t.Fatalf("read probe script: %v", err)
	}
	var script probeScript
	if err := json.Unmarshal(raw, &script); err != nil {
		t.Fatalf("parse probe script: %v", err)
	}
	if len(script.Steps) != 6 {
		t.Fatalf("probe steps = %d, want 6 (keep the script and the runner in sync)", len(script.Steps))
	}

	fonts := testkit.TestFontRegistry(t)
	reg, err := studio.StudioLayerRegistry()
	if err != nil {
		t.Fatalf("layer registry: %v", err)
	}
	themeCtx := studio.StudioThemeContext().WithFontRegistry(fonts)
	root := studio.BuildRoot(app.BuildContext{
		StoreScope:   scope,
		FontRegistry: fonts,
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        themeCtx,
	}, studio.NewDirtySink(10), studioSeedRows(t), reg)

	cfg := testkit.StandardHarnessConfig(t, 1280, 800)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = themeCtx.Resolver
	cfg.Drive = true
	h := testkit.NewDriveHarness(t, scope, 1280, 800, root)
	h.RunFrame()

	step := func(method string, params any) json.RawMessage {
		t.Helper()
		payload, perr := h.Call(method, params)
		if perr != nil {
			t.Fatalf("probe step %s: %v", method, perr)
		}
		return payload
	}

	// 1. hello-equivalent: sys.ping through the in-process sink.
	step("sys.ping", nil)

	// 2. tree.dump from the studio root.
	dump := step("tree.dump", map[string]any{
		"from":  script.Steps[1].From,
		"depth": script.Steps[1].Depth,
	})
	var nodes struct {
		Nodes []struct {
			Path string `json:"path"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(dump, &nodes); err != nil || len(nodes.Nodes) == 0 {
		t.Fatalf("tree.dump = %s (%v)", dump, err)
	}

	// 3. store.read e1.chartType: cut-exact, typed value present.
	read := step("store.read", map[string]any{"name": script.Steps[2].Name})
	var stored struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(read, &stored); err != nil {
		t.Fatalf("store.read payload %s: %v", read, err)
	}
	if stored.Value == "" {
		t.Fatalf("e1.chartType = %q, want a chart type", read)
	}

	// 4. input.click at the scripted point.
	step("input.click", map[string]any{"x": script.Steps[3].X, "y": script.Steps[3].Y})

	// 5. facet.query verification: the canvas is arranged, non-empty.
	query := step("facet.query", map[string]any{
		"target":  script.Steps[4].Target,
		"details": []string{"plot", "seriesCount"},
	})
	var q struct {
		Bounds *struct{ Min, Max struct{ X, Y float32 } } `json:"bounds"`
	}
	if err := json.Unmarshal(query, &q); err != nil {
		t.Fatalf("facet.query payload: %v", err)
	}
	if q.Bounds == nil || q.Bounds.Max.X <= q.Bounds.Min.X {
		t.Fatalf("probe verification failed: canvas bounds = %+v", q.Bounds)
	}

	// 6. frame.packet: self-report of the packet hash.
	packet := step("frame.packet", nil)
	var pk struct {
		Epoch string `json:"epoch"`
		Hash  string `json:"hash"`
	}
	if err := json.Unmarshal(packet, &pk); err != nil {
		t.Fatalf("frame.packet payload: %v", err)
	}
	if pk.Hash == "" || pk.Epoch == "" || pk.Hash == "0" || pk.Epoch == "0" {
		t.Fatalf("packet self-report = %+v", pk)
	}
}

// studioSeedRows loads the committed seed (the same rows main.go embeds).
func studioSeedRows(t testing.TB) []dataset.Row {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "demos", "lurpic_studio", "assets", "metrics.csv"))
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	rows, err := dataset.Parse(data)
	if err != nil {
		t.Fatalf("parse seed: %v", err)
	}
	return rows
}
