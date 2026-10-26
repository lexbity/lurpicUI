package canonical

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

func digestCommand(t *testing.T, command gfx.Command) string {
	t.Helper()
	digest, err := DigestCommands(gfx.CommandList{Commands: []gfx.Command{command}})
	if err != nil {
		t.Fatalf("DigestCommands: %v", err)
	}
	return digest
}

func digestFrame(t *testing.T, frame *render.Frame) string {
	t.Helper()
	digest, err := Digest(frame)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	return digest
}

func TestCanonicalEveryFieldMutation(t *testing.T) {
	basePath := gfx.Path{Segments: []gfx.PathSegment{{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: 3, Y: 4}}}}}
	baseBrush := gfx.Brush{Kind: gfx.BrushLinearGradient, Color: gfx.Color{R: 1, A: 1}, GradientStart: gfx.Point{X: 1}, GradientEnd: gfx.Point{X: 4}, GradientStops: []gfx.GradientStop{{Offset: .5, Color: gfx.Color{G: 1, A: 1}}}}
	baseStroke := gfx.StrokeStyle{Width: 2, Cap: gfx.LineCapRound, Join: gfx.LineJoinBevel, MiterLimit: 7, Dash: []float32{2, 3}, DashOffset: 1}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Pix[0], img.Pix[3] = 10, 255
	imgHandle := testframe.Image(img)

	mutations := []struct {
		name string
		a, b gfx.Command
	}{
		{"transform", gfx.PushTransform{Matrix: gfx.Transform{A: 1}}, gfx.PushTransform{Matrix: gfx.Transform{A: 2}}},
		{"clip", gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 2, 3)}, gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 2, 4)}},
		{"opacity", gfx.PushOpacity{Alpha: .5}, gfx.PushOpacity{Alpha: .6}},
		{"fill-rect", gfx.FillRect{Rect: gfx.RectFromXYWH(1, 2, 3, 4), Brush: baseBrush}, gfx.FillRect{Rect: gfx.RectFromXYWH(1, 2, 3, 4), Brush: gfx.Brush{Kind: baseBrush.Kind, Color: gfx.Color{R: 1, A: 1}, GradientStart: baseBrush.GradientStart, GradientEnd: baseBrush.GradientEnd, GradientStops: []gfx.GradientStop{{Offset: .5, Color: gfx.Color{G: .9, A: 1}}}}}},
		{"stroke-rect", gfx.StrokeRect{Rect: gfx.RectFromXYWH(0, 0, 5, 5), Stroke: baseStroke, Brush: baseBrush}, gfx.StrokeRect{Rect: gfx.RectFromXYWH(0, 0, 5, 5), Stroke: gfx.StrokeStyle{Width: 3, Cap: baseStroke.Cap, Join: baseStroke.Join, MiterLimit: baseStroke.MiterLimit, Dash: baseStroke.Dash, DashOffset: baseStroke.DashOffset}, Brush: baseBrush}},
		{"fill-path", gfx.FillPath{Path: basePath, Brush: baseBrush}, gfx.FillPath{Path: gfx.Path{Segments: []gfx.PathSegment{{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: 3, Y: 5}}}}}, Brush: baseBrush}},
		{"stroke-path", gfx.StrokePath{Path: basePath, Stroke: baseStroke, Brush: baseBrush}, gfx.StrokePath{Path: basePath, Stroke: baseStroke, Brush: gfx.Brush{Kind: gfx.BrushSolid, Color: gfx.Color{B: 1, A: 1}}}},
		{"polyline", gfx.DrawPolyline{Points: []gfx.Point{{X: 1, Y: 2}}, Stroke: baseStroke, Brush: baseBrush}, gfx.DrawPolyline{Points: []gfx.Point{{X: 1, Y: 2}}, Stroke: baseStroke, Brush: baseBrush, Closed: true}},
		{"points", gfx.DrawPoints{Points: []gfx.Point{{X: 1, Y: 2}}, Radius: 2, Brush: baseBrush}, gfx.DrawPoints{Points: []gfx.Point{{X: 1, Y: 2}}, Radius: 3, Brush: baseBrush}},
		{"selection", gfx.DrawSelectionRects{Rects: []gfx.Rect{gfx.RectFromXYWH(0, 0, 2, 3)}, Brush: baseBrush}, gfx.DrawSelectionRects{Rects: []gfx.Rect{gfx.RectFromXYWH(0, 0, 2, 4)}, Brush: baseBrush}},
		{"image", gfx.DrawImage{Image: imgHandle, DestRect: gfx.RectFromXYWH(0, 0, 4, 4), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 1}, gfx.DrawImage{Image: imgHandle, DestRect: gfx.RectFromXYWH(0, 0, 4, 4), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: .5}},
		{"texture", gfx.DrawTexture{TextureID: 1, DestRect: gfx.RectFromXYWH(0, 0, 4, 4), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 1}, gfx.DrawTexture{TextureID: 2, DestRect: gfx.RectFromXYWH(0, 0, 4, 4), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 1}},
		{"shadow", gfx.DrawBlurredShadow{Path: basePath, Color: gfx.Color{A: 1}, BlurRadius: 2, Offset: gfx.Point{X: 1}, Inner: true}, gfx.DrawBlurredShadow{Path: basePath, Color: gfx.Color{A: 1}, BlurRadius: 3, Offset: gfx.Point{X: 1}, Inner: true}},
		{"batch-cache-id", gfx.BeginRenderBatch{Bounds: gfx.RectFromXYWH(0, 0, 4, 4), CacheID: 1}, gfx.BeginRenderBatch{Bounds: gfx.RectFromXYWH(0, 0, 4, 4), CacheID: 2}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if a, b := digestCommand(t, mutation.a), digestCommand(t, mutation.b); a == b {
				t.Fatalf("field mutation did not change digest: %s", mutation.name)
			}
		})
	}
	if digestCommand(t, gfx.PopTransform{}) == digestCommand(t, gfx.PopClip{}) {
		t.Fatal("command variant mutation did not change digest")
	}
}

