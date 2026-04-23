package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/ui_replay/model"
)

func TestBundleBuilder_Build(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.scenario",
		DisplayName: "Test Scenario",
		Schema:      "1.0",
		Environment: model.Environment{
			Theme:   "baseline",
			Density: "default",
			Backend: "software",
		},
	}
	scenario.Environment.WindowSize.Width = 1400
	scenario.Environment.WindowSize.Height = 900

	builder := NewBundleBuilder(scenario, t.TempDir())
	builder.SetRunResult(model.StatusPassed)
	builder.SetProvenance(ProvenanceInfo{
		Platform: "test",
	})

	t.Run("empty bundle builds successfully", func(t *testing.T) {
		bundle, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if bundle == nil {
			t.Fatal("Build() returned nil bundle")
		}
		if bundle.Manifest == nil {
			t.Fatal("Bundle missing manifest")
		}
	})

	t.Run("bundle contains correct metadata", func(t *testing.T) {
		bundle, _ := builder.Build()

		if bundle.Manifest.ScenarioID != "test.scenario" {
			t.Errorf("ScenarioID = %v, want test.scenario", bundle.Manifest.ScenarioID)
		}
		if bundle.Manifest.ScenarioName != "Test Scenario" {
			t.Errorf("ScenarioName = %v, want 'Test Scenario'", bundle.Manifest.ScenarioName)
		}
		if bundle.Manifest.RunResult != model.StatusPassed {
			t.Errorf("RunResult = %v, want passed", bundle.Manifest.RunResult)
		}
		if bundle.Manifest.Version != BundleVersion {
			t.Errorf("Version = %v, want %v", bundle.Manifest.Version, BundleVersion)
		}
	})

	t.Run("environment captured correctly", func(t *testing.T) {
		bundle, _ := builder.Build()

		if bundle.Manifest.Environment.Theme != "baseline" {
			t.Errorf("Theme = %v, want baseline", bundle.Manifest.Environment.Theme)
		}
		if bundle.Manifest.Environment.WindowWidth != 1400 {
			t.Errorf("WindowWidth = %d, want 1400", bundle.Manifest.Environment.WindowWidth)
		}
	})
}

func TestBundleBuilder_AddArtifacts(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.artifacts",
		DisplayName: "Artifact Test",
		Schema:      "1.0",
	}

	builder := NewBundleBuilder(scenario, t.TempDir())
	builder.SetRunResult(model.StatusPassed)

	t.Run("add screenshot", func(t *testing.T) {
		builder.AddScreenshot("main view", []byte("png data"), map[string]interface{}{"step": 1})

		bundle, _ := builder.Build()
		arts := bundle.GetArtifactsByType(TypeScreenshot)
		if len(arts) != 1 {
			t.Errorf("Expected 1 screenshot, got %d", len(arts))
		}
		if arts[0].Name != "main_view.png" {
			t.Errorf("Name = %v, want main_view.png", arts[0].Name)
		}
	})

	t.Run("add scene state", func(t *testing.T) {
		state := map[string]interface{}{"scene": "basic", "controls": []string{"button1"}}
		err := builder.AddSceneState("scene_export", state)
		if err != nil {
			t.Errorf("AddSceneState() error = %v", err)
		}

		bundle, _ := builder.Build()
		arts := bundle.GetArtifactsByType(TypeSceneState)
		if len(arts) != 1 {
			t.Errorf("Expected 1 scene state, got %d", len(arts))
		}
	})

	t.Run("add diagnostics", func(t *testing.T) {
		diagnostics := map[string]interface{}{"fps": 60, "memory": "10MB"}
		err := builder.AddDiagnostics("perf", diagnostics)
		if err != nil {
			t.Errorf("AddDiagnostics() error = %v", err)
		}

		bundle, _ := builder.Build()
		arts := bundle.GetArtifactsByType(TypeDiagnostics)
		if len(arts) != 1 {
			t.Errorf("Expected 1 diagnostics, got %d", len(arts))
		}
	})

	t.Run("add log", func(t *testing.T) {
		builder.AddLog("execution", "step 1: started\nstep 1: completed")

		bundle, _ := builder.Build()
		arts := bundle.GetArtifactsByType(TypeLog)
		if len(arts) != 1 {
			t.Errorf("Expected 1 log, got %d", len(arts))
		}
		if string(arts[0].Data) != "step 1: started\nstep 1: completed" {
			t.Error("Log content mismatch")
		}
	})

	t.Run("add assertion report", func(t *testing.T) {
		report := map[string]interface{}{
			"total":   5,
			"passed":  4,
			"failed":  1,
			"summary": "4/5 passed",
		}
		err := builder.AddAssertionReport(report)
		if err != nil {
			t.Errorf("AddAssertionReport() error = %v", err)
		}

		bundle, _ := builder.Build()
		art, found := bundle.GetArtifact("assertion_report.json")
		if !found {
			t.Error("assertion_report.json not found")
		}
		if art.Type != TypeAssertionReport {
			t.Errorf("Type = %v, want assertion_report", art.Type)
		}
	})
}

