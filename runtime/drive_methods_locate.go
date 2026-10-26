package runtime

import (
	"encoding/json"
	"strconv"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// driveLocateError converts a typed locator error to a protocol error with
// the candidate list appended to the detail payload.
func driveLocateError(lerr *locate.Error) *proto.Error {
	code := proto.ErrorCode(lerr.Code)
	var details json.RawMessage
	if len(lerr.Candidates) > 0 || lerr.Truncated {
		if encoded, err := json.Marshal(struct {
			Candidates []locate.Candidate `json:"candidates"`
			Truncated  bool               `json:"truncated"`
		}{lerr.Candidates, lerr.Truncated}); err == nil {
			details = encoded
		}
	}
	return &proto.Error{Code: code, Detail: lerr.Detail, Details: details}
}

// locateResolveParams is the locate.resolve payload.
type locateResolveParams struct {
	Locator locate.Locator `json:"locator"`
}

// driveLocateResolve implements locate.resolve (grant read): resolves a
// locator against the live tree at this cut and reports the match.
func driveLocateResolve(rt *Runtime, cmd drive.Command) driveOutcome {
	var p locateResolveParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "locate.resolve params are malformed"})
	}
	index, available := rt.observationAtCut()
	if !available {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "tree observation capacity is exhausted"})
	}
	id, lerr := rt.resolveObservationLocator(index, p.Locator)
	if lerr != nil {
		return errorOutcome(driveLocateError(lerr))
	}
	entryIndex, exists := index.byID[id]
	if !exists {
		return errorOutcome(&proto.Error{Code: proto.ErrUnresolved, Detail: "resolved facet is no longer in the sealed observation"})
	}
	entry := index.entries[entryIndex]
	payload, err := json.Marshal(struct {
		Path    string `json:"path"`
		TestID  string `json:"testid,omitempty"`
		FacetID string `json:"facet_id"`
	}{entry.row.Path, entry.row.TestID, strconv.FormatUint(uint64(entry.id), 10)})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// driveDescribe adapts a facet implementation to the locator's metadata
// view through the injected adapter (runtime.Config.DescribeMark — the
// marks canonical interfaces are adapted in drive/describe; the runtime
// cannot import marks).
func (rt *Runtime) driveDescribe(impl facet.FacetImpl) locate.Meta {
	if rt.config.DescribeMark == nil {
		return locate.Meta{}
	}
	return rt.config.DescribeMark(impl)
}

// driveHitTest resolves screen coordinates through the current frame hit
// map (the Coords fallback, §3.4).
func (rt *Runtime) driveHitTest(pt gfx.Point) (facet.FacetImpl, bool) {
	hitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap())
	if hitMap == nil {
		return nil, false
	}
	hit := hitMap.HitTest(pt)
	if hit == nil {
		return nil, false
	}
	impl := rt.liveFacet(hit.FacetID)
	if impl == nil {
		return nil, false
	}
	return impl, true
}

// treeDumpParams is the tree.dump payload.
type treeDumpParams struct {
	From     *locate.Locator `json:"from,omitempty"`
	Depth    *int            `json:"depth,omitempty"`
	Snapshot string          `json:"snapshot,omitempty"`
	Offset   string          `json:"offset,omitempty"`
	Limit    int             `json:"limit,omitempty"`
}

// treeNodeDump is one reported node (FR-9).
type treeNodeDump struct {
	Path    string          `json:"path"`
	FacetID string          `json:"facet_id"`
	Kind    string          `json:"kind,omitempty"`
	TestID  string          `json:"testid,omitempty"`
	Role    string          `json:"role,omitempty"`
	Name    string          `json:"name,omitempty"`
	Bounds  *gfx.Rect       `json:"bounds,omitempty"`
	Layer   *treeLayerState `json:"layer,omitempty"`
}

type treeSnapshot struct {
	owner   string
	cut     string
	frame   string
	pages   []treeSnapshotPage
	count   int
	bytes   int
	expires time.Time
}

type treeSnapshotPage struct {
	page    *driveObservationPage
	indices []uint16
}

func (rt *Runtime) deleteTreeSnapshot(id string) {
	if snapshot, ok := rt.treeSnapshots[id]; ok {
		for _, page := range snapshot.pages {
			page.page.refs--
			if page.page.refs < 0 {
				panic("runtime: negative drive observation page reference count")
			}
		}
		rt.treeSnapshotBytes -= snapshot.bytes
		if rt.treeSnapshotBytes < 0 {
			panic("runtime: negative tree snapshot reservation")
		}
		delete(rt.treeSnapshots, id)
	}
}

func (rt *Runtime) cleanupTreeSnapshots() {
	if len(rt.treeSnapshots) == 0 {
		return
	}
	now := rt.clock.WallNow()
	for id, snapshot := range rt.treeSnapshots {
		if !now.Before(snapshot.expires) {
			rt.deleteTreeSnapshot(id)
		}
	}
}

type treeLayerState struct {
	Band  string `json:"band,omitempty"`
	Order string `json:"order,omitempty"`
	Mount string `json:"mount,omitempty"`
}

