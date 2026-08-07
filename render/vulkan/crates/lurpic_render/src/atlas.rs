//! Packed glyph atlas (Slice 5).
//!
//! The GPU atlas packs glyph masks into a single `R8G8_UNORM` texture via a
//! skyline rect-packer: the R channel holds the coverage mask (bitmap glyphs),
//! the G channel holds the signed-distance field (SDF glyphs at size >= 24 px).
//! The atlas is owned by `VulkanState` so its `VkImage` is RAII-released with
//! the device; it is LRU-managed (evictions free atlas space) and grows by
//! doubling with a full re-upload of the live entries. Each `GlyphEntry`
//! retains its mask/SDF bytes so growth can re-upload — the atlas is a packed
//! texture, not a per-glyph bitmap store, but the source bytes are needed for
//! atlas maintenance (documented in the Slice 4 Vec<u8> note).
//!

use std::collections::{HashMap, VecDeque};

use ash::vk;

use crate::error::vk_error;
use crate::gpu::allocator::{ImageAllocation, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{Fence, Image, ImageView, Sampler};
use crate::RenderResult;

/// Glyphs at or above this size (pixels) render from the SDF channel; smaller
/// glyphs render from the coverage-mask channel (the existing size heuristic).
pub const SDF_MIN_SIZE: f32 = 24.0;

/// Initial and maximum atlas texture dimensions.
pub const INITIAL_ATLAS_SIZE: u32 = 1024;
pub const MAX_ATLAS_SIZE: u32 = 4096;

/// Which channel a glyph renders from.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum GlyphMode {
    Bitmap,
    Sdf,
}

