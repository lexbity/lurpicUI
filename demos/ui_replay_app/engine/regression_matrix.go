package engine

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"codeburg.org/lexbit/ui_replay/artifact"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// MatrixCell represents a single backend/platform combination.
type MatrixCell struct {
	Backend  string
	Platform string
	Theme    string
	Density  string
}

// String returns a unique identifier for the cell.
func (mc MatrixCell) String() string {
	return fmt.Sprintf("%s_%s_%s_%s", mc.Backend, mc.Platform, mc.Theme, mc.Density)
}

// MatrixConfig defines the full regression matrix.
type MatrixConfig struct {
	Backends  []string
	Platforms []string
	Themes    []string
	Densities []string
}

// DefaultMatrixConfig returns a default matrix configuration.
func DefaultMatrixConfig() *MatrixConfig {
	return &MatrixConfig{
		Backends:  []string{"software", "vulkan"},
		Platforms: []string{"linux", "windows", "macos"},
		Themes:    []string{"baseline", "dark"},
		Densities: []string{"default", "compact"},
	}
}

// MatrixExecutor runs scenarios across a matrix of configurations.
type MatrixExecutor struct {
	config         *MatrixConfig
	registry       *store.ScenarioRegistry
	outputDir      string
	onCellStart    func(cell MatrixCell, scenario model.ScenarioID)
	onCellComplete func(cell MatrixCell, result *model.RunResult)
	onCellError    func(cell MatrixCell, err error)
	mu             sync.RWMutex
}

// NewMatrixExecutor creates a new matrix executor.
func NewMatrixExecutor(config *MatrixConfig, registry *store.ScenarioRegistry, outputDir string) *MatrixExecutor {
	return &MatrixExecutor{
		config:    config,
		registry:  registry,
		outputDir: outputDir,
	}
}

// SetCellCallbacks sets callbacks for cell execution events.
func (me *MatrixExecutor) SetCellCallbacks(
	onStart func(cell MatrixCell, scenario model.ScenarioID),
	onComplete func(cell MatrixCell, result *model.RunResult),
	onError func(cell MatrixCell, err error),
) {
	me.onCellStart = onStart
	me.onCellComplete = onComplete
	me.onCellError = onError
}

// MatrixResult holds results from a full matrix execution.
type MatrixResult struct {
	Config      *MatrixConfig
	StartedAt   time.Time
	CompletedAt time.Time
	Cells       []CellResult
	Summary     MatrixSummary
}

// CellResult holds the result of executing a scenario in one matrix cell.
type CellResult struct {
	Cell     MatrixCell
	Scenario model.ScenarioID
	Result   *model.RunResult
	Bundle   *artifact.Bundle
	Error    error
	Duration time.Duration
}

// MatrixSummary provides a high-level view of matrix execution.
type MatrixSummary struct {
	TotalCells    int
	PassedCells   int
	FailedCells   int
	ErrorCells    int
	TotalDuration time.Duration
}

// ExecuteMatrix runs all scenarios across all matrix cells.
func (me *MatrixExecutor) ExecuteMatrix(scenarios []model.ScenarioID, runner *Runner) (*MatrixResult, error) {
	result := &MatrixResult{
		Config:    me.config,
		StartedAt: time.Now(),
		Cells:     make([]CellResult, 0),
	}

	// Generate all matrix cells
	cells := me.generateCells()

	// Execute each scenario in each cell
	for _, scenarioID := range scenarios {
		scenario, ok := me.registry.Get(scenarioID)
		if !ok {
			continue // Skip scenarios not in registry
		}

		for _, cell := range cells {
			cellResult := me.executeCell(cell, scenario, runner)
			result.Cells = append(result.Cells, cellResult)
		}
	}

	result.CompletedAt = time.Now()
	result.Summary = me.calculateSummary(result.Cells)

	return result, nil
}

// ExecuteMatrixParallel runs the matrix with parallel cell execution.
func (me *MatrixExecutor) ExecuteMatrixParallel(scenarios []model.ScenarioID, runner *Runner, maxConcurrency int) (*MatrixResult, error) {
	result := &MatrixResult{
		Config:    me.config,
		StartedAt: time.Now(),
		Cells:     make([]CellResult, 0),
	}

	// Generate all matrix cells
	cells := me.generateCells()

	// Create work queue
	type workItem struct {
		cell     MatrixCell
		scenario model.ScenarioID
	}

	workQueue := make([]workItem, 0)
	for _, scenarioID := range scenarios {
		_, ok := me.registry.Get(scenarioID)
		if !ok {
			continue
		}
		for _, cell := range cells {
			workQueue = append(workQueue, workItem{cell, scenarioID})
		}
	}

	// Execute with limited concurrency
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrency)
	resultsChan := make(chan CellResult, len(workQueue))

	for _, work := range workQueue {
		wg.Add(1)
		go func(w workItem) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			scenario, _ := me.registry.Get(w.scenario)
			cellResult := me.executeCell(w.cell, scenario, runner)
			resultsChan <- cellResult
		}(work)
	}

	// Close results channel when done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for cellResult := range resultsChan {
		result.Cells = append(result.Cells, cellResult)
	}

	result.CompletedAt = time.Now()
	result.Summary = me.calculateSummary(result.Cells)

	return result, nil
}

