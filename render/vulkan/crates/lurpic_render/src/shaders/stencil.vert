#version 450

// Stencil-pass vertex shader (Slice 7). Each winding triangle is three
// vertices: the contour edge's endpoints (a, b) as world-space points, plus a
// synthetic third vertex at the viewport bottom below the path (always below in
// NDC, so the winding contribution is correct for any transform). The bottom
// vertex's x is the path's world center (brush_payload[0]) so the triangles'
// horizontal extent stays bounded by the path.

layout(location = 0) in vec2 contour_pos;

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

void main() {
    if (gl_VertexIndex % 3 == 2) {
        // NDC y=+1 is the bottom edge (world y=0 -> NDC -1 in this renderer's
        // top-left origin convention), so +2 places the vertex below the
        // viewport for the downward winding ray.
        float cx = pc.brush_payload[0];
        gl_Position = vec4(cx / pc.surface_size[0] * 2.0 - 1.0, 2.0, 0.0, 1.0);
    } else {
        vec2 world = contour_pos;
        gl_Position = vec4(
            world.x / pc.surface_size[0] * 2.0 - 1.0,
            world.y / pc.surface_size[1] * 2.0 - 1.0,
            0.0,
            1.0
        );
    }
}
