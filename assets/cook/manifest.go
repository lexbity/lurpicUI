package cook

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Manifest models assets/cook.toml.
type Manifest struct {
	Path string `toml:"-"`

	Cook struct {
		Targets []string `toml:"targets"`
	} `toml:"cook"`

	IDs struct {
		Registry string `toml:"registry"`
	} `toml:"ids"`

	SVG struct {
		LOD1ThresholdPX int `toml:"lod1_threshold_px"`
		LOD2ThresholdPX int `toml:"lod2_threshold_px"`
		LOD1RasterSize  int `toml:"lod1_raster_size"`
	} `toml:"svg"`

	Images struct {
		Quality int  `toml:"quality"`
		GenMips bool `toml:"gen_mips"`
		GenLODs bool `toml:"gen_lods"`
	} `toml:"images"`

	Fonts struct {
		DefaultRanges []string `toml:"default_ranges"`
	} `toml:"fonts"`

	Font   map[string]FontRule   `toml:"font"`
	Config map[string]ConfigRule `toml:"config"`

	Pack struct {
		SVGCompression    string `toml:"svg_compression"`
		ImageCompression  string `toml:"image_compression"`
		FontCompression   string `toml:"font_compression"`
		ConfigCompression string `toml:"config_compression"`
	} `toml:"pack"`
}

// FontRule configures per-font Unicode ranges.
type FontRule struct {
	Ranges []string `toml:"ranges"`
}

// ConfigRule configures explicit config dependencies.
type ConfigRule struct {
	Deps []string `toml:"deps"`
}

// LoadManifest reads and parses a TOML cook manifest from path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil, err
	}
	m.Path = path
	return m, nil
}

// ParseManifest parses a TOML manifest from bytes.
func ParseManifest(src []byte) (*Manifest, error) {
	var m Manifest
	if _, err := toml.Decode(string(src), &m); err != nil {
		return nil, fmt.Errorf("parse cook manifest: %w", err)
	}
	return &m, nil
}

// RegistryPath returns the registry path resolved against the manifest location if needed.
func (m *Manifest) RegistryPath() string {
	if m == nil {
		return ""
	}
	if m.IDs.Registry == "" {
		return ""
	}
	if filepath.IsAbs(m.IDs.Registry) || m.Path == "" {
		return filepath.Clean(m.IDs.Registry)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(m.Path), m.IDs.Registry))
}
