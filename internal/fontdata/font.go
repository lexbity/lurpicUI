package fontdata

import (
	_ "embed"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
)

//go:embed testdata/NotoSans-Regular.ttf
var testFontTTF []byte

//go:embed testdata/NotoSans-Bold.ttf
var testFontBoldTTF []byte

// TestFontBytes returns the canonical embedded test font (NotoSans-Regular).
// Never nil or empty.
func TestFontBytes() []byte {
	return testFontTTF
}

// TestFontBoldBytes returns the embedded bold variant (NotoSans-Bold).
// Never nil or empty.
func TestFontBoldBytes() []byte {
	return testFontBoldTTF
}

// TestFontRegistry returns a FontRegistry loaded with the canonical test font
// (NotoSans-Regular). Fatals the test if the font cannot be loaded; never
// returns a nil registry.
func TestFontRegistry(t testing.TB) *text.FontRegistry {
	t.Helper()
	wallet, err := membudget.NewWallet(membudget.DefaultPlan(membudget.Standard).Caps[membudget.Text])
	if err != nil {
		t.Fatalf("fontdata: text wallet: %v", err)
	}
	reg, err := text.NewFontRegistry(wallet, text.DefaultLimits(membudget.Standard).MaxFontFaces)
	if err != nil {
		t.Fatalf("fontdata: NewFontRegistry: %v", err)
	}
	if err := reg.LoadFontBytes(testFontTTF, "NotoSans-Regular.ttf"); err != nil {
		t.Fatalf("fontdata: LoadFontBytes: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}
