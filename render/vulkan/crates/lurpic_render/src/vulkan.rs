//! The renderer's Vulkan lifecycle: instance/device/queue/swapchain + the GPU
//! pipeline (Slice 3: solid rects / stroke rects via instanced quads).
//!
//! `gpu::context::AshContext` owns instance/device/queue/allocator; this module
//! adds the surface/swapchain layer, the MSAA render target, the per-frame
//! instance ring, and the command-buffer recording for present and readback.

#[cfg(target_os = "android")]
use std::ffi::c_void;
use std::sync::{Mutex, OnceLock};

use ash::vk;
use ash::vk::Handle;

use crate::atlas::GlyphAtlas;
use crate::clear_last_error;
use crate::error::vk_error;
use crate::frame::{decode_frame, DecodedFrame};
#[cfg(feature = "test-exports")]
use crate::frame::FrameStats;
use crate::frame_encoder::{DrawKind, FrameEncoder, ShadowFill, Textures};
use crate::gpu::allocator::{GpuBuffer, ImageAllocation, MemoryLocation};
use crate::gpu::context::{AshContext, GpuContext, PhysicalDeviceFeatures};
use crate::gpu::resources::{DescriptorPool, Image, ImageView, Sampler};
use crate::gpu::surface;
use crate::image_store::{ImageFormat, ImageStore};
use crate::pipeline::blur::{BlurPipeline, BLUR_SCRATCH_FORMAT};
use crate::pipeline::glyph::GlyphPipeline;
use crate::pipeline::gradient::GradientPipeline;
use crate::pipeline::solid::SolidPipeline;
use crate::pipeline::stencil::PathFillPipeline;
use crate::pipeline::textured::TexturedPipeline;
use crate::pipeline::{
    PushConstants, STENCIL_FORMAT, BRUSH_LINEAR_GRADIENT,
};
use crate::ring_buffer::{
    InstanceRing, PathRing, UniformRing, DEFAULT_PATH_SLOT_BYTES, DEFAULT_SLOT_BYTES,
    DEFAULT_UNIFORM_SLOT_BYTES,
};
use crate::RenderResult;

/// Per-ring-slot descriptor sets for textured groups (Slice 4). Each slot's
/// pool is reset after its fence signals; 2048 sets comfortably covers any
/// realistic (image, push-state) combination per frame.
const TEXTURED_DESCRIPTOR_SETS_PER_POOL: u32 = 2048;

/// Test-only flag: render the solid pipeline from the RG-swapped fragment
/// shader so the equivalence negative control exercises the real shader
/// toolchain.
#[cfg(feature = "test-exports")]
static FORCE_SWAPPED: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

#[cfg(feature = "test-exports")]
pub fn test_force_swapped_rendering(enabled: bool) {
    FORCE_SWAPPED.store(enabled, std::sync::atomic::Ordering::Relaxed);
}

#[cfg(feature = "test-exports")]
fn force_swapped() -> bool {
    FORCE_SWAPPED.load(std::sync::atomic::Ordering::Relaxed)
}

#[cfg(not(feature = "test-exports"))]
fn force_swapped() -> bool {
    false
}

#[derive(Clone)]
pub struct VulkanCapabilities {
    pub device_name: [std::ffi::c_char; 256],
    pub device_type: i32,
    pub api_version: u32,
    pub driver_version: u32,
    pub max_texture_dimension_2d: u32,
    pub graphics_queue_family_index: u32,
    pub present_queue_family_index: u32,
    pub transfer_queue_family_index: u32,
}

impl VulkanCapabilities {
    pub fn empty() -> Self {
        Self {
            device_name: [0; 256],
            device_type: 0,
            api_version: 0,
            driver_version: 0,
            max_texture_dimension_2d: 0,
            graphics_queue_family_index: 0,
            present_queue_family_index: 0,
            transfer_queue_family_index: 0,
        }
    }
}

/// The MSAA color attachment the solid pipeline renders into; resolved to the
/// swapchain (present) or the readback image. Drop order: view, image, memory.
struct MsaaTarget {
    view: ImageView,
    #[allow(dead_code)] // held for RAII drop ordering
    image: Image,
    #[allow(dead_code)] // held for RAII drop ordering
    allocation: ImageAllocation,
    extent: vk::Extent2D,
    format: vk::Format,
    samples: vk::SampleCountFlags,
}

/// The offscreen render target + staging buffer for GPU readback.
struct ReadbackTarget {
    view: ImageView,
    #[allow(dead_code)] // held for RAII drop ordering
    image: Image,
    #[allow(dead_code)] // held for RAII drop ordering
    allocation: ImageAllocation,
    staging: GpuBuffer,
    extent: vk::Extent2D,
    format: vk::Format,
}

/// The solid-pipeline handles needed for recording a frame (Copy so no
/// reference into `self` outlives the mutable render borrow).
#[derive(Clone, Copy)]
struct SolidHandles {
    handle: vk::Pipeline,
    layout: vk::PipelineLayout,
    unit_quad: vk::Buffer,
    samples: vk::SampleCountFlags,
}

/// A copy of the textured pipeline's record handles for the render loop.
#[derive(Clone, Copy)]
struct TexturedHandles {
    handle: vk::Pipeline,
    layout: vk::PipelineLayout,
    set_layout: vk::DescriptorSetLayout,
    unit_quad: vk::Buffer,
}

/// A copy of the glyph pipeline's record handles for the render loop.
#[derive(Clone, Copy)]
struct GlyphHandles {
    bitmap: vk::Pipeline,
    sdf: vk::Pipeline,
    layout: vk::PipelineLayout,
    unit_quad: vk::Buffer,
}

/// A copy of the gradient pipeline's record handles for the render loop.
#[derive(Clone, Copy)]
struct GradientHandles {
    handle: vk::Pipeline,
    layout: vk::PipelineLayout,
    set_layout: vk::DescriptorSetLayout,
    unit_quad: vk::Buffer,
}

/// A copy of the path-fill pipeline's record handles for the render loop.
#[derive(Clone, Copy)]
struct PathFillHandles {
    stencil: vk::Pipeline,
    stencil_layout: vk::PipelineLayout,
    cover_solid: vk::Pipeline,
    cover_gradient: vk::Pipeline,
    cover_layout: vk::PipelineLayout,
    cover_gradient_layout: vk::PipelineLayout,
    segments_layout: vk::DescriptorSetLayout,
    unit_quad: vk::Buffer,
}

/// The stencil attachment (D24S8, depth aspect unused) used by path fills.
struct StencilTarget {
    view: ImageView,
    #[allow(dead_code)] // held for RAII drop ordering
    image: Image,
    #[allow(dead_code)] // held for RAII drop ordering
    allocation: ImageAllocation,
    extent: vk::Extent2D,
}

/// One R8 scratch image of the shadow blur pair (Slice 9): full-surface,
/// COLOR_ATTACHMENT + SAMPLED. Drop order: view, image, memory.
struct BlurScratch {
    view: ImageView,
    #[allow(dead_code)] // held for RAII drop ordering
    image: Image,
    #[allow(dead_code)] // held for RAII drop ordering
    allocation: ImageAllocation,
    extent: vk::Extent2D,
}

/// The shadow-blur pipeline handles needed for recording a frame (Copy so no
/// reference into `self` outlives the mutable render borrow).
#[derive(Clone, Copy)]
struct BlurHandles {
    mask: vk::Pipeline,
    mask_layout: vk::PipelineLayout,
    blur_h: vk::Pipeline,
    blur_v: vk::Pipeline,
    composite: vk::Pipeline,
    sampler_layout: vk::PipelineLayout,
    sampler_set_layout: vk::DescriptorSetLayout,
    unit_quad: vk::Buffer,
}

/// Where a frame's resolved pixels land.
enum ResolveTarget {
    Swapchain {
        view: vk::ImageView,
        image: vk::Image,
    },
    Offscreen {
        view: vk::ImageView,
        image: vk::Image,
    },
}

pub struct VulkanState {
    // Drop order: swapchain/surface/render targets first, then the context
    // (command pool -> device -> instance via its RAII guards).
    surface: Option<vk::SurfaceKHR>,
    swapchain_loader: Option<ash::khr::swapchain::Device>,
    swapchain: Option<vk::SwapchainKHR>,
    swapchain_images: Vec<vk::Image>,
    swapchain_views: Vec<ImageView>,
    swapchain_extent: vk::Extent2D,
    swapchain_format: vk::Format,
    command_buffer: Option<vk::CommandBuffer>,
    msaa: Option<MsaaTarget>,
    readback: Option<ReadbackTarget>,
    solid_pipeline: Option<SolidPipeline>,
    textured_pipeline: Option<TexturedPipeline>,
    glyph_pipeline: Option<GlyphPipeline>,
    gradient_pipeline: Option<GradientPipeline>,
    path_fill_pipeline: Option<PathFillPipeline>,
    blur_pipeline: Option<BlurPipeline>,
    stencil: Option<StencilTarget>,
    blur_scratch_a: Option<BlurScratch>,
    blur_scratch_b: Option<BlurScratch>,
    blur_sampler: Option<Sampler>,
    sampler_nearest: Option<Sampler>,
    sampler_bilinear: Option<Sampler>,
    descriptor_pools: Vec<DescriptorPool>,
    images: ImageStore,
    atlas: GlyphAtlas,
    ring: Option<InstanceRing>,
    uniform_ring: Option<UniformRing>,
    uniform_alignment: u64,
    path_ring: Option<PathRing>,
    encoder: FrameEncoder,
    requested_width: u32,
    requested_height: u32,
    pending_frame: Option<DecodedFrame>,
    last_frame: Option<DecodedFrame>,
    capabilities: VulkanCapabilities,
    context: AshContext,
}

impl Drop for VulkanState {
    fn drop(&mut self) {
        self.destroy_swapchain_resources();
        if let Some(surface) = self.surface.take() {
            if let Some(loader) = self.context.surface_loader() {
                unsafe {
                    loader.destroy_surface(surface, None);
                }
            }
        }
    }
}

unsafe impl Send for VulkanState {}
unsafe impl Sync for VulkanState {}

