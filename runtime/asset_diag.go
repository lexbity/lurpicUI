package runtime

import (
	"os"
	"strconv"

	"codeburg.org/lexbit/lurpicui/assets"
)

// assetDiagEnvVar is the environment variable that enables asset-path logging.
const assetDiagEnvVar = "LURPIC_ASSET_DIAGNOSTICS"

// LogAssetMount logs a pak or filesystem mount event.
func (rt *Runtime) LogAssetMount(path string, size int64, buildHash string) {
	if !rt.assetDiagEnabled() {
		return
	}
	rt.log.Info("asset.mount",
		"path", path,
		"size", size,
		"buildHash", buildHash,
	)
}

// LogAssetExtract logs extraction progress or phase transitions.
func (rt *Runtime) LogAssetExtract(progress float32, phase string) {
	if !rt.assetDiagEnabled() {
		return
	}
	rt.log.Info("asset.extract",
		"progress", progress,
		"phase", phase,
	)
}

// LogAssetStream logs an asset load/stream event, including GPU residency
// state when the asset manager's registry has it.
func (rt *Runtime) LogAssetStream(id assets.AssetID, typ assets.AssetType, lod int) {
	if !rt.assetDiagEnabled() {
		return
	}
	gpu := "cpu"
	if reg := rt.AssetRegistry(); reg != nil {
		if entry := reg.Get(id); entry != nil && lod >= 0 && lod < len(entry.LODGPUReady) {
			if entry.LODGPUReady[lod] {
				gpu = "gpu"
			}
		}
	}
	rt.log.Debug("asset.stream",
		"id", id.String(),
		"type", typ.String(),
		"lod", lod,
		"gpu", gpu,
	)
}

// LogAssetEvict logs a cache eviction event.
func (rt *Runtime) LogAssetEvict(id assets.AssetID, lod int, reason string) {
	if !rt.assetDiagEnabled() {
		return
	}
	rt.log.Info("asset.evict",
		"id", id.String(),
		"lod", lod,
		"reason", reason,
	)
}

// assetDiagEnabled reports whether asset-path event logging is enabled.
// It checks the Config field first, then falls back to the env var.
func (rt *Runtime) assetDiagEnabled() bool {
	if rt != nil && rt.config.AssetDiagnosticsEnabled {
		return true
	}
	if v, ok := os.LookupEnv(assetDiagEnvVar); ok {
		if enabled, err := strconv.ParseBool(v); err == nil && enabled {
			return true
		}
	}
	return false
}

