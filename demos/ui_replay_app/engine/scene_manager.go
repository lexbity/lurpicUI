package engine

import (
	"fmt"
	"sync"

	"codeburg.org/lexbit/ui_replay/model"
)

// SceneManager handles multi-scene transitions and state tracking.
type SceneManager struct {
	runner        *Runner
	currentScene  string
	sceneHistory  []SceneTransition
	themeHistory  []ThemeChange
	densityHistory []DensityChange
	mu            sync.RWMutex
}

// SceneTransition records a scene change.
type SceneTransition struct {
	From      string
	To        string
	Step      int
	Frame     int
}

// ThemeChange records a theme change.
type ThemeChange struct {
	From   string
	To     string
	Step   int
	Frame  int
}

// DensityChange records a density change.
type DensityChange struct {
	From   string
	To     string
	Step   int
	Frame  int
}

// NewSceneManager creates a new scene manager.
func (r *Runner) NewSceneManager() *SceneManager {
	return &SceneManager{
		runner:         r,
		sceneHistory:   make([]SceneTransition, 0),
		themeHistory:   make([]ThemeChange, 0),
		densityHistory: make([]DensityChange, 0),
	}
}

// TransitionScene records a scene transition.
func (sm *SceneManager) TransitionScene(to string, step int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.runner == nil {
		return fmt.Errorf("scene manager not bound to runner")
	}

	transition := SceneTransition{
		From:  sm.currentScene,
		To:    to,
		Step:  step,
		Frame: sm.runner.frameCounter,
	}

	sm.sceneHistory = append(sm.sceneHistory, transition)
	sm.currentScene = to

	// Log the transition
	if sm.runner.logger != nil {
		sm.runner.logger.Info("scene", fmt.Sprintf("Transitioned from %s to %s", transition.From, to),
			map[string]interface{}{
				"from_scene": transition.From,
				"to_scene":   to,
				"step":       step,
				"frame":      transition.Frame,
			})
	}

	// Re-resolve any pending targets after scene change
	// This ensures targets are valid for the new scene
	return nil
}

// ChangeTheme records a theme change.
func (sm *SceneManager) ChangeTheme(to string, step int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var from string
	if len(sm.themeHistory) > 0 {
		from = sm.themeHistory[len(sm.themeHistory)-1].To
	}

	change := ThemeChange{
		From:  from,
		To:    to,
		Step:  step,
		Frame: sm.runner.frameCounter,
	}

	sm.themeHistory = append(sm.themeHistory, change)

	if sm.runner.logger != nil {
		sm.runner.logger.Info("theme", fmt.Sprintf("Changed from %s to %s", from, to),
			map[string]interface{}{
				"from_theme": from,
				"to_theme":   to,
				"step":       step,
			})
	}
}

// ChangeDensity records a density change.
func (sm *SceneManager) ChangeDensity(to string, step int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var from string
	if len(sm.densityHistory) > 0 {
		from = sm.densityHistory[len(sm.densityHistory)-1].To
	}

	change := DensityChange{
		From:  from,
		To:    to,
		Step:  step,
		Frame: sm.runner.frameCounter,
	}

	sm.densityHistory = append(sm.densityHistory, change)

	if sm.runner.logger != nil {
		sm.runner.logger.Info("density", fmt.Sprintf("Changed from %s to %s", from, to),
			map[string]interface{}{
				"from_density": from,
				"to_density":   to,
				"step":         step,
			})
	}
}

// CurrentScene returns the current scene ID.
func (sm *SceneManager) CurrentScene() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentScene
}

// CurrentTheme returns the current theme.
func (sm *SceneManager) CurrentTheme() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.themeHistory) == 0 {
		return ""
	}
	return sm.themeHistory[len(sm.themeHistory)-1].To
}

// CurrentDensity returns the current density.
func (sm *SceneManager) CurrentDensity() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.densityHistory) == 0 {
		return ""
	}
	return sm.densityHistory[len(sm.densityHistory)-1].To
}

