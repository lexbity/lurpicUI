package runtime_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/capabilities"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// catalogRT is a minimal runtime stub for the cross-seam catalog test.
type catalogRT struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (catalogRT) Schedule(j job.AnyJob)                                              {}
func (catalogRT) CancelJob(id job.JobID)                                             {}
func (catalogRT) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (s catalogRT) RootStyleContext() any                                            { return s.rootStyle }
func (s catalogRT) FacetByID(id facet.FacetID) facet.FacetImpl                       { return nil }
func (s catalogRT) FontRegistry() *text.FontRegistry                                 { return s.fonts }

func findModuleRoot() string {
	dir, err := filepath.Abs(".")
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func TestCatalog_buildsFromCapindexAndSurvivesDispose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-seam catalog test in short mode")
	}

	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	root := findModuleRoot()
	if root == "" {
		t.Fatal("could not find module root")
	}

	caps, err := capabilities.ScanMarks(root)
	if err != nil {
		t.Fatalf("capabilities.ScanMarks: %v", err)
	}

	demoCategories := map[string]bool{"structure": true, "navigation": true, "data": true}
	filtered := make([]capabilities.Capability, 0, len(caps))
	for _, c := range caps {
		if c.Kind != capabilities.KindMark {
			continue
		}
		if !demoCategories[c.Category] {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		t.Fatal("no capabilities matched the demo filter (structure/navigation/data)")
	}

	columns := []structure.TableColumn{
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "category", Label: "Category", Sortable: true},
		{Key: "constructor", Label: "Constructor", Sortable: true},
		{Key: "intent", Label: "Intent"},
	}
	rows := make([]structure.TableRow, 0, len(filtered))
	for _, c := range filtered {
		intent := strings.TrimSpace(c.Intent)
		if idx := strings.IndexAny(intent, ".\n"); idx > 0 {
			intent = intent[:idx+1]
		}
		if len(intent) > 120 {
			intent = intent[:117] + "..."
		}
		rows = append(rows, structure.TableRow{
			Key:   c.Path,
			Cells: []string{c.TypeName, c.Category, c.Constructor, intent},
		})
	}

	data := structure.TableData{
		Columns:       columns,
		Rows:          rows,
		SortColumnKey: "category",
	}
	selection := store.NewValueStore("")

	fonts := fontdata.TestFontRegistry(t)
	rootStyle := theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil)
	rt := catalogRT{
		rootStyle: rootStyle,
		fonts:     fonts,
	}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)

	table := structure.NewTable("marks catalog", data, selection)
	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	defer facet.Dispose(table)

	table.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}})
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, bounds)

	// (a) Table rendered at least one child row.
	if len(table.Children()) == 0 {
		t.Fatal("expected at least one child row in the catalog table")
	}

	// (b) Accessible name is "marks catalog".
	if table.AccessibleName() != "marks catalog" {
		t.Fatalf("AccessibleName=%q, want %q", table.AccessibleName(), "marks catalog")
	}

	// (c) Selection survives dispose+rebuild.
	contracttest.AssertValueSurvivesDispose[string](t,
		func() *store.ValueStore[string] { return selection },
		func(s *store.ValueStore[string]) facet.FacetImpl {
			return structure.NewTable("marks catalog", data, s)
		},
		func(m facet.FacetImpl) {
			m.(*structure.Table).Selection.Set(filtered[0].Path)
		},
	)
}