pub fn mode_for_size(size: f32) -> GlyphMode {
    if size >= SDF_MIN_SIZE {
        GlyphMode::Sdf
    } else {
        GlyphMode::Bitmap
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct GlyphKey {
    pub font_id: u64,
    pub glyph_id: u32,
    pub size_bits: u32,
}

/// A rectangle of the atlas texture, in atlas pixels.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct AtlasRegion {
    pub x: i32,
    pub y: i32,
    pub w: u32,
    pub h: u32,
}

/// A glyph placed in the GPU atlas. The mask/SDF bytes are retained so the
/// atlas can re-upload them when it grows or compacts.
pub struct GlyphEntry {
    pub region: AtlasRegion,
    pub width: u32,
    pub height: u32,
    pub offset_x: f32,
    pub offset_y: f32,
    pub advance: f32,
    pub mode: GlyphMode,
    mask: Vec<u8>,
    sdf: Option<Vec<u8>>,
}

/// A skyline rect-packer: the texture's occupied top edge as a list of
/// horizontal segments. `pack` finds the lowest-y span that fits a w x h block.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
struct Segment {
    x: i32,
    y: i32,
    w: i32,
}

#[derive(Clone, Debug)]
struct Skyline {
    segments: Vec<Segment>,
    height: i32,
}

impl Skyline {
    fn new(width: u32, height: u32) -> Self {
        Self {
            segments: vec![Segment {
                x: 0,
                y: 0,
                w: width as i32,
            }],
            height: height as i32,
        }
    }

    fn reset(&mut self, width: u32, height: u32) {
        *self = Self::new(width, height);
    }

    /// The lowest-y position at which a w x h block fits, returning the start
    /// segment index. Ties go to the leftmost x. O(n^2); the atlas holds only
    /// O(100s) of segments, so this is not a hot path.
    fn find_best(&self, w: u32, h: u32) -> Option<(i32, i32, usize)> {
        let w = w as i32;
        let h = h as i32;
        let mut best: Option<(i32, i32, usize)> = None;
        for i in 0..self.segments.len() {
            let x = self.segments[i].x;
            let x_end = x + w;
            let mut y = 0i32;
            let mut covered_x = x;
            for seg in &self.segments[i..] {
                if covered_x >= x_end {
                    break;
                }
                y = y.max(seg.y);
                if y + h > self.height {
                    break;
                }
                covered_x += seg.w;
            }
            if covered_x >= x_end
                && y + h <= self.height
                && best.is_none_or(|(bx, by, _)| y < by || (y == by && x < bx))
            {
                best = Some((x, y, i));
            }
        }
        best
    }

    /// Places a w x h block whose start segment is `index`, updating the
    /// skyline top edge. Returns the block's (x, y).
    fn place(&mut self, w: u32, h: u32, index: usize) -> (i32, i32) {
        let w = w as i32;
        let h = h as i32;
        let x = self.segments[index].x;
        let x_end = x + w;
        let mut y = 0i32;
        let mut end = index;
        let mut covered_x = x;
        while end < self.segments.len() && covered_x < x_end {
            y = y.max(self.segments[end].y);
            covered_x += self.segments[end].w;
            end += 1;
        }
        // If the last covered segment overshoots x_end, split it.
        if covered_x > x_end {
            let last = self.segments[end - 1];
            let overflow = covered_x - x_end;
            self.segments[end - 1] = Segment {
                x: last.x,
                y: last.y,
                w: last.w - overflow,
            };
            self.segments.insert(
                end,
                Segment {
                    x: x_end,
                    y: last.y,
                    w: overflow,
                },
            );
        }
        // Remove the covered span [index, end) and insert the new top edge.
        let _ = self.segments.drain(index..end);
        self.segments.insert(
            index,
            Segment {
                x,
                y: y + h,
                w,
            },
        );
        // Merge with neighbors at the same height.
        let mut i = index;
        loop {
            let merged_right = if i + 1 < self.segments.len() {
                let (a, b) = (self.segments[i], self.segments[i + 1]);
                if a.y == b.y && a.x + a.w == b.x {
                    self.segments[i] = Segment {
                        x: a.x,
                        y: a.y,
                        w: a.w + b.w,
                    };
                    self.segments.remove(i + 1);
                    true
                } else {
                    false
                }
            } else {
                false
            };
            let merged_left = if !merged_right && i > 0 {
                let (a, b) = (self.segments[i - 1], self.segments[i]);
                if a.y == b.y && a.x + a.w == b.x {
                    self.segments[i - 1] = Segment {
                        x: a.x,
                        y: a.y,
                        w: a.w + b.w,
                    };
                    self.segments.remove(i);
                    true
                } else {
                    false
                }
            } else {
                false
            };
            if merged_left {
                i -= 1;
            }
            if !merged_right && !merged_left {
                break;
            }
        }
        (x, y)
    }
}

/// The GPU glyph atlas: a packed R8G8 texture + skyline packer + LRU.
pub struct GlyphAtlas {
    image: Option<Image>,
    view: Option<ImageView>,
    sampler: Option<Sampler>,
    allocation: Option<ImageAllocation>,
    skyline: Skyline,
    size: u32,
    entries: HashMap<GlyphKey, GlyphEntry>,
    lru: VecDeque<GlyphKey>,
    evictions: usize,
}

impl GlyphAtlas {
    pub fn new() -> Self {
        Self {
            image: None,
            view: None,
            sampler: None,
            allocation: None,
            skyline: Skyline::new(INITIAL_ATLAS_SIZE, INITIAL_ATLAS_SIZE),
            size: INITIAL_ATLAS_SIZE,
            entries: HashMap::new(),
            lru: VecDeque::new(),
            evictions: 0,
        }
    }

    /// The atlas image view (present once the first glyph has been uploaded).
    pub fn image_view(&self) -> Option<vk::ImageView> {
        self.view.as_ref().map(|v| v.handle())
    }

    /// The atlas sampler (nearest + clamp-to-edge; glyphs blit 1:1).
    pub fn sampler(&self) -> Option<vk::Sampler> {
        self.sampler.as_ref().map(|s| s.handle())
    }

    pub fn stats(&self) -> (usize, usize) {
        (self.entries.len(), self.evictions)
    }

    pub fn reset(&mut self) {
        self.image = None;
        self.view = None;
        self.sampler = None;
        self.allocation = None;
        self.skyline = Skyline::new(INITIAL_ATLAS_SIZE, INITIAL_ATLAS_SIZE);
        self.size = INITIAL_ATLAS_SIZE;
        self.entries.clear();
        self.lru.clear();
        self.evictions = 0;
    }

    /// The glyph entry for a (font, glyph, size), if it is resident.
    pub fn get(&self, font_id: u64, glyph_id: u32, size_bits: u32) -> Option<&GlyphEntry> {
        self.entries.get(&GlyphKey {
            font_id,
            glyph_id,
            size_bits,
        })
    }

    /// Uploads a glyph's coverage mask into the atlas, generating its SDF when
    /// the size warrants it. Idempotent per (font, glyph, size).
    #[allow(clippy::too_many_arguments)] // upload carries the full glyph payload + atlas state
    pub fn upload(
        &mut self,
        ctx: &dyn GpuContext,
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
        if width == 0 || height == 0 {
            return Err((
                RenderResult::InitFailed,
                "glyph dimensions are zero".to_string(),
            ));
        }
        let key = GlyphKey {
            font_id,
            glyph_id,
            size_bits,
        };
        if self.entries.contains_key(&key) {
            self.touch(&key);
            return Ok(());
        }
        if mask.len() < (width as usize) * (height as usize) {
            return Err((
                RenderResult::InitFailed,
                "glyph bitmap is truncated".to_string(),
            ));
        }
        if self.image.is_none() {
            self.create_image(ctx, self.size)?;
        }
        let mode = mode_for_size(f32::from_bits(size_bits));
        let sdf = match mode {
            GlyphMode::Sdf => Some(sdf_from_mask(mask, width, height)),
            GlyphMode::Bitmap => None,
        };
        self.evict_to_capacity();
        let region = self.ensure_fit(ctx, width, height)?;
        self.upload_region(ctx, region, mask, sdf.as_deref())?;
        self.entries.insert(
            key.clone(),
            GlyphEntry {
                region,
                width,
                height,
                offset_x,
                offset_y,
                advance,
                mode,
                mask: mask[..(width as usize) * (height as usize)].to_vec(),
                sdf,
            },
        );
        self.lru.push_back(key);
        Ok(())
    }

    /// Evicts least-recently-used glyphs until the LRU capacity is satisfied.
    fn evict_to_capacity(&mut self) {
        const CAPACITY: usize = 4096;
        while self.entries.len() >= CAPACITY {
            if let Some(oldest) = self.lru.pop_front() {
                if self.entries.remove(&oldest).is_some() {
                    self.evictions += 1;
                }
            } else {
                break;
            }
        }
    }

    fn touch(&mut self, key: &GlyphKey) {
        if let Some(pos) = self.lru.iter().position(|k| k == key) {
            self.lru.remove(pos);
        }
        self.lru.push_back(key.clone());
    }

    fn create_image(&mut self, ctx: &dyn GpuContext, size: u32) -> Result<(), (RenderResult, String)> {
        let device = ctx.device();
        let image_info = vk::ImageCreateInfo {
            image_type: vk::ImageType::TYPE_2D,
            format: vk::Format::R8G8_UNORM,
            extent: vk::Extent3D {
                width: size,
                height: size,
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
        unsafe {
            device.bind_image_memory(image.handle(), allocation.memory(), 0)
        }
        .map_err(|e| vk_error("vkBindImageMemory", e.as_raw()))?;
        let view = ImageView::new(device, image.handle(), vk::Format::R8G8_UNORM, vk::ImageAspectFlags::COLOR)?;
        // LINEAR filtering: the SDF shader samples the distance field between
        // texels so the smoothstep reconstruction anti-aliases. The bitmap
        // shader snaps to exact texel centers, where a linear filter returns
        // the texel itself.
        let sampler = Sampler::new(
            device,
            &vk::SamplerCreateInfo {
                mag_filter: vk::Filter::LINEAR,
                min_filter: vk::Filter::LINEAR,
                mipmap_mode: vk::SamplerMipmapMode::NEAREST,
                address_mode_u: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_v: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                address_mode_w: vk::SamplerAddressMode::CLAMP_TO_EDGE,
                ..Default::default()
            },
        )?;
        self.image = Some(image);
        self.view = Some(view);
        self.sampler = Some(sampler);
        self.allocation = Some(allocation);
        self.size = size;
        self.skyline.reset(size, size);
        Ok(())
    }

    /// Ensures a w x h block fits: pack, then recover dead space (reflow),
    /// then grow the texture, re-packing the live entries.
    fn ensure_fit(
        &mut self,
        ctx: &dyn GpuContext,
        w: u32,
        h: u32,
    ) -> Result<AtlasRegion, (RenderResult, String)> {
        if w > MAX_ATLAS_SIZE || h > MAX_ATLAS_SIZE {
            return Err((
                RenderResult::InitFailed,
                format!("glyph {}x{} exceeds the {}px atlas", w, h, MAX_ATLAS_SIZE),
            ));
        }
        if let Some((x, y, index)) = self.skyline.find_best(w, h) {
            return Ok(self.place_at(w, h, x, y, index));
        }
        // Compaction: re-pack the live entries (recovering evicted dead space).
        self.reflow(ctx)?;
        if let Some((x, y, index)) = self.skyline.find_best(w, h) {
            return Ok(self.place_at(w, h, x, y, index));
        }
        // Growth: double the texture, then re-pack.
        let new_size = (self.size * 2).min(MAX_ATLAS_SIZE);
        if new_size <= self.size {
            return Err((
                RenderResult::InitFailed,
                "glyph atlas is at maximum size and cannot fit the glyph".to_string(),
            ));
        }
        self.grow(ctx, new_size)?;
        self.skyline
            .find_best(w, h)
            .map(|(x, y, index)| self.place_at(w, h, x, y, index))
            .ok_or((
                RenderResult::InitFailed,
                "glyph too large for the atlas after growth".to_string(),
            ))
    }

    fn place_at(&mut self, w: u32, h: u32, x: i32, y: i32, index: usize) -> AtlasRegion {
        let (px, py) = self.skyline.place(w, h, index);
        debug_assert_eq!((px, py), (x, y));
        AtlasRegion {
            x: px,
            y: py,
            w,
            h,
        }
    }

    /// Re-packs all live entries into a fresh skyline at the current size,
    /// re-uploading each (recovers space freed by evicted glyphs).
    fn reflow(&mut self, ctx: &dyn GpuContext) -> Result<(), (RenderResult, String)> {
        let live: Vec<(GlyphKey, GlyphEntry)> = std::mem::take(&mut self.entries)
            .into_iter()
            .collect();
        self.lru.clear();
        self.skyline.reset(self.size, self.size);
        for (key, entry) in live {
            self.reupload(ctx, &key, &entry)?;
        }
        Ok(())
    }

    /// Doubles the atlas texture and re-packs + re-uploads all live entries.
    fn grow(&mut self, ctx: &dyn GpuContext, new_size: u32) -> Result<(), (RenderResult, String)> {
        let live: Vec<(GlyphKey, GlyphEntry)> = std::mem::take(&mut self.entries)
            .into_iter()
            .collect();
        self.lru.clear();
        // Recreate the image at the larger size (drops the old view; the frame
        // renderer captures the current view after all uploads for the frame).
        self.image = None;
        self.view = None;
        self.allocation = None;
        self.create_image(ctx, new_size)?;
        for (key, entry) in live {
            self.reupload(ctx, &key, &entry)?;
        }
        Ok(())
    }

    /// Packs and uploads one live entry at its retained data (no LRU/eviction
    /// bookkeeping — used by reflow/grow; the LRU is rebuilt by callers).
    fn reupload(
        &mut self,
        ctx: &dyn GpuContext,
        key: &GlyphKey,
        entry: &GlyphEntry,
    ) -> Result<(), (RenderResult, String)> {
        let w = entry.width;
        let h = entry.height;
        let region = match self.skyline.find_best(w, h) {
            Some((x, y, index)) => self.place_at(w, h, x, y, index),
            None => {
                return Err((
                    RenderResult::OutOfMemory,
                    "atlas reflow could not fit a live glyph".to_string(),
                ))
            }
        };
        self.upload_region(ctx, region, &entry.mask, entry.sdf.as_deref())?;
        let reflowed = GlyphEntry {
            region,
            width: entry.width,
            height: entry.height,
            offset_x: entry.offset_x,
            offset_y: entry.offset_y,
            advance: entry.advance,
            mode: entry.mode,
            mask: entry.mask.clone(),
            sdf: entry.sdf.clone(),
        };
        self.entries.insert(key.clone(), reflowed);
        self.lru.push_back(key.clone());
        Ok(())
    }

    /// Stages the interleaved RG bytes (mask in R, SDF in G) and copies them
    /// into the atlas at `region`, then restores SHADER_READ_ONLY_OPTIMAL.
    fn upload_region(
        &mut self,
        ctx: &dyn GpuContext,
        region: AtlasRegion,
        mask: &[u8],
        sdf: Option<&[u8]>,
    ) -> Result<(), (RenderResult, String)> {
        let image = self.image.as_ref().expect("atlas image exists").handle();
        let count = (region.w as usize) * (region.h as usize);
        let mut rg = vec![0u8; count * 2];
        for i in 0..count {
            rg[i * 2] = mask[i];
            rg[i * 2 + 1] = sdf.map_or(0, |s| s[i]);
        }
        let device = ctx.device();
        let mut staging = ctx.allocator().create_buffer(
            rg.len() as u64,
            vk::BufferUsageFlags::TRANSFER_SRC,
            MemoryLocation::CpuToGpu,
        )?;
        staging.write(0, &rg)?;

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
            let region_copy = vk::BufferImageCopy::default()
                .image_subresource(
                    vk::ImageSubresourceLayers::default()
                        .aspect_mask(vk::ImageAspectFlags::COLOR)
                        .layer_count(1),
                )
                .image_offset(vk::Offset3D {
                    x: region.x,
                    y: region.y,
                    z: 0,
                })
                .image_extent(vk::Extent3D {
                    width: region.w,
                    height: region.h,
                    depth: 1,
                });
            device.cmd_copy_buffer_to_image(
                command_buffer,
                staging.buffer(),
                image,
                vk::ImageLayout::TRANSFER_DST_OPTIMAL,
                &[region_copy],
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
}

impl Default for GlyphAtlas {
    fn default() -> Self {
        Self::new()
    }
}

/// Computes a normalized SDF over the binary coverage mask (inside = >127).
/// The signed distance is clamped to +/-`spread` pixels; `spread` follows the
/// historical heuristic (`max(w,h) * 0.35`) so the GPU reconstruction's
/// `smoothing` can be derived from the region size.
pub fn sdf_from_mask(mask: &[u8], width: u32, height: u32) -> Vec<u8> {
    let w = width as usize;
    let h = height as usize;
    let mut sdf = vec![0u8; w * h];
    if w == 0 || h == 0 {
        return sdf;
    }
    let spread = ((w.max(h)) as f32 * 0.35).max(1.0);
    for y in 0..h {
        for x in 0..w {
            let idx = y * w + x;
            let inside = mask[idx] > 127;
            let mut best = f32::INFINITY;
            for yy in 0..h {
                for xx in 0..w {
                    let other = mask[yy * w + xx] > 127;
                    if other == inside {
                        continue;
                    }
                    let dx = x as f32 - xx as f32;
                    let dy = y as f32 - yy as f32;
                    let d = (dx * dx + dy * dy).sqrt();
                    if d < best {
                        best = d;
                    }
                }
            }
            if !best.is_finite() {
                best = spread;
            }
            let signed = if inside { -best } else { best };
            let normalized = (0.5 - signed / (spread * 2.0)).clamp(0.0, 1.0);
            sdf[idx] = (normalized * 255.0) as u8;
        }
    }
    sdf
}


#[cfg(test)]
mod tests {
    use super::{GlyphMode, Segment, Skyline, mode_for_size, sdf_from_mask};

    #[test]
    fn skyline_packs_and_recovers_regions() {
        let mut sky = Skyline::new(64, 64);
        let (x, y, i) = sky.find_best(16, 16).expect("first block fits");
        let (px, py) = sky.place(16, 16, i);
        assert_eq!((px, py), (x, y));
        assert_eq!((px, py), (0, 0));
        let (x2, y2, i2) = sky.find_best(16, 16).expect("second block fits beside");
        let (px2, py2) = sky.place(16, 16, i2);
        assert_eq!((px2, py2), (x2, y2));
        assert_eq!((px2, py2), (16, 0));
        // A block wider than the first shelf spills to the next row.
        let (_, _, i3) = sky.find_best(64, 16).expect("wide block fits below");
        let (px3, py3) = sky.place(64, 16, i3);
        assert_eq!(py3, 16);
        assert_eq!((px3, py3), (0, 16));
        // The skyline top is tracked: total height is now 32.
        let max_y = sky.segments.iter().map(|s| s.y).max().unwrap();
        assert_eq!(max_y, 32);
    }

    #[test]
    fn skyline_split_and_merge_neighbors() {
        let mut sky = Skyline::new(64, 64);
        let (_, _, i0) = sky.find_best(32, 16).unwrap();
        sky.place(32, 16, i0); // [0,32) at top 16
        let (_, _, i1) = sky.find_best(32, 16).unwrap();
        sky.place(32, 16, i1); // [32,64) at top 16
        // Both segments now at y=16 and adjacent; they merge into one.
        assert_eq!(sky.segments.len(), 1);
        assert_eq!(sky.segments[0], Segment { x: 0, y: 16, w: 64 });
        // A block straddling the split boundary packs above both.
        let (x, y, i) = sky.find_best(64, 16).unwrap();
        let (px, py) = sky.place(64, 16, i);
        assert_eq!((px, py, x), (0, 16, 0));
        assert_eq!(y, 16);
    }

    #[test]
    fn skyline_returns_none_when_full() {
        let mut sky = Skyline::new(32, 32);
        let (_, _, i) = sky.find_best(32, 32).unwrap();
        sky.place(32, 32, i);
        assert!(sky.find_best(1, 1).is_none());
    }

    #[test]
    fn mode_threshold() {
        assert_eq!(mode_for_size(16.0), GlyphMode::Bitmap);
        assert_eq!(mode_for_size(24.0), GlyphMode::Sdf);
        assert_eq!(mode_for_size(48.0), GlyphMode::Sdf);
    }

    #[test]
    fn sdf_from_mask_is_normalized_at_contour() {
        // A 3x3 mask with a filled center: the contour is around (1,1).
        let mask = [0u8, 0, 0, 0, 255, 0, 0, 0, 0];
        let sdf = sdf_from_mask(&mask, 3, 3);
        // Center pixel (inside) is > 0.5; corners are < 0.5.
        assert!(sdf[4] > 127, "inside pixel should be SDF > 0.5, got {}", sdf[4]);
        assert!(sdf[0] < 127, "outside pixel should be SDF < 0.5, got {}", sdf[0]);
        // The distance is symmetric: (0,0) and (2,2) are equally far from the
        // center contour.
        assert_eq!(sdf[0], sdf[8]);
    }

}
