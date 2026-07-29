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

func TestAttachLayerRecordsZPriority(t *testing.T) {
	parent := &layerTestParent{Facet: NewFacet()}
	child := &layerTestChild{Facet: NewFacet()}

	AttachLayer(parent, child, LayerAttachment{ZPriority: 10})

	if got := child.Base().LayerZPriority(); got != 10 {
		t.Fatalf("LayerZPriority = %d, want 10", got)
	}
	children := parent.Base().Children()
	if len(children) != 1 || children[0] != child.Base() {
		t.Fatal("AttachLayer did not add child to parent")
	}
}

func TestAttachLayerPanicsOnZeroPriority(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for ZPriority == 0")
		}
	}()
	parent := &layerTestParent{Facet: NewFacet()}
	child := &layerTestChild{Facet: NewFacet()}
	AttachLayer(parent, child, LayerAttachment{ZPriority: 0})
}

func TestAttachLayerMultipleChildren(t *testing.T) {
	parent := &layerTestParent{Facet: NewFacet()}
	child1 := &layerTestChild{Facet: NewFacet()}
	child2 := &layerTestChild{Facet: NewFacet()}

	AttachLayer(parent, child1, LayerAttachment{ZPriority: 5})
	AttachLayer(parent, child2, LayerAttachment{ZPriority: 10})

	if got := child1.Base().LayerZPriority(); got != 5 {
		t.Fatalf("child1 LayerZPriority = %d, want 5", got)
	}
	if got := child2.Base().LayerZPriority(); got != 10 {
		t.Fatalf("child2 LayerZPriority = %d, want 10", got)
	}
	children := parent.Base().Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}
