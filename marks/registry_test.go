package marks

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// --- Core-based marks for registry testing ---

type regPlainMark struct{ Core }

func (m *regPlainMark) Base() *facet.Facet               { m.Facet.BindImpl(m); return &m.Facet }
func (m *regPlainMark) Descriptor() Descriptor            { return Descriptor{Family: "reg", TypeName: "plain"} }
func (m *regPlainMark) OnAttach(ctx facet.AttachContext)  { m.Core.OnAttach() }
func (m *regPlainMark) OnDetach()                         { m.Core.OnDetach() }
func (m *regPlainMark) OnActivate()                       { m.Core.OnActivate() }
func (m *regPlainMark) OnDeactivate()                     { m.Core.OnDeactivate() }

type regFocusableMark struct{ Core }

func (m *regFocusableMark) Base() *facet.Facet              { m.Facet.BindImpl(m); return &m.Facet }
func (m *regFocusableMark) Descriptor() Descriptor           { return Descriptor{Family: "reg", TypeName: "focusable"} }
func (m *regFocusableMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *regFocusableMark) OnDetach()                        { m.Core.OnDetach() }
func (m *regFocusableMark) OnActivate()                      { m.Core.OnActivate() }
func (m *regFocusableMark) OnDeactivate()                    { m.Core.OnDeactivate() }
func (m *regFocusableMark) Focusable() bool                  { return true }

type regAnchorMark struct{ Core }

func (m *regAnchorMark) Base() *facet.Facet              { m.Facet.BindImpl(m); return &m.Facet }
func (m *regAnchorMark) Descriptor() Descriptor           { return Descriptor{Family: "reg", TypeName: "anchor"} }
func (m *regAnchorMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *regAnchorMark) OnDetach()                        { m.Core.OnDetach() }
func (m *regAnchorMark) OnActivate()                      { m.Core.OnActivate() }
func (m *regAnchorMark) OnDeactivate()                    { m.Core.OnDeactivate() }
func (m *regAnchorMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet { return nil }

type regHitMark struct {
	Core
	hitRole facet.HitRole
}

func newRegHitMark() *regHitMark {
	m := &regHitMark{}
	m.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: false}
	}
	m.AddRole(&m.hitRole)
	return m
}

func (m *regHitMark) Base() *facet.Facet              { m.Facet.BindImpl(m); return &m.Facet }
func (m *regHitMark) Descriptor() Descriptor           { return Descriptor{Family: "reg", TypeName: "hittable"} }
func (m *regHitMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *regHitMark) OnDetach()                        { m.Core.OnDetach() }
func (m *regHitMark) OnActivate()                      { m.Core.OnActivate() }
func (m *regHitMark) OnDeactivate()                    { m.Core.OnDeactivate() }

// --- Tests ---

func TestRegistry_core_marks_register_and_query(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	Register(&regPlainMark{})
	Register(&regFocusableMark{})
	Register(&regAnchorMark{})
	Register(newRegHitMark())

	all := Registered()
	if len(all) != 4 {
		t.Fatalf("expected 4 registered marks, got %d", len(all))
	}

	plain := RegisteredByFamily("reg")
	if len(plain) != 4 {
		t.Fatalf("expected 4 marks in reg family, got %d", len(plain))
	}
}

func TestRegistry_core_mark_descriptor_flags(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	Register(&regPlainMark{})
	Register(&regFocusableMark{})
	Register(&regAnchorMark{})
	Register(newRegHitMark())

	descs := make(map[string]Descriptor)
	for _, d := range Registered() {
		descs[d.TypeName] = d
	}

	t.Run("plain has no capabilities", func(t *testing.T) {
		d, ok := descs["plain"]
		if !ok {
			t.Fatal("missing plain descriptor")
		}
		if d.Family != "reg" {
			t.Errorf("Family = %q, want %q", d.Family, "reg")
		}
		if d.Focusable {
			t.Error("expected Focusable = false")
		}
		if d.ExportsAnchors {
			t.Error("expected ExportsAnchors = false")
		}
		if d.HitTestable {
			t.Error("expected HitTestable = false")
		}
		if d.DataBound {
			t.Error("expected DataBound = false")
		}
	})

	t.Run("focusable mark", func(t *testing.T) {
		d, ok := descs["focusable"]
		if !ok {
			t.Fatal("missing focusable descriptor")
		}
		if !d.Focusable {
			t.Error("expected Focusable = true")
		}
		if d.ExportsAnchors {
			t.Error("expected ExportsAnchors = false")
		}
		if d.HitTestable {
			t.Error("expected HitTestable = false")
		}
	})

	t.Run("anchor mark", func(t *testing.T) {
		d, ok := descs["anchor"]
		if !ok {
			t.Fatal("missing anchor descriptor")
		}
		if !d.ExportsAnchors {
			t.Error("expected ExportsAnchors = true")
		}
		if d.Focusable {
			t.Error("expected Focusable = false")
		}
	})

	t.Run("hit testable mark", func(t *testing.T) {
		d, ok := descs["hittable"]
		if !ok {
			t.Fatal("missing hittable descriptor")
		}
		if !d.HitTestable {
			t.Error("expected HitTestable = true")
		}
		if d.Focusable {
			t.Error("expected Focusable = false")
		}
	})
}

func TestRegistry_core_mark_family_filtering(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	Register(&regPlainMark{})
	Register(&regFocusableMark{})
	Register(&regAnchorMark{})
	Register(newRegHitMark())

	descs := RegisteredByFamily("nonexistent")
	if len(descs) != 0 {
		t.Fatalf("expected 0 for nonexistent family, got %d", len(descs))
	}

	descs = RegisteredByFamily("reg")
	if len(descs) != 4 {
		t.Fatalf("expected 4 for reg family, got %d", len(descs))
	}
}

func TestRegistry_nil_mark_skipped(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	Register(nil)
	if len(Registered()) != 0 {
		t.Fatal("expected nil registration to be skipped")
	}
}

func TestRegistry_descriptor_typename_preserved(t *testing.T) {
	ResetRegistry()
	defer ResetRegistry()

	Register(&regAnchorMark{})
	d := RegisteredByFamily("reg")
	if len(d) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(d))
	}
	if d[0].TypeName != "anchor" {
		t.Fatalf("TypeName = %q, want %q", d[0].TypeName, "anchor")
	}
	if d[0].Family != "reg" {
		t.Fatalf("Family = %q, want %q", d[0].Family, "reg")
	}
}
