// Package capabilities provides a public wrapper around the internal capindex
// scanner so that code outside cmd/lurpiclint/internal/ (cross-seam integration
// tests, the lurpic_studio demo's Capability Index exhibit) can invoke
// loader.Load + capindex.Scan without importing internal packages.
//
// F-capindex-internal resolution (Slice P9): the Capability Index exhibit needs
// the same catalog `lurpiclint capabilities` emits. The generator lives under
// cmd/lurpiclint/internal/capindex and cannot be imported from demos/ (Go
// internal-package rule), so this package is the public seam: both the CLI and
// the demo call capabilities.Scan, guaranteeing "same generator, same module
// root" by construction.
package capabilities

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// modulePath is the framework module path the capindex ScanConfig uses.
const modulePath = "codeburg.org/lexbit/lurpicui"

// Capability describes a single discoverable framework capability.
type Capability = capindex.Capability

// CapabilityKind classifies a framework capability.
type CapabilityKind = capindex.CapabilityKind

const (
	KindMark   CapabilityKind = capindex.KindMark
	KindLayout                = capindex.KindLayout
	KindLayer                 = capindex.KindLayer
)

// FrameworkPatterns returns the loader patterns for the whole framework
// uxauthoring surface: marks, layouts, and the facet base package. This is the
// single source of truth for what "the framework catalog" means; both the CLI
// and the demo build their scans from it.
func FrameworkPatterns(moduleRoot string) []string {
	return []string{
		moduleRoot + "/marks/...",
		moduleRoot + "/layout/...",
		moduleRoot + "/facet",
	}
}

// Scan loads the whole framework surface and scans for capabilities. It is the
// same scan `lurpiclint capabilities` performs (the CLI calls this function).
// moduleRoot is the absolute path to the module root (containing go.mod).
func Scan(moduleRoot string) ([]Capability, error) {
	return scan(moduleRoot, FrameworkPatterns(moduleRoot))
}

// ScanMarks loads only the marks packages and scans for capabilities.
// moduleRoot is the absolute path to the module root (containing go.mod).
func ScanMarks(moduleRoot string) ([]Capability, error) {
	return scan(moduleRoot, []string{moduleRoot + "/marks/..."})
}

func scan(moduleRoot string, patterns []string) ([]Capability, error) {
	result, err := loader.Load(patterns, loader.Config{})
	if err != nil {
		return nil, fmt.Errorf("loader.Load: %w", err)
	}

	caps := capindex.Scan(result, capindex.ScanConfig{
		ModulePath: modulePath,
		ModuleRoot: moduleRoot,
	})
	return caps, nil
}
