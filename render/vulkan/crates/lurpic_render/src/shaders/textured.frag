#version 450

// Textured-quad fragment shader (Slice 4). The sampled texture bytes follow the
// Go image.RGBA convention: premultiplied alpha. The fragment outputs
// premultiplied color times the combined state + draw opacity, which the
// premultiplied-over blend state composites against the backdrop.
//
// v_uv is in source-pixel (texel) coordinates (src.X + t*src.W), identical to
// the oracle's sx/sy, so the texel index is directly comparable. Nearest
// sampling matches the software oracle's round-to-nearest texel selection:
// texel = floor(v_uv + 0.5) with edge clamping, re-centered before sampling so
// the NEAREST sampler's floor-based lookup lands on the round-selected texel.
// Bilinear normalizes v_uv and uses the sampler's linear filter with
// CLAMP_TO_EDGE addressing, which the software oracle's bilinear path mirrors.

layout(location = 0) in vec2 v_uv;
layout(location = 1) in vec2 v_world;

layout(set = 0, binding = 0) uniform sampler2D tex;

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

    vec2 tex_size = vec2(textureSize(tex, 0));
    vec4 c;
    if (pc.brush_payload[0] < 0.5) {
        // Nearest: re-center on the round-selected texel so the NEAREST
        // sampler's floor-based lookup matches the oracle's round-based one.
        vec2 texel = floor(v_uv + 0.5);
        vec2 centered = (texel + 0.5) / tex_size;
        c = texture(tex, centered);
    } else {
        // Bilinear with CLAMP_TO_EDGE addressing.
        c = texture(tex, v_uv / tex_size);
    }

    out_color = vec4(c.rgb, c.a) * pc.opacity;
}
