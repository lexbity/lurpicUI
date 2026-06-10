package capindex

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// ScanConfig controls which packages are introspected.
type ScanConfig struct {
	// ModulePath is the Go module import path (e.g. "codeburg.org/lexbit/lurpicui").
	ModulePath string
	// ModuleRoot is the absolute filesystem path to the module root.
	ModuleRoot string
}

// Scan introspects the loaded Go packages and returns a capability index.
// It identifies marks (exported types in marks/ sub-packages with New*
// constructors), layout containers, and standard layer constants.
func Scan(result *loader.LoadResult, cfg ScanConfig) []Capability {
	var caps []Capability

	for pkgDir, pkg := range result.Packages {
		relPkg, err := filepath.Rel(cfg.ModuleRoot, pkgDir)
		if err != nil {
			continue
		}
		relPkg = filepath.ToSlash(relPkg)

		for _, f := range pkg.Files {
			// Scan types in this file.
			for _, decl := range f.AST.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}

				switch gen.Tok {
				case token.TYPE:
					for _, spec := range gen.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok || !ts.Name.IsExported() {
							continue
						}
						cap := scanType(ts, f, pkg, relPkg, cfg)
						if cap != nil {
							caps = append(caps, *cap)
						}
					}

				case token.CONST:
					// Look for StandardLayer (name, not ID or Order) constants.
					if strings.Contains(relPkg, "layout") {
						for _, spec := range gen.Specs {
							vs, ok := spec.(*ast.ValueSpec)
							if !ok || len(vs.Names) == 0 {
								continue
							}
							name := vs.Names[0].Name
							if strings.HasPrefix(name, "StandardLayer") &&
								!strings.HasPrefix(name, "StandardLayerID") &&
								!strings.HasPrefix(name, "StandardLayerOrder") {
								caps = append(caps, Capability{
									Kind:     KindLayer,
									Path:     relPkg + "." + name,
									TypeName: name,
									Category: "layer",
								})
							}
						}
					}
				}
			}
		}
	}

	// Sort by path for deterministic output.
	sort.Slice(caps, func(i, j int) bool {
		return caps[i].Path < caps[j].Path
	})

	return caps
}

// scanType inspects a single exported type spec and returns a Capability if
// it's a recognised mark or layout type, or nil otherwise.
func scanType(ts *ast.TypeSpec, f *loader.ParsedFile, pkg *loader.Package, relPkg string, cfg ScanConfig) *Capability {
	typeName := ts.Name.Name

	// Determine category from the relative package path.
	category := relPkg
	if idx := strings.LastIndex(relPkg, "/"); idx >= 0 {
		category = relPkg[idx+1:]
	}

	// Look for a New<Type> constructor in the same package.
	constructor := findConstructor(pkg, typeName)

	// Extract intent from the type's doc comment or the constructor's doc.
	intent := extractIntent(ts.Doc)
	if intent == "" && constructor != "" {
		intent = constructorIntent(pkg, constructor)
	}

	// Compute fingerprint.
	fp := computeFingerprint(ts, f)

	// Classify kind.
	kind := classifyKind(relPkg, typeName, constructor)

	// Build the uxauthoring index path.
	path := relPkg + "." + typeName

	return &Capability{
		Kind:        kind,
		Path:        path,
		TypeName:    typeName,
		Category:    category,
		Constructor: constructor,
		Intent:      intent,
		Fingerprint: fp,
	}
}

// findConstructor searches all files in the package for a function named
// New<typeName> that returns *<typeName> or <typeName>.
func findConstructor(pkg *loader.Package, typeName string) string {
	target := "New" + typeName
	for _, f := range pkg.Files {
		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != target {
				continue
			}
			if fn.Type == nil || fn.Type.Results == nil {
				continue
			}
			for _, r := range fn.Type.Results.List {
				if r == nil || len(r.Names) > 0 {
					continue
				}
				star, ok := r.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				sel, ok := star.X.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == typeName {
					return target
				}
				ident, ok := star.X.(*ast.Ident)
				if ok && ident.Name == typeName {
					return target
				}
			}
		}
	}
	return ""
}

// extractIntent returns the first sentence of a doc comment group.
func extractIntent(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	text := doc.Text()
	if idx := strings.Index(text, "."); idx >= 0 {
		text = text[:idx+1]
	}
	return strings.TrimSpace(text)
}

// constructorIntent returns the first sentence of the constructor function's
// doc comment, or empty string.
func constructorIntent(pkg *loader.Package, ctorName string) string {
	for _, f := range pkg.Files {
		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != ctorName {
				continue
			}
			return extractIntent(fn.Doc)
		}
	}
	return ""
}

// classifyKind determines the CapabilityKind based on the package path and
// whether the type has a New* constructor.
func classifyKind(relPkg, typeName, constructor string) CapabilityKind {
	if strings.HasPrefix(relPkg, "marks/") || relPkg == "marks" {
		if constructor != "" {
			return KindMark
		}
	}
	if strings.HasPrefix(relPkg, "layout/") || relPkg == "layout" {
		return KindLayout
	}
	// Fall back to mark if it has a New* constructor.
	if constructor != "" {
		return KindMark
	}
	return KindLayout
}
