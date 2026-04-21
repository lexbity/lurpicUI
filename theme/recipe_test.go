package theme

import "testing"

type samplePatch struct {
	value sampleSlot
}

func (p samplePatch) Apply(base sampleSlot) sampleSlot {
	return p.value
}

type sampleSlot struct {
	A int
	B string
}

func TestResolveSlot_no_patch_returns_base(t *testing.T) {
	base := sampleSlot{A: 1, B: "x"}
	got := resolveSlot(base)
	if got != base {
		t.Fatalf("got %#v want %#v", got, base)
	}
}

func TestResolveSlot_single_patch_overrides(t *testing.T) {
	base := sampleSlot{A: 1, B: "x"}
	got := resolveSlot(base, samplePatch{value: sampleSlot{A: 2, B: "x"}})
	want := sampleSlot{A: 2, B: "x"}
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestResolveSlot_multiple_patches_ordered(t *testing.T) {
	base := sampleSlot{A: 1, B: "x"}
	got := resolveSlot(
		base,
		samplePatch{value: sampleSlot{A: 2, B: "y"}},
		samplePatch{value: sampleSlot{A: 3, B: "z"}},
	)
	want := sampleSlot{A: 3, B: "z"}
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestResolveSlot_does_not_mutate_base(t *testing.T) {
	base := sampleSlot{A: 1, B: "x"}
	_ = resolveSlot(base, samplePatch{value: sampleSlot{A: 4, B: "q"}})
	if base != (sampleSlot{A: 1, B: "x"}) {
		t.Fatalf("base mutated: %#v", base)
	}
}
