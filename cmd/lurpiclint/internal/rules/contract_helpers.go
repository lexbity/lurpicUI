package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// ---------------------------------------------------------------------------
// Per-rule entry point
// ---------------------------------------------------------------------------

// capabilityRule is the single entry point for contract rules LL029–LL033.
// It iterates ctx.Index, filters to KindMark, applies the optional gate, and
// runs capabilityCheck on each remaining capability.
//
// gate may be nil (most rules) or a predicate that further narrows which
// KindMark capabilities are in scope — e.g. LL031's container-and-Children
// gate.  Keeping the gate here centralises the loop so rules never re-implement
// it.
func capabilityRule(ctx *Context, r Rule, methods []string, helper string, gate func(capindex.Capability) bool) []*diag.Diagnostic {
	caps, ok := ctx.Index.([]capindex.Capability)
	if !ok || caps == nil {
		return []*diag.Diagnostic{missingCapindexDiag(r)}
	}
	var diags []*diag.Diagnostic
	for _, c := range caps {
		if c.Kind != capindex.KindMark {
			continue
		}
		if gate != nil && !gate(c) {
			continue
		}
		if d := capabilityCheck(ctx, r, c, methods, helper); d != nil {
			diags = append(diags, d)
		}
	}
	return diags
}

// ---------------------------------------------------------------------------
// Capability-level check
// ---------------------------------------------------------------------------

// capabilityCheck returns a diagnostic if cap declares the named capability
// methods but no test in the same package invokes the matching contracttest
// helper.  Returns nil if the capability is not declared, already wired,
// or legitimately suppressed via //nolint.
func capabilityCheck(ctx *Context, r Rule, cap capindex.Capability, methodNames []string, helperName string) *diag.Diagnostic {
	if !capabilityDeclared(ctx, cap, methodNames) {
		return nil
	}
	if isNolint(ctx, cap, r.ID()) {
		if reason := strings.ToLower(nolintReason(ctx, cap, r.ID())); denylist[reason] {
			return emptyNolintDiag(ctx, r, cap)
		}
		return nil
	}
	if packageTestCallsHelper(ctx, cap, helperName) {
		return nil
	}
	return missingHelperDiag(ctx, r, cap, helperName, methodNames)
}

// denylist lists nolint reason strings that are considered empty or
// placeholder values.  A nolint directive whose (lowercased) reason is on
// this list is treated as missing and produces a warning.
var denylist = map[string]bool{
	"":      true,
	"todo":  true,
	"fixme": true,
}

// ---------------------------------------------------------------------------
// Package file access
// ---------------------------------------------------------------------------

