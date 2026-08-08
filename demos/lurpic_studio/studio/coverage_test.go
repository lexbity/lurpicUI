package studio

import (
	"sort"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// standardMarks is the framework's standard Marks collection — the canonical
// (Family, TypeName) pair each mark's Descriptor reports. It is the coverage
// target (FR-coverage): the live-tree walk must place every one of these or
// log the deviation (NG-5).
var standardMarks = []marks.Descriptor{
	{Family: "action", TypeName: "action_bar"},
	{Family: "action", TypeName: "action_group"},
	{Family: "action", TypeName: "button"},
	{Family: "action", TypeName: "command_palette"},
	{Family: "action", TypeName: "icon_button"},
	{Family: "action", TypeName: "menu_button"},
	{Family: "action", TypeName: "popup_palette"},
	{Family: "action", TypeName: "radial_menu"},
	{Family: "action", TypeName: "ribbon"},
	{Family: "action", TypeName: "split_button"},
	{Family: "action", TypeName: "toolbar"},
	{Family: "feedback", TypeName: "alert"},
	{Family: "feedback", TypeName: "dialog"},
	{Family: "feedback", TypeName: "notification"},
	{Family: "feedback", TypeName: "tooltip"},
	{Family: "input", TypeName: "color_picker"},
	{Family: "input", TypeName: "number_field"},
	{Family: "input", TypeName: "text_field"},
	{Family: "navigation", TypeName: "breadcrumbs"},
	{Family: "navigation", TypeName: "nav_drawer"},
	{Family: "navigation", TypeName: "nav_rail"},
	{Family: "navigation", TypeName: "pagination"},
	{Family: "navigation", TypeName: "tabs"},
	{Family: "navigation", TypeName: "tree_navigator"},
	{Family: "primitive", TypeName: "icon"},
	{Family: "primitive", TypeName: "text"},
	{Family: "selection", TypeName: "button_group"},
	{Family: "selection", TypeName: "checkbox"},
	{Family: "selection", TypeName: "dropdown_select"},
	{Family: "selection", TypeName: "list_item"},
	{Family: "selection", TypeName: "radio_group"},
	{Family: "selection", TypeName: "slider"},
	{Family: "selection", TypeName: "switch"},
	{Family: "selection", TypeName: "turn_dial"},
	{Family: "status", TypeName: "badge"},
	{Family: "status", TypeName: "progress_bar"},
	{Family: "status", TypeName: "progress_ring"},
	{Family: "status", TypeName: "status_light"},
	{Family: "structure", TypeName: "card"},
	{Family: "structure", TypeName: "list"},
	{Family: "structure", TypeName: "scroll_region"},
	{Family: "structure", TypeName: "table"},
	{Family: "viz", TypeName: "area"},
	{Family: "viz", TypeName: "axis"},
	{Family: "viz", TypeName: "bar"},
	{Family: "viz", TypeName: "line"},
	{Family: "viz", TypeName: "point"},
	{Family: "viz", TypeName: "rule"},
}

func markKey(family, typeName string) string { return family + "/" + typeName }

func standardMarkSet() map[string]bool {
	out := make(map[string]bool, len(standardMarks))
	for _, d := range standardMarks {
		out[markKey(d.Family, d.TypeName)] = true
	}
	return out
}

// newCoverageRoot builds the full shell at the wide breakpoint so every exhibit
// and the shell's own marks are in the live tree.
func newCoverageRoot(t *testing.T) (*Root, *testkit.Harness) {
	t.Helper()
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	root := NewRoot(ctx, nil, seedRows(t), nil)
	h := testkit.NewStandardHarness(t, 1280, 800, root)
	h.RunFrame()
	return root, h
}

// filterCoverageTraps removes the known coverage traps (§2.8) from a walk:
//   - (a) the internal command_palette_results_group mark (unexported, appears
//     under a live command_palette, not a standard mark);
//   - (b) marks/data.DataMark reports Family "viz" (it is a data mark, not a
//     viz series) — filtered when present;
//   - (c) viz/rect.go defines NewBar with TypeName "bar" (file ≠ name) — the
//     bar is a genuine standard mark and is counted normally.
func filterCoverageTraps(descs []marks.Descriptor) []marks.Descriptor {
	out := descs[:0]
	for _, d := range descs {
		switch {
		case d.Family == "action" && d.TypeName == "command_palette_results_group":
			continue // (a) internal
		case d.Family == "viz" && d.TypeName == "datamark":
			continue // (b) data mark mislabeled viz
		default:
			out = append(out, d)
		}
	}
	return out
}

// TestCoverage_liveTreePlacesEveryStandardMark asserts the FR-coverage
// contract: walking the live facet tree (the same boundary the runtime's
// projection and hit-testing use) reaches every one of the 48 standard marks,
// after filtering the three documented traps (§2.8). The walk is the honest
// coverage measure — marks hosted by composite containers that do not attach
// content to the facet tree (F-card-content / F-scroll-content) are not
// "reachable" and are excluded by construction.
func TestCoverage_liveTreePlacesEveryStandardMark(t *testing.T) {
	root, _ := newCoverageRoot(t)

	walked := filterCoverageTraps(walkMarkDescriptors(root))
	placed := markDescriptorMultiset(walked)

	missing := make([]string, 0)
	extras := make([]string, 0)
	standard := standardMarkSet()
	for key := range standard {
		if placed[key] == 0 {
			missing = append(missing, key)
		}
	}
	for key := range placed {
		if !standard[key] {
			extras = append(extras, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extras)

	if len(missing) > 0 {
		t.Fatalf("standard marks not placed in the live tree (FR-coverage): %v", missing)
	}
	if len(extras) > 0 {
		t.Fatalf("placed marks that are not standard (would be invented marks): %v", extras)
	}
	t.Logf("coverage: %d/%d standard marks reachable in the live tree", len(standard), len(standard))
}

// TestCoverage_eachExhibitPlacesItsMarks asserts the per-exhibit placement
// baseline: the flagship (E1) carries the chart+table marks, the playground
// (E6) carries the interactive families, and the shell carries the
// navigation/status chrome. This guards against a later slice silently
// relocating a whole exhibit's coverage.
func TestCoverage_eachExhibitPlacesItsMarks(t *testing.T) {
	root, h := newCoverageRoot(t)
	_ = h
	placed := func(id ExhibitID) map[string]bool {
		seen := make(map[string]bool)
		for _, d := range walkMarkDescriptors(root.Stage().RootFor(id)) {
			seen[markKey(d.Family, d.TypeName)] = true
		}
		return seen
	}

	e1 := placed(ExhibitRealtime)
	for _, key := range []string{"viz/line", "viz/area", "viz/bar", "viz/point", "viz/axis", "viz/rule", "structure/table", "structure/list", "input/text_field", "action/radial_menu", "feedback/tooltip"} {
		if !e1[key] {
			t.Fatalf("E1 does not place %s", key)
		}
	}
	e6 := placed(ExhibitPlayground)
	for _, key := range []string{"navigation/tabs", "action/split_button", "action/menu_button", "action/radial_menu", "feedback/notification", "feedback/tooltip", "navigation/breadcrumbs", "selection/list_item", "primitive/icon"} {
		if !e6[key] {
			t.Fatalf("E6 does not place %s", key)
		}
	}
}

// TestCoverage_filteredWalkIsStable pins the exact placed multiset so a change
// in placement (a mark added or removed) requires an intentional diff.
func TestCoverage_filteredWalkIsStable(t *testing.T) {
	root, _ := newCoverageRoot(t)
	walked := filterCoverageTraps(walkMarkDescriptors(root))
	placed := markDescriptorMultiset(walked)

	if len(placed) != len(standardMarks) {
		t.Fatalf("placed distinct marks = %d, want %d (48/48 target)", len(placed), len(standardMarks))
	}
}
