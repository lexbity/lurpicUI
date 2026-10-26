package text_test

import (
	"os"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestSealedGlyphPagePinsFontChargeUntilRelease(t *testing.T) {
	fontBytes, err := os.ReadFile("testdata/NotoSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	wallet, err := membudget.NewWallet(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	limits := text.DefaultLimits(membudget.Compact)
	registry, err := text.NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFontBytes(fontBytes, "fixture-font"); err != nil {
		t.Fatal(err)
	}
	service, err := text.NewService(registry, limits)
	if err != nil {
		t.Fatal(err)
	}
	layoutLease, err := service.Shape(text.ShapeRequest{Content: "frame-owned glyphs", Style: text.TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(layoutLease.Layout.Lines) == 0 || len(layoutLease.Layout.Lines[0].Runs) == 0 {
		t.Fatal("fixture shaping produced no glyph run")
	}
	commands := []gfx.Command{gfx.DrawGlyphRun{Run: layoutLease.Layout.Lines[0].Runs[0]}}
	bytes, err := gfx.EstimateCommandPageBytes(commands)
	if err != nil {
		t.Fatal(err)
	}
	pageCharge, err := wallet.Reserve(bytes)
	if err != nil {
		t.Fatal(err)
	}
	page, err := gfx.SealCommandPageFromSource(len(commands), func(yield func(gfx.Command) bool) {
		for _, command := range commands {
			if !yield(command) {
				return
			}
		}
	}, pageCharge)
	if err != nil {
		_ = pageCharge.Cancel()
		t.Fatal(err)
	}
	if err := pageCharge.Publish(); err != nil {
		_ = page.Release()
		_ = pageCharge.Cancel()
		t.Fatal(err)
	}
	layoutLease.Release()
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, live, retiring := wallet.Snapshot()
	if live == 0 || retiring != 0 {
		t.Fatalf("font source released while sealed page still pins it: live=%d retiring=%d", live, retiring)
	}
	if err := page.Release(); err != nil {
		t.Fatal(err)
	}
	if err := pageCharge.Retire(); err != nil {
		t.Fatal(err)
	}
	if err := pageCharge.Acknowledge(); err != nil {
		t.Fatal(err)
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("font charge remained after page acknowledgment: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}
