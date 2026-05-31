//go:build android && cgo

package vulkan

/*
#cgo LDFLAGS: -llog
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	char device_name[256];
	int32_t device_type;
	uint32_t api_version;
	uint32_t driver_version;
	uint32_t max_texture_dimension_2d;
	uint32_t graphics_queue_family_index;
	uint32_t present_queue_family_index;
	uint32_t transfer_queue_family_index;
} LurpicRenderCapabilities;

const char *lurpic_render_version(void);
const char *lurpic_render_last_error(void);
int lurpic_render_init(void);
int lurpic_render_shutdown(void);
uintptr_t lurpic_render_instance_handle(void);
int lurpic_render_query_capabilities(LurpicRenderCapabilities *out);
int lurpic_render_submit_frame(const unsigned char *data, uintptr_t len);
int lurpic_render_upload_glyph(uint64_t font_id, uint32_t glyph_id, uint32_t size_bits, uint32_t width, uint32_t height, float offset_x, float offset_y, float advance, const unsigned char *pixels, uintptr_t len);
int lurpic_render_create_image(const unsigned char *pixels, uintptr_t len, uint32_t width, uint32_t height, uint32_t stride, uint32_t format, uint64_t *out_handle);
int lurpic_render_destroy_image(uint64_t handle);
void lurpic_render_reset_atlas(void);
int lurpic_render_create_surface_android(void *android_window, uintptr_t instance, uint32_t width, uint32_t height, uintptr_t *out_surface);
int lurpic_render_recreate_surface_android(void *android_window, uint32_t width, uint32_t height);
int lurpic_render_resize(int width, int height);
int lurpic_render_present(void);
unsigned long long lurpic_render_device_generation(void);
void lurpic_render_unload(void);
*/
import "C"

import (
	"errors"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/render/vulkan/internal"
)

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

func Version() (string, error) {
	out := C.lurpic_render_version()
	if out == nil {
		return "", errors.New("vulkan: Android renderer returned a nil version string")
	}
	got := C.GoString(out)
	if got == "" {
		return "", errors.New("vulkan: Android renderer version string was empty")
	}
	return got, nil
}

func Init() error {
	return translateStatus(C.lurpic_render_init())
}

func Shutdown() error {
	return translateStatus(C.lurpic_render_shutdown())
}

func InstanceHandle() uintptr {
	return uintptr(C.lurpic_render_instance_handle())
}

func DeviceGeneration() uint64 {
	return uint64(C.lurpic_render_device_generation())
}

func QueryCapabilities() (Capabilities, error) {
	var caps C.LurpicRenderCapabilities
	if err := translateStatus(C.lurpic_render_query_capabilities(&caps)); err != nil {
		return Capabilities{}, err
	}
	return Capabilities{
		DeviceName:               C.GoString(&caps.device_name[0]),
		DeviceType:               int32(caps.device_type),
		APIVersion:               uint32(caps.api_version),
		DriverVersion:            uint32(caps.driver_version),
		MaxTextureDimension2D:    uint32(caps.max_texture_dimension_2d),
		GraphicsQueueFamilyIndex: uint32(caps.graphics_queue_family_index),
		PresentQueueFamilyIndex:  uint32(caps.present_queue_family_index),
		TransferQueueFamilyIndex: uint32(caps.transfer_queue_family_index),
	}, nil
}

func CreateAndroidSurface(window uintptr, instance uintptr, width, height uint32) (uintptr, error) {
	var out C.uintptr_t
	if err := translateStatus(C.lurpic_render_create_surface_android(
		unsafe.Pointer(window),
		C.uintptr_t(instance),
		C.uint32_t(width),
		C.uint32_t(height),
		&out,
	)); err != nil {
		return 0, err
	}
	if out == 0 {
		return 0, errors.New("vulkan: Rust returned a zero surface handle")
	}
	return uintptr(out), nil
}

func RecreateAndroidSurface(window uintptr, width, height uint32) error {
	return translateStatus(C.lurpic_render_recreate_surface_android(
		unsafe.Pointer(window),
		C.uint32_t(width),
		C.uint32_t(height),
	))
}

func Resize(width, height int) error {
	return translateStatus(C.lurpic_render_resize(C.int(width), C.int(height)))
}

func Present() error {
	return translateStatus(C.lurpic_render_present())
}

func SubmitFrame(data []byte) error {
	if len(data) == 0 {
		return translateStatus(C.lurpic_render_submit_frame((*C.uchar)(nil), 0))
	}
	return translateStatus(C.lurpic_render_submit_frame((*C.uchar)(&data[0]), C.uintptr_t(len(data))))
}

func ResetAtlas() {
	C.lurpic_render_reset_atlas()
}

func UploadGlyph(fontID uint64, glyphID uint32, sizeBits uint32, width, height int, offsetX, offsetY, advance float32, pixels []byte) error {
	if len(pixels) == 0 {
		return errors.New("vulkan: glyph pixel buffer is empty")
	}
	return translateStatus(C.lurpic_render_upload_glyph(
		C.uint64_t(fontID),
		C.uint32_t(glyphID),
		C.uint32_t(sizeBits),
		C.uint32_t(width),
		C.uint32_t(height),
		C.float(offsetX),
		C.float(offsetY),
		C.float(advance),
		(*C.uchar)(&pixels[0]),
		C.uintptr_t(len(pixels)),
	))
}

func UploadImage(pixels []byte, width, height, stride int, format uint32) (uint64, error) {
	if len(pixels) == 0 {
		return 0, errors.New("vulkan: image pixel buffer is empty")
	}
	var handle C.uint64_t
	if err := translateStatus(C.lurpic_render_create_image(
		(*C.uchar)(&pixels[0]),
		C.uintptr_t(len(pixels)),
		C.uint32_t(width),
		C.uint32_t(height),
		C.uint32_t(stride),
		C.uint32_t(format),
		&handle,
	)); err != nil {
		return 0, err
	}
	if handle == 0 {
		return 0, errors.New("vulkan: Rust returned a zero image handle")
	}
	return uint64(handle), nil
}

func DestroyImage(handle uint64) error {
	return translateStatus(C.lurpic_render_destroy_image(C.uint64_t(handle)))
}

func resetRustLibraryLoaderForTest() {}

func translateStatus(code C.int) error {
	return internal.TranslateResult(internal.ResultCode(code), cErrorMessage())
}

func cErrorMessage() string {
	if errPtr := C.lurpic_render_last_error(); errPtr != nil {
		return C.GoString(errPtr)
	}
	return ""
}
