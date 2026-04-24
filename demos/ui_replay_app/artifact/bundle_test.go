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
		Family:      "basic",
		Environment: model.Environment{
			Theme:   "baseline",
			Density: "default",
			Backend: "software",
		},
	}
	scenario.Environment.WindowSize.Width = 1400
	scenario.Environment.WindowSize.Height = 900

	builder := NewBundleBuilder(scenario, t.TempDir())
	builder.SetRunResults(&model.RunResult{
		Status: model.StatusPassed,
		AssertionResults: []model.AssertionResult{
			{Step: 1, Type: model.AssertSceneID, Passed: true},
		},
	})
	builder.SetProvenance(ProvenanceInfo{
		Platform: "test",
	})
	builder.SetDiagnosticsSummary(map[string]interface{}{"fps": 60})

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
		if bundle.OutputPath == "" {
			t.Fatal("Build() should set deterministic output path")
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
		if bundle.Manifest.ScenarioFamily != "basic" {
			t.Errorf("ScenarioFamily = %v, want basic", bundle.Manifest.ScenarioFamily)
		}
		if bundle.Manifest.ActionCount != 0 {
			t.Errorf("ActionCount = %d, want 0", bundle.Manifest.ActionCount)
		}
		if len(bundle.Manifest.AssertionResults) != 1 {
			t.Errorf("AssertionResults len = %d, want 1", len(bundle.Manifest.AssertionResults))
		}
		if bundle.Manifest.DiagnosticsSummary["fps"] != 60 {
			t.Errorf("DiagnosticsSummary[fps] = %v, want 60", bundle.Manifest.DiagnosticsSummary["fps"])
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

	t.Run("deterministic output path", func(t *testing.T) {
		again, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if again.OutputPath != builder.generateBundlePath() {
			t.Errorf("OutputPath = %q, want deterministic path", again.OutputPath)
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

	t.Run("artifact order is deterministic", func(t *testing.T) {
		bundle, _ := builder.Build()
		if len(bundle.Artifacts) == 0 {
			t.Fatal("bundle should contain manifest artifact")
		}
		if bundle.Artifacts[0].Name != "manifest.json" {
			t.Fatalf("first artifact = %q, want manifest.json", bundle.Artifacts[0].Name)
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

func TestBundle_DeterministicArtifactNaming(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "test.naming",
		DisplayName: "Naming Test",
		Schema:      "1.0",
		Family:      "basic",
		Environment: model.Environment{
			Theme:    "baseline",
			Density:  "default",
			Backend:  "software",
			Platform: "linux",
		},
	}
	scenario.Environment.WindowSize.Width = 1400
	scenario.Environment.WindowSize.Height = 900

	outputDir := t.TempDir()

	// Build two bundles with same scenario and environment
	builder1 := NewBundleBuilder(scenario, outputDir)
	builder1.SetRunResult(model.StatusPassed)
	builder1.AddScreenshot("step_1_screenshot", []byte("png1"), nil)
	builder1.AddLog("execution_log", "log content")

	builder2 := NewBundleBuilder(scenario, outputDir)
	builder2.SetRunResult(model.StatusPassed)
	builder2.AddScreenshot("step_1_screenshot", []byte("png1"), nil)
	builder2.AddLog("execution_log", "log content")

	bundle1, err := builder1.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	bundle2, err := builder2.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	t.Run("artifact names are identical across builds", func(t *testing.T) {
		if len(bundle1.Artifacts) != len(bundle2.Artifacts) {
			t.Errorf("Artifact count mismatch: %d vs %d", len(bundle1.Artifacts), len(bundle2.Artifacts))
		}

		for i := range bundle1.Artifacts {
			if i >= len(bundle2.Artifacts) {
				break
			}
			if bundle1.Artifacts[i].Name != bundle2.Artifacts[i].Name {
				t.Errorf("Artifact[%d] name mismatch: %q vs %q", i, bundle1.Artifacts[i].Name, bundle2.Artifacts[i].Name)
			}
		}
	})

	t.Run("bundle paths are deterministic", func(t *testing.T) {
		// Path should be deterministic based on scenario and environment
		if bundle1.OutputPath != bundle2.OutputPath {
			t.Errorf("OutputPath not deterministic: %q vs %q", bundle1.OutputPath, bundle2.OutputPath)
		}
	})

	t.Run("manifest artifacts list is ordered", func(t *testing.T) {
		// Verify artifacts are sorted by type then name
		for i := 1; i < len(bundle1.Manifest.Artifacts); i++ {
			prev := bundle1.Manifest.Artifacts[i-1]
			curr := bundle1.Manifest.Artifacts[i]
			if prev.Type > curr.Type {
				t.Errorf("Artifacts not sorted by type at index %d: %s > %s", i, prev.Type, curr.Type)
			}
			if prev.Type == curr.Type && prev.Name > curr.Name {
				t.Errorf("Artifacts not sorted by name at index %d: %s > %s", i, prev.Name, curr.Name)
			}
		}
	})
}

func TestBundleBuilder_generateBundlePath_Determinism(t *testing.T) {
	scenario := &model.Scenario{
		ID:          "my.scenario.id",
		DisplayName: "My Scenario",
		Schema:      "1.0",
		Environment: model.Environment{
			Theme:    "dark_theme",
			Density:  "high_density",
			Backend:  "vulkan_backend",
			Platform: "linux_platform",
		},
	}

	outputDir := "/some/output/dir"
	builder := NewBundleBuilder(scenario, outputDir)

	// Generate path multiple times
	path1 := builder.generateBundlePath()
	path2 := builder.generateBundlePath()
	path3 := builder.generateBundlePath()

	t.Run("path is deterministic across calls", func(t *testing.T) {
		if path1 != path2 || path2 != path3 {
			t.Errorf("generateBundlePath() not deterministic: %q, %q, %q", path1, path2, path3)
		}
	})

	t.Run("path contains normalized scenario components", func(t *testing.T) {
		// Scenario ID with dots is preserved (normalizeName only converts spaces, slashes, backslashes, colons)
		if !contains(path1, "my.scenario.id") {
			t.Errorf("Path should contain scenario ID: %q", path1)
		}
		// Theme with underscore (spaces converted to underscores)
		if !contains(path1, "dark_theme") {
			t.Errorf("Path should contain theme: %q", path1)
		}
		// Backend with underscore
		if !contains(path1, "vulkan_backend") {
			t.Errorf("Path should contain backend: %q", path1)
		}
	})

	t.Run("path starts with output directory", func(t *testing.T) {
		if len(path1) < len(outputDir) || path1[:len(outputDir)] != outputDir {
			t.Errorf("Path %q should start with output directory %q", path1, outputDir)
		}
	})
}

func TestBundle_ManifestStability(t *testing.T) {
	scenario := &model.Scenario{
		ID:           "test.stability",
		DisplayName:  "Stability Test",
		Schema:       "1.0",
		Family:       "test_family",
		Tags:         []string{"tag1", "tag2", "tag3"},
		Capabilities: []model.Capability{model.CapSceneLoad, model.CapAssertions},
		Environment: model.Environment{
			Theme:   "baseline",
			Density: "default",
			Backend: "software",
		},
	}

	builder := NewBundleBuilder(scenario, t.TempDir())
	builder.SetRunResult(model.StatusPassed)
	builder.SetRunLabel("test_run")

	bundle, _ := builder.Build()

	t.Run("manifest contains all scenario metadata", func(t *testing.T) {
		if bundle.Manifest.ScenarioID != string(scenario.ID) {
			t.Errorf("ScenarioID = %v, want %v", bundle.Manifest.ScenarioID, scenario.ID)
		}
		if bundle.Manifest.ScenarioFamily != scenario.Family {
			t.Errorf("ScenarioFamily = %v, want %v", bundle.Manifest.ScenarioFamily, scenario.Family)
		}
		if len(bundle.Manifest.Tags) != len(scenario.Tags) {
			t.Errorf("Tags length = %v, want %v", len(bundle.Manifest.Tags), len(scenario.Tags))
		}
		if len(bundle.Manifest.Capabilities) != len(scenario.Capabilities) {
			t.Errorf("Capabilities length = %v, want %v", len(bundle.Manifest.Capabilities), len(scenario.Capabilities))
		}
	})

	t.Run("manifest environment is captured", func(t *testing.T) {
		if bundle.Manifest.Environment.Theme != scenario.Environment.Theme {
			t.Errorf("Theme = %v, want %v", bundle.Manifest.Environment.Theme, scenario.Environment.Theme)
		}
		if bundle.Manifest.Environment.Backend != scenario.Environment.Backend {
			t.Errorf("Backend = %v, want %v", bundle.Manifest.Environment.Backend, scenario.Environment.Backend)
		}
	})
}
