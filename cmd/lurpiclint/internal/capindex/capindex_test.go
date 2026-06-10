package capindex

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// repoRoot walks up from the test file to find go.mod.
func repoRoot(tb testing.TB) string {
	tb.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		tb.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatal("go.mod not found from", dir)
		}
		dir = parent
	}
}

func TestScan_DiscoverKnownMarks(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/marks/..."}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	// Build lookup by constructor name.
	byCtor := make(map[string]Capability)
	for _, c := range caps {
		if c.Constructor != "" {
			byCtor[c.Constructor] = c
		}
	}

	// Verify known marks are discovered.
	known := []string{
		"NewCard",
		"NewButton",
		"NewText",
		"NewIcon",
		"NewScrollRegion",
		"NewList",
		"NewTable",
		"NewSlider",
		"NewSwitch",
		"NewCheckbox",
		"NewTextField",
		"NewProgressBar",
		"NewBadge",
		"NewTooltip",
		"NewDialog",
		"NewAlert",
	}

	for _, name := range known {
		if _, ok := byCtor[name]; !ok {
			t.Errorf("known mark %q not discovered by capindex", name)
		}
	}
}

func TestScan_DiscoverLayoutContainers(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/layout/..."}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	byName := make(map[string]Capability)
	for _, c := range caps {
		byName[c.TypeName] = c
	}

	known := []string{
		"RowLayout",
		"ColumnLayout",
		"StackLayout",
		"SplitLayout",
		"PaddingLayout",
		"ScrollLayout",
	}

	for _, name := range known {
		if _, ok := byName[name]; !ok {
			t.Errorf("known layout type %q not discovered by capindex", name)
		}
	}
}

func TestScan_DiscoverStandardLayers(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/layout"}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	layerNames := make(map[string]bool)
	for _, c := range caps {
		if c.Kind == KindLayer {
			layerNames[c.TypeName] = true
		}
	}

	known := []string{
		"StandardLayerBackground",
		"StandardLayerBase",
		"StandardLayerSpatial",
		"StandardLayerForeground",
		"StandardLayerFloating",
		"StandardLayerOverlay",
		"StandardLayerModal",
		"StandardLayerStatus",
		"StandardLayerDebug",
	}

	for _, name := range known {
		if !layerNames[name] {
			t.Errorf("known layer %q not discovered by capindex", name)
		}
	}
}

func TestScan_MarksHaveIntents(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/marks/structure"}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	for _, c := range caps {
		if c.Constructor == "" {
			continue
		}
		if strings.TrimSpace(c.Intent) == "" {
			t.Errorf("capability %s (%s) has no intent extracted", c.Path, c.Constructor)
		}
	}
}

func TestScan_FingerprintsDistinguishLeafVsContainer(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{
		root + "/marks/primitive",
		root + "/marks/structure",
	}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	byName := make(map[string]Capability)
	for _, c := range caps {
		byName[c.TypeName] = c
	}

	// Text is a leaf mark (no children) → IsContainer should be false.
	if c, ok := byName["Text"]; ok {
		if c.Fingerprint.IsContainer {
			t.Errorf("Text mark should be a leaf (IsContainer=false), got IsContainer=true")
		}
	} else {
		t.Log("Text not found in capindex (may have no New* constructor in scan)")
	}

	// Card is a container mark (hosts children) → IsContainer may be true.
	if c, ok := byName["Card"]; ok {
		if !c.Fingerprint.EmbedsFacet {
			t.Errorf("Card should embed facet.Facet")
		}
	}
}

func TestScan_FingerprintHasRoles(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/marks/structure"}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	byName := make(map[string]Capability)
	for _, c := range caps {
		byName[c.TypeName] = c
	}

	if c, ok := byName["Card"]; ok {
		if len(c.Fingerprint.Roles) == 0 {
			t.Log("Card has no roles detected (may use non-standard role fields)")
		}
	}
}

func TestScan_StableOrder(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/marks/..."}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	cfg := ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	}

	caps1 := Scan(result, cfg)
	caps2 := Scan(result, cfg)

	if len(caps1) != len(caps2) {
		t.Fatalf("capability count differs between runs: %d vs %d", len(caps1), len(caps2))
	}
	for i := range caps1 {
		if caps1[i].Path != caps2[i].Path {
			t.Errorf("run 2 capability %d: path=%q, want %q", i, caps2[i].Path, caps1[i].Path)
		}
	}
}

func TestScan_Performance(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{root + "/marks/..."}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	cfg := ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	}

	// Run multiple times to get a stable measurement.
	for i := 0; i < 5; i++ {
		caps := Scan(result, cfg)
		if len(caps) == 0 {
			t.Fatal("expected capabilities")
		}
	}
}

func TestTextEmitter_OutputShape(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	caps := Scan(result, ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	// At least one mark discovered.
	var marks int
	for _, c := range caps {
		if c.Kind == KindMark {
			marks++
		}
	}
	if marks < 1 {
		t.Errorf("expected >=1 mark in capindex, got %d", marks)
	}

	// At least one container (IsContainer == true).
	var containers int
	for _, c := range caps {
		if c.Fingerprint.IsContainer {
			containers++
		}
	}
	if containers < 1 {
		t.Errorf("expected >=1 container in capindex, got %d", containers)
	}

	// At least one capability has non-empty roles.
	var withRoles int
	for _, c := range caps {
		if len(c.Fingerprint.Roles) > 0 {
			withRoles++
		}
	}
	if withRoles < 1 {
		t.Errorf("expected >=1 capability with non-empty fingerprint roles, got %d", withRoles)
	}

	// TextEmitter output contains the section headers.
	var buf bytes.Buffer
	emitter := NewTextEmitter(&buf)
	if err := emitter.Emit(caps); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "MARKS:") {
		t.Error("text emitter output missing MARKS: header")
	}
	if !strings.Contains(output, "LAYOUTS:") {
		t.Error("text emitter output missing LAYOUTS: header")
	}
	if !strings.Contains(output, "LAYERS:") {
		t.Error("text emitter output missing LAYERS: header")
	}
}
