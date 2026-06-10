package diag

import (
	"fmt"
	"io"
)

// GitHubReporter emits diagnostics as GitHub Actions workflow command
// annotations.
//
// Format:
//
//	::error file=foo.go,line=10,col=12::LL003: message
//	::warning file=foo.go,line=10,col=12::LL003: message
//	::notice file=foo.go,line=10,col=12::LL003: message
//
// See https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions
type GitHubReporter struct {
	w io.Writer
}

func (r *GitHubReporter) Name() string { return "github" }

func (r *GitHubReporter) Report(diags []*Diagnostic) error {
	for _, d := range diags {
		cmd := "notice"
		switch d.Severity {
		case SeverityWarn:
			cmd = "warning"
		case SeverityError:
			cmd = "error"
		}

		_, err := fmt.Fprintf(r.w, "::%s file=%s,line=%d,col=%d::%s: %s\n",
			cmd,
			d.Pos.Filename,
			d.Pos.Line,
			d.Pos.Column,
			d.RuleID,
			d.Message,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
