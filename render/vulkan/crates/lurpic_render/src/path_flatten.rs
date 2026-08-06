//! CPU path flattening (Slice 7).
//!
//! Converts the decoded path verbs (lines, quadratic/cubic Beziers, closes)
//! into closed contours of line segments, subdividing curves until they deviate
//! from their chord by less than [`FLATTEN_TOLERANCE`] (0.5 px). The caller
//! supplies the point transform so flattening happens in world/screen space,
//! where the tolerance is meaningful under scaling.

use crate::geometry::{Path, Point, Verb};

/// Flattening tolerance in pixels (Slice 7: "adaptive tolerance ~0.5 px").
pub const FLATTEN_TOLERANCE: f32 = 0.5;

/// Maximum subdivision depth; bounds the recursion so a pathological curve
/// cannot overflow the stack. At depth 32 a segment is sub-atomic.
const MAX_DEPTH: u32 = 32;

/// Flattens `path` under `transform` into closed contours of world-space
/// points. Each contour's last point equals its first (the closing edge is
/// emitted when the path does not already close it). Empty and degenerate
/// contours (< 3 points) are dropped.
pub fn flatten_path(path: &Path, transform: impl Fn(Point) -> Point) -> Vec<Vec<Point>> {
    let mut contours: Vec<Vec<Point>> = Vec::new();
    let mut contour: Vec<Point> = Vec::new();
    let mut start: Option<Point> = None;
    let mut pen: Option<Point> = None;

    let close = |contour: &mut Vec<Point>, contours: &mut Vec<Vec<Point>>, start: Option<Point>| {
        // A contour needs at least three distinct points before the closing
        // edge; anything smaller is a degenerate zero-area line.
        if contour.len() >= 3 {
            if let Some(s) = start {
                if *contour.last().expect("non-empty") != s {
                    contour.push(s);
                }
            }
            contours.push(std::mem::take(contour));
        } else {
            contour.clear();
        }
    };

    for verb in &path.verbs {
        match *verb {
            Verb::MoveTo(p) => {
                close(&mut contour, &mut contours, start);
                let w = transform(p);
                contour.push(w);
                start = Some(w);
                pen = Some(w);
            }
            Verb::LineTo(p) => {
                let w = transform(p);
                contour.push(w);
                pen = Some(w);
            }
            Verb::QuadTo(cp, p) => {
                let p0 = pen.or(start).unwrap_or(Point { x: 0.0, y: 0.0 });
                let cpw = transform(cp);
                let pw = transform(p);
                flatten_quad(&mut contour, p0, cpw, pw, FLATTEN_TOLERANCE, 0);
                pen = Some(pw);
            }
            Verb::CubicTo(c1, c2, p) => {
                let p0 = pen.or(start).unwrap_or(Point { x: 0.0, y: 0.0 });
                let c1w = transform(c1);
                let c2w = transform(c2);
                let pw = transform(p);
                flatten_cubic(&mut contour, p0, c1w, c2w, pw, FLATTEN_TOLERANCE, 0);
                pen = Some(pw);
            }
            Verb::Close => {
                close(&mut contour, &mut contours, start);
                start = None;
                pen = None;
            }
        }
    }
    close(&mut contour, &mut contours, start);
    contours
}

/// Appends the flattened polyline of the quadratic Bezier (p0, cp, p1),
/// including p1. `p0` is already in `out`.
fn flatten_quad(out: &mut Vec<Point>, p0: Point, cp: Point, p1: Point, tol: f32, depth: u32) {
    if depth >= MAX_DEPTH {
        out.push(p1);
        return;
    }
    // Flatness: distance of the control point from the chord midpoint.
    let mid = Point {
        x: (p0.x + p1.x) * 0.5,
        y: (p0.y + p1.y) * 0.5,
    };
    let dev_x = cp.x - mid.x;
    let dev_y = cp.y - mid.y;
    if dev_x * dev_x + dev_y * dev_y <= tol * tol {
        out.push(p1);
        return;
    }
    let a = Point {
        x: (p0.x + cp.x) * 0.5,
        y: (p0.y + cp.y) * 0.5,
    };
    let b = Point {
        x: (cp.x + p1.x) * 0.5,
        y: (cp.y + p1.y) * 0.5,
    };
    let m = Point {
        x: (a.x + b.x) * 0.5,
        y: (a.y + b.y) * 0.5,
    };
    flatten_quad(out, p0, a, m, tol, depth + 1);
    flatten_quad(out, m, b, p1, tol, depth + 1);
}

