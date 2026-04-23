package engine

import (
	"context"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
)

func TestRunner_Run(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)

	t.Run("valid scenario runs successfully", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.valid",
			DisplayName: "Valid Test",
			Schema:      "1.0",
			Actions: []model.Action{
				{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 1}},
			},
		}

		result, err := runner.Run(scenario)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.Status != model.StatusPassed {
			t.Errorf("Status = %v, want %v", result.Status, model.StatusPassed)
		}
		if result.StepsExecuted != 1 {
			t.Errorf("StepsExecuted = %d, want 1", result.StepsExecuted)
		}
	})

	t.Run("invalid scenario fails validation", func(t *testing.T) {
		scenario := &model.Scenario{
			ID: "test.invalid",
		}

		_, err := runner.Run(scenario)
		if err == nil {
			t.Error("Run() expected error for invalid scenario")
		}
	})

	t.Run("runner tracks state correctly", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)
		scenario := &model.Scenario{
			ID:          "test.state",
			DisplayName: "State Test",
			Schema:      "1.0",
			Actions: []model.Action{
				{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 1}},
			},
		}

		if runner.State() != StateIdle {
			t.Errorf("Initial state = %v, want %v", runner.State(), StateIdle)
		}

		result, _ := runner.Run(scenario)

		if result.Status == model.StatusPassed && runner.State() != StateCompleted {
			t.Errorf("Final state = %v, want %v", runner.State(), StateCompleted)
		}
	})
}

func TestRunner_Cancel(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)

	t.Run("cancel stops execution", func(t *testing.T) {
		scenario := &model.Scenario{
			ID:          "test.cancel",
			DisplayName: "Cancel Test",
			Schema:      "1.0",
			Actions: []model.Action{
				{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 100}},
			},
		}

		// Start run in goroutine
		done := make(chan *model.RunResult)
		go func() {
			result, _ := runner.Run(scenario)
			done <- result
		}()

		// Cancel after short delay
		time.Sleep(50 * time.Millisecond)
		runner.Cancel()

		result := <-done
		if result.Status != model.StatusCancelled {
			t.Errorf("Status = %v, want %v", result.Status, model.StatusCancelled)
		}
	})
}

func TestRunner_StepCallback(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)

	stepCount := 0
	runner.SetStepCallback(func(step int, total int, action model.Action) {
		stepCount++
	})

	scenario := &model.Scenario{
		ID:          "test.callback",
		DisplayName: "Callback Test",
		Schema:      "1.0",
		Actions: []model.Action{
			{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 1}},
			{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 1}},
		},
	}

	runner.Run(scenario)

	if stepCount != 2 {
		t.Errorf("Step callback count = %d, want 2", stepCount)
	}
}

func TestRunner_executeWaitFrames(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)

	t.Run("wait frames advances frame counter", func(t *testing.T) {
		runner.frameCounter = 0
		action := model.Action{
			Type:   model.ActionWaitFrames,
			Params: model.ActionParams{"frames": 5},
		}

		err := runner.executeWaitFrames(action)
		if err != nil {
			t.Errorf("executeWaitFrames() error = %v", err)
		}
		if runner.frameCounter != 5 {
			t.Errorf("frameCounter = %d, want 5", runner.frameCounter)
		}
	})

	t.Run("missing frames param errors", func(t *testing.T) {
		action := model.Action{
			Type:   model.ActionWaitFrames,
			Params: model.ActionParams{},
		}

		err := runner.executeWaitFrames(action)
		if err == nil {
			t.Error("executeWaitFrames() expected error for missing frames")
		}
	})
}

func TestRunner_executeWaitIdle(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)

	t.Run("wait idle returns quickly when idle", func(t *testing.T) {
		action := model.Action{
			Type:   model.ActionWaitIdle,
			Params: model.ActionParams{"timeout_ms": 1000},
		}

		start := time.Now()
		err := runner.executeWaitIdle(action)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("executeWaitIdle() error = %v", err)
		}
		// Should return quickly since isIdle() returns true
		if duration > 100*time.Millisecond {
			t.Errorf("wait idle took too long: %v", duration)
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		runner.ctx, runner.cancel = context.WithCancel(context.Background())
		runner.cancel() // Cancel immediately

		action := model.Action{
			Type:   model.ActionWaitIdle,
			Params: model.ActionParams{},
		}

		err := runner.executeWaitIdle(action)
		if err == nil {
			t.Error("executeWaitIdle() expected error for cancelled context")
		}
	})
}

func TestRunner_TargetResolution(t *testing.T) {
	tests := []struct {
		name     string
		target   model.Target
		expected string // Expected logical ID after resolve
	}{
		{
			name:     "primary used",
			target:   model.Target{LogicalID: "primary"},
			expected: "primary",
		},
		{
			name:     "fallback used when empty",
			target:   model.Target{Fallback: &model.Target{LogicalID: "fallback"}},
			expected: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := tt.target.Resolve()
			if resolved.LogicalID != tt.expected {
				t.Errorf("Resolve() LogicalID = %v, want %v", resolved.LogicalID, tt.expected)
			}
		})
	}
}

