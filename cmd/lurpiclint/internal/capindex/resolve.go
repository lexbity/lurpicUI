package capindex

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// resolvedType is an embedded type's AST declaration together with the file it
// was declared in (used for import-table resolution while walking its fields).
type resolvedType struct {
	spec *ast.TypeSpec
	file *loader.ParsedFile
}

// typeResolver resolves embedded type references to their AST type specs across
// the loaded packages.  It lets the fingerprint see promoted role fields: a
// mark embedding marks.Core inherits Core's Layout/Render role fields, which a
// direct-field-only struct walk would miss.
type typeResolver struct {
	modulePath string
	moduleRoot string
	specs      map[string]*ast.TypeSpec // key: dir + "\x00" + typeName
	files      map[*ast.TypeSpec]*loader.ParsedFile
}

// newTypeResolver indexes every named type declaration across the loaded
// packages.
func newTypeResolver(result *loader.LoadResult, cfg ScanConfig) *typeResolver {
	r := &typeResolver{
		modulePath: cfg.ModulePath,
		moduleRoot: cfg.ModuleRoot,
		specs:      make(map[string]*ast.TypeSpec),
		files:      make(map[*ast.TypeSpec]*loader.ParsedFile),
	}
	for dir, pkg := range result.Packages {
		for _, pf := range pkg.Files {
			for _, decl := range pf.AST.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Name == nil {
						continue
					}
					r.specs[dir+"\x00"+ts.Name.Name] = ts
					r.files[ts] = pf
				}
			}
		}
	}
	return r
}

// lookup resolves an anonymous embedded field's type expression to the embedded
// type's declaration.  expr is either an *ast.Ident (same-package type) or a
// *ast.SelectorExpr (<import-alias>.Type).  Returns nil when the type cannot be
// resolved (unloaded package or unknown alias), in which case the caller falls
// back to direct-field analysis only.
func (r *typeResolver) lookup(expr ast.Expr, f *loader.ParsedFile) *resolvedType {
	var dir, typeName string
	switch e := expr.(type) {
	case *ast.Ident:
		typeName = e.Name
		dir = filepath.Dir(f.Path)
	case *ast.SelectorExpr:
		id, ok := e.X.(*ast.Ident)
		if !ok {
			return nil
		}
		importPath, ok := f.Imports[id.Name]
		if !ok {
			return nil
		}
		typeName = e.Sel.Name
		rel := strings.TrimPrefix(importPath, r.modulePath)
		rel = strings.TrimPrefix(rel, "/")
		dir = filepath.Join(r.moduleRoot, rel)
	default:
		return nil
	}

	ts, ok := r.specs[dir+"\x00"+typeName]
	if !ok {
		return nil
	}
	return &resolvedType{spec: ts, file: r.files[ts]}
}
