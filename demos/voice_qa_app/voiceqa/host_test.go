package voiceqa

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

func TestHostSyncPopulatesStores(t *testing.T) {
	host := NewHost(HostOptions{FakeAudio: true})
	if err := host.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = host.Stop() }()

	host.Sync()

	if got := host.Stores().Devices.Get(); len(got.Inputs) == 0 || len(got.Outputs) == 0 {
		t.Fatalf("device snapshot not populated: %#v", got)
	}
	if got := host.Stores().Presets.All(); len(got) == 0 {
		t.Fatal("expected preset list")
	}
	if got := host.Stores().ActivePreset.Get(); got != voicedsp.PresetID("natural") {
		t.Fatalf("active preset = %q, want natural", got)
	}
	if err := host.DispatchVoiceCommand(voiceux.SetBypassCommand{Enabled: true}); err != nil {
		t.Fatalf("dispatch bypass: %v", err)
	}
	if got := host.Stores().FXBypassed.Get(); !got {
		t.Fatal("expected bypass state to update")
	}
	if err := host.DispatchVoiceCommand(voiceux.SetPresetCommand{ID: voicedsp.PresetID("robot")}); err != nil {
		t.Fatalf("dispatch preset: %v", err)
	}
	if got := host.Stores().ActivePreset.Get(); got != voicedsp.PresetID("robot") {
		t.Fatalf("active preset = %q, want robot", got)
	}
}

func TestRootFacetBuildsCatalogAndLayers(t *testing.T) {
	host := NewHost(HostOptions{FakeAudio: true})
	if err := host.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = host.Stop() }()
	host.Sync()

	registry, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("font registry: %v", err)
	}
	shaper := text.NewShaper(registry)
	root := NewRootFacet(theme.Default(), shaper, host)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)
	root.layout.OnArrange(gfx.RectFromXYWH(0, 0, 1680, 1120))
	specs := root.OnLayerSpecs()
	if len(specs) != 2 {
		t.Fatalf("layer spec count = %d, want 2", len(specs))
	}
	if got := len(host.DescriptorRegistry().Marks()); got == 0 {
		t.Fatal("expected voice mark descriptors")
	}
}
