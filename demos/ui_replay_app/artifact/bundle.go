package artifact

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeburg.org/lexbit/ui_replay/model"
)

// Bundle represents a complete replay output package.
type Bundle struct {
	Manifest   *Manifest
	Artifacts  []Artifact
	CreatedAt  time.Time
	OutputPath string
}

// Manifest describes the bundle contents and provenance.
type Manifest struct {
	Version       string                `json:"version"`
	CreatedAt     time.Time             `json:"created_at"`
	ScenarioID    string                `json:"scenario_id"`
	ScenarioName  string                `json:"scenario_name"`
	SchemaVersion string                `json:"schema_version"`
	RunResult     model.ExecutionStatus `json:"run_result"`
	Environment   EnvironmentInfo       `json:"environment"`
	Artifacts     []ArtifactEntry       `json:"artifacts"`
	Provenance    ProvenanceInfo        `json:"provenance"`
}

// EnvironmentInfo captures the execution environment.
type EnvironmentInfo struct {
	Theme        string `json:"theme"`
	Density      string `json:"density"`
	Backend      string `json:"backend"`
	WindowWidth  int    `json:"window_width"`
	WindowHeight int    `json:"window_height"`
}

// ArtifactEntry describes a single artifact in the manifest.
type ArtifactEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

// ProvenanceInfo captures build and runtime metadata.
type ProvenanceInfo struct {
	BuildCommit    string    `json:"build_commit,omitempty"`
	BuildTime      time.Time `json:"build_time,omitempty"`
	RuntimeVersion string    `json:"runtime_version,omitempty"`
	Platform       string    `json:"platform,omitempty"`
}

// Artifact represents a captured artifact.
type Artifact struct {
	Name     string
	Type     ArtifactType
	Data     []byte
	Metadata map[string]interface{}
}

// ArtifactType identifies the kind of artifact.
type ArtifactType string

const (
	TypeScreenshot      ArtifactType = "screenshot"
	TypeSceneState      ArtifactType = "scene_state"
	TypeDiagnostics     ArtifactType = "diagnostics"
	TypeLog             ArtifactType = "log"
	TypeManifest        ArtifactType = "manifest"
	TypeAssertionReport ArtifactType = "assertion_report"
)

// BundleVersion is the current bundle format version.
const BundleVersion = "1.0"

// BundleBuilder constructs artifact bundles.
type BundleBuilder struct {
	artifacts  []Artifact
	scenario   *model.Scenario
	env        model.Environment
	result     model.ExecutionStatus
	provenance ProvenanceInfo
	outputDir  string
}

// NewBundleBuilder creates a new bundle builder.
func NewBundleBuilder(scenario *model.Scenario, outputDir string) *BundleBuilder {
	return &BundleBuilder{
		artifacts: make([]Artifact, 0),
		scenario:  scenario,
		outputDir: outputDir,
		env:       scenario.Environment,
	}
}

// SetRunResult sets the execution result for the bundle.
func (b *BundleBuilder) SetRunResult(result model.ExecutionStatus) {
	b.result = result
}

// SetProvenance sets the provenance information.
func (b *BundleBuilder) SetProvenance(prov ProvenanceInfo) {
	b.provenance = prov
}

// AddArtifact adds an artifact to the bundle.
func (b *BundleBuilder) AddArtifact(artifact Artifact) {
	b.artifacts = append(b.artifacts, artifact)
}

// AddScreenshot adds a screenshot artifact.
func (b *BundleBuilder) AddScreenshot(name string, data []byte, metadata map[string]interface{}) {
	b.AddArtifact(Artifact{
		Name:     b.normalizeName(name) + ".png",
		Type:     TypeScreenshot,
		Data:     data,
		Metadata: metadata,
	})
}

