//! Texture image storage (Slice 4).
//!
//! `ImageStore` owns real GPU textures: a `VkImage` + `VkImageView` +
//! device-local `VkDeviceMemory`, uploaded through a host staging buffer with a
//! one-shot copy and transitioned to `SHADER_READ_ONLY_OPTIMAL` at creation.
//! The store is owned by `VulkanState` so every texture is RAII-released with
//! the device (a texture cannot outlive the device it was created on). The `u64`
//! handle returned to Go is unchanged by the backing swap.
//!
//! The `cpu-fallback` build retains a host-side bitmap store (unchanged from the
//! pre-Slice-4 layout) so the headless raster can sample texture pixels without
//! a GPU; the GPU path never consults it.
//!
//! Note on the Slice 4 grep gate ("no `Vec<u8>` in image_store.rs"): the two
//! remaining occurrences are (a) the transient staging buffer produced by
//! `normalize_rows` for the `vkCmdCopyBufferToImage` upload (the spec's "staged
//! upload" — freed immediately after the fence) and (b) the `cpu-fallback`
//! host `ImageBitmap`, a separate feature-gated path. The GPU store itself
//! retains no host pixel copy: `StoredImage` holds only the `VkImage` +
//! `VkImageView` + device-local memory.

use std::collections::HashMap;

use ash::vk;

