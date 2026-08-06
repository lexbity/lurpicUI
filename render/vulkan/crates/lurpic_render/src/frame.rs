use crate::geometry::{Color, Path, Point, Verb};
use crate::RenderResult;

/// Frame packet wire format (version 2). Little-endian, hand-packed and
/// mirrored by `render/vulkan/packet.go`.
///
/// ```text
/// Frame packet:
///   magic          [4]u8      = "LPVF"
///   version        u32        = 2
///   surface_w      u32
///   surface_h      u32
///   device_pixel_ratio f32
///   batch_count    u32
///   batches        [batch_count]Batch
///
/// Batch:
///   batch_id       u64
///   bounds         Rect (4 x f32)
///   opacity        f32
///   transform      [6]f32     // 2x3 affine, NOT pre-baked into vertices
///   clip_rect      Rect       // batch-local; (0,0,0,0) = no clip
///   command_count  u32
///   commands       [command_count]Command
/// ```
pub const FRAME_MAGIC: &[u8; 4] = b"LPVF";
pub const FRAME_VERSION: u32 = 2;

pub const CMD_FILL_RECT: u8 = 0;
pub const CMD_STROKE_RECT: u8 = 1;
pub const CMD_FILL_PATH: u8 = 2;
pub const CMD_STROKE_PATH: u8 = 3;
pub const CMD_DRAW_POLYLINE: u8 = 4;
pub const CMD_DRAW_POINTS: u8 = 5;
pub const CMD_DRAW_SELECTION_RECTS: u8 = 6;
pub const CMD_PUSH_TRANSFORM: u8 = 7;
pub const CMD_POP_TRANSFORM: u8 = 8;
pub const CMD_PUSH_CLIP_RECT: u8 = 9;
pub const CMD_POP_CLIP: u8 = 10;
pub const CMD_PUSH_OPACITY: u8 = 11;
pub const CMD_POP_OPACITY: u8 = 12;
pub const CMD_DRAW_GLYPH_RUN: u8 = 13;
pub const CMD_DRAW_IMAGE: u8 = 14;
pub const CMD_DRAW_TEXTURE: u8 = 15;
pub const CMD_DRAW_BLURRED_SHADOW: u8 = 16;
pub const CMD_BEGIN_RENDER_BATCH: u8 = 17;
pub const CMD_END_RENDER_BATCH: u8 = 18;

pub const BRUSH_SOLID: u8 = 0;
pub const BRUSH_LINEAR_GRADIENT: u8 = 1;

#[cfg(feature = "cpu-fallback")]
use std::sync::atomic::{AtomicUsize, Ordering};

#[cfg(feature = "cpu-fallback")]
static LAST_VERTEX_COUNT: AtomicUsize = AtomicUsize::new(0);

/// Records the number of vertices the CPU raster adapter produced for the most
/// recent frame. Populated at raster time; surfaced through `FrameStats`.
#[cfg(feature = "cpu-fallback")]
pub fn record_vertex_count(count: usize) {
    LAST_VERTEX_COUNT.store(count, Ordering::Relaxed);
}

#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct FrameStats {
    pub batch_count: usize,
    pub command_count: usize,
    pub vertex_count: usize,
}

#[derive(Clone, Debug)]
pub struct DecodedFrame {
    // Read by the lurpic_render_test_last_{batch,command}_count FFIs
    // (test-exports); the production pipeline does not consume it.
    #[cfg_attr(not(feature = "test-exports"), allow(dead_code))]
    pub stats: FrameStats,
    // Surface metadata feeds the GPU pipeline (Slice 3+); the CPU stepping-stone
    // raster only needs the command stream.
    #[allow(dead_code)]
    pub surface_w: u32,
    #[allow(dead_code)]
    pub surface_h: u32,
    #[allow(dead_code)]
    pub device_pixel_ratio: f32,
    pub batches: Vec<DecodedBatch>,
}

