package icon_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/icon"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestIconAssetCloneDeepCopiesPath(t *testing.T) {
	asset := icon.New(
		"chevron-down",
		7,
		gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)),
		gfx.RectFromXYWH(0, 0, 24, 24),
	)
	clone := asset.Clone()

	asset.Path.Segments[0].Pts[0].X = 99

	if clone.Path.Segments[0].Pts[0].X == 99 {
		t.Fatal("expected clone path to remain independent from original")
	}
}

func TestIconAssetCacheKeyStabilityAndInvalidation(t *testing.T) {
	base := icon.New(
		"chevron-down",
		1,
		gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)),
		gfx.RectFromXYWH(0, 0, 24, 24),
	)
	inputs := icon.CacheInputs{
		ThemeVersion:        11,
		ColorSlot:           theme.ColorText,
		Density:             theme.DefaultDensityScale(theme.DensityIDComfortable, theme.DefaultTokens()),
		ContentScale:        1,
		PreserveAspectRatio: "xMidYMid meet",
	}

	first := base.CacheKey(inputs)
	if got := base.CacheKey(inputs); got != first {
		t.Fatalf("expected stable cache key, got %d then %d", first, got)
	}

	if clone := base.Clone(); clone.CacheKey(icon.CacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}) != first {
		t.Fatal("expected cloning to preserve cache key inputs")
	}

	if got := icon.New("chevron-down", 2, base.Path, base.ViewBox).CacheKey(inputs); got == first {
		t.Fatal("expected revision change to affect cache key")
	}
	if got := base.CacheKey(icon.CacheInputs{
		ThemeVersion:        inputs.ThemeVersion + 1,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected theme version change to affect cache key")
	}
	if got := base.CacheKey(icon.CacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           theme.ColorPrimary,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected color slot change to affect cache key")
	}
	if got := base.CacheKey(icon.CacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             theme.DefaultDensityScale(theme.DensityIDCompact, theme.DefaultTokens()),
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected density change to affect cache key")
	}
	if got := base.CacheKey(icon.CacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        2,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected content scale change to affect cache key")
	}
	if got := base.CacheKey(icon.CacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: "none",
	}); got == first {
		t.Fatal("expected preserveAspectRatio change to affect cache key")
	}
}
