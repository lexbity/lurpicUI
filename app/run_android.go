//go:build android

package app

import (
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/platform/android"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// On Android the platform App is the NativeActivity-backed bridge.
var newPlatformApp = android.NewApp

func init() {
	// Android initAssetManager needs to accept the jobPool parameter to match
	// the desktop path signature. Use a wrapper that ignores the pool for now
	// and update openAndroidAssetManager to use it.
	initAssetManager = func(rtConfig *runtime.Config, jobPool *job.Pool) {
		openAndroidAssetManager(rtConfig, jobPool)
	}
}

func openAndroidAssetManager(rtConfig *runtime.Config, jobPool *job.Pool) {
	if rtConfig.AssetManager != nil {
		return
	}
	if os.Getenv("LURPIC_SKIP_ANDROID_PAK") == "1" {
		fmt.Fprintln(os.Stderr, "app/android: skipping platform pak bootstrap via LURPIC_SKIP_ANDROID_PAK=1")
		return
	}
	pak, err := android.OpenPlatformPak()
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: open platform pak: %v\n", err)
		return
	}
	reg := assets.NewAssetRegistryStore()
	// Load the UUID registry from the APK's assets when available.
	// The cook pipeline produces uuid_registry.json alongside assets.pak.
	idReg := loadAndroidIDRegistry()
	
	// Get the assets wallet from the resource plan.
	// Per D9: the application owns pool construction; the runtime and asset manager consume it.
	var assetsWallet *membudget.Wallet
	if rtConfig.Resources.Plan.Caps[membudget.AssetsCPU] > 0 {
		var err error
		assetsWallet, err = membudget.NewWallet(rtConfig.Resources.Plan.Caps[membudget.AssetsCPU])
		if err != nil {
			assetsWallet = nil
		}
	}
	
	// Per D9: Android uses the identical wiring as desktop.
	// Platform code specializes only the asset source (pak vs devfs).
	backendType := assetBackendFromRuntimeConfig(rtConfig)
	if assetsWallet != nil && jobPool != nil {
		rtConfig.AssetManager = assets.NewManagerWithBudget(reg, pak, idReg, jobPool, assetsWallet, nil, assets.ResidencyCPUOnly)
	} else if jobPool != nil {
		rtConfig.AssetManager = assets.NewManagerWithBudget(reg, pak, idReg, jobPool, nil, nil, assets.ResidencyCPUOnly)
	} else if assetsWallet != nil {
		rtConfig.AssetManager = assets.NewManagerWithBudget(reg, pak, idReg, nil, assetsWallet, nil, assets.ResidencyCPUOnly)
	} else {
		// Fallback to legacy constructor for backward compatibility with tests.
		// Production code should always provide both pool and wallet.
		rtConfig.AssetManager = assets.NewManager(reg, pak, backendType, nil, idReg)
	}
	rtConfig.AssetRegistry = reg
}

// loadAndroidIDRegistry reads the UUID registry from the APK's assets. It
// returns nil when the file is missing or unparseable, but warns first: a nil
// registry means every path-based asset load resolves to an empty handle, so a
// missing uuid_registry.json must not be silent.
func loadAndroidIDRegistry() assets.PathIDRegistry {
	data, err := android.ReadAPKAsset("uuid_registry.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: uuid_registry.json not bundled (%v); "+
			"path-based asset lookups will return empty handles\n", err)
		return nil
	}
	reg, err := assets.ParseJSONPathRegistry(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: invalid uuid_registry.json: %v\n", err)
		return nil
	}
	return reg
}
