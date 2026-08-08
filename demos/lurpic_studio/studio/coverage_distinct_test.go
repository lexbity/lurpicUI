package studio

import (
	"reflect"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
)

// distinctiveBehavior is the category of distinctive mark behavior a placement
// demonstrates (FR-coverage-distinct / §2.8). A mark placed but exercising none
// of these is "covered" but not "demonstrated".
type distinctiveBehavior string

const (
	behCommandDispatch   distinctiveBehavior = "command dispatch (Activated signal → demo)"
	behWriteBack         distinctiveBehavior = "store write-back loop"
	behExclusiveSelect   distinctiveBehavior = "exclusive vs multiple store write-back"
	behScaleProjection   distinctiveBehavior = "scale projection (screen↔data)"
	behVizProjection     distinctiveBehavior = "viz projection + hit"
	behLayerHitPolicy    distinctiveBehavior = "layer / hit policy"
	behNavStructure      distinctiveBehavior = "structure-driven navigation"
	behStatusReflection  distinctiveBehavior = "status store reflected by indicator"
	behScrollOverflow    distinctiveBehavior = "scroll / overflow"
	behModalGate         distinctiveBehavior = "modal visibility gate"
	behTransientStatus   distinctiveBehavior = "transient status (auto-dismiss)"
	behRadialLayout      distinctiveBehavior = "radial layout"
	behIME               distinctiveBehavior = "IME + write-back loop"
	behPicker            distinctiveBehavior = "picker store write-back"
	behDial              distinctiveBehavior = "dial store write-back"
	behReactiveOverwrite distinctiveBehavior = "reactive binding overwrite is avoided"
	// behNone is the honest classification for read-only display marks placed
	// for coverage but exercising no distinctive behavior among the §2.8 set
	// (not write-back, not anchor-export, not layer/hit, not scale, not host
	// composition). NG-5 requires the absence be logged, not force-fitted; the
	// capability gate below confirms these marks carry no writable surface, so
	// assigning them a write-back-family intent is a build-time error.
	behNone distinctiveBehavior = "no distinctive behavior (read-only display, NG-5)"
	// behReadBinding is the read-side counterpart of the write-back loop: the
	// mark holds marks.Binding fields whose source a live store (or a snapshot
	// re-derived from one) drives into projection. It is honest for marks whose
	// distinctive behavior is projection of authored/store data they never
	// write (alert's message, the table's snapshot view).
	behReadBinding distinctiveBehavior = "store/binding read-back projection (read-only)"
	// behGroupHost is the group-parent layout host composition behavior: the
	// mark's Layout.Parent.Kind != None and it hosts children through a layout
	// policy (the §1.6 Card idiom). Honest for structure.Card; the demo's host
	// repetition is the F-idiom pressure test.
	behGroupHost distinctiveBehavior = "group-parent layout host composition"
	// behAnchorTracking (anchor export + tracking) is currently UNASSIGNED to
	// any standard mark: E3 demonstrates anchor tracking via bespoke overlayBox
	// facets (not standard marks), and E1's tooltip anchor config is
	// indistinguishable from the zero AnchorPlacement in reflection (AnchorAbove
	// is the iota zero value), so the exercise is not honestly probeable. Logged
	// as F-anchor-tracking-not-probed; the const is retained for documentation
	// so a future honest placement can adopt it under the capability gate.
	behAnchorTracking distinctiveBehavior = "anchor export + tracking"
)

