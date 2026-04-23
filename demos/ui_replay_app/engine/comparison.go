package engine

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/ui_replay/artifact"
	"codeburg.org/lexbit/ui_replay/model"
)

// ComparisonMode represents different comparison modes for runs.
type ComparisonMode string

const (
	ModeThemeComparison    ComparisonMode = "theme"
	ModeDensityComparison  ComparisonMode = "density"
	ModeBackendComparison  ComparisonMode = "backend"
	ModePlatformComparison ComparisonMode = "platform"
)

// ComparisonRunner executes the same scenario under different conditions.
type ComparisonRunner struct {
	baseRunner *Runner
	mode       ComparisonMode
	variants   []VariantConfig
}

// VariantConfig defines one variation of the comparison.
type VariantConfig struct {
	Name        string
	Theme       string
	Density     string
	Backend     string
	Platform    string
	Description string
}

// ComparisonResult holds results from all variant runs.
type ComparisonResult struct {
	Mode       ComparisonMode
	ScenarioID string
	Variants   []VariantResult
	StartTime  time.Time
	EndTime    time.Time
}

// VariantResult holds the result of a single variant run.
type VariantResult struct {
	Variant   VariantConfig
	RunResult *model.RunResult
	Bundle    *artifact.Bundle
	Error     error
}

// NewComparisonRunner creates a comparison runner.
func NewComparisonRunner(runner *Runner, mode ComparisonMode) *ComparisonRunner {
	return &ComparisonRunner{
		baseRunner: runner,
		mode:       mode,
		variants:   make([]VariantConfig, 0),
	}
}

// AddVariant adds a variant configuration.
func (cr *ComparisonRunner) AddVariant(config VariantConfig) {
	cr.variants = append(cr.variants, config)
}

// GenerateThemeVariants creates variants for theme comparison.
func (cr *ComparisonRunner) GenerateThemeVariants(themes []string) {
	cr.mode = ModeThemeComparison
	cr.variants = make([]VariantConfig, 0)

	for _, theme := range themes {
		cr.AddVariant(VariantConfig{
			Name:        fmt.Sprintf("theme_%s", theme),
			Theme:       theme,
			Description: fmt.Sprintf("Run with %s theme", theme),
		})
	}
}

// GenerateDensityVariants creates variants for density comparison.
func (cr *ComparisonRunner) GenerateDensityVariants(densities []string) {
	cr.mode = ModeDensityComparison
	cr.variants = make([]VariantConfig, 0)

	for _, density := range densities {
		cr.AddVariant(VariantConfig{
			Name:        fmt.Sprintf("density_%s", density),
			Density:     density,
			Description: fmt.Sprintf("Run with %s density", density),
		})
	}
}

// GenerateBackendVariants creates variants for backend comparison.
func (cr *ComparisonRunner) GenerateBackendVariants(backends []string) {
	cr.mode = ModeBackendComparison
	cr.variants = make([]VariantConfig, 0)

	for _, backend := range backends {
		cr.AddVariant(VariantConfig{
			Name:        fmt.Sprintf("backend_%s", backend),
			Backend:     backend,
			Description: fmt.Sprintf("Run with %s backend", backend),
		})
	}
}

// GeneratePlatformVariants creates variants for platform comparison.
func (cr *ComparisonRunner) GeneratePlatformVariants(platforms []string) {
	cr.mode = ModePlatformComparison
	cr.variants = make([]VariantConfig, 0)

	for _, platform := range platforms {
		cr.AddVariant(VariantConfig{
			Name:        fmt.Sprintf("platform_%s", platform),
			Platform:    platform,
			Description: fmt.Sprintf("Run on %s platform", platform),
		})
	}
}