func (me *MatrixExecutor) executeCell(cell MatrixCell, scenario *model.Scenario, runner *Runner) CellResult {
	startTime := time.Now()

	if me.onCellStart != nil {
		me.onCellStart(cell, scenario.ID)
	}

	// Configure environment for this cell
	scenario.Environment.Backend = cell.Backend
	scenario.Environment.Platform = cell.Platform
	scenario.Environment.Theme = cell.Theme
	scenario.Environment.Density = cell.Density

	// Execute the scenario
	runResult, err := runner.Run(scenario)

	cellResult := CellResult{
		Cell:     cell,
		Scenario: scenario.ID,
		Result:   runResult,
		Error:    err,
		Duration: time.Since(startTime),
	}

	if err != nil {
		if me.onCellError != nil {
			me.onCellError(cell, err)
		}
	} else {
		if me.onCellComplete != nil {
			me.onCellComplete(cell, runResult)
		}

		// Create bundle if execution succeeded
		builder := artifact.NewBundleBuilder(scenario, me.outputDir)
		builder.SetRunResult(runResult.Status)
		// Bundle will be built separately if needed
		_ = builder
	}

	return cellResult
}

func (me *MatrixExecutor) generateCells() []MatrixCell {
	var cells []MatrixCell

	for _, backend := range me.config.Backends {
		for _, platform := range me.config.Platforms {
			for _, theme := range me.config.Themes {
				for _, density := range me.config.Densities {
					cells = append(cells, MatrixCell{
						Backend:  backend,
						Platform: platform,
						Theme:    theme,
						Density:  density,
					})
				}
			}
		}
	}

	return cells
}

func (me *MatrixExecutor) calculateSummary(cells []CellResult) MatrixSummary {
	summary := MatrixSummary{
		TotalCells: len(cells),
	}

	for _, cell := range cells {
		if cell.Error != nil {
			summary.ErrorCells++
		} else if cell.Result != nil && cell.Result.Status == model.StatusPassed {
			summary.PassedCells++
			summary.TotalDuration += cell.Duration
		} else {
			summary.FailedCells++
			summary.TotalDuration += cell.Duration
		}
	}

	return summary
}

// ScenarioSubset defines a subset of scenarios to run.
type ScenarioSubset struct {
	Name        string
	Description string
	Filter      ScenarioFilter
	Scenarios   []model.ScenarioID
}

// ScenarioFilter filters scenarios based on criteria.
type ScenarioFilter func(scenario *model.Scenario) bool

// FilterByFamily filters scenarios by family.
func FilterByFamily(family string) ScenarioFilter {
	return func(scenario *model.Scenario) bool {
		// TODO: Add family field to scenario metadata
		return true
	}
}

// FilterByCapability filters scenarios by required capability.
func FilterByCapability(capability string) ScenarioFilter {
	return func(scenario *model.Scenario) bool {
		for _, cap := range scenario.Capabilities {
			if string(cap) == capability {
				return true
			}
		}
		return false
	}
}

// FilterByTag filters scenarios by tag.
func FilterByTag(tag string) ScenarioFilter {
	return func(scenario *model.Scenario) bool {
		for _, t := range scenario.Tags {
			if t == tag {
				return true
			}
		}
		return false
	}
}

// RegistrySubset creates a subset from the registry using a filter.
func RegistrySubset(registry *store.ScenarioRegistry, filter ScenarioFilter) []model.ScenarioID {
	var result []model.ScenarioID

	for _, scenario := range registry.All() {
		if filter(scenario) {
			result = append(result, scenario.ID)
		}
	}

	return result
}

// PortabilityReport summarizes portability test results.
type PortabilityReport struct {
	GeneratedAt time.Time
	Matrix      *MatrixConfig
	Results     []CellResult
	Differences []PortabilityDifference
	Summary     PortabilitySummary
}

// PortabilityDifference records a portability difference.
type PortabilityDifference struct {
	Scenario   model.ScenarioID
	CellA      MatrixCell
	CellB      MatrixCell
	Difference string
	Severity   string
}

// PortabilitySummary provides a high-level view of portability.
type PortabilitySummary struct {
	TotalScenarios    int
	PortableScenarios int
	IssueCount        int
}

