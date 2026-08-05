use std::ffi::c_char;

#[cfg(target_os = "android")]
use std::ffi::c_void;
#[cfg(any(test, feature = "test-exports"))]
use std::collections::HashMap;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::sync::atomic::{AtomicU64, Ordering};
#[cfg(any(test, feature = "test-exports"))]
use std::sync::atomic::AtomicUsize;
use std::sync::{Mutex, OnceLock};
#[cfg(any(test, feature = "test-exports"))]
use std::sync::Arc;

mod atlas;
mod error;
mod ffi_inventory;
mod frame;
mod geometry;
/// The ash integration layer (GpuContext isolation). Public so the integration
/// tests exercise the real pipeline; the C ABI surface is unaffected.
pub mod gpu;
#[cfg(feature = "cpu-fallback")]
mod raster;
mod image_store;
mod pipeline_cache;
#[cfg(feature = "cpu-fallback")]
mod tessellation;
mod vulkan;

pub type RenderHandle = u64;

/// Re-exports for the `GpuContext` isolation layer.
pub use error::vk_error;

/// Pipeline capability flags returned by `lurpic_render_query_pipeline_features`.
#[repr(C)]
#[derive(Clone, Copy, Debug, Default)]
pub struct LurpicRenderPipelineFeatures {
    pub dynamic_rendering: u32,
    pub synchronization2: u32,
    pub extended_dynamic_state: u32,
    pub msaa_2x: u32,
    pub msaa_4x: u32,
    pub msaa_8x: u32,
    pub stencil_fill: u32,
}

#[repr(i32)]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RenderResult {
    Ok = 0,
    InitFailed = 1,
    OutOfMemory = 2,
    InvalidHandle = 3,
    VulkanError = 4,
    Unsupported = 5,
    PacketVersionMismatch = 6,
    Panic = 1000,
    Unknown = 1001,
}

impl RenderResult {
    fn message(self) -> &'static str {
        match self {
            RenderResult::Ok => "ok",
            RenderResult::InitFailed => "init_failed",
            RenderResult::OutOfMemory => "out_of_memory",
            RenderResult::InvalidHandle => "invalid_handle",
            RenderResult::VulkanError => "vulkan_error",
            RenderResult::Unsupported => "unsupported",
            RenderResult::PacketVersionMismatch => "packet_version_mismatch",
            RenderResult::Panic => "panic",
            RenderResult::Unknown => "unknown",
        }
    }
}

const VERSION: &[u8] = b"lurpic_render 0.2.0\0";

static LAST_ERROR: OnceLock<Mutex<Vec<u8>>> = OnceLock::new();
#[cfg(any(test, feature = "test-exports"))]
static REGISTRY: OnceLock<HandleRegistry> = OnceLock::new();
static DEVICE_GENERATION: AtomicU64 = AtomicU64::new(0);
/// Set via `lurpic_render_set_validation`; honored at the next `init`.
static VALIDATION_ENABLED: AtomicU64 = AtomicU64::new(0);

fn last_error() -> &'static Mutex<Vec<u8>> {
    LAST_ERROR.get_or_init(|| Mutex::new(vec![0]))
}

fn lock_last_error() -> std::sync::MutexGuard<'static, Vec<u8>> {
    last_error().lock().unwrap_or_else(|e| e.into_inner())
}

fn set_last_error(message: impl AsRef<str>) {
    let mut buf = lock_last_error();
    buf.clear();
    buf.extend_from_slice(message.as_ref().as_bytes());
    buf.push(0);
}

pub(crate) fn clear_last_error() {
    set_last_error("");
}

fn last_error_ptr() -> *const c_char {
    let buf = lock_last_error();
    buf.as_ptr() as *const c_char
}

fn result_message(code: RenderResult, message: impl AsRef<str>) -> String {
    let message = message.as_ref().trim();
    if message.is_empty() {
        format!("vulkan: {}", code.message())
    } else {
        format!("vulkan: {}: {}", code.message(), message)
    }
}

