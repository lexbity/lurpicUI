package loader

import (
	"go/ast"
	"go/token"
)

// ImportTable maps local identifiers (explicit alias, dot import, or package
// basename) to canonical import paths.  It is used by rules to resolve
// selector expressions (e.g. "facet.LayoutRole") without full type checking.
type ImportTable map[string]string

// ParsedFile holds a single parsed Go source file and its metadata.
type ParsedFile struct {
	Fset    *token.FileSet
	AST     *ast.File
	Path    string
	Pkg     string      // package name as declared in the file ("package foo")
	Imports ImportTable // local identifier → canonical import path
}

// Package groups parsed files belonging to the same Go package directory.
// All files must declare the same package name.
type Package struct {
	Name  string        // package name
	Path  string        // directory path (cleaned, absolute or relative to cwd)
	Files []*ParsedFile // sorted by Path
}

// TypeInfo provides optional resolved type information for rules that need
// it.  The current implementation returns nil for all queries; a future
// phase may back this with go/types stdlib type-checking (which is available
// without external dependencies).  No rule currently requires type info, so
// this interface exists as a forward-compatibility seam.
//
// Phase 13 decision: stdlib-only.  All current rules operate on syntactic
// patterns (AST structure, import tables, doc comments).  Adding full
// go/types type-checking would increase runtime, add failure modes
// (incomplete deps, build tags), and not improve detection accuracy for any
// current rule.  If a future rule genuinely needs type resolution:
//
//  1. Implement Checker using go/types (stdlib, no vendor cost).
//  2. Wire it into Load() via an opt-in Config field.
//  3. Do NOT vendor golang.org/x/tools — justify in writing first.
type TypeInfo interface {
	// Resolve returns the fully-qualified type name for an expression
	// (e.g. "codeburg.org/lexbit/lurpicui/store.CollectionStore"), or
	// empty string if the type cannot be resolved.
	Resolve(expr ast.Expr) string
}

// LoadResult is the output of a Load call.
type LoadResult struct {
	Files    []*ParsedFile       // all files across all packages, sorted by path
	Packages map[string]*Package // keyed by directory path
	Fset     *token.FileSet      // shared FileSet for all parsed files
}
