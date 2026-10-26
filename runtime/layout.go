package runtime

import (
	"fmt"
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/signal"
)

func (rt *Runtime) copyDirtyFacets() map[facet.FacetID]facet.DirtyFlags {
	if len(rt.dirtyFacets) == 0 {
		return nil
	}
	out := make(map[facet.FacetID]facet.DirtyFlags, len(rt.dirtyFacets))
	for id, flags := range rt.dirtyFacets {
		if flags != 0 {
			out[id] = flags
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (rt *Runtime) attachTree(root facet.FacetImpl) {
	if root == nil {
		return
	}
	if err := rt.validateFacetStoreScopes(root); err != nil {
		panic(err)
	}
	facet.Attach(root, facet.AttachContext{
		Runtime:                 rt,
		Assets:                  facet.AssetServices{Manager: rt.assetManager},
		Stores:                  facet.StoreServices{AssetRegistry: rt.config.AssetRegistry},
		TestIDs:                 rt.liveTestIDs,
		OnFacetTreeAttached:     rt.onFacetTreeAttached,
		OnFacetDisposed:         rt.onFacetDisposed,
		OnFacetInvalidated:      rt.onFacetInvalidated,
		OnHostedChildrenChanged: rt.onHostedChildrenChanged,
		OnTickActivityChanged:   rt.onTickActivityChanged,
		OnFocusPolicyChanged:    rt.onFocusPolicyChanged,
	})
	if err := rt.registerTreeReadExports(root); err != nil {
		panic("runtime: read export declaration changed after startup: " + err.Error())
	}
	rt.subscribeLayerMounts(root)
}

func (rt *Runtime) onFocusPolicyChanged(id facet.FacetID) {
	if rt == nil || id == 0 {
		return
	}
	rt.focusDirty = true
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) onFacetInvalidated(id facet.FacetID, flags facet.DirtyFlags, source string) {
	if rt == nil || id == 0 || flags == 0 {
		return
	}
	rt.invalidateDriveObservation()
	rt.recordInvalidation(id, flags, source)
}

func (rt *Runtime) onFacetTreeAttached(root facet.FacetImpl) {
	rt.projectionSystem.InvalidateTopology()
	rt.focusDirty = true
	rt.layerTreeDirty = true
	if rt.liveIndex == nil {
		rt.liveIndex = newLiveIndex()
	}
	rt.liveIndex.registerTree(root)
	rt.refreshHostedDescendants(root)
	rt.refreshTickActivities(root)
}

func (rt *Runtime) onFacetDisposed(id facet.FacetID) {
	rt.projectionSystem.ForgetFacet(id)
	delete(rt.dirtyFacets, id)
	delete(rt.dirtySources, id)
	delete(rt.nextDirtyFacets, id)
	delete(rt.nextDirtySources, id)
	rt.focusDirty = true
	rt.layerTreeDirty = true
	rt.removeActiveTickFacet(id)
	if rt.liveIndex != nil {
		rt.liveIndex.removeSubtree(id)
	}
}

func (rt *Runtime) onTickActivityChanged(id facet.FacetID, role *facet.TickRole, active bool) {
	if rt == nil || id == 0 || role == nil {
		return
	}
	if active {
		if _, exists := rt.activeTickByID[id]; exists {
			return
		}
		impl := rt.liveFacet(id)
		if impl == nil || impl.Base() == nil || impl.Base().TickRole() != role {
			return
		}
		rt.activeTickByID[id] = len(rt.activeTickFacets)
		rt.activeTickFacets = append(rt.activeTickFacets, impl)
		if rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
		return
	}
	rt.removeActiveTickFacet(id)
}

func (rt *Runtime) removeActiveTickFacet(id facet.FacetID) {
	index, exists := rt.activeTickByID[id]
	if !exists {
		return
	}
	last := len(rt.activeTickFacets) - 1
	copy(rt.activeTickFacets[index:], rt.activeTickFacets[index+1:])
	rt.activeTickFacets[last] = nil
	rt.activeTickFacets = rt.activeTickFacets[:last]
	for i := index; i < last; i++ {
		rt.activeTickByID[rt.activeTickFacets[i].Base().ID()] = i
	}
	delete(rt.activeTickByID, id)
}

func (rt *Runtime) onHostedChildrenChanged(hostID facet.FacetID, children []facet.FacetImpl) {
	rt.projectionSystem.InvalidateTopology()
	rt.focusDirty = true
	rt.layerTreeDirty = true
	if hostID == 0 || rt.liveIndex == nil {
		return
	}
	if !rt.liveIndex.contains(hostID) {
		return
	}
	rt.liveIndex.registerHosted(hostID, children)
	rt.refreshHostedDescendants(children...)
	rt.refreshTickActivities(children...)
}

func (rt *Runtime) refreshTickActivities(roots ...facet.FacetImpl) {
	stack := append([]facet.FacetImpl(nil), roots...)
	for len(stack) > 0 {
		last := len(stack) - 1
		impl := stack[last]
		stack[last] = nil
		stack = stack[:last]
		if impl == nil || impl.Base() == nil || impl.Base().State() == facet.StateDisposed {
			continue
		}
		base := impl.Base()
		if role := base.TickRole(); role != nil && role.IsActive() {
			rt.onTickActivityChanged(base.ID(), role, true)
		}
		base.VisitChildrenReverse(func(child facet.FacetImpl) bool {
			stack = append(stack, child)
			return true
		})
	}
}

func (rt *Runtime) refreshHostedDescendants(roots ...facet.FacetImpl) {
	if rt.liveIndex == nil || rt.config.HostedChildren == nil {
		return
	}
	stack := append([]facet.FacetImpl(nil), roots...)
	seen := make(map[facet.FacetID]struct{}, len(stack))
	for len(stack) > 0 {
		last := len(stack) - 1
		impl := stack[last]
		stack[last] = nil
		stack = stack[:last]
		if impl == nil || impl.Base() == nil || impl.Base().State() == facet.StateDisposed {
			continue
		}
		id := impl.Base().ID()
		if _, visited := seen[id]; visited {
			continue
		}
		seen[id] = struct{}{}
		impl.Base().VisitChildren(func(child facet.FacetImpl) bool {
			stack = append(stack, child)
			return true
		})
		hosted := rt.config.HostedChildren(impl)
		rt.liveIndex.registerHosted(id, hosted)
		stack = append(stack, hosted...)
	}
}

func (rt *Runtime) liveFacet(id facet.FacetID) facet.FacetImpl {
	if rt == nil || rt.liveIndex == nil {
		return nil
	}
	impl, _, _ := rt.liveIndex.lookup(id)
	return impl
}

func (rt *Runtime) validateFacetStoreScopes(root facet.FacetImpl) error {
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		impl := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if impl == nil || impl.Base() == nil {
			continue
		}
		base := impl.Base()
		if owned := base.StoreScope(); owned != nil && !rt.config.StoreScope.Owns(owned) {
			return fmt.Errorf("runtime: facet %d reactive scope is outside the configured application scope", base.ID())
		}
		if base.IsLayer() {
			if mount := base.LayerAttachment().Mount; mount != nil && !rt.config.StoreScope.Owns(mount.OwnerScope()) {
				return fmt.Errorf("runtime: facet %d layer mount store is outside the configured application scope", base.ID())
			}
		}
		base.VisitChildren(func(child facet.FacetImpl) bool {
			stack = append(stack, child)
			return true
		})
	}
	return nil
}

// subscribeLayerMounts wires every layer-attached facet's Mount store into the
// runtime's dirty tracking (RX-1 Q4 / FR-3). A Mount flip is a layer-content
// change owned by the framework: it marks the child DirtyLayout|DirtyProjection
// synchronously (so the frame's dirty wave reports the mounted child — E5's
// layer-toggle wave) and requests a frame. The subscription rides the facet's
// own subscription bag, so it is released on dispose. This removes the need
// for apps to hand-route Mount flips or keep a parallel visibility flag.
func (rt *Runtime) subscribeLayerMounts(root facet.FacetImpl) {
	if root == nil {
		return
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		base := node.Base()
		if base.IsLayer() {
			if att := base.LayerAttachment(); att.Mount != nil {
				if impl := base.Impl(); impl != nil {
					childID := base.ID()
					facet.Store(facet.Subscribe(impl), &att.Mount.OnChange, att.Mount.Version, func(signal.Change[bool]) {
						rt.markFacetDirtyByID(childID, facet.DirtyLayout|facet.DirtyProjection, "layer.mount")
						if rt.frameTimer != nil {
							rt.frameTimer.RequestFrame()
						}
					})
				}
			}
		}
		base.VisitChildren(func(child facet.FacetImpl) bool {
			stack = append(stack, child)
			return true
		})
	}
}

func (rt *Runtime) activateTree(root facet.FacetImpl) {
	if root == nil {
		return
	}
	facet.Activate(root)
}

func (rt *Runtime) disposeTree(root facet.FacetImpl) {
	if root == nil {
		return
	}
	// The write lock blocks until any in-flight frame completes, then
	// disposes the tree. Disposal is the only mutation of the facet tree
	// outside the runtime thread, so taking the write lock here enforces the
	// single-driver-thread invariant across the shutdown handshake.
	rt.frameMu.Lock()
	defer rt.frameMu.Unlock()
	facet.Dispose(root)
}

func (rt *Runtime) markTreeDirty(root facet.FacetImpl, flags facet.DirtyFlags) {
	if root == nil {
		return
	}
	rt.invalidateDriveObservation()
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		base := node.Base()
		base.InvalidateWithSource(flags, "runtime.markTreeDirty")
		rt.recordInvalidation(base.ID(), flags, "runtime.markTreeDirty")
		base.VisitChildrenReverse(func(child facet.FacetImpl) bool {
			stack = append(stack, child)
			return true
		})
	}
}

func (rt *Runtime) hasLayoutDirty() bool {
	for _, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout != 0 {
			return true
		}
	}
	return false
}

