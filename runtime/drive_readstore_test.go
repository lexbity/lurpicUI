package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

// readProbe is a facet whose implementation carries addressable store
// fields — the structural addressing surface.
type readProbe struct {
	facet.Facet
	layout   facet.LayoutRole
	Opacity  *store.ValueStore[float64]
	Label    *store.ValueStore[string]
	Rows     *store.CollectionStore[string]
	Computed *store.Derived[int]
	Hidden   string // not a store: store.list must skip, store.read must fail closed
}

func (p *readProbe) Base() *facet.Facet           { return &p.Facet }
func (p *readProbe) OnAttach(facet.AttachContext) {}
func (p *readProbe) OnDetach()                    {}
func (p *readProbe) OnActivate()                  {}
func (p *readProbe) OnDeactivate()                {}

func newReadRuntime(t *testing.T) (*Runtime, *readProbe) {
	t.Helper()
	probe := &readProbe{}
	probe.Facet = facet.NewFacet()
	probe.BindImpl(probe)
	probe.SetTestID("probe")
	probe.Opacity = store.NewValueStore(0.0)
	probe.Opacity.Set(0.75)
	probe.Label = store.NewValueStore("hello")
	probe.Rows = store.NewCollectionStore(func(s string) store.ItemID { return store.ItemID(len(s)) })
	probe.Rows.Insert("alpha")
	probe.Rows.Insert("beta")
	probe.Computed = store.NewDerived(func() int { return probe.Rows.Len() * 2 }, probe.Rows)
	probe.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 80, H: 24}}
	}
	probe.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		probe.layout.ArrangedBounds = bounds
	}
	probe.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	probe.AddRole(&probe.layout)

	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	rt, err := New(cfg, nil, &testWindow{width: 200, height: 100}, &backendFixture{}, probe)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	return rt, probe
}

// driveCallOn runs one drive method through the in-process sink at a cut.
func driveCallOn(t *testing.T, rt *Runtime, method string, params any, grants drive.Grants) drive.Result {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		raw = enc
	}
	cmd := drive.Command{ID: 1, Method: method, Params: raw, Grants: grants, Reply: make(chan drive.Result, 1)}
	rt.DriveSink().Enqueue(cmd)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 128; i++ {
		rt.RunOneFrameAt(base.Add(time.Duration(i) * time.Second))
		select {
		case res := <-cmd.Reply:
			return res
		default:
		}
	}
	t.Fatalf("drive %q did not reply within 128 frames", method)
	return drive.Result{}
}

