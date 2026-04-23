package engine

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
)

func TestDriftDetector_CaptureFingerprint(t *testing.T) {
	detector := NewDriftDetector()
	runner := NewRunner(&runtime.Runtime{}, nil)
	runner.scenario = &model.Scenario{ID: "test.scenario"}

	t.Run("captures fingerprint", func(t *testing.T) {
		result := &model.RunResult{
			Status:        model.StatusPassed,
			StepsExecuted: 5,
			StartTime:     time.Now(),
			EndTime:       time.Now().Add(1 * time.Second),
		}

		fp := detector.CaptureFingerprint(runner, result)

		if fp.RunID == "" {
			t.Error("RunID should not be empty")
		}
		if fp.ScenarioID != "test.scenario" {
			t.Errorf("ScenarioID = %v, want test.scenario", fp.ScenarioID)
		}
		if fp.StepCount != 5 {
			t.Errorf("StepCount = %d, want 5", fp.StepCount)
		}
	})
}

func TestDriftDetector_CompareRuns(t *testing.T) {
	detector := NewDriftDetector()

	t.Run("detects no drift for identical runs", func(t *testing.T) {
		now := time.Now()
		fp1 := RunFingerprint{
			RunID:      "run1",
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}
		fp2 := RunFingerprint{
			RunID:      "run2",
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}

		report := detector.CompareRuns(fp1, fp2)
		if report.Detected {
			t.Error("Should not detect drift for identical runs")
		}
	})

	t.Run("detects frame count drift", func(t *testing.T) {
		now := time.Now()
		fp1 := RunFingerprint{
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}
		fp2 := RunFingerprint{
			FrameCount: 105,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}

		report := detector.CompareRuns(fp1, fp2)
		if !report.Detected {
			t.Error("Should detect frame count drift")
		}
		if report.DriftType != DriftState {
			t.Errorf("DriftType = %v, want state", report.DriftType)
		}
	})

	t.Run("detects step count drift", func(t *testing.T) {
		now := time.Now()
		fp1 := RunFingerprint{
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}
		fp2 := RunFingerprint{
			FrameCount: 100,
			StepCount:  4,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}

		report := detector.CompareRuns(fp1, fp2)
		if !report.Detected {
			t.Error("Should detect step count drift")
		}
	})

	t.Run("detects timing drift", func(t *testing.T) {
		now := time.Now()
		fp1 := RunFingerprint{
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}
		fp2 := RunFingerprint{
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(2 * time.Second), // 2x duration
		}

		report := detector.CompareRuns(fp1, fp2)
		if !report.Detected {
			t.Error("Should detect timing drift with >5% difference")
		}
		if report.DriftType != DriftTiming {
			t.Errorf("DriftType = %v, want timing", report.DriftType)
		}
	})

	t.Run("tolerance prevents false positives", func(t *testing.T) {
		detector.SetTolerance(0.20) // 20% tolerance
		now := time.Now()
		fp1 := RunFingerprint{
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1 * time.Second),
		}
		fp2 := RunFingerprint{
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(1100 * time.Millisecond), // 10% difference, within 20% tolerance
		}

		report := detector.CompareRuns(fp1, fp2)
		if report.Detected {
			t.Error("Should not detect drift within tolerance")
		}
	})
}

func TestDriftClassifier_Classify(t *testing.T) {
	classifier := &DriftClassifier{}

	tests := []struct {
		name     string
		report   DriftReport
		expected string
	}{
		{
			name:     "stable",
			report:   DriftReport{Detected: false},
			expected: "stable",
		},
		{
			name:     "timing drift",
			report:   DriftReport{Detected: true, DriftType: DriftTiming, Severity: "warning"},
			expected: "timing_drift:warning",
		},
		{
			name:     "state drift",
			report:   DriftReport{Detected: true, DriftType: DriftState, Severity: "error"},
			expected: "state_drift:error",
		},
		{
			name:     "environment drift",
			report:   DriftReport{Detected: true, DriftType: DriftEnvironment, Severity: "warning"},
			expected: "environment_drift:warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.Classify(&tt.report)
			if got != tt.expected {
				t.Errorf("Classify() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDriftDetector_StabilityReport(t *testing.T) {
	detector := NewDriftDetector()
	now := time.Now()

	// Create multiple fingerprints
	for i := 0; i < 5; i++ {
		fp := RunFingerprint{
			RunID:      "run" + string(rune('0'+i)),
			FrameCount: 100,
			StepCount:  5,
			StartTime:  now,
			EndTime:    now.Add(time.Duration(1+i) * time.Second),
		}
		detector.fingerprints = append(detector.fingerprints, fp)
	}

	report := detector.GenerateStabilityReport()

	if report.TotalRuns != 5 {
		t.Errorf("TotalRuns = %d, want 5", report.TotalRuns)
	}
	if report.AverageDuration == 0 {
		t.Error("AverageDuration should not be zero")
	}
}

func TestCompareArtifacts(t *testing.T) {
	t.Run("identical artifacts have no differences", func(t *testing.T) {
		a := map[string]string{"key": "value"}
		b := map[string]string{"key": "value"}

		diffs := CompareArtifacts(a, b)
		if len(diffs) != 0 {
			t.Errorf("Expected 0 differences, got %d", len(diffs))
		}
	})

	t.Run("different content has differences", func(t *testing.T) {
		a := map[string]string{"key": "value1"}
		b := map[string]string{"key": "value2"}

		diffs := CompareArtifacts(a, b)
		if len(diffs) == 0 {
			t.Error("Expected differences for different content")
		}
	})
}

func TestDriftDetector_DetectDrift(t *testing.T) {
	detector := NewDriftDetector()
	runner := NewRunner(&runtime.Runtime{}, nil)

	scenario := &model.Scenario{
		ID:          "test.drift",
		DisplayName: "Drift Test",
		Schema:      "1.0",
		Actions: []model.Action{
			{Type: model.ActionWaitFrames, Params: map[string]interface{}{"frames": 1.0}},
		},
	}

	t.Run("detects drift with multiple runs", func(t *testing.T) {
		report, err := detector.DetectDrift(runner, scenario, 3)
		if err != nil {
			t.Errorf("DetectDrift() error = %v", err)
		}
		if report == nil {
			t.Fatal("DetectDrift() returned nil report")
		}
	})

	t.Run("error with less than 2 runs", func(t *testing.T) {
		_, err := detector.DetectDrift(runner, scenario, 1)
		if err == nil {
			t.Error("DetectDrift() should error with < 2 runs")
		}
	})
}