func (rt *Runtime) runLayoutPass(windowSize gfx.Size) {
	if len(rt.dirtyFacets) == 0 {
		rt.lastArrangeCount = 0
		return
	}
	arranged := 0
	rt.invalidateDirtyLayoutCaches()
	roots := rt.layoutDirtyRoots()
	for _, root := range roots {
		if root == nil || root.Base() == nil {
			continue
		}
		bounds := gfx.RectFromXYWH(0, 0, windowSize.W, windowSize.H)
		if root.Base().Parent() != nil {
			layoutRole := root.Base().LayoutRole()
			if layoutRole != nil && !layoutRole.ArrangedBounds.IsEmpty() {
				bounds = layoutRole.ArrangedBounds
			} else {
				// F-layout-root-fallback: this non-root independent layout root
				// was last arranged to empty bounds — its parent's last arrange
				// (e.g. a Stage hiding an inactive exhibit, or Root hiding the
				// narrow overlay in wide mode) gated it. Re-arranging it on its
				// own with the full window would bypass that gating and spread
				// an invisible facet across the screen. Keep it empty so the
				// gating parent's intent stands.
				bounds = gfx.Rect{}
			}
		}
		rt.measureLayoutChild(root, layout.Loose(gfx.Size{W: bounds.Width(), H: bounds.Height()}))
		rt.arrangeLayoutChild(root, bounds)
		arranged++
		rt.clearLayoutDirtyTree(root)
	}
	// A layout root's arrange cascade re-arranges its subtree through each
	// host's OnArrange. An ancestor host's arrange cache can short-circuit its
	// OnArrange when its own bounds are unchanged — which is correct for pure
	// hosts but leaves an internal-only content change (a scroll offset, an
	// active tab body) un-arranged: the dirty facet's OnArrange never re-runs.
	// A dirty facet whose arrange cache is STILL invalid after the cascade was
	// not reached by it; arrange it directly with its current bounds so its
	// own OnArrange re-runs exactly once (a facet the cascade reached has a
	// valid cache and is skipped — no double-arrange).
	arranged += rt.arrangeDirtyLayoutFacets(roots)
	rt.lastArrangeCount = arranged
}

