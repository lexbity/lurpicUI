package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/artifact"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// Runner executes scenarios in a deterministic manner.
type Runner struct {
	ctx            context.Context
	cancel         context.CancelFunc
	runtime        *runtime.Runtime
	root           *facet.Facet
	mu             sync.RWMutex
	state          RunState
	currentStep    int
	totalSteps     int
	scenario       *model.Scenario
	sceneManager   *SceneManager
	onStep         func(step int, total int, action model.Action)
	onComplete     func(result *model.RunResult)
	onError        func(err error)
	frameCounter   int
	idleReadyFrame int
	idleStartTime  time.Time
	logger         *SemanticLogger
	bundleBuilder  *artifact.BundleBuilder
	lastBundle     *artifact.Bundle
}

// RunState represents the current execution state.
type RunState string

const (
	StateIdle      RunState = "idle"
	StatePreparing RunState = "preparing"
	StateRunning   RunState = "running"
	StateWaiting   RunState = "waiting"
	StateAsserting RunState = "asserting"
	StateCapturing RunState = "capturing"
	StateCompleted RunState = "completed"
	StateFailed    RunState = "failed"
	StateCancelled RunState = "cancelled"
)

// NewRunner creates a new scenario runner.
func NewRunner(rt *runtime.Runtime, root *facet.Facet) *Runner {
	return &Runner{
		runtime: rt,
		root:    root,
		state:   StateIdle,
		logger:  NewSemanticLogger(),
	}
}

// Run executes a scenario from start to finish.
func (r *Runner) Run(scenario *model.Scenario) (*model.RunResult, error) {
	if err := scenario.Validate(); err != nil {
		return nil, fmt.Errorf("scenario validation failed: %w", err)
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	defer r.cancel()

	r.scenario = scenario
	r.currentStep = 0
	r.totalSteps = len(scenario.Actions) + len(scenario.Assertions)
	r.frameCounter = 0
	r.idleReadyFrame = 0
	r.sceneManager = r.NewSceneManager()
	if r.logger != nil {
		r.logger.Clear()
	}

	// Initialize bundle builder for artifact collection
	exportDir := store.GetPaths().ExportDir
	if exportDir == "" {
		exportDir = "./export"
	}
	r.bundleBuilder = artifact.NewBundleBuilder(scenario, exportDir)
	r.bundleBuilder.SetProvenance(artifact.ProvenanceInfo{
		Platform: store.EnvironmentStore.Get().Platform,
	})

	result := &model.RunResult{
		ScenarioID: scenario.ID,
		Status:     model.StatusPending,
		StartTime:  time.Now(),
		StepsTotal: r.totalSteps,
	}

	// Update execution state
	r.setState(StatePreparing)
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:      model.StatusRunning,
		CurrentStep: 0,
		TotalSteps:  r.totalSteps,
		Progress:    0,
	})

	// Reset scene to canonical state
	if err := r.resetScene(); err != nil {
		result.Status = model.StatusError
		result.Error = fmt.Sprintf("scene reset failed: %v", err)
		return r.finishRun(result, model.StatusError, StateFailed, 0)
	}

	// Apply environment
	if err := r.applyEnvironment(&scenario.Environment); err != nil {
		result.Status = model.StatusError
		result.Error = fmt.Sprintf("environment application failed: %v", err)
		return r.finishRun(result, model.StatusError, StateFailed, 0)
	}

	// Execute actions
	r.setState(StateRunning)
	for i, action := range scenario.Actions {
		select {
		case <-r.ctx.Done():
			result.StepsExecuted = r.currentStep
			return r.finishRun(result, model.StatusCancelled, StateCancelled, r.currentStep)
		default:
		}

		r.currentStep = i + 1
		r.setCurrentAction(action)
		if r.onStep != nil {
			r.onStep(r.currentStep, r.totalSteps, action)
		}
		if r.logger != nil {
			r.logger.ActionStarted(string(action.Type), r.currentStep)
		}

		start := time.Now()
		if err := r.executeAction(action); err != nil {
			if err == context.Canceled {
				result.StepsExecuted = r.currentStep
				return r.finishRun(result, model.StatusCancelled, StateCancelled, r.currentStep)
			}
			result.Status = model.StatusFailed
			result.Error = fmt.Sprintf("step %d: %v", r.currentStep, err)
			result.StepsExecuted = r.currentStep - 1
			if r.logger != nil {
				r.logger.ActionFailed(string(action.Type), r.currentStep, err)
			}
			if r.onError != nil {
				r.onError(err)
			}
			return r.finishRun(result, model.StatusFailed, StateFailed, r.currentStep)
		}
		if r.logger != nil {
			r.logger.ActionCompleted(string(action.Type), r.currentStep, time.Since(start))
		}

		r.updateProgress()
	}

	// Execute assertions as a read-only checkpoint phase.
	if len(scenario.Assertions) > 0 {
		assertionsPassed, assertionResults, err := r.executeAssertions(scenario.Assertions)
		result.AssertionResults = toModelAssertionResults(assertionResults)
		store.ExecutionStateStore.Set(store.ExecutionState{
			Status:           model.StatusRunning,
			CurrentStep:      r.currentStep,
			TotalSteps:       r.totalSteps,
			CurrentAction:    "assertions",
			Progress:         runProgress(r.currentStep, r.totalSteps),
			AssertionResults: result.AssertionResults,
		})
		if err != nil {
			result.Status = model.StatusFailed
			result.Error = err.Error()
			result.StepsExecuted = r.currentStep
			if r.onError != nil {
				r.onError(err)
			}
			return r.finishRun(result, model.StatusFailed, StateFailed, r.currentStep)
		}
		if !assertionsPassed {
			result.Status = model.StatusFailed
			if len(assertionResults) > 0 {
				result.Error = assertionResults[0].Error()
			}
			result.StepsExecuted = r.currentStep
			return r.finishRun(result, model.StatusFailed, StateFailed, r.currentStep)
		}
	}

	result.Status = model.StatusPassed
	result.StepsExecuted = r.totalSteps
	return r.finishRun(result, model.StatusPassed, StateCompleted, r.totalSteps)
}