func TestCanonicalCommandInventory(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller did not return the test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	fset := token.NewFileSet()
	commandFile, err := parser.ParseFile(fset, filepath.Join(root, "gfx", "command.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse gfx command inventory: %v", err)
	}
	encoderFile, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(file), "encode.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse canonical encoder: %v", err)
	}
	declared := make(map[string]bool)
	for _, decl := range commandFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "isCommand" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		if typ, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
			declared[typ.Name] = true
		}
	}
	encoded := make(map[string]bool)
	ast.Inspect(encoderFile, func(node ast.Node) bool {
		typeSwitch, ok := node.(*ast.TypeSwitchStmt)
		if !ok {
			return true
		}
		for _, stmt := range typeSwitch.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expr := range clause.List {
				selector, ok := expr.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				pkg, ok := selector.X.(*ast.Ident)
				if ok && pkg.Name == "gfx" {
					encoded[selector.Sel.Name] = true
				}
			}
		}
		return true
	})
	if len(declared) == 0 {
		t.Fatal("gfx command inventory is empty")
	}
	for name := range declared {
		if !encoded[name] {
			t.Errorf("production command %s has no canonical encoder case", name)
		}
	}
	for name := range encoded {
		if !declared[name] {
			t.Errorf("canonical encoder handles non-production command %s", name)
		}
	}
}

func TestCanonicalAssetGenerationChangesDigest(t *testing.T) {
	command := func(id uint64) gfx.Command {
		return gfx.DrawTexture{TextureID: id, DestRect: gfx.RectFromXYWH(0, 0, 4, 4), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingBilinear, Opacity: 1}
	}
	if a, b := digestCommand(t, command(17)), digestCommand(t, command(18)); a == b {
		t.Fatal("texture generation change did not change digest")
	}
	first := image.NewRGBA(image.Rect(0, 0, 1, 1))
	second := image.NewRGBA(image.Rect(0, 0, 1, 1))
	first.Pix[0], second.Pix[0] = 1, 2
	firstHandle, secondHandle := testframe.Image(first), testframe.Image(second)
	if a, b := digestCommand(t, gfx.DrawImage{Image: firstHandle}), digestCommand(t, gfx.DrawImage{Image: secondHandle}); a == b {
		t.Fatal("image content change with the same dimensions did not change digest")
	}
}

