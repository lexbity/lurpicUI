package facets

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/testkit"
	"codeburg.org/lexbit/voicedsp"
)

func TestMeterFacet_projects_and_invalidates(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	f := NewMeterFacet(svc)
	facet.Attach(f, facet.AttachContext{})
	svc.Stores().Params.Set(voiceux.AudioParamsView{RMS: 0.75, Peak: 0.9, Clipping: true})
	if got := f.DirtyFlags(); got == 0 {
		t.Fatal("expected dirty flags after params update")
	}
	snap := f.Snapshot(gfx.RectFromXYWH(0, 0, 100, 40))
	if len(snap.Layers) < 5 {
		t.Fatalf("expected meter layers, got %d", len(snap.Layers))
	}
}

func TestPresetBrowser_filter_and_select(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	svc.Stores().Presets.Replace([]voiceux.PresetView{
		{ID: "natural", Name: "Natural"},
		{ID: "robot", Name: "Robot"},
	})
	f := NewPresetBrowserFacet(svc)
	facet.Attach(f, facet.AttachContext{})
	f.SetFilter("ro")
	cards := f.Cards()
	if len(cards) != 1 || cards[0].ID != "robot" {
		t.Fatalf("filtered cards = %#v", cards)
	}
	if err := f.SelectPreset("robot"); err != nil {
		t.Fatalf("select preset: %v", err)
	}
	if got := svc.LastCommand(); got == nil {
		t.Fatal("expected dispatched command")
	}
}

func TestDeviceSelector_dispatches_actions(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	f := NewDeviceSelectorFacet(svc)
	facet.Attach(f, facet.AttachContext{})
	svc.Stores().Devices.Set(voiceux.AudioDeviceSnapshot{
		Inputs: []voicedsp.DeviceInfo{{ID: "mic", Name: "Mic", Input: true}},
	})
	if err := f.RefreshDevices(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if err := f.SelectInput("mic"); err != nil {
		t.Fatalf("select input: %v", err)
	}
	if got, ok := svc.LastAction(); !ok || got.ID != "select_input" {
		t.Fatalf("last action = %#v ok=%v", got, ok)
	}
}

func TestFXChain_reorder_and_param_dispatch(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	svc.Stores().FXChain.Replace([]voiceux.FXSlotView{
		{ID: "a", Enabled: true},
		{ID: "b", Enabled: true},
		{ID: "c", Enabled: true},
	})
	f := NewFXChainFacet(svc)
	facet.Attach(f, facet.AttachContext{})
	if err := f.SetEffectParam("fx1", "mix", 0.5); err != nil {
		t.Fatalf("set effect param: %v", err)
	}
	if err := f.ToggleBypass(true); err != nil {
		t.Fatalf("toggle bypass: %v", err)
	}
	if err := f.Reorder(0, 2); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if got := svc.Stores().FXChain.All()[2].ID; got != "a" {
		t.Fatalf("reordered chain tail = %q", got)
	}
}

func TestCalibrationFlow_projection_and_commands(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	f := NewCalibrationFlowFacet(svc)
	facet.Attach(f, facet.AttachContext{})
	svc.Stores().Calibration.Set(voiceux.CalibrationStateView{
		Phase:     voicedsp.CalibrationPromptA,
		CanCommit: true,
		Progress:  voicedsp.CalibrationProgress{Completed: 2, Total: 5, Quality: voicedsp.CalibrationQualityGood},
	})
	panel := f.Projection(gfx.RectFromXYWH(0, 0, 420, 320))
	if panel.Title == "" || panel.AcceptRect.Width() == 0 {
		t.Fatalf("unexpected calibration panel %#v", panel)
	}
	if err := f.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := f.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := f.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestVowelSpace_projection_maps_live_point(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	f := NewVowelSpaceFacet(svc)
	facet.Attach(f, facet.AttachContext{})
	svc.Stores().Params.Set(voiceux.AudioParamsView{F1Hz: 650, F2Hz: 1700, Vowel: voicedsp.VowelA, FormantConf: 0.8})
	snap := f.Projection(gfx.RectFromXYWH(0, 0, 200, 200))
	if len(snap.Points) == 0 {
		t.Fatal("expected vowel points")
	}
}

func TestMixer_and_stream_bindings(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	svc.Stores().Mixer.Set(voiceux.MixerStateView{
		Buses: []voiceux.MixerBusView{{Bus: voicedsp.BusVoiceProcessed, Gain: 0.5}},
	})
	mixer := NewMixerStripFacet(svc)
	stream := NewStreamWidgetFacet(svc)
	facet.Attach(mixer, facet.AttachContext{})
	facet.Attach(stream, facet.AttachContext{})
	if err := mixer.SetBusGain(voicedsp.BusVoiceProcessed, 0.8); err != nil {
		t.Fatalf("set bus gain: %v", err)
	}
	if err := mixer.SetMonitorEnabled(true); err != nil {
		t.Fatalf("set monitor: %v", err)
	}
	if got := stream.Snapshot(gfx.RectFromXYWH(0, 0, 100, 40)); len(got.Layers) == 0 {
		t.Fatal("expected stream snapshot layers")
	}
}
