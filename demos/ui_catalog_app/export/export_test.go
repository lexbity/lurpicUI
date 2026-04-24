package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportType_String(t *testing.T) {
	tests := []struct {
		exportType ExportType
		want       string
	}{
		{ExportInventory, "inventory"},
		{ExportVisible, "visible"},
		{ExportCoverage, "coverage"},
		{ExportScreenshot, "screenshot"},
		{ExportType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.exportType.String(); got != tt.want {
				t.Errorf("ExportType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat_Extension(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{FormatJSON, ".json"},
		{FormatMarkdown, ".md"},
		{FormatPNG, ".png"},
		{Format(99), ".txt"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.format.Extension(); got != tt.want {
				t.Errorf("Format.Extension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewExporter(t *testing.T) {
	opts := Options{
		Type:      ExportInventory,
		OutputDir: "./test-exports",
		Filename:  "test",
		Format:    FormatJSON,
	}

	exporter := NewExporter(opts)
	if exporter == nil {
		t.Fatal("NewExporter() returned nil")
	}
	if exporter.options.Type != ExportInventory {
		t.Errorf("exporter.options.Type = %v, want %v", exporter.options.Type, ExportInventory)
	}
}

func TestExporter_generateFilename(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		base      string
		wantExt   string
		wantCheck func(string) bool
	}{
		{
			name: "without timestamp",
			opts: Options{
				Type:      ExportInventory,
				Format:    FormatJSON,
				Timestamp: false,
			},
			base:    "inventory",
			wantExt: ".json",
		},
		{
			name: "with timestamp",
			opts: Options{
				Type:      ExportInventory,
				Format:    FormatJSON,
				Timestamp: true,
			},
			base:      "inventory",
			wantExt:   ".json",
			wantCheck: func(s string) bool { return len(s) > len("inventory.json") },
		},
		{
			name: "png format",
			opts: Options{
				Type:      ExportScreenshot,
				Format:    FormatPNG,
				Timestamp: false,
			},
			base:    "screenshot",
			wantExt: ".png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExporter(tt.opts)
			got := e.generateFilename(tt.base)

			if !hasSuffix(got, tt.wantExt) {
				t.Errorf("generateFilename() = %v, want extension %v", got, tt.wantExt)
			}

			if tt.wantCheck != nil && !tt.wantCheck(got) {
				t.Errorf("generateFilename() = %v, failed custom check", got)
			}
		})
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestExporter_ensureOutputDir(t *testing.T) {
	tempDir := t.TempDir()
	opts := Options{
		OutputDir: filepath.Join(tempDir, "nested", "dir"),
	}
	e := NewExporter(opts)

	err := e.ensureOutputDir()
	if err != nil {
		t.Errorf("ensureOutputDir() error = %v", err)
	}

	// Check directory exists
	info, err := os.Stat(opts.OutputDir)
	if err != nil {
		t.Errorf("Output directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Output path is not a directory")
	}
}

func TestExporter_exportInventory(t *testing.T) {
	tempDir := t.TempDir()
	opts := Options{
		Type:      ExportInventory,
		OutputDir: tempDir,
		Format:    FormatJSON,
		Filename:  "test-inventory",
	}

	e := NewExporter(opts)
	result := e.Execute()

	if !result.Success {
		t.Fatalf("exportInventory failed: %v", result.Error)
	}

	if result.Path == "" {
		t.Error("result.Path is empty")
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Errorf("Failed to read exported file: %v", err)
	}

	var export InventoryExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Errorf("Exported file is not valid JSON: %v", err)
	}

	if export.Metadata.Count != len(export.Entries) {
		t.Errorf("Metadata.Count (%d) != len(Entries) (%d)", export.Metadata.Count, len(export.Entries))
	}

	if len(export.Entries) == 0 {
		t.Fatal("inventory export should include entries")
	}
	foundMatrix := false
	for _, entry := range export.Entries {
		if entry.Subcategory == "" {
			t.Errorf("entry %s missing subcategory export", entry.ID)
		}
		if len(entry.Variants) > 0 && len(entry.States) > 0 {
			foundMatrix = true
		}
	}
	if !foundMatrix {
		t.Error("expected at least one entry to export variants and states")
	}

	// Check entries are sorted by ID
	for i := 1; i < len(export.Entries); i++ {
		if export.Entries[i].ID < export.Entries[i-1].ID {
			t.Error("Entries are not sorted by ID")
			break
		}
	}
}

func TestExporter_exportVisible(t *testing.T) {
	tempDir := t.TempDir()
	opts := Options{
		Type:      ExportVisible,
		OutputDir: tempDir,
		Format:    FormatJSON,
		Filename:  "test-visible",
	}

	e := NewExporter(opts)
	result := e.Execute()

	if !result.Success {
		t.Fatalf("exportVisible failed: %v", result.Error)
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Errorf("Failed to read exported file: %v", err)
	}

	var export InventoryExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Errorf("Exported file is not valid JSON: %v", err)
	}
}

func TestExporter_exportCoverage(t *testing.T) {
	tempDir := t.TempDir()
	opts := Options{
		Type:      ExportCoverage,
		OutputDir: tempDir,
		Format:    FormatJSON,
		Filename:  "test-coverage",
	}

	e := NewExporter(opts)
	result := e.Execute()

	if !result.Success {
		t.Fatalf("exportCoverage failed: %v", result.Error)
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Errorf("Failed to read exported file: %v", err)
	}

	var report CoverageReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Errorf("Exported file is not valid JSON: %v", err)
	}

	// Check percentages add up
	total := report.Summary.Implemented + report.Summary.Partial +
		report.Summary.Placeholder + report.Summary.Missing +
		report.Summary.ThemeDependent + report.Summary.LayoutDependent
	if total != report.Summary.Total {
		t.Errorf("Summary counts don't add up: %d + %d + %d + %d + %d + %d = %d, want %d",
			report.Summary.Implemented, report.Summary.Partial,
			report.Summary.Placeholder, report.Summary.Missing,
			report.Summary.ThemeDependent, report.Summary.LayoutDependent,
			total, report.Summary.Total)
	}
	if report.Summary.ThemeDependent == 0 {
		t.Error("expected at least one theme-dependent entry in coverage report")
	}
	if report.Summary.LayoutDependent == 0 {
		t.Error("expected at least one layout-dependent entry in coverage report")
	}

	// Check percentage calculation
	expectedPercent := float64(report.Summary.Implemented) / float64(report.Summary.Total) * 100
	if report.Summary.Percent != expectedPercent {
		t.Errorf("Summary.Percent = %v, want %v", report.Summary.Percent, expectedPercent)
	}
}

func TestExporter_exportScreenshot(t *testing.T) {
	tempDir := t.TempDir()
	opts := Options{
		Type:      ExportScreenshot,
		OutputDir: tempDir,
		Format:    FormatPNG,
		Filename:  "test-screenshot",
	}

	e := NewExporter(opts)
	result := e.Execute()

	if !result.Success {
		t.Fatalf("exportScreenshot failed: %v", result.Error)
	}

	// Verify file exists and has PNG header
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Errorf("Failed to read exported file: %v", err)
	}

	// PNG files start with PNG magic bytes
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) < len(pngHeader) {
		t.Error("Screenshot file too small")
	} else if string(data[:len(pngHeader)]) != string(pngHeader) {
		t.Error("Screenshot file doesn't have PNG header")
	}

	if result.Size <= 0 {
		t.Error("result.Size should be positive")
	}
}

func TestExporter_Execute_UnknownType(t *testing.T) {
	opts := Options{
		Type: ExportType(99),
	}

	e := NewExporter(opts)
	result := e.Execute()

	if result.Success {
		t.Error("Execute with unknown type should fail")
	}
	if result.Error == nil {
		t.Error("Execute with unknown type should return error")
	}
}
