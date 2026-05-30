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
	origNewRuntime := newRuntime
	newRuntime = func(config runtime.Config, platformApp platformApp, window platformWindow, backend renderBackend, root facetRoot) (*runtime.Runtime, error) {
		// Open the platform pak before creating the runtime so it's ready
		// when facets request assets. A missing pak is not fatal — the app
		// may be asset-free or the pak may be delivered later.
		pak, err := android.OpenPlatformPak()
		if err != nil {
			fmt.Fprintf(os.Stderr, "app/android: open platform pak: %v\n", err)
		} else {
			reg := assets.NewAssetRegistryStore()
			// TODO: wire the PathIDRegistry from the cook pipeline so the
			// manager can resolve asset paths to IDs.
			config.AssetManager = assets.NewManager(reg, pak, assets.BackendSoftware, nil, nil)
			config.AssetRegistry = reg
		}
		return origNewRuntime(config, platformApp, window, backend, root)
	}
}
