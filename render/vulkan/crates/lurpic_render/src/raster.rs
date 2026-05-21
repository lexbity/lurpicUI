use crate::atlas::{self, GlyphBitmap};
use crate::frame::{DecodedBatch, DecodedFrame, DecodedImageDraw, DecodedTextRun, Rect};
use crate::image_store;
use crate::tessellation::{Color, Point, Vertex};

const CLEAR_BG: [u8; 4] = [13, 13, 20, 255];

pub fn rasterize_frame(frame: Option<&DecodedFrame>, width: u32, height: u32) -> Vec<u8> {
    let mut pixels = vec![0u8; width as usize * height as usize * 4];
    clear_pixels(&mut pixels, CLEAR_BG);

    let Some(frame) = frame else {
        return pixels;
    };

    for batch in &frame.batches {
        rasterize_batch(&mut pixels, width, height, batch);
    }
    pixels
}

fn rasterize_batch(pixels: &mut [u8], width: u32, height: u32, batch: &DecodedBatch) {
    let clip = batch
        .clip
        .unwrap_or(Rect {
            min: Point { x: 0.0, y: 0.0 },
            max: Point {
                x: width as f32,
                y: height as f32,
            },
        })
        .intersect(Rect {
            min: Point { x: 0.0, y: 0.0 },
            max: Point {
                x: width as f32,
                y: height as f32,
            },
        });
    if clip.is_empty() {
        return;
    }

    for tri in batch.vertices.chunks_exact(3) {
        rasterize_triangle(pixels, width, height, tri, batch.opacity, clip);
    }

    for run in &batch.text_runs {
        rasterize_text_run(pixels, width, height, run, batch.opacity, clip);
    }

    for draw in &batch.image_draws {
        rasterize_image_draw(pixels, width, height, draw, batch.opacity, clip);
    }
}

fn rasterize_triangle(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    tri: &[Vertex],
    opacity: f32,
    clip: Rect,
) {
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

    let inv_area = 1.0 / area;
    for y in min_y..max_y {
        for x in min_x..max_x {
            let p = Point {
                x: x as f32 + 0.5,
                y: y as f32 + 0.5,
            };
            let w0 = edge(b.pos, c.pos, p) * inv_area;
            let w1 = edge(c.pos, a.pos, p) * inv_area;
            let w2 = edge(a.pos, b.pos, p) * inv_area;
            if w0 < 0.0 || w1 < 0.0 || w2 < 0.0 {
                continue;
            }
            let color = blend_color(
                Color {
                    r: a.color.r * w0 + b.color.r * w1 + c.color.r * w2,
                    g: a.color.g * w0 + b.color.g * w1 + c.color.g * w2,
                    b: a.color.b * w0 + b.color.b * w1 + c.color.b * w2,
                    a: a.color.a * w0 + b.color.a * w1 + c.color.a * w2,
                },
                opacity,
            );
            blend_pixel(pixels, width, x as u32, y as u32, color);
        }
    }
}

fn edge(a: Point, b: Point, c: Point) -> f32 {
    (c.x - a.x) * (b.y - a.y) - (c.y - a.y) * (b.x - a.x)
}

fn blend_color(color: Color, opacity: f32) -> [u8; 4] {
    let alpha = (color.a * opacity).clamp(0.0, 1.0);
    [
        (color.b.clamp(0.0, 1.0) * 255.0) as u8,
        (color.g.clamp(0.0, 1.0) * 255.0) as u8,
        (color.r.clamp(0.0, 1.0) * 255.0) as u8,
        (alpha * 255.0) as u8,
    ]
}

fn blend_pixel(pixels: &mut [u8], width: u32, x: u32, y: u32, src: [u8; 4]) {
    let idx = ((y * width + x) * 4) as usize;
    if idx + 3 >= pixels.len() {
        return;
    }

    let dst_b = pixels[idx] as f32 / 255.0;
    let dst_g = pixels[idx + 1] as f32 / 255.0;
    let dst_r = pixels[idx + 2] as f32 / 255.0;
    let dst_a = pixels[idx + 3] as f32 / 255.0;

    let src_a = src[3] as f32 / 255.0;
    let out_a = src_a + dst_a * (1.0 - src_a);
    if out_a <= 0.0 {
        return;
    }

    let src_r = src[2] as f32 / 255.0;
    let src_g = src[1] as f32 / 255.0;
    let src_b = src[0] as f32 / 255.0;
    let out_r = src_r * src_a + dst_r * dst_a * (1.0 - src_a);
    let out_g = src_g * src_a + dst_g * dst_a * (1.0 - src_a);
    let out_b = src_b * src_a + dst_b * dst_a * (1.0 - src_a);

    pixels[idx] = (out_b.clamp(0.0, 1.0) * 255.0) as u8;
    pixels[idx + 1] = (out_g.clamp(0.0, 1.0) * 255.0) as u8;
    pixels[idx + 2] = (out_r.clamp(0.0, 1.0) * 255.0) as u8;
    pixels[idx + 3] = (out_a.clamp(0.0, 1.0) * 255.0) as u8;
}

