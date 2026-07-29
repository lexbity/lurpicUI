package classify

import (
	"go/ast"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// Resolver resolves a same-package function call to its function body.
// Implementations MUST be bounded and package-local.
type Resolver interface {
	FuncBody(call *ast.CallExpr) *ast.BlockStmt
}

// PkgFuncResolver indexes all function and method declarations in a package
// and resolves calls by function name.  Keying by Name only (not receiver) is
// sufficient because the signals we count are body-structural, not
// receiver-dependent.
type PkgFuncResolver struct {
	funcs map[string]*ast.FuncDecl
}

// NewPkgFuncResolver builds a resolver from all files in a package.
func NewPkgFuncResolver(pkg *loader.Package) *PkgFuncResolver {
	r := &PkgFuncResolver{
		funcs: make(map[string]*ast.FuncDecl),
	}
	for _, f := range pkg.Files {
		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			r.funcs[fn.Name.Name] = fn
		}
	}
	return r
}

// FuncBody returns the body of a same-package function referenced by a direct
// identifier call (e.g.  arrangeChildAtCtx(a, b, ctx)) or a method call
// (e.g. c.arrangeChildren(...)), or nil if the function is unknown.
//
// Keying by Name only (not receiver) is sufficient because the signals we
// count are body-structural, not receiver-dependent.  False resolution (two
// same-named functions on different receivers) is benign: inlining the wrong
// body produces at worst a false negative.
func (r *PkgFuncResolver) FuncBody(call *ast.CallExpr) *ast.BlockStmt {
	name := funcName(call)
	if name == "" {
		return nil
	}
	fn, ok := r.funcs[name]
	if !ok {
		return nil
	}
	return fn.Body
}
