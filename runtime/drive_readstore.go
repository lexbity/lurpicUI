package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"sort"
	"strconv"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/readstore"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/canonical"
	"codeburg.org/lexbit/lurpicui/store"
)

func (rt *Runtime) initNamedRegistry() error {
	reg, err := readstore.NewRegistry(rt.config.ReadExports)
	if err != nil {
		return err
	}
	rt.namedStores = reg
	return nil
}

type storeTarget struct {
	Name     string          `json:"name,omitempty"`
	Target   *locate.Locator `json:"target,omitempty"`
	Field    string          `json:"field,omitempty"`
	Snapshot string          `json:"snapshot,omitempty"`
	Offset   string          `json:"offset,omitempty"`
	Limit    int             `json:"limit,omitempty"`
}

type storeReadSnapshot struct {
	owner   string
	address string
	version uint64
	items   []json.RawMessage
	bytes   int
	expires time.Time
}

func (t storeTarget) page() (int, int, *proto.Error) {
	limit := t.Limit
	if limit == 0 {
		limit = 256
	}
	if limit < 1 || limit > 256 {
		return 0, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "page limit must be in [1,256]"}
	}
	if t.Offset == "" {
		return 0, limit, nil
	}
	offset, err := strconv.ParseUint(t.Offset, 10, 32)
	if err != nil {
		return 0, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "offset must be an unsigned decimal string"}
	}
	return int(offset), limit, nil
}

func (t storeTarget) resolve(rt *Runtime) (store.ReadExport, string, *proto.Error) {
	if (t.Name != "") == (t.Target != nil || t.Field != "") {
		return store.ReadExport{}, "", &proto.Error{Code: proto.ErrInvalidParams, Detail: "store address requires exactly name or target plus field"}
	}
	if t.Name != "" {
		if t.Target != nil || t.Field != "" {
			return store.ReadExport{}, "", &proto.Error{Code: proto.ErrInvalidParams, Detail: "store address forms are mutually exclusive"}
		}
		export, ok := rt.namedStores.Lookup(t.Name)
		if !ok {
			return store.ReadExport{}, "", &proto.Error{Code: proto.ErrUnresolved, Detail: fmt.Sprintf("no named read export %q", t.Name)}
		}
		return export, t.Name, nil
	}
	if t.Target == nil || t.Field == "" {
		return store.ReadExport{}, "", &proto.Error{Code: proto.ErrInvalidParams, Detail: "facet export address requires target and field"}
	}
	if !t.Target.StableAddress() {
		return store.ReadExport{}, "", &proto.Error{Code: proto.ErrInvalidParams, Detail: "facet exports require a testid or scoped key address"}
	}
	index, available := rt.observationAtCut()
	if !available {
		return store.ReadExport{}, "", &proto.Error{Code: proto.ErrResourceLimit, Detail: "read-export observation index is exhausted"}
	}
	match, lerr := rt.resolveObservationLocator(index, *t.Target)
	if lerr != nil {
		return store.ReadExport{}, "", driveLocateError(lerr)
	}
	entryIndex, exists := index.byID[match]
	if !exists {
		return store.ReadExport{}, "", &proto.Error{Code: proto.ErrUnresolved, Detail: "facet is not present in the sealed observation"}
	}
	export, ok := rt.readExportFor(match, t.Field)
	if !ok {
		return store.ReadExport{}, "", &proto.Error{Code: proto.ErrStateUnexported, Detail: "facet has no declared read export " + t.Field}
	}
	return export, index.entries[entryIndex].row.Path + "." + t.Field, nil
}

type exportListParams struct {
	Target *locate.Locator `json:"target,omitempty"`
	Offset string          `json:"offset,omitempty"`
	Limit  int             `json:"limit,omitempty"`
}

type exportDescription struct {
	Name     string `json:"name,omitempty"`
	Target   string `json:"target,omitempty"`
	Field    string `json:"field,omitempty"`
	Class    string `json:"class"`
	Kind     string `json:"kind"`
	Schema   string `json:"schema"`
	Version  string `json:"version"`
	MaxBytes int    `json:"max_bytes"`
}

