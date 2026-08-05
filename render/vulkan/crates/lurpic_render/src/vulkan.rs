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

use crate::clear_last_error;
use crate::error::vk_error;
use crate::frame::{decode_frame, DecodedFrame};
#[cfg(feature = "test-exports")]
use crate::frame::FrameStats;
use crate::frame_encoder::FrameEncoder;
use crate::gpu::allocator::{GpuBuffer, ImageAllocation, MemoryLocation};
use crate::gpu::context::{AshContext, GpuContext, PhysicalDeviceFeatures};
use crate::gpu::resources::{Image, ImageView};
use crate::gpu::surface;
use crate::pipeline::solid::SolidPipeline;
use crate::ring_buffer::{InstanceRing, DEFAULT_SLOT_BYTES};
use crate::RenderResult;

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

/// Where a frame's resolved pixels land.
enum ResolveTarget {
    Swapchain { view: vk::ImageView, image: vk::Image },
    Offscreen { view: vk::ImageView, image: vk::Image },
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
    ring: Option<InstanceRing>,
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
    let state = guard
        .as_ref()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
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
    let state = guard
        .as_ref()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
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

#[cfg(not(target_os = "android"))]
pub fn create_xcb_surface(
    instance: usize,
    connection: usize,
    window: u32,
    width: u32,
    height: u32,
) -> Result<usize, (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard
        .as_mut()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
    if state.context.instance().handle().as_raw() as usize != instance {
        return Err((
            RenderResult::InvalidHandle,
            "instance handle does not match the active renderer".to_string(),
        ));
    }
    if state.surface.is_some() {
        return Ok(state.surface.unwrap().as_raw() as usize);
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
    let state = guard
        .as_mut()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
    if state.context.instance().handle().as_raw() as usize != instance {
        return Err((
            RenderResult::InvalidHandle,
            "instance handle does not match the active renderer".to_string(),
        ));
    }
    if state.surface.is_some() {
        return Ok(state.surface.unwrap().as_raw() as usize);
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
    let state = guard
        .as_mut()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
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
    let state = guard
        .as_mut()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
    let width = width.max(1) as u32;
    let height = height.max(1) as u32;
    state.requested_width = width;
    state.requested_height = height;
    if state.surface.is_some() {
        state.recreate_swapchain()?;
    }
    Ok(())
}

pub fn submit_frame(data: *const u8, len: usize) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard
        .as_mut()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
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
pub fn readback_frame(data: *const u8, len: usize, width: u32, height: u32) -> Result<Vec<u8>, (RenderResult, String)> {
    let mut guard = state_lock();
    let state = guard
        .as_mut()
        .ok_or((RenderResult::InitFailed, "renderer is not initialized".to_string()))?;
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
        for (dst, src) in caps.device_name.iter_mut().take(len).zip(bytes.iter().take(len)) {
            *dst = *src as std::ffi::c_char;
        }

        let ring = InstanceRing::new(context.allocator(), context.device(), 2, DEFAULT_SLOT_BYTES)?;
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

        Ok(Self {
            surface: None,
            swapchain_loader: None,
            swapchain: None,
            swapchain_images: Vec::new(),
            swapchain_views: Vec::new(),
            swapchain_extent: vk::Extent2D { width: 1, height: 1 },
            swapchain_format: vk::Format::UNDEFINED,
            command_buffer: Some(command_buffer),
            msaa: None,
            readback: None,
            solid_pipeline: None,
            ring: Some(ring),
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
    fn ensure_solid_pipeline(&mut self, format: vk::Format, swapped: bool) -> Result<SolidHandles, (RenderResult, String)> {
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
        let allocation = allocator.allocate_image_memory(image.handle(), requirements, MemoryLocation::GpuOnly)?;
        unsafe {
            device.bind_image_memory(image.handle(), allocation.memory(), 0)
        }
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
        let allocation = allocator.allocate_image_memory(image.handle(), requirements, MemoryLocation::GpuOnly)?;
        unsafe {
            device.bind_image_memory(image.handle(), allocation.memory(), 0)
        }
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
            device.reset_command_pool(self.context.command_pool(), vk::CommandPoolResetFlags::empty())
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

        // Encode the frame into the per-frame instance ring.
        let surface_size = [extent.width as f32, extent.height as f32];
        let encoded = {
            let ring = self.ring.as_mut().expect("ring initialized");
            ring.begin_frame()?;
            self.encoder
                .encode(frame, ring, self.context.allocator(), surface_size)?
        };
        let instance_buffer = self.ring.as_ref().unwrap().current_buffer();

        let command_buffer = self.begin_command_buffer()?;
        let device = self.context.device();


        // Transition the resolve image to COLOR_ATTACHMENT_OPTIMAL.
        let resolve_image = match resolve {
            ResolveTarget::Swapchain { image, .. } | ResolveTarget::Offscreen { image, .. } => image,
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
            ResolveTarget::Swapchain { view, .. } => (*view, vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL),
            ResolveTarget::Offscreen { view, .. } => (*view, vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL),
        };
        let msaa = pipeline.samples != vk::SampleCountFlags::TYPE_1;
        let color_attachment = if msaa {
            vk::RenderingAttachmentInfo::default()
                .image_view(self.msaa.as_ref().unwrap().view.handle())
                .image_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
                .resolve_mode(vk::ResolveModeFlags::AVERAGE)
                .resolve_image_view(resolve_view)
                .resolve_image_layout(resolve_layout)
                .load_op(vk::AttachmentLoadOp::CLEAR)
                .store_op(vk::AttachmentStoreOp::STORE)
                .clear_value(clear)
        } else {
            // No MSAA: render directly into the resolve image.
            vk::RenderingAttachmentInfo::default()
                .image_view(resolve_view)
                .image_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
                .load_op(vk::AttachmentLoadOp::CLEAR)
                .store_op(vk::AttachmentStoreOp::STORE)
                .clear_value(clear)
        };
        let color_attachments = [color_attachment];
        let rendering_info = vk::RenderingInfo::default()
            .render_area(vk::Rect2D {
                offset: vk::Offset2D { x: 0, y: 0 },
                extent,
            })
            .layer_count(1)
            .color_attachments(&color_attachments);
        unsafe {
            device.cmd_begin_rendering(command_buffer, &rendering_info);
            device.cmd_bind_pipeline(command_buffer, vk::PipelineBindPoint::GRAPHICS, pipeline.handle);
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
            device.cmd_set_scissor(
                command_buffer,
                0,
                &[vk::Rect2D {
                    offset: vk::Offset2D { x: 0, y: 0 },
                    extent,
                }],
            );
            let vertex_buffers = [pipeline.unit_quad, instance_buffer];
            let offsets = [0u64, 0u64];
            device.cmd_bind_vertex_buffers(command_buffer, 0, &vertex_buffers, &offsets);
            for group in &encoded.groups {
                device.cmd_push_constants(
                    command_buffer,
                    pipeline.layout,
                    vk::ShaderStageFlags::VERTEX | vk::ShaderStageFlags::FRAGMENT,
                    0,
                    &group.push.bytes(),
                );
                device.cmd_draw(command_buffer, 6, group.instance_count, 0, group.first_instance);
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

    fn present(&mut self) -> Result<(), (RenderResult, String)> {
        let frame = self
            .pending_frame
            .take()
            .map(|frame| {
                self.last_frame = Some(frame.clone());
                frame
            })
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
            swapchain_loader.acquire_next_image(swapchain, u64::MAX, vk::Semaphore::null(), vk::Fence::null())
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

    fn readback(&mut self, frame: &DecodedFrame, width: u32, height: u32) -> Result<Vec<u8>, (RenderResult, String)> {
        let extent = vk::Extent2D { width, height };
        let format = vk::Format::B8G8R8A8_UNORM;
        self.ensure_readback(extent, format)?;
        let view = self.readback.as_ref().unwrap().view.handle();
        let image = self.readback.as_ref().unwrap().image.handle();
        let clear = vk::ClearValue {
            color: vk::ClearColorValue {
                float32: [0.0; 4],
            },
        };
        self.render_into(
            frame,
            extent,
            format,
            ResolveTarget::Offscreen { view, image },
            clear,
        )?;

        let device = self.context.device();
        unsafe {
            device.queue_wait_idle(self.context.queue())
        }
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
        let formats = unsafe {
            surface_loader.get_physical_device_surface_formats(physical_device, surface)
        }
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
        let images = unsafe { swapchain_loader.get_swapchain_images(swapchain) }
            .map_err(|e| {
                unsafe {
                    swapchain_loader.destroy_swapchain(swapchain, None);
                }
                vk_error("vkGetSwapchainImagesKHR", e.as_raw())
            })?;
        let views = images
            .iter()
            .map(|&image| {
                ImageView::new(device, image, format.format, vk::ImageAspectFlags::COLOR)
                    .map_err(|err| {
                        unsafe {
                            swapchain_loader.destroy_swapchain(swapchain, None);
                        }
                        err
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
    modes
        .first()
        .copied()
        .unwrap_or(vk::PresentModeKHR::FIFO)
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
