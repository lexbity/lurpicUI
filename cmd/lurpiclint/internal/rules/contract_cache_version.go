package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// CacheVersion flags named cache structs in marks/ that echo domain data but
// carry no version field.  Without a version key the cache can serve stale
// data when the underlying store changes (FR-11 / Principle 1/8).
type CacheVersion struct{}

func (r *CacheVersion) ID() string                     { return "LL026" }
func (r *CacheVersion) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *CacheVersion) Description() string {
	return "named cache struct echoes domain data without a version field; add version uint64 from store.Version()"
}

func (r *CacheVersion) Explain() string {
	return `LL026: named cache struct echoes domain data but has no version field.

Cache structs that store domain data (structs obtained from a store via .Get()
or similar) must carry a version field populated from the source store's
.Version() to detect staleness.  Without it, the cache can serve stale data
after the underlying store changes.

Fix: add a version uint64 field and a validFor method that checks it:

    type myCache struct {
        version uint64   // <store>.Version() when derived
        items   []DomainItem
    }
    func (c *myCache) validFor(v uint64) bool { return c.version == v }

See marks/selection/dropdown_select_cache.go and
marks/navigation/pagination_cache.go for the established good pattern.`
}

// isDomainEchoField reports whether a struct field echoes domain data.
// A field is considered domain-echoing if its type is declared in the same
// package (an *ast.Ident or *ast.ArrayType/*ast.StarExpr wrapping an Ident),
// excluding built-in types like float32, int, string, bool, etc.
func isDomainEchoField(field *ast.Field) bool {
	if field == nil {
		return false
	}
	typ := field.Type

	// Unwrap slice: []DomainStruct → DomainStruct
	if arr, ok := typ.(*ast.ArrayType); ok {
		typ = arr.Elt
	}

	// Unwrap pointer: *DomainStruct → DomainStruct
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}

	id, ok := typ.(*ast.Ident)
	if !ok {
		return false
	}

	// Built-in / primitive types are not domain echoes.
	switch id.Name {
	case "float32", "float64", "int", "int64", "uint", "uint64",
		"string", "bool", "byte", "rune", "error":
		return false
	}

	// If the Ident's package matches the file's package, it's same-package.
	// Since we can't resolve Obj without go/types, we accept any non-builtin
	// Ident as a potential domain echo.  The cache-struct-name + missing-
	// version preconditions keep precision acceptable.
	return true
}

func (r *CacheVersion) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		// Phase 1: collect named struct types that look like caches and have
		// domain-echo fields but no version field.
		type cacheCandidate struct {
			pos        token.Position
			structName string
		}
		var candidates []cacheCandidate

		for _, pf := range pkg.Files {
			if !isMarksPackage(pf) {
				continue
			}
			if strings.HasSuffix(pf.Path, "_test.go") {
				continue
			}

			for _, decl := range pf.AST.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}

					// Condition (a): must look like a cache struct.
					if !cacheStructNameMatch(ts.Name.Name) {
						continue
					}

					// Condition (c): not versioned — check for version field.
					if structHasVersionField(st) {
						continue
					}

					// Condition (b): must have at least one domain-echo field.
					hasDomainEcho := false
					for _, field := range st.Fields.List {
						if isDomainEchoField(field) {
							hasDomainEcho = true
							break
						}
					}
					if !hasDomainEcho {
						continue
					}

					candidates = append(candidates, cacheCandidate{
						pos:        pf.Fset.Position(ts.Pos()),
						structName: ts.Name.Name,
					})
				}
			}
		}

		for _, c := range candidates {
			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      c.pos,
				Message:  "named cache struct echoes domain data without a version field; add version uint64 from store.Version()",
				Teach: diag.Teaching{
					Did:      "defined a cache struct with domain-echo fields but no version key",
					UseThis:  "add a version uint64 field populated from the source store's .Version() and a validFor() freshness check",
					IndexRef: "<store>.Version() (P8: version-keyed caches)",
				},
			})
		}
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&CacheVersion{})
}
