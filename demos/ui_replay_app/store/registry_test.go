package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"codeburg.org/lexbit/ui_replay/model"
)

func TestScenarioRegistry_Add(t *testing.T) {
	registry := NewScenarioRegistry()

	t.Run("add valid scenario", func(t *testing.T) {
		scenario := model.NewFixtureScenario("test.add", "Test Add")
		scenario.Actions = []model.Action{{Type: model.ActionWaitFrames}}
		scenario.Capabilities = []model.Capability{model.CapSceneLoad}
		s := &scenario
		if err := registry.Add(s); err != nil {
			t.Errorf("Add() error = %v", err)
		}
		if registry.Count() != 1 {
			t.Errorf("Count() = %d, want 1", registry.Count())
		}
	})

	t.Run("reject invalid scenario", func(t *testing.T) {
		s := &model.Scenario{
			ID: "test.invalid",
		}
		if err := registry.Add(s); err == nil {
			t.Error("Add() expected error for invalid scenario")
		}
	})

	t.Run("reject invalid capability", func(t *testing.T) {
		scenario := model.NewFixtureScenario("test.badcap", "Bad Capability")
		scenario.Actions = []model.Action{{Type: model.ActionWaitFrames}}
		scenario.Capabilities = []model.Capability{model.CapSceneLoad, model.Capability("bogus")}
		if err := registry.Add(&scenario); err == nil {
			t.Error("Add() expected error for invalid capability")
		}
	})

	t.Run("reject nil scenario", func(t *testing.T) {
		if err := registry.Add(nil); err == nil {
			t.Error("Add() expected error for nil scenario")
		}
	})
}

func TestScenarioRegistry_Get(t *testing.T) {
	registry := NewScenarioRegistry()
	scenario := model.NewFixtureScenario("test.get", "Test Get")
	scenario.Actions = []model.Action{{Type: model.ActionWaitFrames}}
	scenario.Capabilities = []model.Capability{model.CapSceneLoad}
	s := &scenario
	registry.Add(s)

	t.Run("get existing", func(t *testing.T) {
		got, ok := registry.Get("test.get")
		if !ok {
			t.Error("Get() expected to find scenario")
		}
		if got.ID != "test.get" {
			t.Errorf("Get() ID = %v, want test.get", got.ID)
		}
		got.DisplayName = "mutated"
		again, ok := registry.Get("test.get")
		if !ok {
			t.Fatal("Get() expected to find scenario on second read")
		}
		if again.DisplayName != "Test Get" {
			t.Errorf("Registry scenario was mutated through Get(): got %q", again.DisplayName)
		}
	})

	t.Run("get non-existing", func(t *testing.T) {
		_, ok := registry.Get("test.nonexistent")
		if ok {
			t.Error("Get() expected not to find scenario")
		}
	})
}

func TestScenarioRegistry_All(t *testing.T) {
	registry := NewScenarioRegistry()
	s1 := model.NewFixtureScenario("b", "B")
	s1.Actions = []model.Action{{Type: model.ActionWaitFrames}}
	s1.Capabilities = []model.Capability{model.CapSceneLoad}
	s2 := model.NewFixtureScenario("a", "A")
	s2.Actions = []model.Action{{Type: model.ActionWaitFrames}}
	s2.Capabilities = []model.Capability{model.CapSceneLoad}
	registry.Add(&s1)
	registry.Add(&s2)

	all := registry.All()
	if len(all) != 2 {
		t.Fatalf("All() len = %d, want 2", len(all))
	}
	// Should be sorted by DisplayName
	if all[0].DisplayName != "A" {
		t.Errorf("First item = %v, want A", all[0].DisplayName)
	}
}

func TestScenarioRegistry_LoadResults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test scenario files
	validScenario := `{"id": "test.valid", "display_name": "Valid", "schema": "1.0", "actions": [{"type": "wait_frames"}], "capabilities": ["scene_load"]}`
	invalidScenario := `{"id": "test.invalid", "display_name": "Invalid", "schema": "2.0"}`
	invalidCapability := `{"id": "test.badcap", "display_name": "Bad Cap", "schema": "1.0", "actions": [{"type": "wait_frames"}], "capabilities": ["scene_load", "bogus"]}`
	invalidJSON := `{invalid json`

	os.WriteFile(filepath.Join(tmpDir, "valid.json"), []byte(validScenario), 0644)
	os.WriteFile(filepath.Join(tmpDir, "invalid.json"), []byte(invalidScenario), 0644)
	os.WriteFile(filepath.Join(tmpDir, "badcap.json"), []byte(invalidCapability), 0644)
	os.WriteFile(filepath.Join(tmpDir, "bad.json"), []byte(invalidJSON), 0644)

	// Reset the singleton state for testing
	pathsOnce = sync.Once{}
	pathsErr = nil

	InitRegistry(tmpDir, tmpDir, tmpDir)
	registry := ScenarioRegistryStore.Get()

	results := registry.LoadResults()
	if len(results) != 4 {
		t.Errorf("LoadResults() len = %d, want 4", len(results))
	}

	var loaded, invalid, errorCount int
	for _, r := range results {
		switch r.Status {
		case "loaded":
			loaded++
		case "invalid":
			invalid++
		case "error":
			errorCount++
		}
	}

	if loaded != 1 {
		t.Errorf("Loaded count = %d, want 1", loaded)
	}
	if invalid != 2 {
		t.Errorf("Invalid count = %d, want 2", invalid)
	}
	if errorCount != 1 {
		t.Errorf("Error count = %d, want 1", errorCount)
	}

	if registry.ValidCount() != 1 {
		t.Errorf("ValidCount() = %d, want 1", registry.ValidCount())
	}
	if registry.InvalidCount() != 2 {
		t.Errorf("InvalidCount() = %d, want 2", registry.InvalidCount())
	}
}

func TestLoadResult(t *testing.T) {
	tests := []struct {
		name   string
		result LoadResult
		want   string
	}{
		{
			name:   "loaded",
			result: LoadResult{Path: "test.json", Status: "loaded", ID: "test.1"},
			want:   "loaded",
		},
		{
			name:   "invalid",
			result: LoadResult{Path: "test.json", Status: "invalid", Error: "missing id"},
			want:   "invalid",
		},
		{
			name:   "error",
			result: LoadResult{Path: "test.json", Status: "error", Error: "read failed"},
			want:   "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Status != tt.want {
				t.Errorf("Status = %v, want %v", tt.result.Status, tt.want)
			}
		})
	}
}

func TestSampleScenarioLoadsCleanly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "scenarios", "sample.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var scenario model.Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if err := scenario.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	registry := NewScenarioRegistry()
	if err := registry.Add(&scenario); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if registry.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", registry.Count())
	}
}
