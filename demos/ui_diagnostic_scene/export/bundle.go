package bundle

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	diag "codeburg.org/lexbit/ui_diagnostic_scene/diagnostics"
)

// Manifest describes the runtime context for a bug-report bundle.
type Manifest struct {
	RunID     string `json:"runId"`
	SceneID   string `json:"sceneId"`
	SceneName string `json:"sceneName"`
	Theme     string `json:"theme"`
	Density   string `json:"density"`
	Backend   string `json:"backend"`
	Platform  string `json:"platform"`
	BuildInfo string `json:"buildInfo"`
}

// Artifact describes one exported bundle artifact.
type Artifact struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Format      string `json:"format"`
	Description string `json:"description"`
	Path        string `json:"path,omitempty"`
	Available   bool   `json:"available"`
}

// LogEntry is the normalized event-log record stored in a bundle.
type LogEntry struct {
	Ordinal  int    `json:"ordinal"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Time     string `json:"time,omitempty"`
}

// SceneSnapshot captures the active scene state.
type SceneSnapshot struct {
	SceneID      string         `json:"sceneId"`
	SceneName    string         `json:"sceneName"`
	Description  string         `json:"description"`
	Families     []string       `json:"families"`
	Capabilities map[string]any `json:"capabilities"`
	State        map[string]any `json:"state"`
	Logs         []string       `json:"logs,omitempty"`
}

// Bundle combines the active scene, diagnostics, and logs for bug reports.
type Bundle struct {
	Manifest    Manifest            `json:"manifest"`
	Scene       SceneSnapshot       `json:"scene"`
	Diagnostics DiagnosticsSnapshot `json:"diagnostics"`
	Logs        []LogEntry          `json:"logs"`
	Artifacts   []Artifact          `json:"artifacts"`
}

// DiagnosticsSnapshot captures the app-facing diagnostics state.
type DiagnosticsSnapshot struct {
	Scene        diag.SceneCapabilitySummary `json:"scene"`
	Overlays     diag.ActiveOverlays         `json:"overlays"`
	Focus        diag.FocusSummary           `json:"focus"`
	Hit          diag.HitSummary             `json:"hit"`
	Invalidation diag.InvalidationSummary    `json:"invalidation"`
	Anchor       diag.AnchorSummary          `json:"anchor"`
	Render       diag.RenderBatchSummary     `json:"render"`
	Frames       []diag.FrameStatsView       `json:"frames"`
}

// Input provides the data needed to assemble a bundle.
type Input struct {
	Manifest    Manifest
	Scene       SceneSnapshot
	Diagnostics DiagnosticsSnapshot
	Logs        []LogEntry
	Artifacts   []Artifact
}

// Build assembles a deterministic bundle from the provided inputs.
func Build(in Input) Bundle {
	out := Bundle{
		Manifest:    in.Manifest,
		Scene:       normalizeSceneSnapshot(in.Scene),
		Diagnostics: normalizeDiagnosticsSnapshot(in.Diagnostics),
		Logs:        normalizeLogs(in.Logs),
		Artifacts:   normalizeArtifacts(in.Artifacts),
	}
	if len(out.Artifacts) == 0 {
		out.Artifacts = defaultArtifacts(out.Manifest, out.Scene, out.Diagnostics)
	}
	return out
}

// WriteJSON writes the bundle as indented JSON.
func WriteJSON(w io.Writer, bundle Bundle) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(bundle)
}

// ArtifactName returns a stable artifact filename.
func ArtifactName(kind string, manifest Manifest, ext string) string {
	parts := []string{
		slug(kind),
		slug(manifest.SceneID),
		slug(manifest.Theme),
		slug(manifest.Density),
		slug(manifest.Backend),
		slug(manifest.Platform),
		slug(manifest.RunID),
	}
	name := strings.Join(filterEmpty(parts), "_")
	if ext != "" {
		name += "." + strings.TrimPrefix(ext, ".")
	}
	return name
}

func defaultArtifacts(manifest Manifest, scene SceneSnapshot, diagSnap DiagnosticsSnapshot) []Artifact {
	return []Artifact{
		{
			Name:        ArtifactName("bundle-manifest", manifest, "json"),
			Kind:        "manifest",
			Format:      "json",
			Description: "Bug-report bundle manifest",
			Available:   true,
		},
		{
			Name:        ArtifactName("scene-snapshot", manifest, "json"),
			Kind:        "scene-snapshot",
			Format:      "json",
			Description: "Scene state snapshot",
			Available:   scene.SceneID != "",
		},
		{
			Name:        ArtifactName("diagnostics-snapshot", manifest, "json"),
			Kind:        "diagnostics-snapshot",
			Format:      "json",
			Description: "Diagnostics snapshot",
			Available:   diagSnap.Scene.SceneID != "",
		},
		{
			Name:        ArtifactName("event-logs", manifest, "json"),
			Kind:        "event-logs",
			Format:      "json",
			Description: "Normalized event log stream",
			Available:   true,
		},
		{
			Name:        ArtifactName("screenshot", manifest, "png"),
			Kind:        "screenshot",
			Format:      "png",
			Description: "Placeholder screenshot artifact metadata",
			Available:   scene.SceneID != "" && diagSnap.Scene.SupportsScreenshot,
		},
	}
}

func normalizeSceneSnapshot(snapshot SceneSnapshot) SceneSnapshot {
	out := snapshot
	out.Families = append([]string(nil), snapshot.Families...)
	out.Logs = append([]string(nil), snapshot.Logs...)
	if snapshot.Capabilities != nil {
		out.Capabilities = make(map[string]any, len(snapshot.Capabilities))
		keys := make([]string, 0, len(snapshot.Capabilities))
		for k := range snapshot.Capabilities {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out.Capabilities[k] = snapshot.Capabilities[k]
		}
	}
	if snapshot.State != nil {
		out.State = make(map[string]any, len(snapshot.State))
		keys := make([]string, 0, len(snapshot.State))
		for k := range snapshot.State {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out.State[k] = snapshot.State[k]
		}
	}
	return out
}

func normalizeDiagnosticsSnapshot(snapshot DiagnosticsSnapshot) DiagnosticsSnapshot {
	out := snapshot
	out.Frames = append([]diag.FrameStatsView(nil), snapshot.Frames...)
	sort.SliceStable(out.Frames, func(i, j int) bool {
		return out.Frames[i].FrameNumber < out.Frames[j].FrameNumber
	})
	return out
}

func normalizeLogs(entries []LogEntry) []LogEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]LogEntry, len(entries))
	copy(out, entries)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ordinal != out[j].Ordinal {
			return out[i].Ordinal < out[j].Ordinal
		}
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Message < out[j].Message
	})
	return out
}

func normalizeArtifacts(entries []Artifact) []Artifact {
	if len(entries) == 0 {
		return nil
	}
	out := make([]Artifact, len(entries))
	copy(out, entries)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(s))
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (m Manifest) String() string {
	return fmt.Sprintf("%s/%s/%s", m.SceneID, m.Theme, m.Density)
}
