use crate::tessellation::{self, Color, Path, Point, Verb, Vertex};
use crate::RenderResult;

const FRAME_MAGIC: &[u8; 4] = b"LPVF";
const FRAME_VERSION: u32 = 1;

const CMD_FILL_RECT: u8 = 0;
const CMD_STROKE_RECT: u8 = 1;
const CMD_FILL_PATH: u8 = 2;
const CMD_STROKE_PATH: u8 = 3;
const CMD_DRAW_POLYLINE: u8 = 4;
const CMD_DRAW_POINTS: u8 = 5;
const CMD_DRAW_SELECTION_RECTS: u8 = 6;
const CMD_PUSH_TRANSFORM: u8 = 7;
const CMD_POP_TRANSFORM: u8 = 8;
const CMD_PUSH_CLIP_RECT: u8 = 9;
const CMD_POP_CLIP: u8 = 10;
const CMD_PUSH_OPACITY: u8 = 11;
const CMD_POP_OPACITY: u8 = 12;
const CMD_DRAW_GLYPH_RUN: u8 = 13;
const CMD_DRAW_IMAGE: u8 = 14;

#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct FrameStats {
    pub batch_count: usize,
    pub command_count: usize,
    pub vertex_count: usize,
}

#[derive(Clone, Debug)]
pub struct DecodedFrame {
    pub stats: FrameStats,
    pub batches: Vec<DecodedBatch>,
}

#[derive(Clone, Debug)]
pub struct DecodedBatch {
    pub id: u64,
    pub vertices: Vec<Vertex>,
    pub text_runs: Vec<DecodedTextRun>,
    pub image_draws: Vec<DecodedImageDraw>,
    pub command_count: usize,
    pub opacity: f32,
    pub clip: Option<Rect>,
}

#[derive(Clone, Debug)]
pub struct DecodedTextRun {
    pub font_id: u64,
    pub size_bits: u32,
    pub origin: Point,
    pub color: Color,
    pub glyphs: Vec<DecodedGlyph>,
}

#[derive(Clone, Debug)]
pub struct DecodedGlyph {
    pub glyph_id: u32,
    pub x: f32,
    pub y: f32,
}

#[derive(Clone, Debug)]
pub struct DecodedImageDraw {
    pub handle: u64,
    pub dest: Rect,
    pub src: Rect,
    pub sampling: u8,
    pub opacity: f32,
}

