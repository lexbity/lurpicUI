package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func cmdValidate(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: validate subcommand required (demos)")
		return 1
	}

	switch args[0] {
	case "demos":
		return cmdValidateDemos(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown validate subcommand %q (supported: demos)\n", args[0])
		return 1
	}
}

func cmdValidateDemos(args []string) int {
	fs := flag.NewFlagSet("validate demos", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	root, err := findGoModuleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot find project root: %v\n", err)
		return 1
	}

	type suite struct {
		name string
		dir  string
		args []string
	}

	suites := []suite{
		{
			name: "shared marks",
			dir:  root,
			args: []string{"./marks/structure", "./marks/interaction", "./marks/uiinput", "./marks/uinav"},
		},
		{
			name: "ui_catalog_app",
			dir:  filepath.Join(root, "demos", "ui_catalog_app"),
			args: []string{"./..."},
		},
		{
			name: "ui_replay_app",
			dir:  filepath.Join(root, "demos", "ui_replay_app"),
			args: []string{"./..."},
		},
		{
			name: "ui_diagnostic_scene",
			dir:  filepath.Join(root, "demos", "ui_diagnostic_scene"),
			args: []string{"./..."},
		},
	}

	for _, suite := range suites {
		if err := runGoTestSuite(suite.dir, suite.args...); err != nil {
			fmt.Fprintf(os.Stderr, "Validation failed for %s: %v\n", suite.name, err)
			return 1
		}
	}

	fmt.Println("Validation passed: demos")
	return 0
}

var runGoTestSuite = func(dir string, args ...string) error {
	cmdArgs := append([]string{"test"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findGoModuleRoot() (string, error) {
	return findGoModuleRootFunc()
}

var findGoModuleRootFunc = func() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no go.mod found")
}
