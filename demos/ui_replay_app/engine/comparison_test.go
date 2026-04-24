package engine

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
)

func TestComparisonRunner_GenerateVariants(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	compRunner := NewComparisonRunner(runner, ModeThemeComparison)

	t.Run("generate theme variants", func(t *testing.T) {
		themes := []string{"baseline", "dark", "high_contrast"}
		compRunner.GenerateThemeVariants(themes)

		if len(compRunner.variants) != 3 {
			t.Errorf("Expected 3 variants, got %d", len(compRunner.variants))
		}

		if compRunner.variants[0].Theme != "baseline" {
			t.Errorf("First variant theme = %v, want baseline", compRunner.variants[0].Theme)
		}
	})

	t.Run("generate density variants", func(t *testing.T) {
		compRunner.GenerateDensityVariants([]string{"compact", "comfortable", "spacious"})

		if compRunner.mode != ModeDensityComparison {
			t.Errorf("Mode = %v, want density", compRunner.mode)
		}
		if len(compRunner.variants) != 3 {
			t.Errorf("Expected 3 density variants, got %d", len(compRunner.variants))
		}
	})

	t.Run("generate backend variants", func(t *testing.T) {
		compRunner.GenerateBackendVariants([]string{"software", "vulkan"})

		if compRunner.mode != ModeBackendComparison {
			t.Errorf("Mode = %v, want backend", compRunner.mode)
		}
		if len(compRunner.variants) != 2 {
			t.Errorf("Expected 2 backend variants, got %d", len(compRunner.variants))
		}
	})

	t.Run("generate platform variants", func(t *testing.T) {
		compRunner.GeneratePlatformVariants([]string{"linux", "windows", "macos"})

		if compRunner.mode != ModePlatformComparison {
			t.Errorf("Mode = %v, want platform", compRunner.mode)
		}
		if len(compRunner.variants) != 3 {
			t.Errorf("Expected 3 platform variants, got %d", len(compRunner.variants))
		}
	})
}

func TestComparisonRunner_AddVariant(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	compRunner := NewComparisonRunner(runner, ModeThemeComparison)

	compRunner.AddVariant(VariantConfig{
		Name:        "custom",
		Theme:       "custom_theme",
		Density:     "compact",
		Description: "Custom variant",
	})

	if len(compRunner.variants) != 1 {
		t.Errorf("Expected 1 variant, got %d", len(compRunner.variants))
	}

	if compRunner.variants[0].Name != "custom" {
		t.Errorf("Variant name = %v, want custom", compRunner.variants[0].Name)
	}
}

func TestComparisonRunner_RunClonesScenario(t *testing.T) {
	runner := NewRunner(nil, nil)
	compRunner := NewComparisonRunner(runner, ModeThemeComparison)
	compRunner.AddVariant(VariantConfig{
		Name:  "dark",
		Theme: "dark",
	})

	original := &model.Scenario{
		ID:          "test.clone",
		DisplayName: "Clone Test",
		Schema:      model.SchemaVersion,
		Environment: model.Environment{
			Theme:    "baseline",
			Density:  "default",
			Backend:  "software",
			Platform: "linux",
		},
	}

	result, err := compRunner.Run(original)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if original.Environment.Theme != "baseline" {
		t.Fatalf("original.Environment.Theme = %q, want baseline", original.Environment.Theme)
	}
	if len(result.Variants) != 1 {
		t.Fatalf("Variants len = %d, want 1", len(result.Variants))
	}
	if result.Variants[0].InputScenario == nil {
		t.Fatal("Variant input scenario should be captured")
	}
	if result.Variants[0].InputScenario.Environment.Theme != "dark" {
		t.Fatalf("variant input theme = %q, want dark", result.Variants[0].InputScenario.Environment.Theme)
	}
	if result.Variants[0].AppliedEnvironment.Theme != "dark" {
		t.Fatalf("applied theme = %q, want dark", result.Variants[0].AppliedEnvironment.Theme)
	}
}

