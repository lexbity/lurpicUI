#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Point {
    pub x: f32,
    pub y: f32,
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Color {
    pub r: f32,
    pub g: f32,
    pub b: f32,
    pub a: f32,
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

pub fn tessellate_fill(path: &Path, color: Color) -> Vec<Vertex> {
    let contours = flatten_path(path);
    let mut out = Vec::new();
    for contour in contours {
        if contour.len() < 3 {
            continue;
        }
        if let Some(mut fan) = ear_clip(&contour, color) {
            out.append(&mut fan);
        }
    }
    out
}

pub fn tessellate_stroke(path: &Path, width: f32, color: Color) -> Vec<Vertex> {
    if width <= 0.0 {
        return Vec::new();
    }
    let contours = flatten_path(path);
    let mut out = Vec::new();
    for contour in contours {
        if contour.len() < 2 {
            continue;
        }
        for i in 0..contour.len() - 1 {
            append_segment_quad(&mut out, contour[i], contour[i + 1], width, color);
        }
    }
    out
}

fn flatten_path(path: &Path) -> Vec<Vec<Point>> {
    let mut contours = Vec::new();
    let mut current = Vec::new();
    let mut start = Point { x: 0.0, y: 0.0 };
    let mut cursor = Point { x: 0.0, y: 0.0 };
    for verb in &path.verbs {
        match *verb {
            Verb::MoveTo(p) => {
                if !current.is_empty() {
                    contours.push(current);
                    current = Vec::new();
                }
                current.push(p);
                start = p;
                cursor = p;
            }
            Verb::LineTo(p) => {
                current.push(p);
                cursor = p;
            }
            Verb::QuadTo(c, p) => {
                let mut pts = flatten_quad(cursor, c, p, 0);
                current.append(&mut pts);
                cursor = p;
            }
            Verb::CubicTo(c1, c2, p) => {
                let mut pts = flatten_cubic(cursor, c1, c2, p, 0);
                current.append(&mut pts);
                cursor = p;
            }
            Verb::Close => {
                if !current.is_empty() {
                    if current.first() != current.last() {
                        current.push(start);
                    }
                    contours.push(current);
                    current = Vec::new();
                }
            }
        }
    }
    if !current.is_empty() {
        contours.push(current);
    }
    contours
}

fn flatten_quad(a: Point, b: Point, c: Point, depth: usize) -> Vec<Point> {
    if depth > 5 || quad_flat_enough(a, b, c) {
        return vec![c];
    }
    let ab = midpoint(a, b);
    let bc = midpoint(b, c);
    let abc = midpoint(ab, bc);
    let mut left = flatten_quad(a, ab, abc, depth + 1);
    let mut right = flatten_quad(abc, bc, c, depth + 1);
    left.pop();
    left.append(&mut right);
    left
}

fn flatten_cubic(a: Point, b: Point, c: Point, d: Point, depth: usize) -> Vec<Point> {
    if depth > 6 || cubic_flat_enough(a, b, c, d) {
        return vec![d];
    }
    let ab = midpoint(a, b);
    let bc = midpoint(b, c);
    let cd = midpoint(c, d);
    let abc = midpoint(ab, bc);
    let bcd = midpoint(bc, cd);
    let abcd = midpoint(abc, bcd);
    let mut left = flatten_cubic(a, ab, abc, abcd, depth + 1);
    let mut right = flatten_cubic(abcd, bcd, cd, d, depth + 1);
    left.pop();
    left.append(&mut right);
    left
}

fn quad_flat_enough(a: Point, b: Point, c: Point) -> bool {
    point_distance_to_line(b, a, c) <= 0.25
}

fn cubic_flat_enough(a: Point, b: Point, c: Point, d: Point) -> bool {
    point_distance_to_line(b, a, d) <= 0.25 && point_distance_to_line(c, a, d) <= 0.25
}

fn midpoint(a: Point, b: Point) -> Point {
    Point {
        x: (a.x + b.x) * 0.5,
        y: (a.y + b.y) * 0.5,
    }
}

fn point_distance_to_line(p: Point, a: Point, b: Point) -> f32 {
    let dx = b.x - a.x;
    let dy = b.y - a.y;
    if dx == 0.0 && dy == 0.0 {
        return ((p.x - a.x).powi(2) + (p.y - a.y).powi(2)).sqrt();
    }
    ((dy * p.x - dx * p.y + b.x * a.y - b.y * a.x).abs()) / (dx * dx + dy * dy).sqrt()
}

fn ear_clip(poly: &[Point], color: Color) -> Option<Vec<Vertex>> {
    if poly.len() < 3 {
        return None;
    }
    let mut indices: Vec<usize> = (0..poly.len() - 1).collect();
    if polygon_area(poly) < 0.0 {
        indices.reverse();
    }
    let mut out = Vec::new();
    let mut guard = 0usize;
    while indices.len() >= 3 && guard < 10_000 {
        guard += 1;
        let mut ear_found = false;
        for i in 0..indices.len() {
            let i0 = indices[(i + indices.len() - 1) % indices.len()];
            let i1 = indices[i];
            let i2 = indices[(i + 1) % indices.len()];
            if !is_convex(poly[i0], poly[i1], poly[i2]) {
                continue;
            }
            if contains_point(poly, &indices, i0, i1, i2) {
                continue;
            }
            out.push(Vertex {
                pos: poly[i0],
                color,
            });
            out.push(Vertex {
                pos: poly[i1],
                color,
            });
            out.push(Vertex {
                pos: poly[i2],
                color,
            });
            indices.remove(i);
            ear_found = true;
            break;
        }
        if !ear_found {
            break;
        }
    }
    if out.is_empty() {
        None
    } else {
        Some(out)
    }
}

fn polygon_area(poly: &[Point]) -> f32 {
    let mut area = 0.0;
    for i in 0..poly.len().saturating_sub(1) {
        area += poly[i].x * poly[i + 1].y - poly[i + 1].x * poly[i].y;
    }
    area * 0.5
}

fn is_convex(a: Point, b: Point, c: Point) -> bool {
    cross(a, b, c) > 0.0
}

fn cross(a: Point, b: Point, c: Point) -> f32 {
    (b.x - a.x) * (c.y - a.y) - (b.y - a.y) * (c.x - a.x)
}

fn contains_point(poly: &[Point], indices: &[usize], i0: usize, i1: usize, i2: usize) -> bool {
    let a = poly[i0];
    let b = poly[i1];
    let c = poly[i2];
    for &idx in indices {
        if idx == i0 || idx == i1 || idx == i2 {
            continue;
        }
        if point_in_triangle(poly[idx], a, b, c) {
            return true;
        }
    }
    false
}

fn point_in_triangle(p: Point, a: Point, b: Point, c: Point) -> bool {
    let c1 = cross(a, b, p);
    let c2 = cross(b, c, p);
    let c3 = cross(c, a, p);
    (c1 >= 0.0 && c2 >= 0.0 && c3 >= 0.0) || (c1 <= 0.0 && c2 <= 0.0 && c3 <= 0.0)
}

fn append_segment_quad(out: &mut Vec<Vertex>, a: Point, b: Point, width: f32, color: Color) {
    let dx = b.x - a.x;
    let dy = b.y - a.y;
    let len = (dx * dx + dy * dy).sqrt();
    if len == 0.0 {
        return;
    }
    let nx = -dy / len * width * 0.5;
    let ny = dx / len * width * 0.5;
    let p0 = Point {
        x: a.x + nx,
        y: a.y + ny,
    };
    let p1 = Point {
        x: b.x + nx,
        y: b.y + ny,
    };
    let p2 = Point {
        x: b.x - nx,
        y: b.y - ny,
    };
    let p3 = Point {
        x: a.x - nx,
        y: a.y - ny,
    };
    out.extend_from_slice(&[
        Vertex { pos: p0, color },
        Vertex { pos: p1, color },
        Vertex { pos: p2, color },
        Vertex { pos: p0, color },
        Vertex { pos: p2, color },
        Vertex { pos: p3, color },
    ]);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tessellates_rect_into_two_triangles() {
        let verts = tessellate_fill(
            &Path::rect(0.0, 0.0, 10.0, 20.0),
            Color {
                r: 1.0,
                g: 0.0,
                b: 0.0,
                a: 1.0,
            },
        );
        assert_eq!(verts.len(), 6);
    }

    #[test]
    fn tessellates_rounded_rect() {
        let verts = tessellate_fill(
            &Path::rounded_rect(0.0, 0.0, 20.0, 12.0, 3.0),
            Color {
                r: 1.0,
                g: 1.0,
                b: 0.0,
                a: 1.0,
            },
        );
        assert!(!verts.is_empty());
    }

    #[test]
    fn tessellates_circle_into_triangles() {
        let verts = tessellate_fill(
            &Path::circle(0.0, 0.0, 10.0),
            Color {
                r: 0.0,
                g: 1.0,
                b: 0.0,
                a: 1.0,
            },
        );
        assert!(verts.len() >= 6);
    }

    #[test]
    fn tessellates_stroke_segments() {
        let verts = tessellate_stroke(
            &Path::polyline(
                &[
                    Point { x: 0.0, y: 0.0 },
                    Point { x: 10.0, y: 0.0 },
                    Point { x: 10.0, y: 10.0 },
                ],
                false,
            ),
            2.0,
            Color {
                r: 0.0,
                g: 0.0,
                b: 1.0,
                a: 1.0,
            },
        );
        assert_eq!(verts.len(), 12);
    }
}
