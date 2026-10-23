package action

import (
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
)

// fontRegistryProvider re-declares the marks-root capability probe locally.
type fontRegistryProvider interface {
	FontRegistry() *text.FontRegistry
}

// iconProvider re-declares the icon resolver probe locally.
type iconProvider interface {
	IconResolver() runtime.IconResolver
}

func localShaper(rt any) *text.Shaper {
	if provider, ok := rt.(fontRegistryProvider); ok {
		if registry := provider.FontRegistry(); registry != nil {
			return text.NewShaper(registry)
		}
	}
	return nil
}

func localResolve(rt any, ref string) bool {
	if resolver, ok := rt.(interface {
		ResolveIcon(string) (runtime.IconAsset, bool)
	}); ok {
		_, ok := resolver.ResolveIcon(ref)
		return ok
	}
	return false
}

func localIconProviderProbe(rt any) runtime.IconResolver {
	if p, ok := rt.(iconProvider); ok {
		return p.IconResolver()
	}
	return nil
}