fn catch_render_result<F>(op: &str, f: F) -> RenderResult
where
    F: FnOnce() -> Result<(), (RenderResult, String)>,
{
    match catch_unwind(AssertUnwindSafe(f)) {
        Ok(Ok(())) => {
            clear_last_error();
            RenderResult::Ok
        }
        Ok(Err((code, message))) => {
            set_last_error(result_message(code, message));
            code
        }
        Err(payload) => {
            let message = panic_message(payload);
            set_last_error(format!("vulkan: panic in {}: {}", op, message));
            RenderResult::Panic
        }
    }
}

fn panic_message(payload: Box<dyn std::any::Any + Send>) -> String {
    let payload = payload.as_ref();
    if let Some(message) = payload.downcast_ref::<&str>() {
        return (*message).to_string();
    }
    if let Some(message) = payload.downcast_ref::<String>() {
        return message.clone();
    }
    "unknown panic payload".to_string()
}

/// Serializes tests that mutate the process-global atlas / image store so the
/// parallel unit-test harness cannot interleave destructive resets. Also held by
/// `lurpic_render_test_reset` so FFI-driven resets cannot race Rust unit tests.
#[cfg(any(test, feature = "test-exports"))]
pub(crate) fn state_lock_guard() -> std::sync::MutexGuard<'static, ()> {
    use std::sync::{Mutex, OnceLock};
    static STATE_LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    STATE_LOCK
        .get_or_init(|| Mutex::new(()))
        .lock()
        .unwrap_or_else(|e| e.into_inner())
}

#[cfg(any(test, feature = "test-exports"))]
struct HandleRegistry {
    next: AtomicU64,
    entries: Mutex<HashMap<RenderHandle, TestResource>>,
    destroy_count: AtomicUsize,
    drop_count: Arc<AtomicUsize>,
}

#[cfg(any(test, feature = "test-exports"))]
impl HandleRegistry {
    fn new() -> Self {
        Self {
            next: AtomicU64::new(1),
            entries: Mutex::new(HashMap::new()),
            destroy_count: AtomicUsize::new(0),
            drop_count: Arc::new(AtomicUsize::new(0)),
        }
    }

    fn lock_entries(&self) -> std::sync::MutexGuard<'_, HashMap<RenderHandle, TestResource>> {
        self.entries.lock().unwrap_or_else(|e| e.into_inner())
    }

    fn create_test_handle(&self) -> RenderHandle {
        let handle = self.next.fetch_add(1, Ordering::Relaxed);
        let mut entries = self.lock_entries();
        entries.insert(
            handle,
            TestResource {
                destroyed: false,
                drop_count: Arc::clone(&self.drop_count),
            },
        );
        handle
    }

    fn use_handle(&self, handle: RenderHandle) -> Result<(), (RenderResult, String)> {
        let entries = self.lock_entries();
        if entries.contains_key(&handle) {
            return Ok(());
        }
        Err((
            RenderResult::InvalidHandle,
            format!("handle {} does not exist", handle),
        ))
    }

    fn destroy_handle(&self, handle: RenderHandle) -> Result<(), (RenderResult, String)> {
        let mut entries = self.lock_entries();
        let Some(mut resource) = entries.remove(&handle) else {
            return Err((
                RenderResult::InvalidHandle,
                format!("handle {} does not exist", handle),
            ));
        };
        resource.destroy();
        self.destroy_count.fetch_add(1, Ordering::Relaxed);
        Ok(())
    }

    fn clear(&self) {
        let mut entries = self.entries.lock().expect("registry mutex poisoned");
        entries.clear();
    }

    fn destroy_count(&self) -> u64 {
        self.destroy_count.load(Ordering::Relaxed) as u64
    }

    fn drop_count(&self) -> u64 {
        self.drop_count.load(Ordering::Relaxed) as u64
    }
}

#[cfg(any(test, feature = "test-exports"))]
struct TestResource {
    destroyed: bool,
    drop_count: Arc<AtomicUsize>,
}

#[cfg(any(test, feature = "test-exports"))]
impl TestResource {
    fn destroy(&mut self) {
        self.destroyed = true;
    }
}

#[cfg(any(test, feature = "test-exports"))]
impl Drop for TestResource {
    fn drop(&mut self) {
        if !self.destroyed {
            self.drop_count.fetch_add(1, Ordering::Relaxed);
        }
    }
}

#[cfg(any(test, feature = "test-exports"))]
fn registry() -> &'static HandleRegistry {
    REGISTRY.get_or_init(HandleRegistry::new)
}