func driveStoreList(rt *Runtime, cmd drive.Command) driveOutcome {
	var p exportListParams
	if len(cmd.Params) != 0 {
		if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "store.list params are malformed"})
		}
	}
	offset, limit, perr := (storeTarget{Offset: p.Offset, Limit: p.Limit}).page()
	if perr != nil {
		return errorOutcome(perr)
	}
	index, available := rt.observationAtCut()
	if !available {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "read-export observation index is exhausted"})
	}
	owner := facet.FacetID(0)
	if p.Target != nil {
		if !p.Target.StableAddress() {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "store.list target requires a testid or scoped key address"})
		}
		match, lerr := rt.resolveObservationLocator(index, *p.Target)
		if lerr != nil {
			return errorOutcome(driveLocateError(lerr))
		}
		owner = match
	}
	entries := make([]exportDescription, 0, len(rt.namedStores.Names())+len(rt.facetReadExports)*2)
	if owner == 0 {
		for _, name := range rt.namedStores.Names() {
			export, _ := rt.namedStores.Lookup(name)
			info := export.Info()
			entries = append(entries, exportDescription{Name: name, Class: info.Class.String(), Kind: info.Kind.String(), Schema: info.Schema, Version: strconv.FormatUint(uint64(export.Version()), 10), MaxBytes: info.MaxBytes})
		}
	}
	paths := index.byID
	for id, exports := range rt.facetReadExports {
		indexAt, exists := paths[id]
		if !exists || (owner != 0 && owner != id) {
			continue
		}
		path := index.entries[indexAt].row.Path
		for field, export := range exports {
			info := export.Info()
			entries = append(entries, exportDescription{Target: path, Field: field, Class: info.Class.String(), Kind: info.Kind.String(), Schema: info.Schema, Version: strconv.FormatUint(uint64(export.Version()), 10), MaxBytes: info.MaxBytes})
		}
	}
	sortExportDescriptions(entries)
	if offset > len(entries) {
		offset = len(entries)
	}
	end := min(len(entries), offset+limit)
	next := ""
	if end < len(entries) {
		next = strconv.Itoa(end)
	}
	payload, err := json.Marshal(struct {
		Exports []exportDescription `json:"exports"`
		Next    string              `json:"next,omitempty"`
	}{entries[offset:end], next})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "export catalog encoding failed"})
	}
	return payloadOutcome(payload)
}

func sortExportDescriptions(entries []exportDescription) {
	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Target != right.Target {
			return left.Target < right.Target
		}
		return left.Field < right.Field
	})
}

func (rt *Runtime) visibleChildren(impl facet.FacetImpl) []facet.FacetImpl {
	if impl == nil || impl.Base() == nil {
		return nil
	}
	base := impl.Base()
	children := make([]facet.FacetImpl, 0, len(base.Children())+4)
	seen := make(map[facet.FacetID]struct{}, len(base.Children()))
	for _, child := range base.Children() {
		if child == nil || child.State() == facet.StateDisposed {
			continue
		}
		childImpl := child.Impl()
		if childImpl == nil {
			childImpl = child
		}
		children = append(children, childImpl)
		seen[child.ID()] = struct{}{}
	}
	if rt.config.HostedChildren != nil {
		for _, child := range rt.config.HostedChildren(impl) {
			if child == nil || child.Base() == nil || child.Base().State() == facet.StateDisposed {
				continue
			}
			if _, exists := seen[child.Base().ID()]; exists {
				continue
			}
			seen[child.Base().ID()] = struct{}{}
			children = append(children, child)
		}
	}
	return children
}

