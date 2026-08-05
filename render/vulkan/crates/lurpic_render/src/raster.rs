use crate::atlas::{self, GlyphBitmap};
use crate::frame::{Brush, BrushKind, DecodedBatch, DecodedCommand, DecodedFrame, DecodedGlyph, Rect, Transform};
use crate::image_store;
use crate::geometry::{Color, Path, Point, Vertex};
use crate::tessellation;

const CLEAR_BG: [u8; 4] = [13, 13, 20, 255];

/// Rasterizes a decoded frame to a BGRA pixel buffer using the opaque clear
/// color used by the presentation path.
pub fn rasterize_frame(frame: Option<&DecodedFrame>, width: u32, height: u32) -> Vec<u8> {
    rasterize_frame_with_clear(frame, width, height, CLEAR_BG)
}

/// Rasterizes a decoded frame to a BGRA pixel buffer with an explicit clear
/// color. The readback path clears to transparent so output is comparable with
/// the software backend's transparent origin.
pub fn rasterize_frame_with_clear(
    frame: Option<&DecodedFrame>,
    width: u32,
    height: u32,
    clear: [u8; 4],
) -> Vec<u8> {
    let mut pixels = vec![0u8; width as usize * height as usize * 4];
    clear_pixels(&mut pixels, clear);

    let Some(frame) = frame else {
        crate::frame::record_vertex_count(0);
        return pixels;
    };

    let screen = Rect {
        min: Point { x: 0.0, y: 0.0 },
        max: Point {
            x: width as f32,
            y: height as f32,
        },
    };

    let mut vertex_count = 0usize;
    for batch in &frame.batches {
        vertex_count += rasterize_batch(&mut pixels, width, height, screen, batch);
    }
    crate::frame::record_vertex_count(vertex_count);
    pixels
}

/// State machine mirroring the software backend's `renderState`. Transforms are
/// applied at raster time: the CPU adapter consumes the same `DecodedFrame` the
/// GPU path will consume, so the stepping stone is removable in Slice 3.
struct RasterState {
    transform: Transform,
    transform_stack: Vec<Transform>,
    clip_stack: Vec<Rect>,
    opacity: f32,
    opacity_stack: Vec<f32>,
}

impl RasterState {
    fn clip(&self) -> Rect {
        *self.clip_stack.last().expect("clip stack never empties")
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
        let transformed = self.transform.transform_rect(rect);
        let next = self.clip().intersect(transformed);
        self.clip_stack.push(next);
    }