/// Appends the flattened polyline of the cubic Bezier (p0, c1, c2, p1),
/// including p1. `p0` is already in `out`.
fn flatten_cubic(
    out: &mut Vec<Point>,
    p0: Point,
    c1: Point,
    c2: Point,
    p1: Point,
    tol: f32,
    depth: u32,
) {
    if depth >= MAX_DEPTH {
        out.push(p1);
        return;
    }
    // Flatness: both control points within `tol` of the chord line.
    let d1 = point_line_dist(c1, p0, p1);
    let d2 = point_line_dist(c2, p0, p1);
    if d1 <= tol && d2 <= tol {
        out.push(p1);
        return;
    }
    let a1 = mid(p0, c1);
    let a2 = mid(c1, c2);
    let a3 = mid(c2, p1);
    let b1 = mid(a1, a2);
    let b2 = mid(a2, a3);
    let m = mid(b1, b2);
    flatten_cubic(out, p0, a1, b1, m, tol, depth + 1);
    flatten_cubic(out, m, b2, a3, p1, tol, depth + 1);
}

fn mid(a: Point, b: Point) -> Point {
    Point {
        x: (a.x + b.x) * 0.5,
        y: (a.y + b.y) * 0.5,
    }
}

fn point_line_dist(p: Point, a: Point, b: Point) -> f32 {
    let abx = b.x - a.x;
    let aby = b.y - a.y;
    let len2 = abx * abx + aby * aby;
    if len2 <= f32::EPSILON {
        let dx = p.x - a.x;
        let dy = p.y - a.y;
        return (dx * dx + dy * dy).sqrt();
    }
    let t = ((p.x - a.x) * abx + (p.y - a.y) * aby) / len2;
    let px = a.x + t * abx;
    let py = a.y + t * aby;
    let dx = p.x - px;
    let dy = p.y - py;
    (dx * dx + dy * dy).sqrt()
}

/// Builds the winding-triangle vertex stream for the closed contours: each edge
/// (p_i -> p_{i+1}) becomes three vertices [a.x, a.y, b.x, b.y, 0, 0], where the
/// third slot is a dummy the stencil vertex shader replaces with the viewport-
/// bottom vertex (`gl_VertexIndex % 3 == 2`). Each triangle is therefore 3
/// vertices (6 floats).
pub fn winding_triangles(contours: &[Vec<Point>]) -> Vec<f32> {
    let mut out = Vec::with_capacity(contours.iter().map(|c| c.len().saturating_sub(1) * 6).sum());
    for contour in contours {
        for i in 0..contour.len().saturating_sub(1) {
            let a = contour[i];
            let b = contour[i + 1];
            out.push(a.x);
            out.push(a.y);
            out.push(b.x);
            out.push(b.y);
            out.push(0.0);
            out.push(0.0);
        }
    }
    out
}

/// The axis-aligned bounds of a set of contours, or `None` when empty.
pub fn contours_bounds(contours: &[Vec<Point>]) -> Option<(Point, Point)> {
    let mut min: Option<Point> = None;
    let mut max: Option<Point> = None;
    for contour in contours {
        for p in contour {
            let m = min.get_or_insert(*p);
            m.x = m.x.min(p.x);
            m.y = m.y.min(p.y);
            let m = max.get_or_insert(*p);
            m.x = m.x.max(p.x);
            m.y = m.y.max(p.y);
        }
    }
    match (min, max) {
        (Some(min), Some(max)) => Some((min, max)),
        _ => None,
    }
}

