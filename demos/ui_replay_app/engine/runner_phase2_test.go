package engine

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

func TestRunner_applyEnvironment(t *testing.T) {
	defer store.EnvironmentStore.Set(store.DefaultEnvironment())

	runner := NewRunner(&runtime.Runtime{}, nil)
	runner.sceneManager = runner.NewSceneManager()

	env := &model.Environment{
		Theme:    "dark",
		Density:  "compact",
		Backend:  "vulkan",
		Platform: "linux",
	}
	env.WindowSize.Width = 1024
	env.WindowSize.Height = 768

	if err := runner.applyEnvironment(env); err != nil {
		t.Fatalf("applyEnvironment() error = %v", err)
	}

	got := store.EnvironmentStore.Get()
	if got.Theme != "dark" {
		t.Fatalf("Theme = %q, want dark", got.Theme)
	}
	if got.Density != "compact" {
		t.Fatalf("Density = %q, want compact", got.Density)
	}
	if got.Backend != "vulkan" {
		t.Fatalf("Backend = %q, want vulkan", got.Backend)
	}
	if got.Platform != "linux" {
		t.Fatalf("Platform = %q, want linux", got.Platform)
	}
	if runner.sceneManager.CurrentTheme() != "dark" {
		t.Fatalf("scene theme = %q, want dark", runner.sceneManager.CurrentTheme())
	}
	if runner.sceneManager.CurrentDensity() != "compact" {
		t.Fatalf("scene density = %q, want compact", runner.sceneManager.CurrentDensity())
	}
}

func TestRunner_executeWaitIdleAdvancesToIdleFrame(t *testing.T) {
	runner := NewRunner(&runtime.Runtime{}, nil)
	runner.frameCounter = 0
	runner.idleReadyFrame = 3

	if err := runner.executeWaitIdle(model.Action{Type: model.ActionWaitIdle}); err != nil {
		t.Fatalf("executeWaitIdle() error = %v", err)
	}
	if runner.frameCounter != 3 {
		t.Fatalf("frameCounter = %d, want 3", runner.frameCounter)
	}
}

func TestRunner_executeAssertState(t *testing.T) {
	defer store.EnvironmentStore.Set(store.DefaultEnvironment())

	runner := NewRunner(&runtime.Runtime{}, nil)
	runner.sceneManager = runner.NewSceneManager()
	runner.sceneManager.TransitionScene("basic", 1)
	runner.sceneManager.ChangeTheme("baseline", 0)
	runner.sceneManager.ChangeDensity("default", 0)
	runner.frameCounter = 2

	tests := []struct {
		name   string
		action model.Action
		wantOK bool
	}{
		{
			name: "scene_id",
			action: model.Action{
				Type: model.ActionAssertState,
				Params: model.ActionParams{
					"type":     string(model.AssertSceneID),
					"expected": "basic",
				},
			},
			wantOK: true,
		},
		{
			name: "theme_state",
			action: model.Action{
				Type: model.ActionAssertState,
				Params: model.ActionParams{
					"type":     string(model.AssertThemeState),
					"expected": "baseline",
				},
			},
			wantOK: true,
		},
		{
			name: "density_state",
			action: model.Action{
				Type: model.ActionAssertState,
				Params: model.ActionParams{
					"type":     string(model.AssertDensityState),
					"expected": "default",
				},
			},
			wantOK: true,
		},
		{
			name: "frame_count",
			action: model.Action{
				Type: model.ActionAssertState,
				Params: model.ActionParams{
					"type": string(model.AssertFrameCount),
					"min":  2.0,
					"max":  3.0,
				},
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runner.executeAssertState(tt.action)
			if tt.wantOK && err != nil {
				t.Fatalf("executeAssertState() error = %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Fatalf("executeAssertState() expected error")
			}
		})
	}
}
