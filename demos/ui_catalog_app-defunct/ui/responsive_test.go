package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/marks/structure"
)

func TestCatalogResponsiveLayout(t *testing.T) {
	t.Parallel()

	desktop := structure.ResponsiveLayoutForViewport(
		structure.Viewport{Width: 1600, Height: 900},
		structure.Capabilities{Hover: true, Keyboard: true},
	)
	if desktop.Variant != structure.ShellVariantDesktopDense {
		t.Fatalf("desktop.Variant = %v, want %v", desktop.Variant, structure.ShellVariantDesktopDense)
	}
	if desktop.Navigation != structure.NavigationSidebar {
		t.Fatalf("desktop.Navigation = %v, want %v", desktop.Navigation, structure.NavigationSidebar)
	}

	mobile := structure.ResponsiveLayoutForViewport(
		structure.Viewport{Width: 720, Height: 1280},
		structure.Capabilities{Touch: true},
	)
	if mobile.Variant != structure.ShellVariantMobilePortrait {
		t.Fatalf("mobile.Variant = %v, want %v", mobile.Variant, structure.ShellVariantMobilePortrait)
	}
	if mobile.Navigation != structure.NavigationDrawer {
		t.Fatalf("mobile.Navigation = %v, want %v", mobile.Navigation, structure.NavigationDrawer)
	}

	model := TargetModel()
	visible := model.VisibleSurfaceIDs(mobile)
	if len(visible) != 2 || visible[0] != "content" || visible[1] != "header" {
		t.Fatalf("mobile visible surfaces = %v, want [content header]", visible)
	}
}
