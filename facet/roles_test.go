package facet

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

type lifecycleRole struct {
	attachCalled     int
	activateCalled   int
	deactivateCalled int
	disposeCalled    int
}

func (r *lifecycleRole) onAttach(*Facet)     { r.attachCalled++ }
func (r *lifecycleRole) onActivate(*Facet)   { r.activateCalled++ }
func (r *lifecycleRole) onDeactivate(*Facet) { r.deactivateCalled++ }
func (r *lifecycleRole) onDispose(*Facet)    { r.disposeCalled++ }

func TestRole_lifecycle_hooks_called(t *testing.T) {
	f := &Facet{}
	f.id = nextID()
	f.state = StateCreated

	role := &lifecycleRole{}
	f.roles = []Role{role}

	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)

	if role.attachCalled != 1 {
		t.Fatalf("expected attach hook once, got %d", role.attachCalled)
	}
	if role.activateCalled != 1 {
		t.Fatalf("expected activate hook once, got %d", role.activateCalled)
	}
	if role.deactivateCalled != 1 {
		t.Fatalf("expected deactivate hook once, got %d", role.deactivateCalled)
	}
	if role.disposeCalled != 1 {
		t.Fatalf("expected dispose hook once, got %d", role.disposeCalled)
	}
}

func TestFacet_role_accessor_returns_nil_when_absent(t *testing.T) {
	f := NewFacet()
	if f.LayoutRole() != nil {
		t.Fatal("expected nil layout role")
	}
	if f.RenderRole() != nil {
		t.Fatal("expected nil render role")
	}
	if f.HitRole() != nil {
		t.Fatal("expected nil hit role")
	}
	if f.InputRole() != nil {
		t.Fatal("expected nil input role")
	}
	if f.FocusRole() != nil {
		t.Fatal("expected nil focus role")
	}
	if f.ViewportRole() != nil {
		t.Fatal("expected nil viewport role")
	}
	if f.ProjectionRole() != nil {
		t.Fatal("expected nil projection role")
	}
	if f.TickRole() != nil {
		t.Fatal("expected nil tick role")
	}
}

func TestFacet_role_accessor_returns_role_when_present(t *testing.T) {
	f := NewFacet()
	layout := &LayoutRole{}
	f.AddRole(layout)
	if got := f.LayoutRole(); got != layout {
		t.Fatalf("expected layout accessor to return registered role, got %#v", got)
	}
}

func TestFacet_multiple_roles_all_accessible(t *testing.T) {
	f := NewFacet()
	layout := &LayoutRole{}
	render := &RenderRole{}
	hit := &HitRole{}
	f.AddRole(layout)
	f.AddRole(render)
	f.AddRole(hit)

	if f.LayoutRole() != layout {
		t.Fatal("expected layout role")
	}
	if f.RenderRole() != render {
		t.Fatal("expected render role")
	}
	if f.HitRole() != hit {
		t.Fatal("expected hit role")
	}
}

func TestTickRole_self_extinguishing(t *testing.T) {
	role := &TickRole{}
	if role.IsActive() {
		t.Fatal("expected inactive tick role initially")
	}
	role.RequestTick()
	if !role.IsActive() {
		t.Fatal("expected active after RequestTick")
	}
	role.Reset()
	if role.IsActive() {
		t.Fatal("expected inactive after Reset")
	}
}

func TestTickRole_request_tick_reactivates(t *testing.T) {
	role := &TickRole{}
	role.RequestTick()
	role.Reset()
	if role.IsActive() {
		t.Fatal("expected reset to win")
	}
	role.RequestTick()
	if !role.IsActive() {
		t.Fatal("expected request tick to reactivate")
	}
}

func TestCursorShape_constants_distinct(t *testing.T) {
	seen := map[CursorShape]struct{}{}
	shapes := []CursorShape{
		CursorDefault,
		CursorPointer,
		CursorText,
		CursorCrosshair,
		CursorGrab,
		CursorGrabbing,
		CursorResize,
		CursorNotAllowed,
	}
	for _, shape := range shapes {
		if _, ok := seen[shape]; ok {
			t.Fatalf("duplicate cursor shape value %d", shape)
		}
		seen[shape] = struct{}{}
	}
}

func TestMarkID_zero_is_facet_itself(t *testing.T) {
	if MarkID(0) != 0 {
		t.Fatal("expected MarkID(0) to be the zero sentinel")
	}
}

func TestRoleInterface_not_implementable_externally(t *testing.T) {
	cmd := exec.Command("go", "test", "-tags=rolenegative", "./testdata/roleexternal")
	cmd.Env = append(cmd.Environ(), "GOCACHE=/tmp/lurpic-go-cache", "GOTMPDIR=/tmp/lurpic-go-tmp")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected external role package to fail compilation")
	}
	if !strings.Contains(string(out), "does not implement") {
		t.Fatalf("expected compile failure mentioning interface mismatch, got:\n%s", out)
	}
}

func TestViewportRole_screento_world_roundtrip(t *testing.T) {
	v := &ViewportRole{}
	v.SetPanZoom(gfx.Point{X: 10, Y: -4}, 2)
	screen := v.WorldToScreen(gfx.Point{X: 3, Y: 7})
	world, ok := v.ScreenToWorld(screen)
	if !ok {
		t.Fatal("expected invertible transform")
	}
	if world != (gfx.Point{X: 3, Y: 7}) {
		t.Fatalf("unexpected roundtrip result: %#v", world)
	}
}

func TestViewportRole_screento_world_degenerate(t *testing.T) {
	v := &ViewportRole{}
	v.Transform = gfx.Scale(0, 0)
	if _, ok := v.ScreenToWorld(gfx.Point{}); ok {
		t.Fatal("expected degenerate transform to fail")
	}
}

func TestViewportRole_setpanzoom_updates_transform(t *testing.T) {
	v := &ViewportRole{}
	v.SetPanZoom(gfx.Point{X: 12, Y: 8}, 3)
	if v.Transform.A != 3 || v.Transform.D != 3 || v.Transform.TX != 12 || v.Transform.TY != 8 {
		t.Fatalf("unexpected transform after SetPanZoom: %#v", v.Transform)
	}
}

func TestInputRole_onpointer_nil_safe(t *testing.T) {
	role := &InputRole{}
	if role.OnPointer != nil {
		t.Fatal("expected nil handler")
	}
	if role.OnScroll != nil || role.OnKey != nil || role.OnText != nil {
		t.Fatal("expected all handlers nil by default")
	}
}

func TestTickRole_self_extinguishing_stops_after_n_frames(t *testing.T) {
	role := &TickRole{}
	ticks := 0
	role.OnTick = func(dt time.Duration) {
		ticks++
	}
	role.RequestTick()
	if !role.IsActive() {
		t.Fatal("expected tick role to be active after RequestTick")
	}
	role.Reset()
	if role.IsActive() {
		t.Fatal("expected tick role to deactivate after Reset")
	}
	if ticks != 0 {
		t.Fatalf("expected no automatic ticks in the base phase, got %d", ticks)
	}
}
