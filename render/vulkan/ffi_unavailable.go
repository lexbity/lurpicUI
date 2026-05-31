//go:build (!linux || !cgo) && !android

package vulkan

import "errors"

func Version() (string, error) {
	return "", errors.New("vulkan: Rust bridge requires linux with cgo")
}

func Init() error {
	return errors.New("vulkan: Rust bridge requires linux with cgo")
}

func Shutdown() error {
	return nil
}

func InstanceHandle() uintptr {
	return 0
}

func DeviceGeneration() uint64 {
	return 0
}

func SubmitFrame([]byte) error {
	return errors.New("vulkan: Rust bridge requires linux with cgo")
}

func Present() error {
	return errors.New("vulkan: Rust bridge requires linux with cgo")
}

func Resize(int, int) error {
	return errors.New("vulkan: Rust bridge requires linux with cgo")
}

func UploadGlyph(fontID uint64, glyphID uint32, sizeBits uint32, width, height int, offsetX, offsetY, advance float32, pixels []byte) error {
	return errors.New("vulkan: Rust bridge requires linux with cgo")
}

func UploadImage(pixels []byte, width, height, stride int, format uint32) (uint64, error) {
	return 0, errors.New("vulkan: Rust bridge requires linux with cgo")
}

// Capabilities mirrors the C struct from ffi_linux.go for type compatibility.
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

func QueryCapabilities() (Capabilities, error) {
	return Capabilities{}, errors.New("vulkan: Rust bridge requires linux with cgo")
}

func DestroyImage(handle uint64) error {
	return nil
}

func resetRustLibraryLoaderForTest() {}
