//! The renderer's Vulkan lifecycle: instance/device/queue/swapchain + the
//! CPU-stepping-stone present path, reimplemented on `ash` (Slice 2).
//!
//! The hand-rolled `dlopen`/`dlsym` loader is gone; `ash` provides the dispatch
//! layer. `gpu::context::AshContext` owns instance/device/queue/allocator; this
//! module adds the surface/swapchain/present layer and the frame state.

#[cfg(target_os = "android")]
use std::ffi::c_void;
use std::sync::{Mutex, OnceLock};

use ash::vk;
use ash::vk::Handle;

use crate::clear_last_error;
use crate::error::vk_error;
use crate::frame::{decode_frame, last_vertex_count, DecodedFrame, FrameStats};
use crate::gpu::context::{AshContext, GpuContext, PhysicalDeviceFeatures};
use crate::gpu::surface;
use crate::RenderResult;

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

pub struct VulkanState {
    // Drop order: swapchain/surface/staging first, then the context (which
    // drops command pool -> device -> instance via its RAII guards).
    surface: Option<vk::SurfaceKHR>,
    swapchain_loader: Option<ash::khr::swapchain::Device>,
    swapchain: Option<vk::SwapchainKHR>,
    swapchain_images: Vec<vk::Image>,
    swapchain_extent: vk::Extent2D,
    swapchain_format: vk::Format,
    command_buffer: Option<vk::CommandBuffer>,
    #[cfg(feature = "cpu-fallback")]
    staging: Option<crate::gpu::allocator::GpuBuffer>,
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
    // frame_stats() reflects the last submitted packet (the Backend skips
    // presentation on the headless path).
    if state.surface.is_some() {
        state.present()?;
    }
    Ok(())
}

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
    stats.vertex_count = last_vertex_count();
    stats
}

