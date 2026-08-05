//! Memory allocation (Q6/FR-16).
//!
//! Two tiers behind an `Allocator` trait:
//! - `GpuAllocator` wraps `gpu-allocator` for long-lived resources (textures,
//!   pipelines' backing memory, the staging buffer).
//! - `LinearPool` bump-allocates from a single host-visible buffer for
//!   per-frame transient churn (vertex/index/uniform data in Slice 3); it
//!   resets at fence signal and is unit-tested here.
//!
//! `GpuBuffer` is self-freeing (RAII): its `Drop` returns the allocation to
//! the allocator and destroys the buffer, so a buffer cannot leak on the
//! device-lost path.

use std::cell::RefCell;
use std::rc::Rc;

use ash::vk;

use crate::error::vk_error;
use crate::RenderResult;

/// Memory placement class mirroring `gpu_allocator::vulkan::MemoryLocation`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
#[allow(dead_code)] // GpuOnly/GpuToCpu consumed by the texture pipeline (Slice 4)
pub enum MemoryLocation {
    GpuOnly,
    CpuToGpu,
    GpuToCpu,
}

/// Allocation trait (FR-16). Implementations back long-lived resources through
/// the GPU allocator and transient per-frame churn through the linear pool.
#[allow(dead_code)] // image/texture + transient paths consumed by Slices 3-4
pub trait Allocator {
    fn create_buffer(
        &self,
        size: u64,
        usage: vk::BufferUsageFlags,
        location: MemoryLocation,
    ) -> Result<GpuBuffer, (RenderResult, String)>;
    fn allocate_image_memory(
        &self,
        image: vk::Image,
        requirements: vk::MemoryRequirements,
        location: MemoryLocation,
    ) -> Result<ImageAllocation, (RenderResult, String)>;
    fn free(&self, allocation: ImageAllocation);
    /// Resets per-frame transient state (called at fence signal).
    fn reset_frame(&self, frame: u64);
}

struct Inner {
    gpu: gpu_allocator::vulkan::Allocator,
    device: ash::Device,
}

/// Backing implementation of [`Allocator`] composed of `gpu-allocator` and a
/// per-frame [`LinearPool`].
pub struct GpuAllocator {
    inner: Rc<RefCell<Inner>>,
    linear_pool: RefCell<LinearPool>,
}

impl Clone for GpuAllocator {
    fn clone(&self) -> Self {
        Self {
            inner: Rc::clone(&self.inner),
            linear_pool: RefCell::new(self.linear_pool.borrow().clone()),
        }
    }
}

impl GpuAllocator {
    pub fn new(
        device: &ash::Device,
        instance: &ash::Instance,
        physical_device: vk::PhysicalDevice,
    ) -> Result<Self, (RenderResult, String)> {
        let desc = gpu_allocator::vulkan::AllocatorCreateDesc {
            instance: instance.clone(),
            device: device.clone(),
            physical_device,
            debug_settings: Default::default(),
            buffer_device_address: false,
            allocation_sizes: Default::default(),
        };
        let gpu = gpu_allocator::vulkan::Allocator::new(&desc)
            .map_err(|e| (RenderResult::InitFailed, format!("gpu-allocator init: {}", e)))?;
        Ok(Self {
            inner: Rc::new(RefCell::new(Inner {
                gpu,
                device: device.clone(),
            })),
            linear_pool: RefCell::new(LinearPool::default()),
        })
    }

    fn to_gpu_location(location: MemoryLocation) -> gpu_allocator::MemoryLocation {
        match location {
            MemoryLocation::GpuOnly => gpu_allocator::MemoryLocation::GpuOnly,
            MemoryLocation::CpuToGpu => gpu_allocator::MemoryLocation::CpuToGpu,
            MemoryLocation::GpuToCpu => gpu_allocator::MemoryLocation::GpuToCpu,
        }
    }
}

