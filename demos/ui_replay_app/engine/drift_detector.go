package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"codeburg.org/lexbit/ui_replay/model"
)

// DriftType classifies the source of nondeterminism.
type DriftType string

const (
	DriftTiming      DriftType = "timing"
	DriftExecution   DriftType = "execution"
	DriftAssertion   DriftType = "assertion"
	DriftEnvironment DriftType = "environment"
	DriftArtifact    DriftType = "artifact"
	DriftState       DriftType = "state"
)

// DriftReport captures detected nondeterminism between runs.
type DriftReport struct {
	Detected    bool
	DriftType   DriftType
	Severity    string // "warning", "error", "critical"
	Description string
	Differences []Difference
	RunA        RunFingerprint
	RunB        RunFingerprint
	DetectedAt  time.Time
}

// RunFingerprint captures identifying characteristics of a run.
type RunFingerprint struct {
	RunID           string
	ScenarioID      string
	StartTime       time.Time
	EndTime         time.Time
	FrameCount      int
	StepCount       int
	AssertionHash   string
	ArtifactHash    string
	EnvironmentHash string
	LogHash         string
	ExecutionHash   string
	StateHash       string
}

// DriftDetector detects and classifies nondeterminism.
type DriftDetector struct {
	fingerprints []RunFingerprint
	tolerance    float64 // tolerance for timing differences (0.0-1.0)
}

// NewDriftDetector creates a new drift detector.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{
		fingerprints: make([]RunFingerprint, 0),
		tolerance:    0.05, // 5% tolerance for timing
	}
}

func shortHash(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:16] + "..."
}

// SetTolerance sets the tolerance for timing differences.
func (dd *DriftDetector) SetTolerance(tolerance float64) {
	dd.tolerance = tolerance
}

// CaptureFingerprint creates a fingerprint from a completed run.
func (dd *DriftDetector) CaptureFingerprint(runner *Runner, result *model.RunResult) RunFingerprint {
	fp := RunFingerprint{
		RunID:     generateRunID(),
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		StepCount: result.StepsExecuted,
	}

	if runner != nil {
		fp.FrameCount = runner.frameCounter
	}

	if runner != nil && runner.scenario != nil {
		fp.ScenarioID = string(runner.scenario.ID)
		fp.EnvironmentHash = hashValue(runner.scenario.Environment)
	}

	fp.AssertionHash = hashValue(result.AssertionResults)
	fp.ArtifactHash = hashValue(result.Artifacts)
	fp.ExecutionHash = hashValue(struct {
		Status        model.ExecutionStatus
		Error         string
		StepsExecuted int
		StepsTotal    int
		FrameCount    int
		LogHash       string
	}{
		Status:        result.Status,
		Error:         result.Error,
		StepsExecuted: result.StepsExecuted,
		StepsTotal:    result.StepsTotal,
		FrameCount:    fp.FrameCount,
		LogHash:       "",
	})
	fp.StateHash = fp.ExecutionHash

	if runner != nil && runner.logger != nil {
		fp.LogHash = hashEntries(runner.logger.Entries())
		fp.ExecutionHash = hashValue(struct {
			Status        model.ExecutionStatus
			Error         string
			StepsExecuted int
			StepsTotal    int
			FrameCount    int
			LogHash       string
		}{
			Status:        result.Status,
			Error:         result.Error,
			StepsExecuted: result.StepsExecuted,
			StepsTotal:    result.StepsTotal,
			FrameCount:    fp.FrameCount,
			LogHash:       fp.LogHash,
		})
		fp.StateHash = fp.ExecutionHash
	}

	dd.fingerprints = append(dd.fingerprints, fp)
	return fp
}

