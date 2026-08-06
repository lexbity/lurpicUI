// Shared path-fill winding + coverage evaluation (Slice 7, Q8 coverage-AA
// fallback). Included textually by cover.frag and cover_gradient.frag, which
// declare the `segments` storage buffer and the `pc` push-constant block this
// file references. The include guard keeps a double-include safe.
//
// The cover fragment shader evaluates the nonzero winding number over the
// flattened contour edges at sub-pixel positions to derive coverage, because
// the reference driver's MSAA resolve cannot provide per-sample coverage (Q8
// amendment; 4x/8x resolve half intensity and 4x additionally loses the device
// after the first frame). The winding data lives in the set-1 storage buffer:
// edge i occupies vec2 entries `(base + i) * 3` and `(base + i) * 3 + 1`
// (a, b); the third slot is the stencil winding-triangle's dummy vertex.
//
// Coverage is a full-pixel supersample: a `COVERAGE_GRID` x `COVERAGE_GRID`
// grid across the pixel, sample offsets `(k + 0.5)/GRID - 0.5` relative to the
// pixel center. A 3x3 corner/edge-midpoint/center probe grid returns exact
// coverage for fully-inside and fully-outside pixels (the overwhelming
// majority), so only the 1px edge band pays the full grid cost. The probe grid
// catches the winding-region boundary wherever it crosses the pixel's
// perimeter or center; a path feature narrower than the probe spacing that
// lies entirely between probes (a sub-pixel sliver) is a documented model
// bound (the oracle's exact polygon-area coverage, which the Q8 coverage model
// approximates on every edge anyway). The grid's worst-case coverage error on
// a straight edge is ~1/(2*GRID) of a pixel (16/255 at GRID=8).

#ifndef COVERAGE_GLSL
#define COVERAGE_GLSL

#define COVERAGE_GRID 12

// Nonzero winding number at `p` (world/pixel coordinates) over the flattened
// contour edges. Division-free ray-cast: for a straddling edge, `xint > p.x`
// is rewritten by multiplying through by `(b.y - a.y)` (its sign selects the
// comparison direction), so the inner loop needs no divide (a slow op on
// mobile GPUs). The test is bit-exact with the `xint = a.x + (p.y - a.y) /
// (b.y - a.y) * (b.x - a.x)` form except where the edge is horizontal, which
// the straddle test `(a.y <= p.y) != (b.y <= p.y)` excludes.
int winding_at(vec2 p) {
    int winding = 0;
    int edge_count = int(pc.brush_payload[4]);
    int base_edge = floatBitsToInt(pc.brush_payload[5]);
    for (int i = 0; i < edge_count; i++) {
        vec2 a = segments.data[(base_edge + i) * 3];
        vec2 b = segments.data[(base_edge + i) * 3 + 1];
        if ((a.y <= p.y) != (b.y <= p.y)) {
            float lhs = (p.y - a.y) * (b.x - a.x);
            float rhs = (p.x - a.x) * (b.y - a.y);
            if (b.y > a.y) {
                if (lhs > rhs) {
                    winding += 1;
                }
            } else if (lhs < rhs) {
                winding -= 1;
            }
        }
    }
    return winding;
}

// Coverage of the path within the pixel centered at `world`, in [0, 1].
float path_coverage(vec2 world) {
    // Fast interior: all four pixel corners and the center inside -> fully
    // covered. This is the dominant case for filled paths (the cover quad is
    // the path's bounds), so it costs only five winding evals per interior
    // pixel. A sub-pixel winding-0 sliver lying entirely between those probes
    // is the documented model bound.
    int p0 = winding_at(world + vec2(-0.5, -0.5));
    int p1 = winding_at(world + vec2(0.5, -0.5));
    int p2 = winding_at(world + vec2(-0.5, 0.5));
    int p3 = winding_at(world + vec2(0.5, 0.5));
    int p4 = winding_at(world);
    if (p0 != 0 && p1 != 0 && p2 != 0 && p3 != 0 && p4 != 0) {
        return 1.0;
    }

    // Edge band or hollow region: probe the four edge midpoints too; only when
    // all nine probes are outside can the pixel be safely empty (this catches
    // a sub-pixel tip/wedge that enters the pixel without touching a corner,
    // e.g. the vertex tips of the self-intersecting bowtie fixture).
    int p5 = winding_at(world + vec2(0.0, -0.5));
    int p6 = winding_at(world + vec2(-0.5, 0.0));
    int p7 = winding_at(world + vec2(0.5, 0.0));
    int p8 = winding_at(world + vec2(0.0, 0.5));
    if (p0 == 0 && p1 == 0 && p2 == 0 && p3 == 0 && p4 == 0 &&
        p5 == 0 && p6 == 0 && p7 == 0 && p8 == 0) {
        return 0.0;
    }

    float cov = 0.0;
    for (int i = 0; i < COVERAGE_GRID; i++) {
        float ox = (float(i) + 0.5) / float(COVERAGE_GRID) - 0.5;
        for (int j = 0; j < COVERAGE_GRID; j++) {
            float oy = (float(j) + 0.5) / float(COVERAGE_GRID) - 0.5;
            cov += (winding_at(world + vec2(ox, oy)) != 0) ? 1.0 : 0.0;
        }
    }
    return cov / float(COVERAGE_GRID * COVERAGE_GRID);
}

#endif // COVERAGE_GLSL
