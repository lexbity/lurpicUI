#version 450

// Shadow mask pass (Slice 9): renders the path's analytic coverage into the R8
// blur scratch as the shadow source. `inner` (brush_payload[6]) inverts the
// source to `1 - coverage` for inset shadows. The winding evaluation is shared
// with the path-fill cover (coverage.glsl); no stencil attachment is needed
// because the cover shader is the coverage authority (Q8 amendment).

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
    // [0..3] white brush (unused), [4] edge_count, [5] base_edge, [6] inner.
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 out_color;

#include "coverage.glsl"

void main() {
    float cov = path_coverage(v_world);
    float src = (pc.brush_payload[6] > 0.5) ? (1.0 - cov) : cov;
    out_color = vec4(src);
}
