//go:build android && !cgo

package vulkan

import "errors"

// Capabilities mirrors the C struct from ffi_android.go.
type Capabilities struct {
	DeviceName               string
	DeviceType               int32
	APIVersion               uint32
	DriverVersion            uint32
	MaxTextureDimension2D    uint32
	GraphicsQueueFamilyIndex uint32
	PresentQueueFamilyIndex  uint32
	TransferQueueFamilyIndex uint32
}

// No-op stubs for android without CGO. These satisfy the linker when the
// render/vulkan package is imported during CGO-disabled builds (e.g.,
// go vet on host with GOOS=android). Real rendering requires CGO.

var errCGORequired = errors.New("vulkan: requires CGO (android NDK)")

func Version() (string, error)          { return "", errCGORequired }
func Init() error                       { return errCGORequired }
func Shutdown() error                   { return nil }
func InstanceHandle() uintptr           { return 0 }
func DeviceGeneration() uint64          { return 0 }
func SubmitFrame([]byte) error          { return errCGORequired }
func Present() error                    { return errCGORequired }
func Resize(int, int) error             { return errCGORequired }
func ResetAtlas() {}
func UploadGlyph(uint64, uint32, uint32, int, int, float32, float32, float32, []byte) error {
	return errCGORequired
}
func UploadImage([]byte, int, int, int, uint32) (uint64, error) {
	return 0, errCGORequired
}
func DestroyImage(uint64) error      { return nil }
func CreateAndroidSurface(uintptr, uintptr, uint32, uint32) (uintptr, error) {
	return 0, errCGORequired
}
func RecreateAndroidSurface(uintptr, uint32, uint32) error {
	return errCGORequired
}
func QueryCapabilities() (Capabilities, error) {
	return Capabilities{}, errCGORequired
}
func resetRustLibraryLoaderForTest() {}
