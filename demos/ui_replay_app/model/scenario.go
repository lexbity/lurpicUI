package model

import (
	"fmt"
	"time"
)

// SchemaVersion is the current scenario schema version.
const SchemaVersion = "1.0"

// SupportedSchemaVersions lists all supported schema versions for backward compatibility.
var SupportedSchemaVersions = []string{"1.0"}

// Capability represents a scenario capability declaration.
type Capability string

const (
	CapSceneLoad      Capability = "scene_load"
	CapThemeSwitch    Capability = "theme_switch"
	CapDensitySwitch  Capability = "density_switch"
	CapPointerInput   Capability = "pointer_input"
	CapKeyboardInput  Capability = "keyboard_input"
	CapTextInput      Capability = "text_input"
	CapIME            Capability = "ime"
	CapScreenshots    Capability = "screenshots"
	CapAssertions     Capability = "assertions"
	CapBackgroundJobs Capability = "background_jobs"
)

// ScenarioID uniquely identifies a scenario.
type ScenarioID string

// Scenario represents a declarative replay scenario.
type Scenario struct {
	ID            ScenarioID     `json:"id"`
	DisplayName   string         `json:"display_name"`
	Schema        string         `json:"schema"`
	RequiredScene string         `json:"required_scene,omitempty"`
	Environment   Environment    `json:"environment"`
	Actions       []Action       `json:"actions"`
	Assertions    []Assertion    `json:"assertions"`
	Artifacts     []ArtifactSpec `json:"artifacts"`
	Tags          []string       `json:"tags"`
	Capabilities  []Capability   `json:"capabilities,omitempty"`
	ExpectedState *ExpectedState `json:"expected_state,omitempty"`
	Description   string         `json:"description,omitempty"`
}

// ExpectedState describes the expected scene/control state for validation.
type ExpectedState struct {
	SceneID       string            `json:"scene_id,omitempty"`
	ControlStates map[string]string `json:"control_states,omitempty"`
	Theme         string            `json:"theme,omitempty"`
	Density       string            `json:"density,omitempty"`
	FocusTarget   string            `json:"focus_target,omitempty"`
}

