#version 450

// Separable Gaussian blur, vertical pass (Slice 9). Mirrors blur_horizontal.frag
// over the Y axis; the V pass reads the H-blurred scratch and writes the final
// blurred coverage back into the mask scratch.

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
    // [0] sigma = blur_radius / 3, [1] radius = round(blur_radius).
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 out_color;

void main() {
    float sigma = max(pc.brush_payload[0], 0.001);
    int radius = int(pc.brush_payload[1] + 0.5);
    vec2 tex_size = vec2(textureSize(tex, 0));
    float inv2s = 1.0 / (2.0 * sigma * sigma);
    float acc = 0.0;
    float total = 0.0;
    for (int i = -radius; i <= radius; i++) {
        float w = exp(-float(i * i) * inv2s);
        vec2 uv = (v_uv + vec2(0.0, float(i))) / tex_size;
        acc += w * textureLod(tex, uv, 0.0).r;
        total += w;
    }
    out_color = vec4(acc / max(total, 1e-6));
}