#[derive(Clone, Debug)]
pub struct DecodedBatch {
    #[allow(dead_code)]
    pub id: u64,
    pub bounds: Rect,
    pub opacity: f32,
    /// Batch-level 2x3 affine transform. Applied by the CPU raster adapter at
    /// raster time (stepping stone); consumed as a push constant on the GPU path.
    pub transform: Transform,
    /// Batch-local clip rect from the layer stack; `None` when absent.
    pub clip: Option<Rect>,
    pub commands: Vec<DecodedCommand>,
    #[allow(dead_code)]
    pub command_count: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum BrushKind {
    Solid = 0,
    LinearGradient = 1,
}

impl BrushKind {
    fn from_u8(value: u8) -> Result<Self, (RenderResult, String)> {
        match value {
            BRUSH_SOLID => Ok(BrushKind::Solid),
            BRUSH_LINEAR_GRADIENT => Ok(BrushKind::LinearGradient),
            _ => Err((
                RenderResult::InitFailed,
                format!("unsupported brush kind {}", value),
            )),
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct GradientStop {
    pub offset: f32,
    pub color: Color,
}

#[derive(Clone, Debug)]
pub struct Brush {
    pub kind: BrushKind,
    /// Solid brush color. Zero for gradient brushes.
    pub color: Color,
    // Gradient fields consumed by the gradient pipeline (Slice 6).
    #[allow(dead_code)]
    pub gradient_start: Point,
    #[allow(dead_code)]
    pub gradient_end: Point,
    #[allow(dead_code)]
    pub gradient_stops: Vec<GradientStop>,
}

impl Brush {
    pub fn solid(color: Color) -> Self {
        Self {
            kind: BrushKind::Solid,
            color,
            gradient_start: Point { x: 0.0, y: 0.0 },
            gradient_end: Point { x: 0.0, y: 0.0 },
            gradient_stops: Vec::new(),
        }
    }
}

#[derive(Clone, Debug, Default)]
pub struct StrokeStyle {
    pub width: f32,
    // Full stroke style is decoded without loss (FR-3). The CPU stepping-stone
    // raster honors width only; caps/joins/miter/dash are consumed by the GPU
    // stroke pipeline (Slice 8) via Go-side OffsetContour expansion.
    #[allow(dead_code)]
    pub cap: u8,
    #[allow(dead_code)]
    pub join: u8,
    #[allow(dead_code)]
    pub miter_limit: f32,
    #[allow(dead_code)]
    pub dash: Vec<f32>,
    #[allow(dead_code)]
    pub dash_offset: f32,
}

#[derive(Clone, Debug)]
#[allow(dead_code)] // consumed by the glyph pipeline (Slice 5)
pub struct DecodedGlyph {
    pub glyph_id: u32,
    pub x: f32,
    pub y: f32,
}

#[derive(Clone, Debug)]
pub enum DecodedCommand {
    FillRect {
        rect: Rect,
        brush: Brush,
    },
    StrokeRect {
        rect: Rect,
        stroke: StrokeStyle,
        brush: Brush,
    },
    // Fields consumed by later slices (path fill: Slice 7, strokes: Slice 8,
    // points/selection: GPU raster follow-ons); decoded without loss (FR-3).
    #[allow(dead_code)]
    FillPath {
        path: Path,
        brush: Brush,
    },
    #[allow(dead_code)]
    StrokePath {
        path: Path,
        stroke: StrokeStyle,
        brush: Brush,
    },
    #[allow(dead_code)]
    DrawPolyline {
        points: Vec<Point>,
        stroke: StrokeStyle,
        brush: Brush,
        closed: bool,
    },
    #[allow(dead_code)]
    DrawPoints {
        points: Vec<Point>,
        radius: f32,
        brush: Brush,
    },
    #[allow(dead_code)]
    DrawSelectionRects {
        rects: Vec<Rect>,
        brush: Brush,
    },
    PushTransform {
        matrix: Transform,
    },
    PopTransform,
    PushClipRect {
        rect: Rect,
    },
    PopClip,
    PushOpacity {
        alpha: f32,
    },
    PopOpacity,
    #[allow(dead_code)] // consumed by the glyph pipeline (Slice 5)
    DrawGlyphRun {
        font_id: u64,
        size_bits: u32,
        origin: Point,
        glyphs: Vec<DecodedGlyph>,
        brush: Brush,
    },
    #[allow(dead_code)] // consumed by the texture pipeline (Slice 4)
    DrawImage {
        handle: u64,
        dest: Rect,
        src: Rect,
        sampling: u8,
        opacity: f32,
    },
    // Decoded without loss (FR-3); rendered by the GPU texture pipeline (Slice 4).
    #[allow(dead_code)]
    DrawTexture {
        handle: u64,
        dest: Rect,
        src: Rect,
        sampling: u8,
        opacity: f32,
    },
    // Decoded without loss (FR-3); rendered by the GPU blur pipeline (Slice 9).
    #[allow(dead_code)]
    DrawBlurredShadow {
        path: Path,
        color: Color,
        blur_radius: f32,
        offset: Point,
        inner: bool,
    },
    // Batch control opcodes; consumed by the GPU encoder (Slice 3+).
    #[allow(dead_code)]
    BeginRenderBatch {
        bounds: Rect,
        cache_id: u64,
    },
    #[allow(dead_code)]
    EndRenderBatch,
}

pub fn decode_frame(data: &[u8]) -> Result<DecodedFrame, (RenderResult, String)> {
    let mut reader = Reader::new(data);
    reader.expect_magic(FRAME_MAGIC)?;
    let version = reader.read_u32()?;
    if version != FRAME_VERSION {
        return Err((
            RenderResult::PacketVersionMismatch,
            format!("unsupported frame packet version {}", version),
        ));
    }

    let surface_w = reader.read_u32()?;
    let surface_h = reader.read_u32()?;
    let device_pixel_ratio = reader.read_f32()?;

    let batch_count = reader.read_u32()? as usize;
    let mut batches = Vec::with_capacity(batch_count);
    let mut stats = FrameStats::default();

    for _ in 0..batch_count {
        let id = reader.read_u64()?;
        let bounds = reader.read_rect()?;
        let opacity = reader.read_f32()?;
        let transform = reader.read_transform()?;
        let clip_rect = reader.read_rect()?;
        let command_count = reader.read_u32()? as usize;
        stats.batch_count += 1;
        stats.command_count += command_count;

        let mut commands = Vec::with_capacity(command_count);
        for _ in 0..command_count {
            commands.push(reader.read_command()?);
        }

        let clip = if clip_rect.is_empty() {
            None
        } else {
            Some(clip_rect)
        };

        batches.push(DecodedBatch {
            id,
            bounds,
            opacity,
            transform,
            clip,
            commands,
            command_count,
        });
    }

    if !reader.is_finished() {
        return Err((
            RenderResult::InitFailed,
            format!("frame packet has {} trailing bytes", reader.remaining()),
        ));
    }

    Ok(DecodedFrame {
        stats,
        surface_w,
        surface_h,
        device_pixel_ratio,
        batches,
    })
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Rect {
    pub min: Point,
    pub max: Point,
}

impl Rect {
    pub fn is_empty(self) -> bool {
        self.max.x <= self.min.x || self.max.y <= self.min.y
    }

    /// A degenerate zero-area rect (used as a placeholder for fixed-size stacks
    /// whose members are only read below the active depth).
    pub fn zero() -> Self {
        Self {
            min: Point { x: 0.0, y: 0.0 },
            max: Point { x: 0.0, y: 0.0 },
        }
    }

    pub fn intersect(self, other: Rect) -> Rect {
        Rect {
            min: Point {
                x: self.min.x.max(other.min.x),
                y: self.min.y.max(other.min.y),
            },
            max: Point {
                x: self.max.x.min(other.max.x),
                y: self.max.y.min(other.max.y),
            },
        }
    }

    /// Shrinks (positive d) or grows (negative d) the rect on all sides.
    pub fn inset(self, d: f32) -> Rect {
        Rect {
            min: Point {
                x: self.min.x + d,
                y: self.min.y + d,
            },
            max: Point {
                x: self.max.x - d,
                y: self.max.y - d,
            },
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Transform {
    pub a: f32,
    pub b: f32,
    pub c: f32,
    pub d: f32,
    pub tx: f32,
    pub ty: f32,
}

impl Transform {
    #[allow(dead_code)] // used by tests and the GPU push-constant path
    pub fn identity() -> Self {
        Self {
            a: 1.0,
            d: 1.0,
            b: 0.0,
            c: 0.0,
            tx: 0.0,
            ty: 0.0,
        }
    }

    pub fn multiply(self, other: Self) -> Self {
        Self {
            a: self.a * other.a + self.b * other.c,
            b: self.a * other.b + self.b * other.d,
            c: self.c * other.a + self.d * other.c,
            d: self.c * other.b + self.d * other.d,
            tx: self.a * other.tx + self.b * other.ty + self.tx,
            ty: self.c * other.tx + self.d * other.ty + self.ty,
        }
    }

    pub fn apply_point(self, p: Point) -> Point {
        Point {
            x: self.a * p.x + self.b * p.y + self.tx,
            y: self.c * p.x + self.d * p.y + self.ty,
        }
    }

    /// The 2x3 affine packed as [a, b, c, d, tx, ty] for the shader.
    pub fn to_array(self) -> [f32; 6] {
        [self.a, self.b, self.c, self.d, self.tx, self.ty]
    }

    pub fn transform_rect(self, rect: Rect) -> Rect {        let points = [
            self.apply_point(rect.min),
            self.apply_point(Point {
                x: rect.max.x,
                y: rect.min.y,
            }),
            self.apply_point(Point {
                x: rect.min.x,
                y: rect.max.y,
            }),
            self.apply_point(rect.max),
        ];
        let mut min = points[0];
        let mut max = points[0];
        for p in points.iter().copied().skip(1) {
            if p.x < min.x {
                min.x = p.x;
            }
            if p.y < min.y {
                min.y = p.y;
            }
            if p.x > max.x {
                max.x = p.x;
            }
            if p.y > max.y {
                max.y = p.y;
            }
        }
        Rect { min, max }
    }

    pub(crate) fn from_parts(a: f32, b: f32, c: f32, d: f32, tx: f32, ty: f32) -> Self {
        Self { a, b, c, d, tx, ty }
    }
}

struct Reader<'a> {
    data: &'a [u8],
    pos: usize,
}

impl<'a> Reader<'a> {
    fn new(data: &'a [u8]) -> Self {
        Self { data, pos: 0 }
    }

    fn is_finished(&self) -> bool {
        self.pos == self.data.len()
    }

    fn remaining(&self) -> usize {
        self.data.len().saturating_sub(self.pos)
    }

    fn expect_magic(&mut self, magic: &[u8; 4]) -> Result<(), (RenderResult, String)> {
        if self.remaining() < magic.len() {
            return Err((
                RenderResult::InitFailed,
                "frame packet is truncated".to_string(),
            ));
        }
        if &self.data[self.pos..self.pos + magic.len()] != magic {
            return Err((
                RenderResult::InitFailed,
                "frame packet magic mismatch".to_string(),
            ));
        }
        self.pos += magic.len();
        Ok(())
    }

    fn read_u8(&mut self) -> Result<u8, (RenderResult, String)> {
        self.read_exact(1).map(|bytes| bytes[0])
    }

    fn read_u32(&mut self) -> Result<u32, (RenderResult, String)> {
        let mut bytes = [0u8; 4];
        bytes.copy_from_slice(self.read_exact(4)?);
        Ok(u32::from_le_bytes(bytes))
    }

    fn read_u64(&mut self) -> Result<u64, (RenderResult, String)> {
        let mut bytes = [0u8; 8];
        bytes.copy_from_slice(self.read_exact(8)?);
        Ok(u64::from_le_bytes(bytes))
    }

    fn read_f32(&mut self) -> Result<f32, (RenderResult, String)> {
        let mut bytes = [0u8; 4];
        bytes.copy_from_slice(self.read_exact(4)?);
        Ok(f32::from_le_bytes(bytes))
    }

    fn read_exact(&mut self, len: usize) -> Result<&'a [u8], (RenderResult, String)> {
        if self.remaining() < len {
            return Err((
                RenderResult::InitFailed,
                "frame packet is truncated".to_string(),
            ));
        }
        let start = self.pos;
        self.pos += len;
        Ok(&self.data[start..start + len])
    }

    fn read_point(&mut self) -> Result<Point, (RenderResult, String)> {
        Ok(Point {
            x: self.read_f32()?,
            y: self.read_f32()?,
        })
    }

    fn read_rect(&mut self) -> Result<Rect, (RenderResult, String)> {
        Ok(Rect {
            min: self.read_point()?,
            max: self.read_point()?,
        })
    }

    fn read_color_u8(&mut self) -> Result<Color, (RenderResult, String)> {
        let r = self.read_u8()? as f32 / 255.0;
        let g = self.read_u8()? as f32 / 255.0;
        let b = self.read_u8()? as f32 / 255.0;
        let a = self.read_u8()? as f32 / 255.0;
        Ok(Color { r, g, b, a })
    }

    fn read_transform(&mut self) -> Result<Transform, (RenderResult, String)> {
        Ok(Transform::from_parts(
            self.read_f32()?,
            self.read_f32()?,
            self.read_f32()?,
            self.read_f32()?,
            self.read_f32()?,
            self.read_f32()?,
        ))
    }

    fn read_points(&mut self) -> Result<Vec<Point>, (RenderResult, String)> {
        let count = self.read_u32()? as usize;
        let mut points = Vec::with_capacity(count);
        for _ in 0..count {
            points.push(self.read_point()?);
        }
        Ok(points)
    }

    fn read_rects(&mut self) -> Result<Vec<Rect>, (RenderResult, String)> {
        let count = self.read_u32()? as usize;
        let mut rects = Vec::with_capacity(count);
        for _ in 0..count {
            rects.push(self.read_rect()?);
        }
        Ok(rects)
    }

    fn read_path(&mut self) -> Result<Path, (RenderResult, String)> {
        let verb_count = self.read_u32()? as usize;
        let mut verbs = Vec::with_capacity(verb_count);
        for _ in 0..verb_count {
            let verb = self.read_u8()?;
            let path_verb = match verb {
                0 => Verb::MoveTo(Point { x: 0.0, y: 0.0 }),
                1 => Verb::LineTo(Point { x: 0.0, y: 0.0 }),
                2 => Verb::QuadTo(Point { x: 0.0, y: 0.0 }, Point { x: 0.0, y: 0.0 }),
                3 => Verb::CubicTo(
                    Point { x: 0.0, y: 0.0 },
                    Point { x: 0.0, y: 0.0 },
                    Point { x: 0.0, y: 0.0 },
                ),
                4 => Verb::Close,
                _ => {
                    return Err((
                        RenderResult::InitFailed,
                        format!("unknown path verb {}", verb),
                    ))
                }
            };
            verbs.push(path_verb);
        }

        let point_count = self.read_u32()? as usize;
        let mut points = Vec::with_capacity(point_count);
        for _ in 0..point_count {
            points.push(self.read_point()?);
        }

        let mut points_iter = points.into_iter();
        for verb in &mut verbs {
            match verb {
                Verb::MoveTo(p) => {
                    *p = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                }
                Verb::LineTo(p) => {
                    *p = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                }
                Verb::QuadTo(a, b) => {
                    *a = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                    *b = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                }
                Verb::CubicTo(a, b, c) => {
                    *a = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                    *b = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                    *c = points_iter
                        .next()
                        .ok_or_else(|| path_points_short(verb_count, point_count))?;
                }
                Verb::Close => {}
            }
        }
        Ok(Path { verbs })
    }

    fn read_brush(&mut self) -> Result<Brush, (RenderResult, String)> {
        let kind = BrushKind::from_u8(self.read_u8()?)?;
        match kind {
            BrushKind::Solid => Ok(Brush::solid(self.read_color_u8()?)),
            BrushKind::LinearGradient => {
                let start = self.read_point()?;
                let end = self.read_point()?;
                let stop_count = self.read_u32()? as usize;
                let mut stops = Vec::with_capacity(stop_count);
                for _ in 0..stop_count {
                    let offset = self.read_f32()?;
                    let color = self.read_color_u8()?;
                    stops.push(GradientStop { offset, color });
                }
                Ok(Brush {
                    kind,
                    color: Color::default(),
                    gradient_start: start,
                    gradient_end: end,
                    gradient_stops: stops,
                })
            }
        }
    }

    fn read_stroke_style(&mut self) -> Result<StrokeStyle, (RenderResult, String)> {
        let width = self.read_f32()?;
        let cap = self.read_u8()?;
        let join = self.read_u8()?;
        let miter_limit = self.read_f32()?;
        let dash_count = self.read_u32()? as usize;
        let mut dash = Vec::with_capacity(dash_count);
        for _ in 0..dash_count {
            dash.push(self.read_f32()?);
        }
        let dash_offset = self.read_f32()?;
        Ok(StrokeStyle {
            width,
            cap,
            join,
            miter_limit,
            dash,
            dash_offset,
        })
    }

    fn read_glyphs(&mut self) -> Result<Vec<DecodedGlyph>, (RenderResult, String)> {
        let count = self.read_u32()? as usize;
        let mut glyphs = Vec::with_capacity(count);
        for _ in 0..count {
            glyphs.push(DecodedGlyph {
                glyph_id: self.read_u32()?,
                x: self.read_f32()?,
                y: self.read_f32()?,
            });
        }
        Ok(glyphs)
    }

    fn read_command(&mut self) -> Result<DecodedCommand, (RenderResult, String)> {
        match self.read_u8()? {
            CMD_FILL_RECT => Ok(DecodedCommand::FillRect {
                rect: self.read_rect()?,
                brush: self.read_brush()?,
            }),
            CMD_STROKE_RECT => Ok(DecodedCommand::StrokeRect {
                rect: self.read_rect()?,
                stroke: self.read_stroke_style()?,
                brush: self.read_brush()?,
            }),
            CMD_FILL_PATH => Ok(DecodedCommand::FillPath {
                path: self.read_path()?,
                brush: self.read_brush()?,
            }),
            CMD_STROKE_PATH => Ok(DecodedCommand::StrokePath {
                path: self.read_path()?,
                stroke: self.read_stroke_style()?,
                brush: self.read_brush()?,
            }),
            CMD_DRAW_POLYLINE => Ok(DecodedCommand::DrawPolyline {
                points: self.read_points()?,
                stroke: self.read_stroke_style()?,
                brush: self.read_brush()?,
                closed: self.read_u8()? != 0,
            }),
            CMD_DRAW_POINTS => Ok(DecodedCommand::DrawPoints {
                points: self.read_points()?,
                radius: self.read_f32()?,
                brush: self.read_brush()?,
            }),
            CMD_DRAW_SELECTION_RECTS => Ok(DecodedCommand::DrawSelectionRects {
                rects: self.read_rects()?,
                brush: self.read_brush()?,
            }),
            CMD_PUSH_TRANSFORM => Ok(DecodedCommand::PushTransform {
                matrix: self.read_transform()?,
            }),
            CMD_POP_TRANSFORM => Ok(DecodedCommand::PopTransform),
            CMD_PUSH_CLIP_RECT => Ok(DecodedCommand::PushClipRect {
                rect: self.read_rect()?,
            }),
            CMD_POP_CLIP => Ok(DecodedCommand::PopClip),
            CMD_PUSH_OPACITY => Ok(DecodedCommand::PushOpacity {
                alpha: self.read_f32()?,
            }),
            CMD_POP_OPACITY => Ok(DecodedCommand::PopOpacity),
            CMD_DRAW_GLYPH_RUN => {
                let font_id = self.read_u64()?;
                let size_bits = self.read_u32()?;
                let origin = self.read_point()?;
                let glyphs = self.read_glyphs()?;
                let brush = self.read_brush()?;
                Ok(DecodedCommand::DrawGlyphRun {
                    font_id,
                    size_bits,
                    origin,
                    glyphs,
                    brush,
                })
            }
            CMD_DRAW_IMAGE => Ok(DecodedCommand::DrawImage {
                handle: self.read_u64()?,
                dest: self.read_rect()?,
                src: self.read_rect()?,
                sampling: self.read_u8()?,
                opacity: self.read_f32()?,
            }),
            CMD_DRAW_TEXTURE => Ok(DecodedCommand::DrawTexture {
                handle: self.read_u64()?,
                dest: self.read_rect()?,
                src: self.read_rect()?,
                sampling: self.read_u8()?,
                opacity: self.read_f32()?,
            }),
            CMD_DRAW_BLURRED_SHADOW => {
                let path = self.read_path()?;
                let color = self.read_color_u8()?;
                let blur_radius = self.read_f32()?;
                let offset = self.read_point()?;
                let inner = self.read_u8()? != 0;
                Ok(DecodedCommand::DrawBlurredShadow {
                    path,
                    color,
                    blur_radius,
                    offset,
                    inner,
                })
            }
            CMD_BEGIN_RENDER_BATCH => Ok(DecodedCommand::BeginRenderBatch {
                bounds: self.read_rect()?,
                cache_id: self.read_u64()?,
            }),
            CMD_END_RENDER_BATCH => Ok(DecodedCommand::EndRenderBatch),
            opcode => Err((
                RenderResult::InitFailed,
                format!("unsupported frame packet opcode {}", opcode),
            )),
        }
    }
}

fn path_points_short(verb_count: usize, point_count: usize) -> (RenderResult, String) {
    (
        RenderResult::InitFailed,
        format!(
            "path point array too short: {} verbs, {} points",
            verb_count, point_count
        ),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    struct PacketBuilder {
        bytes: Vec<u8>,
    }

    impl PacketBuilder {
        fn new() -> Self {
            Self { bytes: Vec::new() }
        }

        fn header(&mut self, batch_count: u32) -> &mut Self {
            self.bytes.extend_from_slice(FRAME_MAGIC);
            self.u32(FRAME_VERSION);
            self.u32(64);
            self.u32(64);
            self.f32(1.0);
            self.u32(batch_count);
            self
        }

        fn batch(&mut self, id: u64, bounds: (Point, Point), opacity: f32) -> &mut Self {
            self.u64(id);
            self.point(bounds.0);
            self.point(bounds.1);
            self.f32(opacity);
            self
        }

        fn batch_transform(&mut self, t: Transform) -> &mut Self {
            self.f32(t.a);
            self.f32(t.b);
            self.f32(t.c);
            self.f32(t.d);
            self.f32(t.tx);
            self.f32(t.ty);
            self
        }

        fn batch_clip(&mut self, clip: Option<Rect>) -> &mut Self {
            match clip {
                Some(r) => {
                    self.point(r.min);
                    self.point(r.max);
                }
                None => {
                    self.f32(0.0);
                    self.f32(0.0);
                    self.f32(0.0);
                    self.f32(0.0);
                }
            }
            self
        }

        fn command_count(&mut self, count: u32) -> &mut Self {
            self.u32(count);
            self
        }

        fn solid_brush(&mut self, color: Color) -> &mut Self {
            self.u8(BRUSH_SOLID);
            self.color(color);
            self
        }

        fn stroke_style(&mut self, style: &StrokeStyle) -> &mut Self {
            self.f32(style.width);
            self.u8(style.cap);
            self.u8(style.join);
            self.f32(style.miter_limit);
            self.u32(style.dash.len() as u32);
            for d in &style.dash {
                self.f32(*d);
            }
            self.f32(style.dash_offset);
            self
        }

        fn path(&mut self, path: &Path) -> &mut Self {
            let mut points: Vec<Point> = Vec::new();
            for verb in &path.verbs {
                match *verb {
                    Verb::MoveTo(p) => points.push(p),
                    Verb::LineTo(p) => points.push(p),
                    Verb::QuadTo(a, b) => {
                        points.push(a);
                        points.push(b);
                    }
                    Verb::CubicTo(a, b, c) => {
                        points.push(a);
                        points.push(b);
                        points.push(c);
                    }
                    Verb::Close => {}
                }
            }
            self.u32(path.verbs.len() as u32);
            for verb in &path.verbs {
                self.u8(match verb {
                    Verb::MoveTo(_) => 0,
                    Verb::LineTo(_) => 1,
                    Verb::QuadTo(_, _) => 2,
                    Verb::CubicTo(_, _, _) => 3,
                    Verb::Close => 4,
                });
            }
            self.u32(points.len() as u32);
            for p in points {
                self.point(p);
            }
            self
        }

        fn u8(&mut self, v: u8) -> &mut Self {
            self.bytes.push(v);
            self
        }

        fn u32(&mut self, v: u32) -> &mut Self {
            self.bytes.extend_from_slice(&v.to_le_bytes());
            self
        }

        fn u64(&mut self, v: u64) -> &mut Self {
            self.bytes.extend_from_slice(&v.to_le_bytes());
            self
        }

        fn f32(&mut self, v: f32) -> &mut Self {
            self.bytes.extend_from_slice(&v.to_le_bytes());
            self
        }

        fn point(&mut self, p: Point) -> &mut Self {
            self.f32(p.x);
            self.f32(p.y);
            self
        }

        fn color(&mut self, c: Color) -> &mut Self {
            self.u8((c.r.clamp(0.0, 1.0) * 255.0) as u8);
            self.u8((c.g.clamp(0.0, 1.0) * 255.0) as u8);
            self.u8((c.b.clamp(0.0, 1.0) * 255.0) as u8);
            self.u8((c.a.clamp(0.0, 1.0) * 255.0) as u8)
        }

        fn glyph_run(
            &mut self,
            font_id: u64,
            size_bits: u32,
            origin: Point,
            glyphs: &[DecodedGlyph],
        ) -> &mut Self {
            self.u64(font_id);
            self.u32(size_bits);
            self.point(origin);
            self.u32(glyphs.len() as u32);
            for g in glyphs {
                self.u32(g.glyph_id);
                self.f32(g.x);
                self.f32(g.y);
            }
            self
        }
    }

    fn rect(x0: f32, y0: f32, x1: f32, y1: f32) -> Rect {
        Rect {
            min: Point { x: x0, y: y0 },
            max: Point { x: x1, y: y1 },
        }
    }

    fn assert_ok(frame: Result<DecodedFrame, (RenderResult, String)>) -> DecodedFrame {
        match frame {
            Ok(f) => f,
            Err((code, msg)) => panic!("decode failed: {:?} {}", code, msg),
        }
    }

    #[test]
    fn rejects_version_1_packets() {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(FRAME_MAGIC);
        bytes.extend_from_slice(&1u32.to_le_bytes());
        bytes.extend_from_slice(&0u32.to_le_bytes());
        bytes.extend_from_slice(&0u32.to_le_bytes());
        bytes.extend_from_slice(&1.0f32.to_le_bytes());
        bytes.extend_from_slice(&0u32.to_le_bytes());
        let err = decode_frame(&bytes).expect_err("version 1 must be rejected");
        assert_eq!(err.0, RenderResult::PacketVersionMismatch);
    }

    #[test]
    fn decode_fill_rect_preserves_batch_transform() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(7, (rect(0.0, 0.0, 10.0, 10.0).min, rect(0.0, 0.0, 10.0, 10.0).max), 0.75)
            .batch_transform(Transform::from_parts(1.0, 0.0, 0.0, 1.0, 5.0, 5.0))
            .batch_clip(None)
            .command_count(1)
            .u8(CMD_FILL_RECT)
            .point(Point { x: 0.0, y: 0.0 })
            .point(Point { x: 10.0, y: 10.0 })
            .solid_brush(Color { r: 1.0, g: 0.0, b: 0.0, a: 1.0 });
        let bytes = binding.bytes;

        let frame = assert_ok(decode_frame(&bytes));
        assert_eq!(frame.stats.batch_count, 1);
        assert_eq!(frame.stats.command_count, 1);
        assert_eq!(frame.batches[0].id, 7);
        assert_eq!(frame.batches[0].transform.ty, 5.0);
        assert_eq!(frame.batches[0].opacity, 0.75);
        assert_eq!(frame.batches[0].clip, None);
        match &frame.batches[0].commands[0] {
            DecodedCommand::FillRect { rect, brush } => {
                assert_eq!(rect.max.x, 10.0);
                assert_eq!(brush.kind, BrushKind::Solid);
                assert!((brush.color.r - 1.0).abs() < 0.01);
            }
            other => panic!("unexpected command {:?}", other),
        }
    }

    #[test]
    fn decode_linear_gradient_brush() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(1, (rect(0.0, 0.0, 20.0, 20.0).min, rect(0.0, 0.0, 20.0, 20.0).max), 1.0)
            .batch_transform(Transform::identity())
            .batch_clip(Some(rect(1.0, 2.0, 19.0, 18.0)))
            .command_count(1)
            .u8(CMD_FILL_PATH)
            .path(&Path::rect(0.0, 0.0, 10.0, 10.0))
            .u8(BRUSH_LINEAR_GRADIENT)
            .point(Point { x: 0.0, y: 0.0 })
            .point(Point { x: 10.0, y: 0.0 })
            .u32(2)
            .f32(0.0)
            .color(Color { r: 1.0, g: 0.0, b: 0.0, a: 1.0 })
            .f32(1.0)
            .color(Color { r: 0.0, g: 0.0, b: 1.0, a: 1.0 });
        let bytes = binding.bytes;

        let frame = assert_ok(decode_frame(&bytes));
        assert_eq!(frame.batches[0].clip, Some(rect(1.0, 2.0, 19.0, 18.0)));
        match &frame.batches[0].commands[0] {
            DecodedCommand::FillPath { brush, .. } => {
                assert_eq!(brush.kind, BrushKind::LinearGradient);
                assert_eq!(brush.gradient_stops.len(), 2);
                assert_eq!(brush.gradient_stops[1].offset, 1.0);
            }
            other => panic!("unexpected command {:?}", other),
        }
    }

    #[test]
    fn decode_full_stroke_style() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(1, (rect(0.0, 0.0, 20.0, 20.0).min, rect(0.0, 0.0, 20.0, 20.0).max), 1.0)
            .batch_transform(Transform::identity())
            .batch_clip(None)
            .command_count(1)
            .u8(CMD_STROKE_PATH)
            .path(&Path::rect(0.0, 0.0, 10.0, 10.0))
            .stroke_style(&StrokeStyle {
                width: 3.0,
                cap: 2,
                join: 1,
                miter_limit: 4.5,
                dash: vec![4.0, 2.0],
                dash_offset: 1.5,
            })
            .solid_brush(Color { r: 0.0, g: 1.0, b: 0.0, a: 1.0 });
        let bytes = binding.bytes;

        let frame = assert_ok(decode_frame(&bytes));
        match &frame.batches[0].commands[0] {
            DecodedCommand::StrokePath { stroke, .. } => {
                assert_eq!(stroke.width, 3.0);
                assert_eq!(stroke.cap, 2);
                assert_eq!(stroke.join, 1);
                assert_eq!(stroke.miter_limit, 4.5);
                assert_eq!(stroke.dash, vec![4.0, 2.0]);
                assert_eq!(stroke.dash_offset, 1.5);
            }
            other => panic!("unexpected command {:?}", other),
        }
    }

    #[test]
    fn decode_draw_texture_and_blurred_shadow() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(1, (rect(0.0, 0.0, 64.0, 64.0).min, rect(0.0, 0.0, 64.0, 64.0).max), 1.0)
            .batch_transform(Transform::identity())
            .batch_clip(None)
            .command_count(2)
            .u8(CMD_DRAW_TEXTURE)
            .u64(99)
            .point(Point { x: 1.0, y: 2.0 })
            .point(Point { x: 3.0, y: 4.0 })
            .point(Point { x: 0.0, y: 0.0 })
            .point(Point { x: 1.0, y: 1.0 })
            .u8(1)
            .f32(0.5)
            .u8(CMD_DRAW_BLURRED_SHADOW)
            .path(&Path::rect(0.0, 0.0, 10.0, 10.0))
            .color(Color { r: 0.0, g: 0.0, b: 0.0, a: 0.5 })
            .f32(8.0)
            .point(Point { x: 2.0, y: 3.0 })
            .u8(0);
        let bytes = binding.bytes;

        let frame = assert_ok(decode_frame(&bytes));
        assert!(matches!(
            &frame.batches[0].commands[0],
            DecodedCommand::DrawTexture { handle: 99, sampling: 1, .. }
        ));
        match &frame.batches[0].commands[1] {
            DecodedCommand::DrawBlurredShadow {
                blur_radius,
                offset,
                inner,
                color,
                ..
            } => {
                assert_eq!(*blur_radius, 8.0);
                assert_eq!(offset.x, 2.0);
                assert!(!*inner);
                assert!((color.a - 0.5).abs() < 0.01);
            }
            other => panic!("unexpected command {:?}", other),
        }
    }

    #[test]
    fn decode_glyph_run_brush_after_glyphs() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(1, (rect(0.0, 0.0, 20.0, 20.0).min, rect(0.0, 0.0, 20.0, 20.0).max), 1.0)
            .batch_transform(Transform::identity())
            .batch_clip(None)
            .command_count(1)
            .u8(CMD_DRAW_GLYPH_RUN)
            .glyph_run(
                99,
                16,
                Point { x: 1.0, y: 2.0 },
                &[DecodedGlyph {
                    glyph_id: 42,
                    x: 3.0,
                    y: 4.0,
                }],
            )
            .solid_brush(Color { r: 1.0, g: 1.0, b: 1.0, a: 1.0 });
        let bytes = binding.bytes;

        let frame = assert_ok(decode_frame(&bytes));
        match &frame.batches[0].commands[0] {
            DecodedCommand::DrawGlyphRun {
                font_id,
                size_bits,
                origin,
                glyphs,
                brush,
            } => {
                assert_eq!(*font_id, 99);
                assert_eq!(*size_bits, 16);
                assert_eq!(origin.x, 1.0);
                assert_eq!(glyphs.len(), 1);
                assert_eq!(glyphs[0].glyph_id, 42);
                assert_eq!(brush.kind, BrushKind::Solid);
            }
            other => panic!("unexpected command {:?}", other),
        }
    }

    #[test]
    fn rejects_truncated_packet() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(1, (rect(0.0, 0.0, 20.0, 20.0).min, rect(0.0, 0.0, 20.0, 20.0).max), 1.0)
            .batch_transform(Transform::identity())
            .batch_clip(None)
            .command_count(1)
            .u8(CMD_FILL_RECT)
            .point(Point { x: 0.0, y: 0.0 });
        let bytes = binding.bytes;
        let err = decode_frame(&bytes).expect_err("truncated packet must fail");
        assert_eq!(err.0, RenderResult::InitFailed);
    }

    #[test]
    fn rejects_trailing_bytes() {
        let mut binding = PacketBuilder::new();
        binding
            .header(1)
            .batch(1, (rect(0.0, 0.0, 20.0, 20.0).min, rect(0.0, 0.0, 20.0, 20.0).max), 1.0)
            .batch_transform(Transform::identity())
            .batch_clip(None)
            .command_count(0);
        binding.bytes.push(0xFF);
        let err = decode_frame(&binding.bytes).expect_err("trailing bytes must fail");
        assert_eq!(err.0, RenderResult::InitFailed);
    }
}
