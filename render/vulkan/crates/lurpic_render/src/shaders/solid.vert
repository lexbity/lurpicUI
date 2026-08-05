#version 450

// Instanced solid-quad vertex shader. Each instance is one axis-aligned rect:
// `unit_quad_pos` (vertex) selects the corner, `rect` (instance) carries the
// rect's xy + wh, `color` (instance) is the premultiplied brush color. The
// push-constant transform maps local rect space to world/screen pixels; the
// viewport conversion produces NDC with a top-left origin (Vulkan NDC has
// positive y downward, so world y maps without negation).

layout(location = 0) in vec2 unit_quad_pos;
layout(location = 1) in vec4 rect;
layout(location = 2) in vec4 color;

layout(push_constant) uniform PushConstants {
    float transform[6];   // 2x3 affine: a,b,c,d,tx,ty
    float opacity;
    float clip_min[2];
    float clip_size[2];
    uint clip_active;
    uint brush_kind;
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 v_color;
layout(location = 1) out vec2 v_world;
// The local rect (flat: identical for every vertex of the instance) so the
// fragment shader can derive the transformed edges for coverage AA.
layout(location = 2) flat out vec4 v_rect;

vec2 apply_transform(vec2 p) {
    return vec2(
        pc.transform[0] * p.x + pc.transform[1] * p.y + pc.transform[4],
        pc.transform[2] * p.x + pc.transform[3] * p.y + pc.transform[5]
    );
}

void main() {
    vec2 local = rect.xy + unit_quad_pos * rect.zw;
    vec2 world = apply_transform(local);
    vec2 ndc = vec2(
        world.x / pc.surface_size[0] * 2.0 - 1.0,
        world.y / pc.surface_size[1] * 2.0 - 1.0
    );
    gl_Position = vec4(ndc, 0.0, 1.0);
    v_color = color;
    v_world = world;
    v_rect = rect;
}
