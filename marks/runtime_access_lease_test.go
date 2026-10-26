package marks_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/text"
)

type leaseRuntime struct{ service *text.Service }

func (r leaseRuntime) TextService() *text.Service { return r.service }

type leaseFacet struct{ facet.Facet }

func (f *leaseFacet) Base() *facet.Facet         { f.BindImpl(f); return &f.Facet }
func (*leaseFacet) OnAttach(facet.AttachContext) {}
func (*leaseFacet) OnDetach()                    {}
func (*leaseFacet) OnActivate()                  {}
func (*leaseFacet) OnDeactivate()                {}

func TestShapeForPinsLayoutsUntilFacetDispose(t *testing.T) {
	registry := testkit.TestFontRegistry(t)
	service := testkit.TestTextService(t, registry)
	f := &leaseFacet{Facet: facet.NewFacet()}
	session := marks.ShapeFor(leaseRuntime{service: service}, 1, &f.Facet)
	layout := session.ShapeSimple("facet-owned", text.TextStyle{Family: "Noto Sans", Size: 14})
	if layout == nil || service.Trim(0) != 0 {
		t.Fatal("facet-owned layout was not pinned")
	}
	facet.Dispose(f)
	if service.Trim(0) == 0 {
		t.Fatal("facet disposal did not release the layout lease")
	}
}

func TestShapeForReplacesPriorFacetLeaseSet(t *testing.T) {
	service := testkit.TestTextService(t, testkit.TestFontRegistry(t))
	f := &leaseFacet{Facet: facet.NewFacet()}
	runtime := leaseRuntime{service: service}
	first := marks.ShapeFor(runtime, 1, &f.Facet)
	first.ShapeSimple("first layout", text.TextStyle{Family: "Noto Sans", Size: 14})
	second := marks.ShapeFor(runtime, 1, &f.Facet)
	second.ShapeSimple("second layout", text.TextStyle{Family: "Noto Sans", Size: 14})
	if released := service.Trim(0); released == 0 {
		t.Fatal("replacing the facet lease set retained the old layout")
	}
	facet.Dispose(f)
}
