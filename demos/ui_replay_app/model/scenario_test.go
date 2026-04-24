package model

import (
	"encoding/json"
	"testing"
)

func TestScenario_Validate(t *testing.T) {
	tests := []struct {
		name     string
		scenario Scenario
		wantErr  bool
		errField string
	}{
		{
			name: "valid scenario",
			scenario: Scenario{
				ID:          "test.basic",
				DisplayName: "Basic Test",
				Schema:      "1.0",
				Actions:     []Action{{Type: ActionWaitFrames}},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			scenario: Scenario{
				DisplayName: "Test",
				Schema:      "1.0",
			},
			wantErr:  true,
			errField: "id",
		},
		{
			name: "missing display name",
			scenario: Scenario{
				ID:     "test.missing_name",
				Schema: "1.0",
			},
			wantErr:  true,
			errField: "display_name",
		},
		{
			name: "missing schema",
			scenario: Scenario{
				ID:          "test.missing_schema",
				DisplayName: "Test",
			},
			wantErr:  true,
			errField: "schema",
		},
		{
			name: "unsupported schema",
			scenario: Scenario{
				ID:          "test.bad_schema",
				DisplayName: "Test",
				Schema:      "2.0",
			},
			wantErr:  true,
			errField: "schema",
		},
		{
			name: "invalid action type",
			scenario: Scenario{
				ID:          "test.bad_action",
				DisplayName: "Test",
				Schema:      "1.0",
				Actions:     []Action{{Type: "invalid_action"}},
			},
			wantErr:  true,
			errField: "action.type",
		},
		{
			name: "invalid assertion type",
			scenario: Scenario{
				ID:          "test.bad_assertion",
				DisplayName: "Test",
				Schema:      "1.0",
				Actions:     []Action{{Type: ActionWaitFrames}},
				Assertions:  []Assertion{{Type: "invalid_assertion"}},
			},
			wantErr:  true,
			errField: "assertion.type",
		},
		{
			name: "duplicate artifact name",
			scenario: Scenario{
				ID:          "test.duplicate_artifact",
				DisplayName: "Test",
				Schema:      "1.0",
				Actions:     []Action{{Type: ActionWaitFrames}},
				Artifacts: []ArtifactSpec{
					{Type: ArtifactScreenshot, Name: "shot1"},
					{Type: ArtifactScreenshot, Name: "shot1"},
				},
			},
			wantErr:  true,
			errField: "artifacts",
		},
		{
			name: "invalid capability",
			scenario: Scenario{
				ID:          "test.bad_capability",
				DisplayName: "Test",
				Schema:      "1.0",
				Actions:     []Action{{Type: ActionWaitFrames}},
				Capabilities: []Capability{
					CapSceneLoad,
					Capability("invalid_capability"),
				},
			},
			wantErr:  true,
			errField: "capabilities",
		},
		{
			name: "duplicate capability",
			scenario: Scenario{
				ID:          "test.duplicate_capability",
				DisplayName: "Test",
				Schema:      "1.0",
				Actions:     []Action{{Type: ActionWaitFrames}},
				Capabilities: []Capability{
					CapSceneLoad,
					CapSceneLoad,
				},
			},
			wantErr:  true,
			errField: "capabilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.scenario.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errField != "" {
				if vErr, ok := err.(ValidationError); ok {
					if vErr.Field != tt.errField {
						t.Errorf("Validate() error field = %v, want %v", vErr.Field, tt.errField)
					}
				} else {
					t.Errorf("Validate() error type = %T, want ValidationError", err)
				}
			}
		})
	}
}

