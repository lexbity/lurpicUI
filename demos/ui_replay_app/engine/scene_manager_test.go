package engine

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
)

func TestSceneManager_TransitionScene(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	manager := runner.NewSceneManager()

	t.Run("transition records correctly", func(t *testing.T) {
		err := manager.TransitionScene("scene1", 1)
		if err != nil {
			t.Errorf("TransitionScene() error = %v", err)
		}

		history := manager.SceneHistory()
		if len(history) != 1 {
			t.Errorf("Expected 1 transition, got %d", len(history))
		}
		if history[0].To != "scene1" {
			t.Errorf("Transition.To = %v, want scene1", history[0].To)
		}
	})

	t.Run("current scene updates", func(t *testing.T) {
		manager.TransitionScene("scene2", 2)

		if manager.CurrentScene() != "scene2" {
			t.Errorf("CurrentScene() = %v, want scene2", manager.CurrentScene())
		}
	})

	t.Run("multi-scene detection", func(t *testing.T) {
		if !manager.IsMultiScene() {
			t.Error("IsMultiScene() should be true after multiple transitions")
		}

		visited := manager.GetScenesVisited()
		if len(visited) < 2 {
			t.Errorf("Expected at least 2 scenes visited, got %d", len(visited))
		}
	})
}

func TestSceneManager_ThemeAndDensity(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	manager := runner.NewSceneManager()

	t.Run("theme change records", func(t *testing.T) {
		manager.ChangeTheme("dark", 1)
		manager.ChangeTheme("light", 3)

		history := manager.ThemeHistory()
		if len(history) != 2 {
			t.Errorf("Expected 2 theme changes, got %d", len(history))
		}
		if history[0].To != "dark" {
			t.Errorf("First theme = %v, want dark", history[0].To)
		}
	})

	t.Run("current theme returns last", func(t *testing.T) {
		if manager.CurrentTheme() != "light" {
			t.Errorf("CurrentTheme() = %v, want light", manager.CurrentTheme())
		}
	})

	t.Run("density change records", func(t *testing.T) {
		manager.ChangeDensity("compact", 2)
		manager.ChangeDensity("comfortable", 4)

		history := manager.DensityHistory()
		if len(history) != 2 {
			t.Errorf("Expected 2 density changes, got %d", len(history))
		}
	})

	t.Run("current density returns last", func(t *testing.T) {
		if manager.CurrentDensity() != "comfortable" {
			t.Errorf("CurrentDensity() = %v, want comfortable", manager.CurrentDensity())
		}
	})
}

func TestSceneManager_GetStateAtStep(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	manager := runner.NewSceneManager()

	// Setup transitions
	manager.TransitionScene("scene1", 1)
	manager.ChangeTheme("dark", 2)
	manager.ChangeDensity("compact", 2)
	manager.TransitionScene("scene2", 5)
	manager.ChangeTheme("light", 6)

	t.Run("scene at step", func(t *testing.T) {
		if manager.GetSceneAtStep(1) != "scene1" {
			t.Error("Step 1 should be scene1")
		}
		if manager.GetSceneAtStep(3) != "scene1" {
			t.Error("Step 3 should still be scene1")
		}
		if manager.GetSceneAtStep(5) != "scene2" {
			t.Error("Step 5 should be scene2")
		}
	})

	t.Run("theme at step", func(t *testing.T) {
		if manager.GetThemeAtStep(1) != "" {
			t.Error("Step 1 should have no theme")
		}
		if manager.GetThemeAtStep(3) != "dark" {
			t.Error("Step 3 should have dark theme")
		}
		if manager.GetThemeAtStep(6) != "light" {
			t.Error("Step 6 should have light theme")
		}
	})

	t.Run("phase info", func(t *testing.T) {
		info := manager.GetPhaseInfo(3)
		if info.Scene != "scene1" {
			t.Errorf("PhaseInfo.Scene = %v, want scene1", info.Scene)
		}
		if info.Theme != "dark" {
			t.Errorf("PhaseInfo.Theme = %v, want dark", info.Theme)
		}
		if info.Density != "compact" {
			t.Errorf("PhaseInfo.Density = %v, want compact", info.Density)
		}
	})
}

func TestSceneManager_Reset(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	manager := runner.NewSceneManager()

	manager.TransitionScene("scene1", 1)
	manager.ChangeTheme("dark", 1)
	manager.ChangeDensity("compact", 1)

	manager.Reset()

	if manager.CurrentScene() != "" {
		t.Error("CurrentScene should be empty after reset")
	}
	if manager.CurrentTheme() != "" {
		t.Error("CurrentTheme should be empty after reset")
	}
	if manager.CurrentDensity() != "" {
		t.Error("CurrentDensity should be empty after reset")
	}
	if len(manager.SceneHistory()) != 0 {
		t.Error("SceneHistory should be empty after reset")
	}
}

func TestSceneManager_ReResolveTarget(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	manager := runner.NewSceneManager()

	manager.TransitionScene("basic", 1)

	t.Run("re-resolve returns target", func(t *testing.T) {
		target := model.Target{LogicalID: "button.ok"}
		resolved, err := manager.ReResolveTarget(target)
		if err != nil {
			t.Errorf("ReResolveTarget() error = %v", err)
		}
		if resolved.LogicalID != "button.ok" {
			t.Errorf("Resolved LogicalID = %v, want button.ok", resolved.LogicalID)
		}
	})
}

func TestSceneManager_NoRunner(t *testing.T) {
	manager := &SceneManager{}

	err := manager.TransitionScene("scene", 1)
	if err == nil {
		t.Error("TransitionScene should error when no runner bound")
	}
}
