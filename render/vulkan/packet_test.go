package vulkan

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestEncodeFramePacket_v2Header(t *testing.T) {
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          7,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     0.75,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, G: 0.5, B: 0.25, A: 1})},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	if len(packet) == 0 {
		t.Fatal("expected a non-empty packet")
	}
	if got := string(packet[:4]); got != framePacketMagic {
		t.Fatalf("unexpected magic %q", got)
	}
	if got := binary.LittleEndian.Uint32(packet[4:8]); got != framePacketVersion {
		t.Fatalf("unexpected version %d", got)
	}

	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	if decoded.version != 2 {
		t.Fatalf("version = %d, want 2", decoded.version)
	}
	if len(decoded.batches) != 1 {
		t.Fatalf("batch count = %d, want 1", len(decoded.batches))
	}
	if decoded.trailing != 0 {
		t.Fatalf("trailing bytes = %d, want 0", decoded.trailing)
	}
}

func TestEncodeFramePacket_v2BatchHeader(t *testing.T) {
	transform := gfx.Transform{A: 2, B: 0, C: 0, D: 2, TX: 3, TY: 4}
	frame := &render.Frame{
		FramePacket: render.FramePacket{
			Layers: []render.LayeredBatch{
				{
					RenderOrder: 1,
					ClipRect:    gfx.RectFromXYWH(1, 2, 20, 30),
					Batches: []render.RenderBatch{
						{
							ID:      9,
							Bounds:  gfx.RectFromXYWH(0, 0, 50, 50),
							Opacity: 0.5,
							Commands: gfx.CommandList{Commands: []gfx.Command{
								gfx.PushTransform{Matrix: transform},
								gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
								gfx.PopTransform{},
							}},
						},
					},
				},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	if len(decoded.batches) != 1 {
		t.Fatalf("batch count = %d", len(decoded.batches))
	}
	b := decoded.batches[0]
	if b.id != 9 {
		t.Fatalf("batch id = %d, want 9", b.id)
	}
	if b.bounds != (gfx.RectFromXYWH(0, 0, 50, 50)) {
		t.Fatalf("bounds = %+v", b.bounds)
	}
	if b.opacity != 0.5 {
		t.Fatalf("opacity = %v", b.opacity)
	}
	// The wrapper transform must be lifted into the batch header and the
	// wrapper PushTransform/PopTransform commands dropped.
	if b.transform != transform {
		t.Fatalf("batch transform = %+v, want %+v", b.transform, transform)
	}
	if b.clip != (gfx.RectFromXYWH(1, 2, 20, 30)) {
		t.Fatalf("batch clip = %+v", b.clip)
	}
	if len(b.commands) != 1 {
		t.Fatalf("command count = %d, want 1 (wrapper stripped)", len(b.commands))
	}
}

func TestEncodeFramePacket_fullStrokeStyle(t *testing.T) {
	stroke := gfx.StrokeStyle{
		Width:      3,
		Cap:        gfx.LineCapRound,
		Join:       gfx.LineJoinBevel,
		MiterLimit: 4.5,
		Dash:       []float32{4, 2, 1},
		DashOffset: 1.5,
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.StrokePath{
						Path:   gfx.RectPath(gfx.RectFromXYWH(0, 0, 10, 10)),
						Stroke: stroke,
						Brush:  gfx.SolidBrush(gfx.Color{R: 0, G: 1, B: 0, A: 1}),
					},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmd := decoded.batches[0].commands[0]
	if cmd.kind != packetCmdStrokePath {
		t.Fatalf("opcode = %d, want %d", cmd.kind, packetCmdStrokePath)
	}
	if cmd.stroke.Width != 3 {
		t.Fatalf("width = %v", cmd.stroke.Width)
	}
	if cmd.stroke.Cap != gfx.LineCapRound {
		t.Fatalf("cap = %v", cmd.stroke.Cap)
	}
	if cmd.stroke.Join != gfx.LineJoinBevel {
		t.Fatalf("join = %v", cmd.stroke.Join)
	}
	if cmd.stroke.MiterLimit != 4.5 {
		t.Fatalf("miter = %v", cmd.stroke.MiterLimit)
	}
	if len(cmd.stroke.Dash) != 3 || cmd.stroke.Dash[1] != 2 {
		t.Fatalf("dash = %v", cmd.stroke.Dash)
	}
	if cmd.stroke.DashOffset != 1.5 {
		t.Fatalf("dash offset = %v", cmd.stroke.DashOffset)
	}
}

func TestEncodeFramePacket_gradientBrushNotDropped(t *testing.T) {
	brush := gfx.LinearGradientBrush(
		gfx.Point{X: 0, Y: 0},
		gfx.Point{X: 20, Y: 0},
		[]gfx.GradientStop{
			{Offset: 0, Color: gfx.Color{R: 1, A: 1}},
			{Offset: 0.5, Color: gfx.Color{G: 1, A: 1}},
			{Offset: 1, Color: gfx.Color{B: 1, A: 1}},
		},
	)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 32, 32),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 20, 20), Brush: brush},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmd := decoded.batches[0].commands[0]
	if cmd.kind != packetCmdFillRect {
		t.Fatalf("opcode = %d, want %d", cmd.kind, packetCmdFillRect)
	}
	if cmd.brush.Kind != gfx.BrushLinearGradient {
		t.Fatalf("brush kind = %v, want linear gradient (must not be silently dropped)", cmd.brush.Kind)
	}
	if len(cmd.brush.GradientStops) != 3 {
		t.Fatalf("stop count = %d, want 3", len(cmd.brush.GradientStops))
	}
	if cmd.brush.GradientEnd.X != 20 {
		t.Fatalf("gradient end = %+v", cmd.brush.GradientEnd)
	}
}

func TestEncodeFramePacket_drawTextureDecodes(t *testing.T) {
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawTexture{
						TextureID: 99,
						DestRect:  gfx.RectFromXYWH(1, 2, 8, 8),
						SrcRect:   gfx.RectFromXYWH(0, 0, 4, 4),
						Sampling:  gfx.SamplingBilinear,
						Opacity:   0.5,
					},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmd := decoded.batches[0].commands[0]
	if cmd.kind != packetCmdDrawTexture {
		t.Fatalf("opcode = %d, want %d (DrawTexture must be decodable)", cmd.kind, packetCmdDrawTexture)
	}
	if cmd.textureID != 99 {
		t.Fatalf("texture id = %d", cmd.textureID)
	}
	if cmd.sampling != 1 {
		t.Fatalf("sampling = %d", cmd.sampling)
	}
	if cmd.opacity != 0.5 {
		t.Fatalf("opacity = %v", cmd.opacity)
	}
}

func TestEncodeFramePacket_drawBlurredShadowDecodes(t *testing.T) {
	path := gfx.CirclePath(gfx.Point{X: 16, Y: 16}, 8)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawBlurredShadow{
						Path:       path,
						Color:      gfx.Color{R: 0, G: 0, B: 0, A: 0.5},
						BlurRadius: 8,
						Offset:     gfx.Point{X: 2, Y: 3},
						Inner:      true,
					},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmd := decoded.batches[0].commands[0]
	if cmd.kind != packetCmdDrawBlurredShadow {
		t.Fatalf("opcode = %d, want %d", cmd.kind, packetCmdDrawBlurredShadow)
	}
	if cmd.blurRadius != 8 {
		t.Fatalf("blur radius = %v", cmd.blurRadius)
	}
	if cmd.offset.X != 2 || cmd.offset.Y != 3 {
		t.Fatalf("offset = %+v", cmd.offset)
	}
	if !cmd.inner {
		t.Fatal("inner flag not preserved")
	}
	if len(cmd.path.Segments) != len(path.Segments) {
		t.Fatalf("path segment count = %d, want %d", len(cmd.path.Segments), len(path.Segments))
	}
}

func TestEncodeFramePacket_allOpcodesRoundTrip(t *testing.T) {
	gradient := gfx.LinearGradientBrush(
		gfx.Point{X: 0, Y: 0},
		gfx.Point{X: 10, Y: 0},
		[]gfx.GradientStop{
			{Offset: 0, Color: gfx.Color{R: 1, A: 1}},
			{Offset: 1, Color: gfx.Color{B: 1, A: 1}},
		},
	)
	stroke := gfx.DefaultStroke(2)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 128, 128),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
					gfx.StrokeRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Stroke: stroke, Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
					gfx.FillPath{Path: gfx.RectPath(gfx.RectFromXYWH(0, 0, 10, 10)), Brush: gradient},
					gfx.StrokePath{Path: gfx.RectPath(gfx.RectFromXYWH(0, 0, 10, 10)), Stroke: stroke, Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
					gfx.DrawPolyline{Points: []gfx.Point{{X: 0, Y: 0}, {X: 10, Y: 10}}, Stroke: stroke, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1}), Closed: true},
					gfx.DrawPoints{Points: []gfx.Point{{X: 5, Y: 5}}, Radius: 3, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
					gfx.DrawSelectionRects{Rects: []gfx.Rect{gfx.RectFromXYWH(0, 0, 5, 5)}, Brush: gfx.SolidBrush(gfx.Color{R: 1, G: 0.5, A: 0.5})},
					gfx.PushTransform{Matrix: gfx.Rotation(0.5)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 4, 4), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
					gfx.PopTransform{},
					gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 8, 8)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 4, 4), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
					gfx.PopClip{},
					gfx.PushOpacity{Alpha: 0.5},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 4, 4), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
					gfx.PopOpacity{},
					gfx.DrawTexture{TextureID: 7, DestRect: gfx.RectFromXYWH(0, 0, 4, 4), SrcRect: gfx.RectFromXYWH(0, 0, 4, 4)},
					gfx.DrawBlurredShadow{Path: gfx.RectPath(gfx.RectFromXYWH(0, 0, 4, 4)), Color: gfx.Color{A: 0.25}, BlurRadius: 4},
					gfx.BeginRenderBatch{Bounds: gfx.RectFromXYWH(0, 0, 4, 4), CacheID: 42},
					gfx.EndRenderBatch{},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmds := decoded.batches[0].commands
	wantKinds := []uint8{
		packetCmdFillRect,
		packetCmdStrokeRect,
		packetCmdFillPath,
		packetCmdStrokePath,
		packetCmdDrawPolyline,
		packetCmdDrawPoints,
		packetCmdDrawSelectionRects,
		packetCmdPushTransform,
		packetCmdFillRect,
		packetCmdPopTransform,
		packetCmdPushClipRect,
		packetCmdFillRect,
		packetCmdPopClip,
		packetCmdPushOpacity,
		packetCmdFillRect,
		packetCmdPopOpacity,
		packetCmdDrawTexture,
		packetCmdDrawBlurredShadow,
		packetCmdBeginRenderBatch,
		packetCmdEndRenderBatch,
	}
	if len(cmds) != len(wantKinds) {
		t.Fatalf("command count = %d, want %d", len(cmds), len(wantKinds))
	}
	for i, want := range wantKinds {
		if cmds[i].kind != want {
			t.Fatalf("command %d opcode = %d, want %d", i, cmds[i].kind, want)
		}
	}
}

