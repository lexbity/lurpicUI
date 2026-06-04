package vulkan

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestEncodeFramePacket_solidRectBatch(t *testing.T) {
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
	if got := binary.LittleEndian.Uint32(packet[8:12]); got != 1 {
		t.Fatalf("unexpected batch count %d", got)
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
	if got := packet[44]; got != packetCmdDrawImage {
		t.Fatalf("unexpected opcode %d", got)
	}
	if got := binary.LittleEndian.Uint64(packet[45:53]); got != uploader.handle {
		t.Fatalf("unexpected image handle %d", got)
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
	if got := binary.LittleEndian.Uint32(packet[8:12]); got != 1 {
		t.Fatalf("unexpected batch count %d", got)
	}
	if !containsOpcode(packet, packetCmdFillPath) {
		t.Fatalf("expected icon packet to include fill path opcode, got %v", packet)
	}
}

func TestEncodeFramePacket_drawGlyphRun_preserves_origin_and_glyph_positions(t *testing.T) {
	if _, err := Version(); err != nil {
		t.Skip("vulkan FFI not available:", err)
	}
	reg := mustPacketFontRegistry(t)
	data := mustPacketTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
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
	if got := binary.LittleEndian.Uint32(packet[8:12]); got != 1 {
		t.Fatalf("unexpected batch count %d", got)
	}
	if got := packet[44]; got != packetCmdDrawGlyphRun {
		t.Fatalf("unexpected opcode %d", got)
	}
	if got := binary.LittleEndian.Uint64(packet[45:53]); got != face.CacheKey() {
		t.Fatalf("unexpected face key %d", got)
	}
	if got := math32(packet[53:57]); got != 18 {
		t.Fatalf("unexpected size bits %v", got)
	}
	if got := math32(packet[57:61]); got != 12.5 {
		t.Fatalf("unexpected origin x %v", got)
	}
	if got := math32(packet[61:65]); got != 20.25 {
		t.Fatalf("unexpected origin y %v", got)
	}
	if got := binary.LittleEndian.Uint32(packet[81:85]); got != 1 {
		t.Fatalf("unexpected glyph count %d", got)
	}
	if got := binary.LittleEndian.Uint32(packet[85:89]); got != 65 {
		t.Fatalf("unexpected glyph id %d", got)
	}
	if got := math32(packet[89:93]); got != 3.5 {
		t.Fatalf("unexpected glyph x %v", got)
	}
	if got := math32(packet[93:97]); got != 4.25 {
		t.Fatalf("unexpected glyph y %v", got)
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

func math32(b []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}

func mustPacketFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	return reg
}

func mustPacketTestFont(t *testing.T, rel string) []byte {
	t.Helper()
	path := mustPacketTestFontPath(t, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test font %q: %v", path, err)
	}
	return data
}

func mustPacketTestFontPath(t *testing.T, rel string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	return filepath.Join(string(bytes.TrimSpace(out)), rel)
}
