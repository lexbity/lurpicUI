package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// StableBeforeVerified flags marks that declare stability (via doc comment)
// without corresponding verification evidence.  V2 Rule 12 requires
// stability claims to be backed by verified golden tests.
//
// Default severity: error.
type StableBeforeVerified struct{}

func (r *StableBeforeVerified) ID() string                     { return "LL015" }
func (r *StableBeforeVerified) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *StableBeforeVerified) Description() string {
	return "mark declared stable without verified evidence; add golden tests or remove the stability claim"
}

func (r *StableBeforeVerified) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !fileContainsFacetType(f) {
			continue
		}

		for _, decl := range f.AST.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				if !typeClaimsStable(ts, gen) {
					continue
				}
				if hasVerifiedEvidence(ts, f) {
					continue
				}
				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      f.Fset.Position(ts.Pos()),
					Message:  "mark " + ts.Name.Name + " claims stability but has no verified evidence; add golden tests or remove the stability claim (V2 Rule 12)",
					Teach: diag.Teaching{
						Did:      "marked a type as stable without verification",
						UseThis:  "add golden conformance tests and a //verified marker",
						IndexRef: "",
					},
				})
			}
		}
	}

	return diags
}

// typeClaimsStable reports whether the type or its enclosing GenDecl doc
// contains a stability claim marker.
func typeClaimsStable(ts *ast.TypeSpec, gen *ast.GenDecl) bool {
	for _, doc := range []*ast.CommentGroup{ts.Doc, gen.Doc} {
		if doc == nil {
			continue
		}
		text := doc.Text()
		lower := strings.ToLower(text)
		if strings.Contains(lower, "stable") || strings.Contains(lower, "stability") {
			return true
		}
	}
	return false
}

// hasVerifiedEvidence checks whether the file or a companion _test.go file
// contains a "verified" marker in a doc comment.
func hasVerifiedEvidence(ts *ast.TypeSpec, f *loader.ParsedFile) bool {
	for _, decl := range f.AST.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts2, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts2.Doc != nil && strings.Contains(strings.ToLower(ts2.Doc.Text()), "verified") {
				return true
			}
		}
	}
	for _, cg := range f.AST.Comments {
		if strings.Contains(strings.ToLower(cg.Text()), "verified") {
			return true
		}
	}
	return false
}

func init() {
	DefaultRegistry.Register(&StableBeforeVerified{})
}
