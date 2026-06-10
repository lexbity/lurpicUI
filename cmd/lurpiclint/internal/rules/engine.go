// Package rules defines the lurpiclint rule engine: the Rule interface,
// Registry, Context, and the top-level Run function that produces
// diagnostics.
//
// Every built-in rule registers itself with DefaultRegistry during init().
// Adding a new rule requires only:
//
//  1. Write a type implementing Rule.
//  2. Register it: func init() { DefaultRegistry.Register(&MyRule{}) }
package rules

import (
	"fmt"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// Explainer is an optional interface a Rule can implement to provide
// extended explanation text for the `explain` subcommand.
type Explainer interface {
	Explain() string
}

// A Rule is a single lint check.  Implementations must be pure functions of
// the Context: they must not mutate shared state.
type Rule interface {
	// ID returns the stable rule identifier (e.g. "LL003").
	ID() string

	// DefaultSeverity is the severity assigned when no configuration
	// override exists.
	DefaultSeverity() diag.Severity

	// Description is a one-line human-readable summary.
	Description() string

	// Check runs the rule against the given context and returns any
	// diagnostics found.  The returned slice may be nil or empty.
	Check(ctx *Context) []*diag.Diagnostic
}

// Context carries the parsed source tree and framework metadata available
// to every rule during a check run.
type Context struct {
	Files []*loader.ParsedFile
	Pkgs  map[string]*loader.Package
	Fset  *token.FileSet
	Index any // *capindex.Index when that package exists
	Cfg   any // *config.Config when that package exists
}

// RunConfig controls which rules execute and how their output is adjusted.
type RunConfig struct {
	// EnabledIDs restricts execution to the named rules.  When nil or
	// empty, every registered rule runs.
	EnabledIDs []string

	// DisabledIDs is a set of rules to skip entirely.  Takes precedence
	// over EnabledIDs.
	DisabledIDs map[string]bool

	// SeverityOverrides replaces the default (or rule-reported) severity
	// of the named rules.  A rule whose override is SeverityOff is
	// skipped (equivalent to having it in DisabledIDs).
	SeverityOverrides map[string]diag.Severity
}

// Registry holds the set of known rules.  Rules register themselves via
// Register, typically during init().
type Registry struct {
	rules []Rule
	byID  map[string]Rule
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID: make(map[string]Rule),
	}
}

// Register adds a rule.  Duplicate IDs panic.
func (reg *Registry) Register(r Rule) {
	id := r.ID()
	if _, dup := reg.byID[id]; dup {
		panic(fmt.Sprintf("rule %q already registered", id))
	}
	reg.byID[id] = r
	reg.rules = append(reg.rules, r)
}

// Rules returns a copy of the registered rule slice, ordered by insertion.
func (reg *Registry) Rules() []Rule {
	out := make([]Rule, len(reg.rules))
	copy(out, reg.rules)
	return out
}

// Lookup returns the rule with the given ID, or nil.
func (reg *Registry) Lookup(id string) Rule {
	return reg.byID[id]
}

// Reset removes all rules.  Used in tests.
func (reg *Registry) Reset() {
	reg.rules = nil
	reg.byID = make(map[string]Rule)
}

// DefaultRegistry is the package-level registry that all built-in rules
// register themselves with during init().
var DefaultRegistry = NewRegistry()

// Run executes every enabled rule in the registry and returns aggregated
// diagnostics sorted by (severity desc, position).
func Run(ctx *Context, registry *Registry, cfg RunConfig) []*diag.Diagnostic {
	rules := registry.Rules()

	if len(cfg.EnabledIDs) > 0 {
		enabled := make(map[string]bool, len(cfg.EnabledIDs))
		for _, id := range cfg.EnabledIDs {
			enabled[id] = true
		}
		filtered := make([]Rule, 0, len(enabled))
		for _, r := range rules {
			if enabled[r.ID()] {
				filtered = append(filtered, r)
			}
		}
		rules = filtered
	}

	var all []*diag.Diagnostic
	for _, rule := range rules {
		id := rule.ID()

		// Check disable list.
		if cfg.DisabledIDs[id] {
			continue
		}

		// Check if SeverityOff via override.
		if override, ok := cfg.SeverityOverrides[id]; ok && override == diag.SeverityOff {
			continue
		}

		diags := rule.Check(ctx)

		// Apply severity overrides.
		if override, ok := cfg.SeverityOverrides[id]; ok {
			for _, d := range diags {
				d.Severity = override
			}
		}

		all = append(all, diags...)
	}

	diag.SortDiagnostics(all)
	return all
}
