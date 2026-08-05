#version 450

// Fullscreen triangle covering the whole viewport; the clear pipeline uses it
// to fill the render target with a push-constant color. Geometry is generated
// from gl_VertexIndex so no vertex buffer is required.
void main() {
    vec2 positions[3] = vec2[](
        vec2(-1.0, -1.0),
        vec2( 3.0, -1.0),
        vec2(-1.0,  3.0)
    );
    vec2 position = positions[gl_VertexIndex];
    gl_Position = vec4(position, 0.0, 1.0);
}
