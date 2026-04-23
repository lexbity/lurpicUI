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
	DriftState       DriftType = "state"
	DriftEnvironment DriftType = "environment"
	DriftArtifact    DriftType = "artifact"
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
	RunID         string
	ScenarioID    string
	StartTime     time.Time
	EndTime       time.Time
	FrameCount    int
	StepCount     int
	AssertionHash string
	LogHash       string
	StateHash     string
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

// SetTolerance sets the tolerance for timing differences.
func (dd *DriftDetector) SetTolerance(tolerance float64) {
	dd.tolerance = tolerance
}

// CaptureFingerprint creates a fingerprint from a completed run.
func (dd *DriftDetector) CaptureFingerprint(runner *Runner, result *model.RunResult) RunFingerprint {
	fp := RunFingerprint{
		RunID:      generateRunID(),
		ScenarioID: string(runner.scenario.ID),
		StartTime:  result.StartTime,
		EndTime:    result.EndTime,
		FrameCount: runner.frameCounter,
		StepCount:  result.StepsExecuted,
	}

	// Hash assertion results if available
	if runner.logger != nil {
		fp.LogHash = hashEntries(runner.logger.Entries())
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
		report.DriftType = DriftTiming
		report.Severity = "warning"
		report.Description = fmt.Sprintf("Timing drift detected: %v vs %v", durationA, durationB)
		report.Differences = append(report.Differences, Difference{
			Field:    "duration",
			ValueA:   durationA,
			ValueB:   durationB,
			Severity: "warning",
		})
	}

	// Check frame count drift
	if runA.FrameCount != runB.FrameCount {
		report.Detected = true
		report.DriftType = DriftState
		report.Severity = "error"
		report.Description = fmt.Sprintf("Frame count mismatch: %d vs %d", runA.FrameCount, runB.FrameCount)
		report.Differences = append(report.Differences, Difference{
			Field:    "frame_count",
			ValueA:   runA.FrameCount,
			ValueB:   runB.FrameCount,
			Severity: "error",
		})
	}

	// Check step count drift
	if runA.StepCount != runB.StepCount {
		report.Detected = true
		report.DriftType = DriftState
		report.Severity = "error"
		report.Description = fmt.Sprintf("Step count mismatch: %d vs %d", runA.StepCount, runB.StepCount)
		report.Differences = append(report.Differences, Difference{
			Field:    "step_count",
			ValueA:   runA.StepCount,
			ValueB:   runB.StepCount,
			Severity: "error",
		})
	}

	// Check log hash drift
	if runA.LogHash != "" && runB.LogHash != "" && runA.LogHash != runB.LogHash {
		report.Detected = true
		if report.DriftType == "" {
			report.DriftType = DriftEnvironment
		}
		report.Differences = append(report.Differences, Difference{
			Field:    "log_hash",
			ValueA:   runA.LogHash[:16] + "...",
			ValueB:   runB.LogHash[:16] + "...",
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
		result, err := runner.Run(scenario)
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
	case DriftState:
		return fmt.Sprintf("state_drift:%s", report.Severity)
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
