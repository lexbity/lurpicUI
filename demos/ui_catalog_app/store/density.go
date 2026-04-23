package store

import (
	"codeburg.org/lexbit/lurpicui/store"
)

// DensityMode represents the UI density setting.
type DensityMode uint8

const (
	DensityCompact DensityMode = iota
	DensityNormal
	DensityComfortable
)

func (d DensityMode) String() string {
	switch d {
	case DensityCompact:
		return "Compact"
	case DensityNormal:
		return "Normal"
	case DensityComfortable:
		return "Comfortable"
	default:
		return "Unknown"
	}
}

// SpacingScale returns the spacing multiplier for this density.
func (d DensityMode) SpacingScale() float32 {
	switch d {
	case DensityCompact:
		return 0.75
	case DensityNormal:
		return 1.0
	case DensityComfortable:
		return 1.25
	default:
		return 1.0
	}
}

// AllDensityModes returns all available density modes.
func AllDensityModes() []DensityMode {
	return []DensityMode{DensityCompact, DensityNormal, DensityComfortable}
}

// DensityStore holds the currently selected density mode.
var DensityStore = store.NewValueStore[DensityMode](DensityNormal)

// SetDensity sets the density mode.
func SetDensity(mode DensityMode) {
	DensityStore.Set(mode)
}

// GetDensity returns the current density mode.
func GetDensity() DensityMode {
	return DensityStore.Get()
}

// GetSpacing returns a spacing value scaled by the current density.
func GetSpacing(base float32) float32 {
	return base * DensityStore.Get().SpacingScale()
}
