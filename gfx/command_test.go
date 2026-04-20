package gfx

import "testing"

func TestCommandList_add_and_len(t *testing.T) {
	var cl CommandList
	cl.Add(FillRect{})
	cl.Add(PopClip{})
	if got := cl.Len(); got != 2 {
		t.Fatalf("expected len 2, got %d", got)
	}
	cl.Reset()
	if got := cl.Len(); got != 0 {
		t.Fatalf("expected len 0 after reset, got %d", got)
	}
}

func TestCommandList_type_preservation(t *testing.T) {
	var cl CommandList
	cl.Add(FillRect{Rect: RectFromXYWH(1, 2, 3, 4)})
	cl.Add(DrawPoints{Radius: 5})

	if _, ok := cl.Commands[0].(FillRect); !ok {
		t.Fatalf("expected first command to be FillRect, got %T", cl.Commands[0])
	}
	if _, ok := cl.Commands[1].(DrawPoints); !ok {
		t.Fatalf("expected second command to be DrawPoints, got %T", cl.Commands[1])
	}
}

func TestBeginEndRenderBatch_symmetry(t *testing.T) {
	cmds := []Command{
		BeginRenderBatch{CacheID: 1},
		PushOpacity{Alpha: 0.5},
		BeginRenderBatch{CacheID: 2},
		EndRenderBatch{},
		EndRenderBatch{},
	}

	depth := 0
	for _, cmd := range cmds {
		switch cmd.(type) {
		case BeginRenderBatch:
			depth++
		case EndRenderBatch:
			depth--
		}
		if depth < 0 {
			t.Fatal("RenderBatch stack underflow")
		}
	}
	if depth != 0 {
		t.Fatalf("expected RenderBatch stack to end balanced, got depth %d", depth)
	}
}
