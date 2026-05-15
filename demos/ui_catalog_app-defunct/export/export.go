// Package export provides export functionality for generating shareable artifacts
// from the ui_catalog application.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ExportType represents the type of export to perform.
type ExportType uint8

const (
	ExportInventory ExportType = iota
	ExportVisible
	ExportCoverage
	ExportScreenshot
)

func (e ExportType) String() string {
	switch e {
	case ExportInventory:
		return "inventory"
	case ExportVisible:
		return "visible"
	case ExportCoverage:
		return "coverage"
	case ExportScreenshot:
		return "screenshot"
	default:
		return "unknown"
	}
}

// Options contains configuration for an export operation.
type Options struct {
	Type      ExportType
	OutputDir string
	Filename  string
	Timestamp bool
	Format    Format
}

// Format represents the output format.
type Format uint8

const (
	FormatJSON Format = iota
	FormatMarkdown
	FormatPNG
)

func (f Format) Extension() string {
	switch f {
	case FormatJSON:
		return ".json"
	case FormatMarkdown:
		return ".md"
	case FormatPNG:
		return ".png"
	default:
		return ".txt"
	}
}

// Result contains the outcome of an export operation.
type Result struct {
	Success  bool
	Path     string
	Size     int64
	Checksum string
	Error    error
}

// Exporter handles export operations.
type Exporter struct {
	options Options
}

// NewExporter creates a new exporter with the given options.
func NewExporter(opts Options) *Exporter {
	return &Exporter{options: opts}
}

// Execute performs the export operation.
func (e *Exporter) Execute() Result {
	switch e.options.Type {
	case ExportInventory:
		return e.exportInventory()
	case ExportVisible:
		return e.exportVisible()
	case ExportCoverage:
		return e.exportCoverage()
	case ExportScreenshot:
		return e.exportScreenshot()
	default:
		return Result{Success: false, Error: fmt.Errorf("unknown export type: %v", e.options.Type)}
	}
}

// generateFilename creates the output filename with optional timestamp.
func (e *Exporter) generateFilename(base string) string {
	name := base
	if e.options.Timestamp {
		timestamp := time.Now().UTC().Format("20060102_150405")
		name = fmt.Sprintf("%s_%s", base, timestamp)
	}
	return name + e.options.Format.Extension()
}

// ensureOutputDir creates the output directory if it doesn't exist.
func (e *Exporter) ensureOutputDir() error {
	if e.options.OutputDir == "" {
		e.options.OutputDir = "."
	}
	return os.MkdirAll(e.options.OutputDir, 0755)
}

// writeFile writes data to the output file.
func (e *Exporter) writeFile(filename string, data []byte) Result {
	if err := e.ensureOutputDir(); err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to create output directory: %w", err)}
	}

	path := filepath.Join(e.options.OutputDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to write file: %w", err)}
	}

	info, err := os.Stat(path)
	if err != nil {
		return Result{Success: true, Path: path, Error: fmt.Errorf("failed to stat file: %w", err)}
	}

	return Result{
		Success: true,
		Path:    path,
		Size:    info.Size(),
	}
}
