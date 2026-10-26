package runtime

import (
	"errors"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestPacketCapacityRefusedWithoutPartialPublish(t *testing.T) {
	rt := &Runtime{frameStorage: testFrameStorage(t, 4096, 1)}
	input := render.FrameInput{Batches: []render.BatchInput{{Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{}}}}}}
	if _, err := rt.sealFrameGroup(map[string]render.FrameInput{render.PrimaryWindow: input}, false); err == nil {
		t.Fatal("packet larger than configured cap was sealed")
	}
	_, reserved, live, retiring := rt.frameStorage.wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("packet refusal partially published storage: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}

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
	pending := testframe.Group(seal(2))
	rt := &Runtime{
		lastWindowFrames:  map[string]*render.Frame{"__primary__": ready},
		pendingFrameViews: map[uint64]*render.FrameGroup{12: pending},
	}
	rt.completeFrameView(RenderCompletion{Frame: 12, ResourceGeneration: ready.Generation(), Ready: true})
	if rt.pendingFrameViews[12] != pending || rt.lastWindowFrames["__primary__"] != ready {
		t.Fatal("stale acknowledgment changed pending or last-ready frame")
	}
	rt.completeFrameView(RenderCompletion{Frame: 12, ResourceGeneration: pending.Generation(), Err: errors.New("submit failed")})
	if rt.pendingFrameViews[12] != nil || rt.lastWindowFrames["__primary__"] != ready {
		t.Fatal("failed frame replaced the last ready surface")
	}
	if pending.WindowCount() != 0 {
		t.Fatal("failed pending frame lease was not released")
	}

	next := testframe.Group(seal(3))
	rt.pendingFrameViews[13] = next
	descriptor, _ := next.WindowAt(0)
	rt.completeFrameView(RenderCompletion{Frame: 13, ResourceGeneration: next.Generation(), Ready: true, WindowCount: 1, Windows: [render.MaxFrameWindows]WindowCompletion{{WindowDescriptor: descriptor, Ready: true}}})
	if rt.lastWindowFrames["__primary__"].Generation() != descriptor.ResourceGeneration || ready.BatchCount() != 0 {
		t.Fatal("ready generation did not replace and release the prior surface")
	}
	rt.releaseFrameViews()
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("frame view cleanup left wallet charges: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}
