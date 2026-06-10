package studio

import (
	"fmt"
	"os"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

func diagRows() []dataset.Row {
	out := make([]dataset.Row, 0, 8)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	regions := []string{"NA", "EU", "APAC", "LATAM"}
	for i := 0; i < 8; i++ {
		out = append(out, dataset.Row{
			Date:    base.AddDate(0, 0, i),
			Revenue: 1000 + float64(i)*100,
			Users:   500 + float64(i)*10,
			Region:  regions[i%len(regions)],
		})
	}
	return out
}

// TestDiagRenderTree replicates the runtime frame path and dumps per-node
// projection command counts + bounds, so we can see exactly which subtree
// produces no geometry.
func TestDiagRenderTree(t *testing.T) {
	out, _ := os.Create("/tmp/studio_diag.txt")
	defer out.Close()
	logf := func(format string, a ...any) { fmt.Fprintf(out, format+"\n", a...) }
	for _, size := range []gfx.Size{{W: 1280, H: 800}, {W: 648, H: 793}} {
		t.Run(fmt.Sprintf("%gx%g", size.W, size.H), func(t *testing.T) {
			logf("===== window %gx%g =====", size.W, size.H)
			st := state.NewAppState(diagRows())
			fonts := testkit.TestFontRegistry(t)
			root := NewRoot(st, size, fonts)

			facet.Attach(root, facet.AttachContext{})

			lr := root.Base().LayoutRole()
			mr := lr.Measure(facet.MeasureContext{ContentScale: 1}, facet.Constraints{MaxSize: size})
			logf("root measured size = %+v", mr.Size)
			lr.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, size.W, size.H))
			logf("root ArrangedBounds = %+v (mode=%v)", lr.ArrangedBounds, st.LayoutMode.Get())

			var walk func(f facet.FacetImpl, depth int)
			total := 0
			walk = func(f facet.FacetImpl, depth int) {
				if f == nil || f.Base() == nil {
					return
				}
				b := f.Base()
				bounds := gfx.Rect{}
				if r := b.LayoutRole(); r != nil {
					bounds = r.ArrangedBounds
				}
				n := 0
				if pr := b.ProjectionRole(); pr != nil && pr.OnProject != nil {
					if cl := pr.Project(facet.ProjectionContext{}); cl != nil {
						n = len(cl.Commands)
					}
				} else if rr := b.RenderRole(); rr != nil && rr.OnCollect != nil {
					var cl gfx.CommandList
					rr.OnCollect(&cl, bounds)
					n = len(cl.Commands)
				}
				total += n
				indent := ""
				for i := 0; i < depth; i++ {
					indent += "  "
				}
				logf("%s%T bounds=%+v cmds=%d children=%d", indent, f, bounds, n, len(b.Children()))
				for _, c := range b.Children() {
					walk(c, depth+1)
				}
			}
			walk(root, 0)
			logf("TOTAL commands = %d", total)
		})
	}
}
