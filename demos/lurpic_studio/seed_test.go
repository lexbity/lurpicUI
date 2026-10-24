package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadSeed_HonorsLurpicSeed proves the LURPIC_DRIVE-era determinism seam:
// LURPIC_SEED names a CSV on disk that replaces the embedded snapshot, so a
// replay session pins its data (LDP Phase 2, Debt-4 neighborhood).
func TestLoadSeed_HonorsLurpicSeed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.csv")
	csv := "date,revenue,users,region\n2026-01-01,820,449,north\n2026-01-02,852,462,south\n"
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	t.Setenv("LURPIC_SEED", path)
	rows, err := loadSeed()
	if err != nil {
		t.Fatalf("loadSeed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Region != "north" || rows[1].Value != 852 {
		t.Fatalf("rows = %+v %+v", rows[0], rows[1])
	}

	// A broken path is a loud startup error, not a silent embedded fallback.
	t.Setenv("LURPIC_SEED", filepath.Join(dir, "missing.csv"))
	if _, err := loadSeed(); err == nil {
		t.Fatal("missing LURPIC_SEED file must be an error")
	}
}