/// The axis-aligned bounds of the decoded path in LOCAL coordinates (the
/// control points' convex hull), used for the cover quad. `None` for an empty
/// or degenerate path.
pub fn path_bounds(path: &Path) -> Option<(Point, Point)> {
    let mut min: Option<Point> = None;
    let mut max: Option<Point> = None;
    let mut visit = |p: Point| {
        let m = min.get_or_insert(p);
        m.x = m.x.min(p.x);
        m.y = m.y.min(p.y);
        let m = max.get_or_insert(p);
        m.x = m.x.max(p.x);
        m.y = m.y.max(p.y);
    };
    for verb in &path.verbs {
        match *verb {
            Verb::MoveTo(p) | Verb::LineTo(p) => visit(p),
            Verb::QuadTo(cp, p) => {
                visit(cp);
                visit(p);
            }
            Verb::CubicTo(c1, c2, p) => {
                visit(c1);
                visit(c2);
                visit(p);
            }
            Verb::Close => {}
        }
    }
    match (min, max) {
        (Some(min), Some(max)) => Some((min, max)),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pt(x: f32, y: f32) -> Point {
        Point { x, y }
    }

    #[test]
    fn flattens_axis_aligned_rect() {
        let path = Path {
            verbs: vec![
                Verb::MoveTo(pt(0.0, 0.0)),
                Verb::LineTo(pt(10.0, 0.0)),
                Verb::LineTo(pt(10.0, 10.0)),
                Verb::LineTo(pt(0.0, 10.0)),
                Verb::Close,
            ],
        };
        let contours = flatten_path(&path, |p| p);
        assert_eq!(contours.len(), 1);
        let c = &contours[0];
        assert_eq!(c[0], pt(0.0, 0.0));
        assert_eq!(c.last(), Some(&pt(0.0, 0.0)), "contour must be closed");
        assert_eq!(c.len(), 5); // 4 corners + closing point
    }

    #[test]
    fn flattens_quad_to_polyline() {
        // A shallow quad (control near the chord) stays coarse; a tight one
        // subdivides. Both must stay within the tolerance of the true curve.
        let path = Path {
            verbs: vec![
                Verb::MoveTo(pt(0.0, 0.0)),
                Verb::QuadTo(pt(5.0, 20.0), pt(10.0, 0.0)),
                Verb::Close,
            ],
        };
        let contours = flatten_path(&path, |p| p);
        assert_eq!(contours.len(), 1);
        let c = &contours[0];
        assert!(c.len() >= 4, "a tight quad must subdivide, got {}", c.len());
        // Every flattened point must be near the true quadratic Bezier.
        for p in c {
            // Bezier B(t) = (1-t)^2 P0 + 2(1-t)t C + t^2 P1; find t via the
            // x-projection (monotone here) and check y closeness.
            let t = (p.x / 10.0).clamp(0.0, 1.0);
            let u = 1.0 - t;
            let by = u * u * 0.0 + 2.0 * u * t * 20.0 + t * t * 0.0;
            assert!(
                (p.y - by).abs() <= FLATTEN_TOLERANCE + 0.001,
                "flattened point {:?} deviates from the quad at t={}",
                p,
                t
            );
        }
    }

    #[test]
    fn transforms_points_before_flattening() {
        let path = Path {
            verbs: vec![
                Verb::MoveTo(pt(0.0, 0.0)),
                Verb::LineTo(pt(10.0, 0.0)),
                Verb::LineTo(pt(10.0, 10.0)),
                Verb::Close,
            ],
        };
        let contours = flatten_path(&path, |p| pt(p.x * 2.0, p.y + 5.0));
        assert_eq!(contours.len(), 1);
        assert_eq!(contours[0][0], pt(0.0, 5.0));
        assert_eq!(contours[0][1], pt(20.0, 5.0));
    }

    #[test]
    fn drops_degenerate_contours() {
        let path = Path {
            verbs: vec![Verb::MoveTo(pt(0.0, 0.0)), Verb::LineTo(pt(5.0, 5.0)), Verb::Close],
        };
        let contours = flatten_path(&path, |p| p);
        assert!(contours.is_empty(), "a 2-point contour is degenerate");
    }

    #[test]
    fn winding_triangles_emit_three_vertices_per_edge() {
        let contours = vec![vec![pt(0.0, 0.0), pt(10.0, 0.0), pt(10.0, 10.0), pt(0.0, 0.0)]];
        let tri = winding_triangles(&contours);
        // 3 edges -> 9 vertices -> 18 floats.
        assert_eq!(tri.len(), 18);
        assert_eq!(&tri[0..2], &[0.0, 0.0]);
        assert_eq!(&tri[2..4], &[10.0, 0.0]);
        assert_eq!(&tri[4..6], &[0.0, 0.0]); // dummy
        assert_eq!(&tri[6..8], &[10.0, 0.0]);
        assert_eq!(&tri[8..10], &[10.0, 10.0]);
    }
}

#[cfg(test)]
mod pentagon_tests {
    use super::*;

    fn pt(x: f32, y: f32) -> Point {
        Point { x, y }
    }

    fn pentagon() -> Path {
        Path {
            verbs: vec![
                Verb::MoveTo(pt(32.0, 8.0)),
                Verb::LineTo(pt(52.0, 24.0)),
                Verb::LineTo(pt(44.0, 48.0)),
                Verb::LineTo(pt(20.0, 48.0)),
                Verb::LineTo(pt(12.0, 24.0)),
                Verb::Close,
            ],
        }
    }

    #[test]
    fn pentagon_flattens_to_5_edges() {
        let path = pentagon();
        let contours = flatten_path(&path, |p| p);
        assert_eq!(contours.len(), 1, "one contour");
        let c = &contours[0];
        assert_eq!(c.len(), 6, "6 contour points (5 edges)");
        assert_eq!(c[0], pt(32.0, 8.0));
        assert_eq!(c[3], pt(20.0, 48.0));
        assert_eq!(c[4], pt(12.0, 24.0));
        assert_eq!(c[5], pt(32.0, 8.0), "closed");
    }

    #[test]
    fn pentagon_winding_triangles_have_5_edges() {
        let path = pentagon();
        let contours = flatten_path(&path, |p| p);
        let tri = winding_triangles(&contours);
        assert_eq!(tri.len(), 30, "5 edges * 6 floats");
        // Edge 3 = (20,48)->(12,24): floats at index 18..24.
        assert_eq!(&tri[18..20], &[20.0, 48.0]);
        assert_eq!(&tri[20..22], &[12.0, 24.0]);
    }
}
