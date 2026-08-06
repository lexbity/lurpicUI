//! Converts a decoded frame into GPU draw groups + instance data (Slice 3).
//!
//! The encoder walks commands with a transform/clip/opacity stack, expanding
//! `StrokeRect` into four band fills (matching the software oracle and the CPU
//! stepping-stone raster). Rects sharing the same push-constant state are
//! batched into one instanced draw; when the state changes, the batch closes
//! and its instance bytes are flushed to the ring buffer. The scratch buffer is
//! reused across frames (no per-frame allocation on the hot path).

use ash::vk;

use crate::atlas::GlyphAtlas;
use crate::error::vk_error;
use crate::frame::{
    Brush, BrushKind, DecodedBatch, DecodedCommand, DecodedFrame, DecodedGlyph, Rect, Transform,
};
use crate::geometry::{Color, Point};
use crate::gpu::allocator::Allocator;
use crate::image_store::ImageStore;
use crate::path_flatten::{contours_bounds, flatten_path, path_bounds, winding_triangles};
use crate::pipeline::{
    PushConstants, BRUSH_GLYPH, BRUSH_LINEAR_GRADIENT, BRUSH_SOLID, BRUSH_TEXTURED,
};
use crate::ring_buffer::{InstanceRing, PathRing, UniformRing, INSTANCE_STRIDE};
use crate::RenderResult;

/// Which pipeline + rendering mode a draw group uses.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum DrawKind {
    Solid,
    Textured,
    GlyphBitmap,
    GlyphSdf,
    Gradient,
}

/// One instanced draw: a range of the ring's instance buffer rendered with a
/// single push-constant state. Textured and glyph groups carry the descriptor
/// set binding their image/atlas view + sampler; solid groups have none.
#[derive(Clone, Copy, Debug)]
pub struct DrawGroup {
    pub kind: DrawKind,
    pub push: PushConstants,
    pub first_instance: u32,
    pub instance_count: u32,
    pub descriptor_set: Option<vk::DescriptorSet>,
}

/// The texture/atlas-facing resources the encoder needs to build per-group
/// descriptor sets for `DrawImage`/`DrawTexture` (Slice 4), `DrawGlyphRun`
/// (Slice 5), and `BrushLinearGradient` (Slice 6). The pool is reset per ring
/// slot after its fence signals, so the sets live for exactly the frame's
/// submission.
pub struct Textures<'a> {
    pub images: &'a ImageStore,
    pub atlas: &'a GlyphAtlas,
    pub descriptor_pool: vk::DescriptorPool,
    pub descriptor_layout: vk::DescriptorSetLayout,
    pub gradient_layout: vk::DescriptorSetLayout,
    pub segments_layout: vk::DescriptorSetLayout,
    pub sampler_nearest: vk::Sampler,
    pub sampler_bilinear: vk::Sampler,
    pub uniform_alignment: u64,
    pub device: &'a ash::Device,
}

impl Textures<'_> {
    /// Allocates a combined-image-sampler descriptor set for the given texture
    /// and sampling mode from the frame's descriptor pool.
    fn allocate_descriptor_set(
        &self,
        handle: u64,
        sampler: vk::Sampler,
    ) -> Result<vk::DescriptorSet, (RenderResult, String)> {
        let stored = self.images.get(handle).ok_or_else(|| {
            (
                RenderResult::InvalidHandle,
                format!("texture handle {} does not exist", handle),
            )
        })?;
        self.write_descriptor(stored.view.handle(), sampler)
    }

    /// Allocates a combined-image-sampler descriptor set binding the packed
    /// glyph atlas view + sampler. All glyph groups share the same atlas, so
    /// every such set has identical content.
    fn allocate_glyph_descriptor_set(
        &self,
    ) -> Result<vk::DescriptorSet, (RenderResult, String)> {
        let view = self.atlas.image_view().ok_or_else(|| {
            (
                RenderResult::InitFailed,
                "glyph atlas is not initialized".to_string(),
            )
        })?;
        let sampler = self.atlas.sampler().ok_or_else(|| {
            (
                RenderResult::InitFailed,
                "glyph atlas sampler is not initialized".to_string(),
            )
        })?;
        self.write_descriptor(view, sampler)
    }

    /// Allocates a gradient UBO region from the uniform ring, writes the stop
    /// data, and allocates a UNIFORM_BUFFER descriptor set binding it (Slice 6).
    fn allocate_gradient_descriptor(
        &self,
        uniform_ring: &mut UniformRing,
        allocator: &dyn Allocator,
        ubo_bytes: &[u8],
    ) -> Result<vk::DescriptorSet, (RenderResult, String)> {
        let (buffer, offset) = uniform_ring.write(allocator, ubo_bytes, self.uniform_alignment)?;
        let layouts = [self.gradient_layout];
        let alloc_info = vk::DescriptorSetAllocateInfo {
            descriptor_pool: self.descriptor_pool,
            descriptor_set_count: 1,
            p_set_layouts: layouts.as_ptr(),
            ..Default::default()
        };
        let sets = unsafe { self.device.allocate_descriptor_sets(&alloc_info) }
            .map_err(|e| vk_error("vkAllocateDescriptorSets", e.as_raw()))?;
        let set = sets[0];
        let buffer_info = vk::DescriptorBufferInfo {
            buffer,
            offset,
            range: ubo_bytes.len() as u64,
        };
        let write = vk::WriteDescriptorSet::default()
            .dst_set(set)
            .dst_binding(0)
            .dst_array_element(0)
            .descriptor_type(vk::DescriptorType::UNIFORM_BUFFER)
            .buffer_info(std::slice::from_ref(&buffer_info));
        unsafe {
            self.device.update_descriptor_sets(&[write], &[]);
        }
        Ok(set)
    }

    /// Allocates a STORAGE_BUFFER descriptor set binding the path's flattened
    /// contour edges (Slice 7): the winding-triangle stream in the path ring,
    /// so the cover shader can evaluate the winding at sub-pixel offsets.
    fn allocate_segments_descriptor(
        &self,
        buffer: vk::Buffer,
        byte_offset: u64,
        byte_size: u64,
    ) -> Result<vk::DescriptorSet, (RenderResult, String)> {
        let layouts = [self.segments_layout];
        let alloc_info = vk::DescriptorSetAllocateInfo {
            descriptor_pool: self.descriptor_pool,
            descriptor_set_count: 1,
            p_set_layouts: layouts.as_ptr(),
            ..Default::default()
        };
        let sets = unsafe { self.device.allocate_descriptor_sets(&alloc_info) }
            .map_err(|e| vk_error("vkAllocateDescriptorSets", e.as_raw()))?;
        let set = sets[0];
        let buffer_info = vk::DescriptorBufferInfo {
            buffer,
            offset: byte_offset,
            range: byte_size,
        };
        let write = vk::WriteDescriptorSet::default()
            .dst_set(set)
            .dst_binding(0)
            .dst_array_element(0)
            .descriptor_type(vk::DescriptorType::STORAGE_BUFFER)
            .buffer_info(std::slice::from_ref(&buffer_info));
        unsafe {
            self.device.update_descriptor_sets(&[write], &[]);
        }
        Ok(set)
    }

    fn write_descriptor(
        &self,
        view: vk::ImageView,
        sampler: vk::Sampler,
    ) -> Result<vk::DescriptorSet, (RenderResult, String)> {
        let layouts = [self.descriptor_layout];
        let alloc_info = vk::DescriptorSetAllocateInfo {
            descriptor_pool: self.descriptor_pool,
            descriptor_set_count: 1,
            p_set_layouts: layouts.as_ptr(),
            ..Default::default()
        };
        let sets = unsafe { self.device.allocate_descriptor_sets(&alloc_info) }
            .map_err(|e| vk_error("vkAllocateDescriptorSets", e.as_raw()))?;
        let set = sets[0];
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
            self.device.update_descriptor_sets(&[write], &[]);
        }
        Ok(set)
    }
}

