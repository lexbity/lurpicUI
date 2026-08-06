//! Double-buffered per-frame instance buffer (NFR-4).
//!
//! One host-visible buffer per ring slot (default 2). Frame `f` writes into
//! slot `f % n`; the slot's fence (from frame `f - n`) is waited on and reset
//! at `begin_frame`, then handed to the submit so the GPU cannot race the next
//! write into the same slot. Zero per-frame allocations on the hot path: the
//! buffers are created once and grown only on demand.

use ash::vk;

use crate::gpu::allocator::{Allocator, GpuBuffer, MemoryLocation};
use crate::gpu::resources::Fence;
use crate::RenderResult;

/// Instance layout: [rect.x, rect.y, rect.w, rect.h, color.r, color.g,
/// color.b, color.a] — 8 floats, 32 bytes. Mirrors the solid pipeline's
/// instance bindings.
pub const INSTANCE_STRIDE: u64 = 32;

/// Default per-slot capacity (NFR-4: 8 MB/frame).
pub const DEFAULT_SLOT_BYTES: u64 = 8 * 1024 * 1024;

/// Default per-slot capacity for the gradient UBO ring (uniform data).
pub const DEFAULT_UNIFORM_SLOT_BYTES: u64 = 1 * 1024 * 1024;

/// Default per-slot capacity for the path vertex ring (Slice 7 winding
/// triangles).
pub const DEFAULT_PATH_SLOT_BYTES: u64 = 4 * 1024 * 1024;

/// A double-buffered host-visible vertex arena for flattened path winding
/// triangles (Slice 7). Bump-allocated at encode time; synchronization is
/// shared with the `InstanceRing` slot fence (all frame buffers submit
/// together). Mirrors `UniformRing`.
pub struct PathRing {
    buffers: Vec<GpuBuffer>,
    slot_capacity: u64,
    slot_used: Vec<u64>,
    frame_index: u64,
    frames: usize,
}

impl PathRing {
    pub fn new(
        allocator: &dyn Allocator,
        frames: usize,
        slot_capacity: u64,
    ) -> Result<Self, (RenderResult, String)> {
        let frames = frames.max(1);
        let mut buffers = Vec::with_capacity(frames);
        for _ in 0..frames {
            buffers.push(allocator.create_buffer(
                slot_capacity,
                vk::BufferUsageFlags::VERTEX_BUFFER,
                MemoryLocation::CpuToGpu,
            )?);
        }
        Ok(Self {
            buffers,
            slot_capacity,
            slot_used: vec![0; frames],
            frame_index: 0,
            frames,
        })
    }

    /// Marks the start of a frame. The instance ring's `begin_frame` (which
    /// waits the slot fence) must have run first; this shares its slot and
    /// rewinds the slot's bump cursor.
    pub fn begin_frame(&mut self) {
        let slot = (self.frame_index % self.frames as u64) as usize;
        self.slot_used[slot] = 0;
        self.frame_index += 1;
    }

    /// Bump-allocates `data` (a vec2 vertex stream, 4-byte aligned) into the
    /// current slot and returns the buffer + the first vec2 vertex index.
    pub fn append_vertices(
        &mut self,
        allocator: &dyn Allocator,
        data: &[f32],
    ) -> Result<(vk::Buffer, u32), (RenderResult, String)> {
        let slot = ((self.frame_index - 1) % self.frames as u64) as usize;
        let used = self.slot_used[slot];
        let needed = used + (data.len() as u64) * 4;
        if needed > self.slot_capacity {
            self.grow_slot(allocator, slot, needed)?;
        }
        self.buffers[slot].write(used, bytemuck_f32_bytes(data))?;
        self.slot_used[slot] = needed;
        Ok((self.buffers[slot].buffer(), (used / 8) as u32))
    }

    /// The current slot's path vertex buffer.
    pub fn current_buffer(&self) -> vk::Buffer {
        let slot = ((self.frame_index - 1) % self.frames as u64) as usize;
        self.buffers[slot].buffer()
    }

    fn grow_slot(
        &mut self,
        allocator: &dyn Allocator,
        slot: usize,
        needed: u64,
    ) -> Result<(), (RenderResult, String)> {
        let new_capacity = (needed * 2).max(self.slot_capacity * 2);
        let grown = allocator.create_buffer(
            new_capacity,
            vk::BufferUsageFlags::VERTEX_BUFFER,
            MemoryLocation::CpuToGpu,
        )?;
        let old_used = self.slot_used[slot];
        if old_used > 0 {
            let old_ptr = self.buffers[slot].mapped_ptr().ok_or((
                RenderResult::InitFailed,
                "path buffer is not host-mapped".to_string(),
            ))?;
            let new_ptr = grown.mapped_ptr().ok_or((
                RenderResult::InitFailed,
                "grown path buffer is not host-mapped".to_string(),
            ))?;
            unsafe {
                std::ptr::copy_nonoverlapping(old_ptr, new_ptr, old_used as usize);
            }
        }
        self.buffers[slot] = grown; // drops the old buffer
        self.slot_capacity = new_capacity;
        Ok(())
    }
}

