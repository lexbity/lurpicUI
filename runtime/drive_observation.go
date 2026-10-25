package runtime

import (
	"strconv"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

const (
	driveObservationNodeLimit = 16384
	driveObservationByteLimit = 8 << 20
	driveObservationPageSize  = 256
	driveObservationEntrySize = int(unsafe.Sizeof(driveObservationEntry{}))
)

// driveObservationEntry is a value-only index record. It intentionally keeps
// no facet, role, store, or projection pointers, so pages can be read after a
// later frame without observing mutable engine state.
type driveObservationEntry struct {
	id     facet.FacetID
	parent facet.FacetID
	depth  int
	meta   locate.Meta
	row    treeNodeDump
}

// driveObservationPage is an immutable view over one contiguous generation of
// index entries. Snapshot references pin the backing array; a refresh detects
// those references and builds the next generation in a separate array.
type driveObservationPage struct {
	entries []driveObservationEntry
	refs    int
}

// driveObservationIndex is the current sealed runtime observation. It is
// materialized lazily when structural reads are first used, then refreshed at
// frame seals. Paged tree snapshots pin selected immutable pages and remain
// session-owned independently of subsequent index refreshes.
type driveObservationIndex struct {
	entries   []driveObservationEntry
	pages     []*driveObservationPage
	byID      map[facet.FacetID]int
	cut       string
	frame     string
	bytes     int
	dirty     bool
	exhausted bool
}

func (rt *Runtime) invalidateDriveObservation() {
	if rt.driveObservation != nil {
		rt.driveObservation.dirty = true
	}
}

func (rt *Runtime) observationAtCut() (*driveObservationIndex, bool) {
	index := rt.driveObservation
	if index == nil {
		index = &driveObservationIndex{
			entries: make([]driveObservationEntry, 0, rt.config.Drive.ObservationNodeCapacity),
			byID:    make(map[facet.FacetID]int, rt.config.Drive.ObservationNodeCapacity),
			dirty:   true,
		}
		rt.driveObservation = index
	}
	if index.dirty {
		rt.refreshDriveObservation(index)
	}
	if index.exhausted {
		return index, false
	}
	return index, true
}

func (rt *Runtime) refreshDriveObservation(index *driveObservationIndex) {
	pinned := false
	for _, page := range index.pages {
		if page.refs != 0 {
			pinned = true
			break
		}
	}
	if pinned {
		index.entries = make([]driveObservationEntry, 0, rt.config.Drive.ObservationNodeCapacity)
		index.byID = make(map[facet.FacetID]int, rt.config.Drive.ObservationNodeCapacity)
	} else {
		index.entries = index.entries[:0]
		clear(index.byID)
	}
	index.pages = nil
	index.bytes = cap(index.entries) * driveObservationEntrySize
	index.exhausted = false
	if index.bytes > rt.config.Drive.ObservationByteCapacity {
		index.exhausted = true
		index.dirty = false
		index.cut, index.frame = rt.observationCutAndFrame()
		return
	}
	nodes, exceeded := locate.VisibleTreeBounded(rt.root, rt.driveDescribe, rt.visibleChildren, rt.config.Drive.ObservationNodeCapacity)
	if exceeded {
		index.exhausted = true
		index.dirty = false
		index.cut, index.frame = rt.observationCutAndFrame()
		return
	}
	for _, node := range nodes {
		row := treeNodeDump{
			Path:    node.Path,
			TestID:  node.Base.TestID(),
			FacetID: strconv.FormatUint(uint64(node.Base.ID()), 10),
			Kind:    node.Meta.Kind,
			Role:    node.Meta.Role,
			Name:    node.Meta.Name,
		}
		if layoutRole := node.Base.LayoutRole(); layoutRole != nil {
			bounds := layoutRole.ArrangedBounds
			if !bounds.IsEmpty() {
				row.Bounds = &bounds
			}
		}
		if node.Base.IsLayer() {
			attachment := node.Base.LayerAttachment()
			state := treeLayerState{Band: attachment.Band.String(), Order: strconv.FormatInt(int64(attachment.Order), 10)}
			if attachment.Mount != nil {
				if attachment.Mount.Get() {
					state.Mount = "mounted"
				} else {
					state.Mount = "unmounted"
				}
			}
			row.Layer = &state
		}
		entry := driveObservationEntry{id: node.Base.ID(), parent: node.ParentID, depth: node.Depth, meta: node.Meta, row: row}
		index.bytes += 128 + len(row.Path) + len(row.TestID) + len(row.Kind) + len(row.Role) + len(row.Name)
		if row.Bounds != nil {
			index.bytes += 32
		}
		if row.Layer != nil {
			index.bytes += len(row.Layer.Band) + len(row.Layer.Order) + len(row.Layer.Mount)
		}
		if index.bytes > rt.config.Drive.ObservationByteCapacity {
			index.entries = index.entries[:0]
			clear(index.byID)
			index.exhausted = true
			index.dirty = false
			index.cut, index.frame = rt.observationCutAndFrame()
			return
		}
		index.byID[entry.id] = len(index.entries)
		index.entries = append(index.entries, entry)
	}
	index.pages = observationPages(index.entries)
	index.bytes += len(index.pages) * 32
	if index.bytes > rt.config.Drive.ObservationByteCapacity {
		index.entries = index.entries[:0]
		clear(index.byID)
		index.pages = nil
		index.exhausted = true
		index.cut, index.frame = rt.observationCutAndFrame()
		index.dirty = false
		return
	}
	index.cut, index.frame = rt.observationCutAndFrame()
	index.dirty = false
}

func observationPages(entries []driveObservationEntry) []*driveObservationPage {
	pages := make([]*driveObservationPage, 0, (len(entries)+driveObservationPageSize-1)/driveObservationPageSize)
	for start := 0; start < len(entries); start += driveObservationPageSize {
		end := min(len(entries), start+driveObservationPageSize)
		pageEntries := entries[start:end]
		pages = append(pages, &driveObservationPage{entries: pageEntries})
	}
	return pages
}

func (rt *Runtime) resolveObservationLocator(index *driveObservationIndex, locator locate.Locator) (facet.FacetID, *locate.Error) {
	if err := locate.Validate(locator); err != nil {
		return 0, &locate.Error{Code: "protocol.invalid_params", Detail: err.Error()}
	}
	scope := facet.FacetID(0)
	if locator.Within != nil {
		var err *locate.Error
		scope, err = rt.resolveObservationScope(index, *locator.Within, 0)
		if err != nil {
			return 0, err
		}
	}
	if locator.Coords != nil {
		id, ok := rt.driveHitID(gfx.Point{X: locator.Coords[0], Y: locator.Coords[1]})
		if !ok {
			return 0, locate.ErrUnresolved(locator)
		}
		entryIndex, exists := index.byID[id]
		if !exists || (scope != 0 && (id == scope || !observationDescendant(index, scope, id))) {
			return 0, locate.ErrUnresolved(locator)
		}
		return index.entries[entryIndex].id, nil
	}
	found := facet.FacetID(0)
	candidates := make([]locate.Candidate, 0, 10)
	count := 0
	for _, entry := range index.entries {
		if scope != 0 && (entry.id == scope || !observationDescendant(index, scope, entry.id)) {
			continue
		}
		if !observationMatches(locator, entry) {
			continue
		}
		count++
		if len(candidates) < 10 {
			candidates = append(candidates, observationCandidate(entry))
		}
		if found == 0 {
			found = entry.id
		}
	}
	if count == 0 {
		return 0, locate.ErrUnresolved(locator)
	}
	if count > 1 {
		return 0, locate.ErrAmbiguous(locator, candidates, count > len(candidates))
	}
	return found, nil
}

func (rt *Runtime) resolveObservationScope(index *driveObservationIndex, locator locate.Locator, parentScope facet.FacetID) (facet.FacetID, *locate.Error) {
	if err := locate.Validate(locator); err != nil {
		return 0, &locate.Error{Code: "protocol.invalid_params", Detail: err.Error()}
	}
	if locator.Within != nil {
		var err *locate.Error
		parentScope, err = rt.resolveObservationScope(index, *locator.Within, parentScope)
		if err != nil {
			return 0, err
		}
	}
	found := facet.FacetID(0)
	candidates := make([]locate.Candidate, 0, 10)
	count := 0
	for _, entry := range index.entries {
		if parentScope != 0 && entry.id != parentScope && !observationDescendant(index, parentScope, entry.id) {
			continue
		}
		if !observationMatches(locator, entry) {
			continue
		}
		count++
		if len(candidates) < 10 {
			candidates = append(candidates, observationCandidate(entry))
		}
		if found == 0 {
			found = entry.id
		}
	}
	if count == 0 {
		return 0, locate.ErrUnresolved(locator)
	}
	if count > 1 {
		return 0, locate.ErrAmbiguous(locator, candidates, count > len(candidates))
	}
	return found, nil
}

func observationMatches(locator locate.Locator, entry driveObservationEntry) bool {
	switch {
	case locator.TestID != "":
		return entry.row.TestID == locator.TestID
	case locator.Role != "":
		return entry.meta.Role == locator.Role && entry.meta.Name == locator.Name
	case locator.Key != "":
		return entry.meta.Key == locator.Key
	case locator.Text != "":
		return entry.meta.Name == locator.Text
	case locator.Kind != "":
		return entry.meta.Kind == locator.Kind
	default:
		return false
	}
}

func observationDescendant(index *driveObservationIndex, ancestor, candidate facet.FacetID) bool {
	for id := candidate; id != 0; {
		parentIndex, exists := index.byID[id]
		if !exists {
			return false
		}
		parent := index.entries[parentIndex].parent
		if parent == ancestor {
			return true
		}
		id = parent
	}
	return false
}

func observationCandidate(entry driveObservationEntry) locate.Candidate {
	return locate.Candidate{FacetPath: entry.row.Path, TestID: entry.row.TestID, Role: entry.meta.Role, Name: entry.meta.Name, Kind: entry.meta.Kind}
}

func (rt *Runtime) driveHitID(point gfx.Point) (facet.FacetID, bool) {
	hitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap())
	if hitMap == nil {
		return 0, false
	}
	hit := hitMap.HitTest(point)
	if hit == nil || hit.FacetID == 0 {
		return 0, false
	}
	return hit.FacetID, true
}

func (rt *Runtime) observationCutAndFrame() (string, string) {
	frame := strconv.FormatUint(rt.frameNumber, 10)
	if rt.drivePort == nil {
		return "0", frame
	}
	return rt.drivePort.Cut(), rt.drivePort.Frame()
}
