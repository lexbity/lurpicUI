#version 450

// Trivial fragment shader for the stencil pass (Slice 7). The pipeline writes
// no color (color write mask 0); only the stencil buffer is updated.

layout(location = 0) out vec4 out_color;

void main() {
    out_color = vec4(0.0);
}
