package marks

// Registry records Descriptors for tooling and diagnostics.
// Population is explicit: marks register themselves at init time or are
// walked at startup. No reflection-based discovery.
type Registry struct {
	entries map[string][]Descriptor
}

var global Registry

func init() {
	global.entries = make(map[string][]Descriptor)
}

// Register records a mark's descriptor in the global registry.
func Register(m Mark) {
	if m == nil {
		return
	}
	d := Describe(m)
	global.entries[d.Family] = append(global.entries[d.Family], d)
}

// Registered returns all registered descriptors across all families.
func Registered() []Descriptor {
	var out []Descriptor
	for _, descs := range global.entries {
		out = append(out, descs...)
	}
	return out
}

// RegisteredByFamily returns descriptors for a single family.
func RegisteredByFamily(family string) []Descriptor {
	return global.entries[family]
}

// ResetRegistry clears all entries (for testing only).
func ResetRegistry() {
	global.entries = make(map[string][]Descriptor)
}
