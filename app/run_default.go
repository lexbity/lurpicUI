//go:build !android

package app

import "codeburg.org/lexbit/lurpicui/platform/linux"

// On desktop targets the platform App is the Linux (X11/Wayland) backend.
var newPlatformApp = linux.NewApp