func driveStoreRead(rt *Runtime, cmd drive.Command) driveOutcome {
	var p storeTarget
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "store.read params are malformed"})
	}
	offset, limit, perr := p.page()
	if perr != nil {
		return errorOutcome(perr)
	}
	if p.Snapshot != "" {
		if p.Name != "" || p.Target != nil || p.Field != "" {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "store snapshot continuation cannot change its address"})
		}
		snapshot, ok := rt.storeReadSnapshots[p.Snapshot]
		if !ok || snapshot.owner != cmd.Session || !rt.clock.WallNow().Before(snapshot.expires) {
			if ok && !rt.clock.WallNow().Before(snapshot.expires) {
				rt.deleteStoreReadSnapshot(p.Snapshot)
			}
			return errorOutcome(&proto.Error{Code: proto.ErrArtifactExpired, Detail: "store collection snapshot expired or is not owned by this session"})
		}
		return storeSnapshotPage(snapshot, p.Snapshot, offset, limit)
	}
	if offset != 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "collection continuation requires its snapshot handle"})
	}
	export, address, perr := p.resolve(rt)
	if perr != nil {
		return errorOutcome(perr)
	}
	if export.Info().Kind == store.DerivedExportKind || export.Info().Kind == store.DetailExportKind {
		return errorOutcome(&proto.Error{Code: proto.ErrStateWrongKind, Detail: "use derived.read or facet.query for this export kind"})
	}
	if export.Info().Kind != store.CollectionExportKind {
		value, version, total, _, err := export.Capture(0, limit)
		if err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: err.Error()})
		}
		payload, err := json.Marshal(struct {
			Address string          `json:"address"`
			Value   json.RawMessage `json:"value"`
			Version string          `json:"version"`
			Total   string          `json:"total,omitempty"`
		}{address, value, strconv.FormatUint(version, 10), strconv.FormatUint(total, 10)})
		if err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "read export encoding failed"})
		}
		if len(payload) > 256<<10 {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "read export result exceeds 256 KiB; declare a paged collection export"})
		}
		return payloadOutcome(payload)
	}
	count, err := export.CollectionLength()
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrStateWrongKind, Detail: err.Error()})
	}
	if count > 16384 {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "collection snapshot exceeds the 16384 item bound"})
	}
	reservation := export.Info().MaxBytes + count*24 + 128
	if perr := rt.reserveStoreReadSnapshot(cmd.Session, reservation); perr != nil {
		return errorOutcome(perr)
	}
	items, version, err := export.CaptureCollection()
	if err != nil {
		rt.releaseStoreReadSnapshotReservation(reservation)
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: err.Error()})
	}
	contentBytes := 2
	for i, item := range items {
		contentBytes += len(item)
		if i > 0 {
			contentBytes++
		}
	}
	actual := contentBytes + len(items)*24 + 128
	rt.releaseStoreReadSnapshotReservation(reservation)
	if contentBytes > export.Info().MaxBytes {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "collection snapshot exceeds its declared byte bound"})
	}
	if len(items) <= limit {
		payload, err := encodeStoreSnapshotPage(address, version, items, 0, limit, "")
		if err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "collection page encoding exceeded its declared bound"})
		}
		return payloadOutcome(payload)
	}
	token, err := rt.retainStoreReadSnapshot(cmd.Session, address, version, items, actual)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: err.Error()})
	}
	payload, err := encodeStoreSnapshotPage(address, version, items, 0, limit, token)
	if err != nil {
		rt.deleteStoreReadSnapshot(token)
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "collection page encoding exceeded its declared bound"})
	}
	return payloadOutcome(payload)
}

func (rt *Runtime) cleanupStoreReadSnapshots() {
	if len(rt.storeReadSnapshots) == 0 {
		return
	}
	now := rt.clock.WallNow()
	for id, snapshot := range rt.storeReadSnapshots {
		if !now.Before(snapshot.expires) {
			rt.deleteStoreReadSnapshot(id)
		}
	}
}

func (rt *Runtime) cleanupDriveReadSnapshots() {
	rt.cleanupStoreReadSnapshots()
	rt.cleanupTreeSnapshots()
}

func (rt *Runtime) deleteStoreReadSnapshot(id string) {
	if snapshot, ok := rt.storeReadSnapshots[id]; ok {
		rt.storeSnapshotBytes -= snapshot.bytes
		delete(rt.storeReadSnapshots, id)
	}
}