// CompareRuns compares two runs and detects drift.
func (dd *DriftDetector) CompareRuns(runA, runB RunFingerprint) *DriftReport {
	report := &DriftReport{
		RunA:        runA,
		RunB:        runB,
		DetectedAt:  time.Now(),
		Differences: make([]Difference, 0),
	}

	// Check timing drift
	durationA := runA.EndTime.Sub(runA.StartTime)
	durationB := runB.EndTime.Sub(runB.StartTime)

	if dd.isTimingDrift(durationA, durationB) {
		report.Detected = true
		if report.DriftType == "" {
			report.DriftType = DriftTiming
		}
		report.Severity = "warning"
		report.Description = fmt.Sprintf("Timing drift detected: %v vs %v", durationA, durationB)
		report.Differences = append(report.Differences, Difference{
			Field:    "duration",
			ValueA:   durationA,
			ValueB:   durationB,
			Severity: "warning",
		})
	}

	// Check environment drift.
	if runA.EnvironmentHash != "" && runB.EnvironmentHash != "" && runA.EnvironmentHash != runB.EnvironmentHash {
		report.Detected = true
		report.DriftType = DriftEnvironment
		report.Severity = "error"
		report.Description = "Environment mismatch detected"
		report.Differences = append(report.Differences, Difference{
			Field:    "environment",
			ValueA:   shortHash(runA.EnvironmentHash),
			ValueB:   shortHash(runB.EnvironmentHash),
			Severity: "error",
		})
	}

	// Check semantic assertion drift.
	if runA.AssertionHash != "" && runB.AssertionHash != "" && runA.AssertionHash != runB.AssertionHash {
		report.Detected = true
		if report.DriftType == "" || report.DriftType == DriftTiming {
			report.DriftType = DriftAssertion
		}
		report.Severity = "error"
		report.Description = "Assertion output mismatch detected"
		report.Differences = append(report.Differences, Difference{
			Field:    "assertions",
			ValueA:   shortHash(runA.AssertionHash),
			ValueB:   shortHash(runB.AssertionHash),
			Severity: "error",
		})
	}

	// Check artifact drift.
	if runA.ArtifactHash != "" && runB.ArtifactHash != "" && runA.ArtifactHash != runB.ArtifactHash {
		report.Detected = true
		if report.DriftType == "" || report.DriftType == DriftTiming {
			report.DriftType = DriftArtifact
		}
		report.Severity = "error"
		report.Description = "Artifact mismatch detected"
		report.Differences = append(report.Differences, Difference{
			Field:    "artifacts",
			ValueA:   shortHash(runA.ArtifactHash),
			ValueB:   shortHash(runB.ArtifactHash),
			Severity: "error",
		})
	}

	// Check execution drift.
	if runA.ExecutionHash != "" && runB.ExecutionHash != "" && runA.ExecutionHash != runB.ExecutionHash {
		report.Detected = true
		if report.DriftType == "" || report.DriftType == DriftTiming {
			report.DriftType = DriftExecution
		}
		report.Severity = "error"
		report.Description = fmt.Sprintf("Execution mismatch: frame %d vs %d, step %d vs %d", runA.FrameCount, runB.FrameCount, runA.StepCount, runB.StepCount)
		report.Differences = append(report.Differences, Difference{
			Field:    "execution",
			ValueA:   shortHash(runA.ExecutionHash),
			ValueB:   shortHash(runB.ExecutionHash),
			Severity: "error",
		})
	}

	// Frame and step count mismatches are tracked as execution drift details.
	if runA.FrameCount != runB.FrameCount {
		report.Detected = true
		if report.DriftType == "" || report.DriftType == DriftTiming {
			report.DriftType = DriftExecution
		}
		report.Severity = "error"
		report.Description = fmt.Sprintf("Frame count mismatch: %d vs %d", runA.FrameCount, runB.FrameCount)
		report.Differences = append(report.Differences, Difference{
			Field:    "frame_count",
			ValueA:   runA.FrameCount,
			ValueB:   runB.FrameCount,
			Severity: "error",
		})
	}

	if runA.StepCount != runB.StepCount {
		report.Detected = true
		if report.DriftType == "" || report.DriftType == DriftTiming {
			report.DriftType = DriftExecution
		}
		report.Severity = "error"
		report.Description = fmt.Sprintf("Step count mismatch: %d vs %d", runA.StepCount, runB.StepCount)
		report.Differences = append(report.Differences, Difference{
			Field:    "step_count",
			ValueA:   runA.StepCount,
			ValueB:   runB.StepCount,
			Severity: "error",
		})
	}

	// Check log hash drift last; it is a symptom of execution drift.
	if runA.LogHash != "" && runB.LogHash != "" && runA.LogHash != runB.LogHash {
		report.Detected = true
		if report.DriftType == "" || report.DriftType == DriftTiming {
			report.DriftType = DriftExecution
		}
		report.Differences = append(report.Differences, Difference{
			Field:    "log_hash",
			ValueA:   shortHash(runA.LogHash),
			ValueB:   shortHash(runB.LogHash),
			Severity: "warning",
		})
	}

	if !report.Detected {
		report.Description = "No drift detected between runs"
	}

	return report
}