func TestBundle_SaveAndLoad(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.save_load",
		DisplayName: "Save/Load Test",
		Schema:      "1.0",
	}

	t.Run("save to disk and load", func(t *testing.T) {
		outputDir := t.TempDir()
		builder := NewBundleBuilder(scenario, outputDir)
		builder.SetRunResult(model.StatusPassed)
		builder.AddLog("test", "test content")

		bundle, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		// Save to disk
		if err := bundle.SaveToDisk(); err != nil {
			t.Fatalf("SaveToDisk() error = %v", err)
		}

		// Verify directory exists
		if _, err := os.Stat(bundle.OutputPath); os.IsNotExist(err) {
			t.Fatal("Bundle directory not created")
		}

		// Load bundle
		loaded, err := LoadBundle(bundle.OutputPath)
		if err != nil {
			t.Fatalf("LoadBundle() error = %v", err)
		}

		if loaded.Manifest.ScenarioID != string(scenario.ID) {
			t.Errorf("Loaded ScenarioID = %v, want %v", loaded.Manifest.ScenarioID, scenario.ID)
		}
	})

	t.Run("save as zip and load", func(t *testing.T) {
		outputDir := t.TempDir()
		builder := NewBundleBuilder(scenario, outputDir)
		builder.SetRunResult(model.StatusPassed)
		builder.AddLog("test", "zip test content")

		bundle, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		// Save as ZIP
		zipPath := filepath.Join(outputDir, "bundle.zip")
		if err := bundle.SaveAsZip(zipPath); err != nil {
			t.Fatalf("SaveAsZip() error = %v", err)
		}

		// Verify ZIP exists
		if _, err := os.Stat(zipPath); os.IsNotExist(err) {
			t.Fatal("ZIP file not created")
		}

		// Load from ZIP
		loaded, err := LoadBundleFromZip(zipPath)
		if err != nil {
			t.Fatalf("LoadBundleFromZip() error = %v", err)
		}

		if loaded.Manifest.ScenarioID != string(scenario.ID) {
			t.Errorf("Loaded ScenarioID = %v, want %v", loaded.Manifest.ScenarioID, scenario.ID)
		}

		// Check artifact content
		logArt, found := loaded.GetArtifact("test.log")
		if !found {
			t.Error("test.log not found in loaded bundle")
		}
		if string(logArt.Data) != "zip test content" {
			t.Error("Log content mismatch after ZIP load")
		}
	})
}

func TestBundle_Validate(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.validate",
		DisplayName: "Validate Test",
		Schema:      "1.0",
	}

	t.Run("valid bundle passes", func(t *testing.T) {
		builder := NewBundleBuilder(scenario, t.TempDir())
		builder.SetRunResult(model.StatusPassed)
		builder.AddLog("test", "content")

		bundle, _ := builder.Build()
		if err := bundle.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("missing manifest fails", func(t *testing.T) {
		bundle := &Bundle{
			Manifest: nil,
		}
		if err := bundle.Validate(); err == nil {
			t.Error("Validate() should fail with missing manifest")
		}
	})

	t.Run("wrong version fails", func(t *testing.T) {
		builder := NewBundleBuilder(scenario, t.TempDir())
		bundle, _ := builder.Build()
		bundle.Manifest.Version = "2.0"

		if err := bundle.Validate(); err == nil {
			t.Error("Validate() should fail with wrong version")
		}
	})
}

func TestBundle_GetArtifact(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.get",
		DisplayName: "Get Test",
		Schema:      "1.0",
	}

	builder := NewBundleBuilder(scenario, t.TempDir())
	builder.AddLog("first", "first content")
	builder.AddLog("second", "second content")
	bundle, _ := builder.Build()

	t.Run("get existing artifact", func(t *testing.T) {
		art, found := bundle.GetArtifact("first.log")
		if !found {
			t.Error("GetArtifact() should find first.log")
		}
		if string(art.Data) != "first content" {
			t.Error("Wrong artifact content")
		}
	})

	t.Run("get non-existing artifact", func(t *testing.T) {
		_, found := bundle.GetArtifact("nonexistent.log")
		if found {
			t.Error("GetArtifact() should not find nonexistent.log")
		}
	})

	t.Run("get by type", func(t *testing.T) {
		logs := bundle.GetArtifactsByType(TypeLog)
		if len(logs) != 2 {
			t.Errorf("Expected 2 logs, got %d", len(logs))
		}
	})
}

func TestBundle_Summary(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.summary",
		DisplayName: "Summary Test",
		Schema:      "1.0",
	}

	builder := NewBundleBuilder(scenario, t.TempDir())
	builder.SetRunResult(model.StatusPassed)
	builder.AddLog("test1", "content1")
	builder.AddLog("test2", "content2")
	bundle, _ := builder.Build()

	summary := bundle.Summary()
	if summary == "" {
		t.Error("Summary() should not return empty string")
	}
	if !contains(summary, "test.summary") {
		t.Error("Summary should contain scenario ID")
	}
	if !contains(summary, "passed") {
		t.Error("Summary should contain run result")
	}
}

func TestNormalizeName(t *testing.T) {
	builder := NewBundleBuilder(&model.Scenario{}, "")

	tests := []struct {
		input    string
		expected string
	}{
		{"Simple Name", "simple_name"},
		{"With/Slash", "with_slash"},
		{"With\\Backslash", "with_backslash"},
		{"With:Colon", "with_colon"},
		{"UPPER CASE", "upper_case"},
		{"Mixed Case", "mixed_case"},
	}

	for _, tt := range tests {
		got := builder.normalizeName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCalculateChecksum(t *testing.T) {
	data1 := []byte("test data")
	data2 := []byte("test data")
	data3 := []byte("different data")

	sum1 := calculateChecksum(data1)
	sum2 := calculateChecksum(data2)
	sum3 := calculateChecksum(data3)

	if sum1 != sum2 {
		t.Error("Same data should produce same checksum")
	}
	if sum1 == sum3 {
		t.Error("Different data should produce different checksum")
	}
	if len(sum1) != 64 { // SHA-256 hex is 64 chars
		t.Errorf("Checksum length = %d, want 64", len(sum1))
	}
}

// Helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
