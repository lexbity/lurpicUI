use std::collections::HashMap;
use std::ffi::{c_char, c_void};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, OnceLock};

mod atlas;
mod frame;
mod image_store;
mod raster;
mod tessellation;
mod vulkan;

pub type RenderHandle = u64;

#[repr(i32)]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RenderResult {
    Ok = 0,
    InitFailed = 1,
    OutOfMemory = 2,
    InvalidHandle = 3,
    VulkanError = 4,
    Unsupported = 5,
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
            RenderResult::Panic => "panic",
            RenderResult::Unknown => "unknown",
        }
    }
}

const VERSION: &[u8] = b"lurpic_render 0.2.0\0";

static LAST_ERROR: OnceLock<Mutex<Vec<u8>>> = OnceLock::new();
static REGISTRY: OnceLock<HandleRegistry> = OnceLock::new();

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

fn clear_last_error() {
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

struct HandleRegistry {
    next: AtomicU64,
    entries: Mutex<HashMap<RenderHandle, TestResource>>,
    destroy_count: AtomicUsize,
    drop_count: Arc<AtomicUsize>,
}

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

struct TestResource {
    destroyed: bool,
    drop_count: Arc<AtomicUsize>,
}

impl TestResource {
    fn destroy(&mut self) {
        self.destroyed = true;
    }
}

impl Drop for TestResource {
    fn drop(&mut self) {
        if !self.destroyed {
            self.drop_count.fetch_add(1, Ordering::Relaxed);
        }
    }
}

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

#[no_mangle]
pub extern "C" fn lurpic_render_test_ok() -> RenderResult {
    catch_render_result("test_ok", || Ok(()))
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_error() -> RenderResult {
    catch_render_result("test_error", || {
        Err((
            RenderResult::InitFailed,
            "simulated initialization failure".to_string(),
        ))
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_panic() -> RenderResult {
    catch_render_result("test_panic", || -> Result<(), (RenderResult, String)> {
        panic!("simulated boundary panic")
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_handle_create() -> RenderHandle {
    clear_last_error();
    registry().create_test_handle()
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_handle_use(handle: RenderHandle) -> RenderResult {
    catch_render_result("test_handle_use", || registry().use_handle(handle))
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_handle_destroy(handle: RenderHandle) -> RenderResult {
    catch_render_result("test_handle_destroy", || registry().destroy_handle(handle))
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_reset() -> RenderResult {
    catch_render_result("test_reset", || {
        registry().clear();
        atlas::reset_atlas();
        image_store::reset_images();
        vulkan::shutdown().ok();
        clear_last_error();
        Ok(())
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_destroy_count() -> u64 {
    clear_last_error();
    registry().destroy_count()
}

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
    catch_render_result("init", || vulkan::init())
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

#[no_mangle]
pub extern "C" fn lurpic_render_create_surface_android(
    android_window: *mut c_void,
    instance: usize,
    width: u32,
    height: u32,
    out_surface: *mut usize,
) -> RenderResult {
    catch_render_result("create_surface_android", || {
        let _ = (android_window, instance, width, height);
        if out_surface.is_null() {
            return Err((
                RenderResult::InvalidHandle,
                "surface output pointer is null".to_string(),
            ));
        }
        Err((
            RenderResult::InitFailed,
            "android surface creation is not implemented yet".to_string(),
        ))
    })
}

#[no_mangle]
pub extern "C" fn lurpic_render_resize(width: i32, height: i32) -> RenderResult {
    catch_render_result("resize", || vulkan::resize(width, height))
}

#[no_mangle]
pub extern "C" fn lurpic_render_present() -> RenderResult {
    catch_render_result("present", || vulkan::present())
}

#[no_mangle]
pub extern "C" fn lurpic_render_submit_frame(data: *const u8, len: usize) -> RenderResult {
    catch_render_result("submit_frame", || vulkan::submit_frame(data, len))
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
pub extern "C" fn lurpic_render_test_glyph_atlas_count() -> u64 {
    clear_last_error();
    atlas::atlas_stats().0 as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_glyph_atlas_evictions() -> u64 {
    clear_last_error();
    atlas::atlas_stats().1 as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_image_count() -> u64 {
    clear_last_error();
    image_store::image_stats().0 as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_image_destroy_count() -> u64 {
    clear_last_error();
    image_store::image_stats().1 as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_last_batch_count() -> u64 {
    clear_last_error();
    vulkan::frame_stats().batch_count as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_last_command_count() -> u64 {
    clear_last_error();
    vulkan::frame_stats().command_count as u64
}

#[no_mangle]
pub extern "C" fn lurpic_render_test_last_vertex_count() -> u64 {
    clear_last_error();
    vulkan::frame_stats().vertex_count as u64
}

#[cfg(test)]
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