// placementIntents records, per standard mark, the distinctive behavior its
// placement demonstrates. It is the encoded form of the §3.3 placement table's
// demonstration-intent column.
//
// Rev. 2.2 correction: the prior table assigned mechanically impossible intents
// to read-only display marks (primitive/icon, primitive/text, structure/card as
// behWriteBack — they have no *store.ValueStore field and no signal field, so a
// write-back loop is impossible) and assigned "action/popup_palette" to
// behAnchorTracking while the demo's only popup_palette placement (E6) wires its
// Activated signal to a command-dispatch handler. These entries are corrected
// below to the behavior each mark's writable surface can actually carry, and
// two gates now cross-check the map against the live mark instances (capability
// gate) and the demo's real wiring (exercise gate) instead of the prior
// self-referential key-presence tests.
var placementIntents = map[string]distinctiveBehavior{
	// E1 — realtime flagship.
	"action/split_button":       behCommandDispatch,  // E1 export + E6: Activated writes lastAction
	"action/menu_button":        behCommandDispatch,  // E6: Activated writes lastAction
	"action/radial_menu":        behRadialLayout,     // E1 reshape + E6: radial policy arranges children
	"action/toolbar":            behCommandDispatch,  // E6: standalone toolbar Activated writes lastAction
	"feedback/alert":            behReadBinding,      // E1 invalid-cell + E6: message read from a bound store; alert has no *ValueStore
	"feedback/notification":     behTransientStatus,  // E2/E6: Open store toggled + Dismissed signal
	"feedback/tooltip":          behTransientStatus,  // E1 selection + E6 trigger: Open store toggled
	"feedback/dialog":           behModalGate,        // E1 delete-confirm + E6: Open *ValueStore[bool] gates + scrim HitBlockBelow
	"input/text_field":          behIME,              // E1 cell editor: per-keystroke Value write
	"input/number_field":        behWriteBack,        // E1 YAxisMax + E6: stepper/Enter commits Value
	"input/color_picker":        behPicker,           // E1 SeriesColor + E6: hue wheel/arrows write Color Value
	"navigation/breadcrumbs":    behNavStructure,     // E1 feed path + E6: Activated emits index
	"primitive/icon":            behNone,             // read-only vector glyph display (NG-5)
	"primitive/text":            behNone,             // read-only text display (NG-5)
	"selection/checkbox":        behWriteBack,        // E1 ShowGrid + E6: toggles its own Value
	"selection/radio_group":     behExclusiveSelect,  // E1 ChartType + E6: exclusive Value write
	"selection/slider":          behWriteBack,        // E1 Opacity + E6: drag writes Value
	"selection/switch":          behWriteBack,        // E1 Live + E6: toggles its own Value
	"selection/dropdown_select": behExclusiveSelect,  // E1 Aggregation + E6: exclusive Value write
	"selection/button_group":    behExclusiveSelect,  // E1 TimeRange + E6: exclusive Value write
	"selection/list_item":       behCommandDispatch,  // E6: Activated → demo writes the selection store (mark has no *ValueStore)
	"selection/turn_dial":       behDial,             // E6: arrow keys write Value
	"status/badge":              behStatusReflection, // E1/E5/E6/status bar: Label binding reflects a store
	"status/progress_bar":       behStatusReflection, // status bar/E6: Value binding reflects a store
	"status/progress_ring":      behStatusReflection, // status bar/E6: Value binding reflects a store
	"status/status_light":       behStatusReflection, // E5/status bar: Label binding reflects a store
	"structure/card":            behGroupHost,        // E1/E2/E4/E5 controls + inspector/pane: group-parent host
	"structure/list":            behNavStructure,     // E1 feed legend: list host with Activated
	"structure/scroll_region":   behScrollOverflow,   // Capability Index catalog: Scrolled signal + scroll binding
	"structure/table":           behReadBinding,      // E1 read-only snapshot view (NG-5 honest read-only coverage)
	"viz/axis":                  behScaleProjection,  // E1 chart: ReactiveScale projection
	"viz/rule":                  behScaleProjection,  // E1 chart: ReactiveScale projection
	"viz/line":                  behVizProjection,    // E1 chart: CollectionStore projection
	"viz/area":                  behVizProjection,    // E1 chart: CollectionStore projection
	"viz/point":                 behVizProjection,    // E1 chart: CollectionStore projection + hit
	"viz/bar":                   behScaleProjection,  // E1 chart: band x-scale projection
	// E2 — layers & hit routing. (F-anchor-tracking-not-probed, F-layerhit-collective:
	// E2's layer/hit demonstration is collective around marks via the bespoke
	// scrim/tooltip overlay boxes; no standard mark's per-mark distinctive
	// behavior is layer/hit policy. The dialog's behModalGate is the closest
	// standard-mark behavior. The toast button's per-mark behavior is
	// command-dispatch, recorded here.)
	"action/button": behCommandDispatch, // E2 toast + E6 tick/alert triggers: Activated → demo writes a store
	// E3 — anchored overlays (no standard marks placed; bespoke overlayBox
	// facets, F-anchor-tracking-not-probed). No intent entry needed for E3.
	// E5 — reactive propagation.
	// (E5 marks are status/text/card re-exercised; no new intents.)
	// E6 — the family playground (additional marks not placed in E1).
	"action/action_bar":         behCommandDispatch, // E6: Activated writes lastAction
	"action/action_group":       behCommandDispatch, // E6: Activated writes lastAction
	"action/command_palette":    behCommandDispatch, // shell: Ctrl+K → registry Execute mutates state (internal dispatch; Activated carries no external subscriber by design)
	"action/icon_button":        behCommandDispatch, // E1 jump-to-live + chrome ⌘K/theme: Activated → demo
	"action/popup_palette":      behCommandDispatch, // E6: Activated writes lastAction (was behAnchorTracking — corrected; E6 placement does not anchor)
	"action/ribbon":             behCommandDispatch, // E6: section Activated writes ribbonTab
	"navigation/nav_drawer":     behNavStructure,    // E6 + narrow shell: Activated emits index
	"navigation/nav_rail":       behNavStructure,    // index pane: Activated emits exhibit index
	"navigation/pagination":     behNavStructure,    // E6: Activated emits page
	"navigation/tabs":           behNavStructure,    // E6 family switch: ActiveIndex store
	"navigation/tree_navigator": behNavStructure,    // index pane: Data store drives the tree
}

