package studio

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

func collectAllMarks(f facet.FacetImpl, found *[]marks.Mark) {
	if f == nil || f.Base() == nil {
		return
	}
	if m, ok := f.(marks.Mark); ok {
		*found = append(*found, m)
	}
	for _, child := range f.Base().Children() {
		impl := child.Impl()
		if impl != nil {
			collectAllMarks(impl, found)
		} else {
			collectAllMarks(child, found)
		}
	}
}

func accessibleState() *state.AppState {
	raw, err := os.ReadFile(filepath.FromSlash("../assets/metrics.csv"))
	if err != nil {
		return state.NewAppState(nil)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		return state.NewAppState(nil)
	}
	return state.NewAppState(rows)
}

func TestAllAccessibleMarksHaveNonEmptyName(t *testing.T) {
	s := accessibleState()
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var allMarks []marks.Mark
	collectAllMarks(root, &allMarks)

	knownEmpty := map[string]bool{
		"status/progress_bar":  true,
		"status/progress_ring": true,
		"status/status_light":  true,
	}

	var missingNames []string
	for _, m := range allMarks {
		acc, ok := m.(marks.Accessible)
		if !ok {
			continue
		}
		if acc.AccessibleName() == "" {
			desc := marks.Describe(m)
			key := desc.Family + "/" + desc.TypeName
			if !knownEmpty[key] {
				missingNames = append(missingNames, key)
			}
		}
	}

	if len(missingNames) > 0 {
		t.Errorf("%d accessible marks have empty AccessibleName:\n", len(missingNames))
		for _, name := range missingNames {
			t.Errorf("  %s", name)
		}
	}
}

func TestSpecificMarkAccessibleNames(t *testing.T) {
	s := accessibleState()
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var allMarks []marks.Mark
	collectAllMarks(root, &allMarks)

	expected := map[string]string{
		"action/ribbon":   "Main Ribbon",
		"navigation/tabs": "Center View",
		"structure/table": "Data Table",
	}

	for _, m := range allMarks {
		acc, ok := m.(marks.Accessible)
		if !ok {
			continue
		}
		desc := marks.Describe(m)
		key := desc.Family + "/" + desc.TypeName
		want, ok := expected[key]
		if !ok {
			continue
		}
		got := acc.AccessibleName()
		if got != want {
			t.Errorf("%s: expected AccessibleName %q, got %q", key, want, got)
		}
	}
}

func TestIconOnlyMarksHaveAccessibleName(t *testing.T) {
	s := accessibleState()
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var allMarks []marks.Mark
	collectAllMarks(root, &allMarks)

	for _, m := range allMarks {
		acc, ok := m.(marks.Accessible)
		if !ok {
			continue
		}
		desc := marks.Describe(m)
		if desc.TypeName == "icon_button" {
			if acc.AccessibleName() == "" {
				t.Errorf("icon_button %s has empty AccessibleName", desc.Family)
			}
		}
	}
}

func TestAllFocusableMarksWork(t *testing.T) {
	s := accessibleState()
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var allMarks []marks.Mark
	collectAllMarks(root, &allMarks)

	focusedMark := map[string]int{}
	for _, m := range allMarks {
		base := m.Base()
		if base == nil {
			continue
		}
		focusRole := base.FocusRole()
		if focusRole == nil || focusRole.Focusable == nil {
			continue
		}
		canFocus := focusRole.Focusable()
		desc := marks.Describe(m)
		key := desc.Family + "/" + desc.TypeName
		if canFocus {
			focusedMark[key]++
		}
	}
	if len(focusedMark) == 0 {
		t.Error("no focusable marks found in tree")
	}
	t.Logf("Focusable marks: %d types, %d total", len(focusedMark), func() int {
		s := 0
		for _, c := range focusedMark { s += c }
		return s
	}())
	for k, c := range focusedMark {
		t.Logf("  %s: %d", k, c)
	}
}

func TestAllMarksHaveValidDescriptors(t *testing.T) {
	s := accessibleState()
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(s, gfx.Size{W: 1280, H: 800}, fonts)

	var allMarks []marks.Mark
	collectAllMarks(root, &allMarks)

	typeCount := map[string]int{}
	for _, m := range allMarks {
		desc := marks.Describe(m)
		key := desc.Family + "/" + desc.TypeName
		typeCount[key]++
		if desc.Family == "" {
			t.Errorf("mark has empty family: %+v", desc)
		}
		if desc.TypeName == "" {
			t.Errorf("mark has empty TypeName: %+v", desc)
		}
	}
}
