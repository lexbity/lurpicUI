package testkit

import (
	"encoding/json"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
)

// locateProbe is a minimal FacetImpl tree for the locator end-to-end tests:
// descriptor kind, accessibility role/name, and a keyed item, all structural
// (the canonical marks implementations are contract-tested in marks/).
type locateProbe struct {
	facet.Facet
	layout facet.LayoutRole

	kind string
	role string
	name string
	key  string
}

func (p *locateProbe) Base() *facet.Facet           { return &p.Facet }
func (p *locateProbe) OnAttach(facet.AttachContext) {}
func (p *locateProbe) OnDetach()                    {}
func (p *locateProbe) OnActivate()                  {}
func (p *locateProbe) OnDeactivate()                {}

// Descriptor returns the marks descriptor (exact signature so the injected
// drive/describe adapter recognizes it).
func (p *locateProbe) Descriptor() marks.Descriptor { return splitKind(p.kind) }

// Accessibility mirrors marks.Accessible structurally.
func (p *locateProbe) AccessibilityRole() string { return p.role }
func (p *locateProbe) AccessibleName() string    { return p.name }

// ItemKey mirrors marks.KeyedItem structurally.
func (p *locateProbe) ItemKey() string { return p.key }

func splitKind(k string) marks.Descriptor {
	family, typeName, _ := strings.Cut(k, "/")
	return marks.Descriptor{Family: family, TypeName: typeName}
}

func newLocateProbe(testID, kind, role, name, key string) *locateProbe {
	p := &locateProbe{kind: kind, role: role, name: name, key: key}
	p.Facet = facet.NewFacet()
	p.BindImpl(p)
	p.SetTestID(testID)
	p.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 80, H: 24}}
	}
	p.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.layout.ArrangedBounds = bounds
		for _, child := range p.Children() {
			if lr := child.LayoutRole(); lr != nil {
				lr.Arrange(ctx, bounds)
			}
		}
	}
	p.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	p.AddRole(&p.layout)
	return p
}

func newLocateHarness(t *testing.T) *DriveHarness {
	t.Helper()
	root := newLocateProbe("probe-root", "shell/root", "region", "", "")
	save := newLocateProbe("probe-save", "action/button", "button", "Save", "")
	export := newLocateProbe("", "action/button", "button", "Export PNG", "")
	row := newLocateProbe("probe-row", "selection/list_item", "listitem", "Row A", "row-17")
	root.AddChild(save.Base())
	root.AddChild(export.Base())
	root.AddChild(row.Base())

	cfg := StandardHarnessConfig(t, 200, 120)
	cfg.Drive = true
	h := NewHarness(t, cfg, root)
	h.RunFrame()
	return &DriveHarness{Harness: h}
}

func TestDriveLocate_ResolveByTestID(t *testing.T) {
	h := newLocateHarness(t)
	payload := h.CallOK("locate.resolve", map[string]any{
		"locator": map[string]string{"testid": "probe-save"},
	})
	var got struct {
		Path   string `json:"path"`
		TestID string `json:"testid"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload %s: %v", payload, err)
	}
	if got.TestID != "probe-save" || got.Path != "probe-root/probe-save" {
		t.Fatalf("resolve = %+v", got)
	}
}

func TestDriveLocate_UnresolvedTypedError(t *testing.T) {
	h := newLocateHarness(t)
	_, perr := h.Call("locate.resolve", map[string]any{
		"locator": map[string]string{"testid": "nope"},
	})
	if perr == nil || perr.Code != "locator.unresolved" {
		t.Fatalf("err = %+v, want locator.unresolved", perr)
	}
}

func TestDriveLocate_AmbiguousReportsCandidates(t *testing.T) {
	h := newLocateHarness(t)
	_, perr := h.Call("locate.resolve", map[string]any{
		"locator": map[string]string{"kind": "action/button"},
	})
	if perr == nil || perr.Code != "locator.ambiguous" {
		t.Fatalf("err = %+v, want locator.ambiguous", perr)
	}
	var cands []locateCandidate
	if err := json.Unmarshal([]byte(perr.Detail[strings.IndexByte(perr.Detail, '['):]), &cands); err != nil {
		t.Fatalf("detail %q: %v", perr.Detail, err)
	}
	if len(cands) != 2 {
		t.Fatalf("candidates = %d, want 2", len(cands))
	}
}

func TestDriveLocate_KeyLocator(t *testing.T) {
	h := newLocateHarness(t)
	payload := h.CallOK("locate.resolve", map[string]any{
		"locator": map[string]string{"key": "row-17"},
	})
	var got struct {
		TestID string `json:"testid"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if got.TestID != "probe-row" {
		t.Fatalf("key resolved %q, want probe-row", got.TestID)
	}
}

func TestDriveTreeDump_SubtreeWithBoundsAndKind(t *testing.T) {
	h := newLocateHarness(t)
	payload := h.CallOK("tree.dump", map[string]any{
		"from":  map[string]string{"testid": "probe-root"},
		"depth": 2,
	})
	var got struct {
		Nodes []struct {
			Path   string `json:"path"`
			Kind   string `json:"kind"`
			TestID string `json:"testid"`
			Bounds *struct {
				MinX, MinY, MaxX, MaxY float32
			} `json:"bounds"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload %s: %v", payload, err)
	}
	if len(got.Nodes) < 4 {
		t.Fatalf("nodes = %d, want >= 4 (root + 3 children)", len(got.Nodes))
	}
	byTestID := map[string]bool{}
	for _, n := range got.Nodes {
		byTestID[n.TestID] = true
		if n.Kind == "" {
			t.Fatalf("node %q has no descriptor kind", n.Path)
		}
	}
	for _, want := range []string{"probe-root", "probe-save", "probe-row"} {
		if !byTestID[want] {
			t.Fatalf("tree.dump missing TestID %q in %+v", want, got.Nodes)
		}
	}
}

func TestDriveTreeDump_BoundsPresent(t *testing.T) {
	h := newLocateHarness(t)
	payload := h.CallOK("tree.dump", map[string]any{
		"from": map[string]string{"testid": "probe-save"},
	})
	var got struct {
		Nodes []struct {
			Path   string `json:"path"`
			Bounds *struct {
				MinX, MinY float32
			} `json:"bounds"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if len(got.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(got.Nodes))
	}
	if got.Nodes[0].Bounds == nil {
		t.Fatalf("arranged facet must report bounds: %s", payload)
	}
}

type locateCandidate struct {
	FacetPath string `json:"path"`
	TestID    string `json:"testid"`
	Kind      string `json:"kind"`
}
