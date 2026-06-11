package marks

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// --- Fake marks for capability assertions ---

type fakePlainMark struct{ facet.Facet }

func (f *fakePlainMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakePlainMark) Descriptor() Descriptor {
	return Descriptor{Family: "test", TypeName: "fakePlain"}
}
func (f *fakePlainMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakePlainMark) OnDetach()                        {}
func (f *fakePlainMark) OnActivate()                      {}
func (f *fakePlainMark) OnDeactivate()                    {}

type fakeFocusableMark struct{ facet.Facet }

func (f *fakeFocusableMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakeFocusableMark) Descriptor() Descriptor {
	return Descriptor{Family: "test", TypeName: "fakeFocusable"}
}
func (f *fakeFocusableMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakeFocusableMark) OnDetach()                        {}
func (f *fakeFocusableMark) OnActivate()                      {}
func (f *fakeFocusableMark) OnDeactivate()                    {}
func (f *fakeFocusableMark) Focusable() bool                  { return true }

type fakeAnchorMark struct{ facet.Facet }

func (f *fakeAnchorMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakeAnchorMark) Descriptor() Descriptor {
	return Descriptor{Family: "test", TypeName: "fakeAnchor"}
}
func (f *fakeAnchorMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakeAnchorMark) OnDetach()                        {}
func (f *fakeAnchorMark) OnActivate()                      {}
func (f *fakeAnchorMark) OnDeactivate()                    {}
func (f *fakeAnchorMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return nil
}

type fakeAccessibleMark struct{ facet.Facet }

func (f *fakeAccessibleMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakeAccessibleMark) Descriptor() Descriptor {
	return Descriptor{Family: "test", TypeName: "fakeAccessible"}
}
func (f *fakeAccessibleMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakeAccessibleMark) OnDetach()                        {}
func (f *fakeAccessibleMark) OnActivate()                      {}
func (f *fakeAccessibleMark) OnDeactivate()                    {}
func (f *fakeAccessibleMark) AccessibilityRole() string        { return "button" }
func (f *fakeAccessibleMark) AccessibleName() string           { return "test" }

type fakeCompositeMark struct{ facet.Facet }

func (f *fakeCompositeMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakeCompositeMark) Descriptor() Descriptor {
	return Descriptor{Family: "test", TypeName: "fakeComposite"}
}
func (f *fakeCompositeMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakeCompositeMark) OnDetach()                        {}
func (f *fakeCompositeMark) OnActivate()                      {}
func (f *fakeCompositeMark) OnDeactivate()                    {}
func (f *fakeCompositeMark) ChildMarks() []Mark               { return nil }

type fakeDataBoundMark struct{ facet.Facet }

func (f *fakeDataBoundMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakeDataBoundMark) Descriptor() Descriptor {
	return Descriptor{Family: "viz", TypeName: "fakeData"}
}
func (f *fakeDataBoundMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakeDataBoundMark) OnDetach()                        {}
func (f *fakeDataBoundMark) OnActivate()                      {}
func (f *fakeDataBoundMark) OnDeactivate()                    {}
func (f *fakeDataBoundMark) BoundData() any                   { return nil }

type fakeAllCapabilitiesMark struct {
	facet.Facet
	hitRole facet.HitRole
}

func newFakeAllCapabilitiesMark() *fakeAllCapabilitiesMark {
	m := &fakeAllCapabilitiesMark{Facet: facet.NewFacet()}
	m.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: false}
	}
	m.AddRole(&m.hitRole)
	return m
}

func (f *fakeAllCapabilitiesMark) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }
func (f *fakeAllCapabilitiesMark) Descriptor() Descriptor {
	return Descriptor{Family: "all", TypeName: "fakeAll"}
}
func (f *fakeAllCapabilitiesMark) OnAttach(ctx facet.AttachContext) {}
func (f *fakeAllCapabilitiesMark) OnDetach()                        {}
func (f *fakeAllCapabilitiesMark) OnActivate()                      {}
func (f *fakeAllCapabilitiesMark) OnDeactivate()                    {}
func (f *fakeAllCapabilitiesMark) Focusable() bool                  { return true }
func (f *fakeAllCapabilitiesMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return nil
}
func (f *fakeAllCapabilitiesMark) AccessibilityRole() string { return "button" }
func (f *fakeAllCapabilitiesMark) AccessibleName() string    { return "all" }
func (f *fakeAllCapabilitiesMark) ChildMarks() []Mark        { return nil }

// --- Tests ---

