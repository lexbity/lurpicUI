#version 450

// Instanced textured-quad vertex shader (Slice 4). Each instance is one quad:
// `dest_rect` (local xywh) and `src_rect` (texture-space xywh). The push
// constant transform maps local rect space to world/screen pixels, matching the
// solid pipeline. The UV is derived from the local normalized position so a
// 2D affine transform of the parallelogram maps to the texture rect exactly
// (the inverse of an affine map is affine, so the interpolated UV is correct
// even for rotated/scaled dest rects).

layout(location = 0) in vec2 unit_quad_pos;
layout(location = 1) in vec4 dest_rect;
layout(location = 2) in vec4 src_rect;

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

layout(location = 0) out vec2 v_uv;
layout(location = 1) out vec2 v_world;

vec2 apply_transform(vec2 p) {
    return vec2(
        pc.transform[0] * p.x + pc.transform[1] * p.y + pc.transform[4],
        pc.transform[2] * p.x + pc.transform[3] * p.y + pc.transform[5]
    );
}

void main() {
    vec2 local = dest_rect.xy + unit_quad_pos * dest_rect.zw;
    vec2 world = apply_transform(local);
    vec2 ndc = vec2(
        world.x / pc.surface_size[0] * 2.0 - 1.0,
        world.y / pc.surface_size[1] * 2.0 - 1.0
    );
    gl_Position = vec4(ndc, 0.0, 1.0);
    v_uv = src_rect.xy + unit_quad_pos * src_rect.zw;
    v_world = world;
}
