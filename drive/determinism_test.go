package drive_test

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// TestDeterminism_VirtualReplay is the NFR-6 proof: the same studio seed +
// the same command script in Virtual mode yields identical frame packet
// hashes across two full runs. The script interleaves time advances with a
// deterministic input timeline (pointer moves that trigger hover settle on
// virtual time, periodic clicks on stage chrome).
func TestDeterminism_VirtualReplay(t *testing.T) {
	// NFR-6 demands 1000 consecutive frames. Under the race detector the
	// studio render inflates ~13x (a single 1000-frame replay costs ~8
	// minutes), so the race suite replays a shorter prefix of the same
	// script — same two-run comparison, same teeth; the full-length proof
	// runs in every non-race suite (the convention the studio's alive-
	// coverage walk already uses, testkit.RaceEnabled).
	advances := 1000
	if testkit.RaceEnabled {
		advances = 120
	}

	first := replayStudioSession(t, advances)
	second := replayStudioSession(t, advances)

	if len(first) != len(second) {
		t.Fatalf("hash sequence lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("frame packet hash diverged at frame %d: %#x vs %#x", i, first[i], second[i])
		}
	}
}

// replayStudioSession boots the studio headless on the testkit runtime with
// the drive sink installed, switches to Virtual time, and replays the fixed
// script, returning the packet hash of every executed frame.
func replayStudioSession(t *testing.T, advances int) []uint64 {
	t.Helper()

	fonts := testkit.TestFontRegistry(t)
	reg, err := studio.StudioLayerRegistry()
	if err != nil {
		t.Fatalf("studio layer registry: %v", err)
	}
	themeCtx := studio.StudioThemeContext().WithFontRegistry(fonts)
	root := studio.BuildRoot(app.BuildContext{
		FontRegistry: fonts,
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        themeCtx,
	}, studio.NewDirtySink(10), loadDeterminismSeed(t), reg)

	cfg := testkit.StandardHarnessConfig(t, 1280, 800)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = themeCtx.Resolver
	cfg.Drive = true
	cfg.Clock = runtime.ClockConfig{InitialMode: runtime.ClockVirtual, EpochNS: 1_800_000_000_000_000_000}
	h := testkit.NewHarness(t, cfg, root)
	h.RunFrame() // deterministic bootstrap transition at the configured epoch

	hashes := make([]uint64, 0, advances*2)
	framePeriod := map[string]string{"duration_ns": "16667000"}

	for i := 0; i < advances; i++ {
		// Deterministic input timeline: a pointer move every 61 advances
		// (hover settle then fires on virtual time), a click every 211.
		switch {
		case i%211 == 100:
			h.InjectEvent(testkit.PointerPress(640, 60, platform.PointerLeft))
		case i%211 == 101:
			h.InjectEvent(testkit.PointerRelease(640, 60, platform.PointerLeft))
		case i%61 == 30:
			h.InjectEvent(testkit.PointerMove(320+float32(i%97), 200+float32(i%53)))
		}

		// Each explicit token executes one frame and returns only after its
		// ready surface acknowledgment.
		if _, perr := h.DriveCall(t, "time.advance", framePeriod); perr != nil {
			t.Fatalf("advance %d: %v", i, perr)
		}
		hashes = append(hashes, h.Runtime().FramePacketHash())
	}
	return hashes
}

// loadDeterminismSeed reads the studio's committed metrics snapshot — the
// same file LURPIC_SEED pins for live runs (loadSeed honors it in main.go).
func loadDeterminismSeed(t *testing.T) []dataset.Row {
	t.Helper()
	path := filepath.Join("..", "demos", "lurpic_studio", "assets", "metrics.csv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seed %s: %v", path, err)
	}
	rows, err := dataset.Parse(data)
	if err != nil {
		t.Fatalf("parse seed: %v", err)
	}
	return rows
}
