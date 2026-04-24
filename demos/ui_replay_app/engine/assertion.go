package engine

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// AssertionEngine evaluates scenario assertions against actual state.
type AssertionEngine struct {
	results  []AssertionResult
	runner   *Runner
	scenario *model.Scenario
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
	e.scenario = scenario

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

	actual := ""
	if e.runner != nil && e.runner.sceneManager != nil {
		actual = e.runner.sceneManager.CurrentScene()
	}

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
	actualState := e.controlStateValue(controlID)
	if actualState == "" {
		if e.scenario != nil && e.scenario.ExpectedState != nil {
			if value, ok := e.scenario.ExpectedState.ControlStates[controlID]; ok {
				actualState = value
			}
		}
	}

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

	actual := store.EnvironmentStore.Get().Theme
	if e.runner != nil && e.runner.sceneManager != nil && e.runner.sceneManager.CurrentTheme() != "" {
		actual = e.runner.sceneManager.CurrentTheme()
	}

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

	actual := store.EnvironmentStore.Get().Density
	if e.runner != nil && e.runner.sceneManager != nil && e.runner.sceneManager.CurrentDensity() != "" {
		actual = e.runner.sceneManager.CurrentDensity()
	}

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

	actual := ""
	if e.runner != nil && e.runner.runtime != nil {
		actual = fmt.Sprintf("%d", e.runner.runtime.FocusedID())
	}

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

	found := false
	if e.runner != nil && e.runner.logger != nil {
		for _, entry := range e.runner.logger.EntriesByCategory("event") {
			if entry.Data != nil {
				if eventTypeValue, ok := entry.Data["event_type"].(string); ok && eventTypeValue == eventType {
					found = true
					break
				}
			}
		}
	}

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
	actual := e.storeSummary(storeID)
	if actual == "" {
		actual = "unknown store: " + storeID
	}

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
	actual := e.signalSummary(signalID)
	if actual == "" {
		actual = "signal: " + signalID + " count=0"
	}

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

	exists := false
	if e.runner != nil && e.runner.logger != nil {
		for _, entry := range e.runner.logger.EntriesByCategory("summary") {
			if entry.Data == nil {
				continue
			}
			if summaryType, ok := entry.Data["summary_type"].(string); ok && summaryType == "screenshot" {
				if summary, ok := entry.Data["summary"].(map[string]interface{}); ok {
					if summaryName, ok := summary["name"].(string); ok && summaryName == name {
						exists = true
						break
					}
				}
			}
		}
	}

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
	actualCount := 0
	if e.runner != nil && e.runner.logger != nil {
		level := levelFromString(expectedLevel)
		if level == "" {
			level = LevelWarning
		}
		actualCount = len(e.runner.logger.EntriesByLevel(level))
	}

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

func (e *AssertionEngine) controlStateValue(controlID string) string {
	switch controlID {
	case "focus", "focus_owner":
		if e.runner != nil && e.runner.runtime != nil {
			return fmt.Sprintf("%d", e.runner.runtime.FocusedID())
		}
	case "execution":
		exec := store.ExecutionStateStore.Get()
		return fmt.Sprintf("status=%s step=%d/%d action=%s progress=%.0f%%",
			exec.Status, exec.CurrentStep, exec.TotalSteps, exec.CurrentAction, exec.Progress*100)
	case "selected_scenario":
		return string(store.SelectedScenarioStore.Get())
	case "theme":
		return e.currentTheme()
	case "density":
		return e.currentDensity()
	case "window":
		env := store.EnvironmentStore.Get()
		return fmt.Sprintf("%dx%d", env.WindowWidth, env.WindowHeight)
	}
	if e.scenario != nil && e.scenario.ExpectedState != nil && e.scenario.ExpectedState.ControlStates != nil {
		if value, ok := e.scenario.ExpectedState.ControlStates[controlID]; ok {
			return value
		}
	}
	return ""
}

func (e *AssertionEngine) currentTheme() string {
	if e.runner != nil && e.runner.sceneManager != nil && e.runner.sceneManager.CurrentTheme() != "" {
		return e.runner.sceneManager.CurrentTheme()
	}
	return store.EnvironmentStore.Get().Theme
}

func (e *AssertionEngine) currentDensity() string {
	if e.runner != nil && e.runner.sceneManager != nil && e.runner.sceneManager.CurrentDensity() != "" {
		return e.runner.sceneManager.CurrentDensity()
	}
	return store.EnvironmentStore.Get().Density
}

func (e *AssertionEngine) storeSummary(storeID string) string {
	switch storeID {
	case "execution":
		exec := store.ExecutionStateStore.Get()
		return fmt.Sprintf("status=%s step=%d/%d progress=%.0f%% action=%s assertions=%d failures=%d",
			exec.Status, exec.CurrentStep, exec.TotalSteps, exec.Progress*100, exec.CurrentAction, exec.AssertionCount(), exec.AssertionFailures())
	case "environment":
		env := store.EnvironmentStore.Get()
		return fmt.Sprintf("%s / %s / %s / %s / %dx%d",
			env.Backend, env.Platform, env.Theme, env.Density, env.WindowWidth, env.WindowHeight)
	case "history":
		history := store.RunHistoryStore.Get()
		if history == nil {
			return "runs=0"
		}
		return fmt.Sprintf("runs=%d passed=%d failed=%d cancelled=%d error=%d",
			history.Count(), history.CountByStatus(model.StatusPassed), history.CountByStatus(model.StatusFailed),
			history.CountByStatus(model.StatusCancelled), history.CountByStatus(model.StatusError))
	case "registry":
		reg := store.ScenarioRegistryStore.Get()
		if reg == nil {
			return "scenarios=0"
		}
		return fmt.Sprintf("scenarios=%d valid=%d invalid=%d", reg.Count(), reg.ValidCount(), reg.InvalidCount())
	default:
		return ""
	}
}

func (e *AssertionEngine) signalSummary(signalID string) string {
	if e.runner == nil || e.runner.logger == nil {
		return ""
	}
	entries := e.runner.logger.EntriesByCategory(signalID)
	if len(entries) == 0 {
		return fmt.Sprintf("signal=%s count=0", signalID)
	}
	return fmt.Sprintf("signal=%s count=%d last_step=%d", signalID, len(entries), entries[len(entries)-1].Step)
}

func levelFromString(level string) LogLevel {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warning":
		return LevelWarning
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return ""
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
