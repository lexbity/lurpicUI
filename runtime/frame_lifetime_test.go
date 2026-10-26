package runtime

import (
	"errors"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestFailedAndLateFrameAcknowledgmentsPreserveLastReady(t *testing.T) {
	wallet, err := membudget.NewWallet(4096)
	if err != nil {
		t.Fatal(err)
	}
	seal := func(id render.RenderBatchID) *render.Frame {
		t.Helper()
		input := render.FrameInput{Batches: []render.BatchInput{{ID: id}}}
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

	ready := seal(1)
	pending := seal(2)
	rt := &Runtime{
		lastWindowFrames:  map[string]*render.Frame{"__primary__": ready},
		pendingFrameViews: map[uint64]*render.Frame{12: pending},
	}
	rt.completeFrameView(RenderCompletion{Frame: 12, ResourceGeneration: ready.Generation(), Ready: true})
	if rt.pendingFrameViews[12] != pending || rt.lastWindowFrames["__primary__"] != ready {
		t.Fatal("stale acknowledgment changed pending or last-ready frame")
	}
	rt.completeFrameView(RenderCompletion{Frame: 12, ResourceGeneration: pending.Generation(), Err: errors.New("submit failed")})
	if rt.pendingFrameViews[12] != nil || rt.lastWindowFrames["__primary__"] != ready {
		t.Fatal("failed frame replaced the last ready surface")
	}
	if pending.BatchCount() != 0 {
		t.Fatal("failed pending frame lease was not released")
	}

	next := seal(3)
	rt.pendingFrameViews[13] = next
	rt.completeFrameView(RenderCompletion{Frame: 13, ResourceGeneration: next.Generation(), Ready: true})
	if rt.lastWindowFrames["__primary__"] != next || ready.BatchCount() != 0 {
		t.Fatal("ready generation did not replace and release the prior surface")
	}
	rt.releaseFrameViews()
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("frame view cleanup left wallet charges: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}
