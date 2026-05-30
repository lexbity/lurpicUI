//go:build linux && !android && cgo

package vulkan

/*
#cgo linux LDFLAGS: -ldl
#include <stdlib.h>
#include <stdint.h>

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

int lurpic_render_load(const char *library_path);
const char *lurpic_render_last_error(void);
const char *lurpic_render_version(void);
int lurpic_render_init(void);
int lurpic_render_shutdown(void);
uintptr_t lurpic_render_instance_handle(void);
int lurpic_render_query_capabilities(LurpicRenderCapabilities *out);
int lurpic_render_submit_frame(const unsigned char *data, uintptr_t len);
int lurpic_render_upload_glyph(uint64_t font_id, uint32_t glyph_id, uint32_t size_bits, uint32_t width, uint32_t height, float offset_x, float offset_y, float advance, const unsigned char *pixels, uintptr_t len);
int lurpic_render_create_image(const unsigned char *pixels, uintptr_t len, uint32_t width, uint32_t height, uint32_t stride, uint32_t format, uint64_t *out_handle);
int lurpic_render_destroy_image(uint64_t handle);
int lurpic_render_create_xcb_surface(uintptr_t instance, uintptr_t connection, uint32_t window, uint32_t width, uint32_t height, uintptr_t *out_surface);
int lurpic_render_resize(int width, int height);
int lurpic_render_present(void);
int lurpic_render_test_ok(void);
int lurpic_render_test_error(void);
int lurpic_render_test_panic(void);
unsigned long long lurpic_render_test_handle_create(void);
int lurpic_render_test_handle_use(unsigned long long handle);
int lurpic_render_test_handle_destroy(unsigned long long handle);
int lurpic_render_test_reset(void);
unsigned long long lurpic_render_test_destroy_count(void);
unsigned long long lurpic_render_test_drop_count(void);
unsigned long long lurpic_render_test_last_batch_count(void);
unsigned long long lurpic_render_test_last_command_count(void);
unsigned long long lurpic_render_test_last_vertex_count(void);
unsigned long long lurpic_render_test_glyph_atlas_count(void);
unsigned long long lurpic_render_test_glyph_atlas_evictions(void);
unsigned long long lurpic_render_test_image_count(void);
unsigned long long lurpic_render_test_image_destroy_count(void);
unsigned long long lurpic_render_device_generation(void) __attribute__((weak));
unsigned long long lurpic_render_device_generation(void) { return 0; }
void lurpic_render_unload(void);
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"
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

var rustLibraryLoadOnce sync.Once
var rustLibraryLoadErr error
var rustLibraryPathResolver = defaultRustLibraryPath

func defaultRustLibraryPath() (string, error) {
	if override := os.Getenv("LURPIC_RENDER_RUST_LIBRARY"); override != "" {
		if filepath.IsAbs(override) {
			return filepath.Clean(override), nil
		}
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("vulkan: resolve LURPIC_RENDER_RUST_LIBRARY: %w", err)
		}
		return abs, nil
	}

	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		return "", errors.New("vulkan: unable to determine repository root")
	}
	root := filepath.Clean(filepath.Dir(file))
	candidates := []string{
		filepath.Join(root, "crates", "lurpic_render", "target", "debug", rustSharedLibraryName()),
		filepath.Join(root, "crates", "lurpic_render", "target", "release", rustSharedLibraryName()),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"vulkan: rust library not found; tried %q and %q",
		candidates[0],
		candidates[1],
	)
}

func rustSharedLibraryName() string {
	return "liblurpic_render.so"
}

func loadRustLibrary() error {
	rustLibraryLoadOnce.Do(func() {
		path, err := rustLibraryPathResolver()
		if err != nil {
			rustLibraryLoadErr = err
			return
		}
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))
		if rc := C.lurpic_render_load(cPath); rc != 0 {
			msg := "vulkan: failed to load Rust library"
			if errPtr := C.lurpic_render_last_error(); errPtr != nil {
				msg = C.GoString(errPtr)
			}
			rustLibraryLoadErr = errors.New(msg)
			return
		}
	})
	return rustLibraryLoadErr
}

// Version reports the Rust-side renderer version string.
func Version() (string, error) {
	if err := loadRustLibrary(); err != nil {
		return "", err
	}
	version := C.lurpic_render_version()
	if version == nil {
		msg := "vulkan: Rust version function returned nil"
		if errPtr := C.lurpic_render_last_error(); errPtr != nil {
			msg = C.GoString(errPtr)
		}
		return "", errors.New(msg)
	}
	got := C.GoString(version)
	if got == "" {
		return "", errors.New("vulkan: Rust version string was empty")
	}
	return got, nil
}

func Init() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_init())
}

func Shutdown() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_shutdown())
}

func InstanceHandle() uintptr {
	if err := loadRustLibrary(); err != nil {
		return 0
	}
	return uintptr(C.lurpic_render_instance_handle())
}

func DeviceGeneration() uint64 {
	if err := loadRustLibrary(); err != nil {
		return 0
	}
	// The weak stub above returns 0 when the Rust library hasn't been rebuilt
	// with this symbol. Once rebuilt, the Rust implementation overrides the stub.
	return uint64(C.lurpic_render_device_generation())
}

func QueryCapabilities() (Capabilities, error) {
	if err := loadRustLibrary(); err != nil {
		return Capabilities{}, err
	}
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

func CreateXcbSurface(instance uintptr, connection uintptr, window uint32, width, height uint32) (uintptr, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	var out C.uintptr_t
	status := C.lurpic_render_create_xcb_surface(C.uintptr_t(instance), C.uintptr_t(connection), C.uint32_t(window), C.uint32_t(width), C.uint32_t(height), &out)
	if err := translateStatus(status); err != nil {
		return 0, err
	}
	if out == 0 {
		return 0, errors.New("vulkan: Rust returned a zero surface handle")
	}
	return uintptr(out), nil
}

func Resize(width, height int) error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_resize(C.int(width), C.int(height)))
}

func Present() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_present())
}

func SubmitFrame(data []byte) error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	if len(data) == 0 {
		return translateStatus(C.lurpic_render_submit_frame((*C.uchar)(nil), 0))
	}
	return translateStatus(C.lurpic_render_submit_frame((*C.uchar)(unsafe.Pointer(&data[0])), C.uintptr_t(len(data))))
}

func UploadGlyph(fontID uint64, glyphID uint32, sizeBits uint32, width, height int, offsetX, offsetY, advance float32, pixels []byte) error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	if width <= 0 || height <= 0 {
		return errors.New("vulkan: glyph dimensions must be positive")
	}
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
		(*C.uchar)(unsafe.Pointer(&pixels[0])),
		C.uintptr_t(len(pixels)),
	))
}

func UploadImage(pixels []byte, width, height, stride int, format uint32) (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if width <= 0 || height <= 0 {
		return 0, errors.New("vulkan: image dimensions must be positive")
	}
	if stride <= 0 {
		return 0, errors.New("vulkan: image stride must be positive")
	}
	if len(pixels) == 0 {
		return 0, errors.New("vulkan: image pixel buffer is empty")
	}
	var out C.uint64_t
	status := C.lurpic_render_create_image(
		(*C.uchar)(unsafe.Pointer(&pixels[0])),
		C.uintptr_t(len(pixels)),
		C.uint32_t(width),
		C.uint32_t(height),
		C.uint32_t(stride),
		C.uint32_t(format),
		&out,
	)
	if err := translateStatus(status); err != nil {
		return 0, err
	}
	if out == 0 {
		return 0, errors.New("vulkan: Rust returned a zero image handle")
	}
	return uint64(out), nil
}

func DestroyImage(handle uint64) error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_destroy_image(C.uint64_t(handle)))
}

func testResultOK() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_test_ok())
}

func testResultError() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_test_error())
}

func testResultPanic() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_test_panic())
}

func testHandleCreate() (internal.Handle, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	handle := internal.Handle(C.lurpic_render_test_handle_create())
	if msg := cErrorMessage(); msg != "" {
		return 0, internal.TranslateResult(internal.ResultUnknown, msg)
	}
	if handle == 0 {
		return 0, internal.TranslateResult(internal.ResultInvalidHandle, "Rust returned a zero handle")
	}
	return handle, nil
}

func testHandleUse(handle internal.Handle) error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_test_handle_use(C.ulonglong(handle)))
}

func testHandleDestroy(handle internal.Handle) error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_test_handle_destroy(C.ulonglong(handle)))
}

func testResetRustState() error {
	if err := loadRustLibrary(); err != nil {
		return err
	}
	return translateStatus(C.lurpic_render_test_reset())
}

func testDestroyCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_destroy_count()), nil
}

func testDropCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_drop_count()), nil
}

func testLastBatchCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_last_batch_count()), nil
}

func testLastCommandCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_last_command_count()), nil
}

func testLastVertexCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_last_vertex_count()), nil
}

func testGlyphAtlasCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_glyph_atlas_count()), nil
}

func testGlyphAtlasEvictions() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_glyph_atlas_evictions()), nil
}

func testImageCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_image_count()), nil
}

func testImageDestroyCount() (uint64, error) {
	if err := loadRustLibrary(); err != nil {
		return 0, err
	}
	if msg := cErrorMessage(); msg != "" {
		return 0, errors.New(msg)
	}
	return uint64(C.lurpic_render_test_image_destroy_count()), nil
}

func translateStatus(code C.int) error {
	return internal.TranslateResult(internal.ResultCode(code), cErrorMessage())
}

func cErrorMessage() string {
	if errPtr := C.lurpic_render_last_error(); errPtr != nil {
		return C.GoString(errPtr)
	}
	return ""
}

func resetRustLibraryLoaderForTest() {
	C.lurpic_render_unload()
	rustLibraryLoadOnce = sync.Once{}
	rustLibraryLoadErr = nil
	rustLibraryPathResolver = defaultRustLibraryPath
}