// Cancel stops the current execution.
func (r *Runner) Cancel() {
	if r.cancel != nil {
		r.cancel()
	}
	r.setState(StateCancelled)
}

// State returns the current run state.
func (r *Runner) State() RunState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// CurrentStep returns the current step index (1-based).
func (r *Runner) CurrentStep() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentStep
}

// SetStepCallback sets the step progress callback.
func (r *Runner) SetStepCallback(cb func(step int, total int, action model.Action)) {
	r.onStep = cb
}

// SetCompleteCallback sets the completion callback.
func (r *Runner) SetCompleteCallback(cb func(result *model.RunResult)) {
	r.onComplete = cb
}

// SetErrorCallback sets the error callback.
func (r *Runner) SetErrorCallback(cb func(err error)) {
	r.onError = cb
}

func (r *Runner) setState(state RunState) {
	r.mu.Lock()
	oldState := r.state
	r.state = state
	r.mu.Unlock()

	if r.logger != nil && oldState != state {
		r.logger.StateChanged(oldState, state)
	}
}

func (r *Runner) updateProgress() {
	progress := float32(r.currentStep) / float32(r.totalSteps)
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:      model.StatusRunning,
		CurrentStep: r.currentStep,
		TotalSteps:  r.totalSteps,
		Progress:    progress,
	})
}

