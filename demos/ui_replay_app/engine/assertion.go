package engine

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/ui_replay/model"
)

// AssertionEngine evaluates scenario assertions against actual state.
type AssertionEngine struct {
	results []AssertionResult
	runner  *Runner
}

// AssertionResult captures the outcome of a single assertion.
type AssertionResult struct {
	Type      string
	Passed    bool
	Expected  interface{}
	Actual    interface{}
	Message   string
	Timestamp time.Time
	Step      int
}

// NewAssertionEngine creates a new assertion engine.
func (r *Runner) NewAssertionEngine() *AssertionEngine {
	return &AssertionEngine{
		results: make([]AssertionResult, 0),
		runner:  r,
	}
}

// Evaluate runs all scenario assertions and returns results.
func (e *AssertionEngine) Evaluate(scenario *model.Scenario) ([]AssertionResult, error) {
	e.results = make([]AssertionResult, 0, len(scenario.Assertions))

	for i, assertion := range scenario.Assertions {
		result := e.evaluateAssertion(i+1, assertion)
		e.results = append(e.results, result)
	}

	return e.results, nil
}

// EvaluateSingle evaluates a single assertion.
func (e *AssertionEngine) EvaluateSingle(step int, assertion model.Assertion) AssertionResult {
	return e.evaluateAssertion(step, assertion)
}

