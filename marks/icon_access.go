package marks

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/icon"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/theme"
)

// iconProvider is the duck-typed runtime capability exposing the icon
// resolver. Runtime services objects satisfy it structurally.
type iconProvider interface {
	IconResolver() icon.Resolver
}

// IconResolverFrom returns the runtime's icon resolver, or nil when the
// runtime is nil or exposes no resolver.
func IconResolverFrom(rt any) icon.Resolver {
	if rt == nil {
		return nil
	}
	if p, ok := rt.(iconProvider); ok {
		return p.IconResolver()
	}
	return nil
}

// ResolveIconFrom resolves an icon reference through the runtime's icon
// capability: either an IconResolver() provider or a direct ResolveIcon
// implementation. ok is false when the runtime exposes no resolver or the
// reference is unknown.
func ResolveIconFrom(rt any, ref string) (icon.Asset, bool) {
	if rt == nil {
		return icon.Asset{}, false
	}
	if provider, ok := rt.(iconProvider); ok {
		if resolver := provider.IconResolver(); resolver != nil {
			return resolver.ResolveIcon(ref)
		}
	}
	if resolver, ok := rt.(interface {
		ResolveIcon(string) (icon.Asset, bool)
	}); ok {
		return resolver.ResolveIcon(ref)
	}
	return icon.Asset{}, false
}

// IconCommands renders a resolved icon asset fitted inside bounds with the
// given material: aspect-preserving uniform scale, translated into the box.
// It returns nil for empty paths, empty bounds, or transparent materials.
func IconCommands(asset icon.Asset, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if len(asset.Path.Segments) == 0 || bounds.IsEmpty() || theme.IsTransparentMaterial(material) {
		return nil
	}
	box := asset.ViewBox
	if box.IsEmpty() {
		box = gfxsvg.Bounds(asset.Path)
	}
	if box.IsEmpty() || box.Width() == 0 || box.Height() == 0 {
		return nil
	}
	sx := bounds.Width() / box.Width()
	sy := bounds.Height() / box.Height()
	scale := mathutil.Min(sx, sy)
	if scale <= 0 {
		return nil
	}
	target := gfxsvg.Transformed(asset.Path, gfx.Identity().Multiply(gfx.Translation(bounds.Min.X-box.Min.X*scale, bounds.Min.Y-box.Min.Y*scale)).Multiply(gfx.Scale(scale, scale)))
	return []gfx.Command{gfx.FillPath{Path: target, Brush: gfx.SolidBrush(theme.MaterialColor(material))}}
}
