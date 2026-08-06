//! GPU pipeline stages (Slice 3+: solid quads, textures, glyphs, gradients,
//! stencil fill, blur).

pub mod glyph;
pub mod gradient;
pub mod solid;
pub mod stencil;
pub mod textured;

use crate::gpu::context::GpuContext;
use crate::gpu::resources::{Pipeline, PipelineLayout, ShaderModule};
use crate::RenderResult;

/// Push-constant `brush_kind` values shared by the pipeline shaders and the
/// frame encoder. `solid`, `textured`, `glyph`, and `linear_gradient` are
/// rendered today (Slices 3-6).
pub const BRUSH_SOLID: u32 = 0;
pub const BRUSH_LINEAR_GRADIENT: u32 = 1;
pub const BRUSH_TEXTURED: u32 = 2;
pub const BRUSH_GLYPH: u32 = 3;

/// The stencil attachment format for the path-fill pipelines (Slice 7). The
/// depth aspect is unused; D24_UNORM_S8_UINT is universally supported.
pub const STENCIL_FORMAT: ash::vk::Format = ash::vk::Format::D24_UNORM_S8_UINT;

/// Shared builder for the instanced-quad pipelines (solid, textured, glyph,
/// gradient, and the stencil-cover variants). The vertex-input state is caller
/// supplied (quad pipelines bind the unit quad + instance rect; the stencil
/// pipeline binds contour points). Returns the pipeline and its layout.
pub(crate) fn build_graphics_pipeline<'a>(
    ctx: &dyn GpuContext,
    vert: &[u8],
    frag: &[u8],
    set_layouts: &[ash::vk::DescriptorSetLayout],
    vertex_input: ash::vk::PipelineVertexInputStateCreateInfo<'a>,
    color_format: ash::vk::Format,
    samples: ash::vk::SampleCountFlags,
    color_write_mask: ash::vk::ColorComponentFlags,
    stencil: Option<(
        &'a ash::vk::PipelineDepthStencilStateCreateInfo<'a>,
        ash::vk::Format,
    )>,
) -> Result<(Pipeline, PipelineLayout), (RenderResult, String)> {
    use ash::vk;

    let device = ctx.device();

    let vert = ShaderModule::new(device, &spv_words(vert))?;
    let frag = ShaderModule::new(device, &spv_words(frag))?;
    let layout = PipelineLayout::new(device, set_layouts, &[PushConstants::layout()])?;

    let stages = [
        vk::PipelineShaderStageCreateInfo {
            stage: vk::ShaderStageFlags::VERTEX,
            module: vert.handle(),
            p_name: c"main".as_ptr(),
            ..Default::default()
        },
        vk::PipelineShaderStageCreateInfo {
            stage: vk::ShaderStageFlags::FRAGMENT,
            module: frag.handle(),
            p_name: c"main".as_ptr(),
            ..Default::default()
        },
    ];

    let input_assembly = vk::PipelineInputAssemblyStateCreateInfo {
        topology: vk::PrimitiveTopology::TRIANGLE_LIST,
        ..Default::default()
    };
    let viewport_state = vk::PipelineViewportStateCreateInfo {
        viewport_count: 1,
        scissor_count: 1,
        ..Default::default()
    };
    let rasterization = vk::PipelineRasterizationStateCreateInfo {
        polygon_mode: vk::PolygonMode::FILL,
        line_width: 1.0,
        cull_mode: vk::CullModeFlags::NONE,
        front_face: vk::FrontFace::COUNTER_CLOCKWISE,
        ..Default::default()
    };
    let multisample = vk::PipelineMultisampleStateCreateInfo {
        rasterization_samples: samples,
        ..Default::default()
    };

    // Premultiplied-alpha "over": the fragment shader emits premultiplied
    // color, so the source factor is ONE.
    let color_attachment = vk::PipelineColorBlendAttachmentState {
        blend_enable: vk::TRUE,
        src_color_blend_factor: vk::BlendFactor::ONE,
        dst_color_blend_factor: vk::BlendFactor::ONE_MINUS_SRC_ALPHA,
        color_blend_op: vk::BlendOp::ADD,
        src_alpha_blend_factor: vk::BlendFactor::ONE,
        dst_alpha_blend_factor: vk::BlendFactor::ONE_MINUS_SRC_ALPHA,
        alpha_blend_op: vk::BlendOp::ADD,
        color_write_mask,
        ..Default::default()
    };
    let color_blend = vk::PipelineColorBlendStateCreateInfo {
        attachment_count: 1,
        p_attachments: &color_attachment,
        ..Default::default()
    };
    let dynamic_states = [vk::DynamicState::VIEWPORT, vk::DynamicState::SCISSOR];
    let dynamic_state = vk::PipelineDynamicStateCreateInfo {
        dynamic_state_count: dynamic_states.len() as u32,
        p_dynamic_states: dynamic_states.as_ptr(),
        ..Default::default()
    };

    let color_formats = [color_format];
    let mut rendering_info =
        vk::PipelineRenderingCreateInfo::default().color_attachment_formats(&color_formats);
    if let Some((_, stencil_format)) = stencil {
        rendering_info.stencil_attachment_format = stencil_format;
    }

    let mut create_info = vk::GraphicsPipelineCreateInfo::default()
        .stages(&stages)
        .vertex_input_state(&vertex_input)
        .input_assembly_state(&input_assembly)
        .viewport_state(&viewport_state)
        .rasterization_state(&rasterization)
        .multisample_state(&multisample)
        .color_blend_state(&color_blend)
        .dynamic_state(&dynamic_state)
        .push_next(&mut rendering_info)
        .layout(layout.handle())
        .render_pass(vk::RenderPass::null())
        .subpass(0);
    if let Some((stencil_state, _)) = stencil {
        create_info = create_info.depth_stencil_state(stencil_state);
    }

    let create_infos = [create_info];
    let pipeline = Pipeline::new(device, ctx.pipeline_cache(), &create_infos)?;
    Ok((pipeline, layout))
}

