package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/ui_replay/model"
)

// LoadResult tracks the outcome of loading a scenario file.
type LoadResult struct {
	Path    string `json:"path"`
	Status  string `json:"status"` // "loaded", "invalid", "error"
	Error   string `json:"error,omitempty"`
	ID      string `json:"id,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// RegistryPaths holds the configured directory paths.
type RegistryPaths struct {
	ScenarioDir string
	HistoryDir  string
	ExportDir   string
}

var (
	paths     RegistryPaths
	pathsOnce sync.Once
	pathsErr  error
)

// InitRegistry initializes the registry with directory paths.
func InitRegistry(scenarioDir, historyDir, exportDir string) error {
	pathsOnce.Do(func() {
		paths = RegistryPaths{
			ScenarioDir: scenarioDir,
			HistoryDir:  historyDir,
			ExportDir:   exportDir,
		}

		for _, dir := range []string{scenarioDir, historyDir, exportDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				pathsErr = fmt.Errorf("failed to create directory %q: %w", dir, err)
				return
			}
		}

		if err := loadScenarios(); err != nil {
			pathsErr = err
		}
	})
	return pathsErr
}

// GetPaths returns the configured paths.
func GetPaths() RegistryPaths {
	return paths
}

// ScenarioRegistryStore holds all loaded scenarios.
var ScenarioRegistryStore = store.NewValueStore[*ScenarioRegistry](nil)

// ScenarioRegistry manages the collection of scenarios.
type ScenarioRegistry struct {
	scenarios   map[model.ScenarioID]*model.Scenario
	loadResults []LoadResult
	mu          sync.RWMutex
}

// NewScenarioRegistry creates an empty registry.
func NewScenarioRegistry() *ScenarioRegistry {
	return &ScenarioRegistry{
		scenarios:   make(map[model.ScenarioID]*model.Scenario),
		loadResults: make([]LoadResult, 0),
	}
}

// LoadResults returns the detailed load results.
func (r *ScenarioRegistry) LoadResults() []LoadResult {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]LoadResult, len(r.loadResults))
	copy(result, r.loadResults)
	return result
}

// ValidCount returns the number of valid (loaded) scenarios.
func (r *ScenarioRegistry) ValidCount() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, lr := range r.loadResults {
		if lr.Status == "loaded" {
			count++
		}
	}
	return count
}

// InvalidCount returns the number of invalid scenarios.
func (r *ScenarioRegistry) InvalidCount() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, lr := range r.loadResults {
		if lr.Status == "invalid" {
			count++
		}
	}
	return count
}

// Add adds a scenario to the registry.
func (r *ScenarioRegistry) Add(s *model.Scenario) error {
	if r == nil {
		return fmt.Errorf("cannot add to nil registry")
	}
	if s == nil {
		return fmt.Errorf("cannot add nil scenario")
	}
	if err := s.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenarios[s.ID] = s
	return nil
}

// Get retrieves a scenario by ID.
func (r *ScenarioRegistry) Get(id model.ScenarioID) (*model.Scenario, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.scenarios[id]
	return s, ok
}

// All returns all scenarios sorted by display name.
func (r *ScenarioRegistry) All() []*model.Scenario {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []*model.Scenario
	for _, s := range r.scenarios {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].DisplayName < list[j].DisplayName
	})
	return list
}

// Count returns the number of scenarios.
func (r *ScenarioRegistry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.scenarios)
}

// loadScenarios scans the scenario directory and loads all .json files.
func loadScenarios() error {
	registry := NewScenarioRegistry()

	entries, err := os.ReadDir(paths.ScenarioDir)
	if err != nil {
		if os.IsNotExist(err) {
			ScenarioRegistryStore.Set(registry)
			return nil
		}
		return fmt.Errorf("failed to read scenario directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(paths.ScenarioDir, entry.Name())
		result := LoadResult{Path: path}

		data, err := os.ReadFile(path)
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("read file: %v", err)
			registry.addResult(result)
			continue
		}

		var scenario model.Scenario
		if err := json.Unmarshal(data, &scenario); err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("parse JSON: %v", err)
			registry.addResult(result)
			continue
		}

		if err := registry.Add(&scenario); err != nil {
			result.Status = "invalid"
			result.Error = err.Error()
			if scenario.ID != "" {
				result.ID = string(scenario.ID)
			}
			registry.addResult(result)
			continue
		}

		result.Status = "loaded"
		result.ID = string(scenario.ID)
		result.Summary = scenario.Summary()
		registry.addResult(result)
	}

	ScenarioRegistryStore.Set(registry)
	return nil
}

func (r *ScenarioRegistry) addResult(result LoadResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadResults = append(r.loadResults, result)
}

// SelectedScenarioStore holds the currently selected scenario ID.
var SelectedScenarioStore = store.NewValueStore[model.ScenarioID]("")

// SelectedScenario returns the currently selected scenario, if any.
func SelectedScenario() (*model.Scenario, bool) {
	id := SelectedScenarioStore.Get()
	if id == "" {
		return nil, false
	}
	reg := ScenarioRegistryStore.Get()
	if reg == nil {
		return nil, false
	}
	return reg.Get(id)
}

// SelectScenario sets the selected scenario by ID.
func SelectScenario(id model.ScenarioID) {
	SelectedScenarioStore.Set(id)
}

// ClearSelection clears the current scenario selection.
func ClearSelection() {
	SelectedScenarioStore.Set("")
}
