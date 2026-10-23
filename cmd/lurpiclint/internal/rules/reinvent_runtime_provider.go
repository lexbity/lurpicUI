package rules

import (
	"go/ast"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// RuntimeProviderReDeclaration (LL038) flags local re-declarations of the
// runtime-access duck types inside marks subpackages. Font-registry lookup,
// icon resolution, and icon rendering are centralized in the marks root
// package (marks.ShaperFor, marks.FontRegistryFrom, marks.IconResolverFrom,
// marks.ResolveIconFrom, marks.IconCommands); a mark that re-declares the
// provider interface locally is copy-paste plumbing that will drift from the
// canonical capability probe.
//
// The rule matches interface types (named or inline) declared in a marks
// subpackage (marks/<family>/...) whose method set contains one of the
// canonical capability method names: FontRegistry, IconResolver, ResolveIcon.
// The marks root package itself is exempt — it owns the canonical
// declarations.
type RuntimeProviderReDeclaration struct{}

func (r *RuntimeProviderReDeclaration) ID() string                     { return "LL038" }
func (r *RuntimeProviderReDeclaration) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *RuntimeProviderReDeclaration) Description() string {
	return "local re-declaration of a runtime-access provider interface in a marks subpackage; use the marks root helper (marks.ShaperFor / marks.ResolveIconFrom / marks.IconCommands)"
}

func (r *RuntimeProviderReDeclaration) Explain() string {
	return `LL038: a marks subpackage re-declares a runtime-access provider interface.

Runtime capabilities a mark needs (font registry, icon resolver) are reached
through the shared helpers in the marks root package: marks.ShaperFor,
marks.FontRegistryFrom, marks.IconResolverFrom, marks.ResolveIconFrom, and
marks.IconCommands. These helpers own the duck-typed capability probes
exactly once. A local re-declaration of the same provider interface is
copy-paste plumbing: it drifts from the canonical probe and multiplies the
same ten lines across every mark.

Fix: delete the local interface and call the marks root helper.

If a genuinely new runtime capability is needed, extend the marks root
package once — do not re-declare the probe per mark.`
}

// providerMethodNames are the canonical runtime-access capability probes.
var providerMethodNames = map[string]bool{
	"FontRegistry": true,
	"IconResolver": true,
	"ResolveIcon":  true,
}

func (r *RuntimeProviderReDeclaration) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic
	for _, f := range ctx.Files {
		if !isMarksSubpackageFile(f) {
			continue
		}
		pos := f.Fset
		ast.Inspect(f.AST, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			iface, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}
			if m := providerInterfaceMatch(iface); m != "" {
				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      pos.Position(ts.Pos()),
					Message:  "local provider interface " + ts.Name.Name + " re-declares the " + m + " runtime capability; use the marks root helper instead",
					Teach: diag.Teaching{
						Did:      "re-declared the " + m + " provider interface locally",
						UseThis:  "marks.ShaperFor / marks.ResolveIconFrom / marks.IconCommands",
						IndexRef: "marks/",
					},
				})
			}
			return true
		})
		// Inline anonymous provider assertions (runtime.(interface{ ... }))
		// are the same re-declaration written as a type assertion.
		ast.Inspect(f.AST, func(n ast.Node) bool {
			assert, ok := n.(*ast.TypeAssertExpr)
			if !ok {
				return true
			}
			iface, ok := assert.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}
			if m := providerInterfaceMatch(iface); m != "" {
				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      pos.Position(assert.Pos()),
					Message:  "inline provider type assertion re-declares the " + m + " runtime capability; use the marks root helper instead",
					Teach: diag.Teaching{
						Did:      "re-declared the " + m + " provider interface inline",
						UseThis:  "marks.ShaperFor / marks.ResolveIconFrom / marks.IconCommands",
						IndexRef: "marks/",
					},
				})
			}
			return true
		})
	}
	return diags
}

// providerInterfaceMatch returns the matched capability method name when the
// interface declares any canonical provider method.
func providerInterfaceMatch(iface *ast.InterfaceType) string {
	if iface.Methods == nil {
		return ""
	}
	for _, field := range iface.Methods.List {
		if len(field.Names) == 0 {
			continue // embedded interface, not a method declaration
		}
		if providerMethodNames[field.Names[0].Name] {
			return field.Names[0].Name
		}
	}
	return ""
}

// isMarksSubpackageFile reports whether the file lives in a marks subpackage
// (marks/<family>/...), excluding the marks root package that owns the
// canonical provider declarations. Paths may be absolute or relative; the
// check compares the containing directory name against "marks".
func isMarksSubpackageFile(f *loader.ParsedFile) bool {
	clean := filepath.ToSlash(f.Path)
	if !strings.Contains(clean, "/marks/") && !strings.HasPrefix(clean, "marks/") {
		return false
	}
	return filepath.Base(filepath.Dir(clean)) != "marks"
}

func init() {
	DefaultRegistry.Register(&RuntimeProviderReDeclaration{})
}
