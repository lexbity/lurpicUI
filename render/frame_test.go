package render_test

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/canonical"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestFrameCommandViewsCannotMutatePage(t *testing.T) {
	stops := []gfx.GradientStop{{Offset: 0, Color: gfx.Color{R: 1, A: 1}}, {Offset: 1, Color: gfx.Color{B: 1, A: 1}}}
	brush := gfx.LinearGradientBrush(gfx.Point{}, gfx.Point{X: 20}, stops)
	path := gfx.RectPath(gfx.RectFromXYWH(0, 0, 20, 20))
	stroke := gfx.DefaultStroke(2)
	stroke.Dash = []float32{2, 3}
	commands := []gfx.Command{
		gfx.FillRect{Brush: brush}, gfx.StrokeRect{Brush: brush, Stroke: stroke},
		gfx.FillPath{Path: path, Brush: brush}, gfx.StrokePath{Path: path, Brush: brush, Stroke: stroke},
		gfx.DrawPolyline{Points: []gfx.Point{{X: 1}}, Stroke: stroke, Brush: brush},
		gfx.DrawPoints{Points: []gfx.Point{{Y: 2}}, Brush: brush},
		gfx.DrawGlyphRun{Run: text.GlyphRun{Glyphs: []text.PositionedGlyph{{GlyphID: 1}}, GraphemeAdvances: []float32{2}, Text: "x"}, Brush: brush},
		gfx.DrawSelectionRects{Rects: []gfx.Rect{gfx.RectFromXYWH(0, 0, 2, 3)}, Brush: brush},
		gfx.DrawBlurredShadow{Path: path},
	}
	input := render.FrameInput{Batches: []render.BatchInput{{Commands: gfx.CommandList{Commands: commands}}}}
	_, reservation := reserveFrame(t, input, 0)
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Release()
	before, err := canonical.Digest(frame)
	if err != nil {
		t.Fatal(err)
	}
	stops[0].Color.R = .25
	path.Segments[0].Pts[0].X = 999
	stroke.Dash[0] = 999
	frame.VisitBatchCommands(0, func(command gfx.CommandView) bool {
		var check func(reflect.Value)
		check = func(value reflect.Value) {
			if value.Kind() == reflect.Slice && !value.IsNil() {
				t.Fatalf("scalar accessor leaked writable slice %s", value.Type())
			}
			if value.Kind() == reflect.Struct {
				for i := 0; i < value.NumField(); i++ {
					if value.Type().Field(i).IsExported() {
						check(value.Field(i))
					}
				}
			}
		}
		check(reflect.ValueOf(command.Scalars()))
		if command.Points().Len() > 0 {
			point := command.Points().At(0)
			point.X = 999
		}
		if command.Path().Segments.Len() > 0 {
			segment := command.Path().Segments.At(0)
			segment.Pts[0].X = 999
		}
		if command.Brush().GradientStops.Len() > 0 {
			stop := command.Brush().GradientStops.At(0)
			stop.Offset = 999
		}
		return true
	})
	after, err := canonical.Digest(frame)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("source or reader mutation changed sealed command page")
	}
	if allocations := testing.AllocsPerRun(100, func() {
		frame.VisitBatchCommands(0, func(command gfx.CommandView) bool { _ = command.Scalars(); return true })
	}); allocations != 0 {
		t.Fatalf("immutable reads allocate: %v", allocations)
	}
}

func TestFrameReservationIncludesNestedCommandPayloads(t *testing.T) {
	const elements = 4096
	brush := gfx.LinearGradientBrush(gfx.Point{}, gfx.Point{X: 1}, make([]gfx.GradientStop, elements))
	path := gfx.Path{Segments: make([]gfx.PathSegment, elements)}
	stroke := gfx.DefaultStroke(1)
	stroke.Dash = make([]float32, elements)
	for name, command := range map[string]gfx.Command{
		"fill-gradient":        gfx.FillRect{Brush: brush},
		"stroke-gradient-dash": gfx.StrokeRect{Brush: brush, Stroke: stroke},
		"fill-path":            gfx.FillPath{Path: path, Brush: brush},
		"stroke-path":          gfx.StrokePath{Path: path, Brush: brush, Stroke: stroke},
		"glyph-gradient":       gfx.DrawGlyphRun{Brush: brush},
	} {
		t.Run(name, func(t *testing.T) {
			bytes, err := gfx.EstimateCommandPageBytes([]gfx.Command{command})
			if err != nil {
				t.Fatal(err)
			}
			if bytes < elements*20 {
				t.Fatalf("nested backing undercharged: %d bytes", bytes)
			}
		})
	}
}

func reserveFrame(t *testing.T, input render.FrameInput, extra uint64) (*membudget.Wallet, *membudget.Reservation) {
	t.Helper()
	bytes, err := render.EstimateFrameBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	if bytes == 0 {
		bytes = 1
	}
	wallet, err := membudget.NewWallet(bytes + extra)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := wallet.Reserve(bytes + extra)
	if err != nil {
		t.Fatal(err)
	}
	return wallet, reservation
}

func TestFrameLayerRangesPreserveBatchOrder(t *testing.T) {
	input := render.FrameInput{
		Batches: []render.BatchInput{{ID: 1}, {ID: 2}, {ID: 3}},
		Layers: []render.LayerInput{
			{RenderOrder: 1000, ClipRect: gfx.RectFromXYWH(0, 0, 10, 10), FirstBatch: 0, BatchCount: 1},
			{RenderOrder: 7000, FirstBatch: 1, BatchCount: 2},
		},
	}
	_, reservation := reserveFrame(t, input, 0)
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Release()
	if frame.LayerCount() != 2 {
		t.Fatalf("layers=%d want 2", frame.LayerCount())
	}
	first, ok := frame.LayerAt(0)
	if !ok || first.RenderOrder != 1000 || first.ClipRect.Width() != 10 || first.FirstBatch != 0 || first.BatchCount != 1 {
		t.Fatalf("base layer=%+v", first)
	}
	second, ok := frame.LayerAt(1)
	if !ok || second.RenderOrder != 7000 || second.FirstBatch != 1 || second.BatchCount != 2 {
		t.Fatalf("modal layer=%+v", second)
	}
	if frame.BatchCount() != 3 {
		t.Fatalf("batches=%d want 3", frame.BatchCount())
	}
}