fn state_lock() -> std::sync::MutexGuard<'static, Option<VulkanState>> {
    static STATE: OnceLock<Mutex<Option<VulkanState>>> = OnceLock::new();
    STATE
        .get_or_init(|| Mutex::new(None))
        .lock()
        .expect("vulkan state mutex poisoned")
}

pub fn init(validation_enabled: bool) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    if guard.is_some() {
        clear_last_error();
        return Ok(());
    }

    let context = AshContext::init(validation_enabled)?;
    let state = VulkanState::new(context)?;
    *guard = Some(state);
    clear_last_error();
    Ok(())
}

pub fn shutdown() -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    *guard = None;
    clear_last_error();
    Ok(())
}

pub fn pipeline_features() -> Result<PhysicalDeviceFeatures, (RenderResult, String)> {
    let guard = state_lock();
    let state = guard.as_ref().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    Ok(*state.context.features())
}

pub fn query_capabilities(out: *mut VulkanCapabilities) -> Result<(), (RenderResult, String)> {
    if out.is_null() {
        return Err((
            RenderResult::InvalidHandle,
            "output pointer is null".to_string(),
        ));
    }
    let guard = state_lock();
    let state = guard.as_ref().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    unsafe {
        std::ptr::write(out, state.capabilities.clone());
    }
    clear_last_error();
    Ok(())
}

pub fn instance_handle() -> usize {
    let guard = state_lock();
    guard
        .as_ref()
        .map(|state| state.context.instance().handle().as_raw() as usize)
        .unwrap_or(0)
}

/// Whether the renderer's Vulkan state is initialized (and thus GPU image
/// storage is available). The image FFI routes to the GPU store when true and
/// to the `cpu-fallback` host store otherwise.
pub fn is_initialized() -> bool {
    state_lock().is_some()
}

#[cfg(not(target_os = "android"))]
pub fn create_xcb_surface(
    instance: usize,
    connection: usize,
    window: u32,
    width: u32,
    height: u32,
) -> Result<usize, (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    if state.context.instance().handle().as_raw() as usize != instance {
        return Err((
            RenderResult::InvalidHandle,
            "instance handle does not match the active renderer".to_string(),
        ));
    }
    if let Some(surface) = state.surface {
        return Ok(surface.as_raw() as usize);
    }
    let surface_handle = surface::create_xcb_surface(
        state.context.entry(),
        state.context.instance(),
        state.context.surface_loader().unwrap(),
        state.context.physical_device(),
        state.context.queue_family(),
        connection,
        window,
    )?;
    state.surface = Some(surface_handle);
    state.requested_width = width;
    state.requested_height = height;
    state.recreate_swapchain()?;
    Ok(surface_handle.as_raw() as usize)
}

#[cfg(target_os = "android")]
pub fn create_android_surface(
    instance: usize,
    android_window: *mut c_void,
    width: u32,
    height: u32,
) -> Result<usize, (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    if state.context.instance().handle().as_raw() as usize != instance {
        return Err((
            RenderResult::InvalidHandle,
            "instance handle does not match the active renderer".to_string(),
        ));
    }
    if let Some(surface) = state.surface {
        return Ok(surface.as_raw() as usize);
    }
    let surface_handle = surface::create_android_surface(
        state.context.entry(),
        state.context.instance(),
        state.context.surface_loader().unwrap(),
        state.context.physical_device(),
        state.context.queue_family(),
        android_window,
    )?;
    state.surface = Some(surface_handle);
    state.requested_width = width;
    state.requested_height = height;
    state.recreate_swapchain()?;
    Ok(surface_handle.as_raw() as usize)
}

#[cfg(target_os = "android")]
pub fn recreate_surface_android(
    android_window: *mut c_void,
    width: u32,
    height: u32,
) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    if let Some(old) = state.surface.take() {
        if let Some(loader) = state.context.surface_loader() {
            unsafe {
                loader.destroy_surface(old, None);
            }
        }
    }
    state.destroy_swapchain_resources();
    let surface_handle = surface::create_android_surface(
        state.context.entry(),
        state.context.instance(),
        state.context.surface_loader().unwrap(),
        state.context.physical_device(),
        state.context.queue_family(),
        android_window,
    )?;
    state.surface = Some(surface_handle);
    state.requested_width = width;
    state.requested_height = height;
    state.recreate_swapchain()?;
    Ok(())
}

pub fn resize(width: i32, height: i32) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    let width = width.max(1) as u32;
    let height = height.max(1) as u32;
    state.requested_width = width;
    state.requested_height = height;
    if state.surface.is_some() {
        state.recreate_swapchain()?;
    }
    Ok(())
}

/// Creates a real GPU texture from the given pixel rows (Slice 4: the backing
/// is now a `VkImage` + `VkImageView` + device-local memory, not host bytes).
/// Returns the `u64` handle the Go side keeps.
pub fn create_image(
    pixels: &[u8],
    width: u32,
    height: u32,
    stride: u32,
    format: ImageFormat,
) -> Result<u64, (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    state
        .images
        .create(&state.context, pixels, width, height, stride, format)
}

pub fn destroy_image(handle: u64) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    state.images.destroy(handle)
}

#[cfg(any(feature = "test-exports", test))]
pub fn image_stats() -> (usize, usize) {
    let guard = state_lock();
    guard
        .as_ref()
        .map(|state| state.images.stats())
        .unwrap_or((0, 0))
}

#[cfg(any(feature = "test-exports", test))]
pub fn reset_images() {
    let mut guard = state_lock();
    if let Some(state) = guard.as_mut() {
        state.images.reset();
    }
}

/// Uploads a glyph's coverage mask into the packed GPU atlas (Slice 5). The
/// atlas grows and compacts as needed; the SDF is generated for sizes >= 24 px.
#[allow(clippy::too_many_arguments)] // glyph upload carries the full glyph payload
pub fn upload_glyph(
    font_id: u64,
    glyph_id: u32,
    size_bits: u32,
    mask: &[u8],
    width: u32,
    height: u32,
    offset_x: f32,
    offset_y: f32,
    advance: f32,
) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    state
        .atlas
        .upload(&state.context, font_id, glyph_id, size_bits, mask, width, height, offset_x, offset_y, advance)
}

pub fn reset_atlas() {
    let mut guard = state_lock();
    if let Some(state) = guard.as_mut() {
        state.atlas.reset();
    }
}

#[cfg(any(feature = "test-exports", test))]
pub fn glyph_stats() -> (usize, usize) {
    let guard = state_lock();
    guard
        .as_ref()
        .map(|state| state.atlas.stats())
        .unwrap_or((0, 0))
}

pub fn submit_frame(data: *const u8, len: usize) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    if len == 0 {
        state.pending_frame = None;
        return Ok(());
    }
    if data.is_null() {
        return Err((
            RenderResult::InvalidHandle,
            "frame packet pointer is null".to_string(),
        ));
    }
    let bytes = unsafe { std::slice::from_raw_parts(data, len) };
    let frame = decode_frame(bytes)?;
    state.pending_frame = Some(frame);
    // Without a surface there is nothing to present; the frame is retained so
    // frame_stats() reflects the last submitted packet.
    if state.surface.is_some() {
        state.present()?;
    }
    Ok(())
}

/// Renders the given frame offscreen and returns its BGRA pixels (the GPU
/// readback path used by the equivalence harness).
pub fn readback_frame(
    data: *const u8,
    len: usize,
    width: u32,
    height: u32,
) -> Result<Vec<u8>, (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard.as_mut().ok_or((
        RenderResult::InitFailed,
        "renderer is not initialized".to_string(),
    ))?;
    if len == 0 || data.is_null() {
        return Err((
            RenderResult::InvalidHandle,
            "readback requires a non-empty frame packet".to_string(),
        ));
    }
    if width == 0 || height == 0 {
        return Err((
            RenderResult::InitFailed,
            "readback dimensions are zero".to_string(),
        ));
    }
    let bytes = unsafe { std::slice::from_raw_parts(data, len) };
    let frame = decode_frame(bytes)?;
    let pixels = state.readback(&frame, width, height)?;
    Ok(pixels)
}

#[cfg(feature = "test-exports")]
pub fn frame_stats() -> FrameStats {
    let guard = state_lock();
    let mut stats = guard
        .as_ref()
        .and_then(|state| {
            state
                .pending_frame
                .as_ref()
                .or(state.last_frame.as_ref())
                .map(|frame| frame.stats)
        })
        .unwrap_or_default();
    stats.vertex_count = 0;
    stats
}