func (rt *Runtime) reserveStoreReadSnapshot(session string, bytes int) *proto.Error {
	rt.cleanupStoreReadSnapshots()
	if bytes < 1 || bytes > 8<<20 || rt.storeSnapshotBytes+rt.storeSnapshotReserved+bytes > 8<<20 {
		return &proto.Error{Code: proto.ErrResourceLimit, Detail: "immutable export snapshot reservation exceeds the 8 MiB application budget"}
	}
	owned := 0
	for _, snapshot := range rt.storeReadSnapshots {
		if snapshot.owner == session {
			owned++
		}
	}
	if owned >= 8 || rt.nextStoreSnapshot == ^uint64(0) {
		return &proto.Error{Code: proto.ErrResourceLimit, Detail: "collection snapshot handle capacity is exhausted"}
	}
	rt.storeSnapshotReserved += bytes
	return nil
}

func (rt *Runtime) releaseStoreReadSnapshotReservation(bytes int) {
	rt.storeSnapshotReserved -= bytes
	if rt.storeSnapshotReserved < 0 {
		panic("runtime: negative collection snapshot reservation")
	}
}

func (rt *Runtime) retainStoreReadSnapshot(session, address string, version uint64, items []json.RawMessage, bytes int) (string, error) {
	if rt.storeReadSnapshots == nil {
		rt.storeReadSnapshots = make(map[string]storeReadSnapshot)
	}
	if rt.storeSnapshotBytes+bytes > 8<<20 || rt.nextStoreSnapshot == ^uint64(0) {
		return "", fmt.Errorf("immutable export snapshots exceed the 8 MiB application budget")
	}
	rt.nextStoreSnapshot++
	token := "store/" + strconv.FormatUint(rt.nextStoreSnapshot, 10)
	owned := 0
	for _, retained := range rt.storeReadSnapshots {
		if retained.owner == session {
			owned++
		}
	}
	if owned >= 8 {
		return "", fmt.Errorf("collection snapshot handle capacity is exhausted")
	}
	copyItems := make([]json.RawMessage, len(items))
	for i, item := range items {
		copyItems[i] = append(json.RawMessage(nil), item...)
	}
	rt.storeReadSnapshots[token] = storeReadSnapshot{owner: session, address: address, version: version, items: copyItems, bytes: bytes, expires: rt.clock.WallNow().Add(60 * time.Second)}
	rt.storeSnapshotBytes += bytes
	return token, nil
}

func storeSnapshotPage(snapshot storeReadSnapshot, token string, offset, limit int) driveOutcome {
	if offset > len(snapshot.items) {
		offset = len(snapshot.items)
	}
	payload, err := encodeStoreSnapshotPage(snapshot.address, snapshot.version, snapshot.items, offset, limit, token)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "collection page encoding exceeded its declared bound"})
	}
	return payloadOutcome(payload)
}

func encodeStoreSnapshotPage(address string, version uint64, items []json.RawMessage, offset, limit int, token string) ([]byte, error) {
	end := min(len(items), offset+limit)
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	value, err := json.Marshal(struct {
		Items  []json.RawMessage `json:"items"`
		Offset string            `json:"offset"`
		Total  string            `json:"total"`
	}{items[offset:end], strconv.Itoa(offset), strconv.Itoa(len(items))})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(struct {
		Address  string          `json:"address"`
		Value    json.RawMessage `json:"value"`
		Version  string          `json:"version"`
		Total    string          `json:"total"`
		Offset   string          `json:"offset"`
		Snapshot string          `json:"snapshot,omitempty"`
		Next     string          `json:"next,omitempty"`
	}{address, value, strconv.FormatUint(version, 10), strconv.Itoa(len(items)), strconv.Itoa(offset), token, next})
	if err == nil && len(payload) > 256<<10 {
		return nil, fmt.Errorf("collection page exceeds 256 KiB")
	}
	return payload, err
}

func driveDerivedRead(rt *Runtime, cmd drive.Command) driveOutcome {
	var p storeTarget
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "derived.read params are malformed"})
	}
	export, address, perr := p.resolve(rt)
	if perr != nil {
		return errorOutcome(perr)
	}
	if export.Info().Kind != store.DerivedExportKind {
		return errorOutcome(&proto.Error{Code: proto.ErrStateWrongKind, Detail: "derived.read requires a declared derived export"})
	}
	value, version, _, dirty, err := export.Capture(0, 1)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: err.Error()})
	}
	payload, err := json.Marshal(struct {
		Address string          `json:"address"`
		Value   json.RawMessage `json:"value"`
		Dirty   bool            `json:"dirty"`
		Version string          `json:"version"`
	}{address, value, dirty, strconv.FormatUint(version, 10)})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "derived export encoding failed"})
	}
	return payloadOutcome(payload)
}