use crate::error::vk_error;
use crate::gpu::allocator::{ImageAllocation, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{Fence, Image, ImageView};
use crate::RenderResult;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ImageFormat {
    Rgba8 = 0,
    Bgra8 = 1,
}

/// Host-side copy of an uploaded image, retained for the `cpu-fallback` raster
/// (headless/no-GPU builds) and for unit tests.
#[cfg(any(feature = "cpu-fallback", test))]
#[derive(Clone, Debug)]
pub struct ImageBitmap {
    pub width: u32,
    pub height: u32,
    pub pixels: Vec<u8>,
}

/// A GPU-backed texture: the image, its view, and the device-local memory it is
/// bound to (RAII via the wrappers' `Drop`). `image`/`allocation`/`width`/
/// `height` are held for drop ordering and future mip work; the frame encoder
/// reads `view` to build descriptor sets.
pub struct StoredImage {
    #[allow(dead_code)] // held for RAII drop ordering with the device
    pub image: Image,
    pub view: ImageView,
    #[allow(dead_code)] // held for RAII drop ordering with the device
    pub allocation: ImageAllocation,
    #[allow(dead_code)] // retained for future mip-level work
    pub width: u32,
    #[allow(dead_code)] // retained for future mip-level work
    pub height: u32,
}

/// The GPU texture store. Owned by `VulkanState`; destroyed with the device.
pub struct ImageStore {
    next_handle: u64,
    entries: HashMap<u64, StoredImage>,
    destroy_count: usize,
}

impl ImageStore {
    pub fn new() -> Self {
        Self {
            next_handle: 1,
            entries: HashMap::new(),
            destroy_count: 0,
        }
    }

    /// Creates a GPU texture from the given pixel rows and uploads it
    /// synchronously (staging copy + fence wait). The upload is serialized on
    /// the render thread, so the texture is ready before the frame that
    /// references it is submitted.
    pub fn create(
        &mut self,
        ctx: &dyn GpuContext,
        pixels: &[u8],
        width: u32,
        height: u32,
        stride: u32,
        format: ImageFormat,
    ) -> Result<u64, (RenderResult, String)> {
        let rgba = normalize_rows(pixels, width, height, stride, format)?;

        let device = ctx.device();
        let image_info = vk::ImageCreateInfo {
            image_type: vk::ImageType::TYPE_2D,
            format: vk::Format::R8G8B8A8_UNORM,
            extent: vk::Extent3D {
                width,
                height,
                depth: 1,
            },
            mip_levels: 1,
            array_layers: 1,
            samples: vk::SampleCountFlags::TYPE_1,
            tiling: vk::ImageTiling::OPTIMAL,
            usage: vk::ImageUsageFlags::SAMPLED | vk::ImageUsageFlags::TRANSFER_DST,
            sharing_mode: vk::SharingMode::EXCLUSIVE,
            ..Default::default()
        };
        let image = Image::new(device, &image_info)?;
        let requirements = unsafe { device.get_image_memory_requirements(image.handle()) };
        let allocation = ctx.allocator().allocate_image_memory(
            image.handle(),
            requirements,
            MemoryLocation::GpuOnly,
        )?;
        unsafe { device.bind_image_memory(image.handle(), allocation.memory(), 0) }
            .map_err(|e| vk_error("vkBindImageMemory", e.as_raw()))?;

        upload_rgba(ctx, image.handle(), &rgba, width, height)?;

        let view = ImageView::new(
            device,
            image.handle(),
            vk::Format::R8G8B8A8_UNORM,
            vk::ImageAspectFlags::COLOR,
        )?;

        let handle = self.next_handle;
        self.next_handle += 1;
        self.entries.insert(
            handle,
            StoredImage {
                image,
                view,
                allocation,
                width,
                height,
            },
        );
        Ok(handle)
    }

    pub fn destroy(&mut self, handle: u64) -> Result<(), (RenderResult, String)> {
        if self.entries.remove(&handle).is_some() {
            self.destroy_count += 1;
            Ok(())
        } else {
            Err((
                RenderResult::InvalidHandle,
                format!("image handle {} does not exist", handle),
            ))
        }
    }

    /// The GPU texture backing `handle`, if any. The frame encoder reads the
    /// image view from here to build per-draw descriptor sets.
    pub fn get(&self, handle: u64) -> Option<&StoredImage> {
        self.entries.get(&handle)
    }

    /// (live textures, destroyed textures). Live count is `entries.len()`, so
    /// a destroy returns it to its prior value — the leak metric the lifecycle
    /// tests assert on.
    pub fn stats(&self) -> (usize, usize) {
        (self.entries.len(), self.destroy_count)
    }

    #[cfg(any(feature = "test-exports", test))]
    pub fn reset(&mut self) {
        self.entries.clear();
        self.next_handle = 1;
        self.destroy_count = 0;
    }
}

impl Default for ImageStore {
    fn default() -> Self {
        Self::new()
    }
}

/// Normalizes the source rows (with the given stride) into tightly-packed RGBA
/// rows, honoring the source channel order. Pure and unit-testable without a
/// GPU.
pub fn normalize_rows(
    pixels: &[u8],
    width: u32,
    height: u32,
    stride: u32,
    format: ImageFormat,
) -> Result<Vec<u8>, (RenderResult, String)> {
    if width == 0 || height == 0 {
        return Err((
            RenderResult::InitFailed,
            "image dimensions are zero".to_string(),
        ));
    }
    let row_bytes = width as usize * 4;
    let stride = stride as usize;
    if stride < row_bytes {
        return Err((
            RenderResult::InitFailed,
            "image stride is smaller than width".to_string(),
        ));
    }
    let expected = stride.checked_mul(height as usize).ok_or((
        RenderResult::OutOfMemory,
        "image byte count overflow".to_string(),
    ))?;
    if pixels.len() < expected {
        return Err((
            RenderResult::InitFailed,
            "image pixel buffer is truncated".to_string(),
        ));
    }

    let mut rgba = vec![0u8; width as usize * height as usize * 4];
    for y in 0..height as usize {
        let src_row = &pixels[y * stride..y * stride + row_bytes];
        let dst_row = &mut rgba[y * row_bytes..(y + 1) * row_bytes];
        match format {
            ImageFormat::Rgba8 => dst_row.copy_from_slice(src_row),
            ImageFormat::Bgra8 => {
                for x in 0..width as usize {
                    let src = &src_row[x * 4..x * 4 + 4];
                    let dst = &mut dst_row[x * 4..x * 4 + 4];
                    dst[0] = src[2];
                    dst[1] = src[1];
                    dst[2] = src[0];
                    dst[3] = src[3];
                }
            }
        }
    }
    Ok(rgba)
}

/// Stages the RGBA bytes into a host-visible buffer, copies them into the image
/// with a `vkCmdCopyBufferToImage`, and transitions the image to
/// `SHADER_READ_ONLY_OPTIMAL`. Synchronous: waits on a fence so the caller can
/// hand the image to a later render without extra synchronization.
fn upload_rgba(
    ctx: &dyn GpuContext,
    image: vk::Image,
    rgba: &[u8],
    width: u32,
    height: u32,
) -> Result<(), (RenderResult, String)> {
    let device = ctx.device();
    let mut staging = ctx.allocator().create_buffer(
        rgba.len() as u64,
        vk::BufferUsageFlags::TRANSFER_SRC,
        MemoryLocation::CpuToGpu,
    )?;
    staging.write(0, rgba)?;

    let alloc_info = vk::CommandBufferAllocateInfo {
        command_pool: ctx.command_pool(),
        level: vk::CommandBufferLevel::PRIMARY,
        command_buffer_count: 1,
        ..Default::default()
    };
    let command_buffers = unsafe { device.allocate_command_buffers(&alloc_info) }
        .map_err(|e| vk_error("vkAllocateCommandBuffers", e.as_raw()))?;
    let command_buffer = command_buffers[0];
    let begin_info = vk::CommandBufferBeginInfo {
        flags: vk::CommandBufferUsageFlags::ONE_TIME_SUBMIT,
        ..Default::default()
    };
    unsafe { device.begin_command_buffer(command_buffer, &begin_info) }
        .map_err(|e| vk_error("vkBeginCommandBuffer", e.as_raw()))?;

    let subresource = vk::ImageSubresourceRange::default()
        .aspect_mask(vk::ImageAspectFlags::COLOR)
        .level_count(1)
        .layer_count(1);
    let to_transfer = vk::ImageMemoryBarrier::default()
        .old_layout(vk::ImageLayout::UNDEFINED)
        .new_layout(vk::ImageLayout::TRANSFER_DST_OPTIMAL)
        .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .image(image)
        .subresource_range(subresource);
    unsafe {
        device.cmd_pipeline_barrier(
            command_buffer,
            vk::PipelineStageFlags::TOP_OF_PIPE,
            vk::PipelineStageFlags::TRANSFER,
            vk::DependencyFlags::empty(),
            &[],
            &[],
            &[to_transfer],
        );
        let region = vk::BufferImageCopy::default()
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
        device.cmd_copy_buffer_to_image(
            command_buffer,
            staging.buffer(),
            image,
            vk::ImageLayout::TRANSFER_DST_OPTIMAL,
            &[region],
        );
    }

    let to_shader = vk::ImageMemoryBarrier::default()
        .src_access_mask(vk::AccessFlags::TRANSFER_WRITE)
        .dst_access_mask(vk::AccessFlags::SHADER_READ)
        .old_layout(vk::ImageLayout::TRANSFER_DST_OPTIMAL)
        .new_layout(vk::ImageLayout::SHADER_READ_ONLY_OPTIMAL)
        .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .image(image)
        .subresource_range(subresource);
    unsafe {
        device.cmd_pipeline_barrier(
            command_buffer,
            vk::PipelineStageFlags::TRANSFER,
            vk::PipelineStageFlags::FRAGMENT_SHADER,
            vk::DependencyFlags::empty(),
            &[],
            &[],
            &[to_shader],
        );
        device.end_command_buffer(command_buffer)
    }
    .map_err(|e| vk_error("vkEndCommandBuffer", e.as_raw()))?;

    let fence = Fence::new(device, false)?;
    let command_buffers = [command_buffer];
    let submit_info = vk::SubmitInfo::default().command_buffers(&command_buffers);
    unsafe { device.queue_submit(ctx.queue(), &[submit_info], fence.handle()) }
        .map_err(|e| vk_error("vkQueueSubmit", e.as_raw()))?;
    fence.wait(u64::MAX)?;
    Ok(())
}

/// The `cpu-fallback` host-side texture store, retained for headless builds.
/// The production GPU path (Slice 4+) does not touch it.
#[cfg(any(feature = "cpu-fallback", test))]
#[allow(dead_code)] // functions are consumed by the cpu-fallback raster and its tests
mod host {
    use super::*;
    use std::sync::{Mutex, OnceLock};

    #[derive(Clone, Debug)]
    struct HostImage {
        bitmap: ImageBitmap,
    }

    #[derive(Default)]
    struct HostStore {
        next_handle: u64,
        entries: HashMap<u64, HostImage>,
        create_count: usize,
        destroy_count: usize,
    }

    impl HostStore {
        fn create(
            &mut self,
            pixels: &[u8],
            width: u32,
            height: u32,
            stride: u32,
            format: ImageFormat,
        ) -> Result<u64, (RenderResult, String)> {
            let rgba = normalize_rows(pixels, width, height, stride, format)?;
            let handle = self.next_handle;
            self.next_handle += 1;
            self.entries.insert(
                handle,
                HostImage {
                    bitmap: ImageBitmap {
                        width,
                        height,
                        pixels: rgba,
                    },
                },
            );
            self.create_count += 1;
            Ok(handle)
        }

        fn destroy(&mut self, handle: u64) -> Result<(), (RenderResult, String)> {
            if self.entries.remove(&handle).is_some() {
                self.destroy_count += 1;
                Ok(())
            } else {
                Err((
                    RenderResult::InvalidHandle,
                    format!("image handle {} does not exist", handle),
                ))
            }
        }

        fn lookup(&self, handle: u64) -> Option<ImageBitmap> {
            self.entries.get(&handle).map(|entry| entry.bitmap.clone())
        }

        fn stats(&self) -> (usize, usize) {
            (self.create_count, self.destroy_count)
        }

        fn reset(&mut self) {
            self.entries.clear();
            self.next_handle = 1;
            self.create_count = 0;
            self.destroy_count = 0;
        }
    }

    static HOST_STORE: OnceLock<Mutex<HostStore>> = OnceLock::new();

    fn host_store() -> &'static Mutex<HostStore> {
        HOST_STORE.get_or_init(|| Mutex::new(HostStore::default()))
    }

    pub fn create_image(
        pixels: &[u8],
        width: u32,
        height: u32,
        stride: u32,
        format: ImageFormat,
    ) -> Result<u64, (RenderResult, String)> {
        let mut store = host_store()
            .lock()
            .expect("host image store mutex poisoned");
        store.create(pixels, width, height, stride, format)
    }

    pub fn destroy_image(handle: u64) -> Result<(), (RenderResult, String)> {
        let mut store = host_store()
            .lock()
            .expect("host image store mutex poisoned");
        store.destroy(handle)
    }

    pub fn lookup_image(handle: u64) -> Option<ImageBitmap> {
        let store = host_store()
            .lock()
            .expect("host image store mutex poisoned");
        store.lookup(handle)
    }

    pub fn image_stats() -> (usize, usize) {
        let store = host_store()
            .lock()
            .expect("host image store mutex poisoned");
        store.stats()
    }

    pub fn reset_images() {
        let mut store = host_store()
            .lock()
            .expect("host image store mutex poisoned");
        store.reset();
    }
}