/// Converts `&[f32]` to `&[u8]` (the vertex stream is 4-byte aligned so the
/// reinterpret is well-formed; no `bytemuck` dependency is pulled in).
fn bytemuck_f32_bytes(data: &[f32]) -> &[u8] {
    unsafe {
        std::slice::from_raw_parts(data.as_ptr().cast::<u8>(), data.len() * 4)
    }
}

/// A double-buffered host-visible uniform-buffer arena for per-frame UBO churn
/// (Slice 6 gradients; the per-frame linear pool of Q6). Bump-allocated at
/// encode time and reset per slot. Synchronization is shared with the
/// `InstanceRing`: the same slot fence protects both buffers (they are used by
/// the same queue submit), so `begin_frame` only advances the cursor — the
/// caller must have already waited the slot's fence via the instance ring.
pub struct UniformRing {
    buffers: Vec<GpuBuffer>,
    slot_capacity: u64,
    slot_used: Vec<u64>,
    frame_index: u64,
    frames: usize,
}

impl UniformRing {
    pub fn new(
        allocator: &dyn Allocator,
        frames: usize,
        slot_capacity: u64,
    ) -> Result<Self, (RenderResult, String)> {
        let frames = frames.max(1);
        let mut buffers = Vec::with_capacity(frames);
        for _ in 0..frames {
            buffers.push(allocator.create_buffer(
                slot_capacity,
                vk::BufferUsageFlags::UNIFORM_BUFFER,
                MemoryLocation::CpuToGpu,
            )?);
        }
        Ok(Self {
            buffers,
            slot_capacity,
            slot_used: vec![0; frames],
            frame_index: 0,
            frames,
        })
    }

    /// Marks the start of a frame. The instance ring's `begin_frame` (which
    /// waits the slot fence) must have run first; this shares its slot and
    /// rewinds the slot's bump cursor.
    pub fn begin_frame(&mut self) {
        let slot = (self.frame_index % self.frames as u64) as usize;
        self.slot_used[slot] = 0;
        self.frame_index += 1;
    }

    /// Bump-allocates `data` into the current slot's uniform buffer, aligned to
    /// `alignment` (minUniformBufferOffsetAlignment; the UBO struct is 16-byte
    /// aligned). Returns the buffer and the byte offset the descriptor binds.
    pub fn write(
        &mut self,
        allocator: &dyn Allocator,
        data: &[u8],
        alignment: u64,
    ) -> Result<(vk::Buffer, u64), (RenderResult, String)> {
        let alignment = alignment.max(1);
        let slot = ((self.frame_index - 1) % self.frames as u64) as usize;
        let aligned = self.slot_used[slot].div_ceil(alignment) * alignment;
        let end = aligned
            .checked_add(data.len() as u64)
            .ok_or((RenderResult::OutOfMemory, "uniform ring write overflow".to_string()))?;
        if end > self.slot_capacity {
            self.grow_slot(allocator, slot, end)?;
        }
        self.buffers[slot].write(aligned, data)?;
        self.slot_used[slot] = end;
        Ok((self.buffers[slot].buffer(), aligned))
    }

    fn grow_slot(
        &mut self,
        allocator: &dyn Allocator,
        slot: usize,
        needed: u64,
    ) -> Result<(), (RenderResult, String)> {
        let new_capacity = (needed * 2).max(self.slot_capacity * 2);
        let grown = allocator.create_buffer(
            new_capacity,
            vk::BufferUsageFlags::UNIFORM_BUFFER,
            MemoryLocation::CpuToGpu,
        )?;
        let old_used = self.slot_used[slot];
        if old_used > 0 {
            let old_ptr = self.buffers[slot].mapped_ptr().ok_or((
                RenderResult::InitFailed,
                "uniform buffer is not host-mapped".to_string(),
            ))?;
            let new_ptr = grown.mapped_ptr().ok_or((
                RenderResult::InitFailed,
                "grown uniform buffer is not host-mapped".to_string(),
            ))?;
            unsafe {
                std::ptr::copy_nonoverlapping(old_ptr, new_ptr, old_used as usize);
            }
        }
        self.buffers[slot] = grown; // drops the old buffer
        self.slot_capacity = new_capacity;
        Ok(())
    }
}

pub struct InstanceRing {
    buffers: Vec<GpuBuffer>,
    fences: Vec<Fence>,
    slot_capacity: u64,
    slot_used: Vec<u64>,
    frame_index: u64,
    frames: usize,
}

