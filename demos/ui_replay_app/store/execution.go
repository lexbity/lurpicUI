package store

import (
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/ui_replay/model"
)

// ExecutionStateStore holds the current execution state.
var ExecutionStateStore = store.NewValueStore[ExecutionState](ExecutionState{})

// ExecutionState represents the current replay execution status.
type ExecutionState struct {
	Status        model.ExecutionStatus
	CurrentStep   int
	TotalSteps    int
	CurrentAction string
	Error         string
	Progress      float32
}

// IsRunning returns true if execution is in progress.
func (e ExecutionState) IsRunning() bool {
	return e.Status == model.StatusRunning
}

// CanStart returns true if a new execution can be started.
func (e ExecutionState) CanStart() bool {
	return e.Status == model.StatusPending ||
		e.Status == model.StatusPassed ||
		e.Status == model.StatusFailed ||
		e.Status == model.StatusError ||
		e.Status == model.StatusCancelled ||
		e.Status == ""
}

// RunHistoryStore holds the history of executed runs.
var RunHistoryStore = store.NewValueStore[*RunHistory](nil)

// RunHistory manages completed run results.
type RunHistory struct {
	runs []model.RunResult
}

// NewRunHistory creates an empty run history.
func NewRunHistory() *RunHistory {
	return &RunHistory{
		runs: make([]model.RunResult, 0),
	}
}

// Add adds a run result to the history.
func (h *RunHistory) Add(result model.RunResult) {
	if h == nil {
		return
	}
	h.runs = append(h.runs, result)
}

// All returns all runs in reverse chronological order.
func (h *RunHistory) All() []model.RunResult {
	if h == nil {
		return nil
	}
	n := len(h.runs)
	result := make([]model.RunResult, n)
	for i, r := range h.runs {
		result[n-1-i] = r
	}
	return result
}

// Latest returns the most recent run, if any.
func (h *RunHistory) Latest() (model.RunResult, bool) {
	if h == nil || len(h.runs) == 0 {
		return model.RunResult{}, false
	}
	return h.runs[len(h.runs)-1], true
}

// Count returns the total number of runs.
func (h *RunHistory) Count() int {
	if h == nil {
		return 0
	}
	return len(h.runs)
}

// CountByStatus returns the count of runs with the given status.
func (h *RunHistory) CountByStatus(status model.ExecutionStatus) int {
	if h == nil {
		return 0
	}
	count := 0
	for _, r := range h.runs {
		if r.Status == status {
			count++
		}
	}
	return count
}