impl VulkanState {
    fn new(context: AshContext) -> Result<Self, (RenderResult, String)> {
        let queue_family = context.queue_family();
        let mut caps = VulkanCapabilities::empty();
        let props = unsafe {
            context
                .instance()
                .get_physical_device_properties(context.physical_device())
        };
        caps.device_type = props.device_type.as_raw();
        caps.api_version = props.api_version;
        caps.driver_version = props.driver_version;
        caps.max_texture_dimension_2d = props.limits.max_image_dimension2_d;
        caps.graphics_queue_family_index = queue_family;
        caps.present_queue_family_index = queue_family;
        caps.transfer_queue_family_index = queue_family;
        let raw = unsafe { std::ffi::CStr::from_ptr(props.device_name.as_ptr()) };
        let bytes = raw.to_bytes();
        let len = bytes.len().min(caps.device_name.len().saturating_sub(1));
        for (dst, src) in caps
            .device_name
            .iter_mut()
            .take(len)
            .zip(bytes.iter().take(len))
        {
            *dst = *src as std::ffi::c_char;
        }

        let ring = InstanceRing::new(context.allocator(), context.device(), 2, DEFAULT_SLOT_BYTES)?;
        let uniform_ring = UniformRing::new(
            context.allocator(),
            2,
            DEFAULT_UNIFORM_SLOT_BYTES,
        )?;
        let path_ring = PathRing::new(context.allocator(), 2, DEFAULT_PATH_SLOT_BYTES)?;
        let uniform_alignment = unsafe {
            context
                .instance()
                .get_physical_device_properties(context.physical_device())
                .limits
                .min_uniform_buffer_offset_alignment
        }
        .max(16);
        let command_buffer = {
            let alloc_info = vk::CommandBufferAllocateInfo {
                command_pool: context.command_pool(),
                level: vk::CommandBufferLevel::PRIMARY,
                command_buffer_count: 1,
                ..Default::default()
            };
            let buffers = unsafe { context.device().allocate_command_buffers(&alloc_info) }
                .map_err(|e| vk_error("vkAllocateCommandBuffers", e.as_raw()))?;
            buffers[0]
        };

        // Shared samplers for textured draws (Slice 4). Sampling mode is per
        // draw, so the samplers are context-wide; the frame encoder selects one
        // when building each group's descriptor set.
        let sampler_nearest = Sampler::new(
            context.device(),
            &vk::SamplerCreateInfo {
                mag_filter: vk::Filter::NEAREST,
                min_filter: vk::Filter::NEAREST,
                mipmap_mode: vk::SamplerMipmapMode::NEAREST,
                address_mode_u: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_v: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_w: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                ..Default::default()
            },
        )?;
        let sampler_bilinear = Sampler::new(
            context.device(),
            &vk::SamplerCreateInfo {
                mag_filter: vk::Filter::LINEAR,
                min_filter: vk::Filter::LINEAR,
                mipmap_mode: vk::SamplerMipmapMode::NEAREST,
                address_mode_u: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_v: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_w: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                min_lod: 0.0,
                max_lod: 0.0,
                ..Default::default()
            },
        )?;

        // The blur scratch sampler: NEAREST + CLAMP_TO_EDGE so the blur and
        // composite texel reads land exactly on the oracle's array indices
        // (Slice 9), with the surface edge handled gracefully.
        let blur_sampler = Sampler::new(
            context.device(),
            &vk::SamplerCreateInfo {
                mag_filter: vk::Filter::NEAREST,
                min_filter: vk::Filter::NEAREST,
                mipmap_mode: vk::SamplerMipmapMode::NEAREST,
                address_mode_u: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_v: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_w: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                min_lod: 0.0,
                max_lod: 0.0,
                ..Default::default()
            },
        )?;

        // One descriptor pool per ring slot, so a slot's pool is only reset
        // after its fence signals (no in-flight sets are ever invalidated).
        // Pool sizes cover the sampler (textured/glyph) and uniform-buffer
        // (gradient) descriptor types.
        let frames = 2;
        let mut descriptor_pools = Vec::with_capacity(frames);
        let pool_sizes = [
            vk::DescriptorPoolSize {
                ty: vk::DescriptorType::COMBINED_IMAGE_SAMPLER,
                descriptor_count: TEXTURED_DESCRIPTOR_SETS_PER_POOL,
            },
            vk::DescriptorPoolSize {
                ty: vk::DescriptorType::UNIFORM_BUFFER,
                descriptor_count: TEXTURED_DESCRIPTOR_SETS_PER_POOL,
            },
            vk::DescriptorPoolSize {
                ty: vk::DescriptorType::STORAGE_BUFFER,
                descriptor_count: TEXTURED_DESCRIPTOR_SETS_PER_POOL,
            },
        ];
        for _ in 0..frames {
            descriptor_pools.push(DescriptorPool::new(
                context.device(),
                &pool_sizes,
                TEXTURED_DESCRIPTOR_SETS_PER_POOL,
            )?);
        }

        Ok(Self {
            surface: None,
            swapchain_loader: None,
            swapchain: None,
            swapchain_images: Vec::new(),
            swapchain_views: Vec::new(),
            swapchain_extent: vk::Extent2D {
                width: 1,
                height: 1,
            },
            swapchain_format: vk::Format::UNDEFINED,
            command_buffer: Some(command_buffer),
            msaa: None,
            readback: None,
            solid_pipeline: None,
            textured_pipeline: None,
            glyph_pipeline: None,
            gradient_pipeline: None,
            path_fill_pipeline: None,
            blur_pipeline: None,
            stencil: None,
            blur_scratch_a: None,
            blur_scratch_b: None,
            blur_sampler: Some(blur_sampler),
            sampler_nearest: Some(sampler_nearest),
            sampler_bilinear: Some(sampler_bilinear),
            descriptor_pools,
            images: ImageStore::new(),
            atlas: GlyphAtlas::new(),
            ring: Some(ring),
            uniform_ring: Some(uniform_ring),
            uniform_alignment,
            path_ring: Some(path_ring),
            encoder: FrameEncoder::default(),
            requested_width: 1,
            requested_height: 1,
            pending_frame: None,
            last_frame: None,
            capabilities: caps,
            context,
        })
    }

    fn destroy_swapchain_resources(&mut self) {
        if let (Some(loader), Some(swapchain)) =
            (self.swapchain_loader.take(), self.swapchain.take())
        {
            unsafe {
                loader.destroy_swapchain(swapchain, None);
            }
        }
        self.swapchain_images.clear();
        self.swapchain_views.clear();
        self.swapchain_format = vk::Format::UNDEFINED;
    }

    fn msaa_samples(&self) -> vk::SampleCountFlags {
        // Q8 amended: the solid pipeline renders single-sampled and computes
        // analytic coverage AA in the fragment shader instead of MSAA. The
        // reference driver's 4x/8x MSAA resolve averages to half intensity and
        // 2x depends on the driver's resolve being correct; the shader coverage
        // model is deterministic and matches the software oracle's analytic AA.
        // (See devdocs/notes/vulkan-equivalence-baseline.md, Q8 amendment.)
        vk::SampleCountFlags::TYPE_1
    }

    #[cfg_attr(not(feature = "test-exports"), allow(unused_variables))]
    fn ensure_solid_pipeline(
        &mut self,
        format: vk::Format,
        swapped: bool,
    ) -> Result<SolidHandles, (RenderResult, String)> {
        #[cfg(feature = "test-exports")]
        let needs_rebuild = !self
            .solid_pipeline
            .as_ref()
            .is_some_and(|p| p.format() == format && p.swapped() == swapped);
        #[cfg(not(feature = "test-exports"))]
        let needs_rebuild = !self
            .solid_pipeline
            .as_ref()
            .is_some_and(|p| p.format() == format);
        if needs_rebuild {
            #[cfg(feature = "test-exports")]
            let pipeline = if swapped {
                SolidPipeline::new_swapped(&self.context, format, self.msaa_samples())?
            } else {
                SolidPipeline::new(&self.context, format, self.msaa_samples())?
            };
            #[cfg(not(feature = "test-exports"))]
            let pipeline = SolidPipeline::new(&self.context, format, self.msaa_samples())?;
            self.solid_pipeline = Some(pipeline);
        }

        let p = self.solid_pipeline.as_ref().unwrap();
        Ok(SolidHandles {
            handle: p.handle(),
            layout: p.layout(),
            unit_quad: p.unit_quad_buffer(),
            samples: p.samples(),
        })
    }

    /// Builds (or reuses) the textured pipeline and returns its record handles.
    fn ensure_textured_pipeline(
        &mut self,
        format: vk::Format,
    ) -> Result<TexturedHandles, (RenderResult, String)> {
        if self
            .textured_pipeline
            .as_ref()
            .is_none_or(|p| p.format() != format)
        {
            self.textured_pipeline = Some(TexturedPipeline::new(
                &self.context,
                format,
                self.msaa_samples(),
            )?);
        }
        let p = self.textured_pipeline.as_ref().unwrap();
        Ok(TexturedHandles {
            handle: p.handle(),
            layout: p.layout(),
            set_layout: p.set_layout(),
            unit_quad: p.unit_quad_buffer(),
        })
    }

    fn ensure_glyph_pipeline(&mut self, format: vk::Format) -> Result<GlyphHandles, (RenderResult, String)> {
        if self
            .glyph_pipeline
            .as_ref()
            .is_none_or(|p| p.format() != format)
        {
            self.glyph_pipeline = Some(GlyphPipeline::new(
                &self.context,
                format,
                self.msaa_samples(),
            )?);
        }
        let p = self.glyph_pipeline.as_ref().unwrap();
        Ok(GlyphHandles {
            bitmap: p.bitmap_handle(),
            sdf: p.sdf_handle(),
            layout: p.layout(),
            unit_quad: p.unit_quad_buffer(),
        })
    }

    fn ensure_gradient_pipeline(&mut self, format: vk::Format) -> Result<GradientHandles, (RenderResult, String)> {
        if self
            .gradient_pipeline
            .as_ref()
            .is_none_or(|p| p.format() != format)
        {
            self.gradient_pipeline = Some(GradientPipeline::new(
                &self.context,
                format,
                self.msaa_samples(),
            )?);
        }
        let p = self.gradient_pipeline.as_ref().unwrap();
        Ok(GradientHandles {
            handle: p.handle(),
            layout: p.layout(),
            set_layout: p.set_layout(),
            unit_quad: p.unit_quad_buffer(),
        })
    }

    fn ensure_path_fill_pipeline(
        &mut self,
        format: vk::Format,
    ) -> Result<PathFillHandles, (RenderResult, String)> {
        if self
            .path_fill_pipeline
            .as_ref()
            .is_none_or(|p| p.format() != format)
        {
            self.path_fill_pipeline = Some(PathFillPipeline::new(
                &self.context,
                format,
                self.msaa_samples(),
            )?);
        }
        let p = self.path_fill_pipeline.as_ref().unwrap();
        Ok(PathFillHandles {
            stencil: p.stencil_handle(),
            stencil_layout: p.stencil_layout(),
            cover_solid: p.cover_solid_handle(),
            cover_gradient: p.cover_gradient_handle(),
            cover_layout: p.cover_layout(),
            cover_gradient_layout: p.cover_gradient_layout(),
            segments_layout: p.segments_set_layout(),
            unit_quad: p.unit_quad_buffer(),
        })
    }

    /// Builds (or reuses) the shadow-blur pipelines (Slice 9).
    fn ensure_blur_pipeline(&mut self, format: vk::Format) -> Result<BlurHandles, (RenderResult, String)> {
        if self
            .blur_pipeline
            .as_ref()
            .is_none_or(|p| p.format() != format)
        {
            self.blur_pipeline = Some(BlurPipeline::new(
                &self.context,
                format,
                self.msaa_samples(),
            )?);
        }
        let p = self.blur_pipeline.as_ref().unwrap();
        Ok(BlurHandles {
            mask: p.mask_handle(),
            mask_layout: p.mask_layout(),
            blur_h: p.blur_h_handle(),
            blur_v: p.blur_v_handle(),
            composite: p.composite_handle(),
            sampler_layout: p.sampler_layout(),
            sampler_set_layout: p.sampler_set_layout(),
            unit_quad: p.unit_quad_buffer(),
        })
    }

