#version 450

// Instanced glyph-quad vertex shader (Slice 5). Each instance is one glyph:
// `dest_rect` is the WORLD-space placement (origin + glyph offset, rounded to
// pixels) and `region_rect` is the glyph's rectangle inside the packed atlas
// (atlas-pixel coordinates). The dest rect is already in world/screen space —
// the software oracle rounds the transformed placement and blits the mask 1:1 —
// so the shader does NOT apply the push transform to it. The rects are passed
// flat so the fragment shader derives the mask texel directly from the world
// position (robust to interpolation rounding), matching the oracle's per-pixel
// mask blit.

layout(location = 0) in vec2 unit_quad_pos;
layout(location = 1) in vec4 dest_rect;
layout(location = 2) in vec4 region_rect;

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

layout(location = 0) out vec2 v_world;
layout(location = 1) flat out vec4 v_dest_rect;
layout(location = 2) flat out vec4 v_region_rect;

void main() {
    vec2 world = dest_rect.xy + unit_quad_pos * dest_rect.zw;
    vec2 ndc = vec2(
        world.x / pc.surface_size[0] * 2.0 - 1.0,
        world.y / pc.surface_size[1] * 2.0 - 1.0
    );
    gl_Position = vec4(ndc, 0.0, 1.0);
    v_world = world;
    v_dest_rect = dest_rect;
    v_region_rect = region_rect;
}