func (r *Runner) finishRun(result *model.RunResult, status model.ExecutionStatus, state RunState, currentStep int) (*model.RunResult, error) {
	result.Status = status
	result.EndTime = time.Now()
	r.setState(state)

	// Finalize bundle with run results before saving to history
	if r.bundleBuilder != nil {
		r.bundleBuilder.SetRunResults(result)
		if bundle, err := r.bundleBuilder.Build(); err == nil {
			r.lastBundle = bundle
			// Auto-save bundle for passed/failed runs
			if status == model.StatusPassed || status == model.StatusFailed {
				_ = bundle.SaveToDisk()
			}
		}
	}

	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:           status,
		CurrentStep:      currentStep,
		TotalSteps:       r.totalSteps,
		CurrentAction:    "",
		Error:            result.Error,
		Progress:         runProgress(currentStep, r.totalSteps),
		AssertionResults: result.AssertionResults,
	})
	r.clearCurrentAction()
	if result.Status == model.StatusPassed || result.Status == model.StatusFailed || result.Status == model.StatusError || result.Status == model.StatusCancelled {
		history := store.RunHistoryStore.Get()
		if history == nil {
			history = store.NewRunHistory()
			store.RunHistoryStore.Set(history)
		}
		history.Add(*result)
	}
	if r.onComplete != nil {
		r.onComplete(result)
	}

	return result, nil
}

func runProgress(currentStep, totalSteps int) float32 {
	if totalSteps <= 0 {
		return 0
	}
	if currentStep >= totalSteps {
		return 1
	}
	if currentStep < 0 {
		return 0
	}
	return float32(currentStep) / float32(totalSteps)
}

func (r *Runner) applyEnvironment(env *model.Environment) error {
	current := store.EnvironmentStore.Get()
	if env.Backend != "" {
		current.Backend = env.Backend
	}
	if env.Platform != "" {
		current.Platform = env.Platform
	}
	if env.Theme != "" {
		current.Theme = env.Theme
	}
	if env.Density != "" {
		current.Density = env.Density
	}
	if env.WindowSize.Width > 0 {
		current.WindowWidth = env.WindowSize.Width
		current.WindowHeight = env.WindowSize.Height
	}
	store.EnvironmentStore.Set(current)
	if r.sceneManager == nil {
		r.sceneManager = r.NewSceneManager()
	}
	if env.Theme != "" {
		r.sceneManager.ChangeTheme(env.Theme, 0)
	}
	if env.Density != "" {
		r.sceneManager.ChangeDensity(env.Density, 0)
	}
	if env.WindowSize.Width > 0 && env.WindowSize.Height > 0 && r.runtime != nil {
		r.runtime.ResizeWindow(env.WindowSize.Width, env.WindowSize.Height)
	}
	if r.runtime != nil {
		r.runtime.RequestFrame()
	}
	return nil
}

func (r *Runner) resetScene() error {
	// Reset frame counter for deterministic timing
	r.frameCounter = 0
	r.idleReadyFrame = 0
	r.idleStartTime = time.Time{}
	if r.sceneManager != nil {
		r.sceneManager.Reset()
	}

	// Clear any stale runtime state
	if r.runtime != nil {
		r.runtime.ClearInputState()
		r.runtime.ClearFocus()
		if r.root != nil && r.root.Base() != nil {
			r.runtime.Invalidate(r.root.Base().ID(), facet.DirtyAll, "runner.resetScene")
		}
		r.runtime.RequestFrame()
	}

	// Reset stores to known state
	// Note: We don't clear the registry, just execution state
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:        model.StatusRunning,
		CurrentStep:   0,
		TotalSteps:    r.totalSteps,
		CurrentAction: "",
		Progress:      0,
	})

	return nil
}