    fn create_blur_scratch(
        &self,
        extent: vk::Extent2D,
    ) -> Result<BlurScratch, (RenderResult, String)> {
        let device = self.context.device();
        let allocator = self.context.allocator();
        let image_info = vk::ImageCreateInfo {
            image_type: vk::ImageType::TYPE_2D,
            format: BLUR_SCRATCH_FORMAT,
            extent: vk::Extent3D {
                width: extent.width,
                height: extent.height,
                depth: 1,
            },
            mip_levels: 1,
            array_layers: 1,
            samples: vk::SampleCountFlags::TYPE_1,
            tiling: vk::ImageTiling::OPTIMAL,
            usage: vk::ImageUsageFlags::COLOR_ATTACHMENT | vk::ImageUsageFlags::SAMPLED,
            sharing_mode: vk::SharingMode::EXCLUSIVE,
            ..Default::default()
        };
        let image = Image::new(device, &image_info)?;
        let requirements = unsafe { device.get_image_memory_requirements(image.handle()) };
        let allocation = allocator.allocate_image_memory(
            image.handle(),
            requirements,
            MemoryLocation::GpuOnly,
        )?;
        unsafe { device.bind_image_memory(image.handle(), allocation.memory(), 0) }
            .map_err(|e| vk_error("vkBindImageMemory", e.as_raw()))?;
        let view = ImageView::new(
            device,
            image.handle(),
            BLUR_SCRATCH_FORMAT,
            vk::ImageAspectFlags::COLOR,
        )?;
        Ok(BlurScratch {
            view,
            image,
            allocation,
            extent,
        })
    }

    /// The two R8 blur scratch images, recreated when the extent changes.
    fn ensure_blur_scratch(&mut self, extent: vk::Extent2D) -> Result<(), (RenderResult, String)> {
        if self
            .blur_scratch_a
            .as_ref()
            .is_some_and(|s| s.extent == extent)
        {
            return Ok(());
        }
        self.blur_scratch_a = Some(self.create_blur_scratch(extent)?);
        self.blur_scratch_b = Some(self.create_blur_scratch(extent)?);
        Ok(())
    }

    /// The stencil attachment (D24S8), recreated when the extent changes.
    fn ensure_stencil(&mut self, extent: vk::Extent2D) -> Result<(), (RenderResult, String)> {
        if self
            .stencil
            .as_ref()
            .is_some_and(|s| s.extent == extent)
        {
            return Ok(());
        }
        let device = self.context.device();
        let allocator = self.context.allocator();
        let image_info = vk::ImageCreateInfo {
            image_type: vk::ImageType::TYPE_2D,
            format: STENCIL_FORMAT,
            extent: vk::Extent3D {
                width: extent.width,
                height: extent.height,
                depth: 1,
            },
            mip_levels: 1,
            array_layers: 1,
            samples: vk::SampleCountFlags::TYPE_1,
            tiling: vk::ImageTiling::OPTIMAL,
            usage: vk::ImageUsageFlags::DEPTH_STENCIL_ATTACHMENT,
            sharing_mode: vk::SharingMode::EXCLUSIVE,
            ..Default::default()
        };
        let image = Image::new(device, &image_info)?;
        let requirements = unsafe { device.get_image_memory_requirements(image.handle()) };
        let allocation = allocator.allocate_image_memory(
            image.handle(),
            requirements,
            MemoryLocation::GpuOnly,
        )?;
        unsafe {
            device.bind_image_memory(image.handle(), allocation.memory(), 0)
        }
        .map_err(|e| vk_error("vkBindImageMemory", e.as_raw()))?;
        let view = ImageView::new(
            device,
            image.handle(),
            STENCIL_FORMAT,
            vk::ImageAspectFlags::DEPTH | vk::ImageAspectFlags::STENCIL,
        )?;
        self.stencil = Some(StencilTarget {
            view,
            image,
            allocation,
            extent,
        });
        Ok(())
    }

    fn ensure_msaa(
        &mut self,
        extent: vk::Extent2D,
        format: vk::Format,
    ) -> Result<(), (RenderResult, String)> {
        let samples = self.msaa_samples();
        if self
            .msaa
            .as_ref()
            .is_some_and(|m| m.extent == extent && m.format == format && m.samples == samples)
        {
            return Ok(());
        }
        let device = self.context.device();
        let allocator = self.context.allocator();

        let image_info = vk::ImageCreateInfo {
            image_type: vk::ImageType::TYPE_2D,
            format,
            extent: vk::Extent3D {
                width: extent.width,
                height: extent.height,
                depth: 1,
            },
            mip_levels: 1,
            array_layers: 1,
            samples,
            tiling: vk::ImageTiling::OPTIMAL,
            usage: vk::ImageUsageFlags::COLOR_ATTACHMENT,
            sharing_mode: vk::SharingMode::EXCLUSIVE,
            ..Default::default()
        };
        let image = Image::new(device, &image_info)?;
        let requirements = unsafe { device.get_image_memory_requirements(image.handle()) };
        let allocation = allocator.allocate_image_memory(
            image.handle(),
            requirements,
            MemoryLocation::GpuOnly,
        )?;
        unsafe { device.bind_image_memory(image.handle(), allocation.memory(), 0) }
            .map_err(|e| vk_error("vkBindImageMemory", e.as_raw()))?;
        let view = ImageView::new(device, image.handle(), format, vk::ImageAspectFlags::COLOR)?;
        self.msaa = Some(MsaaTarget {
            view,
            image,
            allocation,
            extent,
            format,
            samples,
        });
        Ok(())
    }

    fn ensure_readback(
        &mut self,
        extent: vk::Extent2D,
        format: vk::Format,
    ) -> Result<(), (RenderResult, String)> {
        if self
            .readback
            .as_ref()
            .is_some_and(|r| r.extent == extent && r.format == format)
        {
            return Ok(());
        }
        let device = self.context.device();
        let allocator = self.context.allocator();
        let image_info = vk::ImageCreateInfo {
            image_type: vk::ImageType::TYPE_2D,
            format,
            extent: vk::Extent3D {
                width: extent.width,
                height: extent.height,
                depth: 1,
            },
            mip_levels: 1,
            array_layers: 1,
            samples: vk::SampleCountFlags::TYPE_1,
            tiling: vk::ImageTiling::OPTIMAL,
            usage: vk::ImageUsageFlags::COLOR_ATTACHMENT | vk::ImageUsageFlags::TRANSFER_SRC,
            sharing_mode: vk::SharingMode::EXCLUSIVE,
            ..Default::default()
        };
        let image = Image::new(device, &image_info)?;
        let requirements = unsafe { device.get_image_memory_requirements(image.handle()) };
        let allocation = allocator.allocate_image_memory(
            image.handle(),
            requirements,
            MemoryLocation::GpuOnly,
        )?;
        unsafe { device.bind_image_memory(image.handle(), allocation.memory(), 0) }
            .map_err(|e| vk_error("vkBindImageMemory", e.as_raw()))?;
        let view = ImageView::new(device, image.handle(), format, vk::ImageAspectFlags::COLOR)?;
        // GpuToCpu (host-visible, cached): reading back 1080p via write-combined
        // CpuToGpu memory was measured at ~240ms for the 8MB copy.
        let staging = allocator.create_buffer(
            (extent.width as u64) * (extent.height as u64) * 4,
            vk::BufferUsageFlags::TRANSFER_DST,
            MemoryLocation::GpuToCpu,
        )?;
        self.readback = Some(ReadbackTarget {
            view,
            image,
            allocation,
            staging,
            extent,
            format,
        });
        Ok(())
    }

    fn begin_command_buffer(&mut self) -> Result<vk::CommandBuffer, (RenderResult, String)> {
        let Some(command_buffer) = self.command_buffer else {
            return Err((
                RenderResult::InitFailed,
                "command buffer is not allocated".to_string(),
            ));
        };
        let device = self.context.device();
        unsafe {
            device.reset_command_pool(
                self.context.command_pool(),
                vk::CommandPoolResetFlags::empty(),
            )
        }
        .map_err(|e| vk_error("vkResetCommandPool", e.as_raw()))?;
        let begin_info = vk::CommandBufferBeginInfo {
            flags: vk::CommandBufferUsageFlags::ONE_TIME_SUBMIT,
            ..Default::default()
        };
        unsafe { device.begin_command_buffer(command_buffer, &begin_info) }
            .map_err(|e| vk_error("vkBeginCommandBuffer", e.as_raw()))?;
        Ok(command_buffer)
    }

