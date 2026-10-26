package marks

import "codeburg.org/lexbit/lurpicui/text"

// fontRegistryProvider is the duck-typed runtime capability a mark uses to
// reach the app's font registry. Runtime services objects satisfy it
// structurally; marks never import the runtime package.
type fontRegistryProvider interface {
	FontRegistry() *text.FontRegistry
}

type textServiceProvider interface{ TextService() *text.Service }

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

// ShaperFor returns the runtime-owned text service. Text shaping never creates
// an ad-hoc engine in a mark.
func ShaperFor(runtime any) *text.Service {
	if runtime == nil {
		return nil
	}
	provider, ok := runtime.(textServiceProvider)
	if !ok {
		return nil
	}
	return provider.TextService()
}

// ShapeFor binds a value scale to the shared runtime text service.
func ShapeFor(runtime any, contentScale float32) *text.ShapeSession {
	service := ShaperFor(runtime)
	if service == nil {
		return nil
	}
	if contentScale == 0 {
		contentScale = 1
	}
	session, err := service.AtScale(contentScale)
	if err != nil {
		panic(err)
	}
	return session
}