func (r *Runner) executeAction(action model.Action) error {
	switch action.Type {
	case model.ActionWaitFrames:
		return r.executeWaitFrames(action)
	case model.ActionWaitIdle:
		return r.executeWaitIdle(action)
	case model.ActionSceneLoad:
		return r.executeSceneLoad(action)
	case model.ActionClick:
		return r.executeClick(action)
	case model.ActionPointerMove:
		return r.executePointerMove(action)
	case model.ActionDrag:
		return r.executeDrag(action)
	case model.ActionKeyInput:
		return r.executeKeyInput(action)
	case model.ActionTextInput:
		return r.executeTextInput(action)
	case model.ActionIMEHook:
		return r.executeIMEHook(action)
	case model.ActionScreenshot:
		return r.executeScreenshot(action)
	case model.ActionAssertState:
		return r.executeAssertState(action)
	case model.ActionSwitchTheme:
		return r.executeSwitchTheme(action)
	case model.ActionSwitchDensity:
		return r.executeSwitchDensity(action)
	case model.ActionResizeWindow:
		return r.executeResizeWindow(action)
	case model.ActionExportBundle:
		return r.executeExportBundle(action)
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func (r *Runner) executeWaitFrames(action model.Action) error {
	var frames int
	switch v := action.Params["frames"].(type) {
	case float64:
		frames = int(v)
	case int:
		frames = v
	default:
		return fmt.Errorf("wait_frames: missing or invalid 'frames' param")
	}
	if frames < 1 {
		return fmt.Errorf("wait_frames: frames must be positive")
	}

	r.setState(StateWaiting)
	for i := 0; i < frames; i++ {
		if r.ctx != nil {
			select {
			case <-r.ctx.Done():
				return context.Canceled
			default:
			}
		}
		r.frameCounter++
		r.idleReadyFrame = r.frameCounter
		if r.runtime != nil {
			r.runtime.RequestFrame()
		}
		time.Sleep(time.Millisecond)
	}

	r.setState(StateRunning)
	return nil
}

func (r *Runner) executeWaitIdle(action model.Action) error {
	timeoutMs := 5000.0 // default 5 seconds
	if t, ok := action.Params["timeout_ms"].(float64); ok {
		timeoutMs = t
	} else if t, ok := action.Params["timeout_ms"].(int); ok {
		timeoutMs = float64(t)
	}

	r.setState(StateWaiting)
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	for {
		if r.ctx != nil {
			select {
			case <-r.ctx.Done():
				return context.Canceled
			default:
			}
		}
		if r.isIdle() {
			break
		}
		if time.Now().After(deadline) {
			r.setState(StateRunning)
			return fmt.Errorf("wait_idle: timeout after %v ms", timeoutMs)
		}
		r.frameCounter++
		if r.runtime != nil {
			r.runtime.RequestFrame()
		}
		if r.frameCounter >= r.idleReadyFrame {
			break
		}
		time.Sleep(time.Millisecond)
	}

	r.setState(StateRunning)
	return nil
}

func (r *Runner) isIdle() bool {
	return r.frameCounter >= r.idleReadyFrame
}

func (r *Runner) executeSceneLoad(action model.Action) error {
	scene, ok := action.Params["scene"].(string)
	if !ok {
		return fmt.Errorf("scene_load: missing or invalid 'scene' param")
	}

	if r.sceneManager == nil {
		r.sceneManager = r.NewSceneManager()
	}
	if err := r.sceneManager.TransitionScene(scene, r.currentStep); err != nil {
		return err
	}
	if r.logger != nil {
		r.logger.EventCaptured("scene_load", scene, map[string]interface{}{"step": r.currentStep})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeClick(action model.Action) error {
	target := action.Target.Resolve()
	if target.IsEmpty() {
		return fmt.Errorf("click: missing target")
	}

	if r.sceneManager != nil {
		if _, err := r.sceneManager.ReResolveTarget(target); err != nil {
			return err
		}
	}
	if r.logger != nil {
		r.logger.EventCaptured("click", target.LogicalID, map[string]interface{}{"step": r.currentStep})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executePointerMove(action model.Action) error {
	x, xok := action.Params["x"].(float64)
	y, yok := action.Params["y"].(float64)
	if !xok || !yok {
		return fmt.Errorf("pointer_move: missing or invalid coordinates")
	}

	_ = x
	_ = y
	if r.logger != nil {
		r.logger.EventCaptured("pointer_move", "", map[string]interface{}{
			"x": x, "y": y, "step": r.currentStep,
		})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeScreenshot(action model.Action) error {
	name, _ := action.Params["name"].(string)
	if name == "" {
		return fmt.Errorf("screenshot: missing or invalid 'name' param")
	}
	r.setState(StateCapturing)
	r.markActivity(1)
	if r.logger != nil {
		r.logger.SummaryCaptured("screenshot", map[string]interface{}{
			"name": name,
			"step": r.currentStep,
		})
	}

	r.setState(StateRunning)
	return nil
}

func (r *Runner) executeSwitchTheme(action model.Action) error {
	theme, ok := action.Params["theme"].(string)
	if !ok {
		return fmt.Errorf("switch_theme: missing or invalid 'theme' param")
	}

	env := store.EnvironmentStore.Get()
	env.Theme = theme
	store.EnvironmentStore.Set(env)
	if r.sceneManager == nil {
		r.sceneManager = r.NewSceneManager()
	}
	r.sceneManager.ChangeTheme(theme, r.currentStep)
	r.markActivity(1)

	return nil
}

func (r *Runner) executeSwitchDensity(action model.Action) error {
	density, ok := action.Params["density"].(string)
	if !ok {
		return fmt.Errorf("switch_density: missing or invalid 'density' param")
	}

	env := store.EnvironmentStore.Get()
	env.Density = density
	store.EnvironmentStore.Set(env)
	if r.sceneManager == nil {
		r.sceneManager = r.NewSceneManager()
	}
	r.sceneManager.ChangeDensity(density, r.currentStep)
	r.markActivity(1)

	return nil
}

func (r *Runner) executeResizeWindow(action model.Action) error {
	width, wok := action.Params["width"].(float64)
	height, hok := action.Params["height"].(float64)
	if !wok || !hok {
		return fmt.Errorf("resize_window: missing or invalid dimensions")
	}

	if width <= 0 || height <= 0 {
		return fmt.Errorf("resize_window: dimensions must be positive")
	}
	env := store.EnvironmentStore.Get()
	env.WindowWidth = int(width)
	env.WindowHeight = int(height)
	store.EnvironmentStore.Set(env)
	if r.runtime != nil {
		r.runtime.ResizeWindow(int(width), int(height))
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeDrag(action model.Action) error {
	target := action.Target.Resolve()
	if target.IsEmpty() {
		return fmt.Errorf("drag: missing target")
	}

	destX, xok := action.Params["dest_x"].(float64)
	destY, yok := action.Params["dest_y"].(float64)
	if !xok || !yok {
		return fmt.Errorf("drag: missing or invalid destination coordinates")
	}

	_, _, _ = target, destX, destY
	if r.sceneManager != nil {
		if _, err := r.sceneManager.ReResolveTarget(target); err != nil {
			return err
		}
	}
	if r.logger != nil {
		r.logger.EventCaptured("drag", target.LogicalID, map[string]interface{}{
			"dest_x": destX, "dest_y": destY, "step": r.currentStep,
		})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeKeyInput(action model.Action) error {
	key, ok := action.Params["key"].(string)
	if !ok {
		return fmt.Errorf("key_input: missing or invalid 'key' param")
	}

	_ = key
	if r.logger != nil {
		r.logger.EventCaptured("key_input", "", map[string]interface{}{"key": key, "step": r.currentStep})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeTextInput(action model.Action) error {
	text, ok := action.Params["text"].(string)
	if !ok {
		return fmt.Errorf("text_input: missing or invalid 'text' param")
	}

	_ = text
	if r.logger != nil {
		r.logger.EventCaptured("text_input", "", map[string]interface{}{"text": text, "step": r.currentStep})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeIMEHook(action model.Action) error {
	actionType, ok := action.Params["action"].(string)
	if !ok {
		return fmt.Errorf("ime_hook: missing or invalid 'action' param")
	}

	_ = actionType
	if r.logger != nil {
		r.logger.EventCaptured("ime_hook", "", map[string]interface{}{"action": actionType, "step": r.currentStep})
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeAssertState(action model.Action) error {
	r.setState(StateAsserting)
	defer r.setState(StateRunning)

	assertionType, ok := action.Params["type"].(string)
	if !ok {
		return fmt.Errorf("assert_state: missing or invalid 'type' param")
	}

	if passed, err := r.evaluateActionAssertion(assertionType, action.Params); err != nil {
		if r.logger != nil {
			r.logger.AssertionChecked(assertionType, r.currentStep, false)
		}
		return err
	} else if !passed {
		if r.logger != nil {
			r.logger.AssertionChecked(assertionType, r.currentStep, false)
		}
		return fmt.Errorf("assert_state: assertion failed for %s", assertionType)
	}
	if r.logger != nil {
		r.logger.AssertionChecked(assertionType, r.currentStep, true)
	}
	r.markActivity(1)

	return nil
}

func (r *Runner) executeAssertions(assertions []model.Assertion) (bool, []AssertionResult, error) {
	engine := r.NewAssertionEngine()
	results := make([]AssertionResult, 0, len(assertions))
	for i, assertion := range assertions {
		r.currentStep = len(r.scenario.Actions) + i + 1
		r.setCurrentAssertion(assertion)
		result := engine.evaluateAssertion(r.currentStep, assertion)
		results = append(results, result)
		r.publishAssertionResults(results)
		if r.logger != nil {
			r.logger.AssertionChecked(string(assertion.Type), r.currentStep, result.Passed)
		}
		if !result.Passed {
			return false, results, fmt.Errorf(result.Error())
		}
		r.updateProgress()
	}
	return true, results, nil
}

func (r *Runner) setCurrentAssertion(assertion model.Assertion) {
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:           model.StatusRunning,
		CurrentStep:      r.currentStep,
		TotalSteps:       r.totalSteps,
		CurrentAction:    "assert:" + string(assertion.Type),
		Progress:         runProgress(r.currentStep, r.totalSteps),
		AssertionResults: nil,
	})
}

func (r *Runner) publishAssertionResults(results []AssertionResult) {
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:           model.StatusRunning,
		CurrentStep:      r.currentStep,
		TotalSteps:       r.totalSteps,
		CurrentAction:    "assertions",
		Progress:         runProgress(r.currentStep, r.totalSteps),
		AssertionResults: toModelAssertionResults(results),
	})
}

func toModelAssertionResults(results []AssertionResult) []model.AssertionResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]model.AssertionResult, len(results))
	for i, result := range results {
		out[i] = model.AssertionResult{
			Step:   result.Step,
			Type:   model.AssertionType(result.Type),
			Passed: result.Passed,
			Reason: result.Message,
		}
	}
	return out
}

func (r *Runner) evaluateActionAssertion(assertionType string, params model.ActionParams) (bool, error) {
	switch model.AssertionType(assertionType) {
	case model.AssertSceneID:
		expected, ok := params["expected"].(string)
		if !ok {
			return false, fmt.Errorf("assert_state: missing or invalid 'expected' param")
		}
		actual := ""
		if r.sceneManager != nil {
			actual = r.sceneManager.CurrentScene()
		}
		return actual == expected, nil
	case model.AssertThemeState:
		expected, ok := params["expected"].(string)
		if !ok {
			return false, fmt.Errorf("assert_state: missing or invalid 'expected' param")
		}
		actual := store.EnvironmentStore.Get().Theme
		if r.sceneManager != nil && r.sceneManager.CurrentTheme() != "" {
			actual = r.sceneManager.CurrentTheme()
		}
		return actual == expected, nil
	case model.AssertDensityState:
		expected, ok := params["expected"].(string)
		if !ok {
			return false, fmt.Errorf("assert_state: missing or invalid 'expected' param")
		}
		actual := store.EnvironmentStore.Get().Density
		if r.sceneManager != nil && r.sceneManager.CurrentDensity() != "" {
			actual = r.sceneManager.CurrentDensity()
		}
		return actual == expected, nil
	case model.AssertFrameCount:
		minFrames, hasMin := params["min"].(float64)
		maxFrames, hasMax := params["max"].(float64)
		if !hasMin && !hasMax {
			return false, fmt.Errorf("assert_state: missing 'min' or 'max' param")
		}
		actual := float64(r.frameCounter)
		if hasMin && actual < minFrames {
			return false, nil
		}
		if hasMax && actual > maxFrames {
			return false, nil
		}
		return true, nil
	case model.AssertFocusOwner:
		expected, ok := params["expected"].(string)
		if !ok {
			return false, fmt.Errorf("assert_state: missing or invalid 'expected' param")
		}
		actual := ""
		if r.runtime != nil {
			actual = fmt.Sprintf("%d", r.runtime.FocusedID())
		}
		return actual == expected, nil
	default:
		paramsCopy := make(model.AssertionParams, len(params))
		for k, v := range params {
			if k == "type" {
				continue
			}
			paramsCopy[k] = v
		}
		result := r.NewAssertionEngine().EvaluateSingle(r.currentStep, model.Assertion{
			Type:   model.AssertionType(assertionType),
			Params: paramsCopy,
		})
		if !result.Passed {
			return false, fmt.Errorf(result.Error())
		}
		return true, nil
	}
}

func (r *Runner) executeExportBundle(action model.Action) error {
	path, _ := action.Params["path"].(string)
	if path == "" {
		return fmt.Errorf("export_bundle: missing or invalid 'path' param")
	}

	// Ensure bundle builder is initialized
	if r.bundleBuilder == nil {
		return fmt.Errorf("export_bundle: no bundle builder available")
	}

	// Build the bundle
	bundle, err := r.bundleBuilder.Build()
	if err != nil {
		return fmt.Errorf("export_bundle: failed to build bundle: %w", err)
	}

	// Determine output format (directory or zip)
	if strings.HasSuffix(path, ".zip") {
		if err := bundle.SaveAsZip(path); err != nil {
			return fmt.Errorf("export_bundle: failed to save zip: %w", err)
		}
	} else {
		// Override output path for custom location
		bundle.OutputPath = path
		if err := bundle.SaveToDisk(); err != nil {
			return fmt.Errorf("export_bundle: failed to save bundle: %w", err)
		}
	}

	r.lastBundle = bundle
	r.markActivity(1)
	if r.logger != nil {
		r.logger.SummaryCaptured("bundle_export", map[string]interface{}{
			"path":   path,
			"step":   r.currentStep,
			"bundle": bundle.Manifest.ScenarioID,
		})
	}
	return nil
}

// GetLastBundle returns the most recently exported bundle, if any.
func (r *Runner) GetLastBundle() *artifact.Bundle {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastBundle
}

// CreateBundle builds a bundle from the current execution state.
// This can be called after Run() completes to get the execution artifacts.
func (r *Runner) CreateBundle(outputDir string) (*artifact.Bundle, error) {
	if r.scenario == nil {
		return nil, fmt.Errorf("no scenario loaded")
	}

	builder := artifact.NewBundleBuilder(r.scenario, outputDir)

	// Get execution result from history
	history := store.RunHistoryStore.Get()
	if history != nil {
		if latest, ok := history.Latest(); ok {
			builder.SetRunResults(&latest)
		}
	}

	// Set provenance
	builder.SetProvenance(artifact.ProvenanceInfo{
		Platform: store.EnvironmentStore.Get().Platform,
	})

	return builder.Build()
}

func (r *Runner) setCurrentAction(action model.Action) {
	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:        model.StatusRunning,
		CurrentStep:   r.currentStep,
		TotalSteps:    r.totalSteps,
		CurrentAction: string(action.Type),
		Progress:      runProgress(r.currentStep-1, r.totalSteps),
	})
}

func (r *Runner) clearCurrentAction() {
	state := store.ExecutionStateStore.Get()
	state.CurrentAction = ""
	store.ExecutionStateStore.Set(state)
}

func (r *Runner) markActivity(frames int) {
	if frames < 1 {
		frames = 1
	}
	if next := r.frameCounter + frames; next > r.idleReadyFrame {
		r.idleReadyFrame = next
	}
	if r.runtime != nil {
		r.runtime.RequestFrame()
	}
}

// Scheduler provides deterministic event ordering.
type Scheduler struct {
	events      []TimedEvent
	currentTime int64 // monotonic frame count
	runner      *Runner
}

// TimedEvent represents an event scheduled for a specific time.
type TimedEvent struct {
	Time   int64
	Action func() error
	Done   bool
	ID     string
}

// NewScheduler creates a new deterministic scheduler.
func (r *Runner) NewScheduler() *Scheduler {
	return &Scheduler{
		events:      make([]TimedEvent, 0),
		currentTime: 0,
		runner:      r,
	}
}

// Schedule adds an event to be executed at the given frame time.
func (s *Scheduler) Schedule(frameOffset int64, action func() error) string {
	event := TimedEvent{
		Time:   s.currentTime + frameOffset,
		Action: action,
		ID:     fmt.Sprintf("evt_%d_%d", s.currentTime, len(s.events)),
	}
	s.events = append(s.events, event)
	return event.ID
}

// Tick advances time and executes due events.
func (s *Scheduler) Tick() error {
	s.currentTime++

	// Execute events scheduled for this time (in registration order)
	for i := range s.events {
		if s.events[i].Time == s.currentTime && !s.events[i].Done {
			if err := s.events[i].Action(); err != nil {
				return fmt.Errorf("scheduled event %s failed: %w", s.events[i].ID, err)
			}
			s.events[i].Done = true
		}
	}

	return nil
}

// Advance moves time forward by n frames, executing all due events.
func (s *Scheduler) Advance(frames int) error {
	for i := 0; i < frames; i++ {
		if err := s.Tick(); err != nil {
			return err
		}
	}
	return nil
}

// CurrentTime returns the current scheduler time.
func (s *Scheduler) CurrentTime() int64 {
	return s.currentTime
}

// Step advances execution by one step (for manual stepping).
func (r *Runner) Step() error {
	if r.scenario == nil {
		return fmt.Errorf("no scenario loaded")
	}

	if r.currentStep >= len(r.scenario.Actions) {
		return fmt.Errorf("all steps completed")
	}

	action := r.scenario.Actions[r.currentStep]
	if err := r.executeAction(action); err != nil {
		return err
	}

	r.currentStep++
	r.updateProgress()

	return nil
}

// BackgroundJobHandler handles version-safe background work.
type BackgroundJobHandler struct {
	version int64
	runner  *Runner
}

// NewBackgroundJobHandler creates a job handler for the current execution version.
func (r *Runner) NewBackgroundJobHandler() *BackgroundJobHandler {
	return &BackgroundJobHandler{
		version: time.Now().UnixNano(),
		runner:  r,
	}
}

// CommitResult commits a background job result if the version matches.
func (h *BackgroundJobHandler) CommitResult(result job.AnyResult) error {
	if h.runner.ctx.Err() != nil {
		return context.Canceled
	}

	// Check if this is still the current execution
	// If the runner has moved on, discard stale results

	// TODO: Implement version checking against current execution

	return nil
}

// CheckpointGate blocks until the checkpoint can advance.
type CheckpointGate struct {
	runner *Runner
	step   int
}

// NewCheckpointGate creates a checkpoint gate at the current step.
func (r *Runner) NewCheckpointGate() *CheckpointGate {
	return &CheckpointGate{
		runner: r,
		step:   r.currentStep,
	}
}

// Wait blocks until the checkpoint is allowed to advance.
func (g *CheckpointGate) Wait(ctx context.Context) error {
	// Wait for:
	// 1. All background jobs for this step to complete
	// 2. Scene to be in stable state
	// 3. No pending commits

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check runner context if available
		if g.runner.ctx != nil {
			select {
			case <-g.runner.ctx.Done():
				return context.Canceled
			default:
			}
		}

		if g.runner.isIdle() {
			return nil
		}

		time.Sleep(16 * time.Millisecond)
	}
}