    /// The shared GPU render: encodes the frame, records the instanced solid
    /// draws into `resolve` (MSAA -> resolve), and submits. `copy_out` carries
    /// the frame's instance data out of the ring; the caller owns synchronization
    /// (present vs wait-idle).
    #[allow(clippy::too_many_arguments)]
    fn render_into(
        &mut self,
        frame: &DecodedFrame,
        extent: vk::Extent2D,
        format: vk::Format,
        resolve: ResolveTarget,
        clear: vk::ClearValue,
    ) -> Result<(), (RenderResult, String)> {
        self.ensure_msaa(extent, format)?;
        let pipeline = self.ensure_solid_pipeline(format, force_swapped())?;
        let textured = self.ensure_textured_pipeline(format)?;
        let glyph = self.ensure_glyph_pipeline(format)?;
        let gradient = self.ensure_gradient_pipeline(format)?;
        let path_fill = self.ensure_path_fill_pipeline(format)?;
        let blur = self.ensure_blur_pipeline(format)?;
        self.ensure_stencil(extent)?;
        self.ensure_blur_scratch(extent)?;

        // Encode the frame into the per-frame instance ring. Each ring slot has
        // its own descriptor pool, reset here after the slot's fence signals so
        // no in-flight descriptor set is ever invalidated. The uniform and path
        // rings share the instance ring's slot synchronization (the same fence
        // protects all buffers), so they advance without a separate wait.
        let surface_size = [extent.width as f32, extent.height as f32];
        let encoded = {
            let ring = self.ring.as_mut().expect("ring initialized");
            ring.begin_frame()?;
            self.uniform_ring.as_mut().unwrap().begin_frame();
            self.path_ring.as_mut().unwrap().begin_frame();
            let slot = ring.current_slot();
            self.descriptor_pools[slot].reset()?;
            let textures = Textures {
                images: &self.images,
                atlas: &self.atlas,
                descriptor_pool: self.descriptor_pools[slot].handle(),
                descriptor_layout: textured.set_layout,
                gradient_layout: gradient.set_layout,
                segments_layout: path_fill.segments_layout,
                sampler_nearest: self.sampler_nearest.as_ref().unwrap().handle(),
                sampler_bilinear: self.sampler_bilinear.as_ref().unwrap().handle(),
                uniform_alignment: self.uniform_alignment,
                device: self.context.device(),
            };
            self.encoder.encode(
                frame,
                ring,
                self.uniform_ring.as_mut().unwrap(),
                self.path_ring.as_mut().unwrap(),
                self.context.allocator(),
                surface_size,
                &textures,
            )?
        };
        let instance_buffer = self.ring.as_ref().unwrap().current_buffer();
        let path_buffer = self.path_ring.as_ref().unwrap().current_buffer();

        let command_buffer = self.begin_command_buffer()?;
        let device = self.context.device().clone();

        // Transition the resolve image to COLOR_ATTACHMENT_OPTIMAL.
        let resolve_image = match resolve {
            ResolveTarget::Swapchain { image, .. } | ResolveTarget::Offscreen { image, .. } => {
                image
            }
        };
        let barrier_to_attachment = vk::ImageMemoryBarrier::default()
            .old_layout(vk::ImageLayout::UNDEFINED)
            .new_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
            .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .image(resolve_image)
            .subresource_range(
                vk::ImageSubresourceRange::default()
                    .aspect_mask(vk::ImageAspectFlags::COLOR)
                    .level_count(1)
                    .layer_count(1),
            );
        unsafe {
            device.cmd_pipeline_barrier(
                command_buffer,
                vk::PipelineStageFlags::TOP_OF_PIPE,
                vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                vk::DependencyFlags::empty(),
                &[],
                &[],
                &[barrier_to_attachment],
            );
        }

        // Dynamic rendering: MSAA attachment resolving to the target.
        let (resolve_view, resolve_layout) = match &resolve {
            ResolveTarget::Swapchain { view, .. } => {
                (*view, vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
            }
            ResolveTarget::Offscreen { view, .. } => {
                (*view, vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
            }
        };
        // Transition the stencil attachment (cleared to 0 at frame start so the
        // per-path clears reset it between fills).
        let stencil_image = self.stencil.as_ref().unwrap().image.handle();
        let stencil_barrier = vk::ImageMemoryBarrier::default()
            .old_layout(vk::ImageLayout::UNDEFINED)
            .new_layout(vk::ImageLayout::DEPTH_STENCIL_ATTACHMENT_OPTIMAL)
            .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .image(stencil_image)
            .subresource_range(
                vk::ImageSubresourceRange::default()
                    .aspect_mask(
                        vk::ImageAspectFlags::DEPTH | vk::ImageAspectFlags::STENCIL,
                    )
                    .level_count(1)
                    .layer_count(1),
            );
        unsafe {
            device.cmd_pipeline_barrier(
                command_buffer,
                vk::PipelineStageFlags::TOP_OF_PIPE,
                vk::PipelineStageFlags::EARLY_FRAGMENT_TESTS,
                vk::DependencyFlags::empty(),
                &[],
                &[],
                &[stencil_barrier],
            );
        }
        let msaa = pipeline.samples != vk::SampleCountFlags::TYPE_1;

        // Slice 9: when the frame carries shadows, render them (clear the main
        // target, then per-shadow mask + separable blur into the R8 scratch +
        // composite into the target) BEFORE the geometry pass, so shadows sit
        // behind the content. The geometry pass then loads the target instead of
        // clearing it. Without shadows the geometry pass clears as before.
        if !encoded.shadows.is_empty() {
            self.render_shadows(
                command_buffer,
                &blur,
                &encoded.shadows,
                instance_buffer,
                extent,
                resolve_view,
                resolve_layout,
                clear,
            )?;
        }

        let color_attachment = if msaa {
            vk::RenderingAttachmentInfo::default()
                .image_view(self.msaa.as_ref().unwrap().view.handle())
                .image_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
                .resolve_mode(vk::ResolveModeFlags::AVERAGE)
                .resolve_image_view(resolve_view)
                .resolve_image_layout(resolve_layout)
                .load_op(if encoded.shadows.is_empty() {
                    vk::AttachmentLoadOp::CLEAR
                } else {
                    vk::AttachmentLoadOp::LOAD
                })
                .store_op(vk::AttachmentStoreOp::STORE)
                .clear_value(clear)
        } else {
            // No MSAA: render directly into the resolve image.
            vk::RenderingAttachmentInfo::default()
                .image_view(resolve_view)
                .image_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
                .load_op(if encoded.shadows.is_empty() {
                    vk::AttachmentLoadOp::CLEAR
                } else {
                    vk::AttachmentLoadOp::LOAD
                })
                .store_op(vk::AttachmentStoreOp::STORE)
                .clear_value(clear)
        };
        let color_attachments = [color_attachment];
        let stencil_attachment = vk::RenderingAttachmentInfo::default()
            .image_view(self.stencil.as_ref().unwrap().view.handle())
            .image_layout(vk::ImageLayout::DEPTH_STENCIL_ATTACHMENT_OPTIMAL)
            .load_op(vk::AttachmentLoadOp::CLEAR)
            .store_op(vk::AttachmentStoreOp::STORE)
            .clear_value(vk::ClearValue {
                depth_stencil: vk::ClearDepthStencilValue {
                    depth: 0.0,
                    stencil: 0,
                },
            });
        let rendering_info = vk::RenderingInfo::default()
            .render_area(vk::Rect2D {
                offset: vk::Offset2D { x: 0, y: 0 },
                extent,
            })
            .layer_count(1)
            .color_attachments(&color_attachments)
            .stencil_attachment(&stencil_attachment);
        unsafe {
            device.cmd_begin_rendering(command_buffer, &rendering_info);
            device.cmd_set_viewport(
                command_buffer,
                0,
                &[vk::Viewport {
                    x: 0.0,
                    y: 0.0,
                    width: extent.width as f32,
                    height: extent.height as f32,
                    min_depth: 0.0,
                    max_depth: 1.0,
                }],
            );
            for group in &encoded.groups {
                // Select the pipeline + unit quad for the group's kind. Solid,
                // textured, and glyph pipelines share the unit-quad (binding 0)
                // + instance (binding 1) layout, so the vertex buffers are
                // re-bound per group with the active pipeline's unit quad.
                let (pipe, pipe_layout, unit_quad) = match group.kind {
                    DrawKind::Solid => (pipeline.handle, pipeline.layout, pipeline.unit_quad),
                    DrawKind::Textured => {
                        (textured.handle, textured.layout, textured.unit_quad)
                    }
                    DrawKind::GlyphBitmap => (glyph.bitmap, glyph.layout, glyph.unit_quad),
                    DrawKind::GlyphSdf => (glyph.sdf, glyph.layout, glyph.unit_quad),
                    DrawKind::Gradient => {
                        (gradient.handle, gradient.layout, gradient.unit_quad)
                    }
                };
                device.cmd_bind_pipeline(command_buffer, vk::PipelineBindPoint::GRAPHICS, pipe);
                let vertex_buffers = [unit_quad, instance_buffer];
                let offsets = [0u64, 0u64];
                device.cmd_bind_vertex_buffers(command_buffer, 0, &vertex_buffers, &offsets);
                if let Some(set) = group.descriptor_set {
                    device.cmd_bind_descriptor_sets(
                        command_buffer,
                        vk::PipelineBindPoint::GRAPHICS,
                        pipe_layout,
                        0,
                        &[set],
                        &[],
                    );
                }
                // Per-draw scissor (Slice 3 forward): the clip is axis-aligned,
                // so the rasterizer can cull fragments outside it before the
                // fragment shader runs. This is the hardware-idiomatic clip; the
                // shader discard remains only for the exact float clip boundary.
                let scissor = clip_scissor(&group.push, extent);
                device.cmd_set_scissor(command_buffer, 0, &[scissor]);
                device.cmd_push_constants(
                    command_buffer,
                    pipe_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &group.push.bytes(),
                );
                device.cmd_draw(
                    command_buffer,
                    6,
                    group.instance_count,
                    0,
                    group.first_instance,
                );
            }

            // Stencil-buffer path fills (Slice 7): clear the path's stencil
            // region, accumulate the winding number over the flattened
            // triangles, then fill the path's bounding quad keeping only
            // nonzero-winding fragments.
            for fill in &encoded.path_fills {
                // Clear the stencil within the path's world bounds.
                let clear_rect = clear_rect_for(fill.clear_rect, extent);
                let clear_attachment = vk::ClearAttachment {
                    aspect_mask: vk::ImageAspectFlags::STENCIL,
                    color_attachment: 0,
                    clear_value: vk::ClearValue {
                        depth_stencil: vk::ClearDepthStencilValue {
                            depth: 0.0,
                            stencil: 0,
                        },
                    },
                };
                device.cmd_clear_attachments(
                    command_buffer,
                    &[clear_attachment],
                    &[vk::ClearRect {
                        rect: clear_rect,
                        base_array_layer: 0,
                        layer_count: 1,
                    }],
                );

                // Winding pass.
                device.cmd_bind_pipeline(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    path_fill.stencil,
                );
                device.cmd_set_scissor(command_buffer, 0, &[vk::Rect2D {
                    offset: vk::Offset2D { x: 0, y: 0 },
                    extent,
                }]);
                let stencil_push = PushConstants::default_for_stencil(fill.bottom_center_x, surface_size);
                device.cmd_push_constants(
                    command_buffer,
                    path_fill.stencil_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &stencil_push.bytes(),
                );
                device.cmd_bind_vertex_buffers(command_buffer, 0, &[path_buffer], &[0]);
                device.cmd_draw(
                    command_buffer,
                    fill.vertex_count,
                    1,
                    fill.first_vertex,
                    0,
                );
                // Cover pass: solid or gradient brush, gated by stencil != 0,
                // with the winding coverage computed from the contour edges.
                let cover_pipe = if fill.push.brush_kind == BRUSH_LINEAR_GRADIENT {
                    path_fill.cover_gradient
                } else {
                    path_fill.cover_solid
                };
                device.cmd_bind_pipeline(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    cover_pipe,
                );
                // Set 0: gradient UBO (gradient covers only). Set 1: contour
                // edges storage buffer (always). The gradient cover's layout
                // differs at set 0 (UBO vs empty).
                let cover_layout = if fill.push.brush_kind == BRUSH_LINEAR_GRADIENT {
                    path_fill.cover_gradient_layout
                } else {
                    path_fill.cover_layout
                };
                if let Some(set) = fill.gradient_descriptor {
                    device.cmd_bind_descriptor_sets(
                        command_buffer,
                        vk::PipelineBindPoint::GRAPHICS,
                        cover_layout,
                        0,
                        &[set],
                        &[],
                    );
                }
                device.cmd_bind_descriptor_sets(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    cover_layout,
                    1,
                    &[fill.segments_descriptor],
                    &[],
                );
                let cover_scissor = clip_scissor(&fill.push, extent);
                device.cmd_set_scissor(command_buffer, 0, &[cover_scissor]);
                device.cmd_push_constants(
                    command_buffer,
                    cover_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &fill.push.bytes(),
                );
                let cover_buffers = [path_fill.unit_quad, instance_buffer];
                device.cmd_bind_vertex_buffers(command_buffer, 0, &cover_buffers, &[0, 0]);
                device.cmd_draw(
                    command_buffer,
                    6,
                    1,
                    0,
                    fill.cover_first_instance,
                );
            }

            device.cmd_end_rendering(command_buffer);
        }

        // Post-render transition + optional copy-out (readback).
        match &resolve {
            ResolveTarget::Swapchain { image, .. } => {
                let barrier = vk::ImageMemoryBarrier::default()
                    .src_access_mask(vk::AccessFlags::COLOR_ATTACHMENT_WRITE)
                    .old_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
                    .new_layout(vk::ImageLayout::PRESENT_SRC_KHR)
                    .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
                    .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
                    .image(*image)
                    .subresource_range(
                        vk::ImageSubresourceRange::default()
                            .aspect_mask(vk::ImageAspectFlags::COLOR)
                            .level_count(1)
                            .layer_count(1),
                    );
                unsafe {
                    device.cmd_pipeline_barrier(
                        command_buffer,
                        vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                        vk::PipelineStageFlags::BOTTOM_OF_PIPE,
                        vk::DependencyFlags::empty(),
                        &[],
                        &[],
                        &[barrier],
                    );
                }
            }
            ResolveTarget::Offscreen { image, .. } => {
                let barrier = vk::ImageMemoryBarrier::default()
                    .src_access_mask(vk::AccessFlags::COLOR_ATTACHMENT_WRITE)
                    .dst_access_mask(vk::AccessFlags::TRANSFER_READ)
                    .old_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
                    .new_layout(vk::ImageLayout::TRANSFER_SRC_OPTIMAL)
                    .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
                    .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
                    .image(*image)
                    .subresource_range(
                        vk::ImageSubresourceRange::default()
                            .aspect_mask(vk::ImageAspectFlags::COLOR)
                            .level_count(1)
                            .layer_count(1),
                    );
                unsafe {
                    device.cmd_pipeline_barrier(
                        command_buffer,
                        vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                        vk::PipelineStageFlags::TRANSFER,
                        vk::DependencyFlags::empty(),
                        &[],
                        &[],
                        &[barrier],
                    );
                    let regions = [vk::BufferImageCopy::default()
                        .image_subresource(
                            vk::ImageSubresourceLayers::default()
                                .aspect_mask(vk::ImageAspectFlags::COLOR)
                                .layer_count(1),
                        )
                        .image_extent(vk::Extent3D {
                            width: extent.width,
                            height: extent.height,
                            depth: 1,
                        })];
                    let staging = self.readback.as_ref().unwrap().staging.buffer();
                    device.cmd_copy_image_to_buffer(
                        command_buffer,
                        *image,
                        vk::ImageLayout::TRANSFER_SRC_OPTIMAL,
                        staging,
                        &regions,
                    );
                }
            }
        }

        unsafe { device.end_command_buffer(command_buffer) }
            .map_err(|e| vk_error("vkEndCommandBuffer", e.as_raw()))?;

        let command_buffers = [command_buffer];
        let submit_info = vk::SubmitInfo::default().command_buffers(&command_buffers);
        let fence = self.ring.as_ref().unwrap().take_fence();
        unsafe { device.queue_submit(self.context.queue(), &[submit_info], fence) }
            .map_err(|e| vk_error("vkQueueSubmit", e.as_raw()))?;
        Ok(())
    }

    /// Records the Slice 9 shadow passes ahead of the geometry pass: clears the
    /// main target (via the first composite's CLEAR load op), then for each
    /// shadow renders its path coverage mask into the R8 scratch, applies the
    /// separable Gaussian (H then V), and composites the tinted blurred mask at
    /// the shadow's offset with premultiplied-over blending.
    #[allow(clippy::too_many_arguments)]
    fn render_shadows(
        &mut self,
        command_buffer: vk::CommandBuffer,
        blur: &BlurHandles,
        shadows: &[ShadowFill],
        instance_buffer: vk::Buffer,
        extent: vk::Extent2D,
        resolve_view: vk::ImageView,
        resolve_layout: vk::ImageLayout,
        clear: vk::ClearValue,
    ) -> Result<(), (RenderResult, String)> {
        let device = self.context.device();

        let scratch_a = self.blur_scratch_a.as_ref().unwrap();
        let scratch_b = self.blur_scratch_b.as_ref().unwrap();
        let sampler = self.blur_sampler.as_ref().unwrap().handle();
        // One sampler descriptor per scratch image; both are bound through the
        // shared blur/composite layout for the whole frame.
        let set_a = self.allocate_scratch_descriptor(
            scratch_a.view.handle(),
            sampler,
            blur.sampler_set_layout,
        )?;
        let set_b = self.allocate_scratch_descriptor(
            scratch_b.view.handle(),
            sampler,
            blur.sampler_set_layout,
        )?;

        let clear_zero = vk::ClearValue {
            color: vk::ClearColorValue { float32: [0.0; 4] },
        };

        // The scratch images start UNDEFINED (fresh allocations). Transition
        // both to COLOR_ATTACHMENT once; the per-shadow loop then shuttles each
        // between COLOR_ATTACHMENT (writes) and SHADER_READ_ONLY (reads).
        self.transition_image(
            command_buffer,
            scratch_a.image.handle(),
            vk::ImageLayout::UNDEFINED,
            vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
            vk::PipelineStageFlags::TOP_OF_PIPE,
            vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
            vk::AccessFlags::empty(),
            vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
        );
        self.transition_image(
            command_buffer,
            scratch_b.image.handle(),
            vk::ImageLayout::UNDEFINED,
            vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
            vk::PipelineStageFlags::TOP_OF_PIPE,
            vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
            vk::AccessFlags::empty(),
            vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
        );

        let mut first_shadow = true;
        let mut first_composite = true;
        for shadow in shadows {
            // 1. Mask pass: the path's winding coverage into scratch_a (the
            // scratch is fully cleared, so reads anywhere within the surface
            // are zero outside the path).
            if !first_shadow {
                self.transition_image(
                    command_buffer,
                    scratch_a.image.handle(),
                    vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL,
                    vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
                    vk::PipelineStageFlags::FRAGMENT_SHADER,
                    vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                    vk::AccessFlags::SHADER_READ,
                    vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
                );
            }
            let mask_attachment = self.scratch_attachment(scratch_a.view.handle(), clear_zero);
            self.begin_rendering(command_buffer, extent, &[mask_attachment], None);
            unsafe {
                self.set_full_viewport(command_buffer, extent);
                device.cmd_set_scissor(command_buffer, 0, &[vk::Rect2D {
                    offset: vk::Offset2D { x: 0, y: 0 },
                    extent,
                }]);
                device.cmd_bind_pipeline(command_buffer, vk::PipelineBindPoint::GRAPHICS, blur.mask);
                // The mask layout's set 1 binds the contour edges; set 0 is the
                // empty layout and stays unbound (the shader reads only set 1).
                device.cmd_bind_descriptor_sets(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    blur.mask_layout,
                    1,
                    &[shadow.segments_descriptor],
                    &[],
                );
                let vertex_buffers = [blur.unit_quad, instance_buffer];
                let offsets = [0u64, 0u64];
                device.cmd_bind_vertex_buffers(command_buffer, 0, &vertex_buffers, &offsets);
                device.cmd_push_constants(
                    command_buffer,
                    blur.mask_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &shadow.mask_push.bytes(),
                );
                device.cmd_draw(
                    command_buffer,
                    6,
                    1,
                    0,
                    shadow.mask_first_instance,
                );
                device.cmd_end_rendering(command_buffer);
            }
            self.transition_image(
                command_buffer,
                scratch_a.image.handle(),
                vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
                vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL,
                vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                vk::PipelineStageFlags::FRAGMENT_SHADER,
                vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
                vk::AccessFlags::SHADER_READ,
            );

            // 2. Horizontal blur: scratch_a -> scratch_b.
            if !first_shadow {
                self.transition_image(
                    command_buffer,
                    scratch_b.image.handle(),
                    vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL,
                    vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
                    vk::PipelineStageFlags::FRAGMENT_SHADER,
                    vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                    vk::AccessFlags::SHADER_READ,
                    vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
                );
            }
            let blur_h_attachment = self.scratch_attachment(scratch_b.view.handle(), clear_zero);
            self.begin_rendering(command_buffer, extent, &[blur_h_attachment], None);
            unsafe {
                self.set_full_viewport(command_buffer, extent);
                device.cmd_set_scissor(command_buffer, 0, &[vk::Rect2D {
                    offset: vk::Offset2D { x: 0, y: 0 },
                    extent,
                }]);
                device.cmd_bind_pipeline(command_buffer, vk::PipelineBindPoint::GRAPHICS, blur.blur_h);
                device.cmd_bind_descriptor_sets(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    blur.sampler_layout,
                    0,
                    &[set_a],
                    &[],
                );
                let vertex_buffers = [blur.unit_quad, instance_buffer];
                let offsets = [0u64, 0u64];
                device.cmd_bind_vertex_buffers(command_buffer, 0, &vertex_buffers, &offsets);
                device.cmd_push_constants(
                    command_buffer,
                    blur.sampler_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &shadow.blur_push.bytes(),
                );
                device.cmd_draw(
                    command_buffer,
                    6,
                    1,
                    0,
                    shadow.blur_h_first_instance,
                );
                device.cmd_end_rendering(command_buffer);
            }
            self.transition_image(
                command_buffer,
                scratch_b.image.handle(),
                vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
                vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL,
                vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                vk::PipelineStageFlags::FRAGMENT_SHADER,
                vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
                vk::AccessFlags::SHADER_READ,
            );

            // 3. Vertical blur: scratch_b -> scratch_a. scratch_a is in
            // SHADER_READ (the mask pass's post-write transition) for every
            // shadow, first included.
            self.transition_image(
                command_buffer,
                scratch_a.image.handle(),
                vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL,
                vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
                vk::PipelineStageFlags::FRAGMENT_SHADER,
                vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                vk::AccessFlags::SHADER_READ,
                vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
            );
            let blur_v_attachment = self.scratch_attachment(scratch_a.view.handle(), clear_zero);
            self.begin_rendering(command_buffer, extent, &[blur_v_attachment], None);
            unsafe {
                self.set_full_viewport(command_buffer, extent);
                device.cmd_set_scissor(command_buffer, 0, &[vk::Rect2D {
                    offset: vk::Offset2D { x: 0, y: 0 },
                    extent,
                }]);
                device.cmd_bind_pipeline(command_buffer, vk::PipelineBindPoint::GRAPHICS, blur.blur_v);
                device.cmd_bind_descriptor_sets(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    blur.sampler_layout,
                    0,
                    &[set_b],
                    &[],
                );
                let vertex_buffers = [blur.unit_quad, instance_buffer];
                let offsets = [0u64, 0u64];
                device.cmd_bind_vertex_buffers(command_buffer, 0, &vertex_buffers, &offsets);
                device.cmd_push_constants(
                    command_buffer,
                    blur.sampler_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &shadow.blur_push.bytes(),
                );
                device.cmd_draw(
                    command_buffer,
                    6,
                    1,
                    0,
                    shadow.blur_v_first_instance,
                );
                device.cmd_end_rendering(command_buffer);
            }
            self.transition_image(
                command_buffer,
                scratch_a.image.handle(),
                vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL,
                vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL,
                vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
                vk::PipelineStageFlags::FRAGMENT_SHADER,
                vk::AccessFlags::COLOR_ATTACHMENT_WRITE,
                vk::AccessFlags::SHADER_READ,
            );

            // 4. Composite: tint the blurred mask into the main target. The
            // first composite clears the target (the frame's initial clear);
            // later ones load it so multiple shadows accumulate.
            let load_op = if first_composite {
                vk::AttachmentLoadOp::CLEAR
            } else {
                vk::AttachmentLoadOp::LOAD
            };
            let composite_attachment = vk::RenderingAttachmentInfo::default()
                .image_view(resolve_view)
                .image_layout(resolve_layout)
                .load_op(load_op)
                .store_op(vk::AttachmentStoreOp::STORE)
                .clear_value(clear);
            self.begin_rendering(command_buffer, extent, &[composite_attachment], None);
            unsafe {
                self.set_full_viewport(command_buffer, extent);
                let scissor = clip_scissor(&shadow.composite_push, extent);
                device.cmd_set_scissor(command_buffer, 0, &[scissor]);
                device.cmd_bind_pipeline(command_buffer, vk::PipelineBindPoint::GRAPHICS, blur.composite);
                device.cmd_bind_descriptor_sets(
                    command_buffer,
                    vk::PipelineBindPoint::GRAPHICS,
                    blur.sampler_layout,
                    0,
                    &[set_a],
                    &[],
                );
                let vertex_buffers = [blur.unit_quad, instance_buffer];
                let offsets = [0u64, 0u64];
                device.cmd_bind_vertex_buffers(command_buffer, 0, &vertex_buffers, &offsets);
                device.cmd_push_constants(
                    command_buffer,
                    blur.sampler_layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &shadow.composite_push.bytes(),
                );
                device.cmd_draw(
                    command_buffer,
                    6,
                    1,
                    0,
                    shadow.composite_first_instance,
                );
                device.cmd_end_rendering(command_buffer);
            }
            first_composite = false;
            first_shadow = false;
        }
        Ok(())
    }

    /// Begins dynamic rendering over the given color attachments (no stencil).
    fn begin_rendering(
        &self,
        command_buffer: vk::CommandBuffer,
        extent: vk::Extent2D,
        color: &[vk::RenderingAttachmentInfo],
        stencil: Option<vk::RenderingAttachmentInfo>,
    ) {
        let mut info = vk::RenderingInfo::default()
            .render_area(vk::Rect2D {
                offset: vk::Offset2D { x: 0, y: 0 },
                extent,
            })
            .layer_count(1)
            .color_attachments(color);
        if let Some(stencil) = stencil.as_ref() {
            info = info.stencil_attachment(stencil);
        }
        unsafe {
            self.context
                .device()
                .cmd_begin_rendering(command_buffer, &info);
        }
    }

    /// Sets the full-surface viewport (the dynamic viewport state all pipelines
    /// share).
    fn set_full_viewport(&self, command_buffer: vk::CommandBuffer, extent: vk::Extent2D) {
        unsafe {
            self.context.device().cmd_set_viewport(
                command_buffer,
                0,
                &[vk::Viewport {
                    x: 0.0,
                    y: 0.0,
                    width: extent.width as f32,
                    height: extent.height as f32,
                    min_depth: 0.0,
                    max_depth: 1.0,
                }],
            );
        }
    }

    /// Transitions a scratch image between COLOR_ATTACHMENT and SHADER_READ_ONLY.
    #[allow(clippy::too_many_arguments)] // a barrier carries all its access fields
    fn transition_image(
        &self,
        command_buffer: vk::CommandBuffer,
        image: vk::Image,
        old_layout: vk::ImageLayout,
        new_layout: vk::ImageLayout,
        src_stage: vk::PipelineStageFlags,
        dst_stage: vk::PipelineStageFlags,
        src_access: vk::AccessFlags,
        dst_access: vk::AccessFlags,
    ) {
        let barrier = vk::ImageMemoryBarrier::default()
            .src_access_mask(src_access)
            .dst_access_mask(dst_access)
            .old_layout(old_layout)
            .new_layout(new_layout)
            .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .image(image)
            .subresource_range(
                vk::ImageSubresourceRange::default()
                    .aspect_mask(vk::ImageAspectFlags::COLOR)
                    .level_count(1)
                    .layer_count(1),
            );
        unsafe {
            self.context.device().cmd_pipeline_barrier(
                command_buffer,
                src_stage,
                dst_stage,
                vk::DependencyFlags::empty(),
                &[],
                &[],
                &[barrier],
            );
        }
    }

    /// Builds the color attachment for an R8 scratch pass (always CLEAR-loaded
    /// so each shadow starts from an all-zero mask).
    fn scratch_attachment(
        &self,
        view: vk::ImageView,
        clear: vk::ClearValue,
    ) -> vk::RenderingAttachmentInfo<'_> {
        vk::RenderingAttachmentInfo::default()
            .image_view(view)
            .image_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
            .load_op(vk::AttachmentLoadOp::CLEAR)
            .store_op(vk::AttachmentStoreOp::STORE)
            .clear_value(clear)
    }

    /// Allocates a combined-image-sampler descriptor set binding a scratch image
    /// view from the current ring slot's descriptor pool.
    fn allocate_scratch_descriptor(
        &self,
        view: vk::ImageView,
        sampler: vk::Sampler,
        layout: vk::DescriptorSetLayout,
    ) -> Result<vk::DescriptorSet, (RenderResult, String)> {
        let slot = self.ring.as_ref().unwrap().current_slot();
        let set = self.descriptor_pools[slot].allocate_set(layout)?;
        let image_info = vk::DescriptorImageInfo::default()
            .image_view(view)
            .image_layout(vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL)
            .sampler(sampler);
        let write = vk::WriteDescriptorSet::default()
            .dst_set(set)
            .dst_binding(0)
            .dst_array_element(0)
            .descriptor_type(vk::DescriptorType::COMBINED_IMAGE_SAMPLER)
            .image_info(std::slice::from_ref(&image_info));
        unsafe {
            self.context
                .device()
                .update_descriptor_sets(&[write], &[]);
        }
        Ok(set)
    }

    fn present(&mut self) -> Result<(), (RenderResult, String)> {
        let frame = self
            .pending_frame
            .take()
            .inspect(|frame| self.last_frame = Some(frame.clone()))
            .or_else(|| self.last_frame.clone());
        let Some(frame) = frame else {
            return Ok(());
        };
        let (Some(swapchain), Some(swapchain_loader)) =
            (self.swapchain, self.swapchain_loader.clone())
        else {
            return Err((
                RenderResult::InitFailed,
                "swapchain is not ready".to_string(),
            ));
        };

        let (image_index, _) = match unsafe {
            swapchain_loader.acquire_next_image(
                swapchain,
                u64::MAX,
                vk::Semaphore::null(),
                vk::Fence::null(),
            )
        } {
            Ok(result) => result,
            Err(e) if e == vk::Result::ERROR_OUT_OF_DATE_KHR || e == vk::Result::SUBOPTIMAL_KHR => {
                self.recreate_swapchain()?;
                return Ok(());
            }
            Err(e) => return Err(vk_error("vkAcquireNextImageKHR", e.as_raw())),
        };
        if image_index as usize >= self.swapchain_views.len() {
            return Err((
                RenderResult::VulkanError,
                "acquired swapchain image index out of range".to_string(),
            ));
        }

        let extent = self.swapchain_extent;
        let format = self.swapchain_format;
        let view = self.swapchain_views[image_index as usize].handle();
        let image = self.swapchain_images[image_index as usize];
        let clear = vk::ClearValue {
            // Swapchain is B8G8R8A8 (choose_surface_format); opaque dark
            // background [13, 13, 20] in BGRA channel order.
            color: vk::ClearColorValue {
                float32: [13.0 / 255.0, 13.0 / 255.0, 20.0 / 255.0, 1.0],
            },
        };

        self.render_into(
            &frame,
            extent,
            format,
            ResolveTarget::Swapchain { view, image },
            clear,
        )?;

        let swapchains = [swapchain];
        let image_indices = [image_index];
        let present_info = vk::PresentInfoKHR::default()
            .swapchains(&swapchains)
            .image_indices(&image_indices);
        match unsafe { swapchain_loader.queue_present(self.context.queue(), &present_info) } {
            Ok(true) | Ok(false) => Ok(()),
            Err(e) if e == vk::Result::ERROR_OUT_OF_DATE_KHR || e == vk::Result::SUBOPTIMAL_KHR => {
                self.recreate_swapchain()?;
                Ok(())
            }
            Err(e) => Err(vk_error("vkQueuePresentKHR", e.as_raw())),
        }
    }

    fn readback(
        &mut self,
        frame: &DecodedFrame,
        width: u32,
        height: u32,
    ) -> Result<Vec<u8>, (RenderResult, String)> {
        let extent = vk::Extent2D { width, height };
        let format = vk::Format::B8G8R8A8_UNORM;
        self.ensure_readback(extent, format)?;
        let view = self.readback.as_ref().unwrap().view.handle();
        let image = self.readback.as_ref().unwrap().image.handle();
        let clear = vk::ClearValue {
            color: vk::ClearColorValue { float32: [0.0; 4] },
        };
        self.render_into(
            frame,
            extent,
            format,
            ResolveTarget::Offscreen { view, image },
            clear,
        )?;

        let device = self.context.device();
        unsafe { device.queue_wait_idle(self.context.queue()) }
            .map_err(|e| vk_error("vkQueueWaitIdle", e.as_raw()))?;

        let size = (width as usize) * (height as usize) * 4;
        let mut pixels = vec![0u8; size];
        let ptr = self
            .readback
            .as_ref()
            .unwrap()
            .staging
            .mapped_ptr()
            .ok_or((
                RenderResult::InitFailed,
                "readback staging buffer is not host-mapped".to_string(),
            ))?;
        unsafe {
            std::ptr::copy_nonoverlapping(ptr, pixels.as_mut_ptr(), size);
        }
        Ok(pixels)
    }

    fn recreate_swapchain(&mut self) -> Result<(), (RenderResult, String)> {
        let Some(surface) = self.surface else {
            return Err((
                RenderResult::InitFailed,
                "Vulkan surface has not been created".to_string(),
            ));
        };
        self.destroy_swapchain_resources();

        let surface_loader = self.context.surface_loader().unwrap();
        let physical_device = self.context.physical_device();

        let caps = unsafe {
            surface_loader.get_physical_device_surface_capabilities(physical_device, surface)
        }
        .map_err(|e| vk_error("vkGetPhysicalDeviceSurfaceCapabilitiesKHR", e.as_raw()))?;
        let formats =
            unsafe { surface_loader.get_physical_device_surface_formats(physical_device, surface) }
                .map_err(|e| vk_error("vkGetPhysicalDeviceSurfaceFormatsKHR", e.as_raw()))?;
        let present_modes = unsafe {
            surface_loader.get_physical_device_surface_present_modes(physical_device, surface)
        }
        .map_err(|e| vk_error("vkGetPhysicalDeviceSurfacePresentModesKHR", e.as_raw()))?;

        let extent = choose_extent(&caps, self.requested_width, self.requested_height);
        let format = choose_surface_format(&formats);
        let present_mode = choose_present_mode(&present_modes);

        let desired = caps.min_image_count.saturating_add(1);
        let image_count = if caps.max_image_count > 0 {
            desired.min(caps.max_image_count)
        } else {
            desired
        };

        let create_info = vk::SwapchainCreateInfoKHR {
            surface,
            min_image_count: image_count.max(1),
            image_format: format.format,
            image_color_space: format.color_space,
            image_extent: extent,
            image_array_layers: 1,
            image_usage: vk::ImageUsageFlags::COLOR_ATTACHMENT,
            image_sharing_mode: vk::SharingMode::EXCLUSIVE,
            pre_transform: caps.current_transform,
            composite_alpha: choose_composite_alpha(caps.supported_composite_alpha),
            present_mode,
            clipped: vk::TRUE,
            ..Default::default()
        };

        let device = self.context.device();
        let swapchain_loader = ash::khr::swapchain::Device::new(self.context.instance(), device);
        let swapchain = unsafe { swapchain_loader.create_swapchain(&create_info, None) }
            .map_err(|e| vk_error("vkCreateSwapchainKHR", e.as_raw()))?;
        let images = unsafe { swapchain_loader.get_swapchain_images(swapchain) }.map_err(|e| {
            unsafe {
                swapchain_loader.destroy_swapchain(swapchain, None);
            }
            vk_error("vkGetSwapchainImagesKHR", e.as_raw())
        })?;
        let views = images
            .iter()
            .map(|&image| {
                ImageView::new(device, image, format.format, vk::ImageAspectFlags::COLOR)
                    .inspect_err(|_| unsafe {
                        swapchain_loader.destroy_swapchain(swapchain, None);
                    })
            })
            .collect::<Result<Vec<_>, _>>()?;

        self.swapchain_loader = Some(swapchain_loader);
        self.swapchain = Some(swapchain);
        self.swapchain_images = images;
        self.swapchain_views = views;
        self.swapchain_extent = extent;
        self.swapchain_format = format.format;
        Ok(())
    }
}

/// Builds the rasterizer scissor for a draw group's axis-aligned clip rect
/// (Slice 3 forward, clip-mechanism benchmark). The scissor covers
/// [floor(min), ceil(max)] so it never culls a pixel the fragment discard would
/// keep; the shader discard handles the exact float clip boundary.
fn clip_scissor(push: &crate::pipeline::PushConstants, extent: vk::Extent2D) -> vk::Rect2D {    if push.clip_active == 0 {
        return vk::Rect2D {
            offset: vk::Offset2D { x: 0, y: 0 },
            extent,
        };
    }
    let x = push.clip_min[0].floor() as i32;
    let y = push.clip_min[1].floor() as i32;
    let max_x = (push.clip_min[0] + push.clip_size[0]).ceil() as i32;
    let max_y = (push.clip_min[1] + push.clip_size[1]).ceil() as i32;
    vk::Rect2D {
        offset: vk::Offset2D { x, y },
        extent: vk::Extent2D {
            width: (max_x - x).max(0) as u32,
            height: (max_y - y).max(0) as u32,
        },
    }
}

/// Clamps a world-space rect to the render area and converts it to an integer
/// `vk::Rect2D` for `vkCmdClearAttachments`.
fn clear_rect_for(rect: crate::frame::Rect, extent: vk::Extent2D) -> vk::Rect2D {
    let x = rect.min.x.floor().max(0.0) as i32;
    let y = rect.min.y.floor().max(0.0) as i32;
    let max_x = rect.max.x.ceil().min(extent.width as f32) as i32;
    let max_y = rect.max.y.ceil().min(extent.height as f32) as i32;
    vk::Rect2D {
        offset: vk::Offset2D { x, y },
        extent: vk::Extent2D {
            width: (max_x - x).max(0) as u32,
            height: (max_y - y).max(0) as u32,
        },
    }
}

fn choose_extent(caps: &vk::SurfaceCapabilitiesKHR, width: u32, height: u32) -> vk::Extent2D {
    if caps.current_extent.width != u32::MAX {
        return caps.current_extent;
    }
    vk::Extent2D {
        width: width.clamp(caps.min_image_extent.width, caps.max_image_extent.width),
        height: height.clamp(caps.min_image_extent.height, caps.max_image_extent.height),
    }
}

fn choose_surface_format(formats: &[vk::SurfaceFormatKHR]) -> vk::SurfaceFormatKHR {
    for format in formats {
        if format.format == vk::Format::B8G8R8A8_UNORM
            && format.color_space == vk::ColorSpaceKHR::SRGB_NONLINEAR
        {
            return *format;
        }
    }
    formats
        .iter()
        .find(|f| f.format == vk::Format::B8G8R8A8_UNORM)
        .or_else(|| formats.first())
        .copied()
        .unwrap_or(vk::SurfaceFormatKHR {
            format: vk::Format::B8G8R8A8_UNORM,
            color_space: vk::ColorSpaceKHR::SRGB_NONLINEAR,
        })
}

fn choose_present_mode(modes: &[vk::PresentModeKHR]) -> vk::PresentModeKHR {
    for mode in modes {
        if *mode == vk::PresentModeKHR::MAILBOX {
            return *mode;
        }
    }
    for mode in modes {
        if *mode == vk::PresentModeKHR::FIFO {
            return *mode;
        }
    }
    modes.first().copied().unwrap_or(vk::PresentModeKHR::FIFO)
}

fn choose_composite_alpha(supported: vk::CompositeAlphaFlagsKHR) -> vk::CompositeAlphaFlagsKHR {
    if supported.contains(vk::CompositeAlphaFlagsKHR::OPAQUE) {
        return vk::CompositeAlphaFlagsKHR::OPAQUE;
    }
    if supported.contains(vk::CompositeAlphaFlagsKHR::INHERIT) {
        return vk::CompositeAlphaFlagsKHR::INHERIT;
    }
    vk::CompositeAlphaFlagsKHR::OPAQUE
}
