package locate

import (
	"encoding/json"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
)

var _ = marks.Descriptor{}

// locateFacet is a minimal FacetImpl carrying descriptor/accessible/keyed
// metadata for matcher tests.
type locateFacet struct {
	facet.Facet
	kind       string
	role       string
	name       string
	key        string
	hitAtPoint *gfx.Point
}

func (f *locateFacet) Base() *facet.Facet           { return &f.Facet }
func (f *locateFacet) OnAttach(facet.AttachContext) {}
func (f *locateFacet) OnDetach()                    {}
func (f *locateFacet) OnActivate()                  {}
func (f *locateFacet) OnDeactivate()                {}
func (f *locateFacet) Descriptor() marks.Descriptor { return parseKind(f.kind) }
func (f *locateFacet) AccessibilityRole() string    { return f.role }
func (f *locateFacet) AccessibleName() string       { return f.name }
func (f *locateFacet) ItemKey() string              { return f.key }
func (f *locateFacet) OnHitTest(pt gfx.Point) bool  { return f.hitAtPoint != nil }
func parseKind(k string) marks.Descriptor {
	family, typeName, _ := strings.Cut(k, "/")
	return marks.Descriptor{Family: family, TypeName: typeName}
}

// describe adapts a locateFacet to the matcher's Meta view (the runtime
// supplies the real adapter over the marks interfaces).
func describe(impl facet.FacetImpl) Meta {
	f, ok := impl.(*locateFacet)
	if !ok {
		return Meta{}
	}
	return Meta{Kind: f.kind, Role: f.role, Name: f.name, Key: f.key}
}

func newLocateFacet(testID, kind, role, name, key string) *locateFacet {
	f := &locateFacet{kind: kind, role: role, name: name, key: key}
	f.BindImpl(f)
	f.SetTestID(testID)
	return f
}

func tree() (*locateFacet, *locateFacet, *locateFacet, *locateFacet) {
	root := newLocateFacet("root", "shell/root", "region", "", "")
	save := newLocateFacet("save-btn", "action/button", "button", "Save", "")
	export := newLocateFacet("", "action/button", "button", "Export PNG", "")
	listRow := newLocateFacet("", "selection/list_item", "listitem", "Row A", "row-17")
	root.AddChild(save.Base())
	root.AddChild(export.Base())
	root.AddChild(listRow.Base())
	facet.Attach(root, facet.AttachContext{})
	return root, save, export, listRow
}

func impl(f *locateFacet) facet.FacetImpl { return f }

func TestResolve_Zero_Unresolved(t *testing.T) {
	root, _, _, _ := tree()
	_, _, err := Resolve(impl(root), Locator{TestID: "no-such"}, describe, nil)
	if err == nil || err.Code != "locator.unresolved" {
		t.Fatalf("err = %+v, want locator.unresolved", err)
	}
	_, _, err = Resolve(impl(root), Locator{Text: "Nonexistent"}, describe, nil)
	if err == nil || err.Code != "locator.unresolved" {
		t.Fatalf("err = %+v, want locator.unresolved", err)
	}
}

