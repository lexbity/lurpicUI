//! Shared geometry types used by the packet decoder, the CPU stepping-stone
//! raster, and (later) the GPU pipeline. Kept out of the `cpu-fallback`-gated
//! `tessellation` module because the decoder and the GPU path need them
//! regardless of the raster feature.

#[derive(Clone, Copy, Debug, PartialEq, Default)]
pub struct Color {
    pub r: f32,
    pub g: f32,
    pub b: f32,
    pub a: f32,
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Point {
    pub x: f32,
    pub y: f32,
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Vertex {
    pub pos: Point,
    pub color: Color,
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Verb {
    MoveTo(Point),
    LineTo(Point),
    QuadTo(Point, Point),
    CubicTo(Point, Point, Point),
    Close,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Path {
    pub verbs: Vec<Verb>,
}

impl Path {
    pub fn new() -> Self {
        Self { verbs: Vec::new() }
    }

    pub fn rect(x: f32, y: f32, w: f32, h: f32) -> Self {
        Self {
            verbs: vec![
                Verb::MoveTo(Point { x, y }),
                Verb::LineTo(Point { x: x + w, y }),
                Verb::LineTo(Point { x: x + w, y: y + h }),
                Verb::LineTo(Point { x, y: y + h }),
                Verb::Close,
            ],
        }
    }

    pub fn polyline(points: &[Point], closed: bool) -> Self {
        if points.is_empty() {
            return Self::new();
        }
        let mut verbs = vec![Verb::MoveTo(points[0])];
        for &p in &points[1..] {
            verbs.push(Verb::LineTo(p));
        }
        if closed {
            verbs.push(Verb::Close);
        }
        Self { verbs }
    }

    #[allow(dead_code)] // used by cpu-fallback paths
    pub fn rounded_rect(x: f32, y: f32, w: f32, h: f32, r: f32) -> Self {
        if r <= 0.0 {
            return Self::rect(x, y, w, h);
        }
        let r = r.min(w.min(h) / 2.0);
        Self {
            verbs: vec![
                Verb::MoveTo(Point { x: x + r, y }),
                Verb::LineTo(Point { x: x + w - r, y }),
                Verb::QuadTo(Point { x: x + w, y }, Point { x: x + w, y: y + r }),
                Verb::LineTo(Point {
                    x: x + w,
                    y: y + h - r,
                }),
                Verb::QuadTo(
                    Point { x: x + w, y: y + h },
                    Point {
                        x: x + w - r,
                        y: y + h,
                    },
                ),
                Verb::LineTo(Point { x: x + r, y: y + h }),
                Verb::QuadTo(Point { x, y: y + h }, Point { x, y: y + h - r }),
                Verb::LineTo(Point { x, y: y + r }),
                Verb::QuadTo(Point { x, y }, Point { x: x + r, y }),
                Verb::Close,
            ],
        }
    }

    #[allow(dead_code)] // used by cpu-fallback paths
    pub fn circle(cx: f32, cy: f32, r: f32) -> Self {
        if r <= 0.0 {
            return Self::new();
        }
        let k = 0.552_284_8 * r;
        Self {
            verbs: vec![
                Verb::MoveTo(Point { x: cx + r, y: cy }),
                Verb::CubicTo(
                    Point {
                        x: cx + r,
                        y: cy + k,
                    },
                    Point {
                        x: cx + k,
                        y: cy + r,
                    },
                    Point { x: cx, y: cy + r },
                ),
                Verb::CubicTo(
                    Point {
                        x: cx - k,
                        y: cy + r,
                    },
                    Point {
                        x: cx - r,
                        y: cy + k,
                    },
                    Point { x: cx - r, y: cy },
                ),
                Verb::CubicTo(
                    Point {
                        x: cx - r,
                        y: cy - k,
                    },
                    Point {
                        x: cx - k,
                        y: cy - r,
                    },
                    Point { x: cx, y: cy - r },
                ),
                Verb::CubicTo(
                    Point {
                        x: cx + k,
                        y: cy - r,
                    },
                    Point {
                        x: cx + r,
                        y: cy - k,
                    },
                    Point { x: cx + r, y: cy },
                ),
                Verb::Close,
            ],
        }
    }
}
