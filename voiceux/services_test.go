package voiceux_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/testkit"
)

func TestDefaultDescriptorRegistryIncludesVoiceUXDescriptors(t *testing.T) {
	reg := voiceux.DefaultDescriptorRegistry()
	if got := len(reg.Marks()); got != 8 {
		t.Fatalf("expected 8 mark descriptors, got %d", got)
	}
	if got := len(reg.Facets()); got != 8 {
		t.Fatalf("expected 8 facet descriptors, got %d", got)
	}
	if got := len(reg.ThemeSlots()); got != 15 {
		t.Fatalf("expected 15 theme slots, got %d", got)
	}
}

func TestDefaultDescriptorRegistrySnapshot(t *testing.T) {
	reg := voiceux.DefaultDescriptorRegistry()
	gotMarks := make([]string, 0, len(reg.Marks()))
	for _, d := range reg.Marks() {
		gotMarks = append(gotMarks, string(d.Type))
	}
	wantMarks := []string{
		"voice_calibration_flow",
		"voice_device_selector",
		"voice_fx_chain",
		"voice_meter",
		"voice_mixer_strip",
		"voice_preset_browser",
		"voice_stream_widget",
		"voice_vowel_space",
	}
	if len(gotMarks) != len(wantMarks) {
		t.Fatalf("mark snapshot length = %d, want %d", len(gotMarks), len(wantMarks))
	}
	for i := range wantMarks {
		if gotMarks[i] != wantMarks[i] {
			t.Fatalf("mark snapshot[%d] = %q, want %q", i, gotMarks[i], wantMarks[i])
		}
	}
	gotThemes := make([]string, 0, len(reg.ThemeSlots()))
	for _, slot := range reg.ThemeSlots() {
		gotThemes = append(gotThemes, slot.Name)
	}
	wantThemes := []string{
		"accent", "accent_muted", "background", "calibration_prompt", "focus_ring",
		"meter_bad", "meter_good", "meter_idle", "meter_warn", "surface",
		"surface_raised", "text", "text_muted", "vowel_live", "vowel_stable",
	}
	for i := range wantThemes {
		if gotThemes[i] != wantThemes[i] {
			t.Fatalf("theme snapshot[%d] = %q, want %q", i, gotThemes[i], wantThemes[i])
		}
	}
}

func TestFakeVoiceServiceRecordsCommandsAndActions(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	if svc.Stores() == nil {
		t.Fatal("expected stores")
	}
	if svc.DescriptorRegistry() == nil {
		t.Fatal("expected registry")
	}
	if err := svc.DispatchVoiceCommand(voiceux.SetPresetCommand{}); err != nil {
		t.Fatalf("dispatch command: %v", err)
	}
	if err := svc.DispatchAction("refresh", map[string]any{"ok": true}); err != nil {
		t.Fatalf("dispatch action: %v", err)
	}
	if got := svc.LastCommand(); got == nil {
		t.Fatal("expected last command")
	}
	if got, ok := svc.LastAction(); !ok || got.ID != "refresh" {
		t.Fatalf("last action = %#v, ok=%v", got, ok)
	}
}