func TestCanonicalGlyphRunFields(t *testing.T) {
	base := gfx.DrawGlyphRun{Run: text.GlyphRun{
		Glyphs: []text.PositionedGlyph{{GlyphID: 7, X: 1, Y: 2, Advance: 3, RuneIndex: 4}},
		Text:   "A", Size: 12, Style: text.TextStyle{Family: "Noto", Size: 12, Weight: text.WeightRegular},
		Bounds: text.RectFromXYWH(0, 0, 8, 12), Advance: 8, Direction: 1, Level: 0,
		GraphemeAdvances: []float32{8}, Language: "en", Script: 1,
		Metrics: text.LineMetrics{Ascent: 9, Descent: 3, LineHeight: 12}, LogicalIndex: 1,
	}}
	mutations := []struct {
		name string
		edit func(*gfx.DrawGlyphRun)
	}{
		{"text", func(c *gfx.DrawGlyphRun) { c.Run.Text = "B" }},
		{"glyph-position", func(c *gfx.DrawGlyphRun) { c.Run.Glyphs[0].X++ }},
		{"glyph-index", func(c *gfx.DrawGlyphRun) { c.Run.Glyphs[0].GlyphID++ }},
		{"style", func(c *gfx.DrawGlyphRun) { c.Run.Style.Weight++ }},
		{"bounds", func(c *gfx.DrawGlyphRun) { c.Run.Bounds.Max.X++ }},
		{"direction", func(c *gfx.DrawGlyphRun) { c.Run.Direction++ }},
		{"language", func(c *gfx.DrawGlyphRun) { c.Run.Language = "ar" }},
		{"script", func(c *gfx.DrawGlyphRun) { c.Run.Script++ }},
		{"metrics", func(c *gfx.DrawGlyphRun) { c.Run.Metrics.Ascent++ }},
		{"grapheme-advance", func(c *gfx.DrawGlyphRun) { c.Run.GraphemeAdvances[0]++ }},
		{"origin", func(c *gfx.DrawGlyphRun) { c.Origin.X++ }},
		{"brush", func(c *gfx.DrawGlyphRun) { c.Brush.Color.R++ }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := base
			changed.Run.Glyphs = append([]text.PositionedGlyph(nil), base.Run.Glyphs...)
			changed.Run.GraphemeAdvances = append([]float32(nil), base.Run.GraphemeAdvances...)
			mutation.edit(&changed)
			if digestCommand(t, base) == digestCommand(t, changed) {
				t.Fatalf("glyph-run field %s did not change digest", mutation.name)
			}
		})
	}
}

func TestCanonicalFrameMetadataMutation(t *testing.T) {
	baseInput := render.FrameInput{
		Layers:       []render.LayerInput{{RenderOrder: 2, ClipRect: gfx.RectFromXYWH(0, 0, 8, 8), FirstBatch: 0, BatchCount: 1}},
		Batches:      []render.BatchInput{{Bounds: gfx.RectFromXYWH(1, 1, 4, 4), Opacity: 1}},
		DirtyRegions: []gfx.Rect{gfx.RectFromXYWH(1, 1, 4, 4)},
	}
	base := testframe.Seal(baseInput)
	defer base.Release()
	mutations := []struct {
		name   string
		mutate func(*render.FrameInput)
	}{
		{"layer-order", func(f *render.FrameInput) { f.Layers[0].RenderOrder++ }},
		{"layer-clip", func(f *render.FrameInput) { f.Layers[0].ClipRect.Max.X++ }},
		{"batch-bounds", func(f *render.FrameInput) { f.Batches[0].Bounds.Max.X++ }},
		{"batch-opacity", func(f *render.FrameInput) { f.Batches[0].Opacity = .5 }},
		{"damage-region", func(f *render.FrameInput) { f.DirtyRegions[0].Max.X++ }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changedInput := render.FrameInput{Layers: append([]render.LayerInput(nil), baseInput.Layers...), Batches: append([]render.BatchInput(nil), baseInput.Batches...), DirtyRegions: append([]gfx.Rect(nil), baseInput.DirtyRegions...)}
			mutation.mutate(&changedInput)
			changed := testframe.Seal(changedInput)
			defer changed.Release()
			if digestFrame(t, base) == digestFrame(t, changed) {
				t.Fatalf("frame metadata field %s did not change digest", mutation.name)
			}
		})
	}
}

func TestCanonicalRejectsNonFiniteValues(t *testing.T) {
	if _, err := DigestCommands(gfx.CommandList{Commands: []gfx.Command{gfx.PushOpacity{Alpha: float32(math.NaN())}}}); err == nil {
		t.Fatal("non-finite command value must be rejected")
	}
	frame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{{Opacity: 1}}})
	defer frame.Release()
	if _, err := Digest(frame); err != nil {
		t.Fatalf("finite frame: %v", err)
	}
}

func TestNegativeOpacityDigest(t *testing.T) {
	frame := func(opacity float32) *render.Frame {
		return testframe.Seal(render.FrameInput{
			Layers: []render.LayerInput{{RenderOrder: 2, ClipRect: gfx.RectFromXYWH(0, 0, 10, 10), FirstBatch: 0, BatchCount: 1}},
			Batches: []render.BatchInput{{
				Bounds:  gfx.RectFromXYWH(1, 1, 8, 8),
				Opacity: opacity,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(1, 1, 8, 8), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			}},
		})
	}
	a, err := Digest(frame(1))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Digest(frame(.5))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("the negative control changed batch opacity without changing the canonical digest")
	}
}
