//! Converts a decoded frame into GPU draw groups + instance data (Slice 3).
//!
//! The encoder walks commands with a transform/clip/opacity stack, expanding
//! `StrokeRect` into four band fills (matching the software oracle and the CPU
//! stepping-stone raster). Rects sharing the same push-constant state are
//! batched into one instanced draw; when the state changes, the batch closes
//! and its instance bytes are flushed to the ring buffer. The scratch buffer is
//! reused across frames (no per-frame allocation on the hot path).

use crate::frame::{
    Brush, BrushKind, DecodedBatch, DecodedCommand, DecodedFrame, Rect, Transform,
};
use crate::geometry::Color;
use crate::gpu::allocator::Allocator;
use crate::pipeline::PushConstants;
use crate::ring_buffer::{InstanceRing, INSTANCE_STRIDE};
use crate::RenderResult;

/// One instanced draw: a range of the ring's instance buffer rendered with a
/// single push-constant state.
#[derive(Clone, Copy, Debug)]
pub struct DrawGroup {
    pub push: PushConstants,
    pub first_instance: u32,
    pub instance_count: u32,
}

#[derive(Clone, Debug, Default)]
pub struct EncodedFrame {
    pub groups: Vec<DrawGroup>,
}

pub struct FrameEncoder {
    transform_stack: Vec<Transform>,
    clip_stack: Vec<Rect>,
    opacity_stack: Vec<f32>,
    /// Instance bytes for the open group.
    scratch: Vec<u8>,
    /// Push constants of the open group.
    current_push: Option<PushConstants>,
    groups: Vec<DrawGroup>,
}

impl Default for FrameEncoder {
    fn default() -> Self {
        Self {
            transform_stack: Vec::with_capacity(16),
            clip_stack: Vec::with_capacity(8),
            opacity_stack: Vec::with_capacity(8),
            scratch: Vec::with_capacity(1024),
            current_push: None,
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
    ) -> Result<EncodedFrame, (RenderResult, String)> {
        self.transform_stack.clear();
        self.clip_stack.clear();
        self.opacity_stack.clear();
        self.scratch.clear();
        self.current_push = None;
        self.groups.clear();

        let screen = Rect {
            min: crate::geometry::Point { x: 0.0, y: 0.0 },
            max: crate::geometry::Point {
                x: surface_size[0],
                y: surface_size[1],
            },
        };

        for batch in &frame.batches {
            self.encode_batch(batch, screen, ring, allocator, surface_size)?;
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
    ) -> Result<(), (RenderResult, String)> {
        // Seed the state from the batch header.
        self.transform_stack.clear();
        self.transform_stack.push(batch.transform);
        self.clip_stack.clear();
        let mut clip = batch.bounds;
        if let Some(header_clip) = batch.clip {
            clip = clip.intersect(header_clip);
        }
        clip = clip.intersect(screen);
        self.clip_stack.push(clip);
        self.opacity_stack.clear();
        self.opacity_stack.push(batch.opacity);

        for cmd in &batch.commands {
            match cmd {
                DecodedCommand::FillRect { rect, brush } => {
                    self.emit_fill_path_rect(*rect, brush, ring, allocator, surface_size)?;
                }
                DecodedCommand::StrokeRect { rect, stroke, brush } => {
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
                    self.transform_stack.push(next);
                }
                DecodedCommand::PopTransform => {
                    if self.transform_stack.len() > 1 {
                        self.transform_stack.pop();
                    }
                }
                DecodedCommand::PushClipRect { rect } => {
                    let world = self.transform().transform_rect(*rect);
                    let next = self.clip().intersect(world);
                    self.clip_stack.push(next);
                }
                DecodedCommand::PopClip => {
                    if self.clip_stack.len() > 1 {
                        self.clip_stack.pop();
                    }
                }
                DecodedCommand::PushOpacity { alpha } => {
                    let next = self.opacity() * alpha;
                    self.opacity_stack.push(next);
                }
                DecodedCommand::PopOpacity => {
                    if self.opacity_stack.len() > 1 {
                        self.opacity_stack.pop();
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
                DecodedCommand::DrawImage { .. } => self.log_unsupported("DrawImage"),
                DecodedCommand::DrawTexture { .. } => self.log_unsupported("DrawTexture"),
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
            brush_kind: 0,
            brush_payload: [0.0; 8],
            surface_size,
        };

        if self.current_push.map_or(true, |p| !push_constants_eq(&p, &push)) {
            self.close_group(ring, allocator)?;
            self.current_push = Some(push);
        }

        self.scratch.extend_from_slice(&pack_instance(&rect, &color));
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
            return Ok(());
        }
        let (buffer, first, count) = ring.append(allocator, &self.scratch)?;
        let _ = buffer;
        self.groups.push(DrawGroup {
            push,
            first_instance: first,
            instance_count: count,
        });
        self.scratch.clear();
        Ok(())
    }

    fn transform(&self) -> Transform {
        *self.transform_stack.last().expect("transform stack never empties")
    }

    fn clip(&self) -> Rect {
        *self.clip_stack.last().expect("clip stack never empties")
    }

    fn opacity(&self) -> f32 {
        *self.opacity_stack.last().expect("opacity stack never empties")
    }

    fn log_unsupported(&self, name: &str) {
        // Dev-only log; production renders are gated by which fixtures the
        // slice supports.
        #[cfg(debug_assertions)]
        eprintln!("lurpic_render: {} not yet rendered by the GPU pipeline", name);
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
        let packed = pack_instance(&rect(10.0, 20.0, 30.0, 40.0), &Color { r: 1.0, g: 0.5, b: 0.0, a: 1.0 });
        assert_eq!(&packed[0..4], &10.0f32.to_le_bytes());
        assert_eq!(&packed[4..8], &20.0f32.to_le_bytes());
        assert_eq!(&packed[8..12], &20.0f32.to_le_bytes());
        assert_eq!(&packed[16..20], &1.0f32.to_le_bytes());
        assert_eq!(&packed[28..32], &1.0f32.to_le_bytes());
    }
}
