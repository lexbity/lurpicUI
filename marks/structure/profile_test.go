package structure

import "testing"

func TestProfileForViewport(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		v    Viewport
		caps Capabilities
		want Profile
	}{
		{
			name: "desktop dense without touch",
			v:    Viewport{Width: 1600, Height: 900},
			caps: Capabilities{Hover: true, Keyboard: true},
			want: ProfileDesktopDense,
		},
		{
			name: "desktop compact without touch",
			v:    Viewport{Width: 1100, Height: 700},
			caps: Capabilities{Hover: true, Keyboard: true},
			want: ProfileDesktopCompact,
		},
		{
			name: "tablet like with touch",
			v:    Viewport{Width: 1200, Height: 1600},
			caps: Capabilities{Touch: true, Hover: true, Keyboard: true},
			want: ProfileTabletLike,
		},
		{
			name: "mobile portrait with touch",
			v:    Viewport{Width: 720, Height: 1280},
			caps: Capabilities{Touch: true},
			want: ProfileMobilePortrait,
		},
		{
			name: "mobile landscape with touch",
			v:    Viewport{Width: 1280, Height: 720},
			caps: Capabilities{Touch: true},
			want: ProfileMobileLandscape,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ProfileForViewport(tc.v, tc.caps); got != tc.want {
				t.Fatalf("ProfileForViewport(%+v, %+v) = %v, want %v", tc.v, tc.caps, got, tc.want)
			}
		})
	}
}

func TestTargetModelValidate(t *testing.T) {
	t.Parallel()

	model := TargetModel{
		AppID:   "demo",
		AppName: "Demo",
		Surfaces: []SurfaceSpec{
			{ID: "primary", Role: SurfacePrimary},
			{ID: "secondary-a", Role: SurfaceSecondary},
			{ID: "secondary-b", Role: SurfaceSecondary},
			{ID: "optional", Role: SurfaceOptional},
		},
	}

	if err := model.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if got := model.PrimarySurfaceIDs(); len(got) != 1 || got[0] != "primary" {
		t.Fatalf("PrimarySurfaceIDs() = %v, want [primary]", got)
	}

	if got := model.SecondarySurfaceIDs(); len(got) != 2 || got[0] != "secondary-a" || got[1] != "secondary-b" {
		t.Fatalf("SecondarySurfaceIDs() = %v, want [secondary-a secondary-b]", got)
	}

	if got := model.OptionalSurfaceIDs(); len(got) != 1 || got[0] != "optional" {
		t.Fatalf("OptionalSurfaceIDs() = %v, want [optional]", got)
	}
}

func TestTargetModelValidateErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		model TargetModel
	}{
		{
			name: "missing app id",
			model: TargetModel{
				Surfaces: []SurfaceSpec{{ID: "primary", Role: SurfacePrimary}},
			},
		},
		{
			name: "missing primary",
			model: TargetModel{
				AppID:   "demo",
				AppName: "Demo",
				Surfaces: []SurfaceSpec{{ID: "secondary", Role: SurfaceSecondary}},
			},
		},
		{
			name: "duplicate ids",
			model: TargetModel{
				AppID:   "demo",
				AppName: "Demo",
				Surfaces: []SurfaceSpec{
					{ID: "primary", Role: SurfacePrimary},
					{ID: "primary", Role: SurfaceSecondary},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.model.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
		})
	}
}