func TestDescribe_plain_mark_has_no_capabilities(t *testing.T) {
	m := &fakePlainMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if d.Family != "test" {
		t.Fatalf("Family = %q, want %q", d.Family, "test")
	}
	if d.TypeName != "fakePlain" {
		t.Fatalf("TypeName = %q, want %q", d.TypeName, "fakePlain")
	}
	if d.Focusable {
		t.Fatal("expected Focusable = false")
	}
	if d.ExportsAnchors {
		t.Fatal("expected ExportsAnchors = false")
	}
	if d.Accessible {
		t.Fatal("expected Accessible = false")
	}
	if d.HostsChildren {
		t.Fatal("expected HostsChildren = false")
	}
	if d.HitTestable {
		t.Fatal("expected HitTestable = false")
	}
	if d.DataBound {
		t.Fatal("expected DataBound = false")
	}
}

func TestDescribe_focusable_flag(t *testing.T) {
	m := &fakeFocusableMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if !d.Focusable {
		t.Fatal("expected Focusable = true")
	}
}

func TestDescribe_anchor_flag(t *testing.T) {
	m := &fakeAnchorMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if !d.ExportsAnchors {
		t.Fatal("expected ExportsAnchors = true")
	}
}

func TestDescribe_accessible_flag(t *testing.T) {
	m := &fakeAccessibleMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if !d.Accessible {
		t.Fatal("expected Accessible = true")
	}
}

func TestDescribe_composite_flag(t *testing.T) {
	m := &fakeCompositeMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if !d.HostsChildren {
		t.Fatal("expected HostsChildren = true")
	}
}

func TestDescribe_hit_testable_flag(t *testing.T) {
	m := newFakeAllCapabilitiesMark()
	facet.Attach(m, facet.AttachContext{})
	d := Describe(m)
	if !d.HitTestable {
		t.Fatal("expected HitTestable = true")
	}
}

func TestDescribe_data_bound_flag(t *testing.T) {
	m := &fakeDataBoundMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if !d.DataBound {
		t.Fatal("expected DataBound = true")
	}
}

func TestDescribe_all_capabilities(t *testing.T) {
	m := newFakeAllCapabilitiesMark()
	facet.Attach(m, facet.AttachContext{})
	d := Describe(m)
	if !d.Focusable {
		t.Error("expected Focusable = true")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
	if !d.Accessible {
		t.Error("expected Accessible = true")
	}
	if !d.HostsChildren {
		t.Error("expected HostsChildren = true")
	}
	if !d.HitTestable {
		t.Error("expected HitTestable = true")
	}
}

func TestDescriptor_author_declares_family_and_typename(t *testing.T) {
	m := &fakeFocusableMark{Facet: facet.NewFacet()}
	d := Describe(m)
	if d.Family != "test" {
		t.Fatalf("Family = %q, want %q", d.Family, "test")
	}
	if d.TypeName != "fakeFocusable" {
		t.Fatalf("TypeName = %q, want %q", d.TypeName, "fakeFocusable")
	}
}

// --- Core-based mark tests ---

type corePlainMark struct{ Core }

func (m *corePlainMark) Base() *facet.Facet               { m.BindImpl(m); return &m.Facet }
func (m *corePlainMark) Descriptor() Descriptor           { return Descriptor{Family: "core", TypeName: "plain"} }
func (m *corePlainMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *corePlainMark) OnDetach()                        { m.Core.OnDetach() }
func (m *corePlainMark) OnActivate()                      { m.Core.OnActivate() }
func (m *corePlainMark) OnDeactivate()                    { m.Core.OnDeactivate() }

func TestDescribe_core_plain_mark_no_capabilities(t *testing.T) {
	m := &corePlainMark{}
	d := Describe(m)
	if d.Focusable {
		t.Error("expected Focusable = false")
	}
	if d.ExportsAnchors {
		t.Error("expected ExportsAnchors = false")
	}
	if d.Accessible {
		t.Error("expected Accessible = false")
	}
	if d.HostsChildren {
		t.Error("expected HostsChildren = false")
	}
	if d.HitTestable {
		t.Error("expected HitTestable = false")
	}
	if d.DataBound {
		t.Error("expected DataBound = false")
	}
}

type coreFocusableMark struct{ Core }

func (m *coreFocusableMark) Base() *facet.Facet { m.BindImpl(m); return &m.Facet }
func (m *coreFocusableMark) Descriptor() Descriptor {
	return Descriptor{Family: "core", TypeName: "focusable"}
}
func (m *coreFocusableMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *coreFocusableMark) OnDetach()                        { m.Core.OnDetach() }
func (m *coreFocusableMark) OnActivate()                      { m.Core.OnActivate() }
func (m *coreFocusableMark) OnDeactivate()                    { m.Core.OnDeactivate() }
func (m *coreFocusableMark) Focusable() bool                  { return true }

func TestDescribe_core_focusable_mark(t *testing.T) {
	m := &coreFocusableMark{}
	d := Describe(m)
	if !d.Focusable {
		t.Fatal("expected Focusable = true")
	}
}

type coreAnchorMark struct{ Core }

func (m *coreAnchorMark) Base() *facet.Facet { m.BindImpl(m); return &m.Facet }
func (m *coreAnchorMark) Descriptor() Descriptor {
	return Descriptor{Family: "core", TypeName: "anchor"}
}
func (m *coreAnchorMark) OnAttach(ctx facet.AttachContext)                              { m.Core.OnAttach() }
func (m *coreAnchorMark) OnDetach()                                                     { m.Core.OnDetach() }
func (m *coreAnchorMark) OnActivate()                                                   { m.Core.OnActivate() }
func (m *coreAnchorMark) OnDeactivate()                                                 { m.Core.OnDeactivate() }
func (m *coreAnchorMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet { return nil }

func TestDescribe_core_anchor_mark(t *testing.T) {
	m := &coreAnchorMark{}
	d := Describe(m)
	if !d.ExportsAnchors {
		t.Fatal("expected ExportsAnchors = true")
	}
}

type coreCompositeMark struct{ Core }

func (m *coreCompositeMark) Base() *facet.Facet { m.BindImpl(m); return &m.Facet }
func (m *coreCompositeMark) Descriptor() Descriptor {
	return Descriptor{Family: "core", TypeName: "composite"}
}
func (m *coreCompositeMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *coreCompositeMark) OnDetach()                        { m.Core.OnDetach() }
func (m *coreCompositeMark) OnActivate()                      { m.Core.OnActivate() }
func (m *coreCompositeMark) OnDeactivate()                    { m.Core.OnDeactivate() }
func (m *coreCompositeMark) ChildMarks() []Mark               { return nil }

func TestDescribe_core_composite_mark(t *testing.T) {
	m := &coreCompositeMark{}
	d := Describe(m)
	if !d.HostsChildren {
		t.Fatal("expected HostsChildren = true")
	}
}

type coreHitTestableMark struct {
	Core
	hitRole facet.HitRole
}

func newCoreHitTestableMark() *coreHitTestableMark {
	m := &coreHitTestableMark{}
	m.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: false}
	}
	m.AddRole(&m.hitRole)
	return m
}

