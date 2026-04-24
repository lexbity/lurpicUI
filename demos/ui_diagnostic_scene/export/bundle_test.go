package bundle

import "testing"

func TestArtifactNameIncludesRuntimeFields(t *testing.T) {
	manifest := Manifest{
		RunID:     "run-1",
		SceneID:   "layout",
		SceneName: "Layout",
		Theme:     "night",
		Density:   "compact",
		Backend:   "software",
		Platform:  "linux-amd64",
	}
	got := ArtifactName("scene-snapshot", manifest, "json")
	want := "scene-snapshot_layout_night_compact_software_linux-amd64_run-1.json"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildNormalizesOrdering(t *testing.T) {
	got := Build(Input{
		Manifest: Manifest{RunID: "run-2", SceneID: "alpha", Theme: "default", Density: "normal", Backend: "software", Platform: "linux"},
		Scene: SceneSnapshot{
			SceneID:  "alpha",
			Families: []string{"basic", "structure"},
			Capabilities: map[string]any{
				"b": 2,
				"a": 1,
			},
			State: map[string]any{
				"z": 26,
				"a": 1,
			},
		},
		Logs:      []LogEntry{{Ordinal: 2, Category: "state mutation", Message: "two"}, {Ordinal: 1, Category: "scene load", Message: "one"}},
		Artifacts: []Artifact{{Kind: "z", Name: "z"}, {Kind: "a", Name: "a"}},
	})

	if got.Logs[0].Ordinal != 1 || got.Logs[1].Ordinal != 2 {
		t.Fatalf("expected logs to be ordered by ordinal, got %#v", got.Logs)
	}
	if got.Artifacts[0].Kind != "a" || got.Artifacts[1].Kind != "z" {
		t.Fatalf("expected artifacts to be ordered by kind, got %#v", got.Artifacts)
	}
	if got.Scene.Capabilities["a"] != 1 || got.Scene.State["a"] != 1 {
		t.Fatalf("expected maps to be preserved")
	}
}
