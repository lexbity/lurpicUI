package rules

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// AssetBootstrapOnly flags app.Asset calls with media file extensions in
// main.go files under cmd/ or demos/.  Large media should be loaded through
// Manager.Load* (LoadImage, LoadFont, LoadAsset) for caching and lifecycle,
// not via app.Asset which is intended for small bootstrap data (≤1 MiB).
//
// Default severity: warn.
type AssetBootstrapOnly struct{}

func (r *AssetBootstrapOnly) ID() string                     { return "LL017" }
func (r *AssetBootstrapOnly) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *AssetBootstrapOnly) Description() string {
	return "app.Asset used with media file; use Manager.Load* for images, fonts, and large assets"
}

func (r *AssetBootstrapOnly) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !r.isMainFile(f.Path) {
			continue
		}

		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if !r.isAppAssetCall(call, f) {
				return true
			}

			if len(call.Args) == 0 {
				return true
			}

			assetPath := r.extractStringLiteral(call.Args[0])
			if assetPath == "" {
				return true
			}

			if r.isMediaExtension(assetPath) {
				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      f.Fset.Position(call.Pos()),
					Message:  "app.Asset used with media file " + assetPath + "; use Manager.Load* instead",
					Teach: diag.Teaching{
						Did:      "loaded media file via app.Asset",
						UseThis:  "Manager.LoadImage, Manager.LoadFont, or Manager.LoadAsset for large media",
						IndexRef: "app/",
					},
				})
			}

			return true
		})
	}

	return diags
}

// isMainFile returns true when path is any main.go file (entry-point
// detection).  Any application — whether inside the lurpicUI repo or an
// external consumer — should use Manager.Load* for large media assets.
func (r *AssetBootstrapOnly) isMainFile(path string) bool {
	return filepath.Base(path) == "main.go"
}

// isAppAssetCall returns true when call is an app.Asset(...) expression
// where "app" resolves to the lurpicui app package.
func (r *AssetBootstrapOnly) isAppAssetCall(call *ast.CallExpr, f *loader.ParsedFile) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if id.Name != "app" {
		return false
	}
	if sel.Sel.Name != "Asset" {
		return false
	}

	// Verify that "app" resolves to the lurpicui app package, not a
	// local variable or a different package with the same alias.
	imp, ok := f.Imports["app"]
	if !ok {
		return false
	}
	return strings.HasSuffix(imp, "/app") || imp == "app"
}

// extractStringLiteral returns the string value of a basic string literal, or
// empty string if expr is not a string literal.
func (r *AssetBootstrapOnly) extractStringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	return strings.Trim(lit.Value, `"`)
}

// isMediaExtension returns true when path has a known media-file extension.
func (r *AssetBootstrapOnly) isMediaExtension(path string) bool {
	mediaExts := map[string]bool{
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".svg":   true,
		".gif":   true,
		".webp":  true,
		".ttf":   true,
		".otf":   true,
		".woff":  true,
		".woff2": true,
		".ktx2":  true,
		".pak":   true,
		".gltf":  true,
		".glb":   true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	return mediaExts[ext]
}

func init() {
	DefaultRegistry.Register(&AssetBootstrapOnly{})
}