pub fn decode_frame(data: &[u8]) -> Result<DecodedFrame, (RenderResult, String)> {
    let mut reader = Reader::new(data);
    reader.expect_magic(FRAME_MAGIC)?;
    let version = reader.read_u32()?;
    if version != FRAME_VERSION {
        return Err((
            RenderResult::InitFailed,
            format!("unsupported frame packet version {}", version),
        ));
    }

    let batch_count = reader.read_u32()? as usize;
    let mut batches = Vec::with_capacity(batch_count);
    let mut stats = FrameStats::default();

    for _ in 0..batch_count {
        let id = reader.read_u64()?;
        let bounds = reader.read_rect()?;
        let opacity = reader.read_f32()?;
        let command_count = reader.read_u32()? as usize;
        stats.batch_count += 1;
        stats.command_count += command_count;

        let mut state = DecodeState::new(bounds, opacity);
        let mut vertices = Vec::new();
        let mut text_runs = Vec::new();
        let mut image_draws = Vec::new();
        for _ in 0..command_count {
            match reader.read_u8()? {
                CMD_FILL_RECT => {
                    let rect = reader.read_rect()?;
                    let color = reader.read_color()?;
                    append_vertices(
                        &mut vertices,
                        tessellation::tessellate_fill(
                            &Path::rect(
                                rect.min.x,
                                rect.min.y,
                                rect.max.x - rect.min.x,
                                rect.max.y - rect.min.y,
                            ),
                            color,
                        ),
                        state.transform,
                    );
                }
                CMD_STROKE_RECT => {
                    let rect = reader.read_rect()?;
                    let width = reader.read_f32()?;
                    let color = reader.read_color()?;
                    append_vertices(
                        &mut vertices,
                        tessellation::tessellate_stroke(
                            &Path::rect(
                                rect.min.x,
                                rect.min.y,
                                rect.max.x - rect.min.x,
                                rect.max.y - rect.min.y,
                            ),
                            width,
                            color,
                        ),
                        state.transform,
                    );
                }
                CMD_FILL_PATH => {
                    let path = reader.read_path()?;
                    let color = reader.read_color()?;
                    append_vertices(
                        &mut vertices,
                        tessellation::tessellate_fill(&path, color),
                        state.transform,
                    );
                }
                CMD_STROKE_PATH => {
                    let path = reader.read_path()?;
                    let width = reader.read_f32()?;
                    let color = reader.read_color()?;
                    append_vertices(
                        &mut vertices,
                        tessellation::tessellate_stroke(&path, width, color),
                        state.transform,
                    );
                }
                CMD_DRAW_POLYLINE => {
                    let closed = reader.read_u8()? != 0;
                    let width = reader.read_f32()?;
                    let points = reader.read_points()?;
                    let color = reader.read_color()?;
                    let path = Path::polyline(&points, closed);
                    append_vertices(
                        &mut vertices,
                        tessellation::tessellate_stroke(&path, width, color),
                        state.transform,
                    );
                }
                CMD_DRAW_POINTS => {
                    let radius = reader.read_f32()?;
                    let points = reader.read_points()?;
                    let color = reader.read_color()?;
                    for point in points {
                        let path = Path::circle(point.x, point.y, radius);
                        append_vertices(
                            &mut vertices,
                            tessellation::tessellate_fill(&path, color),
                            state.transform,
                        );
                    }
                }
                CMD_DRAW_SELECTION_RECTS => {
                    let rects = reader.read_rects()?;
                    let color = reader.read_color()?;
                    for rect in rects {
                        let path = Path::rect(
                            rect.min.x,
                            rect.min.y,
                            rect.max.x - rect.min.x,
                            rect.max.y - rect.min.y,
                        );
                        append_vertices(
                            &mut vertices,
                            tessellation::tessellate_fill(&path, color),
                            state.transform,
                        );
                    }
                }
                CMD_PUSH_TRANSFORM => {
                    let matrix = reader.read_transform()?;
                    state.push_transform(matrix);
                }
                CMD_POP_TRANSFORM => {
                    state.pop_transform();
                }
                CMD_PUSH_CLIP_RECT => {
                    let rect = reader.read_rect()?;
                    state.push_clip_rect(rect);
                }
                CMD_POP_CLIP => {
                    state.pop_clip();
                }
                CMD_PUSH_OPACITY => {
                    let alpha = reader.read_f32()?;
                    state.push_opacity(alpha);
                }
                CMD_POP_OPACITY => {
                    state.pop_opacity();
                }
                CMD_DRAW_GLYPH_RUN => {
                    let font_id = reader.read_u64()?;
                    let size_bits = reader.read_u32()?;
                    let origin = reader.read_point()?;
                    let color = reader.read_color()?;
                    let glyph_count = reader.read_u32()? as usize;
                    let mut glyphs = Vec::with_capacity(glyph_count);
                    for _ in 0..glyph_count {
                        glyphs.push(DecodedGlyph {
                            glyph_id: reader.read_u32()?,
                            x: reader.read_f32()?,
                            y: reader.read_f32()?,
                        });
                    }
                    text_runs.push(DecodedTextRun {
                        font_id,
                        size_bits,
                        origin,
                        color,
                        glyphs,
                    });
                }
                CMD_DRAW_IMAGE => {
                    image_draws.push(DecodedImageDraw {
                        handle: reader.read_u64()?,
                        dest: reader.read_rect()?,
                        src: reader.read_rect()?,
                        sampling: reader.read_u8()?,
                        opacity: reader.read_f32()?,
                    });
                }
                opcode => {
                    return Err((
                        RenderResult::InitFailed,
                        format!("unsupported frame packet opcode {}", opcode),
                    ));
                }
            }
        }

        stats.vertex_count += vertices.len();
        batches.push(DecodedBatch {
            id,
            vertices,
            text_runs,
            image_draws,
            command_count,
            opacity: state.opacity,
            clip: state.clip,
        });
    }

    if !reader.is_finished() {
        return Err((
            RenderResult::InitFailed,
            format!("frame packet has {} trailing bytes", reader.remaining()),
        ));
    }

    Ok(DecodedFrame { stats, batches })
}