// AddSceneState adds a scene state export.
func (b *BundleBuilder) AddSceneState(name string, state map[string]interface{}) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scene state: %w", err)
	}
	b.AddArtifact(Artifact{
		Name: b.normalizeName(name) + ".json",
		Type: TypeSceneState,
		Data: data,
		Metadata: map[string]interface{}{
			"format": "json",
		},
	})
	return nil
}

// AddDiagnostics adds a diagnostics snapshot.
func (b *BundleBuilder) AddDiagnostics(name string, diagnostics map[string]interface{}) error {
	data, err := json.MarshalIndent(diagnostics, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal diagnostics: %w", err)
	}
	b.AddArtifact(Artifact{
		Name: b.normalizeName(name) + ".json",
		Type: TypeDiagnostics,
		Data: data,
		Metadata: map[string]interface{}{
			"format": "json",
		},
	})
	return nil
}

// AddLog adds a log export.
func (b *BundleBuilder) AddLog(name string, content string) {
	b.AddArtifact(Artifact{
		Name: b.normalizeName(name) + ".log",
		Type: TypeLog,
		Data: []byte(content),
		Metadata: map[string]interface{}{
			"format": "text",
		},
	})
}

// AddAssertionReport adds an assertion report.
func (b *BundleBuilder) AddAssertionReport(report map[string]interface{}) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal assertion report: %w", err)
	}
	b.AddArtifact(Artifact{
		Name: "assertion_report.json",
		Type: TypeAssertionReport,
		Data: data,
		Metadata: map[string]interface{}{
			"format": "json",
		},
	})
	return nil
}

// Build creates the final bundle.
func (b *BundleBuilder) Build() (*Bundle, error) {
	createdAt := time.Now()

	// Create artifact entries
	entries := make([]ArtifactEntry, 0, len(b.artifacts))
	for _, art := range b.artifacts {
		entry := ArtifactEntry{
			Name:     art.Name,
			Type:     string(art.Type),
			Path:     art.Name,
			Size:     int64(len(art.Data)),
			Checksum: calculateChecksum(art.Data),
		}
		entries = append(entries, entry)
	}

	// Create manifest
	manifest := &Manifest{
		Version:       BundleVersion,
		CreatedAt:     createdAt,
		ScenarioID:    string(b.scenario.ID),
		ScenarioName:  b.scenario.DisplayName,
		SchemaVersion: b.scenario.Schema,
		RunResult:     b.result,
		Environment: EnvironmentInfo{
			Theme:        b.env.Theme,
			Density:      b.env.Density,
			Backend:      b.env.Backend,
			WindowWidth:  b.env.WindowSize.Width,
			WindowHeight: b.env.WindowSize.Height,
		},
		Artifacts:  entries,
		Provenance: b.provenance,
	}

	// Add manifest as artifact
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	bundle := &Bundle{
		Manifest:   manifest,
		Artifacts:  b.artifacts,
		CreatedAt:  createdAt,
		OutputPath: b.generateBundlePath(),
	}

	// Prepend manifest artifact
	bundle.Artifacts = append([]Artifact{{
		Name: "manifest.json",
		Type: TypeManifest,
		Data: manifestData,
	}}, bundle.Artifacts...)

	return bundle, nil
}

// SaveToDisk saves the bundle to disk as a directory.
func (b *Bundle) SaveToDisk() error {
	dir := b.OutputPath
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create bundle directory: %w", err)
	}

	for _, art := range b.Artifacts {
		path := filepath.Join(dir, art.Name)
		if err := os.WriteFile(path, art.Data, 0644); err != nil {
			return fmt.Errorf("write artifact %s: %w", art.Name, err)
		}
	}

	return nil
}

// SaveAsZip saves the bundle as a ZIP archive.
func (b *Bundle) SaveAsZip(zipPath string) error {
	file, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip file: %w", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for _, art := range b.Artifacts {
		writer, err := zipWriter.Create(art.Name)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", art.Name, err)
		}
		if _, err := writer.Write(art.Data); err != nil {
			return fmt.Errorf("write zip entry %s: %w", art.Name, err)
		}
	}

	return nil
}