func TestResolve_UniqueByTestID(t *testing.T) {
	root, save, _, _ := tree()
	m, _, err := Resolve(impl(root), Locator{TestID: "save-btn"}, describe, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if m.Base.ID() != save.Base().ID() {
		t.Fatalf("resolved facet %d, want %d", m.Base.ID(), save.Base().ID())
	}
	if m.Path != "root/save-btn" {
		t.Fatalf("path = %q", m.Path)
	}
}

func TestResolve_Multiple_AmbiguousWithCandidates(t *testing.T) {
	root, _, _, _ := tree()
	// Two action/button facets exist.
	m, cands, err := Resolve(impl(root), Locator{Kind: "action/button"}, describe, nil)
	if err == nil || err.Code != "locator.ambiguous" {
		t.Fatalf("err = %+v, want locator.ambiguous", err)
	}
	_ = m
	if len(cands) != 2 {
		t.Fatalf("candidates = %d, want 2", len(cands))
	}
	for _, c := range cands {
		if c.Kind != "action/button" {
			t.Fatalf("candidate kind = %q", c.Kind)
		}
		if !strings.HasPrefix(c.FacetPath, "root/") {
			t.Fatalf("candidate path = %q", c.FacetPath)
		}
	}
}

func TestResolve_AmbiguityCappedAtTen(t *testing.T) {
	root := newLocateFacet("root", "shell/root", "", "", "")
	for i := 0; i < 14; i++ {
		child := newLocateFacet("", "action/button", "button", "", "")
		root.AddChild(child.Base())
	}
	facet.Attach(root, facet.AttachContext{})
	_, cands, err := Resolve(impl(root), Locator{Kind: "action/button"}, describe, nil)
	if err == nil || err.Code != "locator.ambiguous" {
		t.Fatalf("err = %+v", err)
	}
	if len(cands) != maxCandidates {
		t.Fatalf("candidates = %d, want capped at %d", len(cands), maxCandidates)
	}
}

func TestResolve_RoleAndName(t *testing.T) {
	root, _, export, _ := tree()
	m, _, err := Resolve(impl(root), Locator{Role: "button", Name: "Export PNG"}, describe, nil)
	if err != nil {
		t.Fatalf("resolve role+name: %v", err)
	}
	if m.Base.ID() != export.Base().ID() {
		t.Fatal("resolved the wrong button")
	}
	// Bare role with two buttons is ambiguous.
	if _, _, err := Resolve(impl(root), Locator{Role: "button"}, describe, nil); err == nil || err.Code != "locator.ambiguous" {
		t.Fatalf("err = %+v, want ambiguous", err)
	}
	// Text matches AccessibleName only, exact.
	m, _, err = Resolve(impl(root), Locator{Text: "Save"}, describe, nil)
	if err != nil {
		t.Fatalf("resolve text: %v", err)
	}
	if m.Base.TestID() != "save-btn" {
		t.Fatal("text resolved the wrong facet")
	}
	// Substring never matches.
	if _, _, err := Resolve(impl(root), Locator{Text: "Sav"}, describe, nil); err == nil || err.Code != "locator.unresolved" {
		t.Fatalf("err = %+v, substring must not match", err)
	}
}

func TestResolve_KeyExact(t *testing.T) {
	root, _, _, listRow := tree()
	m, _, err := Resolve(impl(root), Locator{Key: "row-17"}, describe, nil)
	if err != nil {
		t.Fatalf("resolve key: %v", err)
	}
	if m.Base.ID() != listRow.Base().ID() {
		t.Fatal("key resolved the wrong facet")
	}
	// Substring never matches.
	if _, _, err := Resolve(impl(root), Locator{Key: "row-1"}, describe, nil); err == nil {
		t.Fatal("key substring must not match")
	}
}

func TestResolve_CoordsUsesHitMap(t *testing.T) {
	root, save, _, _ := tree()
	hit := impl(save)
	testHit := func(pt gfx.Point) (facet.FacetImpl, bool) {
		if pt.X == 10 && pt.Y == 20 {
			return hit, true
		}
		return nil, false
	}
	m, _, err := Resolve(impl(root), Locator{Coords: &[2]float32{10, 20}}, describe, testHit)
	if err != nil {
		t.Fatalf("resolve coords: %v", err)
	}
	if m.Base.ID() != save.Base().ID() {
		t.Fatal("coords resolved the wrong facet")
	}
	// A miss is unresolved, not ambiguous — even though several facets exist.
	if _, _, err := Resolve(impl(root), Locator{Coords: &[2]float32{99, 99}}, describe, testHit); err == nil || err.Code != "locator.unresolved" {
		t.Fatalf("err = %+v", err)
	}
}

func TestResolve_NilRoot(t *testing.T) {
	var nilImpl facet.FacetImpl
	if _, _, err := Resolve(nilImpl, Locator{TestID: "x"}, describe, nil); err == nil {
		t.Fatal("nil root must be unresolved")
	}
}

func FuzzLocator(f *testing.F) {
	for _, seed := range []string{
		`{"testid":"save-btn"}`,
		`{"role":"button","name":"Save"}`,
		`{"key":"row-17"}`,
		`{"text":"Export PNG"}`,
		`{"kind":"action/button"}`,
		`{"coords":[1,2]}`,
		`null`,
		`{"coords":["x",null]}`,
	} {
		f.Add(seed)
	}
	root := newLocateFacet("root", "shell/root", "region", "", "")
	child := newLocateFacet("save-btn", "action/button", "button", "Save", "")
	root.AddChild(child.Base())
	facet.Attach(root, facet.AttachContext{})
	f.Fuzz(func(t *testing.T, raw string) {
		var locator Locator
		if err := json.Unmarshal([]byte(raw), &locator); err != nil {
			return
		}
		_, _, _ = Resolve(impl(root), locator, describe, nil)
	})
}
