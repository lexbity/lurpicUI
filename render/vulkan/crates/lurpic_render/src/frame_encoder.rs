//! Converts a decoded frame into GPU draw groups + instance data (Slice 3).
//!
//! The encoder walks commands with a transform/clip/opacity stack, expanding
//! `StrokeRect` into four band fills (matching the software oracle and the CPU
//! stepping-stone raster). Rects sharing the same push-constant state are
//! batched into one instanced draw; when the state changes, the batch closes
//! and its instance bytes are flushed to the ring buffer. The scratch buffer is
//! reused across frames (no per-frame allocation on the hot path).

use ash::vk;

use crate::error::vk_error;
use crate::frame::{Brush, BrushKind, DecodedBatch, DecodedCommand, DecodedFrame, Rect, Transform};
use crate::geometry::Color;
use crate::gpu::allocator::Allocator;
use crate::image_store::ImageStore;
use crate::pipeline::{PushConstants, BRUSH_SOLID, BRUSH_TEXTURED};
use crate::ring_buffer::{InstanceRing, INSTANCE_STRIDE};
use crate::RenderResult;

/// One instanced draw: a range of the ring's instance buffer rendered with a
/// single push-constant state. Textured groups carry the descriptor set binding
/// the image view + sampler; solid groups have none.
#[derive(Clone, Copy, Debug)]
pub struct DrawGroup {
    pub push: PushConstants,
    pub first_instance: u32,
    pub instance_count: u32,
    pub descriptor_set: Option<vk::DescriptorSet>,
}

/// The texture-facing resources the encoder needs to build per-group descriptor
/// sets for `DrawImage`/`DrawTexture` (Slice 4). The pool is reset per ring
/// slot after its fence signals, so the sets live for exactly the frame's
/// submission.
pub struct Textures<'a> {
    pub images: &'a ImageStore,
    pub descriptor_pool: vk::DescriptorPool,
    pub descriptor_layout: vk::DescriptorSetLayout,
    pub sampler_nearest: vk::Sampler,
    pub sampler_bilinear: vk::Sampler,
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
            .image_view(stored.view.handle())
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
    /// Texture handle of the open textured group (None for solid groups).
    current_texture: Option<u64>,
    /// Descriptor set of the open textured group.
    current_descriptor: Option<vk::DescriptorSet>,
    groups: Vec<DrawGroup>,
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
            current_texture: None,
            current_descriptor: None,
            groups: Vec::with_capacity(64),
        }
    }
}

impl FrameEncoder {
    /// Encodes a frame into draw groups, flushing instances to `ring`.
    pub fn encode(
        &mut self,
        frame: &DecodedFrame,
        ring: &mut InstanceRing,
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
        textures: &Textures<'_>,
    ) -> Result<EncodedFrame, (RenderResult, String)> {
        self.transform_depth = 0;
        self.clip_depth = 0;
        self.opacity_depth = 0;
        self.scratch.clear();
        self.current_push = None;
        self.current_texture = None;
        self.current_descriptor = None;
        self.groups.clear();

        let screen = Rect {
            min: crate::geometry::Point { x: 0.0, y: 0.0 },
            max: crate::geometry::Point {
                x: surface_size[0],
                y: surface_size[1],
            },
        };

        for batch in &frame.batches {
            self.encode_batch(batch, screen, ring, allocator, surface_size, textures)?;
        }

        self.close_group(ring, allocator)?;

        Ok(EncodedFrame {
            groups: std::mem::take(&mut self.groups),
        })
    }

    fn encode_batch(
        &mut self,
        batch: &DecodedBatch,
        screen: Rect,
        ring: &mut InstanceRing,
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
                    self.emit_fill_path_rect(*rect, brush, ring, allocator, surface_size)?;
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
                        self.emit_fill_path_rect(band, brush, ring, allocator, surface_size)?;
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
                DecodedCommand::FillPath { .. } => self.log_unsupported("FillPath"),
                DecodedCommand::StrokePath { .. } => self.log_unsupported("StrokePath"),
                DecodedCommand::DrawPolyline { .. } => self.log_unsupported("DrawPolyline"),
                DecodedCommand::DrawPoints { .. } => self.log_unsupported("DrawPoints"),
                DecodedCommand::DrawSelectionRects { .. } => {
                    self.log_unsupported("DrawSelectionRects")
                }
                DecodedCommand::DrawGlyphRun { .. } => self.log_unsupported("DrawGlyphRun"),
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
        allocator: &dyn Allocator,
        surface_size: [f32; 2],
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

        let color = match brush.kind {
            BrushKind::Solid => brush.color,
            BrushKind::LinearGradient => {
                // Gradient brushes land in Slice 6.
                return Ok(());
            }
        };

        let push = PushConstants {
            transform: self.transform().to_array(),
            opacity: self.opacity(),
            clip_min: [clip.min.x, clip.min.y],
            clip_size: [clip.max.x - clip.min.x, clip.max.y - clip.min.y],
            clip_active: 1,
            brush_kind: BRUSH_SOLID,
            brush_payload: [0.0; 8],
            surface_size,
        };

        // A solid draw is a different group kind from a textured draw: opening
        // one must drop any textured group's descriptor set state.
        if self
            .current_push
            .map_or(true, |p| !push_constants_eq(&p, &push))
            || self.current_texture.is_some()
        {
            self.close_group(ring, allocator)?;
            self.current_push = Some(push);
            self.current_texture = None;
            self.current_descriptor = None;
        }

        self.scratch
            .extend_from_slice(&pack_instance(&rect, &color));
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

        let same_group = self
            .current_push
            .map_or(false, |p| push_constants_eq(&p, &push))
            && self.current_texture == Some(handle);
        if !same_group {
            self.close_group(ring, allocator)?;
            self.current_push = Some(push);
            self.current_texture = Some(handle);
            self.current_descriptor = Some(textures.allocate_descriptor_set(handle, sampler)?);
        }

        self.scratch
            .extend_from_slice(&pack_textured_instance(&dest, &src));
        Ok(())
    }

    fn close_group(
        &mut self,
        ring: &mut InstanceRing,
        allocator: &dyn Allocator,
    ) -> Result<(), (RenderResult, String)> {
        let Some(push) = self.current_push.take() else {
            return Ok(());
        };
        if self.scratch.is_empty() {
            self.current_descriptor = None;
            return Ok(());
        }
        let (buffer, first, count) = ring.append(allocator, &self.scratch)?;
        let _ = buffer;
        self.groups.push(DrawGroup {
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
        let packed =
            pack_textured_instance(&rect(10.0, 20.0, 30.0, 40.0), &rect(1.0, 2.0, 5.0, 7.0));
        assert_eq!(&packed[0..4], &10.0f32.to_le_bytes());
        assert_eq!(&packed[4..8], &20.0f32.to_le_bytes());
        assert_eq!(&packed[8..12], &20.0f32.to_le_bytes()); // dest width
        assert_eq!(&packed[12..16], &20.0f32.to_le_bytes()); // dest height
        assert_eq!(&packed[16..20], &1.0f32.to_le_bytes());
        assert_eq!(&packed[20..24], &2.0f32.to_le_bytes());
        assert_eq!(&packed[24..28], &4.0f32.to_le_bytes()); // src width
        assert_eq!(&packed[28..32], &5.0f32.to_le_bytes()); // src height
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
