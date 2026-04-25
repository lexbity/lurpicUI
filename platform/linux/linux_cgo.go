//go:build linux && cgo

package linux

import (
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/linux/internal/display"
)

func NewApp() (platform.App, error) {
	return display.NewApp()
}
