package studio

// Temporary visual-audit driver (not part of the test suite): renders the
// shell's every exhibit, overlay, and responsive arrangement to PNGs under
// LURPIC_AUDIT_OUT. Skipped unless LURPIC_AUDIT_OUT is set. Deleted after the
// audit.

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

func TestZZAuditRender(t *testing.T) {
	out := os.Getenv("LURPIC_AUDIT_OUT")
	if out == "" {
		t.Skip("LURPIC_AUDIT_OUT not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}
	save := func(h *testkit.Harness, name string) {
		t.Helper()
		f, err := os.Create(filepath.Join(out, name+".png"))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, h.Surface().Capture()); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}
	stream := func(h *testkit.Harness, frames int) {
		t.Helper()
		base := time.Now()
		for i := 0; i < frames; i++ {
			h.Runtime().SetFrameClock(base.Add(time.Duration(i) * DefaultFeedCadence))
			h.RunFrame()
		}
	}

	// ---- Wide 1280x800 ----
	root, h := newResponsiveShell(t, 1280, 800)
	stream(h, 30)
	save(h, "wide_01_realtime")

	for _, id := range []ExhibitID{ExhibitCapabilities, ExhibitLayers, ExhibitAnchors, ExhibitPolicies, ExhibitPropagation} {
		root.Shell().ActiveExhibit.Set(id)
		h.RunFrame()
		h.RunFrame()
		save(h, "wide_02_"+string(id))
	}

	// E2 modal open.
	root.Shell().ActiveExhibit.Set(ExhibitLayers)
	h.RunFrame()
	if layers, ok := root.Stage().RootFor(ExhibitLayers).(*Layers); ok {
		layers.ModalOpen().Set(true)
		h.RunFrame()
		h.RunFrame()
		save(h, "wide_03_layers_modal")
		layers.ModalOpen().Set(false)
	} else {
		t.Log("layers facet not reachable")
	}

	// E6 playground, every family tab.
	root.Shell().ActiveExhibit.Set(ExhibitPlayground)
	h.RunFrame()
	pg, ok := root.Stage().RootFor(ExhibitPlayground).(*Playground)
	if !ok {
		t.Fatal("playground facet not reachable")
	}
	for i := 0; i < 6; i++ {
		pg.ActiveTab().Set(i)
		h.RunFrame()
		h.RunFrame()
		save(h, fmt.Sprintf("wide_04_playground_tab%d", i))
	}

	// Command palette open (over the playground).
	root.Shell().CommandOpen.Set(true)
	h.RunFrame()
	h.RunFrame()
	save(h, "wide_05_palette")
	root.Shell().CommandOpen.Set(false)

	// Compact density over realtime.
	root.Shell().ActiveExhibit.Set(ExhibitRealtime)
	root.Shell().Compact.Set(true)
	h.RunFrame()
	h.RunFrame()
	stream(h, 5)
	save(h, "wide_06_compact")
	root.Shell().Compact.Set(false)

	// ---- Narrow 720x800 ----
	nroot, nh := newResponsiveShell(t, 720, 800)
	stream(nh, 10)
	save(nh, "narrow_01_realtime")

	nroot.Shell().IndexOpen.Set(true)
	nh.RunFrame()
	nh.RunFrame()
	save(nh, "narrow_02_drawer")
	nroot.Shell().IndexOpen.Set(false)

	nroot.Shell().InspectorOpen.Set(true)
	nh.RunFrame()
	nh.RunFrame()
	save(nh, "narrow_03_sheet")
	nroot.Shell().InspectorOpen.Set(false)

	nroot.Shell().ActiveExhibit.Set(ExhibitPlayground)
	nh.RunFrame()
	nh.RunFrame()
	save(nh, "narrow_04_playground")

	nroot.Shell().ActiveExhibit.Set(ExhibitCapabilities)
	nh.RunFrame()
	nh.RunFrame()
	save(nh, "narrow_05_capabilities")
}
