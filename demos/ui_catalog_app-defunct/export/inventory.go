package export

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sort"
	"time"

	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// timestamp returns the current UTC timestamp in ISO 8601 format.
func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// InventoryExport represents the full inventory export format.
type InventoryExport struct {
	Metadata ExportMetadata `json:"metadata"`
	Entries  []EntryExport  `json:"entries"`
}

// ExportMetadata contains information about the export.
type ExportMetadata struct {
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Count     int    `json:"count"`
}

// EntryExport represents a single catalog entry in the export.
type EntryExport struct {
	ID                  string          `json:"id"`
	DisplayName         string          `json:"displayName"`
	Family              string          `json:"family"`
	Subcategory         string          `json:"subcategory"`
	ConstructionClass   string          `json:"constructionClass"`
	LogicalID           string          `json:"logicalId"`
	CoverageStatus      string          `json:"coverageStatus"`
	Interactive         bool            `json:"interactive"`
	ThemeSensitive      bool            `json:"themeSensitive"`
	LayoutSensitive     bool            `json:"layoutSensitive"`
	Variants            []model.Variant `json:"variants"`
	States              []model.State   `json:"states"`
	MissingVariants     []string        `json:"missingVariants,omitempty"`
	MissingStates       []string        `json:"missingStates,omitempty"`
	UnsupportedVariants []string        `json:"unsupportedVariants,omitempty"`
	UnsupportedStates   []string        `json:"unsupportedStates,omitempty"`
	Notes               string          `json:"notes,omitempty"`
}

// exportInventory exports the complete inventory as JSON.
func (e *Exporter) exportInventory() Result {
	entries := store.CatalogInstance.AllEntries()

	// Sort entries by ID for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	// Convert to export format
	exportEntries := make([]EntryExport, len(entries))
	for i, entry := range entries {
		exportEntries[i] = EntryExport{
			ID:                  entry.ID,
			DisplayName:         entry.DisplayName,
			Family:              entry.Family.String(),
			Subcategory:         entry.Subcategory,
			ConstructionClass:   entry.ConstructionClass.String(),
			LogicalID:           entry.ID,
			CoverageStatus:      entry.Coverage.String(),
			Interactive:         entry.Interactive,
			ThemeSensitive:      entry.ThemeSensitive,
			LayoutSensitive:     entry.LayoutSensitive,
			Variants:            append([]model.Variant(nil), entry.Variants...),
			States:              append([]model.State(nil), entry.States...),
			MissingVariants:     append([]string(nil), entry.MissingVariants...),
			MissingStates:       append([]string(nil), entry.MissingStates...),
			UnsupportedVariants: append([]string(nil), entry.UnsupportedVariants...),
			UnsupportedStates:   append([]string(nil), entry.UnsupportedStates...),
			Notes:               entry.Notes,
		}
	}

	export := InventoryExport{
		Metadata: ExportMetadata{
			Version:   "1.0",
			Timestamp: timestamp(),
			Count:     len(entries),
		},
		Entries: exportEntries,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to marshal inventory: %w", err)}
	}

	filename := e.generateFilename("inventory")
	if e.options.Filename != "" {
		filename = e.options.Filename + ".json"
	}

	return e.writeFile(filename, data)
}

// exportVisible exports only the currently visible (filtered) entries.
func (e *Exporter) exportVisible() Result {
	entries := store.FilteredEntries.Get()

	// Sort entries by ID for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	// Convert to export format
	exportEntries := make([]EntryExport, len(entries))
	for i, entry := range entries {
		exportEntries[i] = EntryExport{
			ID:                  entry.ID,
			DisplayName:         entry.DisplayName,
			Family:              entry.Family.String(),
			Subcategory:         entry.Subcategory,
			ConstructionClass:   entry.ConstructionClass.String(),
			LogicalID:           entry.ID,
			CoverageStatus:      entry.Coverage.String(),
			Interactive:         entry.Interactive,
			ThemeSensitive:      entry.ThemeSensitive,
			LayoutSensitive:     entry.LayoutSensitive,
			Variants:            append([]model.Variant(nil), entry.Variants...),
			States:              append([]model.State(nil), entry.States...),
			MissingVariants:     append([]string(nil), entry.MissingVariants...),
			MissingStates:       append([]string(nil), entry.MissingStates...),
			UnsupportedVariants: append([]string(nil), entry.UnsupportedVariants...),
			UnsupportedStates:   append([]string(nil), entry.UnsupportedStates...),
			Notes:               entry.Notes,
		}
	}

	export := InventoryExport{
		Metadata: ExportMetadata{
			Version:   "1.0",
			Timestamp: timestamp(),
			Count:     len(entries),
		},
		Entries: exportEntries,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to marshal visible entries: %w", err)}
	}

	filename := e.generateFilename("visible")
	if e.options.Filename != "" {
		filename = e.options.Filename + ".json"
	}

	return e.writeFile(filename, data)
}

// exportCoverage exports a coverage report.
func (e *Exporter) exportCoverage() Result {
	entries := store.CatalogInstance.AllEntries()

	// Calculate coverage statistics
	var total, implemented, partial, placeholder, missing, themeDependent, layoutDependent int
	for _, entry := range entries {
		total++
		switch entry.Coverage {
		case model.CoverageImplemented:
			implemented++
		case model.CoveragePartial:
			partial++
		case model.CoveragePlaceholder:
			placeholder++
		case model.CoverageMissing:
			missing++
		case model.CoverageThemeDependent:
			themeDependent++
		case model.CoverageLayoutDependent:
			layoutDependent++
		}
	}

	report := CoverageReport{
		Metadata: ExportMetadata{
			Version:   "1.0",
			Timestamp: timestamp(),
			Count:     total,
		},
		Summary: CoverageSummary{
			Total:           total,
			Implemented:     implemented,
			Partial:         partial,
			Placeholder:     placeholder,
			Missing:         missing,
			ThemeDependent:  themeDependent,
			LayoutDependent: layoutDependent,
			Percent:         float64(implemented) / float64(total) * 100,
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to marshal coverage report: %w", err)}
	}

	filename := e.generateFilename("coverage")
	if e.options.Filename != "" {
		filename = e.options.Filename + ".json"
	}

	return e.writeFile(filename, data)
}

// CoverageReport represents a coverage report export.
type CoverageReport struct {
	Metadata ExportMetadata  `json:"metadata"`
	Summary  CoverageSummary `json:"summary"`
}

// CoverageSummary contains coverage statistics.
type CoverageSummary struct {
	Total           int     `json:"total"`
	Implemented     int     `json:"implemented"`
	Partial         int     `json:"partial"`
	Placeholder     int     `json:"placeholder"`
	Missing         int     `json:"missing"`
	ThemeDependent  int     `json:"themeDependent"`
	LayoutDependent int     `json:"layoutDependent"`
	Percent         float64 `json:"percent"`
}

// exportScreenshot exports a PNG screenshot (stub - requires render target access).
func (e *Exporter) exportScreenshot() Result {
	// Create a placeholder image showing the current state
	// In a real implementation, this would capture from the render target
	width, height := 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with background color (light gray)
	gray := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, gray)
		}
	}

	// Encode as PNG
	tempFile, err := os.CreateTemp("", "screenshot_*.png")
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to create temp file: %w", err)}
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Actually write the PNG
	filename := e.generateFilename("screenshot")
	if e.options.Filename != "" {
		filename = e.options.Filename + ".png"
	}
	path := e.options.OutputDir + "/" + filename

	file, err := os.Create(path)
	if err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to create file: %w", err)}
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return Result{Success: false, Error: fmt.Errorf("failed to encode PNG: %w", err)}
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