#[repr(C)]
#[derive(Clone, Copy)]
pub struct LurpicRenderCapabilities {
    pub device_name: [c_char; 256],
    pub device_type: i32,
    pub api_version: u32,
    pub driver_version: u32,
    pub max_texture_dimension_2d: u32,
    pub graphics_queue_family_index: u32,
    pub present_queue_family_index: u32,
    pub transfer_queue_family_index: u32,
}

#[no_mangle]
pub extern "C" fn lurpic_render_version() -> *const c_char {
    VERSION.as_ptr() as *const c_char
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_ok() -> RenderResult {
    catch_render_result("test_ok", || Ok(()))
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_error() -> RenderResult {
    catch_render_result("test_error", || {
        Err((
            RenderResult::InitFailed,
            "simulated initialization failure".to_string(),
        ))
    })
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_panic() -> RenderResult {
    catch_render_result("test_panic", || -> Result<(), (RenderResult, String)> {
        panic!("simulated boundary panic")
    })
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_handle_create() -> RenderHandle {
    clear_last_error();
    registry().create_test_handle()
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_handle_use(handle: RenderHandle) -> RenderResult {
    catch_render_result("test_handle_use", || registry().use_handle(handle))
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_handle_destroy(handle: RenderHandle) -> RenderResult {
    catch_render_result("test_handle_destroy", || registry().destroy_handle(handle))
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_reset() -> RenderResult {
    catch_render_result("test_reset", || {
        let _guard = state_lock_guard();
        registry().clear();
        atlas::reset_atlas();
        image_store::reset_images();
        vulkan::shutdown().ok();
        clear_last_error();
        Ok(())
    })
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_destroy_count() -> u64 {
    clear_last_error();
    registry().destroy_count()
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_drop_count() -> u64 {
    clear_last_error();
    registry().drop_count()
}

#[no_mangle]
pub extern "C" fn lurpic_render_last_error() -> *const c_char {
    last_error_ptr()
}

#[no_mangle]
pub extern "C" fn lurpic_render_init() -> RenderResult {
    let result = catch_render_result("init", || {
        vulkan::init(VALIDATION_ENABLED.load(Ordering::SeqCst) != 0)
    });
    if result == RenderResult::Ok {
        DEVICE_GENERATION.fetch_add(1, Ordering::SeqCst);
    }
    result
}

/// Enables/disables the Khronos validation layer for the NEXT `init`. The flag
/// is latched at instance creation; changing it requires init/shutdown.
#[no_mangle]
pub extern "C" fn lurpic_render_set_validation(enabled: u32) -> RenderResult {
    catch_render_result("set_validation", || {
        VALIDATION_ENABLED.store((enabled != 0) as u64, Ordering::SeqCst);
        Ok(())
    })
}

/// Reports the physical device's pipeline-relevant capabilities (honest
/// backend selection, FR-11).
#[no_mangle]
pub extern "C" fn lurpic_render_query_pipeline_features(
    out: *mut LurpicRenderPipelineFeatures,
) -> RenderResult {
    catch_render_result("query_pipeline_features", || {
        if out.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "pipeline features pointer is null".to_string(),
            ));
        }
        let features = vulkan::pipeline_features()?;
        unsafe {
            *out = LurpicRenderPipelineFeatures {
                dynamic_rendering: features.dynamic_rendering as u32,
                synchronization2: features.synchronization2 as u32,
                extended_dynamic_state: features.extended_dynamic_state as u32,
                msaa_2x: features.msaa_2x as u32,
                msaa_4x: features.msaa_4x as u32,
                msaa_8x: features.msaa_8x as u32,
                stencil_fill: features.stencil_fill as u32,
            };
        }
        Ok(())
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_shutdown() -> RenderResult {
    catch_render_result("shutdown", || vulkan::shutdown())
}

#[no_mangle]
pub extern "C" fn lurpic_render_instance_handle() -> usize {
    clear_last_error();
    vulkan::instance_handle()
}

#[no_mangle]
pub extern "C" fn lurpic_render_query_capabilities(
    out: *mut LurpicRenderCapabilities,
) -> RenderResult {
    catch_render_result("query_capabilities", || {
        if out.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "capabilities pointer is null".to_string(),
            ));
        }
        let mut caps = vulkan::VulkanCapabilities::empty();
        vulkan::query_capabilities(&mut caps as *mut _ as *mut _)?;
        unsafe {
            *out = LurpicRenderCapabilities {
                device_name: caps.device_name,
                device_type: caps.device_type,
                api_version: caps.api_version,
                driver_version: caps.driver_version,
                max_texture_dimension_2d: caps.max_texture_dimension_2d,
                graphics_queue_family_index: caps.graphics_queue_family_index,
                present_queue_family_index: caps.present_queue_family_index,
                transfer_queue_family_index: caps.transfer_queue_family_index,
            };
        }
        Ok(())
    })
}

#[cfg(not(target_os = "android"))]
#[no_mangle]
pub extern "C" fn lurpic_render_create_xcb_surface(
    instance: usize,
    connection: usize,
    window: u32,
    width: u32,
    height: u32,
    out_surface: *mut usize,
) -> RenderResult {
    catch_render_result("create_xcb_surface", || {
        if out_surface.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "surface output pointer is null".to_string(),
            ));
        }
        let surface = vulkan::create_xcb_surface(instance, connection, window, width, height)?;
        unsafe {
            *out_surface = surface;
        }
        Ok(())
    })
}

#[cfg(target_os = "android")]
#[no_mangle]
pub extern "C" fn lurpic_render_create_surface_android(
    android_window: *mut c_void,
    instance: usize,
    width: u32,
    height: u32,
    out_surface: *mut usize,
) -> RenderResult {
    catch_render_result("create_surface_android", || {
        if out_surface.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "surface output pointer is null".to_string(),
            ));
        }
        let surface = vulkan::create_android_surface(instance, android_window, width, height)?;
        unsafe {
            *out_surface = surface;
        }
        Ok(())
    })
}