// Run executes all variants and returns comparison results.
func (cr *ComparisonRunner) Run(scenario *model.Scenario) (*ComparisonResult, error) {
	if len(cr.variants) == 0 {
		return nil, fmt.Errorf("no variants configured")
	}

	result := &ComparisonResult{
		Mode:       cr.mode,
		ScenarioID: string(scenario.ID),
		Variants:   make([]VariantResult, 0, len(cr.variants)),
		StartTime:  time.Now(),
	}

	for _, variant := range cr.variants {
		// Apply variant configuration
		if variant.Theme != "" {
			scenario.Environment.Theme = variant.Theme
		}
		if variant.Density != "" {
			scenario.Environment.Density = variant.Density
		}
		if variant.Backend != "" {
			scenario.Environment.Backend = variant.Backend
		}

		// Run the scenario
		runResult, err := cr.baseRunner.Run(scenario)

		variantResult := VariantResult{
			Variant:   variant,
			RunResult: runResult,
			Error:     err,
		}

		result.Variants = append(result.Variants, variantResult)
	}

	result.EndTime = time.Now()
	return result, nil
}

// DiffReport generates a diff report between variants.
type DiffReport struct {
	Mode         ComparisonMode
	Differences  []Difference
	Similarities []Similarity
}

// Difference records a difference between variants.
type Difference struct {
	VariantA string
	VariantB string
	Field    string
	ValueA   interface{}
	ValueB   interface{}
	Severity string // "warning", "error", "info"
}

// Similarity records a similarity between variants.
type Similarity struct {
	Field    string
	Variants []string
	Value    interface{}
}

// GenerateDiffReport compares all variants and generates a report.
func (cr *ComparisonResult) GenerateDiffReport() *DiffReport {
	report := &DiffReport{
		Mode:         cr.Mode,
		Differences:  make([]Difference, 0),
		Similarities: make([]Similarity, 0),
	}

	// Compare each pair of variants
	for i := 0; i < len(cr.Variants); i++ {
		for j := i + 1; j < len(cr.Variants); j++ {
			a := cr.Variants[i]
			b := cr.Variants[j]

			// Compare results
			if a.RunResult != nil && b.RunResult != nil {
				if a.RunResult.Status != b.RunResult.Status {
					report.Differences = append(report.Differences, Difference{
						VariantA: a.Variant.Name,
						VariantB: b.Variant.Name,
						Field:    "status",
						ValueA:   a.RunResult.Status,
						ValueB:   b.RunResult.Status,
						Severity: "error",
					})
				}

				if a.RunResult.StepsExecuted != b.RunResult.StepsExecuted {
					report.Differences = append(report.Differences, Difference{
						VariantA: a.Variant.Name,
						VariantB: b.Variant.Name,
						Field:    "steps_executed",
						ValueA:   a.RunResult.StepsExecuted,
						ValueB:   b.RunResult.StepsExecuted,
						Severity: "warning",
					})
				}
			}

			// Compare errors
			if (a.Error != nil) != (b.Error != nil) {
				report.Differences = append(report.Differences, Difference{
					VariantA: a.Variant.Name,
					VariantB: b.Variant.Name,
					Field:    "error",
					ValueA:   a.Error,
					ValueB:   b.Error,
					Severity: "error",
				})
			}
		}
	}

	return report
}

// AllPassed returns true if all variants passed.
func (cr *ComparisonResult) AllPassed() bool {
	for _, v := range cr.Variants {
		if v.RunResult == nil || v.RunResult.Status != model.StatusPassed {
			return false
		}
	}
	return true
}

// HasDifferences returns true if any differences were found.
func (cr *ComparisonResult) HasDifferences() bool {
	report := cr.GenerateDiffReport()
	return len(report.Differences) > 0
}

// Summary returns a human-readable summary.
func (cr *ComparisonResult) Summary() string {
	passed := 0
	failed := 0
	for _, v := range cr.Variants {
		if v.RunResult != nil && v.RunResult.Status == model.StatusPassed {
			passed++
		} else {
			failed++
		}
	}

	duration := cr.EndTime.Sub(cr.StartTime)
	return fmt.Sprintf("%s comparison: %d/%d passed (%v)", cr.Mode, passed, len(cr.Variants), duration)
}