impl Allocator for GpuAllocator {
    fn create_buffer(
        &self,
        size: u64,
        usage: vk::BufferUsageFlags,
        location: MemoryLocation,
    ) -> Result<GpuBuffer, (RenderResult, String)> {
        if size == 0 {
            return Err((
                RenderResult::InitFailed,
                "buffer size is zero".to_string(),
            ));
        }
        let mut inner = self.inner.borrow_mut();
        let create_info = vk::BufferCreateInfo {
            size,
            usage,
            sharing_mode: vk::SharingMode::EXCLUSIVE,
            ..Default::default()
        };
        let buffer = unsafe { inner.device.create_buffer(&create_info, None) }
            .map_err(|e| vk_error("vkCreateBuffer", e.as_raw()))?;
        let requirements = unsafe { inner.device.get_buffer_memory_requirements(buffer) };
        let desc = gpu_allocator::vulkan::AllocationCreateDesc {
            name: "lurpic_buffer",
            requirements,
            location: Self::to_gpu_location(location),
            linear: true,
            allocation_scheme: gpu_allocator::vulkan::AllocationScheme::DedicatedBuffer(buffer),
        };
        let allocation = match inner.gpu.allocate(&desc) {
            Ok(a) => a,
            Err(e) => {
                unsafe {
                    inner.device.destroy_buffer(buffer, None);
                }
                return Err((
                    RenderResult::OutOfMemory,
                    format!("allocate buffer memory: {}", e),
                ));
            }
        };
        let memory = unsafe { allocation.memory() };
        if let Err(e) = unsafe { inner.device.bind_buffer_memory(buffer, memory, 0) } {
            let _ = inner.gpu.free(allocation);
            unsafe {
                inner.device.destroy_buffer(buffer, None);
            }
            return Err(vk_error("vkBindBufferMemory", e.as_raw()));
        }
        Ok(GpuBuffer {
            allocator: self.clone(),
            buffer,
            allocation: Some(allocation),
            size,
        })
    }

    fn allocate_image_memory(
        &self,
        image: vk::Image,
        requirements: vk::MemoryRequirements,
        location: MemoryLocation,
    ) -> Result<ImageAllocation, (RenderResult, String)> {
        let mut inner = self.inner.borrow_mut();
        let desc = gpu_allocator::vulkan::AllocationCreateDesc {
            name: "lurpic_image",
            requirements,
            location: Self::to_gpu_location(location),
            linear: false,
            allocation_scheme: gpu_allocator::vulkan::AllocationScheme::DedicatedImage(image),
        };
        match inner.gpu.allocate(&desc) {
            Ok(allocation) => Ok(ImageAllocation {
                allocator: self.clone(),
                allocation: Some(allocation),
            }),
            Err(e) => Err((
                RenderResult::OutOfMemory,
                format!("allocate image memory: {}", e),
            )),
        }
    }

    fn free(&self, allocation: ImageAllocation) {
        drop(allocation);
    }

    fn reset_frame(&self, frame: u64) {
        self.linear_pool.borrow_mut().reset(frame);
    }
}

/// RAII buffer: destroys the `vk::Buffer` and returns its memory on drop.
pub struct GpuBuffer {
    allocator: GpuAllocator,
    buffer: vk::Buffer,
    allocation: Option<gpu_allocator::vulkan::Allocation>,
    size: u64,
}

impl GpuBuffer {
    pub fn buffer(&self) -> vk::Buffer {
        self.buffer
    }

    pub fn size(&self) -> u64 {
        self.size
    }

    pub fn mapped_ptr(&self) -> Option<*mut u8> {
        self.allocation
            .as_ref()
            .and_then(|a| a.mapped_ptr())
            .map(|p| p.as_ptr().cast::<u8>())
    }

    /// Copies bytes into a host-visible buffer at the given offset.
    pub fn write(&mut self, offset: u64, data: &[u8]) -> Result<(), (RenderResult, String)> {
        let ptr = self.mapped_ptr().ok_or_else(|| {
            (
                RenderResult::InitFailed,
                "buffer memory is not host-mapped".to_string(),
            )
        })?;
        let end = (offset as usize)
            .checked_add(data.len())
            .ok_or((RenderResult::OutOfMemory, "buffer write overflow".to_string()))?;
        if end > self.size as usize {
            return Err((
                RenderResult::OutOfMemory,
                format!(
                    "buffer write of {} bytes at {} exceeds size {}",
                    data.len(),
                    offset,
                    self.size
                ),
            ));
        }
        unsafe {
            std::ptr::copy_nonoverlapping(data.as_ptr(), ptr.add(offset as usize), data.len());
        }
        Ok(())
    }
}

impl Drop for GpuBuffer {
    fn drop(&mut self) {
        let mut inner = self.allocator.inner.borrow_mut();
        if let Some(allocation) = self.allocation.take() {
            let _ = inner.gpu.free(allocation);
        }
        unsafe {
            inner.device.destroy_buffer(self.buffer, None);
        }
    }
}

/// RAII image memory: returns the allocation to the allocator on drop.
#[allow(dead_code)] // consumed by the texture pipeline (Slice 4)
pub struct ImageAllocation {
    allocator: GpuAllocator,
    allocation: Option<gpu_allocator::vulkan::Allocation>,
}

#[allow(dead_code)]
impl ImageAllocation {
    pub fn memory(&self) -> vk::DeviceMemory {
        let allocation = self.allocation.as_ref().expect("image allocation alive");
        unsafe { allocation.memory() }
    }
}