type facetQueryParams struct {
	Target  locate.Locator `json:"target"`
	Details []string       `json:"details,omitempty"`
}

type facetQueryResult struct {
	Path        string                     `json:"path"`
	TestID      string                     `json:"testid,omitempty"`
	FacetID     string                     `json:"facet_id"`
	Kind        string                     `json:"kind"`
	Role        string                     `json:"role"`
	Name        string                     `json:"name"`
	Bounds      *gfx.Rect                  `json:"bounds,omitempty"`
	Measured    *gfx.Size                  `json:"measured,omitempty"`
	Focused     bool                       `json:"focused"`
	Captured    bool                       `json:"captured"`
	HitDeclared bool                       `json:"hit_declared"`
	HitRegions  []facetHitRegion           `json:"hit_regions"`
	Window      string                     `json:"window"`
	Layer       *treeLayerState            `json:"layer,omitempty"`
	Details     map[string]json.RawMessage `json:"details,omitempty"`
}

type facetHitRegion struct {
	Bounds      gfx.Rect         `json:"bounds"`
	MarkID      string           `json:"mark_id"`
	LayerID     string           `json:"layer_id"`
	Cursor      string           `json:"cursor"`
	PassThrough bool             `json:"pass_through"`
	ZOrder      string           `json:"z_order"`
	Shape       []hitPathSegment `json:"shape,omitempty"`
}

type hitPathSegment struct {
	Verb   uint8        `json:"verb"`
	Points [3]gfx.Point `json:"points"`
}

func driveFacetQuery(rt *Runtime, cmd drive.Command) driveOutcome {
	var p facetQueryParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "facet.query params are malformed"})
	}
	m, _, lerr := locate.Resolve(rt.root, p.Target, rt.driveDescribe, rt.driveHitTest, rt.visibleChildren)
	if lerr != nil {
		return errorOutcome(driveLocateError(lerr))
	}
	result := facetQueryResult{Path: m.Path, TestID: m.Base.TestID(), FacetID: strconv.FormatUint(uint64(m.Base.ID()), 10)}
	meta := rt.driveDescribe(m.Impl)
	result.Kind, result.Role, result.Name = meta.Kind, meta.Role, meta.Name
	if lr := m.Base.LayoutRole(); lr != nil {
		if !lr.ArrangedBounds.IsEmpty() {
			bounds := lr.ArrangedBounds
			result.Bounds = &bounds
		}
		measured := lr.MeasuredSize
		result.Measured = &measured
	}
	if rt.focusManager != nil {
		if focused := rt.focusManager.FocusedImpl(); focused != nil && focused.Base() != nil {
			result.Focused = focused.Base().ID() == m.Base.ID()
		}
	}
	result.Captured = rt.inputSystem.CapturesFacet(m.Base.ID())
	if hr := m.Base.HitRole(); hr != nil {
		result.HitDeclared = hr.OnHitTest != nil
	}
	binding, _ := rt.resolveWindowBindingForFacet(m.Base.ID())
	result.Window = windowBindingString(binding)
	if hitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap()); hitMap != nil {
		for _, entry := range hitMap.Entries() {
			if entry.FacetID != m.Base.ID() {
				continue
			}
			for _, region := range entry.Regions {
				if len(result.HitRegions) >= 128 {
					return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "facet hit-region count exceeds 128"})
				}
				captured := facetHitRegion{
					Bounds: region.Bounds, MarkID: strconv.FormatUint(uint64(region.MarkID), 10),
					LayerID: strconv.FormatUint(uint64(entry.LayerID), 10), Cursor: strconv.FormatUint(uint64(region.Cursor), 10),
					PassThrough: region.PassThrough, ZOrder: strconv.FormatInt(int64(region.ZOrder), 10),
				}
				if region.Shape != nil {
					if len(region.Shape.Segments) > 1024 {
						return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "facet hit-region path exceeds 1024 segments"})
					}
					captured.Shape = make([]hitPathSegment, len(region.Shape.Segments))
					for i, segment := range region.Shape.Segments {
						captured.Shape[i] = hitPathSegment{Verb: uint8(segment.Verb), Points: segment.Pts}
					}
				}
				result.HitRegions = append(result.HitRegions, captured)
			}
		}
	}
	if m.Base.IsLayer() {
		att := m.Base.LayerAttachment()
		layer := &treeLayerState{Band: att.Band.String(), Order: strconv.FormatInt(int64(att.Order), 10)}
		if att.Mount != nil {
			if att.Mount.Get() {
				layer.Mount = "mounted"
			} else {
				layer.Mount = "unmounted"
			}
		}
		result.Layer = layer
	}
	if len(p.Details) > 0 {
		result.Details = make(map[string]json.RawMessage, len(p.Details))
		for _, name := range p.Details {
			if _, exists := result.Details[name]; exists {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "duplicate facet detail name"})
			}
			export, ok := rt.readExportFor(m.Base.ID(), name)
			if !ok || export.Info().Kind != store.DetailExportKind {
				return errorOutcome(&proto.Error{Code: proto.ErrStateUnexported, Detail: "facet detail is not declared: " + name})
			}
			value, _, _, _, err := export.Capture(0, 1)
			if err != nil {
				return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: err.Error()})
			}
			result.Details[name] = value
		}
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "facet query encoding failed"})
	}
	if len(payload) > 256<<10 {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "facet query exceeds 256 KiB"})
	}
	return payloadOutcome(payload)
}

