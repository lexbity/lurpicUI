package config

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// IgnoreDirective represents a parsed //lurpiclint:ignore comment.
type IgnoreDirective struct {
	RuleID string // rule to suppress, or "*" for all
	Reason string // mandatory reason text
	Pos    token.Position
	Valid  bool // true when the ignore is well-formed
}

// ParseIgnoreDirectives extracts all //lurpiclint:ignore directives from a
// parsed file's comment groups.  Directives without a reason are reported
// as invalid (Valid=false).
func ParseIgnoreDirectives(fset *token.FileSet, astFile *ast.File) []IgnoreDirective {
	var dirs []IgnoreDirective
	for _, cg := range astFile.Comments {
		for _, comment := range cg.List {
			text := strings.TrimSpace(comment.Text)
			if !strings.HasPrefix(text, "//lurpiclint:ignore") {
				continue
			}

			rest := strings.TrimSpace(strings.TrimPrefix(text, "//lurpiclint:ignore"))
			pos := fset.Position(comment.Pos())

			ruleID, reason, valid := parseIgnoreBody(rest)

			dirs = append(dirs, IgnoreDirective{
				RuleID: ruleID,
				Reason: reason,
				Pos:    pos,
				Valid:  valid && reason != "",
			})
		}
	}
	return dirs
}

// parseIgnoreBody splits the body after "//lurpiclint:ignore" into rule ID
// and reason.  The reason is mandatory and follows a "--" separator.
func parseIgnoreBody(body string) (ruleID, reason string, valid bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", "", false
	}

	parts := strings.SplitN(body, "--", 2)
	ruleID = strings.TrimSpace(parts[0])
	if ruleID == "" {
		return "", "", false
	}

	if len(parts) < 2 {
		return ruleID, "", false
	}

	reason = strings.TrimSpace(parts[1])
	if reason == "" {
		return ruleID, "", true
	}

	return ruleID, reason, true
}

// SuppressByIgnore filters diagnostics by removing those covered by a valid
// ignore directive.  Invalid directives (missing reason) are themselves
// reported as diagnostics.
func SuppressByIgnore(diags []*diag.Diagnostic, ignores []IgnoreDirective) []*diag.Diagnostic {
	type lineKey struct {
		file string
		line int
	}
	suppressed := make(map[lineKey]map[string]bool)

	for _, ig := range ignores {
		if !ig.Valid {
			continue
		}
		key := lineKey{file: ig.Pos.Filename, line: ig.Pos.Line}
		if suppressed[key] == nil {
			suppressed[key] = make(map[string]bool)
		}
		if ig.RuleID == "*" {
			suppressed[key]["*"] = true
		} else {
			suppressed[key][ig.RuleID] = true
		}
	}

	var out []*diag.Diagnostic
	for _, d := range diags {
		key := lineKey{file: d.Pos.Filename, line: d.Pos.Line}
		if rules, ok := suppressed[key]; ok {
			if rules["*"] || rules[d.RuleID] {
				continue
			}
		}
		out = append(out, d)
	}

	for _, ig := range ignores {
		if ig.Valid {
			continue
		}
		sev := diag.SeverityError
		out = append(out, &diag.Diagnostic{
			RuleID:   "lurpiclint-invalid-ignore",
			Severity: sev,
			Pos:      ig.Pos,
			Message:  fmt.Sprintf("invalid //lurpiclint:ignore directive: missing reason (use: //lurpiclint:ignore %s -- reason)", ig.RuleID),
		})
	}

	return out
}
