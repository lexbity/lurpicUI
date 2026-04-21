package theme

import (
	"reflect"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

type testStyleNode struct {
	id     uint64
	parent *testStyleNode
	impl   any
	style  *StyleContextStore
}

func newTestStyleNode(id uint64) *testStyleNode {
	n := &testStyleNode{id: id}
	n.impl = n
	return n
}

func (n *testStyleNode) Base() *testStyleNode                  { return n }
func (n *testStyleNode) Parent() *testStyleNode                { return n.parent }
func (n *testStyleNode) Impl() any                             { return n.impl }
func (n *testStyleNode) StyleContextStore() *StyleContextStore { return n.style }

func (n *testStyleNode) AddChild(child *testStyleNode) {
	if n == nil || child == nil {
		return
	}
	child.parent = n
}

type testStyleTree struct {
	nodes map[uint64]any
	root  any
}

func (t *testStyleTree) FacetByID(id any) any {
	if t == nil {
		return nil
	}
	if n, ok := id.(uint64); ok {
		return t.nodes[n]
	}
	return nil
}

func (t *testStyleTree) RootStyleContext() any {
	if t == nil {
		return nil
	}
	return t.root
}

func TestMarkStyleResolve_defaultAndOverrides(t *testing.T) {
	tokens := DefaultTokens()
	base := SolidMaterial(gfx.ColorFromRGBA8(20, 40, 60, 255), gfx.ColorFromRGBA8(5, 5, 5, 255), 1)
	style := MarkStyle{Base: base}

	if got := style.Resolve(StateDefault, tokens); !reflect.DeepEqual(got, base) {
		t.Fatalf("default state should return base: %#v", got)
	}

	hover := style.Resolve(StateHover, tokens)
	if !isLighterThan(hover, base) {
		t.Fatalf("hover material should be lighter than base: %#v vs %#v", hover, base)
	}

	pressed := style.Resolve(StatePressed, tokens)
	if !isDarkerThan(pressed, base) {
		t.Fatalf("pressed material should be darker than base: %#v vs %#v", pressed, base)
	}

	disabled := style.Resolve(StateDisabled, tokens)
	if disabled.Opacity != tokens.Color.DisabledOpacity {
		t.Fatalf("unexpected disabled opacity: %v", disabled.Opacity)
	}

	selected := style.Resolve(StateSelected, tokens)
	if len(selected.Fills) != len(base.Fills)+1 {
		t.Fatalf("selected should append one fill: %#v", selected.Fills)
	}

	explicit := SolidMaterial(gfx.ColorFromRGBA8(1, 2, 3, 255), gfx.ColorFromRGBA8(4, 5, 6, 255), 2)
	style = MarkStyle{
		Base:     base,
		Hover:    &explicit,
		Pressed:  &explicit,
		Disabled: &explicit,
		Selected: &explicit,
		Focused:  &explicit,
	}
	for _, state := range []InteractionState{StateHover, StatePressed, StateDisabled, StateSelected, StateFocused} {
		if got := style.Resolve(state, tokens); !reflect.DeepEqual(got, explicit) {
			t.Fatalf("override for state %v should be returned unchanged, got %#v", state, got)
		}
	}
}

func TestStyleContextDerive_purelyFunctional(t *testing.T) {
	ctx := DefaultTokens()
	parent := StyleContext{
		Tokens:    ctx,
		Materials: NewMaterialRegistry(),
		Depth:     3,
	}
	clone := parent
	if got := parent.Derive(StyleContextOverride{}); !reflect.DeepEqual(got, clone) {
		t.Fatalf("empty derive should return equal context: %#v vs %#v", got, clone)
	}

	colors := parent.Tokens.Color
	colors.Primary = gfx.ColorFromRGBA8(1, 2, 3, 255)
	materials := NewMaterialRegistry()
	materials.Define("card", FromToken(gfx.ColorFromRGBA8(9, 8, 7, 255)))
	derived := parent.Derive(StyleContextOverride{
		Colors:    &colors,
		Materials: materials,
	})
	if derived.Tokens.Color.Primary != colors.Primary {
		t.Fatalf("color override not applied")
	}
	if derived.Tokens.Typography != parent.Tokens.Typography {
		t.Fatalf("typography should be unchanged")
	}
	if derived.Materials != materials {
		t.Fatalf("materials override not applied")
	}
	if !reflect.DeepEqual(parent.Tokens.Color.Primary, ctx.Color.Primary) {
		t.Fatalf("parent mutated")
	}

	original := parent
	next := parent
	for i := 0; i < 10; i++ {
		next = next.Derive(StyleContextOverride{})
	}
	if !reflect.DeepEqual(original, parent) || !reflect.DeepEqual(next, parent) {
		t.Fatalf("derive should be purely functional")
	}
}

func TestNewRootStyleContext_andNearestStyleContext(t *testing.T) {
	rootFacet := newTestStyleNode(1)
	childFacet := newTestStyleNode(2)
	leafFacet := newTestStyleNode(3)
	rootFacet.AddChild(childFacet)
	childFacet.AddChild(leafFacet)

	rootStore := NewRootStyleContext(nil, DefaultTokens(), nil)
	childStore := store.NewValueStore(StyleContext{Tokens: DarkTokens(), Materials: NewMaterialRegistry(), Depth: 1})
	childFacet.style = childStore

	tree := &testStyleTree{
		nodes: map[uint64]any{
			rootFacet.id:  rootFacet,
			childFacet.id: childFacet,
			leafFacet.id:  leafFacet,
		},
		root: rootStore,
	}

	if got := NearestStyleContext(tree, rootFacet.id); got != rootStore {
		t.Fatalf("expected root store, got %#v", got)
	}
	if got := NearestStyleContext(tree, leafFacet.id); got != childStore {
		t.Fatalf("expected nearest child store, got %#v", got)
	}
}

func TestStyleContextStore_change_deferred_by_queue_hook(t *testing.T) {
	defer store.SetSignalQueueHook(nil)
	var queued []func()
	store.SetSignalQueueHook(func(fn func()) {
		queued = append(queued, fn)
	})

	ctxStore := NewRootStyleContext(nil, DefaultTokens(), nil)
	var called int
	ctxStore.OnChange.Subscribe(func(signal.Change[StyleContext]) {
		called++
	})

	next := ctxStore.Get()
	next.Depth++
	ctxStore.Set(next)

	if called != 0 {
		t.Fatalf("handler should not run before queued delivery, got %d", called)
	}
	if len(queued) != 1 {
		t.Fatalf("expected one queued signal, got %d", len(queued))
	}

	queued[0]()
	if called != 1 {
		t.Fatalf("handler should run after queued delivery, got %d", called)
	}
}

func TestStyleContextStore_version_increments_on_derive(t *testing.T) {
	ctxStore := NewRootStyleContext(nil, DefaultTokens(), nil)
	before := ctxStore.Version()
	next := ctxStore.Get()
	next.Depth++
	ctxStore.Set(next)
	if got := ctxStore.Version(); got <= before {
		t.Fatalf("version did not increment: %d -> %d", before, got)
	}
}

func TestNearestStyleContext_noAncestor_returnsRoot(t *testing.T) {
	rootStore := NewRootStyleContext(nil, DefaultTokens(), nil)
	root := newTestStyleNode(5)
	tree := &testStyleTree{
		nodes: map[uint64]any{root.id: root},
		root:  rootStore,
	}
	if got := NearestStyleContext(tree, root.id); got != rootStore {
		t.Fatalf("expected root store, got %#v", got)
	}
}

func isLighterThan(a, b Material) bool {
	return luminanceOfMaterial(a) > luminanceOfMaterial(b)
}

func isDarkerThan(a, b Material) bool {
	return luminanceOfMaterial(a) < luminanceOfMaterial(b)
}

func luminanceOfMaterial(m Material) float64 {
	if len(m.Fills) == 0 {
		return 0
	}
	r, g, b, _ := m.Fills[0].Color.ToRGBA8()
	return 0.2126*srgbComponent(float64(r)) + 0.7152*srgbComponent(float64(g)) + 0.0722*srgbComponent(float64(b))
}

func TestStyleContextDerive_materialsOverrideDoesNotMutateParent(t *testing.T) {
	parent := StyleContext{
		Tokens: DefaultTokens(),
	}
	reg := NewMaterialRegistry()
	reg.Define("a", FromToken(gfx.ColorFromRGBA8(1, 1, 1, 255)))
	child := parent.Derive(StyleContextOverride{Materials: reg})
	if parent.Materials != nil {
		t.Fatalf("parent materials should remain nil, got %#v", parent.Materials)
	}
	if child.Materials != reg {
		t.Fatalf("child materials override missing")
	}
}

func TestNearestStyleContext_prefersNearestAncestor(t *testing.T) {
	rootFacet := newTestStyleNode(10)
	childFacet := newTestStyleNode(11)
	leafFacet := newTestStyleNode(12)
	rootFacet.AddChild(childFacet)
	childFacet.AddChild(leafFacet)

	rootStore := NewRootStyleContext(nil, DefaultTokens(), nil)
	childStore := store.NewValueStore(StyleContext{Tokens: DarkTokens(), Materials: NewMaterialRegistry(), Depth: 1})
	rootFacet.style = rootStore
	childFacet.style = childStore

	tree := &testStyleTree{
		nodes: map[uint64]any{
			rootFacet.id:  rootFacet,
			childFacet.id: childFacet,
			leafFacet.id:  leafFacet,
		},
		root: rootStore,
	}

	if got := NearestStyleContext(tree, leafFacet.id); got != childStore {
		t.Fatalf("expected child store, got %#v", got)
	}
}

func TestStyleContextString(t *testing.T) {
	ctx := StyleContext{Depth: 7}
	if got := ctx.String(); !strings.Contains(got, "Depth:7") {
		t.Fatalf("unexpected string: %q", got)
	}
}

func BenchmarkNearestStyleContext_depth10(b *testing.B) {
	tree, leaf := buildStyleTree(10, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NearestStyleContext(tree, leaf.id)
	}
}

func BenchmarkNearestStyleContext_depth1000(b *testing.B) {
	tree, leaf := buildStyleTree(1000, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NearestStyleContext(tree, leaf.id)
	}
}

func buildStyleTree(depth int, withChildContext bool) (*testStyleTree, *testStyleNode) {
	if depth < 1 {
		depth = 1
	}
	nodes := make([]*testStyleNode, depth)
	for i := range nodes {
		nodes[i] = newTestStyleNode(uint64(i + 1))
		if i > 0 {
			nodes[i-1].AddChild(nodes[i])
		}
	}
	rootStore := NewRootStyleContext(nil, DefaultTokens(), nil)
	if withChildContext && depth > 1 {
		childStore := store.NewValueStore(StyleContext{Tokens: DarkTokens(), Materials: NewMaterialRegistry(), Depth: 1})
		nodes[depth-2].style = childStore
	}
	tree := &testStyleTree{
		nodes: make(map[uint64]any, depth),
		root:  rootStore,
	}
	for _, node := range nodes {
		tree.nodes[node.id] = node
	}
	return tree, nodes[depth-1]
}