#[cfg(target_os = "android")]
#[no_mangle]
pub extern "C" fn lurpic_render_recreate_surface_android(
    android_window: *mut c_void,
    width: u32,
    height: u32,
) -> RenderResult {
    let result = catch_render_result("recreate_surface_android", || {
        vulkan::recreate_surface_android(android_window, width, height)
    });
    if result == RenderResult::Ok {
        DEVICE_GENERATION.fetch_add(1, Ordering::SeqCst);
    }
    result
}

#[no_mangle]
pub extern "C" fn lurpic_render_resize(width: i32, height: i32) -> RenderResult {
    catch_render_result("resize", || vulkan::resize(width, height))
}

#[no_mangle]
pub extern "C" fn lurpic_render_submit_frame(data: *const u8, len: usize) -> RenderResult {
    catch_render_result("submit_frame", || vulkan::submit_frame(data, len))
}

/// Test-only readback entry point: decodes a packet v2 frame, rasterizes it with
/// the CPU stepping-stone raster (transparent clear), and writes RGBA pixels to
/// `out_pixels`. This is the GPU-readback contract the equivalence harness uses
/// until the GPU pipeline lands; it does not require a Vulkan device.
#[cfg(feature = "cpu-fallback")]
#[no_mangle]
pub extern "C" fn lurpic_render_submit_and_readback(
    data: *const u8,
    len: usize,
    width: u32,
    height: u32,
    out_pixels: *mut u8,
    out_len: usize,
) -> RenderResult {
    catch_render_result("submit_and_readback", || {
        if out_pixels.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "readback output pointer is null".to_string(),
            ));
        }
        let required = (width as usize)
            .checked_mul(height as usize)
            .and_then(|v| v.checked_mul(4))
            .ok_or((
                RenderResult::OutOfMemory,
                "readback buffer size overflow".to_string(),
            ))?;
        if out_len < required {
            return Err((
                RenderResult::InvalidHandle,
                format!(
                    "readback buffer too small: have {}, need {}",
                    out_len, required
                ),
            ));
        }
        if width == 0 || height == 0 {
            return Err((
                RenderResult::InitFailed,
                "readback dimensions are zero".to_string(),
            ));
        }
        if len > 0 && data.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "frame packet pointer is null".to_string(),
            ));
        }
        let bytes = if len == 0 {
            &[][..]
        } else {
            unsafe { std::slice::from_raw_parts(data, len) }
        };
        let frame = frame::decode_frame(bytes)?;
        let pixels = raster::rasterize_frame_with_clear(Some(&frame), width, height, [0, 0, 0, 0]);
        // The raster produces BGRA (swapchain layout). Readback is RGBA.
        let out = unsafe { std::slice::from_raw_parts_mut(out_pixels, out_len) };
        for (i, px) in pixels.chunks_exact(4).enumerate() {
            let off = i * 4;
            out[off] = px[2];
            out[off + 1] = px[1];
            out[off + 2] = px[0];
            out[off + 3] = px[3];
        }
        Ok(())
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_upload_glyph(
    font_id: u64,
    glyph_id: u32,
    size_bits: u32,
    width: u32,
    height: u32,
    offset_x: f32,
    offset_y: f32,
    advance: f32,
    pixels: *const u8,
    len: usize,
) -> RenderResult {
    catch_render_result("upload_glyph", || {
        if width == 0 || height == 0 {
            return Err((
                RenderResult::InitFailed,
                "glyph dimensions are zero".to_string(),
            ));
        }
        if pixels.is_null() && len != 0 {
            return Err((
                RenderResult::InvalidHandle,
                "glyph pixel pointer is null".to_string(),
            ));
        }
        let data = if len == 0 {
            &[][..]
        } else {
            unsafe { std::slice::from_raw_parts(pixels, len) }
        };
        let expected = (width as usize) * (height as usize);
        if data.len() < expected {
            return Err((
                RenderResult::InitFailed,
                "glyph bitmap is truncated".to_string(),
            ));
        }
        atlas::upload_glyph(
            font_id,
            glyph_id,
            size_bits,
            atlas::GlyphBitmap {
                width,
                height,
                pixels: data[..expected].to_vec(),
                offset_x,
                offset_y,
                advance,
            },
        );
        Ok(())
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_create_image(
    pixels: *const u8,
    len: usize,
    width: u32,
    height: u32,
    stride: u32,
    format: u32,
    out_handle: *mut u64,
) -> RenderResult {
    catch_render_result("create_image", || {
        if out_handle.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "image output pointer is null".to_string(),
            ));
        }
        if pixels.is_null() && len != 0 {
            return Err((
                RenderResult::InvalidHandle,
                "image pixel pointer is null".to_string(),
            ));
        }
        let data = if len == 0 {
            &[][..]
        } else {
            unsafe { std::slice::from_raw_parts(pixels, len) }
        };
        let format = match format {
            0 => image_store::ImageFormat::Rgba8,
            1 => image_store::ImageFormat::Bgra8,
            _ => {
                return Err((
                    RenderResult::InitFailed,
                    format!("unsupported image format {}", format),
                ));
            }
        };
        let handle = image_store::create_image(data, width, height, stride, format)?;
        unsafe {
            *out_handle = handle;
        }
        Ok(())
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_destroy_image(handle: u64) -> RenderResult {
    catch_render_result("destroy_image", || image_store::destroy_image(handle))
}

#[no_mangle]
pub extern "C" fn lurpic_render_reset_atlas() {
    atlas::reset_atlas();
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_glyph_atlas_count() -> u64 {
    clear_last_error();
    atlas::atlas_stats().0 as u64
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_glyph_atlas_evictions() -> u64 {
    clear_last_error();
    atlas::atlas_stats().1 as u64
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_image_count() -> u64 {
    clear_last_error();
    image_store::image_stats().0 as u64
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_image_destroy_count() -> u64 {
    clear_last_error();
    image_store::image_stats().1 as u64
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_last_batch_count() -> u64 {
    clear_last_error();
    vulkan::frame_stats().batch_count as u64
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_last_command_count() -> u64 {
    clear_last_error();
    vulkan::frame_stats().command_count as u64
}

#[cfg(all(feature = "test-exports", feature = "cpu-fallback"))]
#[no_mangle]
pub extern "C" fn lurpic_render_test_last_vertex_count() -> u64 {
    clear_last_error();
    vulkan::frame_stats().vertex_count as u64
}

#[cfg(feature = "test-exports")]
#[no_mangle]
pub extern "C" fn lurpic_render_test_validation_error_count() -> u64 {
    clear_last_error();
    gpu::validation::validation_error_count() as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_device_generation() -> u64 {
    DEVICE_GENERATION.load(Ordering::SeqCst)
}

#[cfg(all(test, feature = "test-exports"))]
mod tests {
    use super::*;
    use std::ffi::CStr;
    use std::sync::{Mutex, OnceLock};

    static TEST_LOCK: OnceLock<Mutex<()>> = OnceLock::new();

    fn test_guard() -> std::sync::MutexGuard<'static, ()> {
        TEST_LOCK
            .get_or_init(|| Mutex::new(()))
            .lock()
            .expect("test mutex poisoned")
    }

    #[test]
    fn ffi_inventory_covers_all_exports() {
        // Every #[no_mangle] export in this crate must be listed in the FFI
        // inventory (the single source of truth for the C ABI codegen). This
        // is the Rust-side half of the drift gate (the Go side regenerates
        // ffi_linux.c from the inventory).
        let manifest = std::env::var("CARGO_MANIFEST_DIR").unwrap();
        let source = std::fs::read_to_string(std::path::Path::new(&manifest).join("src/lib.rs"))
            .expect("read lib.rs");
        let mut exports = std::collections::BTreeSet::new();
        let mut rest = source.as_str();
        while let Some(rel) = rest.find("pub extern \"C\" fn lurpic_render_") {
            rest = &rest[rel + "pub extern \"C\" fn ".len()..];
            let end = rest.find('(').expect("fn signature");
            let name = &rest[..end];
            exports.insert(name.to_string());
        }
        for symbol in ffi_inventory::FFI_SYMBOLS {
            let short = symbol.name.trim_start_matches("lurpic_render_");
            if short.is_empty() {
                continue;
            }
            exports.remove(&symbol.name.to_string());
        }
        assert!(
            exports.is_empty(),
            "exports missing from the FFI inventory: {:?}",
            exports
        );
    }

    #[test]
    fn version_is_non_empty() {
        let _guard = test_guard();
        let ptr = lurpic_render_version();
        assert!(!ptr.is_null());
        let version = unsafe { CStr::from_ptr(ptr) };
        let version = version.to_str().expect("version is valid utf-8");
        assert!(!version.trim().is_empty());
    }

    #[test]
    fn ok_result_has_no_error() {
        let _guard = test_guard();
        assert_eq!(lurpic_render_test_ok(), RenderResult::Ok);
        assert_eq!(
            unsafe { CStr::from_ptr(lurpic_render_last_error()) }
                .to_str()
                .unwrap(),
            ""
        );
    }

    #[test]
    fn error_result_sets_message() {
        let _guard = test_guard();
        assert_eq!(lurpic_render_test_error(), RenderResult::InitFailed);
        let msg = unsafe { CStr::from_ptr(lurpic_render_last_error()) }
            .to_str()
            .unwrap()
            .to_string();
        assert!(msg.contains("init_failed"));
    }

    #[test]
    fn panic_result_is_caught() {
        let _guard = test_guard();
        assert_eq!(lurpic_render_test_panic(), RenderResult::Panic);
        let msg = unsafe { CStr::from_ptr(lurpic_render_last_error()) }
            .to_str()
            .unwrap()
            .to_string();
        assert!(msg.contains("panic in test_panic"));
    }

    #[test]
    fn handles_validate_and_destroy() {
        let _guard = test_guard();
        let baseline_destroy = lurpic_render_test_destroy_count();
        let baseline_drop = lurpic_render_test_drop_count();

        assert_eq!(lurpic_render_test_reset(), RenderResult::Ok);
        let handle = lurpic_render_test_handle_create();
        assert_ne!(handle, 0);
        assert_eq!(lurpic_render_test_handle_use(handle), RenderResult::Ok);
        assert_eq!(lurpic_render_test_handle_destroy(handle), RenderResult::Ok);
        assert_eq!(
            lurpic_render_test_handle_use(handle),
            RenderResult::InvalidHandle
        );
        assert_eq!(lurpic_render_test_destroy_count(), baseline_destroy + 1);
        assert_eq!(lurpic_render_test_drop_count(), baseline_drop);
    }

    #[test]
    fn invalid_handle_is_reported() {
        let _guard = test_guard();
        assert_eq!(lurpic_render_test_reset(), RenderResult::Ok);
        assert_eq!(
            lurpic_render_test_handle_use(0xdead_beef),
            RenderResult::InvalidHandle
        );
    }
}
