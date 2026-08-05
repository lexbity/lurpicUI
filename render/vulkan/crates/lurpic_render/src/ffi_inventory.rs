//! FFI symbol inventory — the single source of truth for the C ABI surface.
//!
//! Every `#[no_mangle]` export must be listed here. `build.rs` serializes this
//! to OUT_DIR/ffi_inventory.json, and the Go-side drift gate
//! (`render/vulkan/ffi_gen_test.go`) regenerates the `ffi_linux.c` dlsym table
//! and the Go cgo declaration blocks from it, failing CI when they drift.
//!
//! Fields:
//! - `name`: the exported symbol (must match a `#[no_mangle]` fn in lib.rs).
//! - `ret`, `args`: the C signature as written in a C declaration.
//! - `platform`: `""` for all platforms, `"linux"` or `"android"` for
//!   platform-specific surface entry points.
//! - `test_only`: true when the symbol is gated behind the `test-exports`
//!   feature. The Linux dlsym loader treats these as optional so a production
//!   cdylib (built without `test-exports`) still loads.
//!
//! Keep this list sorted by symbol name.

#[allow(dead_code)] // consumed by build.rs codegen, the Go drift gate, and the drift test
pub struct FfiSymbol {
    pub name: &'static str,
    pub ret: &'static str,
    pub args: &'static str,
    pub platform: &'static str,
    pub test_only: bool,
}

#[allow(dead_code)]
pub const FFI_SYMBOLS: &[FfiSymbol] = &[
    FfiSymbol { name: "lurpic_render_create_image", ret: "int32_t", args: "const unsigned char *pixels, uintptr_t len, uint32_t width, uint32_t height, uint32_t stride, uint32_t format, uint64_t *out_handle", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_create_surface_android", ret: "int32_t", args: "void *window, uintptr_t instance, uint32_t width, uint32_t height, uintptr_t *out_surface", platform: "android", test_only: false },
    FfiSymbol { name: "lurpic_render_create_xcb_surface", ret: "int32_t", args: "uintptr_t instance, uintptr_t connection, uint32_t window, uint32_t width, uint32_t height, uintptr_t *out_surface", platform: "linux", test_only: false },
    FfiSymbol { name: "lurpic_render_destroy_image", ret: "int32_t", args: "uint64_t handle", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_device_generation", ret: "uint64_t", args: "void", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_init", ret: "int32_t", args: "void", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_instance_handle", ret: "uintptr_t", args: "void", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_last_error", ret: "const char *", args: "void", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_query_capabilities", ret: "int32_t", args: "void *out", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_query_pipeline_features", ret: "int32_t", args: "void *out", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_recreate_surface_android", ret: "int32_t", args: "void *window, uint32_t width, uint32_t height", platform: "android", test_only: false },
    FfiSymbol { name: "lurpic_render_reset_atlas", ret: "void", args: "void", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_resize", ret: "int32_t", args: "int32_t width, int32_t height", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_set_validation", ret: "int32_t", args: "uint32_t enabled", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_shutdown", ret: "int32_t", args: "void", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_submit_and_readback", ret: "int32_t", args: "const unsigned char *data, uintptr_t len, uint32_t width, uint32_t height, unsigned char *out_pixels, uintptr_t out_len", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_submit_frame", ret: "int32_t", args: "const unsigned char *data, uintptr_t len", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_test_destroy_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_drop_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_error", ret: "int32_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_glyph_atlas_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_glyph_atlas_evictions", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_handle_create", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_handle_destroy", ret: "int32_t", args: "uint64_t handle", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_handle_use", ret: "int32_t", args: "uint64_t handle", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_image_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_image_destroy_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_last_batch_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_last_command_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_last_vertex_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_ok", ret: "int32_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_panic", ret: "int32_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_reset", ret: "int32_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_test_validation_error_count", ret: "uint64_t", args: "void", platform: "", test_only: true },
    FfiSymbol { name: "lurpic_render_upload_glyph", ret: "int32_t", args: "uint64_t font_id, uint32_t glyph_id, uint32_t size_bits, uint32_t width, uint32_t height, float offset_x, float offset_y, float advance, const unsigned char *pixels, uintptr_t len", platform: "", test_only: false },
    FfiSymbol { name: "lurpic_render_version", ret: "const char *", args: "void", platform: "", test_only: false },
];

/// Symbols for a given platform, in inventory order.
#[allow(dead_code)]
pub fn symbols_for_platform(platform: &'static str) -> impl Iterator<Item = &'static FfiSymbol> {
    FFI_SYMBOLS
        .iter()
        .filter(move |s| s.platform.is_empty() || s.platform == platform)
}
