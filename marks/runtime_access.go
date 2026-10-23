package marks

import "codeburg.org/lexbit/lurpicui/text"

// fontRegistryProvider is the duck-typed runtime capability a mark uses to
// reach the app's font registry. Runtime services objects satisfy it
// structurally; marks never import the runtime package.
type fontRegistryProvider interface {
	FontRegistry() *text.FontRegistry
}

// FontRegistryFrom returns the font registry the runtime object exposes, or
// nil when the runtime is nil or provides no registry.
func FontRegistryFrom(runtime any) *text.FontRegistry {
	if runtime == nil {
		return nil
	}
	if provider, ok := runtime.(fontRegistryProvider); ok {
		return provider.FontRegistry()
	}
	return nil
}

// ShaperFor returns a shaper over the runtime's font registry, or nil when
// the runtime provides no registry.
func ShaperFor(runtime any) *text.Shaper {
	registry := FontRegistryFrom(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}