func TestComparisonResult_GenerateDiffReport(t *testing.T) {
	result := &ComparisonResult{
		Mode:       ModeThemeComparison,
		ScenarioID: "test.scenario",
		Variants: []VariantResult{
			{
				Variant: VariantConfig{Name: "theme_a"},
				RunResult: &model.RunResult{
					Status:        model.StatusPassed,
					StepsExecuted: 5,
				},
			},
			{
				Variant: VariantConfig{Name: "theme_b"},
				RunResult: &model.RunResult{
					Status:        model.StatusFailed,
					StepsExecuted: 3,
				},
			},
		},
	}

	report := result.GenerateDiffReport()

	t.Run("detects status differences", func(t *testing.T) {
		found := false
		for _, diff := range report.Differences {
			if diff.Field == "status" {
				found = true
				if diff.Severity != "error" {
					t.Error("Status difference should have error severity")
				}
			}
		}
		if !found {
			t.Error("Should detect status difference")
		}
	})

	t.Run("detects step count differences", func(t *testing.T) {
		found := false
		for _, diff := range report.Differences {
			if diff.Field == "steps_executed" {
				found = true
			}
		}
		if !found {
			t.Error("Should detect steps_executed difference")
		}
	})
}

func TestComparisonResult_AllPassed(t *testing.T) {
	t.Run("all passed returns true", func(t *testing.T) {
		result := &ComparisonResult{
			Variants: []VariantResult{
				{RunResult: &model.RunResult{Status: model.StatusPassed}},
				{RunResult: &model.RunResult{Status: model.StatusPassed}},
			},
		}
		if !result.AllPassed() {
			t.Error("AllPassed() should be true when all passed")
		}
	})

	t.Run("one failed returns false", func(t *testing.T) {
		result := &ComparisonResult{
			Variants: []VariantResult{
				{RunResult: &model.RunResult{Status: model.StatusPassed}},
				{RunResult: &model.RunResult{Status: model.StatusFailed}},
			},
		}
		if result.AllPassed() {
			t.Error("AllPassed() should be false when any failed")
		}
	})
}

func TestComparisonResult_HasDifferences(t *testing.T) {
	t.Run("has differences when status differs", func(t *testing.T) {
		result := &ComparisonResult{
			Mode: ModeThemeComparison,
			Variants: []VariantResult{
				{
					Variant:   VariantConfig{Name: "a"},
					RunResult: &model.RunResult{Status: model.StatusPassed},
				},
				{
					Variant:   VariantConfig{Name: "b"},
					RunResult: &model.RunResult{Status: model.StatusFailed},
				},
			},
		}
		if !result.HasDifferences() {
			t.Error("HasDifferences() should be true when statuses differ")
		}
	})
}

func TestComparisonResult_Summary(t *testing.T) {
	result := &ComparisonResult{
		Mode:       ModeThemeComparison,
		ScenarioID: "test",
		Variants: []VariantResult{
			{RunResult: &model.RunResult{Status: model.StatusPassed}},
			{RunResult: &model.RunResult{Status: model.StatusPassed}},
			{RunResult: &model.RunResult{Status: model.StatusFailed}},
		},
	}

	summary := result.Summary()
	if summary == "" {
		t.Error("Summary() should not return empty string")
	}
}

func TestDiffReport_Structure(t *testing.T) {
	report := &DiffReport{
		Mode: ModeThemeComparison,
		Differences: []Difference{
			{
				VariantA: "theme_a",
				VariantB: "theme_b",
				Field:    "status",
				ValueA:   model.StatusPassed,
				ValueB:   model.StatusFailed,
				Severity: "error",
			},
		},
		Similarities: []Similarity{
			{
				Field:    "steps_total",
				Variants: []string{"theme_a", "theme_b"},
				Value:    10,
			},
		},
	}

	if len(report.Differences) != 1 {
		t.Errorf("Expected 1 difference, got %d", len(report.Differences))
	}
	if len(report.Similarities) != 1 {
		t.Errorf("Expected 1 similarity, got %d", len(report.Similarities))
	}
	if report.Mode != ModeThemeComparison {
		t.Errorf("Mode = %v, want theme", report.Mode)
	}
}
