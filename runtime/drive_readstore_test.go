package runtime

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/projection"
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
func (p *readProbe) ReadExports() []store.ReadExport {
	return []store.ReadExport{
		store.FacetValueExport("Opacity", p.Opacity, store.ViewState, "number", 64, store.JSONFreezer[float64]),
		store.FacetCollectionExport("Rows", p.Rows, store.DomainState, "string", 4096, store.JSONFreezer[string]),
		store.FacetDerivedExport("Computed", p.Computed, store.ViewState, "integer", 64, store.JSONFreezer[int]),
		store.FacetDetailExport("label", "string", 64, p.Label.Get, store.JSONFreezer[string]),
	}
}

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
		Version string  `json:"version"`
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
	if payload.Version == "0" {
		t.Fatal("value store must report a nonzero version")
	}
}

func TestStoreRead_NamedAndFailClosed(t *testing.T) {
	rt, probe := newReadRuntime(t)
	if err := rt.RegisterReadExport(store.ValueExport("probe.label", probe.Label, store.DomainState, "string", 64, store.JSONFreezer[string])); err != nil {
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
	// Go fields are private unless the mark explicitly declares a capability.
	res = driveCall(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Hidden",
	}, allGrants())
	if resultCode(res.Err) != string(proto.ErrStateUnexported) {
		t.Fatalf("undeclared field err = %+v, want state.unexported", res.Err)
	}
	// Missing field: fail closed.
	res = driveCall(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Missing",
	}, allGrants())
	if res.Err == nil {
		t.Fatal("missing field must fail")
	}
	res = driveCall(t, rt, "store.read", map[string]any{
		"target": map[string]string{"role": "region", "name": "Probe"},
		"field":  "Opacity",
	}, allGrants())
	if resultCode(res.Err) != string(proto.ErrInvalidParams) {
		t.Fatalf("non-stable export address err=%+v, want protocol.invalid_params", res.Err)
	}
}