func TestEncodeFramePacket_drawGlyphRun_preserves_origin_and_glyph_positions(t *testing.T) {
	if _, err := Version(); err != nil {
		t.Skip("vulkan FFI not available:", err)
	}
	reg := fontdata.TestFontRegistry(t)
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 18})
	run := text.GlyphRun{
		Face:  face,
		Size:  18,
		Style: text.TextStyle{Family: "Noto Sans", Size: 18},
		Glyphs: []text.PositionedGlyph{
			{GlyphID: 65, X: 3.5, Y: 4.25, Advance: 7.5},
		},
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          7,
				Bounds:      gfx.RectFromXYWH(0, 0, 64, 64),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{
						Run:    run,
						Origin: gfx.Point{X: 12.5, Y: 20.25},
						Brush:  gfx.SolidBrush(gfx.Color{R: 1, A: 1}),
					},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmd := decoded.batches[0].commands[0]
	if cmd.kind != packetCmdDrawGlyphRun {
		t.Fatalf("opcode = %d", cmd.kind)
	}
	if cmd.fontID != face.CacheKey() {
		t.Fatalf("font id = %d", cmd.fontID)
	}
	if math.Float32frombits(cmd.sizeBits) != 18 {
		t.Fatalf("size bits = %v", cmd.sizeBits)
	}
	if cmd.origin.X != 12.5 || cmd.origin.Y != 20.25 {
		t.Fatalf("origin = %+v", cmd.origin)
	}
	if len(cmd.glyphs) != 1 || cmd.glyphs[0].glyphID != 65 || cmd.glyphs[0].x != 3.5 {
		t.Fatalf("glyphs = %+v", cmd.glyphs)
	}
}

