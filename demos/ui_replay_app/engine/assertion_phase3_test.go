package engine

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

func TestRunner_RunEvaluatesAssertions(t *testing.T) {
	defer store.EnvironmentStore.Set(store.DefaultEnvironment())
	defer store.ExecutionStateStore.Set(store.ExecutionState{})
	store.RunHistoryStore.Set(store.NewRunHistory())

	runner := NewRunner(&runtime.Runtime{}, nil)
	scenario := &model.Scenario{
		ID:          "test.assertions.pass",
		DisplayName: "Assertions Pass",
		Schema:      "1.0",
		Actions: []model.Action{
			{Type: model.ActionSceneLoad, Params: model.ActionParams{"scene": "basic"}},
			{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 1}},
		},
		Assertions: []model.Assertion{
			{Type: model.AssertSceneID, Params: model.AssertionParams{"expected": "basic"}},
			{Type: model.AssertThemeState, Params: model.AssertionParams{"expected": "baseline"}},
			{Type: model.AssertDensityState, Params: model.AssertionParams{"expected": "default"}},
			{Type: model.AssertFrameCount, Params: model.AssertionParams{"min": 1.0, "max": 1.0}},
		},
	}

	result, err := runner.Run(scenario)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.StatusPassed {
		t.Fatalf("Status = %v, want %v", result.Status, model.StatusPassed)
	}
	if len(result.AssertionResults) != 4 {
		t.Fatalf("AssertionResults len = %d, want 4", len(result.AssertionResults))
	}

	exec := store.ExecutionStateStore.Get()
	if len(exec.AssertionResults) != 4 {
		t.Fatalf("Execution store assertion results len = %d, want 4", len(exec.AssertionResults))
	}
	if exec.AssertionFailures() != 0 {
		t.Fatalf("Execution store assertion failures = %d, want 0", exec.AssertionFailures())
	}

	history := store.RunHistoryStore.Get()
	if history == nil || history.Count() == 0 {
		t.Fatal("expected run history to record the completed run")
	}
}

func TestRunner_RunFailsWhenAssertionViolatesState(t *testing.T) {
	defer store.EnvironmentStore.Set(store.DefaultEnvironment())
	defer store.ExecutionStateStore.Set(store.ExecutionState{})
	store.RunHistoryStore.Set(store.NewRunHistory())

	runner := NewRunner(&runtime.Runtime{}, nil)
	scenario := &model.Scenario{
		ID:          "test.assertions.fail",
		DisplayName: "Assertions Fail",
		Schema:      "1.0",
		Assertions: []model.Assertion{
			{Type: model.AssertFocusOwner, Params: model.AssertionParams{"expected": "42"}},
		},
	}

	result, err := runner.Run(scenario)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.StatusFailed {
		t.Fatalf("Status = %v, want %v", result.Status, model.StatusFailed)
	}
	if len(result.AssertionResults) != 1 {
		t.Fatalf("AssertionResults len = %d, want 1", len(result.AssertionResults))
	}
	if result.AssertionResults[0].Passed {
		t.Fatal("expected the assertion to fail")
	}
	if result.Error == "" {
		t.Fatal("expected a failure error message")
	}
}

func TestAssertionEngine_ObservationAssertions(t *testing.T) {
	defer store.EnvironmentStore.Set(store.DefaultEnvironment())
	defer store.ExecutionStateStore.Set(store.ExecutionState{})
	defer store.RunHistoryStore.Set(store.NewRunHistory())

	runner := NewRunner(&runtime.Runtime{}, nil)
	runner.logger.EventCaptured("click", "button.ok", map[string]interface{}{"step": 1})
	runner.logger.SummaryCaptured("screenshot", map[string]interface{}{"name": "main"})
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:        model.StatusRunning,
		CurrentStep:   2,
		TotalSteps:    5,
		CurrentAction: "assertions",
		Progress:      0.4,
	})

	engine := runner.NewAssertionEngine()

	event := engine.EvaluateSingle(1, model.Assertion{
		Type: model.AssertEventPresent,
		Params: model.AssertionParams{
			"event_type": "click",
		},
	})
	if !event.Passed {
		t.Fatalf("event_present should have passed: %s", event.Message)
	}

	screenshot := engine.EvaluateSingle(2, model.Assertion{
		Type: model.AssertScreenshot,
		Params: model.AssertionParams{
			"name": "main",
		},
	})
	if !screenshot.Passed {
		t.Fatalf("screenshot assertion should have passed: %s", screenshot.Message)
	}

	execSummary := engine.EvaluateSingle(3, model.Assertion{
		Type: model.AssertStoreSummary,
		Params: model.AssertionParams{
			"store_id": "execution",
			"expected": "status=running step=2/5 progress=40% action=assertions assertions=0 failures=0",
		},
	})
	if !execSummary.Passed {
		t.Fatalf("store summary assertion should have passed: %s", execSummary.Message)
	}

	diagnostics := engine.EvaluateSingle(4, model.Assertion{
		Type: model.AssertDiagnostics,
		Params: model.AssertionParams{
			"level":     "debug",
			"max_count": 2.0,
		},
	})
	if !diagnostics.Passed {
		t.Fatalf("diagnostics assertion should have passed: %s", diagnostics.Message)
	}
}
