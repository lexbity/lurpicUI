package studio

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
)

func loadTestRows(t *testing.T) []dataset.Row {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash("../assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	return rows
}

func testFonts(t *testing.T) *text.FontRegistry {
	t.Helper()
	return testkit.TestFontRegistry(t)
}

func testNewRoot(t *testing.T, s *state.AppState, sz gfx.Size) *RootFacet {
	t.Helper()
	return NewRoot(s, sz, testFonts(t))
}

func TestRootWideLayout(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	ctx := facet.MeasureContext{}
	result := root.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	if result.Size.W != 1280 || result.Size.H != 800 {
		t.Fatalf("expected 1280x800, got %v", result.Size)
	}

	root.onArrange(gfx.RectFromXYWH(0, 0, 1280, 800))
	b := root.ArrangedBounds()
	chromeH := root.chromeBar.TotalHeight()

	if b.Header.Height() != chromeH {
		t.Errorf("expected header height %f, got %f", chromeH, b.Header.Height())
	}
	footerH := b.Footer.Height()
	if footerH <= 0 {
		t.Errorf("expected positive footer height, got %f", footerH)
	}
	if b.Header.Min.Y != 0 {
		t.Errorf("header should start at y=0, got %f", b.Header.Min.Y)
	}
	expectedBodyH := float32(800 - chromeH - footerH)
	if b.Body.Height() != expectedBodyH {
		t.Errorf("expected body height %f, got %f", expectedBodyH, b.Body.Height())
	}

	if b.Sources.Width() != 200 {
		t.Errorf("expected sources pane width 200, got %f", b.Sources.Width())
	}
	if b.Inspector.Width() != 280 {
		t.Errorf("expected inspector pane width 280, got %f", b.Inspector.Width())
	}
	expectedCenterW := float32(1280 - 200 - 280)
	if b.Center.Width() != expectedCenterW {
		t.Errorf("expected center pane width %f, got %f", expectedCenterW, b.Center.Width())
	}

	if b.Sources.Min.X != 0 {
		t.Errorf("expected sources at x=0, got %f", b.Sources.Min.X)
	}
	if b.Center.Min.X != 200 {
		t.Errorf("expected center at x=200, got %f", b.Center.Min.X)
	}
	if b.Inspector.Min.X != 200+expectedCenterW {
		t.Errorf("expected inspector at x=%f, got %f", 200+expectedCenterW, b.Inspector.Min.X)
	}
}

func TestRootWideLayoutProportions(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)
	root := testNewRoot(t, s, gfx.Size{W: 1920, H: 1080})

	ctx := facet.MeasureContext{}
	root.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 1920, H: 1080}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 1920, 1080))
	b := root.ArrangedBounds()
	chromeH := root.chromeBar.TotalHeight()

	if b.Sources.Width() != 200 {
		t.Errorf("expected sources pane width 200, got %f", b.Sources.Width())
	}
	if b.Inspector.Width() != 280 {
		t.Errorf("expected inspector pane width 280, got %f", b.Inspector.Width())
	}
	expectedCenterW := float32(1920 - 200 - 280)
	if b.Center.Width() != expectedCenterW {
		t.Errorf("expected center pane width %f, got %f", expectedCenterW, b.Center.Width())
	}

	if b.Header.Height() != chromeH {
		t.Errorf("expected header height %f, got %f", chromeH, b.Header.Height())
	}
	footerH := b.Footer.Height()
	if footerH <= 0 {
		t.Errorf("expected positive footer height, got %f", footerH)
	}
	expectedBodyH := float32(1080 - chromeH - footerH)
	if b.Body.Height() != expectedBodyH {
		t.Errorf("expected body height %f, got %f", expectedBodyH, b.Body.Height())
	}
}

func TestRootNarrowLayout(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)
	root := testNewRoot(t, s, gfx.Size{W: 480, H: 800})

	ctx := facet.MeasureContext{}
	result := root.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	if result.Size.W != 480 || result.Size.H != 800 {
		t.Fatalf("expected 480x800, got %v", result.Size)
	}

	root.onArrange(gfx.RectFromXYWH(0, 0, 480, 800))
	b := root.ArrangedBounds()
	chromeH := root.chromeBar.TotalHeight()

	if b.Header.Height() != chromeH {
		t.Errorf("expected header height %f, got %f", chromeH, b.Header.Height())
	}
	footerH := b.Footer.Height()
	if footerH <= 0 {
		t.Errorf("expected positive footer height, got %f", footerH)
	}
	expectedBodyH := float32(800 - chromeH - footerH)
	if b.Body.Height() != expectedBodyH {
		t.Errorf("expected body height %f, got %f", expectedBodyH, b.Body.Height())
	}

	if b.Center.Width() != 480 {
		t.Errorf("expected center pane full width 480, got %f", b.Center.Width())
	}
	if b.Sources.Width() != 0 || b.Sources.Height() != 0 {
		t.Errorf("expected sources pane collapsed (0x0), got %v", b.Sources)
	}
	if b.Inspector.Width() != 0 || b.Inspector.Height() != 0 {
		t.Errorf("expected inspector pane collapsed (0x0), got %v", b.Inspector)
	}
}

