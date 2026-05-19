package diagnostics_test

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/theme"
)

type iconInspectorRuntime struct {
	rootStyle any
}

func (s iconInspectorRuntime) Schedule(j job.AnyJob)  {}
func (s iconInspectorRuntime) CancelJob(id job.JobID) {}
func (s iconInspectorRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s iconInspectorRuntime) RootStyleContext() any { return s.rootStyle }
func (s iconInspectorRuntime) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}

func TestInspector_describe_includes_icon_snapshot(t *testing.T) {
	icon := primitive.NewIcon(primitive.IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="currentColor"><path d="M1 1H9V9H1Z"/></svg>`))
	icon.SetAccessibleName("Square")
	rt := iconInspectorRuntime{rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil)}
	facet.Attach(icon, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	size := icon.Base().LayoutRole().Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 64, H: 64}}).Size
	icon.Base().LayoutRole().Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(2, 3, size.W, size.H))

	insp := diagnostics.NewInspector(icon)
	info, ok := insp.Find(icon.Base().ID())
	if !ok {
		t.Fatal("expected icon facet")
	}
	if info.Icon == nil {
		t.Fatal("expected icon snapshot")
	}
	if info.Icon.SourceKind != "inline-svg" || info.Icon.AccessibleName != "Square" {
		t.Fatalf("icon snapshot = %#v", info.Icon)
	}
	desc := insp.Describe()
	if desc == "" || !strings.Contains(desc, "Icon:") {
		t.Fatalf("expected describe output to include icon snapshot, got %q", desc)
	}
}