/// The unit-quad + instance-rect vertex input shared by every instanced quad
/// pipeline: binding 0 = unit quad corner (vec2, VERTEX), binding 1 = instance
/// rect (two vec4s, INSTANCE).
const QUAD_BINDINGS: [ash::vk::VertexInputBindingDescription; 2] = [
    ash::vk::VertexInputBindingDescription {
        binding: 0,
        stride: 2 * 4,
        input_rate: ash::vk::VertexInputRate::VERTEX,
    },
    ash::vk::VertexInputBindingDescription {
        binding: 1,
        stride: 8 * 4,
        input_rate: ash::vk::VertexInputRate::INSTANCE,
    },
];
const QUAD_ATTRIBUTES: [ash::vk::VertexInputAttributeDescription; 3] = [
    ash::vk::VertexInputAttributeDescription {
        location: 0,
        binding: 0,
        format: ash::vk::Format::R32G32_SFLOAT,
        offset: 0,
    },
    ash::vk::VertexInputAttributeDescription {
        location: 1,
        binding: 1,
        format: ash::vk::Format::R32G32B32A32_SFLOAT,
        offset: 0,
    },
    ash::vk::VertexInputAttributeDescription {
        location: 2,
        binding: 1,
        format: ash::vk::Format::R32G32B32A32_SFLOAT,
        offset: 16,
    },
];

pub(crate) fn quad_vertex_input() -> ash::vk::PipelineVertexInputStateCreateInfo<'static> {
    ash::vk::PipelineVertexInputStateCreateInfo {
        vertex_binding_description_count: QUAD_BINDINGS.len() as u32,
        p_vertex_binding_descriptions: QUAD_BINDINGS.as_ptr(),
        vertex_attribute_description_count: QUAD_ATTRIBUTES.len() as u32,
        p_vertex_attribute_descriptions: QUAD_ATTRIBUTES.as_ptr(),
        ..Default::default()
    }
}