func TestStoreRead_CollectionSnapshotPinsVersion(t *testing.T) {
	rt, probe := newReadRuntime(t)
	first := driveCall(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Rows",
		"limit":  1,
	}, allGrants())
	if first.Err != nil {
		t.Fatalf("first collection page: %v", first.Err)
	}
	var page struct {
		Value struct {
			Items  []string `json:"items"`
			Offset string   `json:"offset"`
			Total  string   `json:"total"`
		} `json:"value"`
		Snapshot string `json:"snapshot"`
		Next     string `json:"next"`
		Version  string `json:"version"`
	}
	if err := json.Unmarshal(first.Payload, &page); err != nil {
		t.Fatalf("first page payload %s: %v", first.Payload, err)
	}
	if page.Snapshot == "" || page.Next != "1" || page.Value.Total != "2" || len(page.Value.Items) != 1 || page.Value.Items[0] != "alpha" {
		t.Fatalf("first page = %+v (%s)", page, first.Payload)
	}

	probe.Rows.Insert("gamma")
	second := driveCall(t, rt, "store.read", map[string]any{
		"snapshot": page.Snapshot,
		"offset":   page.Next,
		"limit":    1,
	}, allGrants())
	if second.Err != nil {
		t.Fatalf("continuation page: %v", second.Err)
	}
	var continuation struct {
		Value struct {
			Items  []string `json:"items"`
			Offset string   `json:"offset"`
			Total  string   `json:"total"`
		} `json:"value"`
		Version string `json:"version"`
		Next    string `json:"next"`
	}
	if err := json.Unmarshal(second.Payload, &continuation); err != nil {
		t.Fatalf("continuation payload %s: %v", second.Payload, err)
	}
	if continuation.Value.Total != "2" || len(continuation.Value.Items) != 1 || continuation.Value.Items[0] != "beta" || continuation.Version != page.Version || continuation.Next != "" {
		t.Fatalf("continuation mixed collection versions: first=%+v second=%+v payload=%s", page, continuation, second.Payload)
	}

	foreign := driveStoreRead(rt, drive.Command{Session: "another-session", Params: mustDriveJSON(t, map[string]any{"snapshot": page.Snapshot, "offset": "1", "limit": 1})})
	if resultCode(foreign.result.Err) != string(proto.ErrArtifactExpired) {
		t.Fatalf("cross-session snapshot result=%+v, want artifact.expired", foreign.result.Err)
	}
	missing := driveStoreRead(rt, drive.Command{Session: testClientFor(t, rt).sess.ID, Params: mustDriveJSON(t, map[string]any{"snapshot": "store/999", "offset": "1", "limit": 1})})
	if resultCode(missing.result.Err) != string(proto.ErrArtifactExpired) {
		t.Fatalf("missing snapshot result=%+v, want artifact.expired", missing.result.Err)
	}
	retainedBytes := rt.storeSnapshotBytes
	snapshot := rt.storeReadSnapshots[page.Snapshot]
	snapshot.expires = rt.clock.WallNow().Add(-time.Second)
	rt.storeReadSnapshots[page.Snapshot] = snapshot
	rt.cleanupDriveReadSnapshots()
	if rt.storeSnapshotBytes != 0 {
		t.Fatalf("expired snapshot cleanup retained %d bytes", rt.storeSnapshotBytes)
	}
	expired := driveStoreRead(rt, drive.Command{Session: testClientFor(t, rt).sess.ID, Params: mustDriveJSON(t, map[string]any{"snapshot": page.Snapshot, "offset": "1", "limit": 1})})
	if resultCode(expired.result.Err) != string(proto.ErrArtifactExpired) || retainedBytes == 0 {
		t.Fatalf("expired snapshot result=%+v retained=%d before=%d", expired.result.Err, rt.storeSnapshotBytes, retainedBytes)
	}
	version := probe.Rows.Version()
	rt.storeSnapshotBytes = 8 << 20
	refused := driveCall(t, rt, "store.read", map[string]any{
		"target": map[string]string{"testid": "probe"},
		"field":  "Rows",
		"limit":  1,
	}, allGrants())
	rt.storeSnapshotBytes = 0
	if resultCode(refused.Err) != string(proto.ErrResourceLimit) || probe.Rows.Version() != version {
		t.Fatalf("capacity refusal=%+v collection version before=%d after=%d", refused.Err, version, probe.Rows.Version())
	}
}

func TestExportSchemaDigestTracksDeclarationsNotLiveState(t *testing.T) {
	rt, probe := newReadRuntime(t)
	initial := rt.exportSchemaDigest()
	probe.Opacity.Set(0.9)
	probe.Rows.Insert("gamma")
	if got := rt.exportSchemaDigest(); got != initial {
		t.Fatalf("live values changed export schema digest: before=%s after=%s", initial, got)
	}
	if err := rt.RegisterReadExport(store.ValueExport("probe.label", probe.Label, store.DomainState, "string", 64, store.JSONFreezer[string])); err != nil {
		t.Fatalf("register explicit export: %v", err)
	}
	if got := rt.exportSchemaDigest(); got == initial {
		t.Fatal("adding a declared export did not change export schema digest")
	}
}

