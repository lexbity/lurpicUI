package proto

// MethodSpec is one LDP method's contract — the single source of truth
// consumed by the schema generator, the docs parity gate, the server's
// dispatch table, and the runtime's trusted-seam sync test.
type MethodSpec struct {
	Name     string // "sys.ping"
	Grant    string // "read" | "input" | "command" | "time" | "admin" | ""
	Mutating bool
	// Twin names the input/equivalence corpus pair proving the verb
	// equivalent to its GUI twin. REQUIRED non-empty when Mutating.
	// "n/a" marks infrastructure verbs that prove their own contracts.
	Twin string
	Tier string // domain|view|structural|render|input|app|""
	// Advertised marks a fully ported v2 method. A major-2 build advertises
	// only implemented methods (RFC D12); unported entries stay in this
	// table for the trusted in-process testkit seam but MUST NOT appear in
	// the wire registry, schema, client, or docs until their native v2
	// slice lands.
	Advertised bool
}

// Grant names (the canonical v2 vocabulary; the v1 "write" spelling is
// invalid, not an alias).
const (
	GrantRead    = "read"
	GrantInput   = "input"
	GrantCommand = "command"
	GrantTime    = "time"
	GrantAdmin   = "admin"
)

// Tier names (read tiers + interaction tiers).
const (
	TierDomain     = "domain"
	TierView       = "view"
	TierStructural = "structural"
	TierRender     = "render"
	TierInput      = "input"
	TierApp        = "app"
)

// Registry is the full method table. Advertised entries are the complete
// major-2 wire surface (S1: hello + ping at a real cut); non-advertised
// entries are trusted testkit-seam methods whose native v2 slices land in
// later slices. The write grant and store.write have no entry at all.
var Registry = []MethodSpec{
	{Name: "sys.hello", Grant: "", Advertised: true},
	{Name: "sys.ping", Grant: GrantRead, Advertised: true},
	{Name: "sys.info", Grant: GrantRead, Tier: TierStructural},
	{Name: "locate.resolve", Grant: GrantRead, Tier: TierStructural},
	{Name: "tree.dump", Grant: GrantRead, Tier: TierStructural},
	{Name: "store.list", Grant: GrantRead, Tier: TierDomain},
	{Name: "store.read", Grant: GrantRead, Tier: TierDomain},
	{Name: "derived.read", Grant: GrantRead, Tier: TierView},
	{Name: "facet.query", Grant: GrantRead, Tier: TierStructural},
	{Name: "frame.pixels", Grant: GrantRead, Tier: TierRender},
	{Name: "frame.packet", Grant: GrantRead, Tier: TierRender},
	{Name: "cmd.list", Grant: GrantRead, Tier: TierApp, Advertised: true},
	{Name: "cmd.execute", Grant: GrantCommand, Mutating: true, Twin: "cmd.execute/palette", Tier: TierApp, Advertised: true},
	{Name: "op.status", Grant: "", Advertised: true},
	{Name: "input.click", Grant: GrantInput, Mutating: true, Twin: "input.click/button", Tier: TierInput, Advertised: true},
	{Name: "input.press", Grant: GrantInput, Mutating: true, Twin: "input.press/slider", Tier: TierInput, Advertised: true},
	{Name: "input.release", Grant: GrantInput, Mutating: true, Twin: "input.release/slider", Tier: TierInput, Advertised: true},
	{Name: "input.move", Grant: GrantInput, Mutating: true, Twin: "input.move/button", Tier: TierInput, Advertised: true},
	{Name: "input.drag", Grant: GrantInput, Mutating: true, Twin: "input.drag/slider", Tier: TierInput, Advertised: true},
	{Name: "input.type", Grant: GrantInput, Mutating: true, Twin: "input.type/textfield", Tier: TierInput, Advertised: true},
	{Name: "input.scroll", Grant: GrantInput, Mutating: true, Twin: "input.scroll/table", Tier: TierInput, Advertised: true},
	{Name: "input.focus", Grant: GrantInput, Mutating: true, Twin: "input.focus/textfield", Tier: TierInput, Advertised: true},
	{Name: "time.mode", Grant: GrantTime, Mutating: true, Twin: "n/a", Advertised: true},
	{Name: "time.advance", Grant: GrantTime, Mutating: true, Twin: "n/a", Advertised: true},
	{Name: "frame.run", Grant: GrantTime, Mutating: true, Twin: "n/a", Advertised: true},
	{Name: "sub.watch", Grant: GrantRead, Tier: TierStructural},
	{Name: "sub.cancel", Grant: GrantRead, Tier: TierStructural},
}

// AdvertisedRegistry returns the wire surface: fully ported major-2 methods
// only. The schema generator, sys.hello's capability list, the server
// dispatch table, and the docs parity gate all consume this list.
func AdvertisedRegistry() []MethodSpec {
	out := make([]MethodSpec, 0, len(Registry))
	for _, spec := range Registry {
		if spec.Advertised {
			out = append(out, spec)
		}
	}
	return out
}

// MethodGrant returns the registry's grant for a method (the runtime's
// per-step checks and the server's table must agree; sync-tested there).
func MethodGrant(method string) (string, bool) {
	for _, spec := range Registry {
		if spec.Name == method {
			return spec.Grant, true
		}
	}
	return "", false
}

// IsAdvertised reports whether the method is on the major-2 wire surface.
func IsAdvertised(method string) bool {
	for _, spec := range Registry {
		if spec.Name == method {
			return spec.Advertised
		}
	}
	return false
}

// AdvertisedMethodNames returns the sorted advertised method names (stable
// output for the schema digest and sys.hello's capability list).
func AdvertisedMethodNames() []string {
	out := make([]string, 0, len(Registry))
	for _, spec := range Registry {
		if spec.Advertised {
			out = append(out, spec.Name)
		}
	}
	sortStrings(out)
	return out
}

// MethodNames returns the sorted names of every registry entry — the
// trusted-seam surface the runtime dispatch table must match exactly.
func MethodNames() []string {
	out := make([]string, 0, len(Registry))
	for _, spec := range Registry {
		out = append(out, spec.Name)
	}
	sortStrings(out)
	return out
}

// sortStrings is an in-place insertion sort; registry tables are tiny and
// this keeps the package dependency-free.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// SchemaDigest is the deterministic schema digest returned by sys.hello:
// sha256 over "v2\x00" plus the sorted advertised method names, hex-encoded
// with a sha256: prefix. Both endpoints compute it from the same registry.
func SchemaDigest() string {
	// Implemented in schema_digest.go (crypto/sha256).
	return schemaDigest()
}
