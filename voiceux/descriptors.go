package voiceux

import (
	"sort"

	voicemark "codeburg.org/lexbit/lurpicui/voiceux/marks"
)

// FacetDescriptor describes one reusable Voice UX facet.
type FacetDescriptor struct {
	ID          string
	Mark        voicemark.TypeName
	Name        string
	Description string
	Roles       []string
}

// ThemeSlot describes one semantic Voice UX theme slot.
type ThemeSlot struct {
	Name        string
	Description string
	Token       string
}

// ThemeSlots groups the semantic slots used by Voice UX components.
type ThemeSlots struct {
	Background        string
	Surface           string
	SurfaceRaised     string
	Accent            string
	AccentMuted       string
	Text              string
	MutedText         string
	FocusRing         string
	MeterIdle         string
	MeterGood         string
	MeterWarn         string
	MeterBad          string
	CalibrationPrompt string
	VowelLive         string
	VowelStable       string
}

// DescriptorRegistry exposes the descriptors needed by host tooling.
type DescriptorRegistry interface {
	Marks() []voicemark.Descriptor
	Facets() []FacetDescriptor
	ThemeSlots() []ThemeSlot
}

// StaticDescriptorRegistry is a simple immutable registry implementation.
type StaticDescriptorRegistry struct {
	MarkList  []voicemark.Descriptor
	FacetList []FacetDescriptor
	ThemeList []ThemeSlot
}

// DefaultDescriptorRegistry returns the built-in Voice UX descriptor set.
func DefaultDescriptorRegistry() StaticDescriptorRegistry {
	return StaticDescriptorRegistry{
		MarkList:  MarkDescriptors(),
		FacetList: DefaultFacetDescriptors(),
		ThemeList: DefaultThemeSlots().List(),
	}
}

// DefaultFacetDescriptors returns the built-in Voice UX facet descriptors.
func DefaultFacetDescriptors() []FacetDescriptor {
	return []FacetDescriptor{
		{ID: "voice.device_selector", Mark: "voice_device_selector", Name: "Voice Device Selector", Description: "Select capture, output, and monitor devices.", Roles: []string{"layout", "render", "hit", "input", "focus"}},
		{ID: "voice.meter", Mark: "voice_meter", Name: "Voice Meter", Description: "Show live level, pitch, mouth, and confidence meters.", Roles: []string{"layout", "projection", "render", "tick"}},
		{ID: "voice.preset_browser", Mark: "voice_preset_browser", Name: "Voice Preset Browser", Description: "Browse and select voice presets.", Roles: []string{"layout", "render", "hit", "input", "focus"}},
		{ID: "voice.fx_chain", Mark: "voice_fx_chain", Name: "Voice FX Chain", Description: "Inspect and edit the active effect chain.", Roles: []string{"layout", "render", "hit", "input", "focus"}},
		{ID: "voice.calibration_flow", Mark: "voice_calibration_flow", Name: "Voice Calibration Flow", Description: "Guide calibration capture, review, and commit.", Roles: []string{"layout", "projection", "render", "hit", "input", "focus", "tick"}},
		{ID: "voice.vowel_space", Mark: "voice_vowel_space", Name: "Vowel Space", Description: "Plot live and calibrated F1/F2 points.", Roles: []string{"layout", "projection", "render", "hit", "viewport"}},
		{ID: "voice.mixer_strip", Mark: "voice_mixer_strip", Name: "Voice Mixer Strip", Description: "Control bus gains and show bus meters.", Roles: []string{"layout", "render", "hit", "input"}},
		{ID: "voice.stream_widget", Mark: "voice_stream_widget", Name: "Voice Stream Widget", Description: "Compact panel-ready voice controls and meters.", Roles: []string{"layout", "projection", "render", "tick"}},
	}
}

// DefaultThemeSlots returns the built-in Voice UX theme slot set.
func DefaultThemeSlots() ThemeSlots {
	return ThemeSlots{
		Background:        "voice.background",
		Surface:           "voice.surface",
		SurfaceRaised:     "voice.surface_raised",
		Accent:            "voice.accent",
		AccentMuted:       "voice.accent_muted",
		Text:              "voice.text",
		MutedText:         "voice.text_muted",
		FocusRing:         "voice.focus_ring",
		MeterIdle:         "voice.meter.idle",
		MeterGood:         "voice.meter.good",
		MeterWarn:         "voice.meter.warn",
		MeterBad:          "voice.meter.bad",
		CalibrationPrompt: "voice.calibration.prompt",
		VowelLive:         "voice.vowel.live",
		VowelStable:       "voice.vowel.stable",
	}
}

// List returns the theme slots as sorted semantic records.
func (s ThemeSlots) List() []ThemeSlot {
	slots := []ThemeSlot{
		{Name: "background", Description: "Page background and stage canvas", Token: s.Background},
		{Name: "surface", Description: "Default card surface", Token: s.Surface},
		{Name: "surface_raised", Description: "Raised panel surface", Token: s.SurfaceRaised},
		{Name: "accent", Description: "Primary accent color", Token: s.Accent},
		{Name: "accent_muted", Description: "Muted accent and secondary chrome", Token: s.AccentMuted},
		{Name: "text", Description: "Primary text color", Token: s.Text},
		{Name: "text_muted", Description: "Muted text and labels", Token: s.MutedText},
		{Name: "focus_ring", Description: "Keyboard focus ring", Token: s.FocusRing},
		{Name: "meter_idle", Description: "Idle meter fill", Token: s.MeterIdle},
		{Name: "meter_good", Description: "Healthy meter fill", Token: s.MeterGood},
		{Name: "meter_warn", Description: "Warning meter fill", Token: s.MeterWarn},
		{Name: "meter_bad", Description: "Clipping or failure meter fill", Token: s.MeterBad},
		{Name: "calibration_prompt", Description: "Calibration prompt chrome", Token: s.CalibrationPrompt},
		{Name: "vowel_live", Description: "Live vowel point color", Token: s.VowelLive},
		{Name: "vowel_stable", Description: "Stable vowel point color", Token: s.VowelStable},
	}
	sort.SliceStable(slots, func(i, j int) bool { return slots[i].Name < slots[j].Name })
	return slots
}

func (r StaticDescriptorRegistry) Marks() []voicemark.Descriptor {
	out := append([]voicemark.Descriptor(nil), r.MarkList...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Type < out[j].Type
	})
	return out
}

func (r StaticDescriptorRegistry) Facets() []FacetDescriptor {
	out := append([]FacetDescriptor(nil), r.FacetList...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (r StaticDescriptorRegistry) ThemeSlots() []ThemeSlot {
	out := append([]ThemeSlot(nil), r.ThemeList...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