impl Drop for ImageAllocation {
    fn drop(&mut self) {
        let mut inner = self.allocator.inner.borrow_mut();
        if let Some(allocation) = self.allocation.take() {
            let _ = inner.gpu.free(allocation);
        }
    }
}

/// A host-visible bump pool for per-frame transient buffers.
///
/// The pool owns one large `VkBuffer` and a mapped pointer. `allocate` bump
/// allocates a contiguous region; `reset(frame)` marks a frame's region free
/// once the fence for that frame has signaled. Regions older than the ring
/// depth are implicitly reusable.
/// The per-frame linear pool is consumed by the frame encoder in Slice 3; its
/// unit tests exercise the bump/ring logic now.
#[derive(Clone)]
#[allow(dead_code)]
pub struct LinearPool {
    capacity: u64,
    frame_depth: u32,
    cursor: u64,
    frames: Vec<(u64, u64)>, // (frame_index, cursor_at_start)
}

impl Default for LinearPool {
    fn default() -> Self {
        Self {
            capacity: 0,
            frame_depth: 2,
            cursor: 0,
            frames: Vec::new(),
        }
    }
}

#[allow(dead_code)] // consumed by the frame encoder (Slice 3); unit-tested now
impl LinearPool {
    pub fn with_capacity(capacity: u64, frame_depth: u32) -> Self {
        Self {
            capacity: capacity.max(1),
            frame_depth: frame_depth.max(1),
            cursor: 0,
            frames: Vec::new(),
        }
    }

    /// Bump-allocates `size` bytes, aligned to `alignment`. Returns the byte
    /// offset into the pool, or None when the pool is exhausted (caller grows
    /// the backing buffer or recycles).
    pub fn allocate(&mut self, size: u64, alignment: u64) -> Option<u64> {
        let alignment = alignment.max(1);
        let aligned = self.cursor.div_ceil(alignment) * alignment;
        let end = aligned.checked_add(size)?;
        if end > self.capacity {
            return None;
        }
        self.cursor = end;
        Some(aligned)
    }

    /// Records the start offset of a frame's first allocation so `reset` can
    /// rewind to it once the frame retires.
    pub fn mark_frame(&mut self, frame: u64, start: u64) {
        self.frames.push((frame, start));
    }

    /// Rewinds the cursor to the start of the oldest retired frame's region,
    /// making it reusable. Frames older than the ring depth are retired (FIFO).
    pub fn reset(&mut self, frame: u64) {
        let mut retired_start: Option<u64> = None;
        while let Some(&(f, start)) = self.frames.first() {
            if frame.saturating_sub(f) >= self.frame_depth as u64 {
                retired_start = Some(start);
                self.frames.remove(0);
            } else {
                break;
            }
        }
        if let Some(start) = retired_start {
            self.cursor = start;
        }
        if self.frames.is_empty() {
            self.cursor = 0;
        }
    }

    pub fn cursor(&self) -> u64 {
        self.cursor
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bump_allocates_with_alignment() {
        let mut pool = LinearPool::with_capacity(4096, 2);
        let off0 = pool.allocate(100, 16).unwrap();
        assert_eq!(off0 % 16, 0);
        let off1 = pool.allocate(100, 16).unwrap();
        assert!(off1 >= off0 + 100);
        assert_eq!(off1 % 16, 0);
    }

    #[test]
    fn exhausts_with_none() {
        let mut pool = LinearPool::with_capacity(64, 2);
        assert!(pool.allocate(64, 1).is_some());
        assert!(pool.allocate(1, 1).is_none());
    }

    #[test]
    fn reset_recycles_after_ring_depth() {
        let mut pool = LinearPool::with_capacity(4096, 2);
        let f0 = pool.allocate(1000, 1).unwrap();
        pool.mark_frame(0, f0);
        let f1 = pool.allocate(1000, 1).unwrap();
        pool.mark_frame(1, f1);
        assert!(f1 >= f0 + 1000);
        // Frame 2 retires frame 0's region, rewinding the cursor to it.
        pool.reset(2);
        let f2 = pool.allocate(1000, 1).unwrap();
        assert_eq!(f2, f0, "frame 0 region must be recycled");
        pool.mark_frame(2, f2);
        // Frame 3 retires frame 1's region.
        pool.reset(3);
        let f3 = pool.allocate(1000, 1).unwrap();
        assert_eq!(f3, f1, "frame 1 region must be recycled");
    }

    #[test]
    fn alignment_zero_treated_as_one() {
        let mut pool = LinearPool::with_capacity(100, 2);
        let off = pool.allocate(10, 0).unwrap();
        assert_eq!(off, 0);
    }
}
