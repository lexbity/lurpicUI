package studio

import (
	"image/png"
	"os"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
)

func renderRootPNG(t *testing.T, size gfx.Size, path string) {
	st := state.NewAppState(diagRows())
	fonts := testkit.TestFontRegistry(t)
	root := NewRoot(st, size, fonts)
	facet.Attach(root, facet.AttachContext{})

	lr := root.Base().LayoutRole()
	lr.Measure(facet.MeasureContext{ContentScale: 1}, facet.Constraints{MaxSize: size})
	lr.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, size.W, size.H))

	var cmds []gfx.Command
	var walk func(f facet.FacetImpl)
	walk = func(f facet.FacetImpl) {
		if f == nil || f.Base() == nil {
			return
		}
		b := f.Base()
		bounds := gfx.Rect{}
		if r := b.LayoutRole(); r != nil {
			bounds = r.ArrangedBounds
		}
		if pr := b.ProjectionRole(); pr != nil && pr.OnProject != nil {
			if cl := pr.Project(facet.ProjectionContext{Bounds: bounds, ContentScale: 1}); cl != nil {
				cmds = append(cmds, cl.Commands...)
			}
		} else if rr := b.RenderRole(); rr != nil && rr.OnCollect != nil {
			var cl gfx.CommandList
			rr.OnCollect(&cl, bounds)
			cmds = append(cmds, cl.Commands...)
		}
		for _, c := range b.Children() {
			walk(c)
		}
	}
	walk(root)

	surface := testkit.NewMemorySurface(int(size.W), int(size.H))
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("init renderer: %v", err)
	}
	frame := &render.Frame{RenderBatchs: []render.RenderBatch{{
		ID: 1, Bounds: gfx.RectFromXYWH(0, 0, size.W, size.H), Opacity: 1, CommandHash: 1,
		Commands: gfx.CommandList{Commands: cmds},
	}}}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	img := surface.Capture()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%d commands)", path, len(cmds))
}

func TestDiagRenderPNG(t *testing.T) {
	renderRootPNG(t, gfx.Size{W: 1280, H: 800}, "/tmp/studio_wide.png")
	renderRootPNG(t, gfx.Size{W: 648, H: 793}, "/tmp/studio_narrow.png")
}
