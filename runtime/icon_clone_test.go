package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/icon"
)

type iconResolverStub map[string]icon.Asset

func (s iconResolverStub) ResolveIcon(ref string) (icon.Asset, bool) {
	a, ok := s[ref]
	return a, ok
}

// TestResolveIconReturnsClonedSnapshot pins the snapshot contract: the
// runtime clones resolved assets so a caller mutating the returned path can
// never corrupt the resolver's cached geometry.
func TestRuntimeResolveIconReturnsClonedSnapshot(t *testing.T) {
	original := icon.New(
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