// GeneratePortabilityReport creates a portability report from matrix results.
func GeneratePortabilityReport(matrixResult *MatrixResult) *PortabilityReport {
	report := &PortabilityReport{
		GeneratedAt: time.Now(),
		Matrix:      matrixResult.Config,
		Results:     matrixResult.Cells,
		Differences: make([]PortabilityDifference, 0),
	}

	// Group results by scenario
	byScenario := make(map[model.ScenarioID][]CellResult)
	for _, cell := range matrixResult.Cells {
		byScenario[cell.Scenario] = append(byScenario[cell.Scenario], cell)
	}

	// Analyze each scenario for portability differences
	for scenarioID, cells := range byScenario {
		report.Summary.TotalScenarios++

		// Check if all cells passed
		allPassed := true
		for _, cell := range cells {
			if cell.Error != nil || (cell.Result != nil && cell.Result.Status != model.StatusPassed) {
				allPassed = false
				break
			}
		}

		if allPassed {
			report.Summary.PortableScenarios++
		}

		// Find differences between cells
		for i := 0; i < len(cells); i++ {
			for j := i + 1; j < len(cells); j++ {
				a := cells[i]
				b := cells[j]

				// Compare results
				if (a.Error != nil) != (b.Error != nil) {
					report.Differences = append(report.Differences, PortabilityDifference{
						Scenario:   scenarioID,
						CellA:      a.Cell,
						CellB:      b.Cell,
						Difference: "error mismatch",
						Severity:   "error",
					})
					report.Summary.IssueCount++
				}

				if a.Result != nil && b.Result != nil {
					if a.Result.Status != b.Result.Status {
						report.Differences = append(report.Differences, PortabilityDifference{
							Scenario:   scenarioID,
							CellA:      a.Cell,
							CellB:      b.Cell,
							Difference: "status mismatch",
							Severity:   "error",
						})
						report.Summary.IssueCount++
					}
				}
			}
		}
	}

	return report
}

// BaselineManager manages stable baselines per backend/platform combination.
type BaselineManager struct {
	baselines map[string]*Baseline // key: cell.String()
	mu        sync.RWMutex
}

// Baseline represents a known-good baseline for a matrix cell.
type Baseline struct {
	Cell        MatrixCell
	Scenario    model.ScenarioID
	Fingerprint RunFingerprint
	BundlePath  string
	CreatedAt   time.Time
	Version     string
}

// NewBaselineManager creates a new baseline manager.
func NewBaselineManager() *BaselineManager {
	return &BaselineManager{
		baselines: make(map[string]*Baseline),
	}
}

// RecordBaseline records a baseline for a matrix cell.
func (bm *BaselineManager) RecordBaseline(cell MatrixCell, scenario model.ScenarioID, fingerprint RunFingerprint, bundlePath string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	key := cell.String() + "_" + string(scenario)
	bm.baselines[key] = &Baseline{
		Cell:        cell,
		Scenario:    scenario,
		Fingerprint: fingerprint,
		BundlePath:  bundlePath,
		CreatedAt:   time.Now(),
		Version:     "1.0",
	}
}

// GetBaseline retrieves a baseline for a matrix cell.
func (bm *BaselineManager) GetBaseline(cell MatrixCell, scenario model.ScenarioID) (*Baseline, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	key := cell.String() + "_" + string(scenario)
	baseline, ok := bm.baselines[key]
	return baseline, ok
}

// CompareToBaseline compares a run result to its baseline.
func (bm *BaselineManager) CompareToBaseline(cell MatrixCell, scenario model.ScenarioID, result *model.RunResult) (*DriftReport, bool) {
	baseline, ok := bm.GetBaseline(cell, scenario)
	if !ok {
		return nil, false
	}

	// Compare current result to baseline fingerprint
	detector := NewDriftDetector()
	report := detector.CompareRuns(baseline.Fingerprint, RunFingerprint{
		FrameCount: result.StepsExecuted * 10, // Placeholder
		StepCount:  result.StepsExecuted,
		StartTime:  result.StartTime,
		EndTime:    result.EndTime,
	})

	return report, true
}

// SaveBaselines saves all baselines to disk.
func (bm *BaselineManager) SaveBaselines(outputDir string) error {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	for key := range bm.baselines {
		_ = filepath.Join(outputDir, key+".baseline.json")
		// TODO: Implement JSON serialization
	}

	return nil
}

// ArtifactComparisonPolicy defines how artifacts should be compared.
type ArtifactComparisonPolicy struct {
	Cell              MatrixCell
	IgnoredFields     []string
	TolerancePercent  float64
	CompareContent    bool
	CompareTimestamps bool
}

// NewArtifactComparisonPolicy creates a default policy for a matrix cell.
func NewArtifactComparisonPolicy(cell MatrixCell) *ArtifactComparisonPolicy {
	return &ArtifactComparisonPolicy{
		Cell:              cell,
		IgnoredFields:     []string{"timestamp", "created_at"},
		TolerancePercent:  5.0,
		CompareContent:    true,
		CompareTimestamps: false,
	}
}

// CompareArtifacts compares two artifacts using the policy.
func (acp *ArtifactComparisonPolicy) CompareArtifacts(a, b []byte) []Difference {
	normalizer := NewEnvironmentNormalizer()

	// Normalize artifacts before comparison
	normalizedA, _ := normalizer.NormalizeArtifact("", a)
	normalizedB, _ := normalizer.NormalizeArtifact("", b)

	return CompareArtifacts(normalizedA, normalizedB)
}