func TestFrameBatchMetadataAndCommands(t *testing.T) {
	input := render.FrameInput{Batches: []render.BatchInput{{ID: 42, Bounds: gfx.RectFromXYWH(5, 6, 20, 30), Opacity: .5, Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{Rect: gfx.RectFromXYWH(5, 6, 20, 30)}}}}}}
	_, reservation := reserveFrame(t, input, 0)
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Release()
	b, ok := frame.BatchAt(0)
	if !ok || b.ID != 42 || b.Bounds.Width() != 20 || b.Bounds.Height() != 30 || b.Opacity != .5 {
		t.Fatalf("batch=%+v", b)
	}
	count := 0
	if !frame.VisitBatchCommands(0, func(gfx.CommandView) bool { count++; return true }) || count != 1 {
		t.Fatalf("command count=%d", count)
	}
}

func TestFramePagesWaitForAllConsumers(t *testing.T) {
	points := []gfx.Point{{X: 2, Y: 3}}
	input := render.FrameInput{Batches: []render.BatchInput{{ID: 1, Commands: gfx.CommandList{Commands: []gfx.Command{gfx.DrawPolyline{Points: points}}}}}}
	wallet, reservation := reserveFrame(t, input, 0)
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := frame.Retain()
	if err != nil {
		t.Fatal(err)
	}
	points[0].X = 99
	input.Batches[0].ID = 2
	var x float32
	frame.VisitBatchCommands(0, func(command gfx.CommandView) bool { x = command.Points().At(0).X; return false })
	if x != 2 {
		t.Fatalf("sealed command changed with input: x=%v", x)
	}
	if batch, _ := retained.BatchAt(0); batch.ID != 1 {
		t.Fatalf("sealed metadata changed with input: %+v", batch)
	}
	if err := frame.Release(); err != nil {
		t.Fatal(err)
	}
	_, _, live, retiring := wallet.Snapshot()
	if live == 0 || retiring != 0 {
		t.Fatalf("charge before final lease release live=%d retiring=%d", live, retiring)
	}
	if err := retained.Release(); err != nil {
		t.Fatal(err)
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("charge after final release = reserved %d live %d retiring %d", reserved, live, retiring)
	}
	if err := retained.Release(); err == nil {
		t.Fatal("duplicate lease release succeeded")
	}
}

func TestFramePagesRejectLateGenerationRelease(t *testing.T) {
	wallet, err := membudget.NewWallet(4096)
	if err != nil {
		t.Fatal(err)
	}
	seal := func() *render.Frame {
		input := render.FrameInput{}
		bytes, err := render.EstimateFrameBytes(input)
		if err != nil {
			t.Fatal(err)
		}
		reservation, err := wallet.Reserve(bytes + 1)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := render.SealFrame(input, reservation)
		if err != nil {
			_ = reservation.Cancel()
			t.Fatal(err)
		}
		return frame
	}
	old := seal()
	oldGeneration := old.Generation()
	if err := old.Release(); err != nil {
		t.Fatal(err)
	}
	newer := seal()
	if newer.Generation() <= oldGeneration {
		t.Fatalf("resource generation reused: old=%d new=%d", oldGeneration, newer.Generation())
	}
	if err := old.Release(); err == nil {
		t.Fatal("late release of old generation succeeded")
	}
	_, _, live, _ := wallet.Snapshot()
	if live != newer.BackingBytes() {
		t.Fatalf("late release changed replacement charge: live=%d newer=%d", live, newer.BackingBytes())
	}
	if err := newer.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestFrameRefusesInsufficientReservation(t *testing.T) {
	input := render.FrameInput{Batches: []render.BatchInput{{Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{}}}}}}
	wallet, err := membudget.NewWallet(1)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := wallet.Reserve(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := render.SealFrame(input, reservation); err == nil {
		t.Fatal("undersized reservation accepted")
	}
	if err := reservation.Cancel(); err != nil {
		t.Fatal(err)
	}
}

func TestFrameReleasedAccessIsUnavailable(t *testing.T) {
	input := render.FrameInput{}
	_, reservation := reserveFrame(t, input, 0)
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	if err := frame.Release(); err != nil {
		t.Fatal(err)
	}
	if _, ok := frame.BatchAt(0); ok {
		t.Fatal("released frame returned batch metadata")
	}
	if _, err := frame.Retain(); err == nil {
		t.Fatal("released frame retained")
	}
}

func TestFrameLeaseCountIsBoundedByReservedMetadata(t *testing.T) {
	input := render.FrameInput{}
	wallet, reservation := reserveFrame(t, input, 0)
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	leases := []*render.Frame{frame}
	for len(leases) < 8 {
		retained, err := frame.Retain()
		if err != nil {
			t.Fatalf("retain %d: %v", len(leases)+1, err)
		}
		leases = append(leases, retained)
	}
	if _, err := frame.Retain(); err == nil {
		t.Fatal("frame lease limit was exceeded")
	}
	for _, lease := range leases {
		if err := lease.Release(); err != nil {
			t.Fatal(err)
		}
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("bounded leases left wallet charges: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}
