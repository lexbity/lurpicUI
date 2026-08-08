package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/theme"
)

// newShell returns a harness running one frame of the gallery shell at the
// given window size.
func newShell(t *testing.T, w, h int) (*Root, *testkit.Harness) {
	t.Helper()
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: float32(w), H: float32(h)},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	root := NewRoot(ctx, nil, nil)
	harness := testkit.NewStandardHarness(t, w, h, root)
	harness.RunFrame()
	return root, harness
}

func arranged(t *testing.T, f interface{ Base() *facet.Facet }) gfx.Rect {
	t.Helper()
	return f.Base().LayoutRole().ArrangedBounds
}

func TestRootShell_threePaneSplit(t *testing.T) {
	const w, h = 1280, 800
	root, _ := newShell(t, w, h)

	chrome := arranged(t, root.ChromeStack())
	gallery := arranged(t, root.GallerySplit())
	status := arranged(t, root.StatusBar())

	// Vertical stack: chrome on top, status at the bottom, gallery between.
	if chrome.Min.X != 0 || chrome.Max.X != w {
		t.Fatalf("chrome bounds = %v, want full width", chrome)
	}
	if status.Min.X != 0 || status.Max.X != w {
		t.Fatalf("status bounds = %v, want full width", status)
	}
	if chrome.Min.Y != 0 {
		t.Fatalf("chrome top = %v, want 0", chrome.Min.Y)
	}
	if status.Max.Y != h {
		t.Fatalf("status bottom = %v, want %d", status.Max.Y, h)
	}
	if gallery.Min.Y != chrome.Max.Y {
		t.Fatalf("gallery top = %v, want chrome bottom %v", gallery.Min.Y, chrome.Max.Y)
	}
	if status.Min.Y != gallery.Max.Y {
		t.Fatalf("status top = %v, want gallery bottom %v", status.Min.Y, gallery.Max.Y)
	}
	if gallery.Min.Y >= gallery.Max.Y {
		t.Fatalf("gallery has zero/negative height: %v", gallery)
	}

	// The three panes, in order, fill the gallery.
	panes := root.GallerySplit().Panes()
	if len(panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(panes))
	}
	b0 := arranged(t, panes[0].Facet)
	b1 := arranged(t, panes[1].Facet)
	b2 := arranged(t, panes[2].Facet)

	if b0.Min.X != 0 || b2.Max.X != w {
		t.Fatalf("pane span = %v..%v, want 0..%d", b0.Min.X, b2.Max.X, w)
	}
	if b0.Min.Y != gallery.Min.Y || b0.Max.Y != gallery.Max.Y {
		t.Fatalf("pane 0 height = %v, want gallery %v", b0, gallery)
	}
	if b1.Min.Y != b0.Min.Y || b1.Max.Y != b0.Max.Y {
		t.Fatalf("pane 1 height = %v, want pane 0 %v", b1, b0)
	}
	if b2.Min.Y != b0.Min.Y || b2.Max.Y != b0.Max.Y {
		t.Fatalf("pane 2 height = %v, want pane 0 %v", b2, b0)
	}

	// Fixed panes keep their declared width; the stage absorbs the residual.
	if b0.Width() != indexPaneWidth {
		t.Fatalf("index pane width = %v, want %d", b0.Width(), indexPaneWidth)
	}
	if b2.Width() != inspectorPaneWidth {
		t.Fatalf("inspector pane width = %v, want %d", b2.Width(), inspectorPaneWidth)
	}
	residual := float32(w) - indexPaneWidth - inspectorPaneWidth - 2*dividerSize
	if b1.Width() != residual {
		t.Fatalf("stage pane width = %v, want residual %v", b1.Width(), residual)
	}

	// Dividers: exactly DividerSize between consecutive panes.
	if d := b1.Min.X - b0.Max.X; d != dividerSize {
		t.Fatalf("divider after index = %v, want %d", d, dividerSize)
	}
	if d := b2.Min.X - b1.Max.X; d != dividerSize {
		t.Fatalf("divider before inspector = %v, want %d", d, dividerSize)
	}
}

func TestRootShell_galleryStretchesToResidual(t *testing.T) {
	// At 1280x800 the gallery must span from the chrome bottom to the status
	// top — the MainAxisMax stretch absorbed the vertical residual.
	root, _ := newShell(t, 1280, 800)
	chrome := arranged(t, root.ChromeStack())
	status := arranged(t, root.StatusBar())
	gallery := arranged(t, root.GallerySplit())
	if got := status.Min.Y - chrome.Max.Y; got != gallery.Height() {
		t.Fatalf("gallery height = %v, want residual %v", gallery.Height(), got)
	}
}

func TestRootShell_noOverflowAcrossSizes(t *testing.T) {
	for _, size := range [][2]int{{1280, 800}, {1024, 768}, {960, 600}} {
		root, _ := newShell(t, size[0], size[1])
		status := arranged(t, root.StatusBar())
		if status.Max.Y > float32(size[1]) {
			t.Fatalf("at %dx%d status bottom %v overflows the window", size[0], size[1], status.Max.Y)
		}
		chrome := arranged(t, root.ChromeStack())
		if chrome.Min.X != 0 || chrome.Max.X != float32(size[0]) {
			t.Fatalf("at %dx%d chrome width = %v, want full width", size[0], size[1], chrome)
		}
	}
}
