package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/ui_catalog/store"
)

func TestLayoutProfileForDensity(t *testing.T) {
	compact := LayoutProfileForDensity(store.DensityCompact)
	normal := LayoutProfileForDensity(store.DensityNormal)
	comfortable := LayoutProfileForDensity(store.DensityComfortable)

	if !(compact.CardWidth < normal.CardWidth && normal.CardWidth < comfortable.CardWidth) {
		t.Fatalf("card widths not ordered by density: compact=%v normal=%v comfortable=%v", compact.CardWidth, normal.CardWidth, comfortable.CardWidth)
	}
	if !(compact.HeaderHeight < normal.HeaderHeight && normal.HeaderHeight < comfortable.HeaderHeight) {
		t.Fatalf("header heights not ordered by density: compact=%v normal=%v comfortable=%v", compact.HeaderHeight, normal.HeaderHeight, comfortable.HeaderHeight)
	}
	if compact.ContentPadding >= normal.ContentPadding || normal.ContentPadding >= comfortable.ContentPadding {
		t.Fatalf("content padding not increasing with density: compact=%v normal=%v comfortable=%v", compact.ContentPadding, normal.ContentPadding, comfortable.ContentPadding)
	}
}

func TestCalculateShellBoundsWithProfile_DensityChangesGeometry(t *testing.T) {
	window := gfx.RectFromXYWH(0, 0, 1280, 800)
	compact := LayoutProfileForDensity(store.DensityCompact)
	normal := LayoutProfileForDensity(store.DensityNormal)
	comfortable := LayoutProfileForDensity(store.DensityComfortable)

	compactShell := CalculateShellBoundsWithProfile(window, compact.SidebarWidthDefault, compact.InspectorWidthDefault, compact)
	normalShell := CalculateShellBoundsWithProfile(window, normal.SidebarWidthDefault, normal.InspectorWidthDefault, normal)
	comfortableShell := CalculateShellBoundsWithProfile(window, comfortable.SidebarWidthDefault, comfortable.InspectorWidthDefault, comfortable)

	if !(compactShell.Header.Height() < normalShell.Header.Height() && normalShell.Header.Height() < comfortableShell.Header.Height()) {
		t.Fatalf("header geometry did not scale with density: compact=%v normal=%v comfortable=%v", compactShell.Header.Height(), normalShell.Header.Height(), comfortableShell.Header.Height())
	}
	if !(compactShell.Content.Width() > comfortableShell.Content.Width()) {
		t.Fatalf("expected compact content width > comfortable content width, got compact=%v comfortable=%v", compactShell.Content.Width(), comfortableShell.Content.Width())
	}
}
