#version 450

// Gradient path-fill cover shader (Slice 7, Q8 coverage-AA fallback). Combines
// the Slice 6 linear-gradient color with the nonzero-winding full-pixel
// supersample coverage (see coverage.glsl; the gradient cover shares the same
// winding + coverage evaluation as the solid cover, so both honor the same
// flattened edges and the same AA model).

layout(location = 1) in vec2 v_world;
layout(location = 2) flat in vec4 v_rect;

layout(set = 0, binding = 0) uniform GradientUbo {
    uint stop_count;
    vec4 data[32];
} gubo;

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
    // [0..3] = gradient start/end, [4] = edge_count, [5] = base_edge.
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 out_color;

#include "coverage.glsl"

vec4 stop_color(int i) {
    return vec4(gubo.data[2 * i].yzw, gubo.data[2 * i + 1].x);
}

void main() {
    if (pc.clip_active != 0u) {
        vec2 clip_max = vec2(pc.clip_min[0] + pc.clip_size[0], pc.clip_min[1] + pc.clip_size[1]);
        if (v_world.x < pc.clip_min[0] || v_world.x >= clip_max.x ||
            v_world.y < pc.clip_min[1] || v_world.y >= clip_max.y) {
            discard;
        }
    }

    float cov = path_coverage(v_world);
    if (cov <= 0.0) {
        out_color = vec4(0.0);
        return;
    }

    vec2 start = vec2(pc.brush_payload[0], pc.brush_payload[1]);
    vec2 end = vec2(pc.brush_payload[2], pc.brush_payload[3]);
    vec2 d = end - start;
    float denom = dot(d, d);
    int n = int(gubo.stop_count);
    if (denom == 0.0 || n <= 0) {
        vec4 last = stop_color(max(n - 1, 0));
        out_color = vec4(last.rgb * cov, last.a * cov) * pc.opacity;
        return;
    }
    float t = (dot(v_world - start, d)) / denom;
    t = clamp(t, 0.0, 1.0);

    int ia = 0;
    int ib = max(n - 1, 0);
    float oa = gubo.data[0].x;
    float ob = gubo.data[2 * max(n - 1, 0)].x;
    vec4 ca = stop_color(0);
    vec4 cb = stop_color(max(n - 1, 0));
    for (int i = 0; i < n - 1; i++) {
        if (t >= gubo.data[2 * i].x && t <= gubo.data[2 * (i + 1)].x) {
            ia = i;
            ib = i + 1;
            oa = gubo.data[2 * i].x;
            ob = gubo.data[2 * (i + 1)].x;
            ca = stop_color(i);
            cb = stop_color(i + 1);
            break;
        }
    }

    vec4 col;
    if (ob == oa) {
        col = cb;
    } else {
        float f = (t - oa) / (ob - oa);
        col = mix(ca, cb, f);
    }
    out_color = vec4(col.rgb * cov, col.a * cov) * pc.opacity;
}
