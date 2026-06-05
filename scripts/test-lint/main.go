package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	exitCode := 0

	roots := os.Args[1:]
	if len(roots) == 0 {
		roots = []string{"."}
	}

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.Contains(path, "vendor") || strings.Contains(path, ".git") {
				return nil
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.AllErrors|parser.ParseComments)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: parse error: %v\n", path, err)
				return nil
			}

			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}

				if fn.Body == nil {
					continue
				}

				checkNoAssertion(path, fset, fn)
				checkDiscardedSubject(path, fset, fn)
				checkLogOnly(path, fset, fn, f.Imports)
			}

			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk error: %v\n", err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func hasAssertion(stmt ast.Stmt) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := fmt.Sprintf("%s.%s", sel.X, sel.Sel.Name)
		switch name {
		case "t.Error", "t.Errorf", "t.Fatal", "t.Fatalf":
			found = true
			return false
		}
		if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "require" || id.Name == "assert") {
			found = true
			return false
		}
		return true
	})
	return found
}

func checkNoAssertion(path string, fset *token.FileSet, fn *ast.FuncDecl) {
	if hasAssertion(fn.Body) {
		return
	}
	pos := fset.Position(fn.Pos())
	fmt.Fprintf(os.Stderr, "%s:%d:%d: test %q has no assertion (t.Error/t.Fatal/require/assert) — may pass despite bug\n",
		path, pos.Line, pos.Column, fn.Name.Name)
}

func checkDiscardedSubject(path string, fset *token.FileSet, fn *ast.FuncDecl) {
	var lastDefine *ast.AssignStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			if stmt.Tok == token.DEFINE {
				lastDefine = stmt
			} else {
				discardCheck(fset, path, fn, lastDefine, stmt)
				lastDefine = nil
			}
		case *ast.ExprStmt:
			lastDefine = nil
		case *ast.ReturnStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			lastDefine = nil
		}
		return true
	})
}

func discardCheck(fset *token.FileSet, path string, fn *ast.FuncDecl, def *ast.AssignStmt, stmt *ast.AssignStmt) {
	if def == nil || stmt.Tok != token.ASSIGN {
		return
	}
	if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return
	}
	underscore, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok || underscore.Name != "_" {
		return
	}
	rhsIdent, ok := stmt.Rhs[0].(*ast.Ident)
	if !ok {
		return
	}
	for _, lhs := range def.Lhs {
		defIdent, ok := lhs.(*ast.Ident)
		if ok && defIdent.Name == rhsIdent.Name {
			pos := fset.Position(stmt.Pos())
			fmt.Fprintf(os.Stderr, "%s:%d:%d: test %q discards computed value %q via `_ = %s` — use it in an assertion\n",
				path, pos.Line, pos.Column, fn.Name.Name, rhsIdent.Name, rhsIdent.Name)
		}
	}
}

func checkLogOnly(path string, fset *token.FileSet, fn *ast.FuncDecl, imports []*ast.ImportSpec) {
	hasTestPackage := false
	for _, imp := range imports {
		if strings.Contains(imp.Path.Value, "testing") {
			hasTestPackage = true
			break
		}
	}
	if !hasTestPackage {
		return
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		if len(ifStmt.Body.List) == 0 {
			return true
		}
		hasLogOnly := false
		for _, stmt := range ifStmt.Body.List {
			expr, ok := stmt.(*ast.ExprStmt)
			if !ok {
				return false
			}
			call, ok := expr.X.(*ast.CallExpr)
			if !ok {
				return false
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			if sel.Sel.Name == "Log" || sel.Sel.Name == "Logf" {
				hasLogOnly = true
			} else if sel.Sel.Name == "Error" || sel.Sel.Name == "Errorf" || sel.Sel.Name == "Fatal" || sel.Sel.Name == "Fatalf" {
				return true
			} else {
				return false
			}
		}
		if hasLogOnly {
			pos := fset.Position(ifStmt.Pos())
			fmt.Fprintf(os.Stderr, "%s:%d:%d: test %q uses t.Log in if-body without sibling assertion — use t.Fatalf instead\n",
				path, pos.Line, pos.Column, fn.Name.Name)
		}
		return true
	})
}