fn append_vertices(out: &mut Vec<Vertex>, verts: Vec<Vertex>, transform: Transform) {
    out.reserve(verts.len());
    for mut v in verts {
        v.pos = transform.apply_point(v.pos);
        out.push(v);
    }
}

#[derive(Clone, Copy, Debug)]
struct Transform {
    a: f32,
    b: f32,
    c: f32,
    d: f32,
    tx: f32,
    ty: f32,
}

impl Transform {
    fn identity() -> Self {
        Self {
            a: 1.0,
            d: 1.0,
            b: 0.0,
            c: 0.0,
            tx: 0.0,
            ty: 0.0,
        }
    }

    fn multiply(self, other: Self) -> Self {
        Self {
            a: self.a * other.a + self.b * other.c,
            b: self.a * other.b + self.b * other.d,
            c: self.c * other.a + self.d * other.c,
            d: self.c * other.b + self.d * other.d,
            tx: self.a * other.tx + self.b * other.ty + self.tx,
            ty: self.c * other.tx + self.d * other.ty + self.ty,
        }
    }

    fn apply_point(self, p: Point) -> Point {
        Point {
            x: self.a * p.x + self.b * p.y + self.tx,
            y: self.c * p.x + self.d * p.y + self.ty,
        }
    }

    fn from_parts(a: f32, b: f32, c: f32, d: f32, tx: f32, ty: f32) -> Self {
        Self { a, b, c, d, tx, ty }
    }
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
}

struct DecodeState {
    transform_stack: Vec<Transform>,
    clip_stack: Vec<Rect>,
    opacity_stack: Vec<f32>,
    pub transform: Transform,
    pub clip: Option<Rect>,
    pub opacity: f32,
}

impl DecodeState {
    fn new(bounds: Rect, opacity: f32) -> Self {
        Self {
            transform_stack: Vec::new(),
            clip_stack: vec![bounds],
            opacity_stack: Vec::new(),
            transform: Transform::identity(),
            clip: Some(bounds),
            opacity,
        }
    }

    fn push_transform(&mut self, matrix: Transform) {
        self.transform_stack.push(self.transform);
        self.transform = self.transform.multiply(matrix);
    }

    fn pop_transform(&mut self) {
        if let Some(prev) = self.transform_stack.pop() {
            self.transform = prev;
        }
    }

    fn push_clip_rect(&mut self, rect: Rect) {
        let rect = self.transform.transform_rect(rect);
        let next = match self.clip {
            Some(current) => current.intersect(rect),
            None => rect,
        };
        self.clip_stack.push(next);
        self.clip = Some(next);
    }

    fn pop_clip(&mut self) {
        if self.clip_stack.pop().is_some() {
            self.clip = self.clip_stack.last().copied();
        }
    }

    fn push_opacity(&mut self, alpha: f32) {
        self.opacity_stack.push(self.opacity);
        self.opacity *= alpha;
    }

