package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/marks/structure"
)

func TestTargetModel(t *testing.T) {
	t.Parallel()

	model := TargetModel()
	if err := model.Validate(); err != nil {
		t.Fatalf("TargetModel().Validate() error = %v", err)
	}

	if got := model.PrimarySurfaceIDs(); len(got) != 2 || got[0] != "content" || got[1] != "header" {
		t.Fatalf("PrimarySurfaceIDs() = %v, want [content header]", got)
	}

	if got := model.SecondarySurfaceIDs(); len(got) != 2 || got[0] != "inspector" || got[1] != "sidebar" {
		t.Fatalf("SecondarySurfaceIDs() = %v, want [inspector sidebar]", got)
	}

	if got := model.OptionalSurfaceIDs(); len(got) != 1 || got[0] != "footer" {
		t.Fatalf("OptionalSurfaceIDs() = %v, want [footer]", got)
	}
}

func TestTargetProfile(t *testing.T) {
	t.Parallel()

	profile := TargetProfile(structure.Viewport{Width: 1080, Height: 1920}, structure.Capabilities{Touch: true, IME: true})
	if profile != structure.ProfileMobilePortrait {
		t.Fatalf("TargetProfile() = %v, want %v", profile, structure.ProfileMobilePortrait)
	}
}
