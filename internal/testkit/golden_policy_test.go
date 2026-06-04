package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGoldenPolicy_missing_fails verifies that a missing golden causes a test
// failure when -update-golden is not set. (P-Golden: goldens assert.)
func TestGoldenPolicy_missing_fails(t *testing.T) {
	old := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = old })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	r := &recordingTB{}
	AssertGolden(r, s, "policy_missing")
	if len(r.errors) == 0 {
		t.Fatal("missing golden must fail without -update-golden")
	}
}

// TestGoldenPolicy_update_flag_creates verifies that -update-golden creates a
// golden when it does not exist, and does not fail.
func TestGoldenPolicy_update_flag_creates(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	oldUpdate := *updateGolden
	*updateGolden = true
	t.Cleanup(func() { *updateGolden = oldUpdate })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	AssertGolden(t, s, "policy_created")
	if _, err := os.Stat(filepath.Join(goldenBaseDir, "policy_created.png")); err != nil {
		t.Fatalf("golden was not created: %v", err)
	}
}

// TestGoldenPolicy_match_passes verifies that a present-and-matching golden
// passes without error.
func TestGoldenPolicy_match_passes(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	oldUpdate := *updateGolden
	*updateGolden = true
	AssertGolden(t, s, "policy_match")
	*updateGolden = oldUpdate

	AssertGolden(t, s, "policy_match")
}

// TestGoldenPolicy_mismatch_fails verifies that a mismatch writes the _actual
// dump and reports the error.
func TestGoldenPolicy_mismatch_fails(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	oldUpdate := *updateGolden
	*updateGolden = true
	AssertGolden(t, s, "policy_mismatch")
	*updateGolden = oldUpdate

	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 0
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	r := &recordingTB{}
	AssertGolden(r, s, "policy_mismatch")
	if len(r.errors) == 0 {
		t.Fatal("mismatch must report error")
	}
	actualPath := filepath.Join(goldenBaseDir, "policy_mismatch_actual.png")
	if _, err := os.Stat(actualPath); err != nil {
		t.Fatal("mismatch must create _actual.png dump")
	}
}