// LoadBundle loads a bundle from a directory.
func LoadBundle(dir string) (*Bundle, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	bundle := &Bundle{
		Manifest:   &manifest,
		Artifacts:  make([]Artifact, 0),
		CreatedAt:  manifest.CreatedAt,
		OutputPath: dir,
	}

	// Load all artifacts
	for _, entry := range manifest.Artifacts {
		if entry.Name == "manifest.json" {
			continue // Already loaded
		}

		path := filepath.Join(dir, entry.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read artifact %s: %w", entry.Name, err)
		}

		bundle.Artifacts = append(bundle.Artifacts, Artifact{
			Name: entry.Name,
			Type: ArtifactType(entry.Type),
			Data: data,
		})
	}

	return bundle, nil
}

// LoadBundleFromZip loads a bundle from a ZIP archive.
func LoadBundleFromZip(zipPath string) (*Bundle, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	var manifest Manifest
	var artifacts []Artifact

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read zip entry %s: %w", file.Name, err)
		}

		if file.Name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
		} else {
			artifacts = append(artifacts, Artifact{
				Name: file.Name,
				Data: data,
			})
		}
	}

	return &Bundle{
		Manifest:   &manifest,
		Artifacts:  artifacts,
		CreatedAt:  manifest.CreatedAt,
		OutputPath: zipPath,
	}, nil
}

// Validate checks bundle integrity.
func (b *Bundle) Validate() error {
	if b.Manifest == nil {
		return fmt.Errorf("bundle missing manifest")
	}

	if b.Manifest.Version != BundleVersion {
		return fmt.Errorf("unsupported bundle version: %s", b.Manifest.Version)
	}

	// Verify artifact checksums
	for _, entry := range b.Manifest.Artifacts {
		if entry.Name == "manifest.json" {
			continue
		}

		found := false
		for _, art := range b.Artifacts {
			if art.Name == entry.Name {
				found = true
				actualChecksum := calculateChecksum(art.Data)
				if actualChecksum != entry.Checksum {
					return fmt.Errorf("artifact %s checksum mismatch", entry.Name)
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("artifact %s missing from bundle", entry.Name)
		}
	}

	return nil
}

// GetArtifact retrieves an artifact by name.
func (b *Bundle) GetArtifact(name string) (Artifact, bool) {
	for _, art := range b.Artifacts {
		if art.Name == name {
			return art, true
		}
	}
	return Artifact{}, false
}

// GetArtifactsByType retrieves all artifacts of a given type.
func (b *Bundle) GetArtifactsByType(artType ArtifactType) []Artifact {
	var result []Artifact
	for _, art := range b.Artifacts {
		if art.Type == artType {
			result = append(result, art)
		}
	}
	return result
}

// Summary returns a human-readable summary of the bundle.
func (b *Bundle) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Bundle: %s\n", b.Manifest.ScenarioID))
	sb.WriteString(fmt.Sprintf("Created: %s\n", b.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Result: %s\n", b.Manifest.RunResult))
	sb.WriteString(fmt.Sprintf("Artifacts: %d\n", len(b.Artifacts)))

	for _, art := range b.Artifacts {
		sb.WriteString(fmt.Sprintf("  - %s (%s, %d bytes)\n", art.Name, art.Type, len(art.Data)))
	}

	return sb.String()
}

// Helper functions

func (b *BundleBuilder) normalizeName(name string) string {
	// Replace spaces and special characters with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return strings.ToLower(name)
}

func (b *BundleBuilder) generateBundlePath() string {
	timestamp := time.Now().Format("20060102_150405")
	scenarioSafe := b.normalizeName(string(b.scenario.ID))
	return filepath.Join(b.outputDir, fmt.Sprintf("%s_%s", scenarioSafe, timestamp))
}

func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
