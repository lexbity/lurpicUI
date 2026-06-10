package loader

import (
	"go/ast"
	"strings"
)

// BuildImportTable constructs an ImportTable from a file's import specifications.
//
// Blank imports (_ "pkg") are excluded — they cannot be referenced in code.
// Dot imports (. "pkg") are included with key ".".
// Explicit aliases (alias "pkg") use the alias as the key.
// Unaliased imports ("pkg") use the last path segment as the key.
func BuildImportTable(specs []*ast.ImportSpec) ImportTable {
	t := make(ImportTable, len(specs))
	for _, spec := range specs {
		if spec.Name != nil && spec.Name.Name == "_" {
			continue // blank import — not referenceable
		}
		path := strings.Trim(spec.Path.Value, "\"")
		key := importKey(spec, path)
		// Last occurrence wins if there is a conflict, matching Go toolchain
		// behaviour.
		t[key] = path
	}
	return t
}

// importKey returns the local identifier under which an import is referenced
// in source code.
func importKey(spec *ast.ImportSpec, importPath string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	parts := strings.Split(importPath, "/")
	return parts[len(parts)-1]
}