// Environment describes the execution environment.
type Environment struct {
	Theme      string `json:"theme,omitempty"`
	Density    string `json:"density,omitempty"`
	Backend    string `json:"backend,omitempty"`
	Platform   string `json:"platform,omitempty"`
	WindowSize struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"window_size,omitempty"`
}

// ActionType identifies the type of action.
type ActionType string

const (
	ActionSceneLoad     ActionType = "scene_load"
	ActionClick         ActionType = "click"
	ActionPointerMove   ActionType = "pointer_move"
	ActionDrag          ActionType = "drag"
	ActionKeyInput      ActionType = "key_input"
	ActionTextInput     ActionType = "text_input"
	ActionIMEHook       ActionType = "ime_hook"
	ActionWaitFrames    ActionType = "wait_frames"
	ActionWaitIdle      ActionType = "wait_idle"
	ActionSwitchTheme   ActionType = "switch_theme"
	ActionSwitchDensity ActionType = "switch_density"
	ActionResizeWindow  ActionType = "resize_window"
	ActionAssertState   ActionType = "assert_state"
	ActionScreenshot    ActionType = "screenshot"
	ActionExportBundle  ActionType = "export_bundle"
)

// Action represents a single replay action.
type Action struct {
	Type   ActionType   `json:"type"`
	Target Target       `json:"target,omitempty"`
	Params ActionParams `json:"params,omitempty"`
}

// Target identifies the logical target for an action.
// Supports stable logical IDs with coordinate fallback.
type Target struct {
	LogicalID string  `json:"logical_id,omitempty"`
	X         float32 `json:"x,omitempty"`
	Y         float32 `json:"y,omitempty"`
	Fallback  *Target `json:"fallback,omitempty"`
}

// IsEmpty returns true if the target has no primary identification.
// A target with only a fallback is considered empty.
func (t Target) IsEmpty() bool {
	return t.LogicalID == "" && t.X == 0 && t.Y == 0
}

// Resolve returns the primary target or its fallback if empty.
func (t Target) Resolve() Target {
	if t.IsEmpty() && t.Fallback != nil {
		return *t.Fallback
	}
	return t
}

// ActionParams contains action-specific parameters.
type ActionParams map[string]interface{}

// AssertionType identifies the type of assertion.
type AssertionType string

const (
	AssertSceneID       AssertionType = "scene_id"
	AssertControlState  AssertionType = "control_state"
	AssertThemeState    AssertionType = "theme_state"
	AssertDensityState  AssertionType = "density_state"
	AssertFocusOwner    AssertionType = "focus_owner"
	AssertEventPresent  AssertionType = "event_present"
	AssertStoreSummary  AssertionType = "store_summary"
	AssertSignalSummary AssertionType = "signal_summary"
	AssertScreenshot    AssertionType = "screenshot"
	AssertDiagnostics   AssertionType = "diagnostics"
	AssertFrameCount    AssertionType = "frame_count"
)

// Assertion represents a state checkpoint.
type Assertion struct {
	Type   AssertionType   `json:"type"`
	Params AssertionParams `json:"params,omitempty"`
}

// AssertionParams contains assertion-specific parameters.
type AssertionParams map[string]interface{}

// ArtifactType identifies artifact types.
type ArtifactType string

const (
	ArtifactScreenshot  ArtifactType = "screenshot"
	ArtifactLog         ArtifactType = "log"
	ArtifactSceneExport ArtifactType = "scene_export"
	ArtifactDiagnostics ArtifactType = "diagnostics"
)

// ArtifactSpec specifies expected artifacts.
type ArtifactSpec struct {
	Type     ArtifactType `json:"type"`
	Name     string       `json:"name,omitempty"`
	Required bool         `json:"required"`
}

// ValidationError represents a structured validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Step    int    `json:"step,omitempty"`
}

func (v ValidationError) Error() string {
	if v.Step > 0 {
		return fmt.Sprintf("step %d: %s: %s", v.Step, v.Field, v.Message)
	}
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// Validate checks the scenario for validity.
func (s *Scenario) Validate() error {
	if s.ID == "" {
		return ValidationError{Field: "id", Message: "missing scenario ID"}
	}
	if s.DisplayName == "" {
		return ValidationError{Field: "display_name", Message: "missing display name"}
	}
	if s.Schema == "" {
		return ValidationError{Field: "schema", Message: "missing schema version"}
	}
	if !isSupportedSchema(s.Schema) {
		return ValidationError{
			Field:   "schema",
			Message: fmt.Sprintf("unsupported schema version %q (supported: %v)", s.Schema, SupportedSchemaVersions),
		}
	}
	if err := s.validateActions(); err != nil {
		return err
	}
	if err := s.validateAssertions(); err != nil {
		return err
	}
	if err := s.validateArtifacts(); err != nil {
		return err
	}
	return nil
}

func isSupportedSchema(version string) bool {
	for _, v := range SupportedSchemaVersions {
		if v == version {
			return true
		}
	}
	return false
}

func (s *Scenario) validateActions() error {
	for i, action := range s.Actions {
		if err := action.Validate(i + 1); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scenario) validateAssertions() error {
	for i, assertion := range s.Assertions {
		if err := assertion.Validate(i + 1); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scenario) validateArtifacts() error {
	names := make(map[string]bool)
	for i, artifact := range s.Artifacts {
		if err := artifact.Validate(i + 1); err != nil {
			return err
		}
		if artifact.Name != "" {
			if names[artifact.Name] {
				return ValidationError{
					Field:   "artifacts",
					Message: fmt.Sprintf("duplicate artifact name: %q", artifact.Name),
					Step:    i + 1,
				}
			}
			names[artifact.Name] = true
		}
	}
	return nil
}

// Validate checks the action for validity.
func (a Action) Validate(step int) error {
	if a.Type == "" {
		return ValidationError{Field: "action.type", Message: "missing action type", Step: step}
	}
	if !isValidActionType(a.Type) {
		return ValidationError{
			Field:   "action.type",
			Message: fmt.Sprintf("unsupported action type: %q", a.Type),
			Step:    step,
		}
	}
	return nil
}

func isValidActionType(t ActionType) bool {
	validTypes := []ActionType{
		ActionSceneLoad, ActionClick, ActionPointerMove, ActionDrag,
		ActionKeyInput, ActionTextInput, ActionIMEHook, ActionWaitFrames,
		ActionWaitIdle, ActionSwitchTheme, ActionSwitchDensity,
		ActionResizeWindow, ActionAssertState, ActionScreenshot, ActionExportBundle,
	}
	for _, vt := range validTypes {
		if vt == t {
			return true
		}
	}
	return false
}

// Validate checks the assertion for validity.
func (a Assertion) Validate(step int) error {
	if a.Type == "" {
		return ValidationError{Field: "assertion.type", Message: "missing assertion type", Step: step}
	}
	if !isValidAssertionType(a.Type) {
		return ValidationError{
			Field:   "assertion.type",
			Message: fmt.Sprintf("unsupported assertion type: %q", a.Type),
			Step:    step,
		}
	}
	return nil
}

func isValidAssertionType(t AssertionType) bool {
	validTypes := []AssertionType{
		AssertSceneID, AssertControlState, AssertThemeState, AssertDensityState,
		AssertFocusOwner, AssertEventPresent, AssertStoreSummary, AssertSignalSummary,
		AssertScreenshot, AssertDiagnostics, AssertFrameCount,
	}
	for _, vt := range validTypes {
		if vt == t {
			return true
		}
	}
	return false
}

// Validate checks the artifact spec for validity.
func (a ArtifactSpec) Validate(step int) error {
	if a.Type == "" {
		return ValidationError{Field: "artifact.type", Message: "missing artifact type", Step: step}
	}
	if !isValidArtifactType(a.Type) {
		return ValidationError{
			Field:   "artifact.type",
			Message: fmt.Sprintf("unsupported artifact type: %q", a.Type),
			Step:    step,
		}
	}
	return nil
}

func isValidArtifactType(t ArtifactType) bool {
	validTypes := []ArtifactType{
		ArtifactScreenshot, ArtifactLog, ArtifactSceneExport, ArtifactDiagnostics,
	}
	for _, vt := range validTypes {
		if vt == t {
			return true
		}
	}
	return false
}

// HasCapability checks if the scenario declares a specific capability.
func (s *Scenario) HasCapability(cap Capability) bool {
	for _, c := range s.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// Summary returns a human-readable summary of the scenario.
func (s *Scenario) Summary() string {
	return fmt.Sprintf("%s (%s): %d actions, %d assertions, %d artifacts",
		s.DisplayName, s.ID, len(s.Actions), len(s.Assertions), len(s.Artifacts))
}

// ExecutionStatus represents the status of a replay execution.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusPassed    ExecutionStatus = "passed"
	StatusFailed    ExecutionStatus = "failed"
	StatusError     ExecutionStatus = "error"
	StatusCancelled ExecutionStatus = "cancelled"
)

// RunResult contains the outcome of a replay execution.
type RunResult struct {
	ScenarioID       ScenarioID        `json:"scenario_id"`
	Status           ExecutionStatus   `json:"status"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time,omitempty"`
	StepsExecuted    int               `json:"steps_executed"`
	StepsTotal       int               `json:"steps_total"`
	AssertionResults []AssertionResult `json:"assertion_results"`
	Artifacts        []string          `json:"artifacts"`
	Error            string            `json:"error,omitempty"`
}

// AssertionResult represents a single assertion outcome.
type AssertionResult struct {
	Step   int           `json:"step"`
	Type   AssertionType `json:"type"`
	Passed bool          `json:"passed"`
	Reason string        `json:"reason,omitempty"`
}

// Duration returns the run duration.
func (r *RunResult) Duration() time.Duration {
	if r.EndTime.IsZero() {
		return time.Since(r.StartTime)
	}
	return r.EndTime.Sub(r.StartTime)
}