// AllPassed returns true if all assertions passed.
func (e *AssertionEngine) AllPassed() bool {
	for _, r := range e.results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// Results returns all assertion results.
func (e *AssertionEngine) Results() []AssertionResult {
	return e.results
}

// Failed returns only failed assertions.
func (e *AssertionEngine) Failed() []AssertionResult {
	var failed []AssertionResult
	for _, r := range e.results {
		if !r.Passed {
			failed = append(failed, r)
		}
	}
	return failed
}

func (e *AssertionEngine) evaluateAssertion(step int, assertion model.Assertion) AssertionResult {
	result := AssertionResult{
		Type:      string(assertion.Type),
		Timestamp: time.Now(),
		Step:      step,
	}

	switch assertion.Type {
	case model.AssertSceneID:
		return e.evaluateSceneID(step, assertion)
	case model.AssertControlState:
		return e.evaluateControlState(step, assertion)
	case model.AssertThemeState:
		return e.evaluateThemeState(step, assertion)
	case model.AssertDensityState:
		return e.evaluateDensityState(step, assertion)
	case model.AssertFocusOwner:
		return e.evaluateFocusOwner(step, assertion)
	case model.AssertEventPresent:
		return e.evaluateEventPresent(step, assertion)
	case model.AssertStoreSummary:
		return e.evaluateStoreSummary(step, assertion)
	case model.AssertSignalSummary:
		return e.evaluateSignalSummary(step, assertion)
	case model.AssertScreenshot:
		return e.evaluateScreenshot(step, assertion)
	case model.AssertDiagnostics:
		return e.evaluateDiagnostics(step, assertion)
	case model.AssertFrameCount:
		return e.evaluateFrameCount(step, assertion)
	default:
		result.Passed = false
		result.Message = fmt.Sprintf("unknown assertion type: %s", assertion.Type)
		return result
	}
}

func (e *AssertionEngine) evaluateSceneID(step int, assertion model.Assertion) AssertionResult {
	expected, ok := assertion.Params["expected"].(string)
	if !ok {
		return AssertionResult{
			Type:     string(model.AssertSceneID),
			Passed:   false,
			Expected: expected,
			Message:  "missing or invalid 'expected' param",
			Step:     step,
		}
	}

	// TODO: Get actual scene ID from runtime
	actual := "unknown" // Placeholder

	passed := actual == expected
	return AssertionResult{
		Type:     string(model.AssertSceneID),
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
		Message:  fmt.Sprintf("scene_id: expected '%s', got '%s'", expected, actual),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateControlState(step int, assertion model.Assertion) AssertionResult {
	controlID, ok := assertion.Params["control_id"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertControlState),
			Passed:  false,
			Message: "missing or invalid 'control_id' param",
			Step:    step,
		}
	}

	expectedState, _ := assertion.Params["expected"].(string)

	// TODO: Get actual control state from runtime
	actualState := "unknown" // Placeholder

	passed := actualState == expectedState
	return AssertionResult{
		Type:     string(model.AssertControlState),
		Passed:   passed,
		Expected: expectedState,
		Actual:   actualState,
		Message:  fmt.Sprintf("control '%s': expected '%s', got '%s'", controlID, expectedState, actualState),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateThemeState(step int, assertion model.Assertion) AssertionResult {
	expected, ok := assertion.Params["expected"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertThemeState),
			Passed:  false,
			Message: "missing or invalid 'expected' param",
			Step:    step,
		}
	}

	// TODO: Get actual theme from runtime
	actual := "baseline" // Placeholder

	passed := actual == expected
	return AssertionResult{
		Type:     string(model.AssertThemeState),
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
		Message:  fmt.Sprintf("theme: expected '%s', got '%s'", expected, actual),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateDensityState(step int, assertion model.Assertion) AssertionResult {
	expected, ok := assertion.Params["expected"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertDensityState),
			Passed:  false,
			Message: "missing or invalid 'expected' param",
			Step:    step,
		}
	}

	// TODO: Get actual density from runtime
	actual := "default" // Placeholder

	passed := actual == expected
	return AssertionResult{
		Type:     string(model.AssertDensityState),
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
		Message:  fmt.Sprintf("density: expected '%s', got '%s'", expected, actual),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateFocusOwner(step int, assertion model.Assertion) AssertionResult {
	expected, ok := assertion.Params["expected"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertFocusOwner),
			Passed:  false,
			Message: "missing or invalid 'expected' param",
			Step:    step,
		}
	}

	// TODO: Get actual focus owner from runtime
	actual := "" // Placeholder - empty means no focus

	passed := actual == expected
	return AssertionResult{
		Type:     string(model.AssertFocusOwner),
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
		Message:  fmt.Sprintf("focus: expected '%s', got '%s'", expected, actual),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateEventPresent(step int, assertion model.Assertion) AssertionResult {
	eventType, ok := assertion.Params["event_type"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertEventPresent),
			Passed:  false,
			Message: "missing or invalid 'event_type' param",
			Step:    step,
		}
	}

	// TODO: Check event log for presence of event type
	found := false // Placeholder

	return AssertionResult{
		Type:     string(model.AssertEventPresent),
		Passed:   found,
		Expected: eventType,
		Actual:   found,
		Message:  fmt.Sprintf("event '%s': present=%v", eventType, found),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateStoreSummary(step int, assertion model.Assertion) AssertionResult {
	storeID, ok := assertion.Params["store_id"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertStoreSummary),
			Passed:  false,
			Message: "missing or invalid 'store_id' param",
			Step:    step,
		}
	}

	expected, _ := assertion.Params["expected"].(string)

	// TODO: Get actual store state summary
	actual := "unknown" // Placeholder

	passed := actual == expected
	return AssertionResult{
		Type:     string(model.AssertStoreSummary),
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
		Message:  fmt.Sprintf("store '%s': expected '%s', got '%s'", storeID, expected, actual),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateSignalSummary(step int, assertion model.Assertion) AssertionResult {
	signalID, ok := assertion.Params["signal_id"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertSignalSummary),
			Passed:  false,
			Message: "missing or invalid 'signal_id' param",
			Step:    step,
		}
	}

	expected, _ := assertion.Params["expected"].(string)

	// TODO: Get actual signal state summary
	actual := "unknown" // Placeholder

	passed := actual == expected
	return AssertionResult{
		Type:     string(model.AssertSignalSummary),
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
		Message:  fmt.Sprintf("signal '%s': expected '%s', got '%s'", signalID, expected, actual),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateScreenshot(step int, assertion model.Assertion) AssertionResult {
	name, ok := assertion.Params["name"].(string)
	if !ok {
		return AssertionResult{
			Type:    string(model.AssertScreenshot),
			Passed:  false,
			Message: "missing or invalid 'name' param",
			Step:    step,
		}
	}

	// TODO: Check if screenshot exists
	exists := false // Placeholder

	return AssertionResult{
		Type:     string(model.AssertScreenshot),
		Passed:   exists,
		Expected: name,
		Actual:   exists,
		Message:  fmt.Sprintf("screenshot '%s': exists=%v", name, exists),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateDiagnostics(step int, assertion model.Assertion) AssertionResult {
	expectedLevel, _ := assertion.Params["level"].(string)
	maxCount, _ := assertion.Params["max_count"].(float64)

	// TODO: Get actual diagnostic counts by level
	actualCount := 0 // Placeholder

	passed := float64(actualCount) <= maxCount
	return AssertionResult{
		Type:     string(model.AssertDiagnostics),
		Passed:   passed,
		Expected: maxCount,
		Actual:   actualCount,
		Message:  fmt.Sprintf("diagnostics (%s): max %v, got %d", expectedLevel, maxCount, actualCount),
		Step:     step,
	}
}

func (e *AssertionEngine) evaluateFrameCount(step int, assertion model.Assertion) AssertionResult {
	minFrames, hasMin := assertion.Params["min"].(float64)
	maxFrames, hasMax := assertion.Params["max"].(float64)

	actual := e.runner.frameCounter

	var passed bool
	var message string

	if hasMin && hasMax {
		passed = float64(actual) >= minFrames && float64(actual) <= maxFrames
		message = fmt.Sprintf("frame count: expected [%v, %v], got %d", minFrames, maxFrames, actual)
	} else if hasMin {
		passed = float64(actual) >= minFrames
		message = fmt.Sprintf("frame count: expected >= %v, got %d", minFrames, actual)
	} else if hasMax {
		passed = float64(actual) <= maxFrames
		message = fmt.Sprintf("frame count: expected <= %v, got %d", maxFrames, actual)
	} else {
		passed = false
		message = "frame count: missing 'min' or 'max' param"
	}

	return AssertionResult{
		Type:     string(model.AssertFrameCount),
		Passed:   passed,
		Expected: fmt.Sprintf("[%v, %v]", minFrames, maxFrames),
		Actual:   actual,
		Message:  message,
		Step:     step,
	}
}

// Error returns a formatted error string for failed assertions.
func (r AssertionResult) Error() string {
	if r.Passed {
		return ""
	}
	return fmt.Sprintf("step %d: %s", r.Step, r.Message)
}

// Summary returns a human-readable summary of assertion results.
func (e *AssertionEngine) Summary() AssertionSummary {
	passed := 0
	failed := 0
	var failures []string

	for _, r := range e.results {
		if r.Passed {
			passed++
		} else {
			failed++
			failures = append(failures, r.Message)
		}
	}

	return AssertionSummary{
		Total:    len(e.results),
		Passed:   passed,
		Failed:   failed,
		Failures: failures,
	}
}

// AssertionSummary provides a high-level view of assertion results.
type AssertionSummary struct {
	Total    int
	Passed   int
	Failed   int
	Failures []string
}

// String returns a formatted summary string.
func (s AssertionSummary) String() string {
	if s.Failed == 0 {
		return fmt.Sprintf("All %d assertions passed", s.Total)
	}
	return fmt.Sprintf("%d/%d assertions failed", s.Failed, s.Total)
}
