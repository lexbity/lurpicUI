package facet

import (
	"testing"
)

type layerTestChild struct {
	Facet
}

func (c *layerTestChild) Base() *Facet             { return &c.Facet }
func (c *layerTestChild) OnAttach(_ AttachContext) {}
func (c *layerTestChild) OnDetach()                {}
func (c *layerTestChild) OnActivate()              {}
func (c *layerTestChild) OnDeactivate()            {}

type layerTestParent struct {
	Facet
}

func (p *layerTestParent) Base() *Facet             { return &p.Facet }
func (p *layerTestParent) OnAttach(_ AttachContext) {}
func (p *layerTestParent) OnDetach()                {}
func (p *layerTestParent) OnActivate()              {}
func (p *layerTestParent) OnDeactivate()            {}

func TestAttachLayerRecordsBand(t *testing.T) {
	parent := &layerTestParent{Facet: NewFacet()}
	child := &layerTestChild{Facet: NewFacet()}

	AttachLayer(parent, child, LayerAttachment{Band: ZBandModal})

	if !child.Base().IsLayer() {
		t.Fatal("AttachLayer did not mark the child as a layer")
	}
	if got := child.Base().LayerAttachment().Band; got != ZBandModal {
		t.Fatalf("layer band = %v, want ZBandModal", got)
	}
	children := parent.Base().Children()
	if len(children) != 1 || children[0] != child.Base() {
		t.Fatal("AttachLayer did not add child to parent")
	}
}

func TestAttachLayerPanicsOnInvalidBand(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid band")
		}
	}()
	parent := &layerTestParent{Facet: NewFacet()}
	child := &layerTestChild{Facet: NewFacet()}
	AttachLayer(parent, child, LayerAttachment{Band: 99})
}

func TestAttachLayerMultipleChildren(t *testing.T) {
	parent := &layerTestParent{Facet: NewFacet()}
	child1 := &layerTestChild{Facet: NewFacet()}
	child2 := &layerTestChild{Facet: NewFacet()}

	AttachLayer(parent, child1, LayerAttachment{Band: ZBandContent})
	AttachLayer(parent, child2, LayerAttachment{Band: ZBandModal})

	if got := child1.Base().LayerAttachment().Band; got != ZBandContent {
		t.Fatalf("child1 band = %v, want ZBandContent", got)
	}
	if got := child2.Base().LayerAttachment().Band; got != ZBandModal {
		t.Fatalf("child2 band = %v, want ZBandModal", got)
	}
	children := parent.Base().Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}
