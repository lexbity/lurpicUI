// Package recipes defines shared slot structs used by semantic theme recipes.
//
// The package is intentionally mark-agnostic. Concrete recipe resolvers live in
// family subpackages and may depend on this shared slot vocabulary, but they do
// not import marks packages.
package recipes