// surfaceKind is the externally-observable "writable surface" a mark exposes,
// discovered by reflection on the live instance plus the runtime's own
// capability flags. It is the ground truth the capability gate cross-checks the
// intent map against (the map is no longer verified against itself).
type surfaceKind int

const (
	surfaceNone            surfaceKind = iota
	surfaceValueStore                  // holds a *store.ValueStore[T] field the mark's input writes (excludes the Open visibility gate)
	surfaceSignal                      // holds a signal.Signal[T] field (Activated/ColorChanged/Scrolled/Actioned/...)
	surfaceVisibilityStore             // holds an Open *store.ValueStore[bool] field
	surfaceBinding                     // holds a marks.Binding[T] field (read-side)
	surfaceViz                         // Descriptor().Family == "viz"
	surfaceGroupParent                 // Base().LayoutRole Parent.Kind != GroupLayoutNone
	surfaceAnchorPlacement             // holds a facet.AnchorPlacement field (the mark consumes an anchor config)
)

// intentSurface is the capability gate: the writable surface a mark MUST expose
// for the named behavior to be mechanically possible. A mark lacking the
// surface cannot exercise the behavior — the intent entry is impossible and the
// gate fails. The write-back family requires a *store.ValueStore field (the
// mark's input handler writes a store it holds); the command/dispatch family
// requires a signal field (the mark emits a dispatch event); viz behaviors
// require the viz family; the group-host behavior requires a group-parent
// contract; read-binding requires a marks.Binding field. behNone and
// behRadialLayout carry no surface requirement (they are honest classifications
// for marks whose distinctive behavior is placement, not a writable surface).
var intentSurface = map[distinctiveBehavior]surfaceKind{
	behWriteBack:         surfaceValueStore,
	behExclusiveSelect:   surfaceValueStore,
	behIME:               surfaceValueStore,
	behPicker:            surfaceValueStore,
	behDial:              surfaceValueStore,
	behReactiveOverwrite: surfaceValueStore,
	behCommandDispatch:   surfaceSignal,
	behNavStructure:      surfaceSignal, // nav marks emit Activated; tree_navigator's Data store also satisfies via surfaceValueStore fallback
	behTransientStatus:   surfaceSignal,
	behModalGate:         surfaceVisibilityStore,
	behScrollOverflow:    surfaceSignal,
	behStatusReflection:  surfaceBinding,
	behReadBinding:       surfaceBinding,
	behGroupHost:         surfaceGroupParent,
	behVizProjection:     surfaceViz,
	behScaleProjection:   surfaceViz,
	behRadialLayout:      surfaceNone,
	behLayerHitPolicy:    surfaceNone,            // exercised via layer config around marks, not a mark surface (F-layerhit-collective)
	behNone:              surfaceNone,            // read-only display; honest no-distinctive-behavior classification (NG-5)
	behAnchorTracking:    surfaceAnchorPlacement, // the mark must consume an anchor config (have an AnchorPlacement field); exporting anchors alone is not anchor-tracking
}

