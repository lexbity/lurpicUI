package studio

import (
	"fmt"
	"testing"
)

func TestDbgE2Wave(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitLayers)
	h.RunFrame()
	h.RunFrame()
	e2 := stage.ActiveRoot().(*Layers)
	e2.ModalOpen().Set(true)
	h.RunFrame()
	snap, ok := sink.Latest()
	fmt.Printf("ok=%v count=%d scrimID=%d\n", ok, len(snap.Dirty), e2.scrim.Base().ID())
	for id, flags := range snap.Dirty {
		fmt.Printf("  id=%d flags=%v\n", id, flags)
	}
}