// SceneHistory returns all scene transitions.
func (sm *SceneManager) SceneHistory() []SceneTransition {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]SceneTransition, len(sm.sceneHistory))
	copy(result, sm.sceneHistory)
	return result
}

// ThemeHistory returns all theme changes.
func (sm *SceneManager) ThemeHistory() []ThemeChange {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]ThemeChange, len(sm.themeHistory))
	copy(result, sm.themeHistory)
	return result
}

// DensityHistory returns all density changes.
func (sm *SceneManager) DensityHistory() []DensityChange {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]DensityChange, len(sm.densityHistory))
	copy(result, sm.densityHistory)
	return result
}

// HasSceneChanged returns true if the scene has changed from initial.
func (sm *SceneManager) HasSceneChanged() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sceneHistory) > 0
}

// GetSceneAtStep returns the active scene at a given step.
func (sm *SceneManager) GetSceneAtStep(step int) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	current := ""
	for _, transition := range sm.sceneHistory {
		if transition.Step <= step {
			current = transition.To
		}
	}
	return current
}

// GetThemeAtStep returns the active theme at a given step.
func (sm *SceneManager) GetThemeAtStep(step int) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	current := ""
	for _, change := range sm.themeHistory {
		if change.Step <= step {
			current = change.To
		}
	}
	return current
}

// GetDensityAtStep returns the active density at a given step.
func (sm *SceneManager) GetDensityAtStep(step int) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	current := ""
	for _, change := range sm.densityHistory {
		if change.Step <= step {
			current = change.To
		}
	}
	return current
}

// ReResolveTarget re-resolves a target for the current scene.
// This should be called after scene transitions to ensure targets are valid.
func (sm *SceneManager) ReResolveTarget(target model.Target) (model.Target, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.currentScene == "" {
		return target, nil // No scene context, return original
	}

	// TODO: Implement actual target re-resolution against current scene
	// For now, return the resolved target as-is
	resolved := target.Resolve()

	if sm.runner.logger != nil {
		sm.runner.logger.Debug("target", fmt.Sprintf("Re-resolved target for scene %s", sm.currentScene),
			map[string]interface{}{
				"logical_id": resolved.LogicalID,
				"scene":      sm.currentScene,
			})
	}

	return resolved, nil
}

// Reset clears all history.
func (sm *SceneManager) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.currentScene = ""
	sm.sceneHistory = make([]SceneTransition, 0)
	sm.themeHistory = make([]ThemeChange, 0)
	sm.densityHistory = make([]DensityChange, 0)
}

// PhaseInfo captures the presentation state at a specific phase.
type PhaseInfo struct {
	Step    int
	Scene   string
	Theme   string
	Density string
	Frame   int
}

// GetPhaseInfo returns the complete presentation state at a given step.
func (sm *SceneManager) GetPhaseInfo(step int) PhaseInfo {
	return PhaseInfo{
		Step:    step,
		Scene:   sm.GetSceneAtStep(step),
		Theme:   sm.GetThemeAtStep(step),
		Density: sm.GetDensityAtStep(step),
		Frame:   sm.runner.frameCounter,
	}
}

// MultiSceneRun represents a run that spans multiple scenes.
type MultiSceneRun struct {
	ScenarioID    string
	ScenesVisited []string
	Transitions   []SceneTransition
	RunState      RunState
}

// IsMultiScene returns true if the run visited multiple scenes.
func (sm *SceneManager) IsMultiScene() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.sceneHistory) == 0 {
		return false
	}

	// Check if any transitions went to different scenes
	visited := make(map[string]bool)
	for _, t := range sm.sceneHistory {
		visited[t.To] = true
	}
	return len(visited) > 1
}

// GetScenesVisited returns the list of unique scenes visited.
func (sm *SceneManager) GetScenesVisited() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	visited := make(map[string]bool)
	for _, t := range sm.sceneHistory {
		visited[t.To] = true
	}

	result := make([]string, 0, len(visited))
	for scene := range visited {
		result = append(result, scene)
	}
	return result
}