func TestStoreList_UnionOfNamedAndStructural(t *testing.T) {
	rt, probe := newReadRuntime(t)
	if err := rt.RegisterReadExport(store.CollectionExport("probe.rows", probe.Rows, store.DomainState, "string", 4096, store.JSONFreezer[string])); err != nil {
		t.Fatalf("register: %v", err)
	}
	res := driveCall(t, rt, "store.list", nil, allGrants())
	if res.Err != nil {
		t.Fatalf("store.list: %v", res.Err)
	}
	var payload struct {
		Exports []struct {
			Name   string `json:"name"`
			Target string `json:"target"`
			Field  string `json:"field"`
			Kind   string `json:"kind"`
		} `json:"exports"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	kinds := map[string]string{}
	for _, s := range payload.Exports {
		if s.Name != "" {
			kinds[s.Name] = s.Kind
			continue
		}
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
	if resultCode(res.Err) != string(proto.ErrStateWrongKind) {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestFacetQuery_Shape(t *testing.T) {
	rt, probe := newReadRuntime(t)
	path := gfx.NewPath().MoveTo(gfx.Point{X: 1, Y: 2}).LineTo(gfx.Point{X: 4, Y: 6}).Build()
	rt.projectionSystem.SetCurrentHitMap(projection.NewHitMap(projection.HitMapEntry{
		FacetID: probe.Base().ID(),
		Regions: []projection.HitRegion{{Bounds: gfx.Rect{Min: gfx.Point{X: 1, Y: 2}, Max: gfx.Point{X: 4, Y: 6}}, Shape: &path, MarkID: 17}},
	}))
	res := driveCall(t, rt, "facet.query", map[string]any{
		"target":  map[string]string{"testid": "probe"},
		"details": []string{"label"},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("facet.query: %v", res.Err)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	for _, key := range []string{"path", "testid", "facet_id", "kind", "details", "hit_declared", "hit_regions", "captured", "window"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("facet.query missing %q: %s", key, res.Payload)
		}
	}
	var hitDetails struct {
		HitRegions []struct {
			MarkID string `json:"mark_id"`
			Shape  []struct {
				Verb uint8 `json:"verb"`
			} `json:"shape"`
		} `json:"hit_regions"`
	}
	if err := json.Unmarshal(res.Payload, &hitDetails); err != nil || len(hitDetails.HitRegions) != 1 || hitDetails.HitRegions[0].MarkID != "17" || len(hitDetails.HitRegions[0].Shape) != 2 {
		t.Fatalf("facet hit geometry = %+v (%v), payload=%s", hitDetails, err, res.Payload)
	}
	// Unresolved target: typed error.
	if res := driveCall(t, rt, "facet.query", map[string]any{
		"target": map[string]string{"testid": "nope"},
	}, allGrants()); resultCode(res.Err) != string(proto.ErrUnresolved) {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestTreeDumpSnapshotIsImmutableAndSessionBound(t *testing.T) {
	rt, probe := newReadRuntime(t)
	probe.layout.ArrangedBounds = gfx.Rect{Min: gfx.Point{X: 10, Y: 12}, Max: gfx.Point{X: 40, Y: 36}}
	if probe.Base().LayoutRole() == nil {
		t.Fatal("probe layout role missing")
	}
	first := driveTreeDump(rt, drive.Command{Session: "session-a", Params: mustDriveJSON(t, map[string]any{"from": map[string]string{"testid": "probe"}, "limit": 1})})
	if first.result.Err != nil {
		t.Fatalf("first tree page: %v", first.result.Err)
	}
	var page struct {
		Snapshot string         `json:"snapshot"`
		Cut      string         `json:"cut"`
		Nodes    []treeNodeDump `json:"nodes"`
	}
	if err := json.Unmarshal(first.result.Payload, &page); err != nil || page.Snapshot == "" || len(page.Nodes) != 1 {
		t.Fatalf("first tree page = %s (%v)", first.result.Payload, err)
	}
	before := page.Nodes[0]
	probe.layout.ArrangedBounds = gfx.Rect{Min: gfx.Point{X: 50, Y: 60}, Max: gfx.Point{X: 90, Y: 84}}
	second := driveTreeDump(rt, drive.Command{Session: "session-a", Params: mustDriveJSON(t, map[string]any{"snapshot": page.Snapshot, "limit": 1})})
	if second.result.Err != nil {
		t.Fatalf("same-session tree page: %v", second.result.Err)
	}
	var replay struct {
		Snapshot string         `json:"snapshot"`
		Cut      string         `json:"cut"`
		Nodes    []treeNodeDump `json:"nodes"`
	}
	if err := json.Unmarshal(second.result.Payload, &replay); err != nil || replay.Cut != page.Cut || len(replay.Nodes) != 1 {
		t.Fatalf("replayed tree page = %s (%v)", second.result.Payload, err)
	}
	if before.Bounds == nil || replay.Nodes[0].Bounds == nil || *replay.Nodes[0].Bounds != *before.Bounds {
		t.Fatalf("snapshot changed after live layout mutation: before=%+v replay=%+v", before.Bounds, replay.Nodes[0].Bounds)
	}
	foreign := driveTreeDump(rt, drive.Command{Session: "session-b", Params: mustDriveJSON(t, map[string]any{"snapshot": page.Snapshot})})
	if resultCode(foreign.result.Err) != string(proto.ErrArtifactExpired) {
		t.Fatalf("cross-session tree snapshot err=%v, want artifact.expired", foreign.result.Err)
	}
}

func TestSnapshotCopyOnWriteCutIsolation(t *testing.T) {
	rt, probe := newReadRuntime(t)
	rt.RunOneFrame()
	first := driveTreeDump(rt, drive.Command{Session: "cow-session", Params: mustDriveJSON(t, map[string]any{"from": map[string]string{"testid": "probe"}, "limit": 1})})
	if first.result.Err != nil {
		t.Fatalf("capture tree snapshot: %v", first.result.Err)
	}
	var receipt struct {
		Snapshot string         `json:"snapshot"`
		Nodes    []treeNodeDump `json:"nodes"`
	}
	if err := json.Unmarshal(first.result.Payload, &receipt); err != nil || receipt.Snapshot == "" || len(receipt.Nodes) != 1 {
		t.Fatalf("snapshot receipt = %s (%v)", first.result.Payload, err)
	}
	snapshot := rt.treeSnapshots[receipt.Snapshot]
	if len(snapshot.pages) == 0 || snapshot.bytes < rt.driveObservation.bytes {
		t.Fatalf("snapshot did not pin observation pages: %+v", snapshot)
	}
	pinned := snapshot.pages[0].page
	if pinned.refs != 1 {
		t.Fatalf("page refs=%d, want one pinned snapshot", pinned.refs)
	}
	oldBacking := &pinned.entries[0]
	oldBounds := receipt.Nodes[0].Bounds
	probe.layout.ArrangedBounds = gfx.Rect{Min: gfx.Point{X: 71, Y: 72}, Max: gfx.Point{X: 171, Y: 132}}
	rt.invalidateDriveObservation()
	rt.RunOneFrame()
	index, ok := rt.observationAtCut()
	if !ok {
		t.Fatal("new observation generation unavailable")
	}
	if len(index.pages) == 0 || index.pages[0] == pinned {
		t.Fatal("refresh reused a page generation still pinned by a snapshot")
	}
	if &pinned.entries[0] != oldBacking || pinned.entries[0].row.Bounds == nil || oldBounds == nil || *pinned.entries[0].row.Bounds != *oldBounds {
		t.Fatal("refresh mutated the snapshot's pinned page")
	}
	current := index.entries[index.byID[probe.Base().ID()]].row.Bounds
	if current == nil || *current != probe.layout.ArrangedBounds {
		t.Fatalf("new page bounds=%+v want=%+v", current, probe.layout.ArrangedBounds)
	}
	retained := rt.treeSnapshotBytes
	rt.deleteTreeSnapshot(receipt.Snapshot)
	if pinned.refs != 0 || rt.treeSnapshotBytes != 0 || retained == 0 {
		t.Fatalf("release refs=%d bytes=%d retained_before=%d", pinned.refs, rt.treeSnapshotBytes, retained)
	}
}

func TestSnapshotIndexCapacityDoesNotRestrictApp(t *testing.T) {
	root := &facet.Facet{}
	*root = facet.NewFacet()
	root.BindImpl(root)
	for i := 0; i < 2; i++ {
		child := &facet.Facet{}
		*child = facet.NewFacet()
		child.BindImpl(child)
		root.AddChildRuntime(child)
	}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	cfg.Drive.ObservationNodeCapacity = 2
	testSessionTable(t, &cfg)
	rt, err := New(cfg, nil, &testWindow{width: 200, height: 100}, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	before := rt.frameNumber
	result := driveTreeDump(rt, drive.Command{Session: "index-capacity", Params: mustDriveJSON(t, map[string]any{"limit": 1})})
	if resultCode(result.result.Err) != string(proto.ErrResourceLimit) {
		t.Fatalf("oversized tree observation = %+v, want resource.limit", result.result.Err)
	}
	rt.RunOneFrame()
	if rt.frameNumber != before+1 {
		t.Fatalf("ordinary app frame stopped after observation capacity exhaustion: before=%d after=%d", before, rt.frameNumber)
	}
}

func TestTreeObservationIndexRefreshesAtFrameCut(t *testing.T) {
	rt, probe := newReadRuntime(t)
	rt.RunOneFrame()
	index, ok := rt.observationAtCut()
	if !ok {
		t.Fatal("initial observation index unavailable")
	}
	if index.frame != rt.drivePort.Frame() || index.cut != rt.drivePort.Cut() {
		t.Fatalf("initial index attribution cut/frame=%s/%s runtime=%s/%s", index.cut, index.frame, rt.drivePort.Cut(), rt.drivePort.Frame())
	}
	probe.layout.ArrangedBounds = gfx.Rect{Min: gfx.Point{X: 21, Y: 22}, Max: gfx.Point{X: 121, Y: 82}}
	rt.invalidateDriveObservation()
	rt.RunOneFrame()
	index, ok = rt.observationAtCut()
	if !ok {
		t.Fatal("refreshed observation index unavailable")
	}
	if index.frame != rt.drivePort.Frame() || index.cut != rt.drivePort.Cut() {
		t.Fatalf("refreshed index attribution cut/frame=%s/%s runtime=%s/%s", index.cut, index.frame, rt.drivePort.Cut(), rt.drivePort.Frame())
	}
	entry := index.entries[index.byID[probe.Base().ID()]]
	if entry.row.Bounds == nil || *entry.row.Bounds != probe.layout.ArrangedBounds {
		t.Fatalf("refreshed bounds=%+v want=%+v", entry.row.Bounds, probe.layout.ArrangedBounds)
	}
}

func TestResolveUsesCutObservationIndex(t *testing.T) {
	rt, probe := newReadRuntime(t)
	rt.RunOneFrame()
	result := driveLocateResolve(rt, drive.Command{Params: mustDriveJSON(t, map[string]any{
		"locator": map[string]string{"testid": "probe"},
	})})
	if result.result.Err != nil {
		t.Fatalf("locate.resolve: %v", result.result.Err)
	}
	var resolved struct {
		Path    string `json:"path"`
		TestID  string `json:"testid"`
		FacetID string `json:"facet_id"`
	}
	if err := json.Unmarshal(result.result.Payload, &resolved); err != nil {
		t.Fatalf("locate payload %s: %v", result.result.Payload, err)
	}
	if resolved.Path != "probe" || resolved.TestID != "probe" || resolved.FacetID != strconv.FormatUint(uint64(probe.Base().ID()), 10) {
		t.Fatalf("resolved cut identity=%+v", resolved)
	}
	rt.config.DescribeMark = func(impl facet.FacetImpl) locate.Meta {
		if impl == probe {
			return locate.Meta{Kind: "action/button", Role: "button", Name: "Rows", Key: "row-17"}
		}
		return locate.Meta{}
	}
	rt.invalidateDriveObservation()
	index, available := rt.observationAtCut()
	if !available {
		t.Fatal("updated observation index unavailable")
	}
	for _, locator := range []locate.Locator{
		{Role: "button", Name: "Rows"},
		{Key: "row-17"},
		{Kind: "action/button"},
		{Text: "Rows"},
	} {
		got, err := rt.resolveObservationLocator(index, locator)
		if err != nil || got != probe.Base().ID() {
			t.Errorf("resolve %+v = %d, %v", locator, got, err)
		}
	}
	if _, err := rt.resolveObservationLocator(index, locate.Locator{TestID: "probe", Kind: "action/button"}); err == nil || err.Code != string(proto.ErrInvalidParams) {
		t.Fatalf("mixed locator = %v, want protocol.invalid_params", err)
	}
}

func mustDriveJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
