// Package capabilities provides a public wrapper around the internal capindex
// scanner so that cross-seam integration tests outside cmd/lurpiclint/internal/
// can invoke loader.Load + capindex.Scan without importing internal packages.
package capabilities

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// Capability describes a single discoverable framework capability.
type Capability = capindex.Capability

// CapabilityKind classifies a framework capability.
type CapabilityKind = capindex.CapabilityKind

const (
	KindMark   CapabilityKind = capindex.KindMark
	KindLayout                = capindex.KindLayout
	KindLayer                 = capindex.KindLayer
)

// ScanMarks loads the marks packages and scans for capabilities.
// moduleRoot is the absolute path to the module root (containing go.mod).
func ScanMarks(moduleRoot string) ([]Capability, error) {
	result, err := loader.Load(
		[]string{moduleRoot + "/marks/..."},
		loader.Config{},
	)
	if err != nil {
		return nil, fmt.Errorf("loader.Load: %w", err)
	}

	caps := capindex.Scan(result, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: moduleRoot,
	})
	return caps, nil
}