func TestTarget_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		want   bool
	}{
		{
			name:   "empty target",
			target: Target{},
			want:   true,
		},
		{
			name:   "logical ID only",
			target: Target{LogicalID: "button.ok"},
			want:   false,
		},
		{
			name:   "coordinates only",
			target: Target{X: 100, Y: 200},
			want:   false,
		},
		{
			name:   "with fallback only",
			target: Target{Fallback: &Target{LogicalID: "fallback"}},
			want:   true, // still empty because Fallback is not checked in IsEmpty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.target.IsEmpty(); got != tt.want {
				t.Errorf("Target.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTarget_Resolve(t *testing.T) {
	tests := []struct {
		name          string
		target        Target
		wantLogicalID string
	}{
		{
			name:          "primary target used",
			target:        Target{LogicalID: "primary"},
			wantLogicalID: "primary",
		},
		{
			name:          "fallback used when empty",
			target:        Target{Fallback: &Target{LogicalID: "fallback"}},
			wantLogicalID: "fallback",
		},
		{
			name:          "primary preferred over fallback",
			target:        Target{LogicalID: "primary", Fallback: &Target{LogicalID: "fallback"}},
			wantLogicalID: "primary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.target.Resolve()
			if got.LogicalID != tt.wantLogicalID {
				t.Errorf("Target.Resolve() LogicalID = %v, want %v", got.LogicalID, tt.wantLogicalID)
			}
		})
	}
}

func TestScenario_HasCapability(t *testing.T) {
	s := Scenario{
		Capabilities: []Capability{CapSceneLoad, CapScreenshots},
	}

	if !s.HasCapability(CapSceneLoad) {
		t.Error("Expected HasCapability(CapSceneLoad) to be true")
	}
	if !s.HasCapability(CapScreenshots) {
		t.Error("Expected HasCapability(CapScreenshots) to be true")
	}
	if s.HasCapability(CapIME) {
		t.Error("Expected HasCapability(CapIME) to be false")
	}
}

func TestScenario_HasFamily(t *testing.T) {
	s := Scenario{
		Family:        "basic",
		RequiredScene: "basic",
		Tags:          []string{"smoke", "basic"},
	}

	if !s.HasFamily("basic") {
		t.Error("Expected HasFamily(basic) to be true")
	}
	if !s.HasFamily("smoke") {
		t.Error("Expected HasFamily(smoke) to be true via tags")
	}
	if s.HasFamily("chart") {
		t.Error("Expected HasFamily(chart) to be false")
	}
}

func TestScenario_Clone(t *testing.T) {
	original := &Scenario{
		ID:          "test.clone",
		DisplayName: "Clone Test",
		Schema:      SchemaVersion,
		Family:      "basic",
		Environment: Environment{
			Theme:    "baseline",
			Density:  "default",
			Backend:  "software",
			Platform: "linux",
		},
		Actions: []Action{
			{
				Type:   ActionSceneLoad,
				Target: Target{LogicalID: "button.primary", Fallback: &Target{LogicalID: "fallback"}},
				Params: ActionParams{"scene": "basic"},
			},
		},
		Assertions: []Assertion{{Type: AssertSceneID, Params: AssertionParams{"expected": "basic"}}},
		Artifacts:  []ArtifactSpec{{Type: ArtifactScreenshot, Name: "shot", Required: true}},
		Tags:       []string{"basic"},
		Capabilities: []Capability{
			CapSceneLoad,
			CapScreenshots,
		},
		ExpectedState: &ExpectedState{
			SceneID:       "basic",
			ControlStates: map[string]string{"mode": "default"},
			Theme:         "baseline",
		},
	}

	clone := original.Clone()
	if clone == nil {
		t.Fatal("Clone() returned nil")
	}
	if clone == original {
		t.Fatal("Clone() returned the original pointer")
	}

	clone.Family = "chart"
	clone.Environment.Theme = "dark"
	clone.Actions[0].Params["scene"] = "chart"
	clone.Actions[0].Target.Fallback.LogicalID = "changed"
	clone.Tags[0] = "chart"
	clone.Capabilities[0] = CapAssertions
	clone.ExpectedState.ControlStates["mode"] = "chart"

	if original.Family != "basic" {
		t.Fatalf("original.Family = %q, want basic", original.Family)
	}
	if original.Environment.Theme != "baseline" {
		t.Fatalf("original.Environment.Theme = %q, want baseline", original.Environment.Theme)
	}
	if original.Actions[0].Params["scene"] != "basic" {
		t.Fatalf("original.Actions[0].Params[scene] = %v, want basic", original.Actions[0].Params["scene"])
	}
	if original.Actions[0].Target.Fallback.LogicalID != "fallback" {
		t.Fatalf("original fallback logical ID = %q, want fallback", original.Actions[0].Target.Fallback.LogicalID)
	}
	if original.Tags[0] != "basic" {
		t.Fatalf("original.Tags[0] = %q, want basic", original.Tags[0])
	}
	if original.Capabilities[0] != CapSceneLoad {
		t.Fatalf("original.Capabilities[0] = %q, want %q", original.Capabilities[0], CapSceneLoad)
	}
	if original.ExpectedState.ControlStates["mode"] != "default" {
		t.Fatalf("original ExpectedState.ControlStates[mode] = %q, want default", original.ExpectedState.ControlStates["mode"])
	}
}

func TestNewFixtureScenario(t *testing.T) {
	scenario := NewFixtureScenario("test.fixture", "Fixture Test")

	if scenario.ID != "test.fixture" {
		t.Fatalf("ID = %q, want test.fixture", scenario.ID)
	}
	if scenario.DisplayName != "Fixture Test" {
		t.Fatalf("DisplayName = %q, want Fixture Test", scenario.DisplayName)
	}
	if scenario.Schema != SchemaVersion {
		t.Fatalf("Schema = %q, want %q", scenario.Schema, SchemaVersion)
	}
}

func TestScenario_JSONRoundTrip(t *testing.T) {
	original := Scenario{
		ID:            "test.roundtrip",
		DisplayName:   "Round Trip",
		Schema:        SchemaVersion,
		Family:        "basic",
		RequiredScene: "basic",
		Environment: Environment{
			Theme:    "baseline",
			Density:  "default",
			Backend:  "software",
			Platform: "linux",
		},
		Actions: []Action{{Type: ActionSceneLoad, Params: ActionParams{"scene": "basic"}}},
		Assertions: []Assertion{{Type: AssertSceneID, Params: AssertionParams{
			"expected": "basic",
		}}},
		Artifacts: []ArtifactSpec{{Type: ArtifactScreenshot, Name: "initial", Required: true}},
		Tags:      []string{"basic", "smoke"},
		Capabilities: []Capability{
			CapSceneLoad,
			CapScreenshots,
			CapAssertions,
		},
		ExpectedState: &ExpectedState{
			SceneID: "basic",
			Theme:   "baseline",
			Density: "default",
		},
		Description: "round trip fixture",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Scenario
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.ID != original.ID {
		t.Fatalf("decoded ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.DisplayName != original.DisplayName {
		t.Fatalf("decoded DisplayName = %q, want %q", decoded.DisplayName, original.DisplayName)
	}
	if decoded.RequiredScene != original.RequiredScene {
		t.Fatalf("decoded RequiredScene = %q, want %q", decoded.RequiredScene, original.RequiredScene)
	}
	if decoded.Family != original.Family {
		t.Fatalf("decoded Family = %q, want %q", decoded.Family, original.Family)
	}
	if len(decoded.Capabilities) != len(original.Capabilities) {
		t.Fatalf("decoded Capabilities len = %d, want %d", len(decoded.Capabilities), len(original.Capabilities))
	}
	if decoded.ExpectedState == nil || decoded.ExpectedState.SceneID != "basic" {
		t.Fatalf("decoded ExpectedState = %#v, want scene_id basic", decoded.ExpectedState)
	}
}

func TestScenario_Summary(t *testing.T) {
	s := Scenario{
		ID:          "test.summary",
		DisplayName: "Summary Test",
		Actions:     []Action{{Type: ActionWaitFrames}, {Type: ActionScreenshot}},
		Assertions:  []Assertion{{Type: AssertSceneID}},
		Artifacts:   []ArtifactSpec{{Type: ArtifactScreenshot}},
	}

	summary := s.Summary()
	expected := "Summary Test (test.summary): 2 actions, 1 assertions, 1 artifacts"
	if summary != expected {
		t.Errorf("Summary() = %q, want %q", summary, expected)
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "with step",
			err:  ValidationError{Field: "action.type", Message: "missing type", Step: 3},
			want: "step 3: action.type: missing type",
		},
		{
			name: "without step",
			err:  ValidationError{Field: "id", Message: "missing ID"},
			want: "id: missing ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSupportedSchema(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"supported 1.0", "1.0", true},
		{"unsupported 2.0", "2.0", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSupportedSchema(tt.version); got != tt.want {
				t.Errorf("isSupportedSchema(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
