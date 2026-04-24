// Package model defines the core types for UI Replay scenarios, actions, assertions,
// and execution results. It provides validation, cloning, and schema compatibility
// checks to ensure reliable replay execution across different versions.
//
// Schema Compatibility:
//
// The current schema version is "1.0". Scenarios with supported schema versions
// are accepted; others are rejected during validation. When loading scenarios
// from external sources, use SchemaMigrator to convert older schemas before
// validation.
//
// Basic usage:
//
//	scenario := &model.Scenario{
//	    ID:          "my.scenario",
//	    DisplayName: "My Scenario",
//	    Schema:      model.SchemaVersion,
//	    Actions:     []model.Action{{Type: model.ActionWaitFrames}},
//	}
//	if err := scenario.Validate(); err != nil {
//	    // handle validation error
//	}
package model

import (
	"fmt"
	"strings"
	"time"
)

// SchemaVersion is the current scenario schema version.
const SchemaVersion = "1.0"

// SupportedSchemaVersions lists all supported schema versions for backward compatibility.
var SupportedSchemaVersions = []string{"1.0"}

// SchemaMigrator converts scenarios between schema versions.
// It provides forward compatibility by migrating older schemas to the current version.
type SchemaMigrator struct {
	SourceVersion string
	TargetVersion string
}

// NewSchemaMigrator creates a migrator for the given source version.
// Returns nil if no migration is needed or if the source version is unsupported.
func NewSchemaMigrator(sourceVersion string) *SchemaMigrator {
	if sourceVersion == SchemaVersion {
		return nil // No migration needed
	}
	if !isSupportedSchema(sourceVersion) {
		return nil // Unsupported source version
	}
	return &SchemaMigrator{
		SourceVersion: sourceVersion,
		TargetVersion: SchemaVersion,
	}
}

// MigrateScenario converts a scenario to the target schema version.
// It updates the schema field and applies any necessary transformations.
func (m *SchemaMigrator) MigrateScenario(s *Scenario) error {
	if m == nil || s == nil {
		return nil
	}
	if s.Schema == m.TargetVersion {
		return nil // Already at target version
	}

	// Apply version-specific migrations
	switch m.SourceVersion {
	case "1.0":
		// No migrations needed - 1.0 is the base version
	default:
		return fmt.Errorf("no migration path from version %s to %s", m.SourceVersion, m.TargetVersion)
	}

	s.Schema = m.TargetVersion
	return nil
}

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

var supportedCapabilities = map[Capability]struct{}{
	CapSceneLoad:      {},
	CapThemeSwitch:    {},
	CapDensitySwitch:  {},
	CapPointerInput:   {},
	CapKeyboardInput:  {},
	CapTextInput:      {},
	CapIME:            {},
	CapScreenshots:    {},
	CapAssertions:     {},
	CapBackgroundJobs: {},
}

// ScenarioID uniquely identifies a scenario.
type ScenarioID string

// Scenario represents a declarative replay scenario.
type Scenario struct {
	ID            ScenarioID     `json:"id"`
	DisplayName   string         `json:"display_name"`
	Schema        string         `json:"schema"`
	Family        string         `json:"family,omitempty"`
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

// NewFixtureScenario returns a registry-safe scenario scaffold for tests and fixtures.
// It fills in the required schema fields so callers can focus on the fields under test.
func NewFixtureScenario(id ScenarioID, displayName string) Scenario {
	return Scenario{
		ID:          id,
		DisplayName: displayName,
		Schema:      SchemaVersion,
	}
}

// Clone returns a deep copy of the scenario so callers can mutate the copy safely.
func (s *Scenario) Clone() *Scenario {
	if s == nil {
		return nil
	}

	clone := *s
	clone.Environment = s.Environment.Clone()
	clone.Actions = cloneActions(s.Actions)
	clone.Assertions = cloneAssertions(s.Assertions)
	clone.Artifacts = cloneArtifacts(s.Artifacts)
	clone.Tags = append([]string(nil), s.Tags...)
	clone.Capabilities = append([]Capability(nil), s.Capabilities...)
	clone.ExpectedState = cloneExpectedState(s.ExpectedState)
	return &clone
}

// Clone returns a copy of the environment.
func (e Environment) Clone() Environment {
	return Environment{
		Theme:      e.Theme,
		Density:    e.Density,
		Backend:    e.Backend,
		Platform:   e.Platform,
		WindowSize: e.WindowSize,
	}
}

func cloneActions(actions []Action) []Action {
	if len(actions) == 0 {
		return nil
	}
	out := make([]Action, len(actions))
	for i, action := range actions {
		out[i] = action
		out[i].Params = cloneActionParams(action.Params)
		out[i].Target = cloneTarget(action.Target)
	}
	return out
}

func cloneAssertions(assertions []Assertion) []Assertion {
	if len(assertions) == 0 {
		return nil
	}
	out := make([]Assertion, len(assertions))
	for i, assertion := range assertions {
		out[i] = assertion
		out[i].Params = cloneAssertionParams(assertion.Params)
	}
	return out
}

func cloneArtifacts(artifacts []ArtifactSpec) []ArtifactSpec {
	if len(artifacts) == 0 {
		return nil
	}
	out := make([]ArtifactSpec, len(artifacts))
	copy(out, artifacts)
	return out
}

func cloneTarget(target Target) Target {
	clone := target
	if target.Fallback != nil {
		fallback := cloneTarget(*target.Fallback)
		clone.Fallback = &fallback
	}
	return clone
}

func cloneExpectedState(state *ExpectedState) *ExpectedState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.ControlStates != nil {
		clone.ControlStates = make(map[string]string, len(state.ControlStates))
		for key, value := range state.ControlStates {
			clone.ControlStates[key] = value
		}
	}
	return &clone
}

