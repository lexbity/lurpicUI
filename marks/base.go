package marks

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// bindingSubscriber is satisfied by Binding[T] for any T.
type bindingSubscriber interface {
	SubscribeOnChange(func()) func()
	DirtyFlags() facet.DirtyFlags
	IsDynamic() bool
}

// Core eliminates per-mark boilerplate through composition.
//
// The concrete mark embeds Core and provides its own Base() calling
// BindImpl(self). Core embeds facet.Facet so the concrete mark inherits
// AddRole, Invalidate, DirtyFlags, and the role accessor methods
// (LayoutRole, RenderRole, etc.) through promotion.
//
// Role fields (Layout, Render, Projection, etc.) are intentionally named
// without the "Role" suffix to avoid ambiguity with Facet's accessor methods
// (LayoutRole, RenderRole, etc.) at the same promotion depth.
type Core struct {
	facet.Facet

	Layout     facet.LayoutRole
	Render     facet.RenderRole
	Projection facet.ProjectionRole
	Hit        facet.HitRole
	Input      facet.InputRole
	Focus      facet.FocusRole
	Viewport   facet.ViewportRole
	Tick       facet.TickRole

	// BuildCommands is the single render/projection hook.
	// When set, RegisterRoles auto-wires ProjectionRole.OnProject.
	BuildCommands func(ctx facet.ProjectionContext) []gfx.Command

	subscriptions []bindingSubscriber
	cleanups      []func()
	rolesReady    bool
}

// AddBinding registers a dynamic binding. Core subscribes to the binding's
// source in OnAttach and invalidates the facet with the binding's declared
// dirty flags on every source change. Const/nil bindings are silently skipped.
func (c *Core) AddBinding(s bindingSubscriber) {
	if s == nil || !s.IsDynamic() {
		return
	}
	c.subscriptions = append(c.subscriptions, s)
}

// RegisterRoles scans the exported role fields and registers every configured
// role with the Facet via AddRole. Safe to call multiple times.
//
// If BuildCommands is set, ProjectionRole.OnProject is auto-wired to wrap
// BuildCommands in a CommandList. Marks call this at the end of their
// constructor.
func (c *Core) RegisterRoles() {
	if c.rolesReady {
		return
	}
	c.rolesReady = true

	if c.Layout.OnMeasure != nil {
		c.AddRole(&c.Layout)
	}
	if c.Render.OnCollect != nil {
		c.AddRole(&c.Render)
	}

	if c.BuildCommands != nil && c.Projection.OnProject == nil {
		c.Projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
			cmds := c.BuildCommands(ctx)
			if len(cmds) == 0 {
				return nil
			}
			return &gfx.CommandList{Commands: cmds}
		}
	}
	if c.Projection.OnProject != nil {
		c.AddRole(&c.Projection)
	}

	if c.Hit.OnHitTest != nil {
		c.AddRole(&c.Hit)
	}
	if c.Input.OnPointer != nil ||
		c.Input.OnTouch != nil ||
		c.Input.OnScroll != nil ||
		c.Input.OnKey != nil ||
		c.Input.OnText != nil ||
		c.Input.OnDismiss != nil {
		c.AddRole(&c.Input)
	}
	if c.Focus.Focusable != nil {
		c.AddRole(&c.Focus)
	}
	if c.Viewport.Transform != (gfx.Transform{}) {
		c.AddRole(&c.Viewport)
	}
	if c.Tick.OnTick != nil {
		c.AddRole(&c.Tick)
	}
}

// OnAttach subscribes all registered dynamic bindings, invalidating the
// Facet on every source change. Marks call this from their OnAttach.
func (c *Core) OnAttach() {
	for _, s := range c.subscriptions {
		flags := s.DirtyFlags()
		cleanup := s.SubscribeOnChange(func() {
			c.Invalidate(flags)
		})
		if cleanup != nil {
			c.cleanups = append(c.cleanups, cleanup)
		}
	}
}

// OnDetach unsubscribes all bindings. Marks call this from their OnDetach.
func (c *Core) OnDetach() {
	for _, cl := range c.cleanups {
		if cl != nil {
			cl()
		}
	}
	c.cleanups = c.cleanups[:0]
}

// OnActivate is a no-op default called from the concrete mark.
func (c *Core) OnActivate() {}

// OnDeactivate is a no-op default called from the concrete mark.
func (c *Core) OnDeactivate() {}

// DefaultAnchors computes the standard five bounds anchors from the given
// arranged bounds. Marks call this from their ExportAnchors override.
func (c *Core) DefaultAnchors(bounds gfx.Rect, ctx layout.AnchorExportContext) layout.AnchorSet {
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds_center": {
			X: (bounds.Min.X + bounds.Max.X) * 0.5,
			Y: (bounds.Min.Y + bounds.Max.Y) * 0.5,
		},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    {X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  {X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": {X: bounds.Max.X, Y: bounds.Max.Y},
	}
}
