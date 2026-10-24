package equivalence

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// The fixtures are the corpus's mark trees: real production marks mounted as
// harness roots (the Q7 blessed path — the mark fills the harness window),
// with the mutation-observable stores registered under stable names for
// trajectory capture. Both corpus paths boot from the same builder; only the
// interaction differs.

// fixtureHarness boots a DriveHarness over the mark root.
func fixtureHarness(t testing.TB, root facet.FacetImpl) *Fixture {
	t.Helper()
	return fixtureHarnessWithRegistry(t, root, nil)
}

// fixtureHarnessWithRegistry additionally wires the command registry (the
// cmd.execute fixture).
func fixtureHarnessWithRegistry(t testing.TB, root facet.FacetImpl, reg *runtime.CommandRegistry) *Fixture {
	t.Helper()
	cfg := testkit.StandardHarnessConfig(t, 480, 240)
	cfg.CommandRegistry = reg
	h := testkit.NewDriveHarnessWithConfig(t, cfg, root)
	// Settle frame: the first projection builds the hit map (testkit's
	// documented first-frame contract) — both corpus paths boot from the
	// same settled state.
	h.RunFrame()
	return &Fixture{Harness: h}
}

// newButtonFixture builds a Button whose activation increments a named
// counter store (the click/move observable).
func newButtonFixture(t testing.TB) *Fixture {
	t.Helper()
	clicks := store.NewValueStore(0)
	btn := action.NewButton(marks.Const("OK"), marks.Const(uiinput.ButtonFilled))
	btn.Activated.Subscribe(func(signal.Unit) {
		clicks.Set(clicks.Get() + 1)
	})
	fix := fixtureHarness(t, btn)
	fix.RegisterNames(t, map[string]versionedStore{
		"clicks": clicks,
	})
	return fix
}

// newSliderFixture builds a Slider over a named value store.
func newSliderFixture(t testing.TB) *Fixture {
	t.Helper()
	value := store.NewValueStore(0.5)
	slider := selection.NewSlider("Opacity", 0, 1, 0.1, value)
	fix := fixtureHarness(t, slider)
	fix.RegisterNames(t, map[string]versionedStore{
		"slider.value": value,
	})
	return fix
}

// newTextFixture builds two IME text fields over named value stores. Field A
// starts focused (via the input.focus verb at boot) so the tab-route GUI twin
// has a real focus to move — the input system's tab advance requires a
// current focus target.
func newTextFixture(t testing.TB) *Fixture {
	t.Helper()
	valueA := store.NewValueStore("")
	valueB := store.NewValueStore("")
	fieldA := input.NewTextField("Source name", uiinput.TextInputOutlined, valueA)
	fieldA.Base().SetTestID("field-a")
	fieldB := input.NewTextField("Target name", uiinput.TextInputOutlined, valueB)
	fieldB.Base().SetTestID("field-b")

	root := &twoFieldRoot{}
	root.Facet = facet.NewFacet()
	root.BindImpl(root)
	root.AddChild(fieldA.Base())
	root.AddChild(fieldB.Base())

	fix := fixtureHarness(t, root)
	if _, perr := fix.Harness.Call("input.focus", map[string]any{
		"target": map[string]string{"testid": "field-a"},
	}); perr != nil {
		t.Fatalf("focus field-a: %v", perr)
	}
	fix.RegisterNames(t, map[string]versionedStore{
		"field.a": valueA,
		"field.b": valueB,
	})
	return fix
}

// twoFieldRoot is a minimal structured host for the two-field fixture.
type twoFieldRoot struct {
	facet.Facet
}

func (r *twoFieldRoot) Base() *facet.Facet           { return &r.Facet }
func (r *twoFieldRoot) OnAttach(facet.AttachContext) {}
func (r *twoFieldRoot) OnDetach()                    {}
func (r *twoFieldRoot) OnActivate()                  {}
func (r *twoFieldRoot) OnDeactivate()                {}

// newScrollFixture builds a scroll region (the scroll-observable host).
func newScrollFixture(t testing.TB) *Fixture {
	t.Helper()
	region := structure.NewScrollRegion("Feed")
	return fixtureHarness(t, region)
}

// newPaletteFixture builds the cmd.execute fixture: a registry command whose
// Execute flips a named store, bound to an activation button. The drive verb
// dispatches registry.Execute directly; the GUI twin clicks the button —
// palette activation routes through the same registry.Execute dispatch.
func newPaletteFixture(t testing.TB) *Fixture {
	t.Helper()
	toggled := store.NewValueStore(false)
	registry := runtime.NewCommandRegistry()
	registry.Register(runtime.CommandEntry{
		ID:    "toggle.enabled",
		Title: "Toggle enabled",
		Execute: func() {
			toggled.Set(!toggled.Get())
		},
	})
	btn := action.NewButton(marks.Const("Toggle"), marks.Const(uiinput.ButtonFilled))
	btn.Activated.Subscribe(func(signal.Unit) {
		registry.Execute("toggle.enabled")
	})
	fix := fixtureHarnessWithRegistry(t, btn, registry)
	fix.RegisterNames(t, map[string]versionedStore{
		"toggled": toggled,
	})
	return fix
}

// newValueFixture builds a checkbox bound to a named store — the indirect
// GUI twin of store.write.
func newValueFixture(t testing.TB) *Fixture {
	t.Helper()
	value := store.NewValueStore(selection.CheckboxStateOff)
	box := selection.NewCheckbox("Enabled", value)
	fix := fixtureHarness(t, box)
	fix.RegisterNames(t, map[string]versionedStore{
		"checkbox": value,
	})
	return fix
}