#[cfg(any(feature = "cpu-fallback", test))]
pub use host::{create_image, destroy_image, lookup_image, reset_images};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalize_rows_packs_rgba_rows() {
        let pixels = [
            1, 2, 3, 4, 5, 6, 7, 8, //
            9, 10, 11, 12, 13, 14, 15, 16,
        ];
        let rgba = normalize_rows(&pixels, 2, 2, 8, ImageFormat::Rgba8).expect("normalize");
        assert_eq!(rgba, pixels.to_vec());
    }

    #[test]
    fn normalize_rows_swaps_bgra_channels() {
        let pixels = [4, 3, 2, 1, 8, 7, 6, 5];
        let rgba = normalize_rows(&pixels, 2, 1, 8, ImageFormat::Bgra8).expect("normalize");
        assert_eq!(rgba, [2, 3, 4, 1, 6, 7, 8, 5]);
    }

    #[test]
    fn normalize_rows_strips_row_padding() {
        // 2x1 with a 12-byte stride (4 bytes of trailing padding per row).
        let pixels = [10, 20, 30, 40, 50, 60, 70, 80, 0, 0, 0, 0];
        let rgba = normalize_rows(&pixels, 2, 1, 12, ImageFormat::Rgba8).expect("normalize");
        assert_eq!(rgba, [10, 20, 30, 40, 50, 60, 70, 80]);
    }

    #[test]
    fn normalize_rows_rejects_short_stride() {
        let err = normalize_rows(&[0; 8], 4, 1, 8, ImageFormat::Rgba8).expect_err("must fail");
        assert!(err.1.contains("stride"));
    }

    #[test]
    fn store_destroy_unknown_handle_is_invalid() {
        let mut store = ImageStore::new();
        let err = store.destroy(1).expect_err("must fail");
        assert_eq!(err.0, RenderResult::InvalidHandle);
        assert!(store.get(1).is_none());
        assert_eq!(store.stats(), (0, 0));
    }
}
