package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCmdValidateDemos_runsExpectedSuites(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	t.Chdir(tmp)

	originalRunner := runGoTestSuite
	originalRootFinder := findGoModuleRootFunc
	defer func() { runGoTestSuite = originalRunner }()
	defer func() { findGoModuleRootFunc = originalRootFinder }()

	var got []struct {
		dir  string
		args []string
	}
	runGoTestSuite = func(dir string, args ...string) error {
		got = append(got, struct {
			dir  string
			args []string
		}{dir: dir, args: append([]string(nil), args...)})
		return nil
	}
	findGoModuleRootFunc = func() (string, error) { return tmp, nil }

	if exit := cmdValidateDemos(nil); exit != 0 {
		t.Fatalf("cmdValidateDemos() exit = %d, want 0", exit)
	}

	want := []struct {
		dir  string
		args []string
	}{
		{dir: tmp, args: []string{"./marks/structure", "./marks/interaction", "./marks/uiinput", "./marks/uinav"}},
		{dir: filepath.Join(tmp, "demos", "ui_catalog_app"), args: []string{"./..."}},
		{dir: filepath.Join(tmp, "demos", "ui_replay_app"), args: []string{"./..."}},
		{dir: filepath.Join(tmp, "demos", "ui_diagnostic_scene"), args: []string{"./..."}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runGoTestSuite calls = %#v, want %#v", got, want)
	}
}