#[derive(Clone, Debug, Default)]
pub struct EncodedFrame {
    pub groups: Vec<DrawGroup>,
    pub path_fills: Vec<PathFill>,
}

/// One stencil-buffer path fill (Slice 7): a winding-triangle stencil pass
/// followed by a stencil-gated cover quad whose shader computes the 4x
/// supersample coverage from the flattened contour edges.
#[derive(Clone, Debug)]
pub struct PathFill {
    /// First winding-triangle vertex in the path ring (in vertices).
    pub first_vertex: u32,
    /// Winding-triangle vertex count (3 per triangle).
    pub vertex_count: u32,
    /// World-space center x of the path — the stencil bottom vertex's x so the
    /// winding triangles stay bounded by the path's horizontal extent.
    pub bottom_center_x: f32,
    /// The cover quad's instance index in the instance ring.
    pub cover_first_instance: u32,
    /// World-space path bounds (for the stencil clear rect).
    pub clear_rect: Rect,
    /// Cover-pass push constants (transform, clip, opacity, brush + edge info).
    pub push: PushConstants,
    /// Gradient UBO descriptor set (set 0; solid path fills have none).
    pub gradient_descriptor: Option<vk::DescriptorSet>,
    /// The flattened contour edges storage-buffer descriptor (set 1).
    pub segments_descriptor: vk::DescriptorSet,
}

/// Fixed stack depth for transform/clip/opacity nesting (Slice 3: "fixed-size
/// [Transform; 16], UI nesting is shallow"). Exceeding it is a malformed frame
/// and aborts encoding rather than growing on the hot path (NFR-6).
const MAX_DEPTH: usize = 16;

pub struct FrameEncoder {
    transform_stack: [Transform; MAX_DEPTH],
    transform_depth: usize,
    clip_stack: [Rect; MAX_DEPTH],
    clip_depth: usize,
    opacity_stack: [f32; MAX_DEPTH],
    opacity_depth: usize,
    /// Instance bytes for the open group.
    scratch: Vec<u8>,
    /// Push constants of the open group.
    current_push: Option<PushConstants>,
    /// Rendering kind of the open group.
    current_kind: Option<DrawKind>,
    /// Texture handle of the open textured group (None for solid/glyph groups).
    current_texture: Option<u64>,
    /// Descriptor set of the open textured/glyph group.
    current_descriptor: Option<vk::DescriptorSet>,
    groups: Vec<DrawGroup>,
    path_fills: Vec<PathFill>,
}

impl Default for FrameEncoder {
    fn default() -> Self {
        Self {
            transform_stack: [Transform::identity(); MAX_DEPTH],
            transform_depth: 0,
            clip_stack: [Rect::zero(); MAX_DEPTH],
            clip_depth: 0,
            opacity_stack: [1.0; MAX_DEPTH],
            opacity_depth: 0,
            scratch: Vec::with_capacity(1024),
            current_push: None,
            current_kind: None,
            current_texture: None,
            current_descriptor: None,
            groups: Vec::with_capacity(64),
            path_fills: Vec::with_capacity(8),
        }
    }
}

