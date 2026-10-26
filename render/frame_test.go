package render_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
)

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
	if !frame.VisitBatchCommands(0, func(gfx.Command) bool { count++; return true }) || count != 1 {
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
	frame.VisitBatchCommands(0, func(command gfx.Command) bool { x = command.(gfx.DrawPolyline).Points[0].X; return false })
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
