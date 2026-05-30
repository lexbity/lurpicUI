//go:build android

package app

import (
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/platform/android"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// On Android the platform App is the NativeActivity-backed bridge.
var newPlatformApp = android.NewApp

func init() {
	initAssetManager = openAndroidAssetManager
}

func openAndroidAssetManager(rtConfig *runtime.Config) {
	if rtConfig.AssetManager != nil {
		return
	}
	pak, err := android.OpenPlatformPak()
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: open platform pak: %v\n", err)
		return
	}
	reg := assets.NewAssetRegistryStore()
	// PathIDRegistry is wired from the cooked asset manifest. When nil, the
	// manager returns empty handles until the ID registry is set up (Phase 12).
	rtConfig.AssetManager = assets.NewManager(reg, pak, assets.BackendSoftware, nil, nil)
	rtConfig.AssetRegistry = reg
}
