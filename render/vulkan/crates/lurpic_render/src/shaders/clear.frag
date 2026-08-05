#version 450

// Solid color output driven by a push constant.
layout(location = 0) out vec4 outColor;

layout(push_constant) uniform PushConstants {
    vec4 color;
} pc;

void main() {
    outColor = pc.color;
}