func (m *coreHitTestableMark) Base() *facet.Facet { m.BindImpl(m); return &m.Facet }
func (m *coreHitTestableMark) Descriptor() Descriptor {
	return Descriptor{Family: "core", TypeName: "hittable"}
}
func (m *coreHitTestableMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *coreHitTestableMark) OnDetach()                        { m.Core.OnDetach() }
func (m *coreHitTestableMark) OnActivate()                      { m.Core.OnActivate() }
func (m *coreHitTestableMark) OnDeactivate()                    { m.Core.OnDeactivate() }

func TestDescribe_core_hit_testable_mark(t *testing.T) {
	m := newCoreHitTestableMark()
	d := Describe(m)
	if !d.HitTestable {
		t.Fatal("expected HitTestable = true (via registered HitRole)")
	}
}

type coreAllCapabilitiesMark struct {
	Core
	hitRole facet.HitRole
}

func newCoreAllCapabilitiesMark() *coreAllCapabilitiesMark {
	m := &coreAllCapabilitiesMark{}
	m.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: false}
	}
	m.AddRole(&m.hitRole)
	return m
}

func (m *coreAllCapabilitiesMark) Base() *facet.Facet { m.BindImpl(m); return &m.Facet }
func (m *coreAllCapabilitiesMark) Descriptor() Descriptor {
	return Descriptor{Family: "core", TypeName: "all"}
}
func (m *coreAllCapabilitiesMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *coreAllCapabilitiesMark) OnDetach()                        { m.Core.OnDetach() }
func (m *coreAllCapabilitiesMark) OnActivate()                      { m.Core.OnActivate() }
func (m *coreAllCapabilitiesMark) OnDeactivate()                    { m.Core.OnDeactivate() }
func (m *coreAllCapabilitiesMark) Focusable() bool                  { return true }
func (m *coreAllCapabilitiesMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return nil
}
func (m *coreAllCapabilitiesMark) ChildMarks() []Mark { return nil }

func TestDescribe_core_all_capabilities(t *testing.T) {
	m := newCoreAllCapabilitiesMark()
	d := Describe(m)
	if !d.Focusable {
		t.Error("expected Focusable = true")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
	if !d.HostsChildren {
		t.Error("expected HostsChildren = true")
	}
	if !d.HitTestable {
		t.Error("expected HitTestable = true")
	}
	if d.DataBound {
		t.Error("expected DataBound = false (not implemented)")
	}
}
