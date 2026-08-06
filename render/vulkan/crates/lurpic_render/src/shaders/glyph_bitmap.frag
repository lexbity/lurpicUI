#version 450

// Bitmap glyph fragment shader (Slice 5, sizes < 24 px). The atlas R channel
// holds the coverage mask. Glyphs blit 1:1, so the mask texel is derived
// directly from the world position: texel_in_mask = floor(world - dest_rect),
// which is exact for any sample point within a dest pixel (robust to
// interpolation rounding). The premultiplied brush color is scaled by the mask
// and opacity, matching the software oracle's per-pixel mask blit.

layout(location = 0) in vec2 v_world;
layout(location = 1) flat in vec4 v_dest_rect;
layout(location = 2) flat in vec4 v_region_rect;

layout(set = 0, binding = 0) uniform sampler2D atlas;

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

void main() {
    if (pc.clip_active != 0u) {
        vec2 clip_max = vec2(pc.clip_min[0] + pc.clip_size[0], pc.clip_min[1] + pc.clip_size[1]);
        if (v_world.x < pc.clip_min[0] || v_world.x >= clip_max.x ||
            v_world.y < pc.clip_min[1] || v_world.y >= clip_max.y) {
            discard;
        }
    }

    vec2 tex_size = vec2(textureSize(atlas, 0));
    vec2 mask_texel = floor(v_world - v_dest_rect.xy);
    vec2 texel = v_region_rect.xy + mask_texel;
    vec2 uv = (texel + 0.5) / tex_size;
    float mask = texture(atlas, uv).r;

    float alpha = mask * pc.opacity;
    out_color = vec4(pc.brush_payload[0] * alpha,
                     pc.brush_payload[1] * alpha,
                     pc.brush_payload[2] * alpha,
                     pc.brush_payload[3] * alpha);
}
