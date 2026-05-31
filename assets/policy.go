package assets

// Residency is the per-asset-type decision for GPU residency.
type Residency uint8

const (
	// ResidencyCPU means the asset is only stored in CPU memory (decoded
	// bytes). All consumers use the CPU data (bitmap/vector). This is the
	// default for SVG, font faces, config, and any type when the backend
	// lacks GPU texture support or the global mode is CPUOnly.
	ResidencyCPU Residency = iota

	// ResidencyGPU means the asset should be uploaded to a GPU texture
	// when the backend supports it and the global mode allows GPU
	// residency. The CPU LOD is preserved as a fallback so the asset
	// stays drawable during device-loss recovery or eviction.
	// Currently only AssetTypeImage is GPU-eligible.
	ResidencyGPU
)

// GPUUploadEligible returns true when an asset type can be GPU-resident
// under any mode. This is a per-type policy gate independent of the
// current mode or backend capability: it answers "can this type ever
// be stored on the GPU?" Consumers combine this with the active mode
// and backend state via AssetResidency for the final decision.
func GPUUploadEligible(t AssetType) bool {
	switch t {
	case AssetTypeImage:
		return true
	default:
		return false
	}
}

// AssetResidency returns the residency decision for an asset type given
// the global residency mode and whether the backend is GPU-capable.
// This is the single source of truth for per-type policy and must be
// consistent between decode-enqueue and drawable.Resolve.
//
// Per-type rules (locked):
//
//	AssetTypeImage   → ResidencyGPU when mode allows and backend capable
//	AssetTypeSVG     → ResidencyCPU always (vector; re-rasterised at
//	                    draw size — a fixed GPU bitmap defeats LOD)
//	AssetTypeFont    → ResidencyCPU (Phase 13 promotes glyph atlas)
//	AssetTypeConfig  → ResidencyCPU always
func AssetResidency(t AssetType, mode ResidencyMode, gpuCapable bool) Residency {
	if !gpuCapable || !GPUUploadEligible(t) {
		return ResidencyCPU
	}
	switch mode {
	case ResidencyCPUOnly:
		return ResidencyCPU
	case ResidencyGPUResident:
	case ResidencyAuto:
	}
	return ResidencyGPU
}
