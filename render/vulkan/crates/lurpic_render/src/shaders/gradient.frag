#version 450

// Gradient-quad fragment shader (Slice 6). Computes the linear-gradient color
// per pixel by projecting the world position onto the brush's start->end line
// (absolute-pixel coordinates from the push constants), then looking up the
// color across the stop array in the gradient UBO. The stop scan mirrors the
// software oracle's linear scan exactly (find the containing pair, lerp). The
// rect coverage AA is shared with the solid pipeline.

layout(location = 0) in vec4 v_color;
layout(location = 1) in vec2 v_world;
layout(location = 2) flat in vec4 v_rect;

layout(set = 0, binding = 0) uniform GradientUbo {
    uint stop_count;
    // 2 vec4s per stop: (offset, r, g, b) and (a, 0, 0, 0).
    vec4 data[32];
} gubo;

layout(push_constant) uniform PushConstants {
    float transform[6];
    float opacity;
    float clip_min[2];
    float clip_size[2];
    uint clip_active;
    uint brush_kind;
    // [0..3] = gradient start/end, [4] = stop_count, [5..6] = stop-content hash.
    float brush_payload[8];
    float surface_size[2];
} pc;

layout(location = 0) out vec4 out_color;

vec2 apply_transform(vec2 p) {
    return vec2(
        pc.transform[0] * p.x + pc.transform[1] * p.y + pc.transform[4],
        pc.transform[2] * p.x + pc.transform[3] * p.y + pc.transform[5]
    );
}

float edge_coverage(vec2 p, vec2 a, vec2 b) {
    vec2 ab = b - a;
    vec2 n = vec2(-ab.y, ab.x);
    float len = length(ab);
    if (len < 1e-6) {
        return 0.0;
    }
    vec2 un = n / len;
    float d = dot(p - a, un);
    float extent = max(abs(un.x), abs(un.y));
    return clamp(0.5 + d / extent, 0.0, 1.0);
}

// Color of stop i, packed as two vec4s.
vec4 stop_color(int i) {
    return vec4(gubo.data[2 * i].yzw, gubo.data[2 * i + 1].x);
}

float stop_offset(int i) {
    return gubo.data[2 * i].x;
}

void main() {
    if (pc.clip_active != 0u) {
        vec2 clip_max = vec2(pc.clip_min[0] + pc.clip_size[0], pc.clip_min[1] + pc.clip_size[1]);
        if (v_world.x < pc.clip_min[0] || v_world.x >= clip_max.x ||
            v_world.y < pc.clip_min[1] || v_world.y >= clip_max.y) {
            discard;
        }
    }

    vec2 c0 = apply_transform(v_rect.xy);
    vec2 c1 = apply_transform(v_rect.xy + vec2(v_rect.z, 0.0));
    vec2 c2 = apply_transform(v_rect.xy + v_rect.zw);
    vec2 c3 = apply_transform(v_rect.xy + vec2(0.0, v_rect.w));

    float cov = edge_coverage(v_world, c0, c1)
              * edge_coverage(v_world, c1, c2)
              * edge_coverage(v_world, c2, c3)
              * edge_coverage(v_world, c3, c0);

    vec2 start = vec2(pc.brush_payload[0], pc.brush_payload[1]);
    vec2 end = vec2(pc.brush_payload[2], pc.brush_payload[3]);
    vec2 d = end - start;
    float denom = dot(d, d);

    int n = int(gubo.stop_count);
    // Degenerate gradient (zero-length axis): the oracle returns the last stop.
    if (denom == 0.0 || n <= 0) {
        vec4 last = stop_color(max(n - 1, 0));
        out_color = vec4(last.rgb * pc.opacity, last.a * pc.opacity) * cov;
        return;
    }

    float t = (dot(v_world - start, d)) / denom;
    t = clamp(t, 0.0, 1.0);

    // Match the oracle's linear scan: first pair whose interval contains t;
    // fall back to the first/last stop when no interval matches.
    int ia = 0;
    int ib = max(n - 1, 0);
    float oa = stop_offset(0);
    float ob = stop_offset(max(n - 1, 0));
    vec4 ca = stop_color(0);
    vec4 cb = stop_color(max(n - 1, 0));
    for (int i = 0; i < n - 1; i++) {
        if (t >= stop_offset(i) && t <= stop_offset(i + 1)) {
            ia = i;
            ib = i + 1;
            oa = stop_offset(i);
            ob = stop_offset(i + 1);
            ca = stop_color(i);
            cb = stop_color(i + 1);
            break;
        }
    }

    vec4 col;
    if (ob == oa) {
        col = cb;
    } else {
        float f = (t - oa) / (ob - oa);
        col = mix(ca, cb, f);
    }

    out_color = vec4(col.rgb * pc.opacity, col.a * pc.opacity) * cov;
}
