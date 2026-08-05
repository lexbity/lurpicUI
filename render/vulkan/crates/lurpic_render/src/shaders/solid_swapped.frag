#version 450

// Test-only fragment shader: identical to solid.frag except the red and green
// channels of the brush color are deliberately swapped. This is the spec's
// negative control: it proves the equivalence harness catches a real shader
// regression through the actual shader toolchain (glslc -> SPIR-V -> pipeline)
// rather than a post-processing byte swap.

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

    // DELIBERATE REGRESSION: red and green swapped.
    out_color = vec4(v_color.g, v_color.r, v_color.b, v_color.a) * pc.opacity * cov;
}
