package svg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "codeburg.org/lexbit/lurpicui/gfx"
)

func almostEqual(a, b float32) bool {
	const eps = 1e-5
	if a < b {
		return b-a <= eps
	}
	return a-b <= eps
}

func almostEqualPoint(a, b Point) bool {
	return almostEqual(a.X, b.X) && almostEqual(a.Y, b.Y)
}

func almostEqualRect(a, b Rect) bool {
	return almostEqualPoint(a.Min, b.Min) && almostEqualPoint(a.Max, b.Max)
}

func TestParseSVG_flowbiteChevronDown(t *testing.T) {
	doc := mustParseSVGFile(t, "arrows", "chevron-down.svg")
	if !almostEqualRect(doc.ViewBox, RectFromXYWH(0, 0, 24, 24)) {
		t.Fatalf("unexpected viewBox: %+v", doc.ViewBox)
	}
	if len(doc.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(doc.Elements))
	}
	el := doc.Elements[0]
	if el.Fill.Kind != SVGPaintNone {
		t.Fatalf("expected fill none, got %#v", el.Fill)
	}
	if el.Stroke == nil {
		t.Fatal("expected stroke to be present")
	}
	if el.Stroke.Paint.Kind != SVGPaintCurrentColor {
		t.Fatalf("expected currentColor stroke, got %#v", el.Stroke.Paint)
	}
	if !almostEqual(el.Stroke.Width, 2) {
		t.Fatalf("expected stroke width 2, got %v", el.Stroke.Width)
	}
	if el.Stroke.Cap != LineCapRound || el.Stroke.Join != LineJoinRound {
		t.Fatalf("unexpected stroke style: %#v", el.Stroke)
	}
	if len(el.Path.Segments) != 3 {
		t.Fatalf("expected 3 path segments, got %d", len(el.Path.Segments))
	}
	if el.Path.Segments[0].Verb != PathMoveTo || el.Path.Segments[1].Verb != PathLineTo || el.Path.Segments[2].Verb != PathLineTo {
		t.Fatalf("unexpected path verbs: %#v", el.Path.Segments)
	}
	if doc.Bounds.IsEmpty() {
		t.Fatal("expected non-empty document bounds")
	}
}

func TestParseSVG_transformFlattening_and_inheritedPaint(t *testing.T) {
	src := []byte(`
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="none" stroke="currentColor">
  <g transform="translate(10 5)">
    <g transform="scale(2)">
      <path fill="currentColor" fill-opacity="0.5" d="M1 1h2v2H1Z"/>
    </g>
  </g>
</svg>`)
	doc, err := ParseSVG(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(doc.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(doc.Elements))
	}
	el := doc.Elements[0]
	if el.Fill.Kind != SVGPaintCurrentColor {
		t.Fatalf("expected inherited currentColor fill, got %#v", el.Fill)
	}
	if !almostEqual(el.Fill.Opacity, 0.5) {
		t.Fatalf("expected fill opacity 0.5, got %v", el.Fill.Opacity)
	}
	if el.Stroke == nil || el.Stroke.Paint.Kind != SVGPaintCurrentColor {
		t.Fatalf("expected inherited currentColor stroke, got %#v", el.Stroke)
	}
	if len(el.Path.Segments) != 5 {
		t.Fatalf("expected 5 segments for rect path, got %d", len(el.Path.Segments))
	}
		if got := el.Path.Segments[0].Pts[0]; !almostEqualPoint(got, Point{X: 12, Y: 7}) {
		t.Fatalf("expected transformed move point at (12,7), got %+v", got)
	}
}

func TestParseSVG_clipPathAndUse(t *testing.T) {
	doc := mustParseSVGFile(t, "food:beverage", "pizza-slice.svg")
	if len(doc.Definitions) == 0 {
		t.Fatal("expected at least one definition")
	}
	foundClip := false
	for _, def := range doc.Definitions {
		if def.Kind == SVGDefinitionClipPath && def.ClipPath != nil && def.ClipPath.ID == "a" {
			foundClip = true
			break
		}
	}
	if !foundClip {
		t.Fatalf("expected clipPath definition with id a, got %#v", doc.Definitions)
	}
	if len(doc.Elements) != 1 {
		t.Fatalf("expected 1 visible element, got %d", len(doc.Elements))
	}
	if doc.Elements[0].ClipPath == nil {
		t.Fatal("expected clip path to be resolved onto the visible element")
	}
}

func TestParseSVG_userCircle_arcsAreNormalized(t *testing.T) {
	doc := mustParseSVGFile(t, "user", "user-circle.svg")
	if len(doc.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(doc.Elements))
	}
	el := doc.Elements[0]
	if len(el.Path.Segments) < 4 {
		t.Fatalf("expected arc path to normalize to multiple segments, got %#v", el.Path.Segments)
	}
	if doc.Bounds.IsEmpty() {
		t.Fatal("expected non-empty bounds")
	}
}

func TestParseSVG_rejectsUnsupportedConstructs(t *testing.T) {
	cases := map[string]string{
		"script":        `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"foreignObject": `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject/></svg>`,
		"animate":       `<svg xmlns="http://www.w3.org/2000/svg"><animate/></svg>`,
		"external-use":  `<svg xmlns="http://www.w3.org/2000/svg"><use href="https://example.com/icon.svg#x"/></svg>`,
		"bad-color":     `<svg xmlns="http://www.w3.org/2000/svg"><path fill="bogus" d="M0 0h1"/></svg>`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSVG([]byte(src)); err == nil {
				t.Fatal("expected parse failure")
			}
		})
	}
}

func TestParseSVG_deepNestedUseAndIndexingIterative(t *testing.T) {
	const depth = 2048
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1">`)
	for i := 0; i < depth; i++ {
		b.WriteString("<g>")
	}
	b.WriteString(`<rect id="leaf" x="0" y="0" width="1" height="1"/>`)
	for i := 0; i < depth; i++ {
		b.WriteString("</g>")
	}
	b.WriteString(`<use href="#leaf"/>`)
	b.WriteString(`</svg>`)

	doc, err := ParseSVG([]byte(b.String()))
	if err != nil {
		t.Fatalf("parse deep svg: %v", err)
	}
	if len(doc.Elements) != 2 {
		t.Fatalf("expected 2 visible elements, got %d", len(doc.Elements))
	}
	for i, el := range doc.Elements {
		if el.ID != "leaf" {
			t.Fatalf("element %d id = %q, want leaf", i, el.ID)
		}
		if el.Bounds.IsEmpty() {
			t.Fatalf("element %d bounds should be non-empty", i)
		}
	}
}

func mustParseSVGFile(t *testing.T, parts ...string) SVGDocument {
	t.Helper()
	pathParts := append([]string{"..", "..", "assets", "static-default", "flowbite-icons", "src", "outline"}, parts...)
	data, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}
	doc, err := ParseSVG(data)
	if err != nil {
		t.Fatalf("parse svg: %v", err)
	}
	return doc
}
