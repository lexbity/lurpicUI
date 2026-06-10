package studio

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

func collectAllMarkDescriptors(f facet.FacetImpl, depth int, found *[]marks.Descriptor) {
	if f == nil || f.Base() == nil {
		return
	}
	if m, ok := f.(marks.Mark); ok {
		*found = append(*found, marks.Describe(m))
	}
	for _, child := range f.Base().Children() {
		impl := child.Impl()
		if impl != nil {
			collectAllMarkDescriptors(impl, depth+1, found)
		} else {
			collectAllMarkDescriptors(child, depth+1, found)
		}
	}
}

func expected48Marks() map[string]int {
	return map[string]int{
		"primitive/text":                 1,
		"primitive/icon":                 0,
		"action/button":                  0,
		"action/icon_button":             0,
		"action/split_button":            0,
		"action/menu_button":             0,
		"action/toolbar":                 1,
		"action/ribbon":                  1,
		"action/action_bar":              0,
		"action/action_group":            1,
		"action/radial_menu":             1,
		"action/command_palette":         1,
		"action/popup_palette":           1,
		"input/text_field":               1,
		"input/number_field":             1,
		"input/color_picker":             1,
		"selection/checkbox":             1,
		"selection/radio_group":          1,
		"selection/slider":               1,
		"selection/switch":               1,
		"selection/dropdown_select":      1,
		"selection/button_group":         1,
		"selection/list_item":            4,
		"selection/turn_dial":            1,
		"navigation/breadcrumbs":         0,
		"navigation/nav_drawer":          1,
		"navigation/nav_rail":            1,
		"navigation/pagination":          1,
		"navigation/tabs":                1,
		"navigation/tree_navigator":      1,
		"feedback/alert":                 1,
		"feedback/dialog":                1,
		"feedback/notification":          1,
		"feedback/tooltip":               1,
		"status/badge":                   1,
		"status/progress_bar":            1,
		"status/progress_ring":           1,
		"status/status_light":            1,
		"structure/card":                 1,
		"structure/list":                 0,
		"structure/scroll_region":        3,
		"structure/table":                1,
		"viz/axis":                       2,
		"viz/rule":                       1,
		"viz/point":                      1,
		"viz/line":                       1,
		"viz/area":                       1,
		"viz/bar":                        1,
	}
}

func markKey(d marks.Descriptor) string {
	return d.Family + "/" + d.TypeName
}

func TestCoverageAll48MarksPresent(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("../assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	s := state.NewAppState(rows)
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var descriptors []marks.Descriptor
	collectAllMarkDescriptors(root, 0, &descriptors)

	got := map[string]int{}
	for _, d := range descriptors {
		got[markKey(d)]++
	}

	expected := expected48Marks()
	var missing []string
	var extra []string
	var countMismatch []string

	for key, want := range expected {
		if want <= 0 {
			continue
		}
		if got[key] == 0 {
			missing = append(missing, key)
		} else if got[key] != want {
			countMismatch = append(countMismatch, key)
		}
	}
	for key := range got {
		want := expected[key]
		if want <= 0 && got[key] > 0 {
			extra = append(extra, key)
		} else if want > 0 && got[key] != want {
			countMismatch = append(countMismatch, key)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing marks (%d):", len(missing))
		for _, m := range missing {
			t.Errorf("  - %s (expected %d)", m, expected[m])
		}
	}
	if len(extra) > 0 {
		t.Errorf("Unexpected marks (%d):", len(extra))
		for _, e := range extra {
			t.Errorf("  - %s (got %d)", e, got[e])
		}
	}
	if len(countMismatch) > 0 {
		t.Errorf("Count mismatches (%d):", len(countMismatch))
		for _, m := range countMismatch {
			t.Errorf("  - %s: expected %d, got %d", m, expected[m], got[m])
		}
	}
	if len(missing) > 0 || len(extra) > 0 || len(countMismatch) > 0 {
		t.Fail()
	}

	total := 0
	for _, v := range got {
		total += v
	}
	t.Logf("Total marks in tree: %d", total)
}

func TestCoverageMarkCountSummary(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("../assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	s := state.NewAppState(rows)
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var descriptors []marks.Descriptor
	collectAllMarkDescriptors(root, 0, &descriptors)

	if len(descriptors) == 0 {
		t.Fatal("no marks found in tree")
	}

	familyCount := map[string]int{}
	for _, d := range descriptors {
		familyCount[d.Family]++
	}

	if len(familyCount) < 5 {
		t.Errorf("expected at least 5 mark families, got %d: %v", len(familyCount), familyCount)
	}

	var families []string
	for f := range familyCount {
		families = append(families, f)
	}
	sort.Strings(families)

	t.Logf("Marks by family:")
	for _, f := range families {
		t.Logf("  %s: %d", f, familyCount[f])
	}
	t.Logf("Total: %d marks across %d families", len(descriptors), len(families))
}
