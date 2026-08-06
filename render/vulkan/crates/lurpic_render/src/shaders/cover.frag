#version 450

// Solid path-fill cover shader (Slice 7, Q8 coverage-AA fallback). The cover
// quad spans the path's bounding box; the fragment shader evaluates the
// nonzero winding number at a full-pixel supersample grid over the flattened
// contour edges (bound via the set-1 storage buffer) and averages — the
// analytic-AA fallback for when per-sample stencil testing (MSAA) is
// unavailable on the reference driver. The winding math is shared with the
// gradient cover in coverage.glsl.

layout(location = 1) in vec2 v_world;
layout(location = 2) flat in vec4 v_rect;

layout(set = 1, binding = 0) readonly buffer Segments {
    vec2 data[];
} segments;

layout(push_constant) uniform PushConstants {
    float transform[6];
    float opacity;
    float clip_min[2];
    float clip_size[2];
    uint clip_active;
    uint brush_kind;
    // [0..3] = brush color (premultiplied), [4] = edge_count, [5] = base_edge.
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 out_color;

#include "coverage.glsl"

void main() {
    if (pc.clip_active != 0u) {
        vec2 clip_max = vec2(pc.clip_min[0] + pc.clip_size[0], pc.clip_min[1] + pc.clip_size[1]);
        if (v_world.x < pc.clip_min[0] || v_world.x >= clip_max.x ||
            v_world.y < pc.clip_min[1] || v_world.y >= clip_max.y) {
            discard;
        }
    }

    float cov = path_coverage(v_world);
    out_color = vec4(pc.brush_payload[0] * cov,
                     pc.brush_payload[1] * cov,
                     pc.brush_payload[2] * cov,
                     pc.brush_payload[3] * cov) * pc.opacity;
}
