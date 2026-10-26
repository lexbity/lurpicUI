package runtime

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// imeProbe hosts a real text field (IME-enabled) — the input.type route a
// real keyboard takes.
type imeProbe struct {
	facet.Facet
	field *input.TextField
}

func (p *imeProbe) Base() *facet.Facet           { return &p.Facet }
func (p *imeProbe) OnAttach(facet.AttachContext) {}
func (p *imeProbe) OnDetach()                    {}
func (p *imeProbe) OnActivate()                  {}
func (p *imeProbe) OnDeactivate()                {}

// TestDriveInputType_IMEFidelity (G5.4): input.type delivers text through
// EventText — the IME route a real keyboard takes — and the focused text
// field's store ends with exactly the synthesized text.
func TestDriveInputType_IMEFidelity(t *testing.T) {
	scope := store.NewScope()
	q := &recordQueue{}
	value := store.NewValueStore(scope, "")
	field := input.NewTextField(scope, "Source name", uiinput.TextInputOutlined, value)
	field.SetTestID("probe-field")

	root := &imeProbe{}
	root.Facet = facet.NewFacet()
	root.BindImpl(root)
	root.SetTestID("ime-root")
	root.field = field
	root.AddChild(field.Base())

	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.StoreScope = scope
	cfg.StoreIdentity = store.NewRuntimeIdentity()
	cfg.HostedChildren = marks.HostedChildrenFor
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	rt, err := New(cfg, &inputQueueApp{q: q}, &testWindow{width: 200, height: 100}, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	defer rt.Shutdown()
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rt.RunOneFrameAt(epoch)

	res := inputDriveCall(t, rt, "input.focus", map[string]any{
		"target": map[string]string{"testid": "probe-field"},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("input.focus: %v", res.Err)
	}

	res = inputDriveCall(t, rt, "input.type", map[string]any{"text": "hello"}, allGrants())
	if res.Err != nil {
		t.Fatalf("input.type: %v", res.Err)
	}
	if got := value.Get(); got != "hello" {
		t.Fatalf("field value = %q, want %q", got, "hello")
	}
	// The synthesized text event carried SourceDrive.
	sawDriveText := false
	for _, ev := range q.seenDriveEvents() {
		if te, ok := ev.(platform.EventText); ok && te.Source == platform.SourceDrive {
			sawDriveText = true
		}
	}
	if !sawDriveText {
		t.Fatal("input.type must synthesize a SourceDrive EventText (the IME route)")
	}
}

var (
	_ = gfx.Point{}
	_ = theme.DefaultTokens
)
