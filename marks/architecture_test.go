package marks

import (
	"os/exec"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestDescribe_no_reflect_import(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{.Imports}}", "codeburg.org/lexbit/lurpicui/marks").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	if strings.Contains(string(out), "reflect") {
		t.Fatalf("marks package imports reflect: %s", out)
	}
}

func TestRegistry_population_explicit(t *testing.T) {
	ResetRegistry()
	before := len(Registered())

	m := &fakePlainMark{Facet: facet.NewFacet()}
	Register(m)

	after := len(Registered())
	if after != before+1 {
		t.Fatalf("expected 1 registered mark, got %d (before=%d)", after-before, before)
	}
}

func TestRegistry_by_family(t *testing.T) {
	ResetRegistry()

	m1 := &fakeFocusableMark{Facet: facet.NewFacet()}
	m2 := &fakePlainMark{Facet: facet.NewFacet()}
	m3 := &fakeDataBoundMark{Facet: facet.NewFacet()}

	Register(m1)
	Register(m2)
	Register(m3)

	desc := RegisteredByFamily("test")
	if len(desc) != 2 {
		t.Fatalf("expected 2 marks in test family, got %d", len(desc))
	}

	desc = RegisteredByFamily("viz")
	if len(desc) != 1 {
		t.Fatalf("expected 1 mark in viz family, got %d", len(desc))
	}
}

func TestRegistry_empty_family_returns_empty(t *testing.T) {
	ResetRegistry()
	desc := RegisteredByFamily("nonexistent")
	if len(desc) != 0 {
		t.Fatalf("expected empty slice, got %d", len(desc))
	}
}

func TestRegistry_reset(t *testing.T) {
	ResetRegistry()
	Register(&fakeFocusableMark{Facet: facet.NewFacet()})
	if len(Registered()) == 0 {
		t.Fatal("expected registered marks after Register")
	}
	ResetRegistry()
	if len(Registered()) != 0 {
		t.Fatal("expected empty registry after Reset")
	}
}