/// The stencil-cover depth/stencil state. Q8 amendment (reference driver): the
/// 4x/8x MSAA resolve is broken, so per-sample stencil testing cannot provide
/// path-fill AA. The cover shader is therefore the coverage authority — it
/// supersamples the nonzero winding from the same flattened edges the stencil
/// pass accumulates — and the hardware stencil test is relaxed to ALWAYS so
/// edge pixels whose sample point lands outside the path (and would otherwise
/// be rejected) still reach the shader and contribute their partial coverage.
/// The stencil pass remains the winding accumulator (FR-5) and the mechanism a
/// working MSAA driver would gate on (compare NOT_EQUAL reference 0).
pub(crate) fn cover_stencil_state() -> ash::vk::PipelineDepthStencilStateCreateInfo<'static> {
    use ash::vk;
    let op = vk::StencilOpState {
        fail_op: vk::StencilOp::KEEP,
        pass_op: vk::StencilOp::KEEP,
        depth_fail_op: vk::StencilOp::KEEP,
        compare_op: vk::CompareOp::ALWAYS,
        compare_mask: 0xFF,
        write_mask: 0,
        reference: 0,
    };
    vk::PipelineDepthStencilStateCreateInfo {
        depth_test_enable: vk::FALSE,
        depth_write_enable: vk::FALSE,
        depth_compare_op: vk::CompareOp::ALWAYS,
        stencil_test_enable: vk::TRUE,
        front: op,
        back: op,
        ..Default::default()
    }
}

/// The winding-pass depth/stencil state: always passes the test and accumulates
/// the winding number (INCR on front faces, DECR on back). The nonzero test is
/// sign-invariant, so this convention matches the oracle's nonzero fill for
/// either contour orientation.
pub(crate) fn winding_stencil_state() -> ash::vk::PipelineDepthStencilStateCreateInfo<'static> {
    use ash::vk;
    let front = vk::StencilOpState {
        fail_op: vk::StencilOp::INCREMENT_AND_WRAP,
        pass_op: vk::StencilOp::INCREMENT_AND_WRAP,
        depth_fail_op: vk::StencilOp::INCREMENT_AND_WRAP,
        compare_op: vk::CompareOp::ALWAYS,
        compare_mask: 0xFF,
        write_mask: 0xFF,
        reference: 0,
    };
    let back = vk::StencilOpState {
        fail_op: vk::StencilOp::DECREMENT_AND_WRAP,
        pass_op: vk::StencilOp::DECREMENT_AND_WRAP,
        depth_fail_op: vk::StencilOp::DECREMENT_AND_WRAP,
        compare_op: vk::CompareOp::ALWAYS,
        compare_mask: 0xFF,
        write_mask: 0xFF,
        reference: 0,
    };
    vk::PipelineDepthStencilStateCreateInfo {
        depth_test_enable: vk::FALSE,
        depth_write_enable: vk::FALSE,
        depth_compare_op: vk::CompareOp::ALWAYS,
        stencil_test_enable: vk::TRUE,
        front,
        back,
        ..Default::default()
    }
}

pub(crate) fn spv_words(bytes: &[u8]) -> Vec<u32> {
    bytes
        .chunks_exact(4)
        .map(|c| u32::from_le_bytes([c[0], c[1], c[2], c[3]]))
        .collect()
}

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

    /// The stencil winding pass's push constants (Slice 7): the world-space
    /// center x of the path in `brush_payload[0]` so the bottom vertex keeps
    /// the winding triangles bounded; transform/clip/opacity are unused.
    pub fn default_for_stencil(bottom_center_x: f32, surface_size: [f32; 2]) -> Self {
        Self {
            transform: [1.0, 0.0, 0.0, 1.0, 0.0, 0.0],
            opacity: 1.0,
            clip_min: [0.0; 2],
            clip_size: [0.0; 2],
            clip_active: 0,
            brush_kind: 0,
            brush_payload: [
                bottom_center_x, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
            ],
            surface_size,
        }
    }

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
        assert_eq!(
            mem::size_of::<PushConstants>(),
            PushConstants::SIZE as usize
        );
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