// DetectDrift runs multiple times and detects any drift.
func (dd *DriftDetector) DetectDrift(runner *Runner, scenario *model.Scenario, runs int) (*DriftReport, error) {
	if runs < 2 {
		return nil, fmt.Errorf("need at least 2 runs to detect drift")
	}

	// Clear previous fingerprints
	dd.fingerprints = make([]RunFingerprint, 0)

	// Run multiple times and capture fingerprints
	for i := 0; i < runs; i++ {
		runScenario := scenario.Clone()
		if runScenario == nil {
			return nil, fmt.Errorf("run %d scenario clone failed", i)
		}
		result, err := runner.Run(runScenario)
		if err != nil {
			return nil, fmt.Errorf("run %d failed: %w", i, err)
		}
		dd.CaptureFingerprint(runner, result)
	}

	// Compare first and last run
	if len(dd.fingerprints) >= 2 {
		report := dd.CompareRuns(dd.fingerprints[0], dd.fingerprints[len(dd.fingerprints)-1])

		// Also compare consecutive runs for additional detail
		for i := 0; i < len(dd.fingerprints)-1; i++ {
			consecutiveReport := dd.CompareRuns(dd.fingerprints[i], dd.fingerprints[i+1])
			if consecutiveReport.Detected {
				// Merge differences
				report.Differences = append(report.Differences, consecutiveReport.Differences...)
			}
		}

		return report, nil
	}

	return &DriftReport{
		Detected:    false,
		Description: "Insufficient runs for drift detection",
	}, nil
}

// isTimingDrift checks if timing difference exceeds tolerance.
func (dd *DriftDetector) isTimingDrift(a, b time.Duration) bool {
	if a == 0 && b == 0 {
		return false
	}
	if a == 0 || b == 0 {
		return true
	}

	diff := a - b
	if diff < 0 {
		diff = -diff
	}

	avg := (a + b) / 2
	ratio := float64(diff) / float64(avg)

	return ratio > dd.tolerance
}

// DriftClassifier provides detailed drift classification.
type DriftClassifier struct{}

// Classify analyzes a drift report and provides classification.
func (dc *DriftClassifier) Classify(report *DriftReport) string {
	if !report.Detected {
		return "stable"
	}

	switch report.DriftType {
	case DriftTiming:
		return fmt.Sprintf("timing_drift:%s", report.Severity)
	case DriftExecution, DriftState:
		return fmt.Sprintf("execution_drift:%s", report.Severity)
	case DriftAssertion:
		return fmt.Sprintf("assertion_drift:%s", report.Severity)
	case DriftEnvironment:
		return fmt.Sprintf("environment_drift:%s", report.Severity)
	case DriftArtifact:
		return fmt.Sprintf("artifact_drift:%s", report.Severity)
	default:
		return fmt.Sprintf("unknown_drift:%s", report.Severity)
	}
}

// generateRunID creates a unique run identifier.
func generateRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}

// hashEntries creates a hash from log entries.
func hashEntries(entries interface{}) string {
	data, err := json.Marshal(entries)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// hashValue creates a hash from any value.
func hashValue(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CompareArtifacts compares two artifacts for differences.
func CompareArtifacts(a, b interface{}) []Difference {
	var diffs []Difference

	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return append(diffs, Difference{
			Field:    "type",
			ValueA:   reflect.TypeOf(a),
			ValueB:   reflect.TypeOf(b),
			Severity: "error",
		})
	}

	// Deep comparison for simple types
	if !reflect.DeepEqual(a, b) {
		diffs = append(diffs, Difference{
			Field:    "content",
			ValueA:   hashValue(a)[:16] + "...",
			ValueB:   hashValue(b)[:16] + "...",
			Severity: "warning",
		})
	}

	return diffs
}

// StabilityReport summarizes stability across multiple runs.
type StabilityReport struct {
	TotalRuns        int
	StableRuns       int
	DriftDetected    int
	DriftType        map[DriftType]int
	AverageDuration  time.Duration
	DurationVariance float64
}

// GenerateStabilityReport analyzes all captured fingerprints.
func (dd *DriftDetector) GenerateStabilityReport() *StabilityReport {
	report := &StabilityReport{
		TotalRuns:  len(dd.fingerprints),
		DriftType:  make(map[DriftType]int),
		StableRuns: len(dd.fingerprints),
	}

	if len(dd.fingerprints) < 2 {
		return report
	}

	// Calculate average duration
	var totalDuration time.Duration
	for _, fp := range dd.fingerprints {
		duration := fp.EndTime.Sub(fp.StartTime)
		totalDuration += duration
	}
	report.AverageDuration = totalDuration / time.Duration(len(dd.fingerprints))

	// Count drifts between consecutive runs
	for i := 0; i < len(dd.fingerprints)-1; i++ {
		drift := dd.CompareRuns(dd.fingerprints[i], dd.fingerprints[i+1])
		if drift.Detected {
			report.DriftDetected++
			report.StableRuns--
			report.DriftType[drift.DriftType]++
		}
	}

	return report
}
