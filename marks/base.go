package marks

import (
	"fmt"
	"reflect"
	"sync"

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

var bindingSubscriberType = reflect.TypeOf((*bindingSubscriber)(nil)).Elem()

var coreType = reflect.TypeOf(Core{})

// bindingField is one declared binding field of a mark: the index path from
// the mark struct to the field holding the Binding value.
type bindingField struct {
	path []int
}

// bindingDecl is the per-mark-type declaration of binding fields, computed
// once and shared by every instance of the type.
type bindingDecl struct {
	fields []bindingField
}

// bindingFieldCache caches the declared-binding walk per mark type. Writers
// are first-attach (RegisterRoles) per type; readers are all later attaches.
// Types live for the process, so the cache is never evicted.
var bindingFieldCache sync.Map // reflect.Type -> *bindingDecl

// declareBindings walks the mark's struct type and records every exported
// binding field: top-level fields plus one level of anonymous embedded
// structs (excluding the embedded Core). The struct declaration is the
// entire binding contract — registration is structural, never imperative.
func declareBindings(t reflect.Type) *bindingDecl {
	if d, ok := bindingFieldCache.Load(t); ok {
		return d.(*bindingDecl)
	}
	d := &bindingDecl{}
	walkBindingFields(t.Elem(), nil, d, t)
	actual, _ := bindingFieldCache.LoadOrStore(t, d)
	return actual.(*bindingDecl)
}

func walkBindingFields(structType reflect.Type, prefix []int, d *bindingDecl, owner reflect.Type) {
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)
		path := append(append([]int(nil), prefix...), i)
		if tag, ok := f.Tag.Lookup("binding"); ok && tag == "-" {
			continue
		}
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft == coreType {
				continue
			}
			if len(prefix) == 0 {
				if f.PkgPath != "" {
					// An embedded unexported struct's exported members are
					// author surface (the language promotes them), but the
					// reflect walk cannot read values through the unexported
					// field. Fail closed: silent skips are the dead-binding
					// bug class this machinery exists to kill.
					for j := 0; j < ft.NumField(); j++ {
						if ft.Field(j).Type.Implements(bindingSubscriberType) {
							panic(fmt.Sprintf("marks: %s embeds unexported struct %s with binding field %s; export the embedded struct so declared bindings can subscribe it", owner, ft, ft.Field(j).Name))
						}
					}
					continue
				}
				walkBindingFields(ft, path, d, owner)
				continue
			}
		}
		if f.PkgPath != "" {
			// Unexported fields are not author surface; internal-only dynamic
			// subscriptions use facet.Store(facet.Subscribe(...)) instead.
			continue
		}
		if f.Type.Implements(bindingSubscriberType) {
			d.fields = append(d.fields, bindingField{path: path})
		}
	}
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

	// subscriptions holds bindings registered via AddBinding — the
	// mark-internal path for bindings created after construction. Declared
	// binding fields (the structural path) are separate and win by default.
	subscriptions []bindingSubscriber
	cleanups      []func()
	rolesReady    bool

	// selfPtr/selfValue are the mark instance passed to RegisterRoles. The
	// declared binding fields are read through selfValue at OnAttach — field
	// values may still change between construction and attach, and the
	// attach-time read is what makes pre-attach replacement work.
	selfPtr   any
	selfValue reflect.Value
	declared  *bindingDecl

	// rt is the runtime captured at attach, used to route binding invalidations
	// into the runtime's per-frame dirty bookkeeping so the frame's dirty
	// regions and layout-root selection observe them. It is nil when the mark
	// is attached outside a runtime (construction, standalone tests).
	rt facet.RuntimeServices
}

// AddBinding registers a mark-internal dynamic binding: Core subscribes to
// the binding's source in OnAttach and invalidates the facet with the
// binding's declared dirty flags on every source change. Const/nil bindings
// are silently skipped.
//
// This is the mark-internal path for bindings the mark creates itself after
// construction. An author's binding contract is declared by the exported
// binding FIELDS of the mark struct: Core subscribes every dynamic field at
// attach (declared bindings), so assignment before attach is the authoring
// contract and AddBinding call sites are illegal outside the marks package
// (lurpiclint LL037).
func (c *Core) AddBinding(s bindingSubscriber) {
	if s == nil || !s.IsDynamic() {
		return
	}
	c.subscriptions = append(c.subscriptions, s)
}

