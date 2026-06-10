package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenUpdate_envVar_true_triggers_update(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	oldFlag := *updateGolden
	*updateGolden = false
	t.Cleanup(func() { *updateGolden = oldFlag })

	t.Setenv("LURPICUI_UPDATE_GOLDEN", "1")

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	AssertGolden(t, s, "policy_env_created")
	if _, err := os.Stat(filepath.Join(goldenBaseDir, "policy_env_created.png")); err != nil {
		t.Fatalf("golden was not created via env var: %v", err)
	}
}

func TestGoldenUpdate_envVar_false_does_not_update(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	oldFlag := *updateGolden
	*updateGolden = false
	t.Cleanup(func() { *updateGolden = oldFlag })

	t.Setenv("LURPICUI_UPDATE_GOLDEN", "0")

	s := NewMemorySurface(1, 1)

	r := &recordingTB{}
	AssertGolden(r, s, "policy_env_nonexistent")
	if len(r.errors) == 0 {
		t.Fatal("missing golden must fail when env var is 0 and flag is unset")
	}
}

func TestGoldenUpdate_flag_takes_precedence_over_env(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	oldFlag := *updateGolden
	*updateGolden = true
	t.Cleanup(func() { *updateGolden = oldFlag })

	t.Setenv("LURPICUI_UPDATE_GOLDEN", "0")

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	AssertGolden(t, s, "policy_flag_takes_precedence")
	if _, err := os.Stat(filepath.Join(goldenBaseDir, "policy_flag_takes_precedence.png")); err != nil {
		t.Fatalf("golden was not created: flag should take precedence over env var: %v", err)
	}
}

func TestGoldenUpdate_envVar_values_true_and_yes(t *testing.T) {
	for _, val := range []string{"true", "yes"} {
		t.Run(val, func(t *testing.T) {
			oldDir := goldenBaseDir
			goldenBaseDir = t.TempDir()
			t.Cleanup(func() { goldenBaseDir = oldDir })

			oldFlag := *updateGolden
			*updateGolden = false
			t.Cleanup(func() { *updateGolden = oldFlag })

			t.Setenv("LURPICUI_UPDATE_GOLDEN", val)

			s := NewMemorySurface(1, 1)
			if err := s.Lock(); err != nil {
				t.Fatalf("lock: %v", err)
			}
			s.Buffer()[0] = 255
			s.Buffer()[3] = 255
			if err := s.Unlock(nil); err != nil {
				t.Fatalf("unlock: %v", err)
			}

			AssertGolden(t, s, "policy_env_"+val)
			if _, err := os.Stat(filepath.Join(goldenBaseDir, "policy_env_"+val+".png")); err != nil {
				t.Fatalf("golden was not created via env var=%q: %v", val, err)
			}
		})
	}
}
