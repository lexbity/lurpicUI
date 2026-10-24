package proto

// MethodSpec is one LDP method's contract (the single source of truth
// consumed by the runtime dispatch tables' sync test, this registry's
// coverage gates, the docs parity test, and the Phase 8 schema generator).
type MethodSpec struct {
	Name     string // "input.click"
	Grant    string // "read" | "input" | "write" | "time" | "admin" | ""
	Mutating bool
	// Twin names the input/equivalence corpus pair proving the verb
	// equivalent to its GUI twin (FR-23). "n/a" marks time-control verbs
	// (time.mode/time.advance/frame.run): they are step infrastructure, not
	// GUI-addressable mutations. REQUIRED non-empty when Mutating.
	Twin string
	Tier string // domain|view|structural|render|input|app|"" (§3.7)
}

// Grant tier/name constants (the canonical grant vocabulary; shared with
// drive/server's parser and grant table).
const (
	GrantRead  = "read"
	GrantWrite = "write"
	GrantInput = "input"
	GrantTime  = "time"
)

// Tier names (§3.7 read tiers + interaction tiers).
const (
	TierDomain     = "domain"
	TierView       = "view"
	TierStructural = "structural"
	TierRender     = "render"
	TierInput      = "input"
	TierApp        = "app"
)

// Registry is the full method table. It MUST stay in sync with the runtime's
// driveMethods table and the server's grant table (both sync-tested).
var Registry = []MethodSpec{
	{Name: "sys.hello", Grant: "", Mutating: false},
	{Name: "sys.ping", Grant: GrantRead, Mutating: false},
	{Name: "sys.info", Grant: GrantRead, Mutating: false},
	{Name: "locate.resolve", Grant: GrantRead, Mutating: false, Tier: TierStructural},
	{Name: "tree.dump", Grant: GrantRead, Mutating: false, Tier: TierStructural},
	{Name: "store.list", Grant: GrantRead, Mutating: false, Tier: TierDomain},
	{Name: "store.read", Grant: GrantRead, Mutating: false, Tier: TierDomain},
	{Name: "store.write", Grant: GrantWrite, Mutating: true, Twin: "store.write/value", Tier: TierDomain},
	{Name: "derived.read", Grant: GrantRead, Mutating: false, Tier: TierView},
	{Name: "facet.query", Grant: GrantRead, Mutating: false, Tier: TierStructural},
	{Name: "frame.pixels", Grant: GrantRead, Mutating: false, Tier: TierRender},
	{Name: "frame.packet", Grant: GrantRead, Mutating: false, Tier: TierRender},
	{Name: "cmd.list", Grant: GrantRead, Mutating: false, Tier: TierApp},
	{Name: "cmd.execute", Grant: GrantWrite, Mutating: true, Twin: "cmd.execute/palette", Tier: TierApp},
	{Name: "input.click", Grant: GrantInput, Mutating: true, Twin: "input.click/button", Tier: TierInput},
	{Name: "input.press", Grant: GrantInput, Mutating: true, Twin: "input.press/slider", Tier: TierInput},
	{Name: "input.release", Grant: GrantInput, Mutating: true, Twin: "input.release/slider", Tier: TierInput},
	{Name: "input.move", Grant: GrantInput, Mutating: true, Twin: "input.move/button", Tier: TierInput},
	{Name: "input.drag", Grant: GrantInput, Mutating: true, Twin: "input.drag/slider", Tier: TierInput},
	{Name: "input.type", Grant: GrantInput, Mutating: true, Twin: "input.type/textfield", Tier: TierInput},
	{Name: "input.scroll", Grant: GrantInput, Mutating: true, Twin: "input.scroll/table", Tier: TierInput},
	{Name: "input.focus", Grant: GrantInput, Mutating: true, Twin: "input.focus/textfield", Tier: TierInput},
	{Name: "time.mode", Grant: GrantTime, Mutating: true, Twin: "n/a"},
	{Name: "time.advance", Grant: GrantTime, Mutating: true, Twin: "n/a"},
	{Name: "frame.run", Grant: "", Mutating: true, Twin: "n/a"},
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

// MethodNames returns the sorted registry names (stable output for docs and
// sys.info parity).
func MethodNames() []string {
	out := make([]string, 0, len(Registry))
	for _, spec := range Registry {
		out = append(out, spec.Name)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
