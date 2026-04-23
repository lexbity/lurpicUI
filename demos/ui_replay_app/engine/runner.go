package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// Runner executes scenarios in a deterministic manner.
type Runner struct {
	ctx           context.Context
	cancel        context.CancelFunc
	runtime       *runtime.Runtime
	root          *facet.Facet
	mu            sync.RWMutex
	state         RunState
	currentStep   int
	totalSteps    int
	scenario      *model.Scenario
	onStep        func(step int, total int, action model.Action)
	onComplete    func(result *model.RunResult)
	onError       func(err error)
	frameCounter  int
	idleStartTime time.Time
	logger        *SemanticLogger
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

	// Apply environment
	if err := r.applyEnvironment(&scenario.Environment); err != nil {
		result.Status = model.StatusError
		result.Error = fmt.Sprintf("environment application failed: %v", err)
		return r.finishRun(result, model.StatusError, StateFailed, 0)
	}

	// Reset scene to canonical state
	if err := r.resetScene(); err != nil {
		result.Status = model.StatusError
		result.Error = fmt.Sprintf("scene reset failed: %v", err)
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
		if r.onStep != nil {
			r.onStep(r.currentStep, r.totalSteps, action)
		}

		if err := r.executeAction(action); err != nil {
			if err == context.Canceled {
				result.StepsExecuted = r.currentStep
				return r.finishRun(result, model.StatusCancelled, StateCancelled, r.currentStep)
			}
			result.Status = model.StatusFailed
			result.Error = fmt.Sprintf("step %d: %v", r.currentStep, err)
			result.StepsExecuted = r.currentStep - 1
			return r.finishRun(result, model.StatusFailed, StateFailed, r.currentStep)
		}

		r.updateProgress()
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

	store.ExecutionStateStore.Set(store.ExecutionState{
		Status:        status,
		CurrentStep:   currentStep,
		TotalSteps:    r.totalSteps,
		CurrentAction: "",
		Error:         result.Error,
		Progress:      runProgress(currentStep, r.totalSteps),
	})

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
	// Apply theme if specified
	if env.Theme != "" {
		// TODO: Apply theme through runtime
	}

	// Apply density if specified
	if env.Density != "" {
		// TODO: Apply density through runtime
	}

	// Apply window size if specified
	if env.WindowSize.Width > 0 && env.WindowSize.Height > 0 {
		// TODO: Apply window size through platform
	}

	return nil
}

func (r *Runner) resetScene() error {
	// Reset frame counter for deterministic timing
	r.frameCounter = 0

	// Clear any stale runtime state
	if r.runtime != nil {
		// Clear focus
		// TODO: r.runtime.SetFocus(nil)

		// Clear selection
		// TODO: r.runtime.ClearSelection()

		// Reset animation clocks
		// TODO: r.runtime.ResetAnimationClocks()
	}

	// Reset stores to known state
	// Note: We don't clear the registry, just execution state

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

	r.setState(StateWaiting)
	targetFrame := r.frameCounter + frames

	for r.frameCounter < targetFrame {
		// Check if context is cancelled (handle nil ctx gracefully)
		if r.ctx != nil {
			select {
			case <-r.ctx.Done():
				return context.Canceled
			default:
			}
		}
		// Advance frame
		r.frameCounter++
		// TODO: Wait for actual frame boundary
		time.Sleep(16 * time.Millisecond) // ~60fps
	}

	r.setState(StateRunning)
	return nil
}

func (r *Runner) executeWaitIdle(action model.Action) error {
	timeoutMs := 5000.0 // default 5 seconds
	if t, ok := action.Params["timeout_ms"].(float64); ok {
		timeoutMs = t
	}

	r.setState(StateWaiting)
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	for time.Now().Before(deadline) {
		// Check if context is cancelled (handle nil ctx gracefully)
		if r.ctx != nil {
			select {
			case <-r.ctx.Done():
				return context.Canceled
			default:
			}
		}

		// Check if idle (no pending jobs, animations complete)
		if r.isIdle() {
			r.setState(StateRunning)
			return nil
		}

		time.Sleep(16 * time.Millisecond)
	}

	r.setState(StateRunning)
	return fmt.Errorf("wait_idle: timeout after %v ms", timeoutMs)
}

func (r *Runner) isIdle() bool {
	// Check for pending jobs
	if r.runtime != nil {
		// TODO: Check if job queue has pending work
		// pendingJobs := r.runtime.JobQueue().PendingCount()
		// if pendingJobs > 0 {
		//     return false
		// }
	}

	// Check for running animations
	// TODO: Check animation state - if any animations are active, not idle

	// Check for pending events
	// TODO: Check if event queue is empty

	return true // Placeholder - consider idle until runtime integration
}

func (r *Runner) executeSceneLoad(action model.Action) error {
	scene, ok := action.Params["scene"].(string)
	if !ok {
		return fmt.Errorf("scene_load: missing or invalid 'scene' param")
	}

	// TODO: Trigger scene load through runtime
	_ = scene

	return nil
}

func (r *Runner) executeClick(action model.Action) error {
	target := action.Target.Resolve()
	if target.IsEmpty() {
		return fmt.Errorf("click: missing target")
	}

	// TODO: Dispatch click event to target
	_ = target

	return nil
}

func (r *Runner) executePointerMove(action model.Action) error {
	x, xok := action.Params["x"].(float64)
	y, yok := action.Params["y"].(float64)
	if !xok || !yok {
		return fmt.Errorf("pointer_move: missing or invalid coordinates")
	}

	// TODO: Dispatch pointer move event
	_, _ = x, y

	return nil
}

func (r *Runner) executeScreenshot(action model.Action) error {
	name, _ := action.Params["name"].(string)

	r.setState(StateCapturing)

	// TODO: Capture screenshot through rendering backend
	_ = name

	r.setState(StateRunning)
	return nil
}

func (r *Runner) executeSwitchTheme(action model.Action) error {
	theme, ok := action.Params["theme"].(string)
	if !ok {
		return fmt.Errorf("switch_theme: missing or invalid 'theme' param")
	}

	// TODO: Apply theme
	_ = theme

	return nil
}

func (r *Runner) executeSwitchDensity(action model.Action) error {
	density, ok := action.Params["density"].(string)
	if !ok {
		return fmt.Errorf("switch_density: missing or invalid 'density' param")
	}

	// TODO: Apply density
	_ = density

	return nil
}

func (r *Runner) executeResizeWindow(action model.Action) error {
	width, wok := action.Params["width"].(float64)
	height, hok := action.Params["height"].(float64)
	if !wok || !hok {
		return fmt.Errorf("resize_window: missing or invalid dimensions")
	}

	// TODO: Resize window
	_, _ = width, height

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

	// TODO: Execute drag operation
	_, _, _ = target, destX, destY

	return nil
}

func (r *Runner) executeKeyInput(action model.Action) error {
	key, ok := action.Params["key"].(string)
	if !ok {
		return fmt.Errorf("key_input: missing or invalid 'key' param")
	}

	// TODO: Dispatch key event
	_ = key

	return nil
}

func (r *Runner) executeTextInput(action model.Action) error {
	text, ok := action.Params["text"].(string)
	if !ok {
		return fmt.Errorf("text_input: missing or invalid 'text' param")
	}

	// TODO: Dispatch text input events
	_ = text

	return nil
}

func (r *Runner) executeIMEHook(action model.Action) error {
	actionType, ok := action.Params["action"].(string)
	if !ok {
		return fmt.Errorf("ime_hook: missing or invalid 'action' param")
	}

	// TODO: Execute IME action (activate, deactivate, set_region, etc.)
	_ = actionType

	return nil
}

func (r *Runner) executeAssertState(action model.Action) error {
	r.setState(StateAsserting)
	defer r.setState(StateRunning)

	assertionType, ok := action.Params["type"].(string)
	if !ok {
		return fmt.Errorf("assert_state: missing or invalid 'type' param")
	}

	// TODO: Implement state assertion checking
	_ = assertionType

	return nil
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
