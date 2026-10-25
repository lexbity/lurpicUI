package drive_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/render/canonical"
	"codeburg.org/lexbit/lurpicui/runtime"
)

type replayFrame struct {
	frame   uint64
	packet  string
	surface string
}

// replayStudioSession boots the studio headless on the testkit runtime with
// the drive sink installed, switches to Virtual time, and replays the fixed
// script, returning the packet hash of every executed frame.
func replayStudioSession(t *testing.T, advances int) []replayFrame {
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

	frames := make([]replayFrame, 0, advances)
	framePeriod := map[string]string{"duration_ns": "16667000"}

	for i := 0; i < advances; i++ {
		// Each explicit token executes one frame and returns only after its
		// ready surface acknowledgment.
		if _, perr := h.DriveCall(t, "time.advance", framePeriod); perr != nil {
			t.Fatalf("advance %d: %v", i, perr)
		}
		frameNumber := h.Runtime().LastFrameStats().FrameNumber
		if frameNumber != uint64(i+2) {
			t.Fatalf("advance %d produced ready frame %d, want exactly %d", i, frameNumber, i+2)
		}
		packet := h.Runtime().LastPrimaryFrame()
		if packet == nil {
			t.Fatalf("advance %d produced no primary ready frame", i)
		}
		packetDigest, err := canonical.Digest(packet)
		if err != nil {
			t.Fatalf("advance %d canonical packet: %v", i, err)
		}
		pixels := h.Surface().Capture()
		frames = append(frames, replayFrame{frame: frameNumber, packet: packetDigest, surface: fmt.Sprintf("%x", sha256.Sum256(pixels.Pix))})
	}
	return frames
}

func assertReplayEqual(t *testing.T, first, second []replayFrame, want int) {
	t.Helper()
	if len(first) != want || len(second) != want {
		t.Fatalf("replay lengths = %d/%d, want %d consecutive ready frames", len(first), len(second), want)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("complete replay diverged at interaction frame %d: first=%+v second=%+v", i, first[i], second[i])
		}
	}
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
