package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

type iconResolverStub map[string]IconAsset

func (r iconResolverStub) ResolveIcon(ref string) (IconAsset, bool) {
	asset, ok := r[ref]
	return asset, ok
}

func TestIconAssetCloneDeepCopiesPath(t *testing.T) {
	asset := NewIconAsset(
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
	base := NewIconAsset(
		"chevron-down",
		1,
		gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)),
		gfx.RectFromXYWH(0, 0, 24, 24),
	)
	inputs := IconCacheInputs{
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

	if clone := base.Clone(); clone.CacheKey(IconCacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}) != first {
		t.Fatal("expected cloning to preserve cache key inputs")
	}

	if got := NewIconAsset("chevron-down", 2, base.Path, base.ViewBox).CacheKey(inputs); got == first {
		t.Fatal("expected revision change to affect cache key")
	}
	if got := base.CacheKey(IconCacheInputs{
		ThemeVersion:        inputs.ThemeVersion + 1,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected theme version change to affect cache key")
	}
	if got := base.CacheKey(IconCacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           theme.ColorPrimary,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected color slot change to affect cache key")
	}
	if got := base.CacheKey(IconCacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             theme.DefaultDensityScale(theme.DensityIDCompact, theme.DefaultTokens()),
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected density change to affect cache key")
	}
	if got := base.CacheKey(IconCacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        2,
		PreserveAspectRatio: inputs.PreserveAspectRatio,
	}); got == first {
		t.Fatal("expected content scale change to affect cache key")
	}
	if got := base.CacheKey(IconCacheInputs{
		ThemeVersion:        inputs.ThemeVersion,
		ColorSlot:           inputs.ColorSlot,
		Density:             inputs.Density,
		ContentScale:        inputs.ContentScale,
		PreserveAspectRatio: "none",
	}); got == first {
		t.Fatal("expected preserveAspectRatio change to affect cache key")
	}
}

func TestRuntimeResolveIconReturnsClonedSnapshot(t *testing.T) {
	original := NewIconAsset(
		"chevron-down",
		3,
		gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)),
		gfx.RectFromXYWH(0, 0, 24, 24),
	)
	rt := &Runtime{
		config: Config{
			IconResolver: iconResolverStub{
				"chevron-down": original,
			},
		},
	}

	got, ok := rt.ResolveIcon("chevron-down")
	if !ok {
		t.Fatal("expected runtime resolver to find icon")
	}
	original.Path.Segments[0].Pts[0].X = 77

	if got.Path.Segments[0].Pts[0].X == 77 {
		t.Fatal("expected runtime to return a cloned icon snapshot")
	}
}