    fn pop_opacity(&mut self) {
        if let Some(prev) = self.opacity_stack.pop() {
            self.opacity = prev;
        }
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

    fn read_color(&mut self) -> Result<Color, (RenderResult, String)> {
        Ok(Color {
            r: self.read_f32()?,
            g: self.read_f32()?,
            b: self.read_f32()?,
            a: self.read_f32()?,
        })
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
        let segment_count = self.read_u32()? as usize;
        let mut verbs = Vec::with_capacity(segment_count);
        for _ in 0..segment_count {
            let verb = self.read_u8()?;
            let path_verb = match verb {
                0 => Verb::MoveTo(self.read_point()?),
                1 => Verb::LineTo(self.read_point()?),
                2 => Verb::QuadTo(self.read_point()?, self.read_point()?),
                3 => Verb::CubicTo(self.read_point()?, self.read_point()?, self.read_point()?),
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
        Ok(Path { verbs })
    }
}

impl Transform {
    fn transform_rect(self, rect: Rect) -> Rect {
        let points = [
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
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write_u8(out: &mut Vec<u8>, value: u8) {
        out.push(value);
    }

    fn write_u32(out: &mut Vec<u8>, value: u32) {
        out.extend_from_slice(&value.to_le_bytes());
    }

    fn write_u64(out: &mut Vec<u8>, value: u64) {
        out.extend_from_slice(&value.to_le_bytes());
    }

    fn write_f32(out: &mut Vec<u8>, value: f32) {
        out.extend_from_slice(&value.to_le_bytes());
    }

    fn write_point(out: &mut Vec<u8>, p: Point) {
        write_f32(out, p.x);
        write_f32(out, p.y);
    }

    fn write_rect(out: &mut Vec<u8>, min: Point, max: Point) {
        write_point(out, min);
        write_point(out, max);
    }

    fn write_color(out: &mut Vec<u8>, color: Color) {
        write_f32(out, color.r);
        write_f32(out, color.g);
        write_f32(out, color.b);
        write_f32(out, color.a);
    }

    fn write_path(out: &mut Vec<u8>, path: &Path) {
        write_u32(out, path.verbs.len() as u32);
        for verb in &path.verbs {
            match *verb {
                Verb::MoveTo(p) => {
                    write_u8(out, 0);
                    write_point(out, p);
                }
                Verb::LineTo(p) => {
                    write_u8(out, 1);
                    write_point(out, p);
                }
                Verb::QuadTo(c, p) => {
                    write_u8(out, 2);
                    write_point(out, c);
                    write_point(out, p);
                }
                Verb::CubicTo(c1, c2, p) => {
                    write_u8(out, 3);
                    write_point(out, c1);
                    write_point(out, c2);
                    write_point(out, p);
                }
                Verb::Close => {
                    write_u8(out, 4);
                }
            }
        }
    }

    #[test]
    fn decode_rect_frame_produces_vertices() {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(FRAME_MAGIC);
        write_u32(&mut bytes, FRAME_VERSION);
        write_u32(&mut bytes, 1);
        write_u64(&mut bytes, 7);
        write_rect(
            &mut bytes,
            Point { x: 0.0, y: 0.0 },
            Point { x: 10.0, y: 10.0 },
        );
        write_f32(&mut bytes, 1.0);
        write_u32(&mut bytes, 1);
        write_u8(&mut bytes, CMD_FILL_RECT);
        write_rect(
            &mut bytes,
            Point { x: 0.0, y: 0.0 },
            Point { x: 10.0, y: 10.0 },
        );
        write_color(
            &mut bytes,
            Color {
                r: 1.0,
                g: 0.0,
                b: 0.0,
                a: 1.0,
            },
        );

        let frame = decode_frame(&bytes).expect("frame decodes");
        assert_eq!(frame.stats.batch_count, 1);
        assert_eq!(frame.stats.command_count, 1);
        assert_eq!(frame.stats.vertex_count, 6);
        assert_eq!(frame.batches[0].id, 7);
        assert_eq!(frame.batches[0].command_count, 1);
        assert_eq!(frame.batches[0].vertices.len(), 6);
    }

    #[test]
    fn decode_path_frame_handles_transform_stack() {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(FRAME_MAGIC);
        write_u32(&mut bytes, FRAME_VERSION);
        write_u32(&mut bytes, 1);
        write_u64(&mut bytes, 1);
        write_rect(
            &mut bytes,
            Point { x: 0.0, y: 0.0 },
            Point { x: 20.0, y: 20.0 },
        );
        write_f32(&mut bytes, 1.0);
        write_u32(&mut bytes, 2);
        write_u8(&mut bytes, CMD_PUSH_TRANSFORM);
        write_f32(&mut bytes, 1.0);
        write_f32(&mut bytes, 0.0);
        write_f32(&mut bytes, 0.0);
        write_f32(&mut bytes, 1.0);
        write_f32(&mut bytes, 5.0);
        write_f32(&mut bytes, 5.0);
        write_u8(&mut bytes, CMD_FILL_PATH);
        write_path(&mut bytes, &Path::rect(0.0, 0.0, 10.0, 10.0));
        write_color(
            &mut bytes,
            Color {
                r: 0.0,
                g: 1.0,
                b: 0.0,
                a: 1.0,
            },
        );

        let frame = decode_frame(&bytes).expect("frame decodes");
        assert_eq!(frame.stats.vertex_count, 6);
        assert!(
            frame.batches[0]
                .vertices
                .iter()
                .any(|vertex| (vertex.pos.x - 5.0).abs() < 0.001
                    && (vertex.pos.y - 5.0).abs() < 0.001)
        );
    }

    #[test]
    fn decode_glyph_run() {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(FRAME_MAGIC);
        write_u32(&mut bytes, FRAME_VERSION);
        write_u32(&mut bytes, 1);
        write_u64(&mut bytes, 1);
        write_rect(
            &mut bytes,
            Point { x: 0.0, y: 0.0 },
            Point { x: 20.0, y: 20.0 },
        );
        write_f32(&mut bytes, 1.0);
        write_u32(&mut bytes, 1);
        write_u8(&mut bytes, CMD_DRAW_GLYPH_RUN);
        write_u64(&mut bytes, 99);
        write_u32(&mut bytes, 16);
        write_point(&mut bytes, Point { x: 1.0, y: 2.0 });
        write_color(
            &mut bytes,
            Color {
                r: 1.0,
                g: 1.0,
                b: 1.0,
                a: 1.0,
            },
        );
        write_u32(&mut bytes, 1);
        write_u32(&mut bytes, 42);
        write_f32(&mut bytes, 3.0);
        write_f32(&mut bytes, 4.0);

        let frame = decode_frame(&bytes).expect("frame decodes");
        assert_eq!(frame.batches[0].text_runs.len(), 1);
        assert_eq!(frame.batches[0].text_runs[0].glyphs.len(), 1);
        assert_eq!(frame.batches[0].text_runs[0].font_id, 99);
    }

    #[test]
    fn decode_image_draw() {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(FRAME_MAGIC);
        write_u32(&mut bytes, FRAME_VERSION);
        write_u32(&mut bytes, 1);
        write_u64(&mut bytes, 1);
        write_rect(
            &mut bytes,
            Point { x: 0.0, y: 0.0 },
            Point { x: 20.0, y: 20.0 },
        );
        write_f32(&mut bytes, 1.0);
        write_u32(&mut bytes, 1);
        write_u8(&mut bytes, CMD_DRAW_IMAGE);
        write_u64(&mut bytes, 55);
        write_rect(
            &mut bytes,
            Point { x: 1.0, y: 2.0 },
            Point { x: 3.0, y: 4.0 },
        );
        write_rect(
            &mut bytes,
            Point { x: 0.0, y: 0.0 },
            Point { x: 5.0, y: 6.0 },
        );
        write_u8(&mut bytes, 1);
        write_f32(&mut bytes, 0.5);

        let frame = decode_frame(&bytes).expect("frame decodes");
        assert_eq!(frame.batches[0].image_draws.len(), 1);
        assert_eq!(frame.batches[0].image_draws[0].handle, 55);
    }
}
