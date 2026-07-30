package rules

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// --- Generator helpers -------------------------------------------------------

// tempGoFile writes a Go source file in a temporary directory and returns
// the directory path.  If subDir is non-empty, the file is created at
// dir/subDir/pkg.go to satisfy path-based package gates (isVizPackage,
// isMarksPackage, etc.).
func tempGoFile(tb testing.TB, pkgName, source string, subDir ...string) string {
	tb.Helper()
	dir := tb.TempDir()

	target := dir
	for _, s := range subDir {
		target = filepath.Join(target, s)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "pkg.go"), []byte(source), 0644); err != nil {
		tb.Fatal(err)
	}
	return target
}

// runRuleOnSource loads a Go source string, runs a single rule, and returns
// the diagnostics.  subDir is passed through to tempGoFile for path gates.
func runRuleOnSource(tb testing.TB, ruleID, source string, subDir ...string) []*diag.Diagnostic {
	tb.Helper()
	dir := tempGoFile(tb, "property", source, subDir...)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading generated source: %v", err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	return Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: []string{ruleID},
	})
}

// --- LL024 property tests ----------------------------------------------------

func TestProperty_LL024_RenamedImport(t *testing.T) {
	// LL024 must detect store.NewValueStore even when "store" is aliased.
	src := `package p

import (
	sv "codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Value *sv.ValueStore[int]
}

func NewWidget(v *sv.ValueStore[int]) *Widget {
	return &Widget{Value: sv.NewValueStore(42)}
}`
	diags := runRuleOnSource(t, "LL024", src)
	if len(diags) == 0 {
		t.Fatal("LL024: renamed store import should still fire")
	}
}

func TestProperty_LL024_NoStoreParam_NoFire(t *testing.T) {
	// No store param → no fire (TreeNavigator pattern).
	src := `package p

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Data *store.ValueStore[[]string]
}

func NewWidget(items []string) *Widget {
	return &Widget{Data: store.NewValueStore(items)}
}`
	diags := runRuleOnSource(t, "LL024", src)
	if len(diags) != 0 {
		t.Errorf("LL024: expected 0 diagnostics for no-store-param constructor, got %d", len(diags))
	}
}

func TestProperty_LL024_UseSet_NoFire(t *testing.T) {
	// Constructor takes store param but uses .Set() → no fire.
	src := `package p

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Value *store.ValueStore[int]
}

func NewWidget(v *store.ValueStore[int]) *Widget {
	w := &Widget{Value: v}
	w.Value.Set(42)
	return w
}`
	diags := runRuleOnSource(t, "LL024", src)
	if len(diags) != 0 {
		t.Errorf("LL024: expected 0 diagnostics for .Set() pattern, got %d", len(diags))
	}
}

// --- LL027 property tests ----------------------------------------------------

func TestProperty_LL027_RenamedFmtImport(t *testing.T) {
	// LL027 must detect fmt.Sprintf even when "fmt" is aliased.
	src := `package p

import (
	f "fmt"
	"codeburg.org/lexbit/lurpicui/signal"
)

type Widget struct {
	Activated signal.Signal[string]
}

func (w *Widget) handle() {
	w.Activated.Emit(f.Sprintf("zoom:%.0f", 100.0))
}`
	diags := runRuleOnSource(t, "LL027", src)
	if len(diags) == 0 {
		t.Fatal("LL027: renamed fmt import should still fire")
	}
}

func TestProperty_LL027_MultipleEmitCalls(t *testing.T) {
	// Multiple Emit calls with formatting — each must fire.
	src := `package p

import (
	"fmt"
	"codeburg.org/lexbit/lurpicui/signal"
)

type Widget struct {
	Activated signal.Signal[string]
}

func (w *Widget) handleA() {
	w.Activated.Emit(fmt.Sprintf("event:%s", "a"))
}
func (w *Widget) handleB() {
	w.Activated.Emit(fmt.Sprintf("event:%s", "b"))
}`
	diags := runRuleOnSource(t, "LL027", src)
	if len(diags) < 2 {
		t.Fatalf("LL027: expected at least 2 diagnostics for 2 Emit calls, got %d", len(diags))
	}
}

func TestProperty_LL027_NoFmt_NoFire(t *testing.T) {
	// No fmt.Sprintf in Emit → no fire.
	src := `package p

import (
	"codeburg.org/lexbit/lurpicui/signal"
)

type Action struct {
	Key string
}

type Widget struct {
	Activated signal.Signal[Action]
}

func (w *Widget) handle() {
	w.Activated.Emit(Action{Key: "toggle"})
}`
	diags := runRuleOnSource(t, "LL027", src)
	if len(diags) != 0 {
		t.Errorf("LL027: expected 0 diagnostics for typed Emit, got %d", len(diags))
	}
}

// --- LL025 property tests ----------------------------------------------------

func TestProperty_LL025_NoTrack_NoNilGuard_Fires(t *testing.T) {
	src := `package viz

import (
	"codeburg.org/lexbit/lurpicui/scale/reactive"
)

type Chart struct {
	XScale *reactive.ReactiveScale
}

func (c *Chart) OnAttach() {}`
	diags := runRuleOnSource(t, "LL025", src, "marks", "viz")
	if len(diags) == 0 {
		t.Fatal("LL025: expected at least 1 diagnostic for OnAttach without Track or nil-guard")
	}
}

func TestProperty_LL025_SubscribeInOnAttach_NoFire(t *testing.T) {
	src := `package viz

import (
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
)

type Chart struct {
	Subs   *signal.Subscriptions
	XScale *reactive.ReactiveScale
}

func (c *Chart) OnAttach() {
	signal.Track(c.Subs, &c.XScale.OnChange, func(signal.Unit) {})
}`
	diags := runRuleOnSource(t, "LL025", src, "marks", "viz")
	if len(diags) != 0 {
		t.Errorf("LL025: expected 0 diagnostics for direct Track in OnAttach, got %d", len(diags))
	}
}

// --- LL026 property tests ----------------------------------------------------

func TestProperty_LL026_VersionedCache_NoFire(t *testing.T) {
	src := `package action

type DomainItem struct {
	ID string
}

type itemCache struct {
	version uint64
	items   []DomainItem
}
`
	diags := runRuleOnSource(t, "LL026", src, "marks", "action")
	if len(diags) != 0 {
		t.Errorf("LL026: expected 0 diagnostics for versioned cache, got %d", len(diags))
	}
}

func TestProperty_LL026_NoVersion_Fires(t *testing.T) {
	src := `package action

type DomainItem struct {
	ID string
}

type itemCache struct {
	items []DomainItem
}
`
	diags := runRuleOnSource(t, "LL026", src, "marks", "action")
	if len(diags) == 0 {
		t.Fatal("LL026: expected at least 1 diagnostic for unversioned cache with domain echo")
	}
}

func TestProperty_LL026_ViewMetrics_NoFire(t *testing.T) {
	src := `package action

type metricsCache struct {
	cachedW float32
	cachedH float32
}
`
	diags := runRuleOnSource(t, "LL026", src, "marks", "action")
	if len(diags) != 0 {
		t.Errorf("LL026: expected 0 diagnostics for view-metrics-only cache, got %d", len(diags))
	}
}

// --- LL028 property tests ----------------------------------------------------

func TestProperty_LL028_NoGfxColor_NoFire(t *testing.T) {
	src := `package viz

type Chart struct{}
`
	diags := runRuleOnSource(t, "LL028", src, "marks", "viz")
	if len(diags) != 0 {
		t.Errorf("LL028: expected 0 diagnostics for file without gfx.Color, got %d", len(diags))
	}
}
