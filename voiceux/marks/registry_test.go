package marks

import (
	"testing"
)

func TestVoiceUXMarkDescriptorsRegistered(t *testing.T) {
	cases := []struct {
		name string
		typ  TypeName
	}{
		{name: "device selector", typ: deviceSelectorType},
		{name: "meter", typ: meterType},
		{name: "preset browser", typ: presetBrowserType},
		{name: "fx chain", typ: fxChainType},
		{name: "calibration flow", typ: calibrationFlowType},
		{name: "vowel space", typ: vowelSpaceType},
		{name: "mixer strip", typ: mixerStripType},
		{name: "stream widget", typ: streamWidgetType},
	}
	for _, tc := range cases {
		found := false
		for _, d := range Descriptors() {
			if d.Type == tc.typ {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing descriptor for %s", tc.name)
		}
	}
}

func TestVoiceUXMarkConstructorsExposeDescriptors(t *testing.T) {
	mark := NewMeterMark("meter-1")
	if got := mark.AuthoredID(); got != "meter-1" {
		t.Fatalf("AuthoredID = %q", got)
	}
	if got := mark.Descriptor().Type; got != meterType {
		t.Fatalf("Descriptor type = %q", got)
	}
}
