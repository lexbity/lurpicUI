package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// VizScaleSubscribe flags viz mark structs that declare a
// *reactive.ReactiveScale field but do not subscribe to its OnChange signal
// via signal.Track in OnAttach (or nil-guard it).  Without subscription the
// mark goes stale when the scale recomputes.
type VizScaleSubscribe struct{}

func (r *VizScaleSubscribe) ID() string                     { return "LL025" }
func (r *VizScaleSubscribe) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *VizScaleSubscribe) Description() string {
	return "viz mark has a *reactive.ReactiveScale field without signal.Track subscription in OnAttach"
}

func (r *VizScaleSubscribe) Explain() string {
	return `LL025: viz mark declares a *reactive.ReactiveScale field but does not
subscribe to its OnChange signal in OnAttach.

Every reactive scale field (XScale, YScale, Scale) must be subscribed in the
mark's OnAttach method so that the mark re-projects when the scale changes.
The established pattern is a nil-guarded signal.Track call:

    if m.XScale != nil {
        signal.Track(m.Subs(), &m.XScale.OnChange, func(signal.Unit) {
            m.Invalidate(facet.DirtyProjection)
        })
    }

A field is considered subscribed if a signal.Track(..., &recv.<field>.OnChange)
call exists in OnAttach, or if the field is nil-guarded (if recv.<field> != nil)
there.`
}

// isReactiveScaleFieldType reports whether field has type *reactive.ReactiveScale.
func isReactiveScaleFieldType(field *ast.Field, imports loader.ImportTable) bool {
	if field == nil || field.Type == nil {
		return false
	}
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "ReactiveScale" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || !isReactiveImport(id.Name, imports) {
		return false
	}
	return true
}

// receiverIdent returns the receiver variable name from a method declaration.
func receiverIdent(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}

// onAttachHasTrackForField reports whether body contains a
// signal.Track(..., &<recv>.<field>.OnChange, ...) call.
func onAttachHasTrackForField(body *ast.BlockStmt, recvName, fieldName string, imports loader.ImportTable) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Track" {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !isSignalImport(id.Name, imports) {
			return true
		}
		// Check any argument for &recv.field.OnChange.
		for _, arg := range call.Args {
			unary, ok := arg.(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				continue
			}
			onChangeSel, ok := unary.X.(*ast.SelectorExpr)
			if !ok || onChangeSel.Sel.Name != "OnChange" {
				continue
			}
			fieldSel, ok := onChangeSel.X.(*ast.SelectorExpr)
			if !ok || fieldSel.Sel.Name != fieldName {
				continue
			}
			recvIdent, ok := fieldSel.X.(*ast.Ident)
			if !ok || recvIdent.Name != recvName {
				continue
			}
			found = true
			return false
		}
		return true
	})
	return found
}

// onAttachHasNilGuardForField reports whether body contains an
// if <recv>.<field> != nil { ... } guard.
func onAttachHasNilGuardForField(body *ast.BlockStmt, recvName, fieldName string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		ifstmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		bin, ok := ifstmt.Cond.(*ast.BinaryExpr)
		if !ok || bin.Op != token.NEQ {
			return true
		}
		// RHS must be nil.
		rhs, ok := bin.Y.(*ast.Ident)
		if !ok || rhs.Name != "nil" {
			return true
		}
		// LHS must be recv.field.
		sel, ok := bin.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != fieldName {
			return true
		}
		recvIdent, ok := sel.X.(*ast.Ident)
		if !ok || recvIdent.Name != recvName {
			return true
		}
		found = true
		return false
	})
	return found
}

func (r *VizScaleSubscribe) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		// Phase 1: collect struct names and their reactive-scale fields.
		structFields := make(map[string][]string) // structName → [fieldName, ...]

		for _, pf := range pkg.Files {
			if !isVizPackage(pf) {
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
					for _, field := range st.Fields.List {
						if !isReactiveScaleFieldType(field, pf.Imports) {
							continue
						}
						for _, name := range field.Names {
							if name != nil {
								structFields[ts.Name.Name] = append(structFields[ts.Name.Name], name.Name)
							}
						}
					}
				}
			}
		}

		if len(structFields) == 0 {
			continue
		}

		// Phase 2: find OnAttach methods and check each scale field.
		for _, pf := range pkg.Files {
			if !isVizPackage(pf) {
				continue
			}
			if strings.HasSuffix(pf.Path, "_test.go") {
				continue
			}

			for _, decl := range pf.AST.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != "OnAttach" || fn.Body == nil {
					continue
				}

				structName := receiverTypeName(fn.Recv)
				if structName == "" {
					continue
				}

				scaleFields, ok := structFields[structName]
				if !ok {
					continue
				}

				recvName := receiverIdent(fn)
				if recvName == "" {
					continue
				}

				for _, fieldName := range scaleFields {
					tracked := onAttachHasTrackForField(fn.Body, recvName, fieldName, pf.Imports)
					guarded := onAttachHasNilGuardForField(fn.Body, recvName, fieldName)

					if !tracked && !guarded {
						diags = append(diags, &diag.Diagnostic{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							Pos:      pf.Fset.Position(fn.Pos()),
							Message:  "viz mark has a *reactive.ReactiveScale field without signal.Track subscription in OnAttach",
							Teach: diag.Teaching{
								Did:      "declared a *reactive.ReactiveScale field but did not subscribe to its OnChange",
								UseThis:  "add signal.Track(&recv.<field>.OnChange, func(…) { … Invalidate(DirtyProjection) }) in OnAttach",
								IndexRef: "signal.Track(&scale.OnChange, … Invalidate(DirtyProjection))",
							},
						})
					}
				}
			}
		}
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&VizScaleSubscribe{})
}