func cloneActionParams(params ActionParams) ActionParams {
	if params == nil {
		return nil
	}
	out := make(ActionParams, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}

func cloneAssertionParams(params AssertionParams) AssertionParams {
	if params == nil {
		return nil
	}
	out := make(AssertionParams, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}

// ExpectedState describes the expected scene/control state for validation.
type ExpectedState struct {
	SceneID       string            `json:"scene_id,omitempty"`
	ControlStates map[string]string `json:"control_states,omitempty"`
	Theme         string            `json:"theme,omitempty"`
	Density       string            `json:"density,omitempty"`
	FocusTarget   string            `json:"focus_target,omitempty"`
}

// String returns a human-readable summary of the expected state.
func (e *ExpectedState) String() string {
	if e == nil {
		return "<nil>"
	}
	parts := []string{}
	if e.SceneID != "" {
		parts = append(parts, fmt.Sprintf("scene=%s", e.SceneID))
	}
	if e.Theme != "" {
		parts = append(parts, fmt.Sprintf("theme=%s", e.Theme))
	}
	if e.Density != "" {
		parts = append(parts, fmt.Sprintf("density=%s", e.Density))
	}
	if e.FocusTarget != "" {
		parts = append(parts, fmt.Sprintf("focus=%s", e.FocusTarget))
	}
	if len(e.ControlStates) > 0 {
		parts = append(parts, fmt.Sprintf("%d controls", len(e.ControlStates)))
	}
	if len(parts) == 0 {
		return "<empty>"
	}
	return strings.Join(parts, ", ")
}

// IsEmpty returns true if no expected state is defined.
func (e *ExpectedState) IsEmpty() bool {
	if e == nil {
		return true
	}
	return e.SceneID == "" && e.Theme == "" && e.Density == "" &&
		e.FocusTarget == "" && len(e.ControlStates) == 0
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

// String returns a human-readable summary of the environment.
func (e Environment) String() string {
	return fmt.Sprintf("%s/%s/%s/%s (%dx%d)",
		e.Backend, e.Platform, e.Theme, e.Density,
		e.WindowSize.Width, e.WindowSize.Height)
}

// IsEmpty returns true if no environment fields are set.
func (e Environment) IsEmpty() bool {
	return e.Theme == "" && e.Density == "" && e.Backend == "" &&
		e.Platform == "" && e.WindowSize.Width == 0 && e.WindowSize.Height == 0
}

// Validate checks the environment for valid values.
// Returns nil for empty environments (which inherit defaults at runtime).
func (e Environment) Validate() error {
	if e.IsEmpty() {
		return nil
	}

	// Validate window size if set
	if e.WindowSize.Width < 0 || e.WindowSize.Height < 0 {
		return ValidationError{
			Field:   "environment.window_size",
			Message: fmt.Sprintf("window dimensions must be positive, got %dx%d", e.WindowSize.Width, e.WindowSize.Height),
		}
	}

	return nil
}

// DisplayString returns a concise display string for UI presentation.
func (e Environment) DisplayString() string {
	parts := []string{}
	if e.Backend != "" {
		parts = append(parts, e.Backend)
	}
	if e.Platform != "" {
		parts = append(parts, e.Platform)
	}
	if e.Theme != "" {
		parts = append(parts, e.Theme)
	}
	if e.Density != "" {
		parts = append(parts, e.Density)
	}
	if len(parts) == 0 {
		return "default"
	}
	return fmt.Sprintf("%s", strings.Join(parts, "/"))
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

// String returns a human-readable summary of the action.
func (a Action) String() string {
	if a.Target.IsEmpty() {
		return string(a.Type)
	}
	if a.Target.LogicalID != "" {
		return fmt.Sprintf("%s[%s]", a.Type, a.Target.LogicalID)
	}
	return fmt.Sprintf("%s[%.1f,%.1f]", a.Type, a.Target.X, a.Target.Y)
}

// String returns a human-readable summary of the target.
func (t Target) String() string {
	if t.IsEmpty() {
		if t.Fallback != nil {
			return fmt.Sprintf("fallback(%s)", t.Fallback.String())
		}
		return "<empty>"
	}
	if t.LogicalID != "" {
		return t.LogicalID
	}
	return fmt.Sprintf("(%.1f,%.1f)", t.X, t.Y)
}

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

// String returns a human-readable summary of the assertion.
func (a Assertion) String() string {
	return string(a.Type)
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

// String returns a human-readable summary of the artifact spec.
func (a ArtifactSpec) String() string {
	if a.Name != "" {
		return fmt.Sprintf("%s:%s", a.Type, a.Name)
	}
	return string(a.Type)
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
	if err := s.validateCapabilities(); err != nil {
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

func (s *Scenario) validateCapabilities() error {
	seen := make(map[Capability]bool)
	for i, capability := range s.Capabilities {
		if capability == "" {
			return ValidationError{
				Field:   "capabilities",
				Message: "missing capability",
				Step:    i + 1,
			}
		}
		if _, ok := supportedCapabilities[capability]; !ok {
			return ValidationError{
				Field:   "capabilities",
				Message: fmt.Sprintf("unsupported capability: %q", capability),
				Step:    i + 1,
			}
		}
		if seen[capability] {
			return ValidationError{
				Field:   "capabilities",
				Message: fmt.Sprintf("duplicate capability: %q", capability),
				Step:    i + 1,
			}
		}
		seen[capability] = true
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

// HasFamily reports whether the scenario belongs to the requested family.
func (s *Scenario) HasFamily(family string) bool {
	if s == nil || family == "" {
		return false
	}
	if s.Family == family {
		return true
	}
	if s.RequiredScene == family {
		return true
	}
	for _, tag := range s.Tags {
		if tag == family {
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
// The status transitions are:
//
//	pending → running → passed
//	              ↓
//	        failed/error/cancelled
type ExecutionStatus string

const (
	// StatusPending indicates the scenario is ready to run but not yet started.
	StatusPending ExecutionStatus = "pending"
	// StatusRunning indicates the scenario is currently executing.
	StatusRunning ExecutionStatus = "running"
	// StatusPassed indicates all actions and assertions completed successfully.
	StatusPassed ExecutionStatus = "passed"
	// StatusFailed indicates one or more assertions failed.
	StatusFailed ExecutionStatus = "failed"
	// StatusError indicates an unexpected error occurred during execution.
	StatusError ExecutionStatus = "error"
	// StatusCancelled indicates the execution was cancelled by the user.
	StatusCancelled ExecutionStatus = "cancelled"
)

// String returns the string representation of the status.
func (e ExecutionStatus) String() string {
	return string(e)
}

// IsTerminal returns true if the status represents a completed execution.
func (e ExecutionStatus) IsTerminal() bool {
	switch e {
	case StatusPassed, StatusFailed, StatusError, StatusCancelled:
		return true
	default:
		return false
	}
}

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

// String returns a human-readable summary of the run result.
func (r RunResult) String() string {
	passed := 0
	failed := 0
	for _, ar := range r.AssertionResults {
		if ar.Passed {
			passed++
		} else {
			failed++
		}
	}
	return fmt.Sprintf("%s: %s (%d/%d steps, %d passed, %d failed assertions, %v duration)",
		r.ScenarioID, r.Status, r.StepsExecuted, r.StepsTotal, passed, failed, r.Duration())
}

// AssertionResult represents a single assertion outcome.
type AssertionResult struct {
	Step   int           `json:"step"`
	Type   AssertionType `json:"type"`
	Passed bool          `json:"passed"`
	Reason string        `json:"reason,omitempty"`
}

// String returns a human-readable summary of the assertion result.
func (ar AssertionResult) String() string {
	status := "PASS"
	if !ar.Passed {
		status = "FAIL"
	}
	if ar.Reason != "" {
		return fmt.Sprintf("[%s] step %d %s: %s", status, ar.Step, ar.Type, ar.Reason)
	}
	return fmt.Sprintf("[%s] step %d %s", status, ar.Step, ar.Type)
}

// Duration returns the run duration.
func (r *RunResult) Duration() time.Duration {
	if r.EndTime.IsZero() {
		return time.Since(r.StartTime)
	}
	return r.EndTime.Sub(r.StartTime)
}