// driveFramePixels retains the pre-artifact software capture path until S7.
func driveFramePixels(rt *Runtime, _ drive.Command) driveOutcome {
	surface := rt.softwareSurface()
	if surface == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "software surface unavailable"})
	}
	if err := surface.Lock(); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "surface lock failed"})
	}
	w, h := surface.Size()
	img := imageFromBuffer(surface.Buffer(), surface.Stride(), w, h)
	_ = surface.Unlock(nil)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderFailed, Detail: "png encoding failed"})
	}
	payload, err := json.Marshal(struct {
		PNG    string `json:"png"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}{pngDataURL(out.Bytes()), w, h})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "pixel capture encoding failed"})
	}
	return payloadOutcome(payload)
}

func driveFramePacket(rt *Runtime, _ drive.Command) driveOutcome {
	frame := rt.LastPrimaryFrame()
	if frame == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "no frame submitted yet"})
	}
	digest, err := canonical.Digest(frame)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderFailed, Detail: "canonical frame encoding failed"})
	}
	payload, err := json.Marshal(struct {
		Epoch   string `json:"epoch"`
		Hash    string `json:"hash"`
		Batches int    `json:"batches"`
	}{strconv.FormatUint(rt.drivePort.Epoch(), 10), digest, len(frame.RenderBatchs)})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "frame packet encoding failed"})
	}
	return payloadOutcome(payload)
}

func (rt *Runtime) softwareSurface() render.SoftwareSurface {
	if rt.window == nil {
		return nil
	}
	if surface, ok := rt.window.Surface().(render.SoftwareSurface); ok {
		return surface
	}
	return nil
}

func imageFromBuffer(buf []byte, stride, w, h int) image.Image {
	return &image.RGBA{Pix: buf, Stride: stride, Rect: image.Rect(0, 0, w, h)}
}

func pngDataURL(pngBytes []byte) string {
	const prefix = "data:image/png;base64,"
	const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb bytes.Buffer
	sb.WriteString(prefix)
	for i := 0; i < len(pngBytes); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], pngBytes[i:])
		b := uint32(chunk[0])<<16 | uint32(chunk[1])<<8 | uint32(chunk[2])
		sb.WriteByte(b64[(b>>18)&0x3F])
		sb.WriteByte(b64[(b>>12)&0x3F])
		if n > 1 {
			sb.WriteByte(b64[(b>>6)&0x3F])
		} else {
			sb.WriteByte('=')
		}
		if n > 2 {
			sb.WriteByte(b64[b&0x3F])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}
