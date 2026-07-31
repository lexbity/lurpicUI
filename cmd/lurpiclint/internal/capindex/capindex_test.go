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
		root + "/marks", // root marks package: marks.Core must be resolvable
		root + "/marks/primitive",
		root + "/marks/structure",
		root + "/facet",
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

	// Text is a leaf mark (hosts no children) → IsContainer must be false even
	// though it embeds marks.Core and inherits Core's layout role.
	if c, ok := byName["Text"]; ok {
		if c.Fingerprint.IsContainer {
			t.Errorf("Text mark should be a leaf (IsContainer=false), got IsContainer=true")
		}
		if !c.Fingerprint.EmbedsFacet {
			t.Errorf("Text should embed facet.Facet (transitively via marks.Core)")
		}
		if !hasRole(c.Fingerprint.Roles, "layout") {
			t.Errorf("Text roles = %v, want a layout role (promoted from marks.Core)", c.Fingerprint.Roles)
		}
	} else {
		t.Log("Text not found in capindex (may have no New* constructor in scan)")
	}

	// Card, List and Table are container marks (host children) → IsContainer
	// must be true, and their roles must include the layout role promoted from
	// marks.Core.
	for _, name := range []string{"Card", "List", "Table"} {
		c, ok := byName[name]
		if !ok {
			t.Errorf("%s not found in capindex", name)
			continue
		}
		if !c.Fingerprint.IsContainer {
			t.Errorf("%s mark should be a container (IsContainer=true), got false; roles=%v", name, c.Fingerprint.Roles)
		}
		if !c.Fingerprint.EmbedsFacet {
			t.Errorf("%s should embed facet.Facet (transitively via marks.Core)", name)
		}
		if !hasRole(c.Fingerprint.Roles, "layout") {
			t.Errorf("%s roles = %v, want a layout role (promoted from marks.Core)", name, c.Fingerprint.Roles)
		}
	}
}

// canonicalContainerMarks are the marks that MUST be classified as containers
// (IsContainer == true).  They are the canonical containers across every marks
// category; if any reports IsContainer=false the fingerprint's
// embedded-type traversal (promoted role fields from marks.Core) broke.
var canonicalContainerMarks = []string{
	"Card", "List", "Table",
	"Toolbar", "ActionGroup", "CommandPalette",
	"ButtonGroup",
	"Notification", "Dialog", "Tooltip", "Alert",
}

// TestScan_CanonicalContainerMarks guards the container-detection regression
// that previously left every Core-embedding mark classified as a non-container:
// the promoted layout/render roles from marks.Core must be counted.
func TestScan_CanonicalContainerMarks(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{
		root + "/marks",
		root + "/marks/...",
		root + "/facet",
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

	for _, name := range canonicalContainerMarks {
		c, ok := byName[name]
		if !ok {
			t.Errorf("canonical container %s not found in capindex", name)
			continue
		}
		if !c.Fingerprint.IsContainer {
			t.Errorf("%s MUST be IsContainer=true, got false (roles=%v) — embedded-type traversal broke", name, c.Fingerprint.Roles)
		}
		if !c.Fingerprint.EmbedsFacet {
			t.Errorf("%s MUST embed facet.Facet (via marks.Core), got false", name)
		}
		if !hasRole(c.Fingerprint.Roles, "layout") {
			t.Errorf("%s roles = %v, MUST contain a promoted layout role from marks.Core", name, c.Fingerprint.Roles)
		}
	}
}

func TestScan_FingerprintHasRoles(t *testing.T) {
	root := repoRoot(t)
	result, err := loader.Load([]string{
		root + "/marks",
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

	if c, ok := byName["Card"]; ok {
		if !hasRole(c.Fingerprint.Roles, "layout") {
			t.Errorf("Card roles = %v, want a layout role (promoted from marks.Core)", c.Fingerprint.Roles)
		}
		if !hasRole(c.Fingerprint.Roles, "render") {
			t.Errorf("Card roles = %v, want a render role (promoted from marks.Core)", c.Fingerprint.Roles)
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

	// Every canonical container MUST be classified as a container.  A bug that
	// collapses the container set to one accidental hit (the pre-fix state)
	// fails this named assertion.
	byName := make(map[string]Capability)
	for _, c := range caps {
		byName[c.TypeName] = c
	}
	for _, name := range canonicalContainerMarks {
		c, ok := byName[name]
		if !ok {
			t.Errorf("canonical container %s not found in capindex", name)
			continue
		}
		if !c.Fingerprint.IsContainer {
			t.Errorf("%s MUST be IsContainer=true, got false (roles=%v)", name, c.Fingerprint.Roles)
		}
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
