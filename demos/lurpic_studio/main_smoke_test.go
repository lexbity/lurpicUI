package main

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRootBuilder_nonNil(t *testing.T) {
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	root := buildRoot(ctx)
	if root == nil {
		t.Fatal("buildRoot returned nil")
	}
}

func TestRootBuilder_measuresToWindowSize(t *testing.T) {
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	root := buildRoot(ctx)
	if root == nil {
		t.Fatal("buildRoot returned nil")
	}

	lr := root.Base().LayoutRole()
	if lr == nil {
		t.Fatal("root facet has no LayoutRole")
	}

	c := facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
	result := lr.OnMeasure(facet.MeasureContext{}, c)
	if result.Size != (gfx.Size{W: 1280, H: 800}) {
		t.Fatalf("expected size 1280x800, got %v", result.Size)
	}
}