func TestRootLayoutModeStoreOnCrossing(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)

	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	if got := s.LayoutMode.Get(); got != state.LayoutWide {
		t.Errorf("expected LayoutWide for 1280 wide, got %v", got)
	}

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	if got := s.LayoutMode.Get(); got != state.LayoutNarrow {
		t.Errorf("expected LayoutNarrow for 480 wide, got %v", got)
	}

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	if got := s.LayoutMode.Get(); got != state.LayoutWide {
		t.Errorf("expected LayoutWide again after 1280 wide, got %v", got)
	}
}

func TestRootLayoutModeNoWriteOnSameWidth(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	s.LayoutMode.Set(state.LayoutWide)
	layoutWriteCount := 0
	subID := s.LayoutMode.OnChange.Subscribe(func(_ signal.Change[state.LayoutMode]) {
		layoutWriteCount++
	})
	defer s.LayoutMode.OnChange.Unsubscribe(subID)

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	if layoutWriteCount != 0 {
		t.Errorf("expected 0 LayoutMode writes for same width, got %d", layoutWriteCount)
	}

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1920, H: 800}})
	if layoutWriteCount != 0 {
		t.Errorf("expected 0 LayoutMode writes when staying wide, got %d", layoutWriteCount)
	}
}

func TestRootLayoutModeWritesOnce(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	layoutWriteCount := 0
	subID := s.LayoutMode.OnChange.Subscribe(func(_ signal.Change[state.LayoutMode]) {
		layoutWriteCount++
	})
	defer s.LayoutMode.OnChange.Unsubscribe(subID)

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	if layoutWriteCount != 1 {
		t.Errorf("expected exactly 1 LayoutMode write when crossing breakpoint, got %d", layoutWriteCount)
	}
}

func TestRootHeaderFooterBodyHeights(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 1280, 800))
	b := root.ArrangedBounds()
	chromeH := root.chromeBar.TotalHeight()

	if b.Header.Min.Y != 0 {
		t.Errorf("header should start at y=0, got %f", b.Header.Min.Y)
	}
	if b.Body.Min.Y != chromeH {
		t.Errorf("body should start at y=%f, got %f", chromeH, b.Body.Min.Y)
	}
	footerH := b.Footer.Height()
	if b.Footer.Min.Y != 800-footerH {
		t.Errorf("footer should start at y=%f, got %f", float32(800-footerH), b.Footer.Min.Y)
	}
}

func TestRootContentSizeMatchesWindow(t *testing.T) {
	rows := loadTestRows(t)
	s := state.NewAppState(rows)

	sizes := []gfx.Size{
		{W: 1024, H: 768},
		{W: 1280, H: 800},
		{W: 1920, H: 1080},
		{W: 360, H: 640},
	}
	for _, sz := range sizes {
		root := testNewRoot(t, s, sz)
		root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: sz})
		root.onArrange(gfx.RectFromXYWH(0, 0, sz.W, sz.H))
		b := root.ArrangedBounds()
		if b.Header.Width() != sz.W {
			t.Errorf("size %v: header width %f != %f", sz, b.Header.Width(), sz.W)
		}
		if b.Body.Width() != sz.W {
			t.Errorf("size %v: body width %f != %f", sz, b.Body.Width(), sz.W)
		}
		if b.Footer.Width() != sz.W {
			t.Errorf("size %v: footer width %f != %f", sz, b.Footer.Width(), sz.W)
		}
		reconstructedH := b.Header.Height() + b.Body.Height() + b.Footer.Height()
		diff := reconstructedH - sz.H
		if diff < -1 || diff > 1 {
			t.Errorf("size %v: reconstructed height %f (header=%f body=%f footer=%f) != %f",
				sz, reconstructedH, b.Header.Height(), b.Body.Height(), b.Footer.Height(), sz.H)
		}
	}
}
