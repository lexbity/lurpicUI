package shell

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

	if got := model.PrimarySurfaceIDs(); len(got) != 2 || got[0] != "scene-host" || got[1] != "topbar" {
		t.Fatalf("PrimarySurfaceIDs() = %v, want [scene-host topbar]", got)
	}

	if got := model.SecondarySurfaceIDs(); len(got) != 2 || got[0] != "diagnostics" || got[1] != "scene-nav" {
		t.Fatalf("SecondarySurfaceIDs() = %v, want [diagnostics scene-nav]", got)
	}

	if got := model.OptionalSurfaceIDs(); len(got) != 1 || got[0] != "logs" {
		t.Fatalf("OptionalSurfaceIDs() = %v, want [logs]", got)
	}
}

func TestTargetProfile(t *testing.T) {
	t.Parallel()

	profile := TargetProfile(structure.Viewport{Width: 1600, Height: 900}, structure.Capabilities{Hover: true, Keyboard: true})
	if profile != structure.ProfileDesktopDense {
		t.Fatalf("TargetProfile() = %v, want %v", profile, structure.ProfileDesktopDense)
	}
}