fn rasterize_text_run(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    run: &DecodedTextRun,
    batch_opacity: f32,
    clip: Rect,
) {
    let origin = run.origin;
    for glyph in &run.glyphs {
        let use_sdf = glyph_prefers_sdf(run.size_bits);
        let bitmap = if use_sdf {
            atlas::lookup_glyph_sdf(run.font_id, glyph.glyph_id, run.size_bits)
                .or_else(|| atlas::lookup_glyph(run.font_id, glyph.glyph_id, run.size_bits))
        } else {
            atlas::lookup_glyph(run.font_id, glyph.glyph_id, run.size_bits)
        };
        let Some(bitmap) = bitmap else {
            continue;
        };
        draw_glyph_bitmap(
            pixels,
            width,
            height,
            &bitmap,
            origin.x + glyph.x + bitmap.offset_x,
            origin.y + glyph.y + bitmap.offset_y,
            run.color,
            batch_opacity,
            clip,
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
    draw: &DecodedImageDraw,
    batch_opacity: f32,
    clip: Rect,
) {
    let Some(image) = image_store::lookup_image(draw.handle) else {
        return;
    };
    if image.width == 0 || image.height == 0 {
        return;
    }

    let src = if draw.src.is_empty() {
        Rect {
            min: Point { x: 0.0, y: 0.0 },
            max: Point {
                x: image.width as f32,
                y: image.height as f32,
            },
        }
    } else {
        draw.src
    };
    let dest = draw.dest.intersect(clip).intersect(Rect {
        min: Point { x: 0.0, y: 0.0 },
        max: Point {
            x: width as f32,
            y: height as f32,
        },
    });
    if dest.is_empty() {
        return;
    }
    let dst_w = dest.max.x - dest.min.x;
    let dst_h = dest.max.y - dest.min.y;
    if dst_w <= 0.0 || dst_h <= 0.0 {
        return;
    }

    for y in dest.min.y.floor() as i32..dest.max.y.ceil() as i32 {
        if y < 0 || y >= height as i32 {
            continue;
        }
        let ty = (y as f32 + 0.5 - dest.min.y) / dst_h;
        let sy = src.min.y + ty * (src.max.y - src.min.y);
        for x in dest.min.x.floor() as i32..dest.max.x.ceil() as i32 {
            if x < 0 || x >= width as i32 {
                continue;
            }
            let tx = (x as f32 + 0.5 - dest.min.x) / dst_w;
            let sx = src.min.x + tx * (src.max.x - src.min.x);
            let color = sample_image(&image, sx, sy, draw.sampling);
            let alpha = color[3] as f32 / 255.0 * batch_opacity * draw.opacity;
            if alpha <= 0.0 {
                continue;
            }
            let src = [
                color[2],
                color[1],
                color[0],
                (alpha.clamp(0.0, 1.0) * 255.0) as u8,
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
    use crate::frame::{DecodedFrame, DecodedGlyph, DecodedImageDraw, DecodedTextRun, FrameStats};
    use crate::image_store::{create_image, reset_images, ImageFormat};

    fn make_triangle() -> DecodedFrame {
        DecodedFrame {
            stats: FrameStats::default(),
            batches: vec![DecodedBatch {
                id: 1,
                vertices: vec![
                    Vertex {
                        pos: Point { x: 2.0, y: 2.0 },
                        color: Color {
                            r: 1.0,
                            g: 0.0,
                            b: 0.0,
                            a: 1.0,
                        },
                    },
                    Vertex {
                        pos: Point { x: 8.0, y: 2.0 },
                        color: Color {
                            r: 1.0,
                            g: 0.0,
                            b: 0.0,
                            a: 1.0,
                        },
                    },
                    Vertex {
                        pos: Point { x: 2.0, y: 8.0 },
                        color: Color {
                            r: 1.0,
                            g: 0.0,
                            b: 0.0,
                            a: 1.0,
                        },
                    },
                ],
                text_runs: vec![],
                image_draws: vec![],
                command_count: 1,
                opacity: 1.0,
                clip: None,
            }],
        }
    }

    #[test]
    fn rasterizes_triangle() {
        let pixels = rasterize_frame(Some(&make_triangle()), 16, 16);
        let idx = ((4 * 16 + 4) * 4) as usize;
        assert!(pixels[idx + 2] > 0);
    }

    #[test]
    fn rasterizes_text_only_batch() {
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
        let frame = DecodedFrame {
            stats: FrameStats::default(),
            batches: vec![DecodedBatch {
                id: 2,
                vertices: vec![],
                text_runs: vec![DecodedTextRun {
                    font_id: 7,
                    size_bits: 16,
                    origin: Point { x: 3.0, y: 4.0 },
                    color: Color {
                        r: 0.0,
                        g: 1.0,
                        b: 0.0,
                        a: 1.0,
                    },
                    glyphs: vec![DecodedGlyph {
                        glyph_id: 42,
                        x: 0.0,
                        y: 0.0,
                    }],
                }],
                image_draws: vec![],
                command_count: 1,
                opacity: 1.0,
                clip: None,
            }],
        };

        let pixels = rasterize_frame(Some(&frame), 8, 8);
        let idx = ((4 * 8 + 3) * 4) as usize;
        assert!(pixels[idx + 1] > 0);
    }

    #[test]
    fn rasterizes_text_run_applies_origin_and_glyph_offsets() {
        reset_atlas();
        upload_glyph(
            11,
            77,
            18,
            GlyphBitmap {
                width: 1,
                height: 1,
                pixels: vec![255],
                offset_x: 2.0,
                offset_y: 3.0,
                advance: 1.0,
            },
        );
        let frame = DecodedFrame {
            stats: FrameStats::default(),
            batches: vec![DecodedBatch {
                id: 4,
                vertices: vec![],
                text_runs: vec![DecodedTextRun {
                    font_id: 11,
                    size_bits: 18,
                    origin: Point { x: 10.0, y: 12.0 },
                    color: Color {
                        r: 1.0,
                        g: 0.0,
                        b: 0.0,
                        a: 1.0,
                    },
                    glyphs: vec![DecodedGlyph {
                        glyph_id: 77,
                        x: 4.0,
                        y: 5.0,
                    }],
                }],
                image_draws: vec![],
                command_count: 1,
                opacity: 1.0,
                clip: None,
            }],
        };

        let pixels = rasterize_frame(Some(&frame), 32, 32);
        let idx = ((20 * 32 + 16) * 4) as usize;
        assert!(pixels[idx + 2] > 0);
    }

    #[test]
    fn sdf_lookup_prefers_large_text() {
        reset_atlas();
        upload_glyph(
            7,
            42,
            32.0f32.to_bits(),
            GlyphBitmap {
                width: 3,
                height: 3,
                pixels: vec![0, 0, 0, 0, 255, 0, 0, 0, 0],
                offset_x: 0.0,
                offset_y: 0.0,
                advance: 1.0,
            },
        );
        let size_bits = 32.0f32.to_bits();
        let bitmap = atlas::lookup_glyph(7, 42, size_bits).expect("bitmap present");
        let sdf = atlas::lookup_glyph_sdf(7, 42, size_bits).expect("sdf present");
        assert_ne!(bitmap.pixels, sdf.pixels);
        assert!(glyph_prefers_sdf(size_bits));
    }

    #[test]
    fn rasterizes_image_draw() {
        reset_images();
        let handle = create_image(&[255, 0, 0, 255], 1, 1, 4, ImageFormat::Rgba8).expect("create");
        let frame = DecodedFrame {
            stats: FrameStats::default(),
            batches: vec![DecodedBatch {
                id: 3,
                vertices: vec![],
                text_runs: vec![],
                image_draws: vec![DecodedImageDraw {
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
                }],
                command_count: 1,
                opacity: 1.0,
                clip: None,
            }],
        };
        let pixels = rasterize_frame(Some(&frame), 8, 8);
        let idx = ((2 * 8 + 2) * 4) as usize;
        assert!(pixels[idx + 2] > 0);
    }
}
