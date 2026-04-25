package shell

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/marks/structure"
)

func TestDiagnosticResponsiveLayout(t *testing.T) {
	t.Parallel()

	desktop := structure.ResponsiveLayoutForViewport(
		structure.Viewport{Width: 1440, Height: 900},
		structure.Capabilities{Hover: true, Keyboard: true},
	)
	if desktop.Variant != structure.ShellVariantDesktopDense {
		t.Fatalf("desktop.Variant = %v, want %v", desktop.Variant, structure.ShellVariantDesktopDense)
	}

	mobile := structure.ResponsiveLayoutForViewport(
		structure.Viewport{Width: 800, Height: 1280},
		structure.Capabilities{Touch: true, IME: true},
	)
	if mobile.Variant != structure.ShellVariantMobilePortrait {
		t.Fatalf("mobile.Variant = %v, want %v", mobile.Variant, structure.ShellVariantMobilePortrait)
	}
	if !mobile.SecondaryCollapsed || !mobile.OptionalCollapsed {
		t.Fatalf("mobile collapse flags = secondary:%v optional:%v, want true/true", mobile.SecondaryCollapsed, mobile.OptionalCollapsed)
	}

	model := TargetModel()
	visible := model.VisibleSurfaceIDs(mobile)
	if len(visible) != 2 || visible[0] != "scene-host" || visible[1] != "topbar" {
		t.Fatalf("mobile visible surfaces = %v, want [scene-host topbar]", visible)
	}
}