    fn pop_clip(&mut self) {
        if self.clip_stack.len() > 1 {
            self.clip_stack.pop();
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

fn rasterize_batch(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    screen: Rect,
    batch: &DecodedBatch,
) -> usize {
    let mut clip = screen.intersect(batch.bounds);
    if let Some(batch_clip) = batch.clip {
        clip = clip.intersect(batch_clip);
    }
    if clip.is_empty() {
        return 0;
    }

    let mut state = RasterState {
        transform: batch.transform,
        transform_stack: Vec::new(),
        clip_stack: vec![clip],
        opacity: batch.opacity,
        opacity_stack: Vec::new(),
    };

    let mut vertex_count = 0usize;
    for cmd in &batch.commands {
        match cmd {
            DecodedCommand::FillRect { rect, brush } => {
                let path = Path::rect(
                    rect.min.x,
                    rect.min.y,
                    rect.max.x - rect.min.x,
                    rect.max.y - rect.min.y,
                );
                vertex_count += rasterize_fill_path(pixels, width, height, &state, &path, brush);
            }
            DecodedCommand::StrokeRect { rect, stroke, brush } => {
                // Replicate the software backend's strokeRect (four band fills
                // between outer and inner insets) so the stepping stone matches
                // the oracle at corners; quad-based strokes leave corner gaps.
                vertex_count += rasterize_stroke_rect(
                    pixels, width, height, &state, *rect, stroke.width, brush,
                );
            }
            DecodedCommand::FillPath { path, brush } => {
                vertex_count += rasterize_fill_path(pixels, width, height, &state, path, brush);
            }
            DecodedCommand::StrokePath { path, stroke, brush } => {
                vertex_count +=
                    rasterize_stroke_path(pixels, width, height, &state, path, stroke.width, brush);
            }
            DecodedCommand::DrawPolyline {
                points,
                stroke,
                brush,
                closed,
            } => {
                let path = Path::polyline(points, *closed);
                vertex_count +=
                    rasterize_stroke_path(pixels, width, height, &state, &path, stroke.width, brush);
            }
            DecodedCommand::DrawPoints {
                points,
                radius,
                brush,
            } => {
                // The software oracle renders points as squares (fillRect of
                // radius*2). Match it rather than drawing circles.
                for point in points {
                    let path = Path::rect(
                        point.x - radius,
                        point.y - radius,
                        radius * 2.0,
                        radius * 2.0,
                    );
                    vertex_count += rasterize_fill_path(pixels, width, height, &state, &path, brush);
                }
            }
            DecodedCommand::DrawSelectionRects { rects, brush } => {
                for rect in rects {
                    let path = Path::rect(
                        rect.min.x,
                        rect.min.y,
                        rect.max.x - rect.min.x,
                        rect.max.y - rect.min.y,
                    );
                    vertex_count +=
                        rasterize_fill_path(pixels, width, height, &state, &path, brush);
                }
            }
            DecodedCommand::PushTransform { matrix } => state.push_transform(*matrix),
            DecodedCommand::PopTransform => state.pop_transform(),
            DecodedCommand::PushClipRect { rect } => state.push_clip_rect(*rect),
            DecodedCommand::PopClip => state.pop_clip(),
            DecodedCommand::PushOpacity { alpha } => state.push_opacity(*alpha),
            DecodedCommand::PopOpacity => state.pop_opacity(),
            DecodedCommand::DrawGlyphRun {
                font_id,
                size_bits,
                origin,
                glyphs,
                brush,
            } => {
                rasterize_text_run(
                    pixels,
                    width,
                    height,
                    &state,
                    *font_id,
                    *size_bits,
                    *origin,
                    glyphs,
                    brush,
                );
            }
            DecodedCommand::DrawImage {
                handle,
                dest,
                src,
                sampling,
                opacity,
            } => {
                rasterize_image_draw(
                    pixels, width, height, &state, *handle, *dest, *src, *sampling, *opacity,
                );
            }
            DecodedCommand::DrawTexture { .. } => {
                // The CPU stepping-stone raster has no texture store; the GPU
                // pipeline (Slice 4) renders these. No-op here by design.
            }
            DecodedCommand::DrawBlurredShadow { .. } => {
                // The CPU stepping-stone raster does not render shadows; the
                // GPU pipeline (Slice 9) does. No-op here by design.
            }
            DecodedCommand::BeginRenderBatch { .. } | DecodedCommand::EndRenderBatch => {}
        }
    }
    vertex_count
}

fn rasterize_stroke_rect(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    state: &RasterState,
    rect: Rect,
    stroke_width: f32,
    brush: &Brush,
) -> usize {
    if stroke_width <= 0.0 {
        return 0;
    }
    let half = stroke_width / 2.0;
    let outer = rect.inset(-half);
    let inner = rect.inset(half);
    let mut vertex_count = 0usize;
    let bands = [
        Rect {
            min: outer.min,
            max: Point {
                x: outer.max.x,
                y: inner.min.y,
            },
        },
        Rect {
            min: Point {
                x: outer.min.x,
                y: inner.min.y,
            },
            max: Point {
                x: inner.min.x,
                y: inner.max.y,
            },
        },
        Rect {
            min: Point {
                x: inner.max.x,
                y: inner.min.y,
            },
            max: Point {
                x: outer.max.x,
                y: inner.max.y,
            },
        },
        Rect {
            min: Point {
                x: outer.min.x,
                y: inner.max.y,
            },
            max: outer.max,
        },
    ];
    for band in bands {
        let path = Path::rect(
            band.min.x,
            band.min.y,
            band.max.x - band.min.x,
            band.max.y - band.min.y,
        );
        vertex_count += rasterize_fill_path(pixels, width, height, state, &path, brush);
    }
    vertex_count
}

fn rasterize_fill_path(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    state: &RasterState,
    path: &crate::geometry::Path,
    brush: &Brush,
) -> usize {
    let verts = tessellation::tessellate_fill(path, brush.color);
    let mut transformed = Vec::with_capacity(verts.len());
    for v in verts {
        transformed.push(Vertex {
            pos: state.transform.apply_point(v.pos),
            color: v.color,
        });
    }
    let clip = state.clip();
    for tri in transformed.chunks_exact(3) {
        rasterize_triangle(pixels, width, height, tri, state.opacity, clip, brush);
    }
    transformed.len()
}

fn rasterize_stroke_path(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    state: &RasterState,
    path: &crate::geometry::Path,
    stroke_width: f32,
    brush: &Brush,
) -> usize {
    let verts = tessellation::tessellate_stroke(path, stroke_width, brush.color);
    let mut transformed = Vec::with_capacity(verts.len());
    for v in verts {
        transformed.push(Vertex {
            pos: state.transform.apply_point(v.pos),
            color: v.color,
        });
    }
    let clip = state.clip();
    for tri in transformed.chunks_exact(3) {
        rasterize_triangle(pixels, width, height, tri, state.opacity, clip, brush);
    }
    transformed.len()
}

fn sample_brush(brush: &Brush, p: Point) -> Color {
    match brush.kind {
        BrushKind::Solid => brush.color,
        BrushKind::LinearGradient => sample_linear_gradient(brush, p),
    }
}

fn sample_linear_gradient(brush: &Brush, p: Point) -> Color {
    let stops = &brush.gradient_stops;
    if stops.is_empty() {
        return Color::default();
    }
    if stops.len() == 1 {
        return stops[0].color;
    }
    let dx = brush.gradient_end.x - brush.gradient_start.x;
    let dy = brush.gradient_end.y - brush.gradient_start.y;
    let denom = dx * dx + dy * dy;
    if denom == 0.0 {
        return stops[stops.len() - 1].color;
    }
    let t = ((p.x - brush.gradient_start.x) * dx + (p.y - brush.gradient_start.y) * dy) / denom;
    let t = t.clamp(0.0, 1.0);

    let mut left = stops[0];
    let mut right = stops[stops.len() - 1];
    for i in 0..stops.len() - 1 {
        if t >= stops[i].offset && t <= stops[i + 1].offset {
            left = stops[i];
            right = stops[i + 1];
            break;
        }
    }
    if right.offset == left.offset {
        return right.color;
    }
    let f = (t - left.offset) / (right.offset - left.offset);
    lerp_color(left.color, right.color, f)
}

fn lerp_color(a: Color, b: Color, t: f32) -> Color {
    Color {
        r: a.r + (b.r - a.r) * t,
        g: a.g + (b.g - a.g) * t,
        b: a.b + (b.b - a.b) * t,
        a: a.a + (b.a - a.a) * t,
    }
}

fn rasterize_triangle(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    tri: &[Vertex],
    opacity: f32,
    clip: Rect,
    brush: &Brush,
) {
    if tri.len() < 3 {
        return;
    }
    let min_x = tri
        .iter()
        .map(|v| v.pos.x)
        .fold(f32::INFINITY, f32::min)
        .floor()
        .max(clip.min.x)
        .max(0.0) as i32;
    let min_y = tri
        .iter()
        .map(|v| v.pos.y)
        .fold(f32::INFINITY, f32::min)
        .floor()
        .max(clip.min.y)
        .max(0.0) as i32;
    let max_x = tri
        .iter()
        .map(|v| v.pos.x)
        .fold(f32::NEG_INFINITY, f32::max)
        .ceil()
        .min(clip.max.x)
        .min(width as f32) as i32;
    let max_y = tri
        .iter()
        .map(|v| v.pos.y)
        .fold(f32::NEG_INFINITY, f32::max)
        .ceil()
        .min(clip.max.y)
        .min(height as f32) as i32;

    if min_x >= max_x || min_y >= max_y {
        return;
    }

    let a = tri[0];
    let b = tri[1];
    let c = tri[2];
    let area = edge(a.pos, b.pos, c.pos);
    if area == 0.0 {
        return;
    }
    // Edge functions carry the winding sign (negative for CW). Normalize so the
    // inside test is sign-agnostic.
    let sign = if area >= 0.0 { 1.0 } else { -1.0 };
    for y in min_y..max_y {
        for x in min_x..max_x {
            let p = Point {
                x: x as f32 + 0.5,
                y: y as f32 + 0.5,
            };
            // Raw edge functions: for integer-aligned geometry these are exact,
            // so the top-left tie-break (`se == 0`) reliably assigns seam
            // pixels to one of the triangles sharing an edge.
            let e0 = edge(b.pos, c.pos, p);
            let e1 = edge(c.pos, a.pos, p);
            let e2 = edge(a.pos, b.pos, p);
            // Top-left rule: a pixel exactly on an edge (e == 0) is owned by
            // exactly one of the triangles sharing it, so shared diagonals in
            // rect/path triangulations are not double-blended (which would
            // darken semi-transparent fills along the seam).
            if !inside(e0 * sign, top_left(b.pos, c.pos))
                || !inside(e1 * sign, top_left(c.pos, a.pos))
                || !inside(e2 * sign, top_left(a.pos, b.pos))
            {
                continue;
            }
            // Sample the brush per-pixel at the output position, matching the
            // software oracle. Solid brushes are constant; linear gradients are
            // piecewise-linear in position, so vertex-color interpolation would
            // ignore internal stop breakpoints and drift on multi-stop ramps.
            // (Sample coordinates are exact only under identity/translation
            // transforms; the GPU gradient pipeline replaces this in Slice 6.)
            let color = blend_color(sample_brush(brush, p), opacity);
            blend_pixel(pixels, width, x as u32, y as u32, color);
        }
    }
}

fn edge(a: Point, b: Point, c: Point) -> f32 {
    (c.x - a.x) * (b.y - a.y) - (c.y - a.y) * (b.x - a.x)
}

/// Reports whether a pixel with edge function value `w` is inside the triangle
/// under the top-left rule: strictly positive, or exactly on an edge that is a
/// top or left edge (owned by this triangle).
fn inside(w: f32, edge_top_left: bool) -> bool {
    w > 0.0 || (w == 0.0 && edge_top_left)
}

/// An edge (p -> q) is top-left when it points up (q.y < p.y) or is a
/// horizontal edge pointing left (q.x < p.x). Adjacent triangles traverse a
/// shared edge in opposite directions, so exactly one of them owns the seam.
fn top_left(p: Point, q: Point) -> bool {
    q.y < p.y || (q.y == p.y && q.x < p.x)
}

/// Converts a (premultiplied) color to premultiplied BGRA bytes, scaling the
/// channels and alpha by `opacity`. The output is consistent with
/// `blend_pixel`'s premultiplied "over" compositing.
fn blend_color(color: Color, opacity: f32) -> [u8; 4] {
    let alpha = (color.a * opacity).clamp(0.0, 1.0);
    let r = (color.r * opacity).clamp(0.0, 1.0);
    let g = (color.g * opacity).clamp(0.0, 1.0);
    let b = (color.b * opacity).clamp(0.0, 1.0);
    [
        (b * 255.0) as u8,
        (g * 255.0) as u8,
        (r * 255.0) as u8,
        (alpha * 255.0) as u8,
    ]
}

/// Premultiplied-alpha "over" compositing of a BGRA source onto the pixel
/// buffer. `src` is premultiplied (channels already include alpha).
fn blend_pixel(pixels: &mut [u8], width: u32, x: u32, y: u32, src: [u8; 4]) {
    let idx = ((y * width + x) * 4) as usize;
    if idx + 3 >= pixels.len() {
        return;
    }

    let dst_a = pixels[idx + 3] as f32 / 255.0;
    let src_a = src[3] as f32 / 255.0;
    let out_a = src_a + dst_a * (1.0 - src_a);
    if out_a <= 0.0 {
        return;
    }

    let src_r = src[2] as f32 / 255.0;
    let src_g = src[1] as f32 / 255.0;
    let src_b = src[0] as f32 / 255.0;
    let dst_r = pixels[idx + 2] as f32 / 255.0;
    let dst_g = pixels[idx + 1] as f32 / 255.0;
    let dst_b = pixels[idx] as f32 / 255.0;

    let out_r = src_r + dst_r * (1.0 - src_a);
    let out_g = src_g + dst_g * (1.0 - src_a);
    let out_b = src_b + dst_b * (1.0 - src_a);

    pixels[idx] = (out_b.clamp(0.0, 1.0) * 255.0) as u8;
    pixels[idx + 1] = (out_g.clamp(0.0, 1.0) * 255.0) as u8;
    pixels[idx + 2] = (out_r.clamp(0.0, 1.0) * 255.0) as u8;
    pixels[idx + 3] = (out_a.clamp(0.0, 1.0) * 255.0) as u8;
}

fn rasterize_text_run(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    state: &RasterState,
    font_id: u64,
    size_bits: u32,
    origin: Point,
    glyphs: &[DecodedGlyph],
    brush: &Brush,
) {
    let use_sdf = glyph_prefers_sdf(size_bits);
    for glyph in glyphs {
        let bitmap = if use_sdf {
            atlas::lookup_glyph_sdf(font_id, glyph.glyph_id, size_bits)
                .or_else(|| atlas::lookup_glyph(font_id, glyph.glyph_id, size_bits))
        } else {
            atlas::lookup_glyph(font_id, glyph.glyph_id, size_bits)
        };
        let Some(bitmap) = bitmap else {
            continue;
        };
        let local = Point {
            x: origin.x + glyph.x,
            y: origin.y + glyph.y,
        };
        let world = state.transform.apply_point(local);
        draw_glyph_bitmap(
            pixels,
            width,
            height,
            &bitmap,
            world.x + bitmap.offset_x,
            world.y + bitmap.offset_y,
            sample_brush(brush, local),
            state.opacity,
            state.clip(),
        );
    }
}

fn glyph_prefers_sdf(size_bits: u32) -> bool {
    f32::from_bits(size_bits) >= 24.0
}

fn draw_glyph_bitmap(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    glyph: &GlyphBitmap,
    x: f32,
    y: f32,
    color: Color,
    opacity: f32,
    clip: Rect,
) {
    if glyph.width == 0 || glyph.height == 0 {
        return;
    }

    let min_x = x.floor().max(clip.min.x).max(0.0) as i32;
    let min_y = y.floor().max(clip.min.y).max(0.0) as i32;
    let max_x = (x + glyph.width as f32)
        .ceil()
        .min(clip.max.x)
        .min(width as f32) as i32;
    let max_y = (y + glyph.height as f32)
        .ceil()
        .min(clip.max.y)
        .min(height as f32) as i32;
    if min_x >= max_x || min_y >= max_y {
        return;
    }

    for sy in 0..glyph.height as i32 {
        let dy = min_y + sy;
        if dy < min_y || dy >= max_y {
            continue;
        }
        for sx in 0..glyph.width as i32 {
            let dx = min_x + sx;
            if dx < min_x || dx >= max_x {
                continue;
            }
            let idx = (sy as u32 * glyph.width + sx as u32) as usize;
            if idx >= glyph.pixels.len() {
                continue;
            }
            let alpha = glyph.pixels[idx] as f32 / 255.0 * opacity;
            if alpha <= 0.0 {
                continue;
            }
            let src = blend_color(color, alpha);
            blend_pixel(pixels, width, dx as u32, dy as u32, src);
        }
    }
}

fn rasterize_image_draw(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    state: &RasterState,
    handle: u64,
    dest: Rect,
    src: Rect,
    sampling: u8,
    opacity: f32,
) {
    let Some(image) = image_store::lookup_image(handle) else {
        return;
    };
    if image.width == 0 || image.height == 0 {
        return;
    }

    let src = if src.is_empty() {
        Rect {
            min: Point { x: 0.0, y: 0.0 },
            max: Point {
                x: image.width as f32,
                y: image.height as f32,
            },
        }
    } else {
        src
    };
    let screen = Rect {
        min: Point { x: 0.0, y: 0.0 },
        max: Point {
            x: width as f32,
            y: height as f32,
        },
    };
    let world_dest = state
        .transform
        .transform_rect(dest)
        .intersect(state.clip())
        .intersect(screen);
    if world_dest.is_empty() {
        return;
    }
    let dst_w = world_dest.max.x - world_dest.min.x;
    let dst_h = world_dest.max.y - world_dest.min.y;
    if dst_w <= 0.0 || dst_h <= 0.0 {
        return;
    }

    for y in world_dest.min.y.floor() as i32..world_dest.max.y.ceil() as i32 {
        if y < 0 || y >= height as i32 {
            continue;
        }
        let ty = (y as f32 + 0.5 - world_dest.min.y) / dst_h;
        let sy = src.min.y + ty * (src.max.y - src.min.y);
        for x in world_dest.min.x.floor() as i32..world_dest.max.x.ceil() as i32 {
            if x < 0 || x >= width as i32 {
                continue;
            }
            let tx = (x as f32 + 0.5 - world_dest.min.x) / dst_w;
            let sx = src.min.x + tx * (src.max.x - src.min.x);
            let color = sample_image(&image, sx, sy, sampling);
            let eff = color[3] as f32 / 255.0 * state.opacity * opacity;
            if eff <= 0.0 {
                continue;
            }
            // Premultiply the (straight-alpha) image pixel by the effective
            // alpha so the BGRA source is consistent with blend_pixel.
            let src = [
                (color[2] as f32 * eff) as u8,
                (color[1] as f32 * eff) as u8,
                (color[0] as f32 * eff) as u8,
                (eff.clamp(0.0, 1.0) * 255.0) as u8,
            ];
            blend_pixel(pixels, width, x as u32, y as u32, src);
        }
    }
}

fn sample_image(image: &image_store::ImageBitmap, sx: f32, sy: f32, sampling: u8) -> [u8; 4] {
    match sampling {
        1 => sample_image_bilinear(image, sx, sy),
        _ => sample_image_nearest(image, sx, sy),
    }
}

fn sample_image_nearest(image: &image_store::ImageBitmap, sx: f32, sy: f32) -> [u8; 4] {
    let x = sx.round().clamp(0.0, image.width.saturating_sub(1) as f32) as u32;
    let y = sy.round().clamp(0.0, image.height.saturating_sub(1) as f32) as u32;
    sample_image_pixel(image, x, y)
}

fn sample_image_bilinear(image: &image_store::ImageBitmap, sx: f32, sy: f32) -> [u8; 4] {
    let x0 = sx.floor();
    let y0 = sy.floor();
    let x1 = x0 + 1.0;
    let y1 = y0 + 1.0;
    let fx = sx - x0;
    let fy = sy - y0;
    let c00 = sample_image_pixel(
        image,
        x0.clamp(0.0, image.width.saturating_sub(1) as f32) as u32,
        y0.clamp(0.0, image.height.saturating_sub(1) as f32) as u32,
    );
    let c10 = sample_image_pixel(
        image,
        x1.clamp(0.0, image.width.saturating_sub(1) as f32) as u32,
        y0.clamp(0.0, image.height.saturating_sub(1) as f32) as u32,
    );
    let c01 = sample_image_pixel(
        image,
        x0.clamp(0.0, image.width.saturating_sub(1) as f32) as u32,
        y1.clamp(0.0, image.height.saturating_sub(1) as f32) as u32,
    );
    let c11 = sample_image_pixel(
        image,
        x1.clamp(0.0, image.width.saturating_sub(1) as f32) as u32,
        y1.clamp(0.0, image.height.saturating_sub(1) as f32) as u32,
    );
    let lerp = |a: u8, b: u8, t: f32| -> f32 { a as f32 * (1.0 - t) + b as f32 * t };
    let mix = |c0: [u8; 4], c1: [u8; 4], t: f32| -> [f32; 4] {
        [
            lerp(c0[0], c1[0], t),
            lerp(c0[1], c1[1], t),
            lerp(c0[2], c1[2], t),
            lerp(c0[3], c1[3], t),
        ]
    };
    let top = mix(c00, c10, fx);
    let bottom = mix(c01, c11, fx);
    [
        (top[0] * (1.0 - fy) + bottom[0] * fy)
            .round()
            .clamp(0.0, 255.0) as u8,
        (top[1] * (1.0 - fy) + bottom[1] * fy)
            .round()
            .clamp(0.0, 255.0) as u8,
        (top[2] * (1.0 - fy) + bottom[2] * fy)
            .round()
            .clamp(0.0, 255.0) as u8,
        (top[3] * (1.0 - fy) + bottom[3] * fy)
            .round()
            .clamp(0.0, 255.0) as u8,
    ]
}

fn sample_image_pixel(image: &image_store::ImageBitmap, x: u32, y: u32) -> [u8; 4] {
    let idx = ((y * image.width + x) * 4) as usize;
    if idx + 3 >= image.pixels.len() {
        return [0, 0, 0, 0];
    }
    [
        image.pixels[idx],
        image.pixels[idx + 1],
        image.pixels[idx + 2],
        image.pixels[idx + 3],
    ]
}

fn clear_pixels(pixels: &mut [u8], color: [u8; 4]) {
    for px in pixels.chunks_exact_mut(4) {
        px.copy_from_slice(&color);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::atlas::{reset_atlas, upload_glyph, GlyphBitmap};
    use crate::frame::{DecodedFrame, FrameStats};
    use crate::image_store::{create_image, reset_images, ImageFormat};

    fn screen_rect(width: u32, height: u32) -> Rect {
        Rect {
            min: Point { x: 0.0, y: 0.0 },
            max: Point {
                x: width as f32,
                y: height as f32,
            },
        }
    }

    fn solid_batch(commands: Vec<DecodedCommand>) -> DecodedBatch {
        DecodedBatch {
            id: 1,
            bounds: screen_rect(64, 64),
            opacity: 1.0,
            transform: Transform::identity(),
            clip: None,
            command_count: commands.len(),
            commands,
        }
    }

    fn red() -> Color {
        Color {
            r: 1.0,
            g: 0.0,
            b: 0.0,
            a: 1.0,
        }
    }

    fn frame(batches: Vec<DecodedBatch>) -> DecodedFrame {
        DecodedFrame {
            stats: FrameStats::default(),
            surface_w: 64,
            surface_h: 64,
            device_pixel_ratio: 1.0,
            batches,
        }
    }

    fn assert_pixel_rgb(px: &[u8], x: u32, y: u32, r: u8, g: u8, b: u8) {
        let idx = ((y * 64 + x) * 4) as usize;
        assert_eq!(px[idx + 2], r, "r at ({},{})", x, y);
        assert_eq!(px[idx + 1], g, "g at ({},{})", x, y);
        assert_eq!(px[idx], b, "b at ({},{})", x, y);
    }

    #[test]
    fn rasterizes_solid_rect() {
        let f = frame(vec![solid_batch(vec![DecodedCommand::FillRect {
            rect: Rect {
                min: Point { x: 2.0, y: 2.0 },
                max: Point { x: 8.0, y: 8.0 },
            },
            brush: Brush::solid(red()),
        }])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 4, 4, 255, 0, 0);
        assert_pixel_rgb(&pixels, 9, 4, 0, 0, 0);
        assert_pixel_rgb(&pixels, 4, 9, 0, 0, 0);
    }

    #[test]
    fn applies_batch_transform_at_raster_time() {
        let mut batch = solid_batch(vec![DecodedCommand::FillRect {
            rect: Rect {
                min: Point { x: 0.0, y: 0.0 },
                max: Point { x: 4.0, y: 4.0 },
            },
            brush: Brush::solid(red()),
        }]);
        batch.transform = Transform::from_parts(1.0, 0.0, 0.0, 1.0, 10.0, 12.0);
        let f = frame(vec![batch]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 11, 13, 255, 0, 0);
        assert_pixel_rgb(&pixels, 2, 2, 0, 0, 0);
    }

    #[test]
    fn applies_nested_command_transform() {
        let f = frame(vec![solid_batch(vec![
            DecodedCommand::PushTransform {
                matrix: Transform::from_parts(1.0, 0.0, 0.0, 1.0, 5.0, 0.0),
            },
            DecodedCommand::FillRect {
                rect: Rect {
                    min: Point { x: 0.0, y: 0.0 },
                    max: Point { x: 4.0, y: 4.0 },
                },
                brush: Brush::solid(red()),
            },
            DecodedCommand::PopTransform,
        ])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 6, 1, 255, 0, 0);
        assert_pixel_rgb(&pixels, 1, 1, 0, 0, 0);
    }

    #[test]
    fn nested_transform_pop_restores_parent() {
        let f = frame(vec![solid_batch(vec![
            DecodedCommand::PushTransform {
                matrix: Transform::from_parts(1.0, 0.0, 0.0, 1.0, 5.0, 0.0),
            },
            DecodedCommand::FillRect {
                rect: Rect {
                    min: Point { x: 0.0, y: 0.0 },
                    max: Point { x: 2.0, y: 2.0 },
                },
                brush: Brush::solid(red()),
            },
            DecodedCommand::PopTransform,
            DecodedCommand::FillRect {
                rect: Rect {
                    min: Point { x: 0.0, y: 0.0 },
                    max: Point { x: 2.0, y: 2.0 },
                },
                brush: Brush::solid(Color { r: 0.0, g: 1.0, b: 0.0, a: 1.0 }),
            },
        ])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        // First rect at (5,0); second (no transform) at (0,0).
        assert_pixel_rgb(&pixels, 5, 1, 255, 0, 0);
        assert_pixel_rgb(&pixels, 1, 1, 0, 255, 0);
    }

    #[test]
    fn clips_to_batch_bounds() {
        let batch = solid_batch(vec![DecodedCommand::FillRect {
            rect: Rect {
                min: Point { x: 0.0, y: 0.0 },
                max: Point { x: 100.0, y: 100.0 },
            },
            brush: Brush::solid(red()),
        }]);
        let f = frame(vec![batch]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 1, 1, 255, 0, 0);
        // The full 64x64 clip is covered; alpha at the far corner is opaque.
        assert!(pixels[((63 * 64 + 63) * 4) as usize + 3] >= 250);
    }

    #[test]
    fn push_clip_rect_restricts_drawing() {
        let f = frame(vec![solid_batch(vec![
            DecodedCommand::PushClipRect {
                rect: Rect {
                    min: Point { x: 4.0, y: 4.0 },
                    max: Point { x: 8.0, y: 8.0 },
                },
            },
            DecodedCommand::FillRect {
                rect: Rect {
                    min: Point { x: 0.0, y: 0.0 },
                    max: Point { x: 100.0, y: 100.0 },
                },
                brush: Brush::solid(red()),
            },
            DecodedCommand::PopClip,
        ])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 6, 6, 255, 0, 0);
        assert_pixel_rgb(&pixels, 2, 2, 0, 0, 0);
    }

    #[test]
    fn opacity_stack_multiplies() {
        let f = frame(vec![solid_batch(vec![
            DecodedCommand::PushOpacity { alpha: 0.5 },
            DecodedCommand::PushOpacity { alpha: 0.5 },
            DecodedCommand::FillRect {
                rect: Rect {
                    min: Point { x: 0.0, y: 0.0 },
                    max: Point { x: 8.0, y: 8.0 },
                },
                brush: Brush::solid(Color { r: 1.0, g: 0.0, b: 0.0, a: 1.0 }),
            },
        ])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        let idx = ((4 * 64 + 4) * 4) as usize;
        // 0.5*0.5 = 0.25 alpha over transparent -> a ~ 64, r premultiplied ~ 64.
        assert!(pixels[idx + 3] >= 60 && pixels[idx + 3] <= 70, "alpha {}", pixels[idx + 3]);
    }

    #[test]
    fn rasterizes_text_run_with_bitmap() {
        let _guard = crate::state_lock_guard();
        reset_atlas();
        upload_glyph(
            7,
            42,
            16,
            GlyphBitmap {
                width: 1,
                height: 1,
                pixels: vec![255],
                offset_x: 0.0,
                offset_y: 0.0,
                advance: 1.0,
            },
        );
        let f = frame(vec![solid_batch(vec![DecodedCommand::DrawGlyphRun {
            font_id: 7,
            size_bits: 16,
            origin: Point { x: 3.0, y: 4.0 },
            glyphs: vec![DecodedGlyph {
                glyph_id: 42,
                x: 0.0,
                y: 0.0,
            }],
            brush: Brush::solid(red()),
        }])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 3, 4, 255, 0, 0);
    }

    #[test]
    fn rasterizes_image_draw() {
        let _guard = crate::state_lock_guard();
        reset_images();
        let handle = create_image(&[0, 255, 0, 255], 1, 1, 4, ImageFormat::Rgba8).expect("create");
        let f = frame(vec![solid_batch(vec![DecodedCommand::DrawImage {
            handle,
            dest: Rect {
                min: Point { x: 2.0, y: 2.0 },
                max: Point { x: 4.0, y: 4.0 },
            },
            src: Rect {
                min: Point { x: 0.0, y: 0.0 },
                max: Point { x: 1.0, y: 1.0 },
            },
            sampling: 0,
            opacity: 1.0,
        }])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        assert_pixel_rgb(&pixels, 2, 2, 0, 255, 0);
    }

    #[test]
    fn gradient_brush_interpolates() {
        let brush = Brush {
            kind: BrushKind::LinearGradient,
            color: Color::default(),
            gradient_start: Point { x: 0.0, y: 0.0 },
            gradient_end: Point { x: 8.0, y: 0.0 },
            gradient_stops: vec![
                crate::frame::GradientStop {
                    offset: 0.0,
                    color: red(),
                },
                crate::frame::GradientStop {
                    offset: 1.0,
                    color: Color {
                        r: 0.0,
                        g: 0.0,
                        b: 1.0,
                        a: 1.0,
                    },
                },
            ],
        };
        let f = frame(vec![solid_batch(vec![DecodedCommand::FillRect {
            rect: Rect {
                min: Point { x: 0.0, y: 0.0 },
                max: Point { x: 8.0, y: 8.0 },
            },
            brush,
        }])]);
        let pixels = rasterize_frame_with_clear(Some(&f), 64, 64, [0, 0, 0, 0]);
        let left = ((1 * 64 + 1) * 4) as usize;
        let right = ((1 * 64 + 6) * 4) as usize;
        // BGRA buffer: left red-dominant (R at index 2), right blue-dominant (B at index 0).
        assert!(pixels[left + 2] > pixels[left]);
        assert!(pixels[right] > pixels[right + 2]);
    }

    #[test]
    fn empty_frame_clears_to_transparent() {
        let f = frame(vec![]);
        let pixels = rasterize_frame_with_clear(Some(&f), 8, 8, [0, 0, 0, 0]);
        assert!(pixels.iter().all(|&b| b == 0));
    }
}
