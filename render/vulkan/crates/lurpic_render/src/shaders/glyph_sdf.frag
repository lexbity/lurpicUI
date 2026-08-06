#version 450

// SDF glyph fragment shader (Slice 5, sizes >= 24 px). The atlas G channel
// holds the normalized signed-distance field; `smoothstep` reconstructs the
// coverage from the distance with a ~1px transition (smoothing derived from the
// SDF spread = max(region size) * 0.35, matching the CPU generator), so large
// glyphs render crisply and scale-invariantly. The mask texel is derived from
// the world position exactly as in the bitmap variant.

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
    // Sample the distance field continuously (LINEAR filter interpolates
    // between SDF texels) at the pixel center, so smoothstep anti-aliases the
    // contour. At 1:1 this is the position half a texel into the glyph's
    // mask pixel.
    vec2 atlas_pos = v_region_rect.xy + (v_world - v_dest_rect.xy);
    vec2 uv = atlas_pos / tex_size;
    float d = texture(atlas, uv).g;

    // Standard SDF reconstruction (smoothstep over the contour). The CPU SDF
    // generator spreads distances by max(w,h)*0.35 px; smoothing is the
    // normalized width of a ~1 px transition centered on the contour (0.5).
    float spread = max(v_region_rect.z, v_region_rect.w) * 0.35;
    float smoothing = 0.25 / max(spread, 0.001);
    float alpha = smoothstep(0.5 - smoothing, 0.5 + smoothing, d) * pc.opacity;

    out_color = vec4(pc.brush_payload[0] * alpha,
                     pc.brush_payload[1] * alpha,
                     pc.brush_payload[2] * alpha,
                     pc.brush_payload[3] * alpha);
}