// RegisterRoles declares every exported binding field of the mark (structural
// binding declaration — Core subscribes dynamic fields at attach, so a field
// replaced any time before attach just works) and registers every configured
// role with the Facet via AddRole.
//
// Call once at the end of the constructor, passing the mark itself (the
// pointer-receiver instance): `m.RegisterRoles(m)`. Safe to call multiple
// times with the same instance; a second call with a different instance
// panics (a copy-paste construction bug). Self must be a non-nil pointer to a
// struct — anything else panics (fail-closed misconfiguration).
//
// If BuildCommands is set, ProjectionRole.OnProject is auto-wired to wrap
// BuildCommands in a CommandList.
func (c *Core) RegisterRoles(self any) {
	if c.rolesReady {
		if c.selfPtr != self {
			panic("marks: RegisterRoles called twice with different mark instances on " + reflect.TypeOf(self).String())
		}
		return
	}
	if self == nil {
		panic("marks: RegisterRoles requires the mark instance (the pointer under construction), got nil")
	}
	t := reflect.TypeOf(self)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		panic("marks: RegisterRoles must be called with the mark pointer (struct), got " + t.String())
	}
	c.selfPtr = self
	c.selfValue = reflect.ValueOf(self).Elem()
	c.declared = declareBindings(t)
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

// OnAttach subscribes every dynamic declared binding field (read live from
// the mark instance — values may have changed since construction) plus any
// mark-internal AddBinding registrations, invalidating the Facet on every
// source change. Marks call this from their OnAttach, passing the
// AttachContext through so Core can capture the runtime for the reactivity
// route.
func (c *Core) OnAttach(ctx facet.AttachContext) {
	c.rt = ctx.Runtime
	if c.declared != nil && c.selfValue.IsValid() {
		for _, f := range c.declared.fields {
			v := c.selfValue
			for _, idx := range f.path {
				v = v.Field(idx)
			}
			s, ok := v.Interface().(bindingSubscriber)
			if !ok || !s.IsDynamic() {
				continue
			}
			c.subscribeBinding(s)
		}
	}
	for _, s := range c.subscriptions {
		c.subscribeBinding(s)
	}
}

// subscribeBinding wires one dynamic binding's source changes to facet
// invalidation (the shared body of both declared and internal subscription).
func (c *Core) subscribeBinding(s bindingSubscriber) {
	flags := s.DirtyFlags()
	cleanup := s.SubscribeOnChange(func() {
		// RX-1 FR-3: a binding-visible content change MUST re-measure,
		// re-arrange through ancestor policies, and re-project within one
		// frame, with zero author-written invalidation routing. When a
		// runtime is attached the change routes through the layout
		// package's propagation entry point; without a runtime it falls
		// back to local flags (standalone/construction projections). A
		// panicking handler is quarantined through the runtime's
		// facet-callback recovery hook (tick-style guardedInvoke shape,
		// copied — marks does not import runtime).
		facet.RunRecovered("binding", c.ID(), func() {
			c.InvalidateContent(flags, "binding")
		})
	})
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanup)
	}
}

// EnableViewport declares that this mark hosts runtime-driven scrollable
// content: it arms the Viewport role (identity transform) so RegisterRoles
// registers it. Every scroll-capable mark MUST call this from its
// constructor; the mark never sets Viewport.Transform by hand.
func (c *Core) EnableViewport() {
	c.Viewport.Transform = gfx.Identity()
}

// InvalidateContent routes a content change at this mark through the RX-1 FR-3
// layout propagation when a runtime is attached, falling back to local flags
// otherwise. The declared flags decide whether the change re-measures (DirtyLayout)
// or only re-projects (DirtyProjection). Marks whose OnAttach subscribes a store
// directly (via facet.Store) call this from their handler instead of Invalidate
// so the change routes through the nearest layout root rather than only setting
// local dirty bits.
func (c *Core) InvalidateContent(flags facet.DirtyFlags, source string) {
	if c == nil {
		return
	}
	if c.rt != nil {
		layout.PropagateContentDirty(c, c.rt, source, flags)
		return
	}
	c.Invalidate(flags)
}

// InvalidateWithSource marks the mark dirty and records the invalidation source.
// When the mark is attached to a runtime and the flags declare DirtyLayout, the
// change also routes the runtime layout pass (RX-1 F-dirtylayout-routing): a
// store-bound geometry change re-measures and re-arranges the mark through the
// policies that arrange it. Projection-only changes stay local (the projection
// re-runs from the local dirty read) — routing them would perturb the shell's
// layout on a feed tick (FR-rt). Unattached marks fall back to local flags.
func (c *Core) InvalidateWithSource(flags facet.DirtyFlags, source string) {
	if c == nil {
		return
	}
	c.Facet.InvalidateWithSource(flags, source)
	if c.rt != nil && flags&facet.DirtyLayout != 0 {
		c.rt.Invalidate(c.ID(), flags|facet.DirtyProjection, source)
	}
}

// Invalidate marks the mark dirty (see InvalidateWithSource for the routing
// semantics: a DirtyLayout invalidation through a live runtime also routes the
// layout pass). Overriding the embedded facet's Invalidate makes interaction-
// and store-triggered geometry changes (a dropdown opening, a tree expanding)
// re-arrange a standalone mark instead of relying on the host to route.
func (c *Core) Invalidate(flags facet.DirtyFlags) {
	c.InvalidateWithSource(flags, "")
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