impl InstanceRing {
    pub fn new(
        allocator: &dyn Allocator,
        device: &ash::Device,
        frames: usize,
        slot_capacity: u64,
    ) -> Result<Self, (RenderResult, String)> {
        let frames = frames.max(1);
        let mut buffers = Vec::with_capacity(frames);
        let mut fences = Vec::with_capacity(frames);
        for _ in 0..frames {
            buffers.push(allocator.create_buffer(
                slot_capacity,
                vk::BufferUsageFlags::VERTEX_BUFFER,
                MemoryLocation::CpuToGpu,
            )?);
            // Signaled so the first begin_frame does not block.
            fences.push(Fence::new(device, true)?);
        }
        Ok(Self {
            buffers,
            fences,
            slot_capacity,
            slot_used: vec![0; frames],
            frame_index: 0,
            frames,
        })
    }

    /// Marks the start of a frame: waits for the oldest in-flight slot and
    /// resets its fence for the upcoming submit.
    pub fn begin_frame(&mut self) -> Result<(), (RenderResult, String)> {
        let slot = (self.frame_index % self.frames as u64) as usize;
        self.fences[slot].wait(u64::MAX)?;
        self.fences[slot].reset()?;
        self.slot_used[slot] = 0;
        self.frame_index += 1;
        Ok(())
    }

    /// Appends instance records and returns (buffer, first_instance,
    /// instance_count). Grows the slot when the capacity is exhausted.
    pub fn append(
        &mut self,
        allocator: &dyn Allocator,
        instances: &[u8],
    ) -> Result<(vk::Buffer, u32, u32), (RenderResult, String)> {
        if instances.is_empty() {
            return Err((
                RenderResult::InitFailed,
                "cannot append an empty instance batch".to_string(),
            ));
        }
        if instances.len() as u64 % INSTANCE_STRIDE != 0 {
            return Err((
                RenderResult::InitFailed,
                format!(
                    "instance data {} bytes is not a multiple of the {} byte stride",
                    instances.len(),
                    INSTANCE_STRIDE
                ),
            ));
        }
        let slot = ((self.frame_index - 1) % self.frames as u64) as usize;
        let used = self.slot_used[slot];
        let needed = used + instances.len() as u64;
        if needed > self.slot_capacity {
            self.grow_slot(allocator, slot, needed)?;
        }
        self.buffers[slot].write(used, instances)?;
        let first_instance = (used / INSTANCE_STRIDE) as u32;
        let instance_count = (instances.len() as u64 / INSTANCE_STRIDE) as u32;
        self.slot_used[slot] = needed;
        Ok((self.buffers[slot].buffer(), first_instance, instance_count))
    }

    /// The fence to attach to the current frame's queue submit.
    pub fn take_fence(&self) -> vk::Fence {
        let slot = ((self.frame_index - 1) % self.frames as u64) as usize;
        self.fences[slot].handle()
    }

    /// The ring slot of the current frame (0..frames-1), used to index
    /// per-slot resources (e.g. descriptor pools in Slice 4).
    pub fn current_slot(&self) -> usize {
        ((self.frame_index - 1) % self.frames as u64) as usize
    }

    /// The instance buffer backing the current frame.
    pub fn current_buffer(&self) -> vk::Buffer {
        let slot = ((self.frame_index - 1) % self.frames as u64) as usize;
        self.buffers[slot].buffer()
    }

    fn grow_slot(
        &mut self,
        allocator: &dyn Allocator,
        slot: usize,
        needed: u64,
    ) -> Result<(), (RenderResult, String)> {
        let new_capacity = (needed * 2).max(self.slot_capacity * 2);
        let grown = allocator.create_buffer(
            new_capacity,
            vk::BufferUsageFlags::VERTEX_BUFFER,
            MemoryLocation::CpuToGpu,
        )?;
        // Copy the previously written region into the grown buffer.
        let old_used = self.slot_used[slot];
        if old_used > 0 {
            let old_ptr = self.buffers[slot].mapped_ptr().ok_or((
                RenderResult::InitFailed,
                "instance buffer is not host-mapped".to_string(),
            ))?;
            let new_ptr = grown.mapped_ptr().ok_or((
                RenderResult::InitFailed,
                "grown instance buffer is not host-mapped".to_string(),
            ))?;
            unsafe {
                std::ptr::copy_nonoverlapping(old_ptr, new_ptr, old_used as usize);
            }
        }
        self.buffers[slot] = grown; // drops the old buffer
        self.slot_capacity = new_capacity;
        Ok(())
    }
}

/// Ring state must be single-threaded (render thread); the existing VulkanState
/// is behind a mutex.
unsafe impl Send for InstanceRing {}
