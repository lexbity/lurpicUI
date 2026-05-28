package runtime

import "codeburg.org/lexbit/lurpicui/text"

// FontRegistry exposes the runtime font registry to mark implementations that
// need text shaping during layout or projection.
func (rt *Runtime) FontRegistry() *text.FontRegistry {
	return rt.config.FontRegistry
}