// driveTreeDump implements tree.dump (grant read, FR-9): descriptors, TestIDs,
// roles/names, arranged bounds, and layer state for a subtree.
func driveTreeDump(rt *Runtime, cmd drive.Command) driveOutcome {
	rt.cleanupTreeSnapshots()
	var p treeDumpParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "tree.dump params are malformed"})
	}
	maxDepth := 64
	if p.Depth != nil {
		maxDepth = *p.Depth
	}
	if maxDepth < 0 || maxDepth > 64 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "tree.dump depth must be in [0,64]"})
	}
	limit := p.Limit
	if limit == 0 {
		limit = 256
	}
	if limit < 1 || limit > 256 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "tree.dump limit must be in [1,256]"})
	}
	offset := 0
	if p.Offset != "" {
		parsed, err := strconv.Atoi(p.Offset)
		if err != nil || parsed < 0 || !decimalDigits(p.Offset) {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "tree.dump offset must be an unsigned decimal string"})
		}
		offset = parsed
	}
	if p.Snapshot != "" && (p.From != nil || p.Depth != nil) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "tree.dump snapshot continuation cannot change its from/depth filter"})
	}
	snapshot, ok := rt.treeSnapshots[p.Snapshot]
	if p.Snapshot != "" && (!ok || snapshot.owner != cmd.Session || !rt.clock.WallNow().Before(snapshot.expires)) {
		return errorOutcome(&proto.Error{Code: proto.ErrArtifactExpired, Detail: "tree snapshot expired or is not owned by this session"})
	}
	if p.Snapshot == "" {
		if rt.treeSnapshots == nil {
			rt.treeSnapshots = make(map[string]treeSnapshot)
		}
		index, available := rt.observationAtCut()
		if !available {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "tree observation capacity is exhausted"})
		}
		startID := facet.FacetID(0)
		if p.From != nil {
			m, lerr := rt.resolveObservationLocator(index, *p.From)
			if lerr != nil {
				return errorOutcome(driveLocateError(lerr))
			}
			startID = m
		}
		selectedPages := make([]treeSnapshotPage, 0, len(index.pages))
		accounted := 0
		count := 0
		for _, page := range index.pages {
			selection := treeSnapshotPage{page: page, indices: make([]uint16, 0, len(page.entries))}
			for pageIndex, node := range page.entries {
				if startID != 0 {
					contains := false
					for current := node.id; current != 0; {
						if current == startID {
							contains = true
							break
						}
						parentIndex, exists := index.byID[current]
						if !exists {
							break
						}
						current = index.entries[parentIndex].parent
					}
					if !contains {
						continue
					}
				}
				nodeDepth := node.depth
				if startID != 0 {
					for current := node.parent; current != 0 && current != startID; {
						parentIndex, exists := index.byID[current]
						if !exists {
							break
						}
						nodeDepth--
						current = index.entries[parentIndex].parent
					}
				}
				if nodeDepth > maxDepth {
					continue
				}
				selection.indices = append(selection.indices, uint16(pageIndex))
				count++
			}
			if len(selection.indices) == 0 {
				continue
			}
			// The pages are views into one contiguous index generation. Pinning
			// any page retains that generation's full immutable value backing, so
			// reserve the full generation once plus this snapshot's page directory
			// before incrementing any page reference.
			accounted += len(selection.indices)*2 + 32
			selectedPages = append(selectedPages, selection)
		}
		if count == 0 {
			// Empty filtered captures still bind to this cut without pinning pages.
			selectedPages = make([]treeSnapshotPage, 0)
		} else {
			accounted += index.bytes
		}
		if accounted > 8<<20 || rt.treeSnapshotBytes+accounted > 8<<20 {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "tree snapshot page reservation exceeds the 8 MiB retained snapshot budget"})
		}
		owned := 0
		for _, retained := range rt.treeSnapshots {
			if retained.owner == cmd.Session {
				owned++
			}
		}
		if owned >= 8 || rt.nextTreeSnapshot == ^uint64(0) {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "tree snapshot handle capacity is exhausted"})
		}
		rt.nextTreeSnapshot++
		token := "tree/" + strconv.FormatUint(rt.nextTreeSnapshot, 10)
		snapshot = treeSnapshot{owner: cmd.Session, cut: index.cut, frame: index.frame, pages: selectedPages, count: count, bytes: accounted, expires: rt.clock.WallNow().Add(60 * time.Second)}
		for _, page := range selectedPages {
			page.page.refs++
		}
		rt.treeSnapshots[token] = snapshot
		rt.treeSnapshotBytes += accounted
		p.Snapshot = token
	}
	if offset > snapshot.count {
		offset = snapshot.count
	}
	end := min(snapshot.count, offset+limit)
	page := snapshot.rows(offset, end)
	next := ""
	if end < snapshot.count {
		next = strconv.Itoa(end)
	}
	payload, err := json.Marshal(struct {
		Snapshot string         `json:"snapshot"`
		Cut      string         `json:"cut"`
		Frame    string         `json:"frame"`
		Nodes    []treeNodeDump `json:"nodes"`
		Next     string         `json:"next,omitempty"`
	}{p.Snapshot, snapshot.cut, snapshot.frame, page, next})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	if len(payload) > 256<<10 {
		return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "tree page exceeds 256 KiB; request a smaller limit"})
	}
	return payloadOutcome(payload)
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func (snapshot treeSnapshot) rows(start, end int) []treeNodeDump {
	if start < 0 || end < start || end > snapshot.count {
		panic("runtime: invalid tree snapshot row range")
	}
	rows := make([]treeNodeDump, 0, end-start)
	position := 0
	for _, page := range snapshot.pages {
		for _, index := range page.indices {
			if position >= start && position < end {
				rows = append(rows, page.page.entries[index].row)
			}
			position++
		}
	}
	if len(rows) != end-start {
		panic("runtime: tree snapshot page directory is inconsistent")
	}
	return rows
}