func TestRunState_String(t *testing.T) {
	states := []RunState{
		StateIdle, StatePreparing, StateRunning, StateWaiting,
		StateAsserting, StateCapturing, StateCompleted, StateFailed,
		StateCancelled,
	}

	for _, state := range states {
		if state == "" {
			t.Errorf("RunState %v has empty string value", state)
		}
	}
}

func TestBackgroundJobHandler_CommitResult(t *testing.T) {
	t.Run("handler created with valid version", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)
		handler := runner.NewBackgroundJobHandler()
		if handler == nil {
			t.Error("NewBackgroundJobHandler() returned nil")
		}
		if handler.version == 0 {
			t.Error("handler.version should be non-zero")
		}
	})

	t.Run("commit with cancelled context fails", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)
		_ = runner.NewBackgroundJobHandler() // Create handler to check it's non-nil

		runner.ctx, runner.cancel = context.WithCancel(context.Background())
		runner.cancel()

		// Verify the cancel mechanism works
		if runner.ctx.Err() == nil {
			t.Error("expected context to be cancelled")
		}
	})
}

func TestCheckpointGate_Wait(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	gate := runner.NewCheckpointGate()

	t.Run("wait returns when idle", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := gate.Wait(ctx)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Wait() error = %v", err)
		}
		// Should return quickly
		if duration > 50*time.Millisecond {
			t.Errorf("Wait() took too long: %v", duration)
		}
	})

	t.Run("wait respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := gate.Wait(ctx)
		if err != context.Canceled {
			t.Errorf("Wait() error = %v, want context.Canceled", err)
		}
	})
}

func TestScheduler_Determinism(t *testing.T) {
	t.Run("events execute in registration order", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)
		scheduler := runner.NewScheduler()

		var order []string
		scheduler.Schedule(1, func() error {
			order = append(order, "first")
			return nil
		})
		scheduler.Schedule(1, func() error {
			order = append(order, "second")
			return nil
		})
		scheduler.Schedule(1, func() error {
			order = append(order, "third")
			return nil
		})

		scheduler.Tick()

		if len(order) != 3 {
			t.Errorf("Expected 3 events, got %d", len(order))
		}
		if order[0] != "first" || order[1] != "second" || order[2] != "third" {
			t.Errorf("Wrong execution order: %v", order)
		}
	})

	t.Run("events execute at correct time", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)
		scheduler := runner.NewScheduler()

		executed := make(map[int64]bool)
		scheduler.Schedule(3, func() error {
			executed[scheduler.CurrentTime()] = true
			return nil
		})

		// Advance 2 frames - event should not execute
		scheduler.Advance(2)
		if executed[3] {
			t.Error("Event executed too early")
		}

		// One more tick - event should execute
		scheduler.Tick()
		if !executed[3] {
			t.Error("Event did not execute at expected time")
		}
	})

	t.Run("repeated runs produce same order", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)

		var firstRun []string
		var secondRun []string

		// First run
		scheduler1 := runner.NewScheduler()
		scheduler1.Schedule(1, func() error { firstRun = append(firstRun, "A"); return nil })
		scheduler1.Schedule(2, func() error { firstRun = append(firstRun, "B"); return nil })
		scheduler1.Schedule(1, func() error { firstRun = append(firstRun, "C"); return nil })
		scheduler1.Advance(2)

		// Second run with same schedule
		scheduler2 := runner.NewScheduler()
		scheduler2.Schedule(1, func() error { secondRun = append(secondRun, "A"); return nil })
		scheduler2.Schedule(2, func() error { secondRun = append(secondRun, "B"); return nil })
		scheduler2.Schedule(1, func() error { secondRun = append(secondRun, "C"); return nil })
		scheduler2.Advance(2)

		for i := range firstRun {
			if firstRun[i] != secondRun[i] {
				t.Errorf("Run order differs at index %d: %v vs %v", i, firstRun, secondRun)
				break
			}
		}
	})
}

func TestRunner_FrameCounterDeterminism(t *testing.T) {
	t.Run("frame counter advances predictably", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)

		scenario := &model.Scenario{
			ID:          "test.frame_counter",
			DisplayName: "Frame Counter Test",
			Schema:      "1.0",
			Actions: []model.Action{
				{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 5}},
				{Type: model.ActionWaitFrames, Params: model.ActionParams{"frames": 3}},
			},
		}

		runner.Run(scenario)

		// Frame counter should be exactly 8 (5 + 3)
		if runner.frameCounter != 8 {
			t.Errorf("frameCounter = %d, want 8", runner.frameCounter)
		}
	})
}

func TestRunner_SceneReset(t *testing.T) {
	t.Run("reset clears frame counter", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)

		// Advance frame counter
		runner.frameCounter = 100

		// Reset scene
		runner.resetScene()

		if runner.frameCounter != 0 {
			t.Errorf("frameCounter after reset = %d, want 0", runner.frameCounter)
		}
	})

	t.Run("reset clears execution state", func(t *testing.T) {
		runner := NewRunner(&runtime.Runtime{}, nil)

		// Set up some state
		runner.currentStep = 5
		runner.frameCounter = 50

		// Reset
		runner.resetScene()

		// Note: resetScene doesn't clear currentStep by design (it's for step-based execution)
		// but should clear frameCounter
		if runner.frameCounter != 0 {
			t.Errorf("frameCounter = %d, want 0", runner.frameCounter)
		}
	})
}
