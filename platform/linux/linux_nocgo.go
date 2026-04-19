//go:build linux && !cgo

package linux

import (
	"errors"

	"codeburg.org/lexbit/lurpicui/platform"
)

func NewApp() (platform.App, error) {
	return nil, errors.New("linux platform: cgo required")
}
