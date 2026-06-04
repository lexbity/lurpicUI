package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/text"
)

func TestFontBytes_not_nil_or_empty(t *testing.T) {
	data := TestFontBytes()
	if len(data) == 0 {
		t.Fatal("TestFontBytes() returned empty slice")
	}
	// TTF header: first 4 bytes should be 0x00010000 for TrueType
	if len(data) < 4 {
		t.Fatal("TestFontBytes() too short")
	}
}

func TestFontBytes_stable(t *testing.T) {
	a := TestFontBytes()
	b := TestFontBytes()
	if len(a) != len(b) {
		t.Fatal("TestFontBytes() returned different sizes across calls")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("TestFontBytes() differs at byte %d", i)
		}
	}
}

func TestFontBoldBytes_not_nil_or_empty(t *testing.T) {
	data := TestFontBoldBytes()
	if len(data) == 0 {
		t.Fatal("TestFontBoldBytes() returned empty slice")
	}
}

func TestFontBoldBytes_different_from_regular(t *testing.T) {
	reg := TestFontBytes()
	bold := TestFontBoldBytes()
	if len(reg) == len(bold) {
		equal := true
		for i := range reg {
			if reg[i] != bold[i] {
				equal = false
				break
			}
		}
		if equal {
			t.Fatal("TestFontBoldBytes() is identical to TestFontBytes()")
		}
	}
}

func TestFontRegistry_never_nil(t *testing.T) {
	reg := TestFontRegistry(t)
	if reg == nil {
		t.Fatal("TestFontRegistry returned nil")
	}
}

func TestFontRegistry_loads_glyph(t *testing.T) {
	reg := TestFontRegistry(t)
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans"})
	if face.IsZero() {
		t.Fatal("TestFontRegistry: resolved face is zero (font not found)")
	}
}

func TestFontRegistry_has_family(t *testing.T) {
	reg := TestFontRegistry(t)
	family := reg.FirstFamily()
	if family == "" {
		t.Fatal("TestFontRegistry: FirstFamily is empty")
	}
}

func TestFontRegistry_sources_non_empty(t *testing.T) {
	reg := TestFontRegistry(t)
	sources := reg.Sources()
	if len(sources) == 0 {
		t.Fatal("TestFontRegistry: Sources() is empty")
	}
}