func TestEncodeFramePacket_drawImageBatch(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	uploader := &fakeImageUploader{handle: 99}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      7,
				Bounds:  gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawImage{
						Image:    img,
						DestRect: gfx.RectFromXYWH(1, 2, 3, 4),
						SrcRect:  gfx.RectFromXYWH(0, 0, 1, 1),
						Sampling: gfx.SamplingBilinear,
						Opacity:  0.5,
					},
				}},
			},
		},
	}

	packet, err := encodeFramePacketWithAssets(frame, uploader)
	if err != nil {
		t.Fatalf("encodeFramePacketWithAssets: %v", err)
	}
	if uploader.calls != 1 {
		t.Fatalf("expected one image upload, got %d", uploader.calls)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	cmd := decoded.batches[0].commands[0]
	if cmd.kind != packetCmdDrawImage {
		t.Fatalf("opcode = %d", cmd.kind)
	}
	if cmd.handle != 99 {
		t.Fatalf("handle = %d", cmd.handle)
	}
	if cmd.sampling != 1 {
		t.Fatalf("sampling = %d", cmd.sampling)
	}
}

func TestEncodeFramePacket_primitiveIconCommands(t *testing.T) {
	tokens := theme.DefaultTokens()
	tokens.Color.Primary = gfx.ColorFromRGBA8(90, 40, 200, 255)
	rt := iconPacketRuntime{rootStyle: theme.NewRootStyleContext(nil, tokens, nil)}
	icon := primitive.NewIcon(primitive.IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="currentColor"><path d="M1 1H9V9H1Z"/></svg>`))
	icon.ColorSlot = marks.Const(theme.ColorPrimary)
	facet.Attach(icon, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	size := icon.Base().LayoutRole().Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 64, H: 64}}).Size
	bounds := gfx.RectFromXYWH(0, 0, size.W, size.H)
	icon.Base().LayoutRole().Arrange(facet.ArrangeContext{}, bounds)
	cmds := icon.Base().ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected icon commands")
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          7,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    *cmds,
			},
		},
	}
	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	decoded, err := decodeTestFramePacket(packet)
	if err != nil {
		t.Fatalf("decodeTestFramePacket: %v", err)
	}
	if len(decoded.batches) != 1 {
		t.Fatalf("batch count = %d", len(decoded.batches))
	}
	if !containsOpcode(packet, packetCmdFillPath) {
		t.Fatalf("expected icon packet to include fill path opcode, got %v", packet)
	}
}

type fakeImageUploader struct {
	handle uint64
	calls  int
}

func (s *fakeImageUploader) ensureImage(img *image.RGBA) (uint64, error) {
	s.calls++
	return s.handle, nil
}

type iconPacketRuntime struct {
	rootStyle any
}

func (s iconPacketRuntime) Schedule(j job.AnyJob)  {}
func (s iconPacketRuntime) CancelJob(id job.JobID) {}
func (s iconPacketRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s iconPacketRuntime) RootStyleContext() any { return s.rootStyle }
func (s iconPacketRuntime) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}

func containsOpcode(packet []byte, opcode uint8) bool {
	for _, b := range packet {
		if b == opcode {
			return true
		}
	}
	return false
}