impl VulkanState {
    fn new(context: AshContext) -> Result<Self, (RenderResult, String)> {
        let queue_family = context.queue_family();
        let mut caps = VulkanCapabilities::empty();
        let props = unsafe { context.instance().get_physical_device_properties(context.physical_device()) };
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

        Ok(Self {
            surface: None,
            swapchain_loader: None,
            swapchain: None,
            swapchain_images: Vec::new(),
            swapchain_extent: vk::Extent2D { width: 1, height: 1 },
            swapchain_format: vk::Format::UNDEFINED,
            command_buffer: None,
            #[cfg(feature = "cpu-fallback")]
            staging: None,
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
        self.command_buffer = None;
        self.swapchain_format = vk::Format::UNDEFINED;
    }

    #[cfg(feature = "cpu-fallback")]
    fn present(&mut self) -> Result<(), (RenderResult, String)> {
        let Some(swapchain) = self.swapchain else {
            return Err((
                RenderResult::InitFailed,
                "swapchain is not ready".to_string(),
            ));
        };
        let swapchain_loader = match self.swapchain_loader.clone() {
            Some(loader) => loader,
            None => {
                return Err((
                    RenderResult::InitFailed,
                    "swapchain loader is not ready".to_string(),
                ));
            }
        };
        let Some(command_buffer) = self.command_buffer else {
            return Err((
                RenderResult::InitFailed,
                "command buffer is not allocated".to_string(),
            ));
        };

        let frame = self
            .pending_frame
            .take()
            .map(|frame| {
                self.last_frame = Some(frame.clone());
                frame
            })
            .or_else(|| self.last_frame.clone());

        let (width, height) = (self.swapchain_extent.width, self.swapchain_extent.height);
        let pixels = crate::raster::rasterize_frame(frame.as_ref(), width, height);
        self.ensure_staging((width as usize) * (height as usize) * 4)?;
        self.upload_staging(&pixels)?;

        let device = self.context.device();
        let queue = self.context.queue();

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
        if image_index as usize >= self.swapchain_images.len() {
            return Err((
                RenderResult::VulkanError,
                "acquired swapchain image index out of range".to_string(),
            ));
        }

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

        let image = self.swapchain_images[image_index as usize];
        let barrier_to_transfer = vk::ImageMemoryBarrier::default()
            .src_access_mask(vk::AccessFlags::empty())
            .dst_access_mask(vk::AccessFlags::TRANSFER_WRITE)
            .old_layout(vk::ImageLayout::UNDEFINED)
            .new_layout(vk::ImageLayout::TRANSFER_DST_OPTIMAL)
            .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .image(image)
            .subresource_range(vk::ImageSubresourceRange::default().aspect_mask(vk::ImageAspectFlags::COLOR).level_count(1).layer_count(1));

        unsafe {
            device.cmd_pipeline_barrier(
                command_buffer,
                vk::PipelineStageFlags::TOP_OF_PIPE,
                vk::PipelineStageFlags::TRANSFER,
                vk::DependencyFlags::empty(),
                &[],
                &[],
                &[barrier_to_transfer],
            );
        }

        let copy_region = vk::BufferImageCopy::default()
            .image_subresource(
                vk::ImageSubresourceLayers::default()
                    .aspect_mask(vk::ImageAspectFlags::COLOR)
                    .layer_count(1),
            )
            .image_extent(vk::Extent3D {
                width,
                height,
                depth: 1,
            });
        let staging_buffer = self.staging.as_ref().unwrap().buffer();
        unsafe {
            device.cmd_copy_buffer_to_image(
                command_buffer,
                staging_buffer,
                image,
                vk::ImageLayout::TRANSFER_DST_OPTIMAL,
                &[copy_region],
            );
        }

        let barrier_to_present = vk::ImageMemoryBarrier::default()
            .src_access_mask(vk::AccessFlags::TRANSFER_WRITE)
            .dst_access_mask(vk::AccessFlags::empty())
            .old_layout(vk::ImageLayout::TRANSFER_DST_OPTIMAL)
            .new_layout(vk::ImageLayout::PRESENT_SRC_KHR)
            .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
            .image(image)
            .subresource_range(vk::ImageSubresourceRange::default().aspect_mask(vk::ImageAspectFlags::COLOR).level_count(1).layer_count(1));
        unsafe {
            device.cmd_pipeline_barrier(
                command_buffer,
                vk::PipelineStageFlags::TRANSFER,
                vk::PipelineStageFlags::BOTTOM_OF_PIPE,
                vk::DependencyFlags::empty(),
                &[],
                &[],
                &[barrier_to_present],
            );
        }

        unsafe { device.end_command_buffer(command_buffer) }
            .map_err(|e| vk_error("vkEndCommandBuffer", e.as_raw()))?;

        let command_buffers = [command_buffer];
        let submit_info = vk::SubmitInfo::default().command_buffers(&command_buffers);
        unsafe { device.queue_submit(queue, &[submit_info], vk::Fence::null()) }
            .map_err(|e| vk_error("vkQueueSubmit", e.as_raw()))?;
        unsafe { device.queue_wait_idle(queue) }
            .map_err(|e| vk_error("vkQueueWaitIdle", e.as_raw()))?;

        let swapchains = [swapchain];
        let image_indices = [image_index];
        let present_info = vk::PresentInfoKHR::default()
            .swapchains(&swapchains)
            .image_indices(&image_indices);
        match unsafe { swapchain_loader.queue_present(queue, &present_info) } {
            Ok(true) | Ok(false) => Ok(()),
            Err(e) if e == vk::Result::ERROR_OUT_OF_DATE_KHR || e == vk::Result::SUBOPTIMAL_KHR => {
                self.recreate_swapchain()?;
                Ok(())
            }
            Err(e) => Err(vk_error("vkQueuePresentKHR", e.as_raw())),
        }
    }

    #[cfg(not(feature = "cpu-fallback"))]
    fn present(&mut self) -> Result<(), (RenderResult, String)> {
        // The GPU pipeline replaces this in Slice 3; until then the stepping
        // stone requires cpu-fallback.
        Err((
            RenderResult::Unsupported,
            "GPU pipeline not yet implemented (cpu-fallback feature disabled)".to_string(),
        ))
    }

    #[cfg(feature = "cpu-fallback")]
    fn ensure_staging(&mut self, size: usize) -> Result<(), (RenderResult, String)> {
        if self
            .staging
            .as_ref()
            .is_some_and(|b| b.size() >= size as u64)
        {
            return Ok(());
        }
        self.staging = None;
        let buffer = self
            .context
            .allocator()
            .create_buffer(
                size as u64,
                vk::BufferUsageFlags::TRANSFER_SRC,
                crate::gpu::allocator::MemoryLocation::CpuToGpu,
            )?;
        self.staging = Some(buffer);
        Ok(())
    }

    #[cfg(feature = "cpu-fallback")]
    fn upload_staging(&mut self, pixels: &[u8]) -> Result<(), (RenderResult, String)> {
        let Some(staging) = self.staging.as_mut() else {
            return Err((
                RenderResult::InitFailed,
                "staging buffer is not allocated".to_string(),
            ));
        };
        staging.write(0, pixels)
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
            image_usage: if caps.supported_usage_flags.contains(vk::ImageUsageFlags::TRANSFER_DST) {
                vk::ImageUsageFlags::TRANSFER_DST
            } else {
                caps.supported_usage_flags
            },
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

        let command_buffer = {
            let alloc_info = vk::CommandBufferAllocateInfo {
                command_pool: self.context.command_pool(),
                level: vk::CommandBufferLevel::PRIMARY,
                command_buffer_count: 1,
                ..Default::default()
            };
            let buffers = unsafe { device.allocate_command_buffers(&alloc_info) }
                .map_err(|e| {
                    unsafe {
                        swapchain_loader.destroy_swapchain(swapchain, None);
                    }
                    vk_error("vkAllocateCommandBuffers", e.as_raw())
                })?;
            buffers[0]
        };

        self.swapchain_loader = Some(swapchain_loader);
        self.swapchain = Some(swapchain);
        self.swapchain_images = images;
        self.swapchain_extent = extent;
        self.swapchain_format = format.format;
        self.command_buffer = Some(command_buffer);
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
