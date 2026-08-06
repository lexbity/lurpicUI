#version 450

// Shadow composite (Slice 9): a textured quad over the shadow's offset region
// sampling the blurred R8 scratch (textured.vert maps dest = blur_region + offset,
// src = blur_region). The shadow color (premultiplied, brush_payload[0..3]) tints
// the blurred coverage and the state opacity applies. Premultiplied-over blending
// composites against the target, matching the software oracle.

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
    // [0..3] premultiplied shadow color.
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
    float a = textureLod(tex, v_uv / tex_size, 0.0).r;
    vec4 shadow = vec4(
        pc.brush_payload[0],
        pc.brush_payload[1],
        pc.brush_payload[2],
        pc.brush_payload[3]
    );
    out_color = shadow * (a * pc.opacity);
}