func TestStoreRead_StructuralField(t *testing.T) {
	rt, _ := newReadRuntime(t)
	res := driveCallOn(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Opacity",
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("store.read: %v", res.Err)
	}
	var payload struct {
		Address string  `json:"address"`
		Value   float64 `json:"value"`
		Version uint64  `json:"version"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload %s: %v", res.Payload, err)
	}
	if payload.Value != 0.75 {
		t.Fatalf("value = %v", payload.Value)
	}
	if !strings.HasSuffix(payload.Address, "probe.Opacity") {
		t.Fatalf("address = %q", payload.Address)
	}
	if payload.Version == 0 {
		t.Fatal("value store must report a nonzero version")
	}
}

func TestStoreRead_NamedAndFailClosed(t *testing.T) {
	rt, probe := newReadRuntime(t)
	if err := rt.RegisterNamedStore("probe.label", probe.Label); err != nil {
		t.Fatalf("register: %v", err)
	}
	res := driveCallOn(t, rt, "store.read", map[string]any{"name": "probe.label"}, allGrants())
	if res.Err != nil {
		t.Fatalf("named read: %v", res.Err)
	}
	var payload struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil || payload.Value != "hello" {
		t.Fatalf("payload %s (%v)", res.Payload, err)
	}

	// Unknown name: typed unresolved error.
	if res := driveCallOn(t, rt, "store.read", map[string]any{"name": "nope"}, allGrants()); res.Err == nil || res.Err.Code != proto.ErrUnresolved {
		t.Fatalf("err = %+v", res.Err)
	}
	// Unaddressable field type: fail closed, naming the actual type.
	res = driveCallOn(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Hidden",
	}, allGrants())
	if res.Err == nil || !strings.Contains(res.Err.Detail, "Hidden") {
		t.Fatalf("err = %+v, must name the field", res.Err)
	}
	// Missing field: fail closed.
	res = driveCallOn(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Missing",
	}, allGrants())
	if res.Err == nil {
		t.Fatal("missing field must fail")
	}
}

func TestStoreList_UnionOfNamedAndStructural(t *testing.T) {
	rt, probe := newReadRuntime(t)
	if err := rt.RegisterNamedStore("probe.rows", probe.Rows); err != nil {
		t.Fatalf("register: %v", err)
	}
	res := driveCallOn(t, rt, "store.list", nil, allGrants())
	if res.Err != nil {
		t.Fatalf("store.list: %v", res.Err)
	}
	var payload struct {
		Stores []struct {
			Target string `json:"target"`
			Field  string `json:"field"`
			Kind   string `json:"kind"`
		} `json:"stores"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	kinds := map[string]string{}
	for _, s := range payload.Stores {
		if s.Field == "" {
			kinds[s.Target] = s.Kind
		} else {
			kinds[s.Target+"."+s.Field] = s.Kind
		}
	}
	// Named registry entries.
	if kinds["probe.rows"] != "collection" {
		t.Fatalf("named entry = %v", kinds["probe.rows"])
	}
	// Structural fields: value, collection, and derived shapes enumerated;
	// the non-store field skipped.
	for _, want := range []struct {
		key, kind string
	}{
		{"probe.Opacity", "value"},
		{"probe.Rows", "collection"},
		{"probe.Computed", "derived"},
	} {
		if kinds[want.key] != want.kind {
			t.Fatalf("%s = %q, want %q", want.key, kinds[want.key], want.kind)
		}
	}
	if _, ok := kinds["probe.Hidden"]; ok {
		t.Fatal("non-store field must not be enumerated")
	}
}

func TestStoreWrite_LabeledAndLogged(t *testing.T) {
	rt, probe := newReadRuntime(t)
	res := driveCallOn(t, rt, "store.write", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Opacity",
		"value":  0.25,
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("store.write: %v", res.Err)
	}
	var payload struct {
		Address               string `json:"address"`
		BypassedAppInvariants bool   `json:"bypassed_app_invariants"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if !payload.BypassedAppInvariants {
		t.Fatal("store.write response must carry bypassed_app_invariants")
	}
	if got := probe.Opacity.Get(); got != 0.25 {
		t.Fatalf("Opacity after write = %v", got)
	}
	// The write landed in the drive frame log (QP-12).
	if sink, ok := rt.DriveSink().(*driveRuntimeSink); ok {
		_ = sink
		found := false
		for _, w := range sink.recentWrites {
			if strings.HasSuffix(w, "probe.Opacity") {
				found = true
			}
		}
		if !found {
			t.Fatal("store.write must land in the frame log")
		}
	}
	// Collection writes insert by decoded item (Phase 5).
	res = driveCallOn(t, rt, "store.write", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Rows",
		"value":  map[string]any{"op": "insert", "item": "zz"},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("collection write: %v", res.Err)
	}
	if got := probe.Rows.Len(); got != 3 {
		t.Fatalf("Rows.Len after insert = %d, want 3", got)
	}
	// Wrong-typed payload: typed error, no write.
	res = driveCallOn(t, rt, "store.write", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Label",
		"value":  42,
	}, allGrants())
	if res.Err == nil {
		t.Fatal("type-mismatched write must fail")
	}
	if got := probe.Label.Get(); got != "hello" {
		t.Fatalf("Label mutated by a failed write: %q", got)
	}
}

func TestStoreWrite_GrantDenied(t *testing.T) {
	rt, probe := newReadRuntime(t)
	res := driveCallOn(t, rt, "store.write", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Opacity",
		"value":  0.9,
	}, drive.Grants{Read: true})
	if res.Err == nil || res.Err.Code != proto.ErrGrant {
		t.Fatalf("err = %+v, want grant.denied", res.Err)
	}
	if got := probe.Opacity.Get(); got != 0.75 {
		t.Fatalf("denied write executed: %v", got)
	}
}

func TestDerivedRead_PeekSemantics(t *testing.T) {
	rt, probe := newReadRuntime(t)
	if probe.Computed == nil {
		t.Fatal("probe must carry a derived store")
	}
	// One frame flushes the derived at its flush phase (phase ~7.5), so the
	// cut read observes a clean, primed value.
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	res := driveCallOn(t, rt, "derived.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Computed",
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("derived.read: %v", res.Err)
	}
	var payload struct {
		Value int  `json:"value"`
		Dirty bool `json:"dirty"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Dirty {
		t.Fatalf("primed derived must be clean: %+v", payload)
	}
	// derived.read on a value store: typed error (peek-only).
	res = driveCallOn(t, rt, "derived.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Opacity",
	}, allGrants())
	if res.Err == nil || !strings.Contains(res.Err.Detail, "not a *store.Derived") {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestFacetQuery_Shape(t *testing.T) {
	rt, _ := newReadRuntime(t)
	res := driveCallOn(t, rt, "facet.query", map[string]any{
		"target":  map[string]string{"testid": "probe"},
		"details": true,
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("facet.query: %v", res.Err)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	for _, key := range []string{"path", "testid", "facet_id", "kind", "details", "hit_declared"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("facet.query missing %q: %s", key, res.Payload)
		}
	}
	// Unresolved target: typed error.
	if res := driveCallOn(t, rt, "facet.query", map[string]any{
		"target": map[string]string{"testid": "nope"},
	}, allGrants()); res.Err == nil || res.Err.Code != proto.ErrUnresolved {
		t.Fatalf("err = %+v", res.Err)
	}
}
