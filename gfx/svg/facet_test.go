package svg

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestSVGFacet_projectsInheritedPaintAnchorsAndHit(t *testing.T) {
	doc, err := ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="currentColor">
		<g opacity="0.5" stroke="currentColor" stroke-width="2" stroke-linecap="round">
			<path d="M1 1H9V9H1Z"/>
			<path d="M1 9L9 1"/>
		</g>
	</svg>`))
	if err != nil {
		t.Fatalf("parse svg: %v", err)
	}

	facet := NewSVGFacet(doc)
	facet.SetCurrentColor(gfx.ColorFromRGBA8(20, 80, 140, 255))
	bounds := gfx.RectFromXYWH(10, 20, 100, 100)

	cmds := facet.Project(bounds)
	if cmds == nil || len(cmds.Commands) < 5 {
		t.Fatalf("expected projected commands, got %#v", cmds)
	}
	if _, ok := cmds.Commands[0].(gfx.PushTransform); !ok {
		t.Fatalf("expected first command to push transform, got %T", cmds.Commands[0])
	}
	if _, ok := cmds.Commands[1].(gfx.PushOpacity); !ok {
		t.Fatalf("expected opacity to be pushed for inherited group opacity, got %T", cmds.Commands[1])
	}
	if fill, ok := cmds.Commands[2].(gfx.FillPath); !ok {
		t.Fatalf("expected fill path command, got %T", cmds.Commands[2])
	} else if r, g, b, a := fill.Brush.Color.ToRGBA8(); r != 20 || g != 80 || b != 140 || a != 255 {
		t.Fatalf("unexpected fill color: %d %d %d %d", r, g, b, a)
	}
	if stroke, ok := cmds.Commands[3].(gfx.StrokePath); !ok {
		t.Fatalf("expected stroke path command, got %T", cmds.Commands[3])
	} else if stroke.Stroke.Width != 20 {
		t.Fatalf("stroke width = %v, want 20", stroke.Stroke.Width)
	}

	anchors := facet.Anchors(bounds)
	for _, name := range []string{SVGAnchorBoundsCenter, SVGAnchorBoundsTopLeft, SVGAnchorBoundsTopRight, SVGAnchorBoundsBottomLeft, SVGAnchorBoundsBottomRight} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	if !facet.HitTest(bounds, gfx.Point{X: 20, Y: 30}) {
		t.Fatal("expected hit test inside the projected bounds")
	}
	if facet.HitTest(bounds, gfx.Point{X: 2, Y: 3}) {
		t.Fatal("expected hit test outside the projected bounds to miss")
	}
	if got := facet.SourceBounds(); got != (gfx.RectFromXYWH(0, 0, 10, 10)) {
		t.Fatalf("source bounds = %#v, want 0 0 10 10", got)
	}
}

func TestSVGFacet_projectsGradientBrushAndClipBounds(t *testing.T) {
	doc, err := ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<defs>
			<linearGradient id="g" x1="0" y1="0" x2="1" y2="0">
				<stop offset="0" stop-color="#000000"/>
				<stop offset="1" stop-color="#ffffff"/>
			</linearGradient>
			<clipPath id="c">
				<rect x="1" y="1" width="8" height="8"/>
			</clipPath>
		</defs>
		<rect x="0" y="0" width="10" height="10" fill="url(#g)" clip-path="url(#c)"/>
	</svg>`))
	if err != nil {
		t.Fatalf("parse svg: %v", err)
	}

	facet := NewSVGFacet(doc)
	cmds := facet.Project(gfx.RectFromXYWH(0, 0, 20, 20))
	if cmds == nil || len(cmds.Commands) < 4 {
		t.Fatalf("expected projected commands, got %#v", cmds)
	}
	if _, ok := cmds.Commands[0].(gfx.PushTransform); !ok {
		t.Fatalf("expected transform command, got %T", cmds.Commands[0])
	}
	if _, ok := cmds.Commands[1].(gfx.PushClipRect); !ok {
		t.Fatalf("expected clip rect command, got %T", cmds.Commands[1])
	}
	fill, ok := cmds.Commands[2].(gfx.FillPath)
	if !ok {
		t.Fatalf("expected fill path command, got %T", cmds.Commands[2])
	}
	if fill.Brush.Kind != gfx.BrushLinearGradient {
		t.Fatalf("brush kind = %v, want linear gradient", fill.Brush.Kind)
	}
}

func TestSVGFacet_rejectsUnsupportedConstructs(t *testing.T) {
	for _, src := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><foreignObject /></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><image href="http://example.com/a.png"/></svg>`,
	} {
		if _, err := ParseSVG([]byte(src)); err == nil {
			t.Fatalf("expected parse failure for unsupported source %q", src)
		}
	}
}
