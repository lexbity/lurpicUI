package engine

import (
	"testing"

	"codeburg.org/lexbit/ui_replay/artifact"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

func TestMatrixCell_String(t *testing.T) {
	cell := MatrixCell{
		Backend:  "software",
		Platform: "linux",
		Theme:    "baseline",
		Density:  "default",
	}

	expected := "software_linux_baseline_default"
	if got := cell.String(); got != expected {
		t.Errorf("String() = %v, want %v", got, expected)
	}
}

func TestDefaultMatrixConfig(t *testing.T) {
	config := DefaultMatrixConfig()

	if len(config.Backends) == 0 {
		t.Error("Backends should not be empty")
	}
	if len(config.Platforms) == 0 {
		t.Error("Platforms should not be empty")
	}
	if len(config.Themes) == 0 {
		t.Error("Themes should not be empty")
	}
	if len(config.Densities) == 0 {
		t.Error("Densities should not be empty")
	}
}

func TestMatrixExecutor_generateCells(t *testing.T) {
	config := &MatrixConfig{
		Backends:  []string{"software", "vulkan"},
		Platforms: []string{"linux"},
		Themes:    []string{"baseline"},
		Densities: []string{"default"},
	}

	registry := store.NewScenarioRegistry()
	executor := NewMatrixExecutor(config, registry, "/tmp")

	cells := executor.generateCells()

	expectedCount := 2 * 1 * 1 * 1 // 2 backends, 1 platform, 1 theme, 1 density
	if len(cells) != expectedCount {
		t.Errorf("Expected %d cells, got %d", expectedCount, len(cells))
	}
}

func TestMatrixExecutor_calculateSummary(t *testing.T) {
	config := DefaultMatrixConfig()
	registry := store.NewScenarioRegistry()
	executor := NewMatrixExecutor(config, registry, "/tmp")

	cells := []CellResult{
		{Error: nil, Result: &model.RunResult{Status: model.StatusPassed}},
		{Error: nil, Result: &model.RunResult{Status: model.StatusPassed}},
		{Error: nil, Result: &model.RunResult{Status: model.StatusFailed}},
		{Error: &testErrorLocal{msg: "error"}, Result: nil},
	}

	summary := executor.calculateSummary(cells)

	if summary.TotalCells != 4 {
		t.Errorf("TotalCells = %d, want 4", summary.TotalCells)
	}
	if summary.PassedCells != 2 {
		t.Errorf("PassedCells = %d, want 2", summary.PassedCells)
	}
	if summary.FailedCells != 1 {
		t.Errorf("FailedCells = %d, want 1", summary.FailedCells)
	}
	if summary.ErrorCells != 1 {
		t.Errorf("ErrorCells = %d, want 1", summary.ErrorCells)
	}
}

func TestMatrixExecutor_SetCellCallbacks(t *testing.T) {
	config := DefaultMatrixConfig()
	registry := store.NewScenarioRegistry()
	executor := NewMatrixExecutor(config, registry, "/tmp")

	executor.SetCellCallbacks(
		func(cell MatrixCell, scenario model.ScenarioID) {},
		func(cell MatrixCell, result *model.RunResult) {},
		func(cell MatrixCell, err error) {},
	)

	// Verify callbacks are set (can't easily test execution without full setup)
	if executor.onCellStart == nil {
		t.Error("onCellStart should be set")
	}
	if executor.onCellComplete == nil {
		t.Error("onCellComplete should be set")
	}
	if executor.onCellError == nil {
		t.Error("onCellError should be set")
	}
}

func TestScenarioFilter(t *testing.T) {
	t.Run("filter by tag", func(t *testing.T) {
		filter := FilterByTag("critical")

		scenarioWithTag := &model.Scenario{
			ID:   "test1",
			Tags: []string{"critical", "ui"},
		}
		scenarioWithoutTag := &model.Scenario{
			ID:   "test2",
			Tags: []string{"ui"},
		}

		if !filter(scenarioWithTag) {
			t.Error("Should match scenario with tag")
		}
		if filter(scenarioWithoutTag) {
			t.Error("Should not match scenario without tag")
		}
	})

	t.Run("filter by capability", func(t *testing.T) {
		filter := FilterByCapability("screenshots")

		scenarioWithCap := &model.Scenario{
			ID:           "test1",
			Capabilities: []model.Capability{model.CapScreenshots},
		}
		scenarioWithoutCap := &model.Scenario{
			ID:           "test2",
			Capabilities: []model.Capability{model.CapSceneLoad},
		}

		if !filter(scenarioWithCap) {
			t.Error("Should match scenario with capability")
		}
		if filter(scenarioWithoutCap) {
			t.Error("Should not match scenario without capability")
		}
	})

	t.Run("filter by family", func(t *testing.T) {
		filter := FilterByFamily("input")

		scenarioWithFamily := &model.Scenario{
			ID:     "test1",
			Family: "input",
		}
		scenarioWithTag := &model.Scenario{
			ID:   "test2",
			Tags: []string{"input"},
		}
		scenarioWithoutFamily := &model.Scenario{ID: "test3", Family: "chart"}

		if !filter(scenarioWithFamily) {
			t.Error("Should match scenario with family")
		}
		if !filter(scenarioWithTag) {
			t.Error("Should match scenario with family tag")
		}
		if filter(scenarioWithoutFamily) {
			t.Error("Should not match scenario without family")
		}
	})
}

func TestGeneratePortabilityReport(t *testing.T) {
	matrixResult := &MatrixResult{
		Config: DefaultMatrixConfig(),
		Cells: []CellResult{
			{
				Scenario: "scenario1",
				Cell:     MatrixCell{Backend: "software", Platform: "linux"},
				Result:   &model.RunResult{Status: model.StatusPassed},
			},
			{
				Scenario: "scenario1",
				Cell:     MatrixCell{Backend: "vulkan", Platform: "linux"},
				Result:   &model.RunResult{Status: model.StatusPassed},
			},
			{
				Scenario: "scenario2",
				Cell:     MatrixCell{Backend: "software", Platform: "linux"},
				Result:   &model.RunResult{Status: model.StatusPassed},
			},
			{
				Scenario: "scenario2",
				Cell:     MatrixCell{Backend: "vulkan", Platform: "linux"},
				Result:   &model.RunResult{Status: model.StatusFailed},
			},
		},
	}

	report := GeneratePortabilityReport(matrixResult)

	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
	if report.Matrix == nil {
		t.Error("Matrix should not be nil")
	}
	if report.Summary.TotalScenarios != 2 {
		t.Errorf("TotalScenarios = %d, want 2", report.Summary.TotalScenarios)
	}
	if report.Summary.PortableScenarios != 1 {
		t.Errorf("PortableScenarios = %d, want 1", report.Summary.PortableScenarios)
	}
	if len(report.Differences) == 0 {
		t.Error("Should have detected differences")
	}
}

func TestBaselineManager(t *testing.T) {
	manager := NewBaselineManager()

	cell := MatrixCell{Backend: "software", Platform: "linux", Theme: "baseline", Density: "default"}
	scenarioID := model.ScenarioID("test.scenario")

	t.Run("record and get baseline", func(t *testing.T) {
		fingerprint := RunFingerprint{
			RunID:      "run1",
			FrameCount: 100,
			StepCount:  5,
		}

		manager.RecordBaseline(cell, scenarioID, fingerprint, "/path/to/bundle")

		baseline, ok := manager.GetBaseline(cell, scenarioID)
		if !ok {
			t.Error("Should find recorded baseline")
		}
		if baseline.Cell != cell {
			t.Error("Cell mismatch")
		}
		if baseline.Scenario != scenarioID {
			t.Error("Scenario mismatch")
		}
		if baseline.BundleVersion != artifact.BundleVersion {
			t.Errorf("BundleVersion = %v, want %v", baseline.BundleVersion, artifact.BundleVersion)
		}
	})

	t.Run("get non-existent baseline", func(t *testing.T) {
		_, ok := manager.GetBaseline(MatrixCell{Backend: "vulkan"}, scenarioID)
		if ok {
			t.Error("Should not find non-existent baseline")
		}
	})

	t.Run("compare to baseline", func(t *testing.T) {
		result := &model.RunResult{
			Status:        model.StatusPassed,
			StepsExecuted: 5,
		}

		report, ok := manager.CompareToBaseline(cell, scenarioID, result)
		if !ok {
			t.Error("Should find baseline for comparison")
		}
		if report == nil {
			t.Error("Report should not be nil")
		}
	})

	t.Run("compare to non-existent baseline", func(t *testing.T) {
		result := &model.RunResult{Status: model.StatusPassed}

		_, ok := manager.CompareToBaseline(MatrixCell{Backend: "vulkan"}, scenarioID, result)
		if ok {
			t.Error("Should not find non-existent baseline")
		}
	})

	t.Run("save and reload baselines", func(t *testing.T) {
		dir := t.TempDir()
		if err := manager.SaveBaselines(dir); err != nil {
			t.Fatalf("SaveBaselines() error = %v", err)
		}

		loaded := NewBaselineManager()
		if err := loaded.LoadBaselines(dir); err != nil {
			t.Fatalf("LoadBaselines() error = %v", err)
		}
		baseline, ok := loaded.GetBaseline(cell, scenarioID)
		if !ok {
			t.Fatal("Loaded baseline not found")
		}
		if baseline.BundleVersion != artifact.BundleVersion {
			t.Errorf("Loaded BundleVersion = %v, want %v", baseline.BundleVersion, artifact.BundleVersion)
		}
	})

	t.Run("reject incompatible baseline version", func(t *testing.T) {
		key := cell.String() + "_" + string(scenarioID)
		manager.baselines[key].BundleVersion = "0.9"
		report, ok := manager.CompareToBaseline(cell, scenarioID, &model.RunResult{Status: model.StatusPassed})
		if !ok {
			t.Fatal("CompareToBaseline should still return report for incompatible baseline")
		}
		if report == nil || !report.Detected {
			t.Fatal("Expected detected incompatibility report")
		}
		if report.Severity != "critical" {
			t.Errorf("Severity = %v, want critical", report.Severity)
		}
	})
}

func TestArtifactComparisonPolicy(t *testing.T) {
	cell := MatrixCell{Backend: "software", Platform: "linux"}
	policy := NewArtifactComparisonPolicy(cell)

	if policy.Cell != cell {
		t.Error("Cell should be set")
	}
	if policy.TolerancePercent != 5.0 {
		t.Errorf("TolerancePercent = %f, want 5.0", policy.TolerancePercent)
	}
	if !policy.CompareContent {
		t.Error("CompareContent should be true")
	}
	if policy.CompareTimestamps {
		t.Error("CompareTimestamps should be false")
	}

	t.Run("compare artifacts", func(t *testing.T) {
		a := []byte("content_a")
		b := []byte("content_b")

		diffs := policy.CompareArtifacts(a, b)
		// Should detect differences since content is different
		if len(diffs) == 0 {
			t.Error("Should detect content differences")
		}
	})
}

func TestRegistrySubset(t *testing.T) {
	registry := store.NewScenarioRegistry()

	// Register some test scenarios
	scenario1 := model.NewFixtureScenario("test1", "Test 1")
	scenario1.Family = "basic"
	scenario1.Tags = []string{"critical"}
	scenario1.Capabilities = []model.Capability{model.CapScreenshots}
	scenario2 := model.NewFixtureScenario("test2", "Test 2")
	scenario2.Family = "input"
	scenario2.Tags = []string{"ui"}
	scenario2.Capabilities = []model.Capability{model.CapSceneLoad}
	scenario3 := model.NewFixtureScenario("test3", "Test 3")
	scenario3.Family = "basic"
	scenario3.Tags = []string{"critical", "ui"}
	scenario3.Capabilities = []model.Capability{model.CapAssertions}

	scenarios := []*model.Scenario{
		&scenario1,
		&scenario2,
		&scenario3,
	}

	for _, s := range scenarios {
		registry.Add(s)
	}

	t.Run("subset by tag", func(t *testing.T) {
		subset := RegistrySubset(registry, FilterByTag("critical"))
		if len(subset) != 2 {
			t.Errorf("Expected 2 scenarios with 'critical' tag, got %d", len(subset))
		}
	})

	t.Run("subset by capability", func(t *testing.T) {
		subset := RegistrySubset(registry, FilterByCapability("screenshots"))
		if len(subset) != 1 {
			t.Errorf("Expected 1 scenario with 'screenshots' capability, got %d", len(subset))
		}
	})

	t.Run("subset by family", func(t *testing.T) {
		subset := RegistrySubset(registry, FilterByFamily("basic"))
		if len(subset) != 2 {
			t.Errorf("Expected 2 scenarios with 'basic' family, got %d", len(subset))
		}
	})
}

// testErrorLocal is a simple error implementation for testing (avoiding conflict with logger_test.go)
type testErrorLocal struct {
	msg string
}

func (e *testErrorLocal) Error() string {
	return e.msg
}
