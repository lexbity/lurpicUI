package platform

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryEventLiteralNamesItsSource is the Debt-1 gate (LDP G5.1): every
// EventPointer/EventKey/EventScroll composite literal in the engine names its
// EventSource explicitly. The zero source is not a valid provenance — an
// unstamped literal is a bug the input pipeline would silently absorb.
//
// Scanned trees: platform/, input/, runtime/, internal/ (non-test, non-vendor).
// testkit's helpers are the sanctioned synthetic-input constructors and are
// stamped too; drive injection goes through EnqueueDrive with SourceDrive.
func TestEveryEventLiteralNamesItsSource(t *testing.T) {
	roots := []string{"..", "../input", "../runtime", "../internal"}
	eventTypes := map[string]bool{
		"EventPointer": true,
		"EventKey":     true,
		"EventScroll":  true,
		"EventText":    true,
	}

	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				switch filepath.Base(path) {
				case "vendor", "testdata", ".git", "build":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Errorf("%s: parse: %v", path, err)
				return nil
			}
			ast.Inspect(file, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				var name string
				switch ty := cl.Type.(type) {
				case *ast.Ident:
					name = ty.Name
				case *ast.SelectorExpr:
					name = ty.Sel.Name
				default:
					return true
				}
				if !eventTypes[name] {
					return true
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Source" {
						return true
					}
				}
				t.Errorf("%s: %s literal at %s lacks an explicit Source (Debt-1: every event names its provenance)",
					path, name, fset.Position(cl.Pos()))
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
