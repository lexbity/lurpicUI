//! RAII wrappers over ash handles (Q10/FR-13).
//!
//! Each wrapper owns a Vulkan handle and `Drop`-destroys it through the owning
//! device. Arena ownership contract: the context owns the device and is dropped
//! last, so every child resource's `Drop` runs while the device is still alive.
//! Destroying a parent while children exist is a validation error; callers
//! build resources inside the context arena and never outlive it.

use ash::vk;

use crate::error::vk_error;
use crate::RenderResult;

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct ShaderModule {
    device: ash::Device,
    handle: vk::ShaderModule,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl ShaderModule {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(device: &ash::Device, code: &[u32]) -> Result<Self, (RenderResult, String)> {
        let info = vk::ShaderModuleCreateInfo {
            code_size: code.len() * core::mem::size_of::<u32>(),
            p_code: code.as_ptr(),
            ..Default::default()
        };
        let handle = unsafe { device.create_shader_module(&info, None) }
            .map_err(|e| vk_error("vkCreateShaderModule", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::ShaderModule {
        self.handle
    }
}

impl Drop for ShaderModule {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_shader_module(self.handle, None);
        }
    }
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct PipelineLayout {
    device: ash::Device,
    handle: vk::PipelineLayout,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl PipelineLayout {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(
        device: &ash::Device,
        set_layouts: &[vk::DescriptorSetLayout],
        push_constant_ranges: &[vk::PushConstantRange],
    ) -> Result<Self, (RenderResult, String)> {
        let info = vk::PipelineLayoutCreateInfo {
            set_layout_count: set_layouts.len() as u32,
            p_set_layouts: set_layouts.as_ptr(),
            push_constant_range_count: push_constant_ranges.len() as u32,
            p_push_constant_ranges: push_constant_ranges.as_ptr(),
            ..Default::default()
        };
        let handle = unsafe { device.create_pipeline_layout(&info, None) }
            .map_err(|e| vk_error("vkCreatePipelineLayout", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::PipelineLayout {
        self.handle
    }
}

impl Drop for PipelineLayout {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_pipeline_layout(self.handle, None);
        }
    }
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct Pipeline {
    device: ash::Device,
    handle: vk::Pipeline,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl Pipeline {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(
        device: &ash::Device,
        pipeline_cache: vk::PipelineCache,
        create_infos: &[vk::GraphicsPipelineCreateInfo],
    ) -> Result<Self, (RenderResult, String)> {
        let pipelines =
            unsafe { device.create_graphics_pipelines(pipeline_cache, create_infos, None) }
                .map_err(|(_, e)| vk_error("vkCreateGraphicsPipelines", e.as_raw()))?;
        let handle = pipelines[0];        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::Pipeline {
        self.handle
    }
}

impl Drop for Pipeline {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_pipeline(self.handle, None);
        }
    }
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct ImageView {
    device: ash::Device,
    handle: vk::ImageView,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl ImageView {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(
        device: &ash::Device,
        image: vk::Image,
        format: vk::Format,
        aspect_mask: vk::ImageAspectFlags,
    ) -> Result<Self, (RenderResult, String)> {
        let info = vk::ImageViewCreateInfo {
            image,
            view_type: vk::ImageViewType::TYPE_2D,
            format,
            subresource_range: vk::ImageSubresourceRange {
                aspect_mask,
                level_count: 1,
                layer_count: 1,
                ..Default::default()
            },
            ..Default::default()
        };
        let handle = unsafe { device.create_image_view(&info, None) }
            .map_err(|e| vk_error("vkCreateImageView", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::ImageView {
        self.handle
    }
}

impl Drop for ImageView {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_image_view(self.handle, None);
        }
    }
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct CommandPool {
    device: ash::Device,
    handle: vk::CommandPool,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl CommandPool {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(
        device: &ash::Device,
        queue_family: u32,
        reset_flags: bool,
    ) -> Result<Self, (RenderResult, String)> {
        let info = vk::CommandPoolCreateInfo {
            flags: if reset_flags {
                vk::CommandPoolCreateFlags::RESET_COMMAND_BUFFER
            } else {
                vk::CommandPoolCreateFlags::empty()
            },
            queue_family_index: queue_family,
            ..Default::default()
        };
        let handle = unsafe { device.create_command_pool(&info, None) }
            .map_err(|e| vk_error("vkCreateCommandPool", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::CommandPool {
        self.handle
    }

    pub fn allocate_primary(&self) -> Result<vk::CommandBuffer, (RenderResult, String)> {
        let info = vk::CommandBufferAllocateInfo {
            command_pool: self.handle,
            level: vk::CommandBufferLevel::PRIMARY,
            command_buffer_count: 1,
            ..Default::default()
        };
        let buffers = unsafe { self.device.allocate_command_buffers(&info) }
            .map_err(|e| vk_error("vkAllocateCommandBuffers", e.as_raw()))?;
        Ok(buffers[0])
    }
}

impl Drop for CommandPool {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_command_pool(self.handle, None);
        }
    }
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct Fence {
    device: ash::Device,
    handle: vk::Fence,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl Fence {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(device: &ash::Device, signaled: bool) -> Result<Self, (RenderResult, String)> {
        let info = vk::FenceCreateInfo {
            flags: if signaled {
                vk::FenceCreateFlags::SIGNALED
            } else {
                vk::FenceCreateFlags::empty()
            },
            ..Default::default()
        };
        let handle = unsafe { device.create_fence(&info, None) }
            .map_err(|e| vk_error("vkCreateFence", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::Fence {
        self.handle
    }

    pub fn wait(&self, timeout_ns: u64) -> Result<(), (RenderResult, String)> {
        unsafe { self.device.wait_for_fences(&[self.handle], true, timeout_ns) }
            .map_err(|e| vk_error("vkWaitForFences", e.as_raw()))
    }

    pub fn reset(&self) -> Result<(), (RenderResult, String)> {
        unsafe { self.device.reset_fences(&[self.handle]) }
            .map_err(|e| vk_error("vkResetFences", e.as_raw()))
    }
}

impl Drop for Fence {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_fence(self.handle, None);
        }
    }
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
pub struct Semaphore {
    device: ash::Device,
    handle: vk::Semaphore,
}

#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
impl Semaphore {
#[allow(dead_code)] // constructed by the GPU pipeline (Slice 3+)
    pub fn new(device: &ash::Device) -> Result<Self, (RenderResult, String)> {
        let handle = unsafe { device.create_semaphore(&Default::default(), None) }
            .map_err(|e| vk_error("vkCreateSemaphore", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
        })
    }

    pub fn handle(&self) -> vk::Semaphore {
        self.handle
    }
}

impl Drop for Semaphore {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_semaphore(self.handle, None);
        }
    }
}
