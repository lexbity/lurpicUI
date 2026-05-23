package cook

import (
	"encoding/binary"
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"

	"codeburg.org/lexbit/lurpicui/assets/schema/lurpic/csg"
)

func TestSVGCompilerCompile(t *testing.T) {
	compiler := &SVGCompiler{}
	if got := compiler.Extensions(); len(got) != 1 || got[0] != ".svg" {
		t.Fatalf("unexpected extensions: %v", got)
	}

	src := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
		<path id="curve" d="M0 0 C0 10 10 10 10 0 Q10 10 0 0 L2 2 Z" fill="#ff0000"/>
		<rect id="big" x="0" y="0" width="20" height="20" fill="#ff0000"/>
		<rect id="small" x="0" y="0" width="10" height="10" fill="#0000ff"/>
	</svg>`)

	lods, err := compiler.Compile(src, PlatformLinux)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(lods) != 3 {
		t.Fatalf("unexpected lod count: %d", len(lods))
	}
	if lods[0].Level != 0 || lods[1].Level != 1 || lods[2].Level != 2 {
		t.Fatalf("unexpected lod levels: %+v", lods)
	}
	if got := len(lods[1].Data); got != 32*32*4 {
		t.Fatalf("unexpected lod1 byte length: %d", got)
	}

	var doc csg.Document
	doc.Init(lods[0].Data, flatbuffers.GetUOffsetT(lods[0].Data))
	if got := doc.ShapesLength(); got != 3 {
		t.Fatalf("unexpected shape count: %d", got)
	}

	var bounds csg.Rect
	if doc.Bounds(&bounds) == nil {
		t.Fatal("expected document bounds")
	}
	var min, max csg.Vec2
	if bounds.Min(&min) == nil || bounds.Max(&max) == nil {
		t.Fatal("expected document bounds vectors")
	}
	if min.X() != 0 || min.Y() != 0 || max.X() != 20 || max.Y() != 20 {
		t.Fatalf("unexpected document bounds: min=(%v,%v) max=(%v,%v)", min.X(), min.Y(), max.X(), max.Y())
	}

	var curve csg.Shape
	if !doc.Shapes(&curve, 0) {
		t.Fatal("expected curve shape")
	}
	if got := curve.VerbsLength(); got != 5 {
		t.Fatalf("unexpected verb count: %d", got)
	}
	wantVerbs := []csg.Verb{csg.VerbMoveTo, csg.VerbCubicTo, csg.VerbQuadTo, csg.VerbLineTo, csg.VerbClose}
	for i, want := range wantVerbs {
		if got := curve.Verbs(i); got != want {
			t.Fatalf("unexpected verb %d: got %v want %v", i, got, want)
		}
	}
	wantCoords := []float32{0, 0, 0, 10, 10, 10, 10, 0, 10, 10, 0, 0, 2, 2}
	if got := curve.CoordsLength(); got != len(wantCoords) {
		t.Fatalf("unexpected coord count: %d", got)
	}
	for i, want := range wantCoords {
		if got := curve.Coords(i); got != want {
			t.Fatalf("unexpected coord %d: got %v want %v", i, got, want)
		}
	}

	color := binary.LittleEndian.Uint32(lods[2].Data)
	const wantColor uint32 = 0xFF2C00D3
	if color != wantColor {
		t.Fatalf("unexpected dominant color: got %#08x want %#08x", color, wantColor)
	}
}
