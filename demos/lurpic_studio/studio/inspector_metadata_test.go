package studio

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// TestInspector_hintCopyAndGrammar asserts RX-1 P9: every exhibit carries a
// "Try:" guidance line rendered in the inspector metadata block, and the
// demonstrated-mark count pluralizes correctly (the "1 marks demonstrated"
// grammar bug is dead).
func TestInspector_hintCopyAndGrammar(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrame()

	for _, e := range exhibitCatalog {
		root.Shell().ActiveExhibit.Set(e.id)
		h.RunFrames(2)
		got := root.Inspector().Hint().Content.Get()
		want := "Try: " + e.hint
		if got != want {
			t.Fatalf("exhibit %s hint = %q, want %q", e.id, got, want)
		}
		if !strings.HasPrefix(got, "Try: ") || strings.TrimSpace(e.hint) == "" {
			t.Fatalf("exhibit %s has no non-trivial Try: hint", e.id)
		}
	}

	// Grammar: singular and plural.
	if got := markCountText(1); got != "1 mark demonstrated" {
		t.Fatalf("markCountText(1) = %q, want the singular form", got)
	}
	if got := markCountText(2); got != "2 marks demonstrated" {
		t.Fatalf("markCountText(2) = %q, want the plural form", got)
	}
	_ = h
}

// TestInspector_hintRenders verifies the hint text is actually projected into
// the inspector card (not just stored).
func TestInspector_hintRenders(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrame()
	insp := root.Inspector()
	hintRect := testkit.RegionOf(insp.Hint())
	if hintRect.IsEmpty() {
		t.Fatal("inspector hint text is not arranged")
	}
	if !testkit.SampleNonBackground(t, h, hintRect, aliveBG(), 64) {
		t.Fatal("inspector hint text renders no pixels")
	}
}
