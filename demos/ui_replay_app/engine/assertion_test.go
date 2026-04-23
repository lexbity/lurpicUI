package engine

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
)

func TestAssertionEngine_Evaluate(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	engine := runner.NewAssertionEngine()

	t.Run("empty scenario returns empty results", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.empty",
			DisplayName: "Empty Test",
			Schema:      "1.0",
		}

		results, err := engine.Evaluate(scenario)
		if err != nil {
			t.Errorf("Evaluate() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results, got %d", len(results))
		}
	})

	t.Run("scene_id assertion evaluates", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.scene",
			DisplayName: "Scene Test",
			Schema:      "1.0",
			Assertions: []model.Assertion{
				{Type: model.AssertSceneID, Params: map[string]interface{}{"expected": "basic"}},
			},
		}

		results, _ := engine.Evaluate(scenario)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		// Should fail since runtime returns "unknown"
		if results[0].Passed {
			t.Error("Expected assertion to fail (placeholder returns unknown)")
		}
		if results[0].Type != string(model.AssertSceneID) {
			t.Errorf("Type = %v, want %v", results[0].Type, model.AssertSceneID)
		}
	})

	t.Run("frame_count assertion evaluates", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.frame",
			DisplayName: "Frame Test",
			Schema:      "1.0",
			Assertions: []model.Assertion{
				{Type: model.AssertFrameCount, Params: map[string]interface{}{"min": 0.0, "max": 100.0}},
			},
		}

		// Set frame counter
		runner.frameCounter = 50

		results, _ := engine.Evaluate(scenario)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		if !results[0].Passed {
			t.Errorf("Expected assertion to pass, got: %s", results[0].Message)
		}
	})

	t.Run("missing params fails assertion", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.missing",
			DisplayName: "Missing Test",
			Schema:      "1.0",
			Assertions: []model.Assertion{
				{Type: model.AssertSceneID, Params: map[string]interface{}{}}, // missing expected
			},
		}

		results, _ := engine.Evaluate(scenario)
		if results[0].Passed {
			t.Error("Expected assertion to fail with missing params")
		}
		if results[0].Message == "" {
			t.Error("Expected error message for missing params")
		}
	})

	t.Run("unknown assertion type fails", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.unknown",
			DisplayName: "Unknown Test",
			Schema:      "1.0",
			Assertions: []model.Assertion{
				{Type: "unknown_type", Params: map[string]interface{}{}},
			},
		}

		results, _ := engine.Evaluate(scenario)
		if results[0].Passed {
			t.Error("Expected assertion to fail with unknown type")
		}
	})
}

func TestAssertionEngine_AllPassed(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)

	t.Run("empty results returns true", func(t *testing.T) {
		engine := runner.NewAssertionEngine()
		if !engine.AllPassed() {
			t.Error("AllPassed() should return true for empty results")
		}
	})

	t.Run("all passed returns true", func(t *testing.T) {
		engine := runner.NewAssertionEngine()
		engine.results = []AssertionResult{
			{Passed: true},
			{Passed: true},
		}
		if !engine.AllPassed() {
			t.Error("AllPassed() should return true when all passed")
		}
	})

	t.Run("one failed returns false", func(t *testing.T) {
		engine := runner.NewAssertionEngine()
		engine.results = []AssertionResult{
			{Passed: true},
			{Passed: false},
		}
		if engine.AllPassed() {
			t.Error("AllPassed() should return false when any failed")
		}
	})
}

func TestAssertionEngine_Failed(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	engine := runner.NewAssertionEngine()

	t.Run("returns only failed assertions", func(t *testing.T) {
		engine.results = []AssertionResult{
			{Passed: true, Message: "passed"},
			{Passed: false, Message: "failed 1"},
			{Passed: true, Message: "passed"},
			{Passed: false, Message: "failed 2"},
		}

		failed := engine.Failed()
		if len(failed) != 2 {
			t.Errorf("Expected 2 failed, got %d", len(failed))
		}
		if failed[0].Message != "failed 1" || failed[1].Message != "failed 2" {
			t.Error("Failed() returned wrong assertions")
		}
	})
}

func TestAssertionEngine_Summary(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	engine := runner.NewAssertionEngine()

	t.Run("summary counts correctly", func(t *testing.T) {
		engine.results = []AssertionResult{
			{Passed: true},
			{Passed: true},
			{Passed: false, Message: "error 1"},
			{Passed: false, Message: "error 2"},
		}

		summary := engine.Summary()
		if summary.Total != 4 {
			t.Errorf("Total = %d, want 4", summary.Total)
		}
		if summary.Passed != 2 {
			t.Errorf("Passed = %d, want 2", summary.Passed)
		}
		if summary.Failed != 2 {
			t.Errorf("Failed = %d, want 2", summary.Failed)
		}
		if len(summary.Failures) != 2 {
			t.Errorf("Failures length = %d, want 2", len(summary.Failures))
		}
	})

	t.Run("summary string all passed", func(t *testing.T) {
		engine := runner.NewAssertionEngine()
		engine.results = []AssertionResult{{Passed: true}}

		summary := engine.Summary()
		str := summary.String()
		if str != "All 1 assertions passed" {
			t.Errorf("String() = %q", str)
		}
	})

	t.Run("summary string with failures", func(t *testing.T) {
		engine := runner.NewAssertionEngine()
		engine.results = []AssertionResult{
			{Passed: true},
			{Passed: false},
		}

		summary := engine.Summary()
		str := summary.String()
		if str != "1/2 assertions failed" {
			t.Errorf("String() = %q", str)
		}
	})
}

func TestAssertionResult_Error(t *testing.T) {
	t.Run("passed returns empty string", func(t *testing.T) {
		r := AssertionResult{Passed: true, Step: 1}
		if r.Error() != "" {
			t.Error("Error() should return empty for passed result")
		}
	})

	t.Run("failed returns error message", func(t *testing.T) {
		r := AssertionResult{Passed: false, Step: 5, Message: "expected foo, got bar"}
		err := r.Error()
		if err != "step 5: expected foo, got bar" {
			t.Errorf("Error() = %q", err)
		}
	})
}