impl FrameEncoder {
    /// Encodes a frame into draw groups, flushing instances to `ring`.
    pub fn encode(
        &mut self,
        frame: &DecodedFrame,
        ring: &mut InstanceRing,
        uniform_ring: &mut UniformRing,
        path_ring: &mut PathRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<EncodedFrame, (RenderResult, String)> {
        self.transform_depth = 0;
        self.clip_depth = 0;
        self.opacity_depth = 0;
        self.scratch.clear();
        self.current_push = None;
        self.current_kind = None;
        self.current_texture = None;
        self.current_descriptor = None;
        self.groups.clear();
        self.path_fills.clear();

        let screen = Rect {
            min: crate::geometry::Point { x: 0.0, y: 0.0 },
            max: crate::geometry::Point {
                x: surface_size[0],
                y: surface_size[1],
            },
        };

        for batch in &frame.batches {
            self.encode_batch(
                batch,
                screen,
                ring,
                uniform_ring,
                path_ring,
                allocator,
                surface_size,
                textures,
            )?;
        }

        self.close_group(ring, allocator)?;

        Ok(EncodedFrame {
            groups: std::mem::take(&mut self.groups),
            path_fills: std::mem::take(&mut self.path_fills),
        })
    }

    fn encode_batch(
        &mut self,
        batch: &DecodedBatch,
        screen: Rect,
        ring: &mut InstanceRing,
        uniform_ring: &mut UniformRing,
        path_ring: &mut PathRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<(), (RenderResult, String)> {
        // Seed the state from the batch header (fixed-depth base entries).
        self.transform_stack[0] = batch.transform;
        self.transform_depth = 1;
        let mut clip = batch.bounds;
        if let Some(header_clip) = batch.clip {
            clip = clip.intersect(header_clip);
        }
        clip = clip.intersect(screen);
        self.clip_stack[0] = clip;
        self.clip_depth = 1;
        self.opacity_stack[0] = batch.opacity;
        self.opacity_depth = 1;

        for cmd in &batch.commands {
            match cmd {
                DecodedCommand::FillRect { rect, brush } => {
                    self.emit_fill_path_rect(
                        *rect,
                        brush,
                        ring,
                        uniform_ring,
                        allocator,
                        surface_size,
                        textures,
                    )?;
                }
                DecodedCommand::StrokeRect {
                    rect,
                    stroke,
                    brush,
                } => {
                    if stroke.width <= 0.0 {
                        continue;
                    }
                    let half = stroke.width / 2.0;
                    let outer = rect.inset(-half);
                    let inner = rect.inset(half);
                    for band in [
                        Rect {
                            min: outer.min,
                            max: crate::geometry::Point {
                                x: outer.max.x,
                                y: inner.min.y,
                            },
                        },
                        Rect {
                            min: crate::geometry::Point {
                                x: outer.min.x,
                                y: inner.min.y,
                            },
                            max: crate::geometry::Point {
                                x: inner.min.x,
                                y: inner.max.y,
                            },
                        },
                        Rect {
                            min: crate::geometry::Point {
                                x: inner.max.x,
                                y: inner.min.y,
                            },
                            max: crate::geometry::Point {
                                x: outer.max.x,
                                y: inner.max.y,
                            },
                        },
                        Rect {
                            min: crate::geometry::Point {
                                x: outer.min.x,
                                y: inner.max.y,
                            },
                            max: outer.max,
                        },
                    ] {
                        self.emit_fill_path_rect(
                            band,
                            brush,
                            ring,
                            uniform_ring,
                            allocator,
                            surface_size,
                            textures,
                        )?;
                    }
                }
                DecodedCommand::PushTransform { matrix } => {
                    let next = self.transform().multiply(*matrix);
                    self.push_transform(next)?;
                }
                DecodedCommand::PopTransform => {
                    if self.transform_depth > 1 {
                        self.transform_depth -= 1;
                    }
                }
                DecodedCommand::PushClipRect { rect } => {
                    let world = self.transform().transform_rect(*rect);
                    let next = self.clip().intersect(world);
                    self.push_clip(next)?;
                }
                DecodedCommand::PopClip => {
                    if self.clip_depth > 1 {
                        self.clip_depth -= 1;
                    }
                }
                DecodedCommand::PushOpacity { alpha } => {
                    let next = self.opacity() * alpha;
                    self.push_opacity(next)?;
                }
                DecodedCommand::PopOpacity => {
                    if self.opacity_depth > 1 {
                        self.opacity_depth -= 1;
                    }
                }
                // Slice 3 renders FillRect/StrokeRect only. The remaining
                // commands are consumed by later slices; a Dev-log is emitted
                // for the first occurrence of each to aid debugging.
                DecodedCommand::FillPath { path, brush } => {
                    self.emit_path_fill(
                        path,
                        brush,
                        ring,
                        uniform_ring,
                        path_ring,
                        allocator,
                        surface_size,
                        textures,
                    )?;
                }
                DecodedCommand::StrokePath { .. } => self.log_unsupported("StrokePath"),
                DecodedCommand::DrawPolyline { .. } => self.log_unsupported("DrawPolyline"),
                DecodedCommand::DrawPoints { .. } => self.log_unsupported("DrawPoints"),
                DecodedCommand::DrawSelectionRects { .. } => {
                    self.log_unsupported("DrawSelectionRects")
                }
                DecodedCommand::DrawGlyphRun {
                    font_id,
                    size_bits,
                    origin,
                    glyphs,
                    brush,
                } => {
                    self.emit_glyph_run(
                        *font_id,
                        *size_bits,
                        *origin,
                        glyphs,
                        brush,
                        ring,
                        allocator,
                        surface_size,
                        textures,
                    )?;
                }
                DecodedCommand::DrawImage {
                    handle,
                    dest,
                    src,
                    sampling,
                    opacity,
                }
                | DecodedCommand::DrawTexture {
                    handle,
                    dest,
                    src,
                    sampling,
                    opacity,
                } => {
                    self.emit_textured_quad(
                        *handle,
                        *dest,
                        *src,
                        *sampling,
                        *opacity,
                        ring,
                        allocator,
                        surface_size,
                        textures,
                    )?;
                }
                DecodedCommand::DrawBlurredShadow { .. } => {
                    self.log_unsupported("DrawBlurredShadow")
                }
                DecodedCommand::BeginRenderBatch { .. } | DecodedCommand::EndRenderBatch => {}
            }
        }
        Ok(())
    }

    fn emit_fill_path_rect(
        &mut self,
        rect: Rect,
        brush: &Brush,
        ring: &mut InstanceRing,
        uniform_ring: &mut UniformRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<(), (RenderResult, String)> {
        // The instance carries the LOCAL rect; the vertex shader applies the
        // push-constant transform (Q4), so the transform must not be baked in.
        let world = self.transform().transform_rect(rect);
        if world.is_empty() {
            return Ok(());
        }
        let clip = self.clip();
        // Cull rects fully outside the clip.
        if world.max.x <= clip.min.x
            || world.min.x >= clip.max.x
            || world.max.y <= clip.min.y
            || world.min.y >= clip.max.y
        {
            return Ok(());
        }

        let push = PushConstants {
            transform: self.transform().to_array(),
            opacity: self.opacity(),
            clip_min: [clip.min.x, clip.min.y],
            clip_size: [clip.max.x - clip.min.x, clip.max.y - clip.min.y],
            clip_active: 1,
            brush_kind: match brush.kind {
                BrushKind::Solid => BRUSH_SOLID,
                BrushKind::LinearGradient => BRUSH_LINEAR_GRADIENT,
            },
            brush_payload: [0.0; 8],
            surface_size,
        };

        match brush.kind {
            BrushKind::Solid => {
                let color = brush.color;
                self.open_group(DrawKind::Solid, push, None, ring, allocator)?;
                self.scratch.extend_from_slice(&pack_instance(&rect, &color));
            }
            BrushKind::LinearGradient => {
                if brush.gradient_stops.is_empty() {
                    return Ok(());
                }
                let start = brush.gradient_start;
                let end = brush.gradient_end;
                let count = brush.gradient_stops.len().min(MAX_GRADIENT_STOPS) as f32;
                let hash = gradient_hash(brush);
                let mut payload = [0.0f32; 8];
                payload[0] = start.x;
                payload[1] = start.y;
                payload[2] = end.x;
                payload[3] = end.y;
                payload[4] = count;
                payload[5] = f32::from_bits(hash as u32);
                payload[6] = f32::from_bits((hash >> 32) as u32);
                let mut push = push;
                push.brush_payload = payload;
                if self.open_group(DrawKind::Gradient, push, None, ring, allocator)? {
                    let ubo = pack_gradient_ubo(brush);
                    self.current_descriptor = Some(textures.allocate_gradient_descriptor(
                        uniform_ring,
                        allocator,
                        &ubo,
                    )?);
                }
                // The instance color is unused by the gradient shader; pack a
                // zero rect color to keep the 32-byte instance layout.
                self.scratch
                    .extend_from_slice(&pack_instance(&rect, &Color::default()));
            }
        }
        Ok(())
    }

    /// Emits a stencil-buffer path fill (Slice 7): flattens the path to
    /// world-space contours, builds winding triangles into the path ring, and
    /// records the cover quad + push constants for the stencil-gated cover
    /// pass. Solid and gradient brushes are both honored.
    fn emit_path_fill(
        &mut self,
        path: &crate::geometry::Path,
        brush: &Brush,
        ring: &mut InstanceRing,
        uniform_ring: &mut UniformRing,
        path_ring: &mut PathRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<(), (RenderResult, String)> {
        let transform = self.transform();
        let contours = flatten_path(path, |p| transform.apply_point(p));
        if contours.is_empty() {
            return Ok(());
        }
        let Some((world_min, world_max)) = contours_bounds(&contours) else {
            return Ok(());
        };
        let world_bounds = Rect {
            min: world_min,
            max: world_max,
        };
        let clip = self.clip();
        if world_bounds.max.x <= clip.min.x
            || world_bounds.min.x >= clip.max.x
            || world_bounds.max.y <= clip.min.y
            || world_bounds.min.y >= clip.max.y
        {
            return Ok(());
        }

        // Winding triangles into the path ring. `first_vertex` is the vec2
        // index; the segments SSBO and the stencil draw index in vec2s.
        let tri = winding_triangles(&contours);
        let (path_buffer, first_vertex) = path_ring.append_vertices(allocator, &tri)?;
        let vertex_count = (tri.len() / 2) as u32;
        let edge_count = vertex_count / 3;
        let base_edge = first_vertex / 3;

        // The cover shader evaluates the winding over the flattened edges.
        let segments_descriptor = textures.allocate_segments_descriptor(
            path_buffer,
            (first_vertex as u64) * 8,
            (vertex_count as u64) * 8,
        )?;

        // Cover instance: the LOCAL path bounds (the cover shader applies the
        // push transform). Solid fills carry the brush color in the instance
        // (also mirrored in the push payload); gradient fills leave it unused.
        let Some((local_min, local_max)) = path_bounds(path) else {
            return Ok(());
        };
        let cover_color = match brush.kind {
            BrushKind::Solid => brush.color,
            BrushKind::LinearGradient => Color::default(),
        };
        let cover = pack_instance(
            &Rect {
                min: local_min,
                max: local_max,
            },
            &cover_color,
        );
        let (_, cover_first_instance, _) = ring.append(allocator, &cover)?;

        let gradient_descriptor = match brush.kind {
            BrushKind::LinearGradient => {
                if brush.gradient_stops.is_empty() {
                    return Ok(());
                }
                let ubo = pack_gradient_ubo(brush);
                Some(textures.allocate_gradient_descriptor(
                    uniform_ring,
                    allocator,
                    &ubo,
                )?)
            }
            BrushKind::Solid => None,
        };

        let clear_rect = world_bounds.intersect(Rect {
            min: crate::geometry::Point { x: 0.0, y: 0.0 },
            max: crate::geometry::Point {
                x: surface_size[0],
                y: surface_size[1],
            },
        });

        // Cover push: brush info + the edge range for the winding coverage.
        let mut payload = [0.0f32; 8];
        payload[4] = edge_count as f32;
        payload[5] = f32::from_bits(base_edge);
        match brush.kind {
            BrushKind::Solid => {
                payload[0] = brush.color.r;
                payload[1] = brush.color.g;
                payload[2] = brush.color.b;
                payload[3] = brush.color.a;
            }
            BrushKind::LinearGradient => {
                payload[0] = brush.gradient_start.x;
                payload[1] = brush.gradient_start.y;
                payload[2] = brush.gradient_end.x;
                payload[3] = brush.gradient_end.y;
            }
        }

        self.path_fills.push(PathFill {
            first_vertex,
            vertex_count,
            bottom_center_x: (world_bounds.min.x + world_bounds.max.x) * 0.5,
            cover_first_instance,
            clear_rect,
            push: PushConstants {
                transform: self.transform().to_array(),
                opacity: self.opacity(),
                clip_min: [clip.min.x, clip.min.y],
                clip_size: [clip.max.x - clip.min.x, clip.max.y - clip.min.y],
                clip_active: 1,
                brush_kind: match brush.kind {
                    BrushKind::Solid => BRUSH_SOLID,
                    BrushKind::LinearGradient => BRUSH_LINEAR_GRADIENT,
                },
                brush_payload: payload,
                surface_size,
            },
            gradient_descriptor,
            segments_descriptor,
        });
        Ok(())
    }

    fn emit_textured_quad(
        &mut self,
        handle: u64,
        dest: Rect,
        src: Rect,
        sampling: u8,
        opacity: f32,
        ring: &mut InstanceRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<(), (RenderResult, String)> {
        let world = self.transform().transform_rect(dest);
        if world.is_empty() {
            return Ok(());
        }
        let clip = self.clip();
        if world.max.x <= clip.min.x
            || world.min.x >= clip.max.x
            || world.max.y <= clip.min.y
            || world.min.y >= clip.max.y
        {
            return Ok(());
        }

        if textures.images.get(handle).is_none() {
            return Err((
                RenderResult::InvalidHandle,
                format!("texture handle {} does not exist", handle),
            ));
        }
        let sampler = match sampling {
            0 => textures.sampler_nearest,
            1 => textures.sampler_bilinear,
            other => {
                return Err((
                    RenderResult::InitFailed,
                    format!("unsupported sampling mode {}", other),
                ))
            }
        };

        let push = PushConstants {
            transform: self.transform().to_array(),
            opacity: self.opacity() * opacity,
            clip_min: [clip.min.x, clip.min.y],
            clip_size: [clip.max.x - clip.min.x, clip.max.y - clip.min.y],
            clip_active: 1,
            brush_kind: BRUSH_TEXTURED,
            // brush_payload[0] carries the sampling mode for the fragment
            // shader; it is part of the group key so nearest and bilinear
            // draws of the same texture do not coalesce.
            brush_payload: [sampling as f32, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0],
            surface_size,
        };

        if self.open_group(DrawKind::Textured, push, Some(handle), ring, allocator)? {
            self.current_descriptor = Some(textures.allocate_descriptor_set(handle, sampler)?);
        }

        self.scratch
            .extend_from_slice(&pack_textured_instance(&dest, &src));
        Ok(())
    }

    /// Emits one instanced glyph quad per glyph in a run (Slice 5). The
    /// placement is rounded AFTER the batch transform (matching the software
    /// oracle), and the dest rect is world-space — the glyph vertex shader does
    /// not re-apply the transform.
    fn emit_glyph_run(
        &mut self,
        font_id: u64,
        size_bits: u32,
        origin: Point,
        glyphs: &[DecodedGlyph],
        brush: &Brush,
        ring: &mut InstanceRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<(), (RenderResult, String)> {
        let color = match brush.kind {
            BrushKind::Solid => brush.color,
            BrushKind::LinearGradient => {
                // Gradient text lands with the gradient pipeline (Slice 6).
                return Ok(());
            }
        };
        let clip = self.clip();
        for glyph in glyphs {
            let Some(entry) = textures.atlas.get(font_id, glyph.glyph_id, size_bits) else {
                // The glyph was not uploaded (an un-rasterizable run member);
                // the software oracle skips the same glyphs.
                continue;
            };
            let local = Point {
                x: origin.x + glyph.x + entry.offset_x,
                y: origin.y + glyph.y + entry.offset_y,
            };
            let world = self.transform().apply_point(local);
            let ox = world.x.round();
            let oy = world.y.round();
            let dest = Rect {
                min: Point { x: ox, y: oy },
                max: Point {
                    x: ox + entry.width as f32,
                    y: oy + entry.height as f32,
                },
            };
            if dest.max.x <= clip.min.x
                || dest.min.x >= clip.max.x
                || dest.max.y <= clip.min.y
                || dest.min.y >= clip.max.y
            {
                continue;
            }

            let kind = match entry.mode {
                crate::atlas::GlyphMode::Bitmap => DrawKind::GlyphBitmap,
                crate::atlas::GlyphMode::Sdf => DrawKind::GlyphSdf,
            };
            let push = PushConstants {
                transform: self.transform().to_array(),
                opacity: self.opacity(),
                clip_min: [clip.min.x, clip.min.y],
                clip_size: [clip.max.x - clip.min.x, clip.max.y - clip.min.y],
                clip_active: 1,
                brush_kind: BRUSH_GLYPH,
                // Premultiplied brush color; the pipeline variant (bitmap vs
                // SDF) is selected by the group kind.
                brush_payload: [color.r, color.g, color.b, color.a, 0.0, 0.0, 0.0, 0.0],
                surface_size,
            };
            if self.open_group(kind, push, None, ring, allocator)? {
                self.current_descriptor = Some(textures.allocate_glyph_descriptor_set()?);
            }

            let region = Rect {
                min: Point {
                    x: entry.region.x as f32,
                    y: entry.region.y as f32,
                },
                max: Point {
                    x: (entry.region.x + entry.region.w as i32) as f32,
                    y: (entry.region.y + entry.region.h as i32) as f32,
                },
            };
            self.scratch
                .extend_from_slice(&pack_textured_instance(&dest, &region));
        }
        Ok(())
    }

    /// Opens a new group unless one with the same kind + push + texture is
    /// already open. Returns true when a new group was opened (the caller then
    /// supplies its descriptor set). Returns false when the existing group is
    /// reused (batched).
    fn open_group(
        &mut self,
        kind: DrawKind,
        push: PushConstants,
        texture: Option<u64>,
        ring: &mut InstanceRing,
        allocator: &dyn Allocator,
    ) -> Result<bool, (RenderResult, String)> {
        let same = self
            .current_push
            .map_or(false, |p| push_constants_eq(&p, &push))
            && self.current_kind == Some(kind)
            && self.current_texture == texture;
        if same {
            return Ok(false);
        }
        self.close_group(ring, allocator)?;
        self.current_push = Some(push);
        self.current_kind = Some(kind);
        self.current_texture = texture;
        self.current_descriptor = None;
        Ok(true)
    }

    fn close_group(
        &mut self,
        ring: &mut InstanceRing,
        allocator: &dyn Allocator,
    ) -> Result<(), (RenderResult, String)> {
        let (Some(push), Some(kind)) = (self.current_push.take(), self.current_kind.take()) else {
            return Ok(());
        };
        if self.scratch.is_empty() {
            self.current_descriptor = None;
            return Ok(());
        }
        let (buffer, first, count) = ring.append(allocator, &self.scratch)?;
        let _ = buffer;
        self.groups.push(DrawGroup {
            kind,
            push,
            first_instance: first,
            instance_count: count,
            descriptor_set: self.current_descriptor.take(),
        });
        self.scratch.clear();
        Ok(())
    }

    fn push_transform(&mut self, value: Transform) -> Result<(), (RenderResult, String)> {
        if self.transform_depth >= MAX_DEPTH {
            return Err((
                RenderResult::InitFailed,
                format!("transform nesting exceeds the fixed {} depth", MAX_DEPTH),
            ));
        }
        self.transform_stack[self.transform_depth] = value;
        self.transform_depth += 1;
        Ok(())
    }

    fn push_clip(&mut self, value: Rect) -> Result<(), (RenderResult, String)> {
        if self.clip_depth >= MAX_DEPTH {
            return Err((
                RenderResult::InitFailed,
                format!("clip nesting exceeds the fixed {} depth", MAX_DEPTH),
            ));
        }
        self.clip_stack[self.clip_depth] = value;
        self.clip_depth += 1;
        Ok(())
    }

    fn push_opacity(&mut self, value: f32) -> Result<(), (RenderResult, String)> {
        if self.opacity_depth >= MAX_DEPTH {
            return Err((
                RenderResult::InitFailed,
                format!("opacity nesting exceeds the fixed {} depth", MAX_DEPTH),
            ));
        }
        self.opacity_stack[self.opacity_depth] = value;
        self.opacity_depth += 1;
        Ok(())
    }

    fn transform(&self) -> Transform {
        self.transform_stack[self.transform_depth - 1]
    }

    fn clip(&self) -> Rect {
        self.clip_stack[self.clip_depth - 1]
    }

    fn opacity(&self) -> f32 {
        self.opacity_stack[self.opacity_depth - 1]
    }

    fn log_unsupported(&self, name: &str) {
        // Dev-only log; production renders are gated by which fixtures the
        // slice supports.
        #[cfg(debug_assertions)]
        eprintln!(
            "lurpic_render: {} not yet rendered by the GPU pipeline",
            name
        );
        #[cfg(not(debug_assertions))]
        let _ = name;
    }
}

fn push_constants_eq(a: &PushConstants, b: &PushConstants) -> bool {
    a.transform == b.transform
        && a.opacity == b.opacity
        && a.clip_min == b.clip_min
        && a.clip_size == b.clip_size
        && a.clip_active == b.clip_active
        && a.brush_kind == b.brush_kind
        && a.brush_payload == b.brush_payload
}

/// Packs a world-space rect + premultiplied color into a 32-byte instance
/// record: [x, y, w, h, r, g, b, a].
fn pack_instance(rect: &Rect, color: &Color) -> [u8; INSTANCE_STRIDE as usize] {
    let mut out = [0u8; INSTANCE_STRIDE as usize];
    let values = [
        rect.min.x,
        rect.min.y,
        rect.max.x - rect.min.x,
        rect.max.y - rect.min.y,
        color.r,
        color.g,
        color.b,
        color.a,
    ];
    for (i, v) in values.iter().enumerate() {
        out[i * 4..i * 4 + 4].copy_from_slice(&v.to_le_bytes());
    }
    out
}

/// Packs a textured quad into a 32-byte instance record (mirrors the textured
/// pipeline's instance bindings): [dest.x, dest.y, dest.w, dest.h, src.x,
/// src.y, src.w, src.h].
fn pack_textured_instance(dest: &Rect, src: &Rect) -> [u8; INSTANCE_STRIDE as usize] {
    let mut out = [0u8; INSTANCE_STRIDE as usize];
    let values = [
        dest.min.x,
        dest.min.y,
        dest.max.x - dest.min.x,
        dest.max.y - dest.min.y,
        src.min.x,
        src.min.y,
        src.max.x - src.min.x,
        src.max.y - src.min.y,
    ];
    for (i, v) in values.iter().enumerate() {
        out[i * 4..i * 4 + 4].copy_from_slice(&v.to_le_bytes());
    }
    out
}

/// Maximum gradient stops per brush (Slice 6; beyond this the stops are
/// truncated at encode, matching the shader's fixed UBO).
pub const MAX_GRADIENT_STOPS: usize = 16;

/// The gradient UBO byte size. Layout (std140, matches `gradient.frag`):
/// `u32 stop_count` at 0, 12 bytes padding, then 32 `vec4`s at 16 — two per
/// stop: (offset, r, g, b) and (a, 0, 0, 0). 16 stops * 32 bytes + 16 header.
pub const GRADIENT_UBO_SIZE: usize = 16 + MAX_GRADIENT_STOPS * 32;

/// Packs a linear-gradient brush into the gradient UBO bytes.
fn pack_gradient_ubo(brush: &Brush) -> [u8; GRADIENT_UBO_SIZE] {
    let mut out = [0u8; GRADIENT_UBO_SIZE];
    let count = brush.gradient_stops.len().min(MAX_GRADIENT_STOPS);
    out[0..4].copy_from_slice(&(count as u32).to_le_bytes());
    for (i, stop) in brush.gradient_stops.iter().take(count).enumerate() {
        let base = 16 + i * 32;
        let vals = [
            stop.offset,
            stop.color.r,
            stop.color.g,
            stop.color.b,
            stop.color.a,
            0.0,
            0.0,
            0.0,
        ];
        for (j, v) in vals.iter().enumerate() {
            out[base + j * 4..base + j * 4 + 4].copy_from_slice(&v.to_le_bytes());
        }
    }
    out
}

/// FNV-1a 64-bit over the gradient's full content (start/end/count/stops).
/// Carried in the push constants so the group key distinguishes gradients that
/// share geometry but differ in stop content (two gradient brushes with the
/// same start/end/count but different colors must not coalesce into one group
/// and share one UBO).
fn gradient_hash(brush: &Brush) -> u64 {
    let mut h: u64 = 0xcbf2_9ce4_8422_2325;
    for v in [
        brush.gradient_start.x,
        brush.gradient_start.y,
        brush.gradient_end.x,
        brush.gradient_end.y,
    ] {
        for b in v.to_le_bytes() {
            h ^= b as u64;
            h = h.wrapping_mul(0x0000_0100_0000_01b3);
        }
    }
    let count = brush.gradient_stops.len().min(MAX_GRADIENT_STOPS);
    for b in (count as u32).to_le_bytes() {
        h ^= b as u64;
        h = h.wrapping_mul(0x0000_0100_0000_01b3);
    }
    for stop in brush.gradient_stops.iter().take(count) {
        for b in stop.offset.to_le_bytes() {
            h ^= b as u64;
            h = h.wrapping_mul(0x0000_0100_0000_01b3);
        }
        for v in [stop.color.r, stop.color.g, stop.color.b, stop.color.a] {
            for b in v.to_le_bytes() {
                h ^= b as u64;
                h = h.wrapping_mul(0x0000_0100_0000_01b3);
            }
        }
    }
    h
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::geometry::Point;

    fn transform(a: f32, b: f32, c: f32, d: f32, tx: f32, ty: f32) -> Transform {
        Transform::from_parts(a, b, c, d, tx, ty)
    }

    fn rect(x0: f32, y0: f32, x1: f32, y1: f32) -> Rect {
        Rect {
            min: Point { x: x0, y: y0 },
            max: Point { x: x1, y: y1 },
        }
    }

    // PushConstants equality across the state the encoder groups on.
    #[test]
    fn push_constants_eq_groups_on_transform_and_clip() {
        let a = PushConstants {
            transform: transform(1.0, 0.0, 0.0, 1.0, 5.0, 5.0).to_array(),
            opacity: 1.0,
            clip_min: [0.0, 0.0],
            clip_size: [100.0, 100.0],
            clip_active: 1,
            brush_kind: 0,
            brush_payload: [0.0; 8],
            surface_size: [100.0, 100.0],
        };
        let mut b = a;
        assert!(push_constants_eq(&a, &b));
        b.opacity = 0.5;
        assert!(!push_constants_eq(&a, &b));
        b.opacity = a.opacity;
        b.clip_min[0] = 1.0;
        assert!(!push_constants_eq(&a, &b));
    }

    // The encoder must expand a stroke rect into four band instances.
    #[test]
    fn pack_instance_layout() {
        let packed = pack_instance(
            &rect(10.0, 20.0, 30.0, 40.0),
            &Color {
                r: 1.0,
                g: 0.5,
                b: 0.0,
                a: 1.0,
            },
        );
        assert_eq!(&packed[0..4], &10.0f32.to_le_bytes());
        assert_eq!(&packed[4..8], &20.0f32.to_le_bytes());
        assert_eq!(&packed[8..12], &20.0f32.to_le_bytes());
        assert_eq!(&packed[16..20], &1.0f32.to_le_bytes());
        assert_eq!(&packed[28..32], &1.0f32.to_le_bytes());
    }

    // Textured instances carry the dest xywh then the src xywh, mirroring the
    // textured pipeline's instance bindings (Slice 4).
    #[test]
    fn pack_textured_instance_layout() {
        let packed = pack_textured_instance(&rect(10.0, 20.0, 30.0, 40.0), &rect(1.0, 2.0, 5.0, 7.0));
        assert_eq!(&packed[0..4], &10.0f32.to_le_bytes());
        assert_eq!(&packed[4..8], &20.0f32.to_le_bytes());
        assert_eq!(&packed[8..12], &20.0f32.to_le_bytes()); // dest width
        assert_eq!(&packed[12..16], &20.0f32.to_le_bytes()); // dest height
        assert_eq!(&packed[16..20], &1.0f32.to_le_bytes());
        assert_eq!(&packed[20..24], &2.0f32.to_le_bytes());
        assert_eq!(&packed[24..28], &4.0f32.to_le_bytes()); // src width
        assert_eq!(&packed[28..32], &5.0f32.to_le_bytes()); // src height
    }

    fn gradient_brush(start: Point, end: Point, stops: Vec<(f32, Color)>) -> Brush {
        Brush {
            kind: BrushKind::LinearGradient,
            color: Color::default(),
            gradient_start: start,
            gradient_end: end,
            gradient_stops: stops
                .into_iter()
                .map(|(offset, color)| crate::frame::GradientStop { offset, color })
                .collect(),
        }
    }

    // The gradient UBO layout must match gradient.frag: stop_count at 0, 12
    // bytes padding, then 2 vec4s per stop (offset,r,g,b)(a,0,0,0) at 16.
    #[test]
    fn pack_gradient_ubo_layout_matches_shader() {
        let brush = gradient_brush(
            Point { x: 0.0, y: 0.0 },
            Point { x: 64.0, y: 0.0 },
            vec![
                (0.0, Color { r: 0.5, g: 0.25, b: 0.1, a: 1.0 }),
                (1.0, Color { r: 0.0, g: 0.75, b: 0.9, a: 0.5 }),
            ],
        );
        let ubo = pack_gradient_ubo(&brush);
        assert_eq!(ubo.len(), GRADIENT_UBO_SIZE);
        assert_eq!(&ubo[0..4], &2u32.to_le_bytes());
        assert_eq!(&ubo[4..16], &[0u8; 12]);
        // Stop 0: (offset, r, g, b) then (a, 0, 0, 0).
        assert_eq!(&ubo[16..20], &0.0f32.to_le_bytes());
        assert_eq!(&ubo[20..24], &0.5f32.to_le_bytes());
        assert_eq!(&ubo[28..32], &0.1f32.to_le_bytes());
        assert_eq!(&ubo[32..36], &1.0f32.to_le_bytes()); // stop 0 alpha
        // Stop 1 at 16 + 32.
        assert_eq!(&ubo[48..52], &1.0f32.to_le_bytes()); // stop 1 offset
        assert_eq!(&ubo[52..56], &0.0f32.to_le_bytes()); // stop 1 r
        assert_eq!(&ubo[64..68], &0.5f32.to_le_bytes()); // stop 1 alpha
    }

    // Gradient groups must not coalesce when the stop content differs even
    // though start/end/count match — the hash drives the group key.
    #[test]
    fn gradient_hash_distinguishes_stop_content() {
        let a = gradient_brush(
            Point { x: 0.0, y: 0.0 },
            Point { x: 64.0, y: 0.0 },
            vec![(0.0, Color { r: 1.0, g: 0.0, b: 0.0, a: 1.0 })],
        );
        let b = gradient_brush(
            Point { x: 0.0, y: 0.0 },
            Point { x: 64.0, y: 0.0 },
            vec![(0.0, Color { r: 0.0, g: 0.0, b: 1.0, a: 1.0 })],
        );
        assert_ne!(gradient_hash(&a), gradient_hash(&b));

        // Same content hashes identically (batching).
        let c = gradient_brush(
            Point { x: 0.0, y: 0.0 },
            Point { x: 64.0, y: 0.0 },
            vec![(0.0, Color { r: 1.0, g: 0.0, b: 0.0, a: 1.0 })],
        );
        assert_eq!(gradient_hash(&a), gradient_hash(&c));
    }

    // The transform/clip/opacity stacks are fixed-size (NFR-6): a frame that
    // nests deeper than MAX_DEPTH aborts with an error instead of growing.
    #[test]
    fn stacks_are_fixed_depth_and_error_on_overflow() {
        let mut e = FrameEncoder::default();
        e.transform_stack[0] = Transform::identity();
        e.transform_depth = 1;
        for _ in 0..MAX_DEPTH - 1 {
            e.push_transform(Transform::identity())
                .expect("shallow push");
        }
        let err = e
            .push_transform(Transform::identity())
            .expect_err("must overflow");
        assert_eq!(err.0, RenderResult::InitFailed);
        assert!(err.1.contains("transform nesting"));

        e.clip_stack[0] = rect(0.0, 0.0, 10.0, 10.0);
        e.clip_depth = 1;
        for _ in 0..MAX_DEPTH - 1 {
            e.push_clip(rect(0.0, 0.0, 10.0, 10.0))
                .expect("shallow push");
        }
        let err = e
            .push_clip(rect(0.0, 0.0, 10.0, 10.0))
            .expect_err("must overflow");
        assert!(err.1.contains("clip nesting"));

        e.opacity_stack[0] = 1.0;
        e.opacity_depth = 1;
        for _ in 0..MAX_DEPTH - 1 {
            e.push_opacity(0.5).expect("shallow push");
        }
        let err = e.push_opacity(0.5).expect_err("must overflow");
        assert!(err.1.contains("opacity nesting"));
    }

    // Pops below the seeded base entry are ignored (mirroring the command
    // handler's `if depth > 1` guard), and the top accessor follows the active
    // depth without underflowing.
    #[test]
    fn stacks_pop_to_base_and_track_depth() {
        let mut e = FrameEncoder::default();
        e.transform_stack[0] = transform(1.0, 0.0, 0.0, 1.0, 5.0, 5.0);
        e.transform_depth = 1;
        e.push_transform(transform(2.0, 0.0, 0.0, 2.0, 0.0, 0.0))
            .unwrap();
        assert_eq!(e.transform(), transform(2.0, 0.0, 0.0, 2.0, 0.0, 0.0));
        if e.transform_depth > 1 {
            e.transform_depth -= 1; // PopTransform
        }
        assert_eq!(e.transform(), transform(1.0, 0.0, 0.0, 1.0, 5.0, 5.0));
        if e.transform_depth > 1 {
            e.transform_depth -= 1; // over-pop guarded: stays at base
        }
        assert_eq!(e.transform_depth, 1);
        assert_eq!(e.transform(), transform(1.0, 0.0, 0.0, 1.0, 5.0, 5.0));
    }
}