// arrangeDirtyLayoutFacets directly re-arranges the dirty layout facets the
// roots' arrange cascade did not reach (their arrange cache is still invalid
// after the cascade). Gated (empty-bounds) facets are skipped; the gating
// parent's intent stands.
func (rt *Runtime) arrangeDirtyLayoutFacets(roots []facet.FacetImpl) int {
	if len(rt.dirtyFacets) == 0 {
		return 0
	}
	rootIDs := make(map[facet.FacetID]struct{}, len(roots))
	for _, r := range roots {
		if r != nil && r.Base() != nil {
			rootIDs[r.Base().ID()] = struct{}{}
		}
	}
	arranged := 0
	for id, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout == 0 {
			continue
		}
		if _, ok := rootIDs[id]; ok {
			continue
		}
		f := rt.liveFacet(id)
		if f == nil || f.Base() == nil {
			continue
		}
		role := f.Base().LayoutRole()
		if role == nil || role.ArrangedBounds.IsEmpty() {
			continue
		}
		if role.HasValidArrangeCache() {
			continue
		}
		rt.arrangeLayoutChild(f, role.ArrangedBounds)
		arranged++
	}
	return arranged
}

// layoutDirtyRoots returns the deduplicated layout roots for the frame's dirty
// layout set. Each dirty facet is resolved to its nearest layout root — the
// nearest ancestor declaring a GroupParentContract, or the app root — so a
// mid-tree content change re-lays through the policies that arrange it rather
// than in isolation (RX-1 FR-3: re-measure + re-arrange through ancestor
// policies). Roots with a layout-dirty ancestor are filtered so the highest
// root's walk covers the subtree.
func (rt *Runtime) layoutDirtyRoots() []facet.FacetImpl {
	if len(rt.dirtyFacets) == 0 {
		return nil
	}
	seen := make(map[facet.FacetID]struct{}, len(rt.dirtyFacets))
	roots := make([]facet.FacetImpl, 0, len(rt.dirtyFacets))
	for id, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout == 0 {
			continue
		}
		f := rt.liveFacet(id)
		if f == nil || f.Base() == nil {
			continue
		}
		root := layout.NearestLayoutRoot(f)
		if root == nil || root.Base() == nil {
			continue
		}
		rid := root.Base().ID()
		if _, ok := seen[rid]; ok {
			continue
		}
		seen[rid] = struct{}{}
		roots = append(roots, root)
	}
	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].Base().ID() < roots[j].Base().ID()
	})
	filtered := roots[:0]
	for _, f := range roots {
		if !rt.hasLayoutDirtyAncestor(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// invalidateDirtyLayoutCaches clears the measure AND arrange caches along the
// path from every dirty layout facet up to its nearest layout root.
//
// The measure cache must be cleared so the re-laid root re-measures the dirty
// subtree instead of serving its cached size. The arrange cache must be cleared
// on every host along the path too: a host whose own arranged bounds did not
// change would otherwise be served from its arrange cache and never re-arrange
// the dirty descendant — the Arrange-cache short-circuit assumes OnArrange is
// a pure bounds→children mapping, which is false for hosts whose OnArrange
// depends on descendant state (e.g. a tabs host that arranges the active body).
func (rt *Runtime) invalidateDirtyLayoutCaches() {
	if rt.root == nil {
		return
	}
	for id, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout == 0 {
			continue
		}
		f := rt.liveFacet(id)
		if f == nil || f.Base() == nil {
			continue
		}
		root := layout.NearestLayoutRoot(f)
		rootID := facet.FacetID(0)
		if root != nil && root.Base() != nil {
			rootID = root.Base().ID()
		}
		for current := f; current != nil; {
			base := current.Base()
			if base == nil {
				break
			}
			if role := base.LayoutRole(); role != nil {
				role.InvalidateCache()
			}
			if rootID != 0 && base.ID() == rootID {
				break
			}
			parent := base.Parent()
			if parent == nil {
				break
			}
			if impl := parent.Impl(); impl != nil {
				current = impl
			} else {
				current = parent
			}
		}
	}
}

func (rt *Runtime) hasLayoutDirtyAncestor(f facet.FacetImpl) bool {
	if f == nil || f.Base() == nil {
		return false
	}
	for parent := f.Base().Parent(); parent != nil; parent = parent.Parent() {
		if flags := rt.dirtyFacets[parent.ID()]; flags&facet.DirtyLayout != 0 {
			return true
		}
	}
	return false
}

func (rt *Runtime) clearLayoutDirtyTree(f facet.FacetImpl) {
	if f == nil || f.Base() == nil {
		return
	}
	if flags := f.Base().DirtyFlags(); flags&facet.DirtyLayout != 0 {
		f.Base().ClearDirty(facet.DirtyLayout)
	}
	f.Base().VisitChildren(func(child facet.FacetImpl) bool {
		rt.clearLayoutDirtyTree(child)
		return true
	})
}

func (rt *Runtime) measureLayoutChild(f facet.FacetImpl, c layout.Constraints) gfx.Size {
	if f == nil || f.Base() == nil {
		return gfx.Size{}
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return gfx.Size{}
	}
	parentBounds := gfx.RectFromXYWH(0, 0, c.MaxSize.W, c.MaxSize.H)
	var themeCtx any
	var contentScale float32 = 1
	if rt != nil {
		themeCtx = rt.themeContext(parentBounds)
		contentScale = rt.contentScale
	}
	var size gfx.Size
	rt.guardedInvoke(f.Base().ID(), "measure", func() {
		size = role.Measure(facet.MeasureContext{
			Runtime:      rt,
			Theme:        themeCtx,
			ContentScale: contentScale,
		}, c).Size
	})
	return size
}

func (rt *Runtime) arrangeLayoutChild(f facet.FacetImpl, bounds gfx.Rect) {
	if f == nil || f.Base() == nil {
		return
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return
	}
	var themeCtx any
	if rt != nil {
		themeCtx = rt.themeContext(bounds)
	}
	ctx := facet.ArrangeContext{
		Runtime:      rt,
		Theme:        themeCtx,
		ContentScale: rt.contentScale,
	}
	rt.guardedInvoke(f.Base().ID(), "arrange", func() {
		role.Arrange(ctx, bounds)
	})
}
