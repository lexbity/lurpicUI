package facet

// LayerAttachment describes the layer contract for a child mounted via AttachLayer.
type LayerAttachment struct {
	ZPriority int32
	Dismissal DismissalScope
	HitPolicy HitPolicy
}

// AttachLayer registers child as a layered child of parent, recording the layer
// contract for the runtime to consume. It panics with a contract message if
// att.ZPriority is zero — callers wanting default placement use AddChild.
//
// AttachLayer is safe to call during construction (the child lifecycle is
// managed by the runtime once the tree is attached).
func AttachLayer(parent, child FacetImpl, att LayerAttachment) {
	if att.ZPriority == 0 {
		panic("facet: AttachLayer requires ZPriority > 0; use AddChild for default placement")
	}
	parent.Base().AddChild(child.Base())
	child.Base().layerZPriority = att.ZPriority
}