// packageFiles returns all parsed files from packages whose directory path
// ends with /<category>.  For example, if category is "structure", it returns
// files from packages like /home/.../marks/structure.
func packageFiles(ctx *Context, category string) []*loader.ParsedFile {
	var out []*loader.ParsedFile
	for _, pkg := range ctx.Pkgs {
		clean := filepath.ToSlash(pkg.Path)
		if strings.HasSuffix(clean, "/"+category) || clean == category {
			out = append(out, pkg.Files...)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Receiver type name extraction
// ---------------------------------------------------------------------------

// receiverTypeName extracts the base type name from a method receiver.
// It handles *T, T, *T[X], T[X], *T[X,Y], and T[X,Y].
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	typ := recv.List[0].Type
	// Unwrap pointer: *T → T
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	// Unwrap single type parameter: T[X] → T
	if idx, ok := typ.(*ast.IndexExpr); ok {
		typ = idx.X
	}
	// Unwrap multiple type parameters: T[X, Y] → T
	if idx, ok := typ.(*ast.IndexListExpr); ok {
		typ = idx.X
	}
	id, ok := typ.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// ---------------------------------------------------------------------------
// Capability method detection
// ---------------------------------------------------------------------------

// capabilityDeclared reports whether the mark type declares all the named
// methods in the same package's non-test files.
func capabilityDeclared(ctx *Context, cap capindex.Capability, methodNames []string) bool {
	for _, name := range methodNames {
		if !packageHasMethod(ctx, cap, name) {
			return false
		}
	}
	return true
}

// packageHasMethod reports whether the mark type has a method with the given
// name declared directly on it (not promoted) in any non-test file of the
// same package.
func packageHasMethod(ctx *Context, cap capindex.Capability, methodName string) bool {
	files := packageFiles(ctx, cap.Category)
	for _, pf := range files {
		if strings.HasSuffix(pf.Path, "_test.go") {
			continue
		}
		found := false
		ast.Inspect(pf.AST, func(n ast.Node) bool {
			if found {
				return false
			}
			fd, ok := n.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Name == nil {
				return true
			}
			if fd.Name.Name != methodName {
				return true
			}
			if receiverTypeName(fd.Recv) == cap.TypeName {
				found = true
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helper call detection
// ---------------------------------------------------------------------------

// packageTestCallsHelper reports whether any _test.go file in the mark's
// package contains a call to the given helper function.  The helper may be
// referenced by bare name (e.g. AssertAnchorExport[t]) or qualified
// (e.g. contracttest.AssertAnchorExport[t]).
func packageTestCallsHelper(ctx *Context, cap capindex.Capability, helperName string) bool {
	files := packageFiles(ctx, cap.Category)
	for _, pf := range files {
		if !strings.HasSuffix(pf.Path, "_test.go") {
			continue
		}
		found := false
		ast.Inspect(pf.AST, func(n ast.Node) bool {
			if found {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := walk.CallFuncName(call)
			if name == helperName || strings.HasSuffix(name, "."+helperName) {
				found = true
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Nolint directive support
// ---------------------------------------------------------------------------

// isNolint reports whether the mark type's declaration has a //nolint:LLxxx
// directive covering the given rule ID.
func isNolint(ctx *Context, cap capindex.Capability, ruleID string) bool {
	gen := findTypeGenDecl(ctx, cap)
	if gen == nil || gen.Doc == nil {
		return false
	}
	for _, comment := range gen.Doc.List {
		body := extractNolintBody(comment.Text)
		if body == "" {
			continue
		}
		for _, id := range strings.Split(body, ",") {
			if strings.TrimSpace(id) == ruleID || strings.TrimSpace(id) == "*" {
				return true
			}
		}
	}
	return false
}

// nolintReason returns the reason string from a //nolint:LLxxx // reason
// directive on the mark type's declaration, or "" if absent.
func nolintReason(ctx *Context, cap capindex.Capability, ruleID string) string {
	gen := findTypeGenDecl(ctx, cap)
	if gen == nil || gen.Doc == nil {
		return ""
	}
	for _, comment := range gen.Doc.List {
		body := extractNolintBody(comment.Text)
		if body == "" {
			continue
		}
		for _, id := range strings.Split(body, ",") {
			if strings.TrimSpace(id) == ruleID || strings.TrimSpace(id) == "*" {
				if reason := extractNolintReason(comment.Text); reason != "" {
					return reason
				}
			}
		}
	}
	return ""
}

// findTypeGenDecl returns the GenDecl that contains the type declaration for
// the given capability's type name, or nil if not found.
func findTypeGenDecl(ctx *Context, cap capindex.Capability) *ast.GenDecl {
	files := packageFiles(ctx, cap.Category)
	for _, pf := range files {
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
				if !ok || ts.Name == nil {
					continue
				}
				if ts.Name.Name == cap.TypeName {
					return gen
				}
			}
		}
	}
	return nil
}

// nolintDirectiveBody strips a leading "//nolint:" or "// nolint:" marker from
// a comment and returns the remainder (the rule-ID portion).  gofmt normalises
// standalone //nolint: directives whose rule IDs are unknown to it (e.g. the
// lurpiclint LL0xx IDs) to "// nolint:", so both forms must be recognised.
// Returns "" when the comment is not a nolint directive.
func nolintDirectiveBody(commentText string) string {
	text := strings.TrimSpace(commentText)
	for _, prefix := range []string{"//nolint:", "// nolint:"} {
		if strings.HasPrefix(text, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	return ""
}

// extractNolintBody pulls the rule-ID portion from a //nolint:LLxxx comment.
// It handles formats like:
//
//	//nolint:LL030
//	//nolint:LL030,LL031 // reason
//	//nolint:LL030 -- reason
func extractNolintBody(commentText string) string {
	rest := nolintDirectiveBody(commentText)
	if rest == "" {
		return ""
	}
	// Strip reason separator if present.
	for _, sep := range []string{" //", " --"} {
		if idx := strings.Index(rest, sep); idx >= 0 {
			rest = strings.TrimSpace(rest[:idx])
			break
		}
	}
	return rest
}

// extractNolintReason pulls the reason from a //nolint:LLxxx // reason
// comment.  It recognizes both " //" and " --" as separators.
func extractNolintReason(commentText string) string {
	rest := nolintDirectiveBody(commentText)
	if rest == "" {
		return ""
	}
	for _, sep := range []string{" //", " --"} {
		if idx := strings.Index(rest, sep); idx >= 0 {
			return strings.TrimSpace(rest[idx+len(sep):])
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Mark type position
// ---------------------------------------------------------------------------

// markTypePos returns the token.Position of the type declaration for the
// given capability's type name, or a synthetic position if not found.
func markTypePos(ctx *Context, cap capindex.Capability) token.Position {
	files := packageFiles(ctx, cap.Category)
	for _, pf := range files {
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
				if !ok || ts.Name == nil {
					continue
				}
				if ts.Name.Name == cap.TypeName {
					return pf.Fset.Position(ts.Pos())
				}
			}
		}
	}
	return token.Position{Filename: cap.Category + "." + cap.TypeName}
}

// ---------------------------------------------------------------------------
// Diagnostic builders
// ---------------------------------------------------------------------------

// missingCapindexDiag returns an error-severity diagnostic indicating that
// the rule could not run because the capability index was not populated.
func missingCapindexDiag(r Rule) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: diag.SeverityError,
		Pos:      token.Position{Filename: "<capindex>"},
		Message:  fmt.Sprintf("%s: capindex not populated -- rule cannot run; ensure lurpiclint is invoked with capability scanning enabled", r.ID()),
	}
}

// missingHelperDiag returns a diagnostic indicating that a mark implements a
// capability but no test calls the matching contracttest helper.
func missingHelperDiag(ctx *Context, r Rule, cap capindex.Capability, helperName string, methodNames []string) *diag.Diagnostic {
	methodList := strings.Join(methodNames, "+")
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: r.DefaultSeverity(),
		Pos:      markTypePos(ctx, cap),
		Message:  fmt.Sprintf("mark %s.%s implements %s but no test in package %s calls contracttest.%s", cap.Category, cap.TypeName, methodList, cap.Category, helperName),
		Teach: diag.Teaching{
			Did:      fmt.Sprintf("declared %s on %s.%s without a contract proof", methodList, cap.Category, cap.TypeName),
			UseThis:  fmt.Sprintf("a Test%s_contract_%s test invoking contracttest.%s", cap.TypeName, contractTestSuffix(helperName), helperName),
			IndexRef: fmt.Sprintf("%s capability contract", cap.Path),
		},
	}
}

// contractTestSuffixes maps a contracttest helper name to the test-name suffix
// used by the framework's contract-test convention (the hand-written suffix in
// the Test<Type>_contract_<suffix> names across marks/*_test.go).
var contractTestSuffixes = map[string]string{
	"AssertDataBound":     "databound",
	"AssertAnchorExport":  "anchor_export",
	"AssertGroupChildren": "group_children",
	"AssertAccessible":    "accessible",
	"AssertFocusable":     "focusable",
}

// contractTestSuffix returns the test-name suffix for a contracttest helper,
// defaulting to a snake-cased derivation for helpers not yet listed.
func contractTestSuffix(helperName string) string {
	if s, ok := contractTestSuffixes[helperName]; ok {
		return s
	}
	name := strings.TrimPrefix(helperName, "Assert")
	var b strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// emptyNolintDiag returns a warning diagnostic indicating that a mark has a
// suppress comment (//nolint:) but the reason is empty or a placeholder.
func emptyNolintDiag(ctx *Context, r Rule, cap capindex.Capability) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: diag.SeverityWarn,
		Pos:      markTypePos(ctx, cap),
		Message:  fmt.Sprintf("mark %s.%s has a nolint directive for %s but the reason is empty or a placeholder; provide a substantive reason or wire the contract helper", cap.Category, cap.TypeName, r.ID()),
		Teach: diag.Teaching{
			Did:      "used nolint without a substantive reason",
			UseThis:  "provide a specific reason or add the contract test helper instead",
			IndexRef: cap.Path,
		},
	}
}
