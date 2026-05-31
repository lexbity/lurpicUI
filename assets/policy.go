package assets

// Residency is the per-asset-type decision for GPU residency.
type Residency uint8

const (
	ResidencyCPU Residency = iota
	ResidencyGPU
)

// AssetResidency returns the residency decision for an asset type given
// the global residency mode and whether the backend is GPU-capable.
// This is the single source of truth for per-type policy and must be
// consistent between decode-enqueue and drawable.Resolve.
func AssetResidency(t AssetType, mode ResidencyMode, gpuCapable bool) Residency {
	if !gpuCapable {
		return ResidencyCPU
	}
	switch mode {
	case ResidencyCPUOnly:
		return ResidencyCPU
	case ResidencyGPUResident:
	case ResidencyAuto:
	}
	switch t {
	case AssetTypeImage:
		return ResidencyGPU
	default:
		return ResidencyCPU
	}
}
