package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
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
	testSessionTable(t, &cfg)
	rt, err := New(cfg, nil, &testWindow{width: 200, height: 100}, &backendFixture{}, probe)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	return rt, probe
}

func TestStoreRead_StructuralField(t *testing.T) {
	rt, _ := newReadRuntime(t)
	res := driveCall(t, rt, "store.read", map[string]any{
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
	res := driveCall(t, rt, "store.read", map[string]any{"name": "probe.label"}, allGrants())
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
	if res := driveCall(t, rt, "store.read", map[string]any{"name": "nope"}, allGrants()); resultCode(res.Err) != string(proto.ErrUnresolved) {
		t.Fatalf("err = %+v", res.Err)
	}
	// Unaddressable field type: fail closed, naming the actual type.
	res = driveCall(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Hidden",
	}, allGrants())
	if res.Err == nil || !strings.Contains(errDetail(res.Err), "Hidden") {
		t.Fatalf("err = %+v, must name the field", res.Err)
	}
	// Missing field: fail closed.
	res = driveCall(t, rt, "store.read", map[string]any{
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
	res := driveCall(t, rt, "store.list", nil, allGrants())
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

// TestStoreWrite_Absent is the S1 negative gate: the direct-write surface
// does not exist. store.write is not a registry entry, so admission rejects
// it with protocol.unknown_method, consumes the request ID, and no store
// mutates. There is no replacement poke verb.
func TestStoreWrite_Absent(t *testing.T) {
	rt, probe := newReadRuntime(t)
	before := probe.Opacity.Get()
	res := driveCall(t, rt, "store.write", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Opacity",
		"value":  0.25,
	}, allGrants())
	if resultCode(res.Err) != string(proto.ErrUnknownMethod) {
		t.Fatalf("store.write err = %+v, want protocol.unknown_method", res.Err)
	}
	if got := probe.Opacity.Get(); got != before {
		t.Fatalf("rejected store.write mutated the store: %v → %v", before, got)
	}
	// The rejection consumed its request ID; a DIFFERENT well-typed request
	// under the same retained ID is a conflict, never a replacement.
	client := testClientFor(t, rt)
	// client.next already points at the rejected ID: retry it unchanged.
	id := client.next
	cmd := drive.Command{
		Session:    client.sess.ID,
		Generation: client.sess.Generation(),
		ID:         id,
		Method:     "store.write",
		Params:     json.RawMessage(`{"target":{"testid":"probe"},"field":"Opacity","value":0.5}`),
		Grants:     allGrants(),
		Reply:      make(chan drive.Result, 1),
	}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatalf("submit: %v", err)
	}
	rt.RunOneFrame()
	res2 := <-cmd.Reply
	if resultCode(res2.Err) != string(proto.ErrRequestConflict) {
		t.Fatalf("changed payload on a rejected id = %+v, want request.conflict", res2.Err)
	}
	if got := probe.Opacity.Get(); got != before {
		t.Fatalf("conflict path mutated the store: %v", got)
	}
}

func TestStoreWrite_NoWriteGrant(t *testing.T) {
	rt, _ := newReadRuntime(t)
	// The write grant is not part of protocol major 2: no grant set can
	// authorize a direct mutation, and parsing "write" fails startup.
	if _, err := drive.ParseGrants("read,write"); err == nil {
		t.Fatal(`ParseGrants("write") must fail`)
	}
	_ = rt
}

func TestDerivedRead_PeekSemantics(t *testing.T) {
	rt, probe := newReadRuntime(t)
	if probe.Computed == nil {
		t.Fatal("probe must carry a derived store")
	}
	// One frame flushes the derived at its flush phase (phase ~7.5), so the
	// cut read observes a clean, primed value.
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	res := driveCall(t, rt, "derived.read", map[string]any{
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
	res = driveCall(t, rt, "derived.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Opacity",
	}, allGrants())
	if res.Err == nil || !strings.Contains(errDetail(res.Err), "not a *store.Derived") {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestFacetQuery_Shape(t *testing.T) {
	rt, _ := newReadRuntime(t)
	res := driveCall(t, rt, "facet.query", map[string]any{
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
	if res := driveCall(t, rt, "facet.query", map[string]any{
		"target": map[string]string{"testid": "nope"},
	}, allGrants()); resultCode(res.Err) != string(proto.ErrUnresolved) {
		t.Fatalf("err = %+v", res.Err)
	}
}