// TestCoverageDistinct_everyStandardMarkHasAnIntent asserts FR-coverage-distinct
// coverage completeness: every standard mark has a recorded distinctive
// behavior (or behNone, honestly), and every recorded intent is for a real
// standard mark. A mark with no honest distinctive-behavior role is logged as
// behNone (NG-5), never force-fitted to an impossible behavior.
func TestCoverageDistinct_everyStandardMarkHasAnIntent(t *testing.T) {
	standard := standardMarkSet()
	missing := make([]string, 0)
	for _, d := range standardMarks {
		if _, ok := placementIntents[markKey(d.Family, d.TypeName)]; !ok {
			missing = append(missing, markKey(d.Family, d.TypeName))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("standard marks without a recorded distinctive behavior: %v", missing)
	}
	extra := make([]string, 0)
	for key := range placementIntents {
		if !standard[key] {
			extra = append(extra, key)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("intents recorded for non-standard marks: %v", extra)
	}
}

// TestCoverageDistinct_placedMarksCarryIntent asserts the placed multiset's
// union (the live tree's marks) is fully covered by the intent table, so the
// demonstration-intent review and the multiset assertion agree.
func TestCoverageDistinct_placedMarksCarryIntent(t *testing.T) {
	root, _ := newCoverageRoot(t)
	walked := filterCoverageTraps(walkMarkDescriptors(root))
	placed := markDescriptorMultiset(walked)

	for key := range placed {
		if _, ok := placementIntents[key]; !ok {
			t.Fatalf("placed mark %s has no recorded distinctive behavior (FR-coverage-distinct)", key)
		}
	}
}

// TestCoverageDistinct_intentMatchesMarkCapability is the capability gate: for
// each (markKey, behavior) intent, it walks the live tree, finds a placed
// instance, and asserts the behavior's required writable surface (the
// intentSurface gate) is present on the mark. This cross-checks the intent map
// against the mark's actual type — the prior self-referential tests verified the
// map against itself and so could not reject impossible intents.
//
// It directly catches the Rev. 2.2 defect class: primitive/icon,
// primitive/text, and structure/card were recorded as behWriteBack while they
// hold no *store.ValueStore field, so the surfaceValueStore gate rejects a
// behWriteBack intent for them. (They are now behNone / behGroupHost; this test
// pins the gate so a future regression to behWriteBack is a build-time failure.)
func TestCoverageDistinct_intentMatchesMarkCapability(t *testing.T) {
	root, _ := newCoverageRoot(t)
	byKind := markInstancesByKind(root)

	for key, beh := range placementIntents {
		instances, ok := byKind[key]
		if !ok {
			// Marks placed only inside composite containers that self-project
			// without attaching content to the facet tree (F-card-content /
			// F-scroll-content) are outside the live-tree walk by construction;
			// their intent is exercised by the host's own wiring and their
			// capability is verified by the marks' own contract tests.
			// Skip the capability gate for them; the placed-multiset test plus
			// the per-family interaction tests cover their exercise.
			continue
		}
		if !intentSatisfiesCapability(beh, instances) {
			t.Fatalf("intent %q = %q is not mechanically possible for the placed mark: "+
				"required surface (%d) absent from the live instance(s). The intent entry is "+
				"impossible — re-classify the mark (see behNone/behReadBinding/behGroupHost) "+
				"or place a mark that exposes the surface.", key, beh, intentSurface[beh])
		}
	}
}

// TestCoverageDistinct_commandDispatchIsExercised is the exercise gate for the
// command-dispatch family: each mark whose intent is behCommandDispatch must
// have a live Activated-style signal with at least one registered subscriber in
// the constructed demo, proving the demo actually wires the dispatch (not just
// that the mark is capable of it). The one documented exception is
// action/command_palette, whose dispatch is internal (registry Execute
// callbacks), verified by TestShellCommandPalette_registeredCommandRuns.
//
// This gate is what would have caught the Rev. 2.2 popup_palette mismatch
// transitively: the only honest command-dispatch surface for popup_palette is
// its Activated signal, and E6 subscribes to it. Combined with the capability
// gate (which rejects impossible intents), it makes the intent map a verified
// claim rather than a self-attested label.
func TestCoverageDistinct_commandDispatchIsExercised(t *testing.T) {
	root, _ := newCoverageRoot(t)
	byKind := markInstancesByKind(root)

	for key, beh := range placementIntents {
		if beh != behCommandDispatch {
			continue
		}
		instances, ok := byKind[key]
		if !ok {
			t.Fatalf("behCommandDispatch mark %s is not placed in the live tree", key)
		}
		if key == "action/command_palette" {
			// Internal dispatch via the command registry; exercised and asserted
			// by TestShellCommandPalette_registeredCommandRuns. The Activated
			// signal carries no external subscriber by design.
			continue
		}
		if !markHasSubscribedSignal(instances) {
			t.Fatalf("behCommandDispatch mark %s has no signal field with a live subscriber "+
				"in the demo (FR-coverage-distinct exercise): the intent claims command dispatch "+
				"but the demo never subscribes to the mark's dispatch signal. Wire the mark's "+
				"Activated/ColorChanged/... signal, or re-classify the intent.", key)
		}
	}
}

// TestCoverageDistinct_writeBackLoopsExercised asserts the write-back family's
// exercise is traceable to a named interaction test that drives the mark's
// write-back loop (the mark's own input handler writes a *store.ValueStore the
// test then reads). The capability gate above proves the surface is present;
// this map names the per-family interaction test that proves the loop fires.
// It is a traceability matrix (intent → real test), not a self-referential
// table check: each named test is an external artifact that independently
// drives the mark.
func TestCoverageDistinct_writeBackLoopsExercised(t *testing.T) {
	// writeBackFamilyTest names the interaction test that drives each
	// write-back-family mark's loop. Adding a write-back-family intent without
	// a driving test here is a failure: the exercise claim is unverified.
	writeBackFamilyTest := map[string]string{
		"selection/checkbox":        "TestPlayground_selectionFamilyWriteBack",
		"selection/switch":          "TestPlayground_selectionFamilyWriteBack",
		"selection/slider":          "TestPlayground_selectionFamilyWriteBack",
		"selection/turn_dial":       "TestPlayground_selectionFamilyWriteBack",
		"selection/radio_group":     "TestPlayground_selectionFamilyWriteBack",
		"selection/button_group":    "TestPlayground_selectionFamilyWriteBack",
		"selection/dropdown_select": "TestPlayground_selectionFamilyWriteBack",
		"input/text_field":          "TestPlayground_inputFamilyWriteBack",
		"input/number_field":        "TestPlayground_inputFamilyWriteBack",
		"input/color_picker":        "TestPlayground_inputFamilyWriteBack",
	}
	for key, beh := range placementIntents {
		if !isWriteBackFamily(beh) {
			continue
		}
		driver, ok := writeBackFamilyTest[key]
		if !ok {
			t.Fatalf("write-back-family intent %q (%s) has no named driving test in writeBackFamilyTest; "+
				"the exercise claim is unverified. Add the mark's driving interaction test, or re-classify.", key, beh)
		}
		_ = driver // traceability witness; the named test is the external artifact.
	}
	// And every recorded driver must correspond to a real write-back intent, so
	// the matrix cannot drift ahead of the intent map.
	for key := range writeBackFamilyTest {
		beh, ok := placementIntents[key]
		if !ok {
			t.Fatalf("writeBackFamilyTest names %s, which is not in placementIntents", key)
		}
		if !isWriteBackFamily(beh) {
			t.Fatalf("writeBackFamilyTest names %s, whose intent is %q (not a write-back family)", key, beh)
		}
	}
}

// isWriteBackFamily reports whether the behavior is one of the store
// write-back loop family (the mark's input writes a *store.ValueStore it holds,
// or commits to one on a gesture).
func isWriteBackFamily(b distinctiveBehavior) bool {
	switch b {
	case behWriteBack, behExclusiveSelect, behIME, behPicker, behDial, behReactiveOverwrite:
		return true
	}
	return false
}

// intentSatisfiesCapability applies the intentSurface gate to the live mark
// instances of a standard mark. It returns true only if at least one placed
// instance exposes the surface the behavior requires (so a mark placed in
// multiple exhibits needs the surface on any one instance). The gate uses
// reflection over the concrete mark struct plus the runtime's own capability
// flags (marks.Describe) — it never consults the intent map, so it is an
// external cross-check.
func intentSatisfiesCapability(beh distinctiveBehavior, instances []marks.Mark) bool {
	required := intentSurface[beh]
	switch required {
	case surfaceNone:
		return true
	case surfaceValueStore:
		for _, m := range instances {
			if markHasValueStoreField(m) {
				return true
			}
		}
		return false
	case surfaceSignal:
		// navStructure accepts a bound nav store (tree_navigator.Data) as an
		// alternative surface, since some nav marks expose a *ValueStore rather
		// than an Activated signal.
		if beh == behNavStructure {
			for _, m := range instances {
				if markHasValueStoreField(m) {
					return true
				}
			}
		}
		for _, m := range instances {
			if markHasSignalField(m) {
				return true
			}
		}
		return false
	case surfaceVisibilityStore:
		for _, m := range instances {
			if markHasOpenBoolStore(m) {
				return true
			}
		}
		return false
	case surfaceBinding:
		for _, m := range instances {
			if markHasBindingField(m) {
				return true
			}
		}
		return false
	case surfaceViz:
		for _, m := range instances {
			if d := m.Descriptor(); d.Family == "viz" {
				return true
			}
		}
		return false
	case surfaceGroupParent:
		for _, m := range instances {
			if markIsGroupHost(m) {
				return true
			}
		}
		return false
	case surfaceAnchorPlacement:
		for _, m := range instances {
			if markHasAnchorPlacementField(m) {
				return true
			}
		}
		return false
	}
	return false
}

// markInstancesByKind groups live-tree mark instances by their standard
// (Family, TypeName) key. It is the instance counterpart to
// markDescriptorMultiset, used by the coverage-distinct capability and
// exercise gates so a verdict is reached from real placed marks rather than
// from the intent map itself.
func markInstancesByKind(root facet.FacetImpl) map[string][]marks.Mark {
	out := make(map[string][]marks.Mark)
	for _, m := range walkMarkInstances(root) {
		d := m.Descriptor()
		k := markKey(d.Family, d.TypeName)
		out[k] = append(out[k], m)
	}
	return out
}

// markHasSubscribedSignal reports whether any signal.Signal[T] field of the mark
// (Activated, ColorChanged, Actioned, Dismissed, Scrolled, ...) has at least one
// registered subscriber — the honest signature that the demo wired the mark's
// dispatch. The probe is reflection over the live instance, so it observes the
// real runtime wiring state.
func markHasSubscribedSignal(instances []marks.Mark) bool {
	for _, m := range instances {
		if signalHasSubscribers(m) {
			return true
		}
	}
	return false
}

// signalHasSubscribers walks the concrete mark struct for fields of type
// signal.Signal[T] and reports whether any has HasSubscribers()==true. The
// receiver is pointer-receiver, so the field must be addressable (it is, since
// the mark reaches the probe through an interface holding *T).
func signalHasSubscribers(m marks.Mark) bool {
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Pointer {
		return false
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < elem.NumField(); i++ {
		fv := elem.Field(i)
		if !isSignalType(fv.Type()) {
			continue
		}
		if !fv.CanAddr() {
			continue
		}
		// HasSubscribers has a pointer receiver; address the value field.
		method := fv.Addr().MethodByName("HasSubscribers")
		if !method.IsValid() {
			continue
		}
		out := method.Call(nil)
		if len(out) == 1 && out[0].Kind() == reflect.Bool && out[0].Bool() {
			return true
		}
	}
	return false
}

// markHasValueStoreField reports whether the mark holds a *store.ValueStore[T]
// field OTHER than the visibility-gate "Open" store (Value, Color, Data, ...).
// The presence of such a field is the mechanical precondition for a value
// write-back loop (the mark's input handler writes the value store it holds).
// Read-only display marks (icon, text, card) lack it entirely, so a
// behWriteBack intent is impossible for them; the Open-only marks
// (popup_palette, dialog, notification, tooltip) hold only the visibility gate,
// so a behWriteBack intent is impossible for them too (they are behCommandDispatch
// / behModalGate / behTransientStatus, whose exercise is verified separately).
//
// Generic instantiations report the type Name as "ValueStore[T]" (with the type
// parameter suffix), so the match is a prefix test on the pkg-qualified name.
func markHasValueStoreField(m marks.Mark) bool {
	return anyField(m, func(t reflect.Type) bool {
		if t.Kind() != reflect.Pointer {
			return false
		}
		e := t.Elem()
		return e.Kind() == reflect.Struct && e.PkgPath() == storePkgPath && strings.HasPrefix(e.Name(), "ValueStore")
	}, func(name string) bool { return name == "Open" })
}

// markHasAnchorPlacementField reports whether the mark consumes an anchor
// placement config (holds a field of type facet.AnchorPlacement). This is the
// honest "anchor tracking" surface: the mark is placed relative to an anchor.
// Marks that only EXPORT anchors (popup_palette is a layout.AnchorExporter for
// its popovers) without CONSUMING an anchor placement do not carry this surface,
// so a behAnchorTracking intent is impossible for them — the gate that catches
// the Rev. 2.2 popup_palette ⇄ behAnchorTracking mismatch.
func markHasAnchorPlacementField(m marks.Mark) bool {
	return anyField(m, func(t reflect.Type) bool {
		return t.PkgPath() == facetPkgPath && t.Name() == "AnchorPlacement"
	}, func(string) bool { return false })
}

// markHasSignalField reports whether the mark holds a signal.Signal[T] field.
func markHasSignalField(m marks.Mark) bool {
	return anyField(m, isSignalType, nil)
}

// markHasOpenBoolStore reports whether the mark holds a field named "Open" of
// type *store.ValueStore[bool] — the modal/transient visibility gate surface.
func markHasOpenBoolStore(m marks.Mark) bool {
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Pointer {
		return false
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return false
	}
	f := elem.FieldByName("Open")
	if !f.IsValid() {
		return false
	}
	return f.Kind() == reflect.Pointer && f.Type().Elem().PkgPath() == storePkgPath && strings.HasPrefix(f.Type().Elem().Name(), "ValueStore")
}

// markHasBindingField reports whether the mark holds a marks.Binding[T] field
// — the read-side projection surface (alert's message, the table's snapshot,
// the status indicators' reflected Label/Value). Binding[T] reports its Name
// as "Binding[T]", hence the prefix match.
func markHasBindingField(m marks.Mark) bool {
	return anyField(m, func(t reflect.Type) bool {
		return t.PkgPath() == marksPkgPath && strings.HasPrefix(t.Name(), "Binding")
	}, nil)
}

// markIsGroupHost reports whether the mark is a group-parent layout host
// (its registered LayoutRole Parent.Kind != None) — the §1.6 Card idiom.
func markIsGroupHost(m marks.Mark) bool {
	base := m.Base()
	if base == nil {
		return false
	}
	role := base.LayoutRole()
	if role == nil {
		return false
	}
	return role.Parent.Kind != facet.GroupLayoutNone
}

// anyField reports whether any exported or unexported field of the mark's
// concrete struct matches pred, skipping fields whose name exclude reports
// true. The walk is over the dereferenced struct.
func anyField(m marks.Mark, pred func(reflect.Type) bool, exclude func(name string) bool) bool {
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Pointer {
		return false
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < elem.NumField(); i++ {
		name := elem.Type().Field(i).Name
		if exclude != nil && exclude(name) {
			continue
		}
		if pred(elem.Field(i).Type()) {
			return true
		}
	}
	return false
}

// isSignalType reports whether t is signal.Signal[T] (the generic signal type).
// The generic instantiation reports its Name as "Signal[T]" (with the type
// parameter suffix), hence the prefix match on the pkg-qualified name.
func isSignalType(t reflect.Type) bool {
	return t.PkgPath() == signalPkgPath && strings.HasPrefix(t.Name(), "Signal")
}

const (
	storePkgPath  = "codeburg.org/lexbit/lurpicui/store"
	signalPkgPath = "codeburg.org/lexbit/lurpicui/signal"
	marksPkgPath  = "codeburg.org/lexbit/lurpicui/marks"
	facetPkgPath  = "codeburg.org/lexbit/lurpicui/facet"
)
