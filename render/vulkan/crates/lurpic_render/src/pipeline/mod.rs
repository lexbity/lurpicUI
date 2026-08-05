//! GPU pipeline stages (Slice 3+: solid quads, then textures, glyphs,
//! gradients, stencil fill, blur).

pub mod solid;

/// Per-draw push constants (Q4). The layout is `#[repr(C)]` and matched
/// byte-for-byte by the shaders (see src/shaders/solid.vert). All fields are
/// 4-byte aligned so the struct maps to GLSL std430 without padding.
#[repr(C)]
#[derive(Clone, Copy, Debug)]
pub struct PushConstants {
    /// 2x3 affine transform: [a, b, c, d, tx, ty].
    pub transform: [f32; 6],
    pub opacity: f32,
    /// World-space clip rectangle min corner.
    pub clip_min: [f32; 2],
    /// World-space clip rectangle size.
    pub clip_size: [f32; 2],
    /// 1 when the clip is active.
    pub clip_active: u32,
    /// Brush kind (0 = solid, 1 = linear gradient).
    pub brush_kind: u32,
    /// Reserved brush payload (gradient stops in Slice 6).
    pub brush_payload: [f32; 8],
    /// Render target size in pixels, for the NDC conversion.
    pub surface_size: [f32; 2],
}

impl PushConstants {
    pub const SIZE: u32 = 92;

    pub fn layout() -> ash::vk::PushConstantRange {
        ash::vk::PushConstantRange {
            stage_flags: ash::vk::ShaderStageFlags::VERTEX | ash::vk::ShaderStageFlags::FRAGMENT,
            offset: 0,
            size: Self::SIZE,
        }
    }

    pub fn bytes(&self) -> [u8; 92] {
        let mut out = [0u8; 92];
        let mut off = 0usize;
        for v in self.transform.iter().chain(std::iter::once(&self.opacity)) {
            out[off..off + 4].copy_from_slice(&v.to_le_bytes());
            off += 4;
        }
        for v in self.clip_min.iter().chain(self.clip_size.iter()) {
            out[off..off + 4].copy_from_slice(&v.to_le_bytes());
            off += 4;
        }
        out[off..off + 4].copy_from_slice(&self.clip_active.to_le_bytes());
        off += 4;
        out[off..off + 4].copy_from_slice(&self.brush_kind.to_le_bytes());
        off += 4;
        for v in self.brush_payload.iter() {
            out[off..off + 4].copy_from_slice(&v.to_le_bytes());
            off += 4;
        }
        for v in self.surface_size.iter() {
            out[off..off + 4].copy_from_slice(&v.to_le_bytes());
            off += 4;
        }
        debug_assert_eq!(off, Self::SIZE as usize);
        out
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::mem;

    #[test]
    fn push_constants_layout_matches_shader() {
        assert_eq!(mem::size_of::<PushConstants>(), PushConstants::SIZE as usize);
        // Offsets match the GLSL std430 layout: all 4-byte aligned.
        assert_eq!(mem::offset_of!(PushConstants, transform), 0);
        assert_eq!(mem::offset_of!(PushConstants, opacity), 24);
        assert_eq!(mem::offset_of!(PushConstants, clip_min), 28);
        assert_eq!(mem::offset_of!(PushConstants, clip_size), 36);
        assert_eq!(mem::offset_of!(PushConstants, clip_active), 44);
        assert_eq!(mem::offset_of!(PushConstants, brush_kind), 48);
        assert_eq!(mem::offset_of!(PushConstants, brush_payload), 52);
        assert_eq!(mem::offset_of!(PushConstants, surface_size), 84);
    }

    #[test]
    fn push_constants_bytes_roundtrip() {
        let pc = PushConstants {
            transform: [1.0, 0.0, 0.0, 1.0, 10.0, 20.0],
            opacity: 0.5,
            clip_min: [4.0, 8.0],
            clip_size: [100.0, 200.0],
            clip_active: 1,
            brush_kind: 0,
            brush_payload: [0.0; 8],
            surface_size: [1920.0, 1080.0],
        };
        let bytes = pc.bytes();
        assert_eq!(&bytes[0..4], &1.0f32.to_le_bytes());
        assert_eq!(&bytes[16..20], &10.0f32.to_le_bytes());
        assert_eq!(&bytes[24..28], &0.5f32.to_le_bytes());
        assert_eq!(&bytes[28..32], &4.0f32.to_le_bytes());
        assert_eq!(&bytes[44..48], &1u32.to_le_bytes());
        assert_eq!(&bytes[84..88], &1920.0f32.to_le_bytes());
    }
}
