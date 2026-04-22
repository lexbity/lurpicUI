package templates

import "testing"

func TestUneNuitTheme_matchesPlan(t *testing.T) {
	theme := UneNuit()
	if theme.Name != "uneNuit" {
		t.Fatalf("name = %q", theme.Name)
	}
	if !theme.Metadata.Dark {
		t.Fatal("uneNuit should be dark")
	}
	if theme.Metadata.BaselineDensity != DensityRegular {
		t.Fatalf("baseline density = %s", theme.Metadata.BaselineDensity)
	}
	if theme.Tokens.Color.Background != colorHex(0x282C34FF) {
		t.Fatalf("background = %#v", theme.Tokens.Color.Background)
	}
	if !theme.Charts.HasOverrides() {
		t.Fatal("uneNuit should define chart overrides")
	}
	if err := theme.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestSythiqueTheme_matchesPlan(t *testing.T) {
	theme := Sythique()
	if theme.Name != "sythique" {
		t.Fatalf("name = %q", theme.Name)
	}
	if !theme.Metadata.Dark {
		t.Fatal("sythique should be dark")
	}
	if theme.Metadata.BaselineDensity != DensityCompact {
		t.Fatalf("baseline density = %s", theme.Metadata.BaselineDensity)
	}
	if theme.Tokens.Color.Primary != colorHex(0x36F9F6FF) {
		t.Fatalf("primary = %#v", theme.Tokens.Color.Primary)
	}
	if theme.Diagnostics(DensityCompact).ChartPaletteSize != 7 {
		t.Fatalf("unexpected chart palette size")
	}
	if err := theme.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestNotesTheme_matchesPlan(t *testing.T) {
	theme := Notes()
	if theme.Name != "notes" {
		t.Fatalf("name = %q", theme.Name)
	}
	if theme.Metadata.Dark {
		t.Fatal("notes should be light")
	}
	if theme.Metadata.BaselineDensity != DensityRegular {
		t.Fatalf("baseline density = %s", theme.Metadata.BaselineDensity)
	}
	if theme.Tokens.Color.Surface != colorHex(0xF3F3F3FF) {
		t.Fatalf("surface = %#v", theme.Tokens.Color.Surface)
	}
	if !theme.Charts.HasOverrides() {
		t.Fatal("notes should define chart overrides")
	}
	if err := theme.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
