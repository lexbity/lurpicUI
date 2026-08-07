//! Shared geometry types used by the packet decoder and the GPU pipeline. The
//! `path_flatten` module consumes `Path`/`Point`/`Verb` for the stencil fill.

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

    /// A polyline path (used by the frame encoder's DrawPolyline arm).
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
}
