package integration

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/chart"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/marks/uiinput"
	"codeburg.org/lexbit/lurpicui/marks/uinav"
	"codeburg.org/lexbit/lurpicui/marks/uinotification"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

type testAnchorMark struct {
	ID   string
	At   gfx.Point
	base facet.Facet
}

func (m *testAnchorMark) Base() *facet.Facet {
	m.base.BindImpl(m)
	return &m.base
}

func (m *testAnchorMark) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyBasic, ConstructionClass: marks.ConstructionPrimitive, Type: marks.TypeName("integration:anchor"), AnchorExporting: true}
}

func (m *testAnchorMark) AuthoredID() string               { return m.ID }
func (m *testAnchorMark) OnAttach(ctx facet.AttachContext) {}
func (m *testAnchorMark) OnDetach()                        {}
func (m *testAnchorMark) OnActivate()                      {}
func (m *testAnchorMark) OnDeactivate()                    {}

func (m *testAnchorMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return layout.AnchorSet{"center": m.At}
}

type boundRangeScale struct {
	Length store.Binding[float64]
}

func (s boundRangeScale) Kind() chart.ScaleKind { return chart.ScaleLinear }

func (s boundRangeScale) Map(value any) float32 {
	v, ok := value.(float64)
	if !ok {
		if vv, ok := value.(int); ok {
			v = float64(vv)
		}
	}
	length := s.Length.Get()
	if length <= 0 {
		length = 1
	}
	return float32(v / 100 * length)
}

func (s boundRangeScale) Ticks(desired int) []any {
	if desired <= 0 {
		desired = 5
	}
	out := make([]any, desired)
	for i := range out {
		out[i] = float64(i) * 25
	}
	return out
}

func (s boundRangeScale) FormatTick(value any) string {
	return "tick"
}

func TestIntegration_slider_controls_axis(t *testing.T) {
	length := store.NewBinding(240.0)
	slider := &uiinput.Slider{
		Value: length,
		Min:   120,
		Max:   400,
	}
	axis := &chart.Axis{
		Orientation: chart.AxisBottom,
		Scale:       boundRangeScale{Length: length},
	}
	root := &structure.Group{Children: []marks.Mark{slider, axis}}
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), root)
	h.RunFrame()
	before := axis.ExportAnchors(layout.AnchorExportContext{})["tick-4"]
	h.InjectEvent(testkit.PointerPress(180, 10, platform.PointerLeft))
	h.RunFrame()
	after := axis.ExportAnchors(layout.AnchorExportContext{})["tick-4"]
	if length.Get() == 240 {
		t.Fatal("expected slider interaction to update shared length binding")
	}
	if before == after {
		t.Fatalf("expected axis anchor to move, before=%#v after=%#v", before, after)
	}
}

func TestIntegration_dialog_hosts_mixed_mark_families(t *testing.T) {
	d := &uinotification.Dialog{
		Open:  store.NewBinding(true),
		Title: "Dialog",
		Body: []marks.Mark{
			&uiinput.TextInput{Value: store.NewBinding("")},
			&uinav.Tabs{Items: []uinav.TabItem{{Key: "a", Label: "A"}}, Selected: store.NewBinding("a")},
			&uinotification.Progress{Mode: uinotification.ProgressDeterminate, Shape: uinotification.ProgressLinear, Value: store.NewBinding(0.5)},
		},
		Actions: []marks.Mark{
			&uiinput.Button{Label: "Close"},
		},
	}
	d.Base()
	if got := len(d.Base().Children()); got < 4 {
		t.Fatalf("dialog children = %d, want at least 4", got)
	}
	families := make(map[marks.Family]bool)
	for _, child := range d.Base().Children() {
		if impl := child.Impl(); impl != nil {
			if authored, ok := impl.(marks.Mark); ok {
				families[authored.Descriptor().Family] = true
			}
		}
	}
	if !families[marks.FamilyUIInput] || !families[marks.FamilyUINav] || !families[marks.FamilyUINotification] {
		t.Fatalf("mixed families not attached: %#v", families)
	}
}

func TestIntegration_menu_anchored_to_icon(t *testing.T) {
	icon := &testAnchorMark{ID: "icon", At: gfx.Point{X: 220, Y: 120}}
	menu := &uinav.Menu{
		Anchor: uinav.AnchorSourceRef{MarkID: "icon", Anchor: "center"},
		Open:   store.NewBinding(true),
		Items:  []uinav.MenuItem{{Key: "a", Label: "Alpha"}},
	}
	root := &structure.Group{Children: []marks.Mark{icon, menu}}
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), root)
	h.RunFrame()
	if icon.Base().Parent() == nil || menu.Base().Parent() == nil {
		t.Fatal("expected icon and menu to be attached into the same tree")
	}
	if icon.Base().Parent() != menu.Base().Parent() {
		t.Fatal("expected icon and menu to share the same parent tree")
	}
}

func TestIntegration_snackbar_floats_above_mixed_content(t *testing.T) {
	snackbar := &uinotification.Snackbar{
		Open:    store.NewBinding(true),
		Message: "Saved",
	}
	root := &structure.Group{Children: []marks.Mark{
		&uiinput.Button{Label: "Under"},
		snackbar,
	}}
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), root)
	h.RunFrame()
	specs := snackbar.OnLayerSpecs()
	if len(specs) != 1 || specs[0].RenderOrder != 500 {
		t.Fatalf("unexpected snackbar layer specs: %#v", specs)
	}
}
