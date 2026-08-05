#version 450

// Solid-quad fragment shader with analytic coverage AA (Q8 fallback).
//
// The reference driver's 4x/8x MSAA resolve averages to half intensity (see
// devdocs/notes/vulkan-equivalence-baseline.md), so the solid pipeline renders
// single-sampled and computes sub-pixel coverage in the shader instead. This
// matches the software oracle's analytic AA model and is driver-independent.
//
// The transformed rect is a parallelogram; the coverage is the product of the
// clamped signed distances to its four edges, normalized by the unit pixel's
// projection onto each edge normal. The world-space clip rect from the push
// constants is applied with a per-pixel discard (corpus clips are integer
// aligned, so the hard clip edge matches the oracle).

layout(location = 0) in vec4 v_color;
layout(location = 1) in vec2 v_world;
layout(location = 2) flat in vec4 v_rect;

layout(push_constant) uniform PushConstants {
    float transform[6];
    float opacity;
    float clip_min[2];
    float clip_size[2];
    uint clip_active;
    uint brush_kind;
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 out_color;

vec2 apply_transform(vec2 p) {
    return vec2(
        pc.transform[0] * p.x + pc.transform[1] * p.y + pc.transform[4],
        pc.transform[2] * p.x + pc.transform[3] * p.y + pc.transform[5]
    );
}

// Fraction of a unit pixel inside the half-plane on the left of directed edge
// (a -> b). World coordinates are pixel coordinates, so the signed distance d
// is in pixels. Near the pixel center the area changes at the rate of the
// edge's intersection length with the pixel, 1 / max(|nx|, |ny|) for a unit
// normal; the clamp keeps the transition over one pixel.
float edge_coverage(vec2 p, vec2 a, vec2 b) {
    vec2 ab = b - a;
    vec2 n = vec2(-ab.y, ab.x);
    float len = length(ab);
    if (len < 1e-6) {
        return 0.0;
    }
    vec2 un = n / len;
    float d = dot(p - a, un);
    float extent = max(abs(un.x), abs(un.y));
    return clamp(0.5 + d / extent, 0.0, 1.0);
}

void main() {
    if (pc.clip_active != 0u) {
        vec2 clip_max = vec2(pc.clip_min[0] + pc.clip_size[0], pc.clip_min[1] + pc.clip_size[1]);
        if (v_world.x < pc.clip_min[0] || v_world.x >= clip_max.x ||
            v_world.y < pc.clip_min[1] || v_world.y >= clip_max.y) {
            discard;
        }
    }

    vec2 c0 = apply_transform(v_rect.xy);
    vec2 c1 = apply_transform(v_rect.xy + vec2(v_rect.z, 0.0));
    vec2 c2 = apply_transform(v_rect.xy + v_rect.zw);
    vec2 c3 = apply_transform(v_rect.xy + vec2(0.0, v_rect.w));

    float cov = edge_coverage(v_world, c0, c1)
              * edge_coverage(v_world, c1, c2)
              * edge_coverage(v_world, c2, c3)
              * edge_coverage(v_world, c3, c0);

    out_color = vec4(v_color.rgb * pc.opacity, v_color.a * pc.opacity) * cov;
}
