//! The path-fill pipelines (Slice 7): a stencil winding pass and the stencil-
//! gated cover passes (solid and gradient brushes). The stencil pass renders
//! the flattened path's winding triangles (see `path_flatten.rs`), accumulating
//! the nonzero winding number into the stencil attachment; the cover pass then
//! fills the path's bounding quad with the brush, keeping only fragments where
//! the accumulated winding is nonzero.

use ash::vk;

use crate::gpu::allocator::{GpuBuffer, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{DescriptorSetLayout, Pipeline, PipelineLayout};
use crate::pipeline::{
    build_graphics_pipeline, cover_stencil_state, quad_vertex_input, winding_stencil_state,
    STENCIL_FORMAT,
};
use crate::RenderResult;

const STENCIL_VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/stencil.vert.spv"));
const STENCIL_FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/stencil.frag.spv"));
const SOLID_VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/solid.vert.spv"));
const COVER_FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/cover.frag.spv"));
const COVER_GRADIENT_FRAG_SPV: &[u8] =
    include_bytes!(concat!(env!("OUT_DIR"), "/cover_gradient.frag.spv"));

/// Descriptor set layout binding for the gradient cover pipeline: set 0
/// binding 0 is the gradient-stops uniform buffer (shared with `gradient.rs`).
pub const UBO_BINDING: u32 = 0;

/// Set 1 binding 0 is the readonly storage buffer of flattened contour edges
/// (the winding-triangle stream from the path ring) the cover shaders evaluate.
pub const SEGMENTS_BINDING: u32 = 0;

const UNIT_QUAD: [[f32; 2]; 6] = [
    [0.0, 0.0],
    [1.0, 0.0],
    [1.0, 1.0],
    [0.0, 0.0],
    [1.0, 1.0],
    [0.0, 1.0],
];

pub struct PathFillPipeline {
    #[allow(dead_code)] // held for arena lifetime (dropped with the context)
    device: ash::Device,
    stencil: Pipeline,
    cover_solid: Pipeline,
    cover_gradient: Pipeline,
    stencil_layout: PipelineLayout,
    cover_layout: PipelineLayout,
    cover_gradient_layout: PipelineLayout,
    #[allow(dead_code)] // held for the cover gradient pipeline's descriptor layout lifetime
    gradient_set_layout: DescriptorSetLayout,
    #[allow(dead_code)] // set 0 of the solid cover (unused by its shader)
    empty_set_layout: DescriptorSetLayout,
    segments_set_layout: DescriptorSetLayout,
    unit_quad_buffer: GpuBuffer,
    format: vk::Format,
    #[allow(dead_code)] // MSAA is selected on the solid pipeline's samples
    samples: vk::SampleCountFlags,
}

impl PathFillPipeline {
    pub fn new(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        let device = ctx.device();

        let gradient_set_layout = DescriptorSetLayout::new(
            device,
            &[vk::DescriptorSetLayoutBinding {
                binding: UBO_BINDING,
                descriptor_type: vk::DescriptorType::UNIFORM_BUFFER,
                descriptor_count: 1,
                stage_flags: vk::ShaderStageFlags::FRAGMENT,
                ..Default::default()
            }],
        )?;
        // The solid cover shader uses only set 1 (segments); set 0 must still
        // occupy the layout's set-0 slot, so give it an empty layout.
        let empty_set_layout = DescriptorSetLayout::new(device, &[])?;
        let segments_set_layout = DescriptorSetLayout::new(
            device,
            &[vk::DescriptorSetLayoutBinding {
                binding: SEGMENTS_BINDING,
                descriptor_type: vk::DescriptorType::STORAGE_BUFFER,
                descriptor_count: 1,
                stage_flags: vk::ShaderStageFlags::FRAGMENT,
                ..Default::default()
            }],
        )?;

        let winding_state = winding_stencil_state();
        let cover_state = cover_stencil_state();

        // Stencil winding pass: contour points (vec2), no color write.
        let (stencil, stencil_layout) = build_graphics_pipeline(
            ctx,
            STENCIL_VERT_SPV,
            STENCIL_FRAG_SPV,
            &[],
            contour_vertex_input(),
            format,
            samples,
            vk::ColorComponentFlags::empty(),
            Some((&winding_state, STENCIL_FORMAT)),
        )?;

        // Solid cover pass: solid brush + winding coverage, gated by the
        // nonzero-winding stencil test.
        let (cover_solid, cover_layout) = build_graphics_pipeline(
            ctx,
            SOLID_VERT_SPV,
            COVER_FRAG_SPV,
            &[empty_set_layout.handle(), segments_set_layout.handle()],
            quad_vertex_input(),
            format,
            samples,
            vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
            Some((&cover_state, STENCIL_FORMAT)),
        )?;

        // Gradient cover pass: gradient shader + UBO + winding coverage, gated
        // by the same stencil test.
        let (cover_gradient, cover_gradient_layout) = build_graphics_pipeline(
            ctx,
            SOLID_VERT_SPV,
            COVER_GRADIENT_FRAG_SPV,
            &[gradient_set_layout.handle(), segments_set_layout.handle()],
            quad_vertex_input(),
            format,
            samples,
            vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
            Some((&cover_state, STENCIL_FORMAT)),
        )?;

        let mut quad_bytes = [0u8; 48];
        for (i, corner) in UNIT_QUAD.iter().enumerate() {
            quad_bytes[i * 8..i * 8 + 4].copy_from_slice(&corner[0].to_le_bytes());
            quad_bytes[i * 8 + 4..i * 8 + 8].copy_from_slice(&corner[1].to_le_bytes());
        }
        let mut unit_quad_buffer = ctx.allocator().create_buffer(
            48,
            vk::BufferUsageFlags::VERTEX_BUFFER,
            MemoryLocation::CpuToGpu,
        )?;
        unit_quad_buffer.write(0, &quad_bytes)?;

        Ok(Self {
            device: device.clone(),
            stencil,
            cover_solid,
            cover_gradient,
            stencil_layout,
            cover_layout,
            cover_gradient_layout,
            gradient_set_layout,
            empty_set_layout,
            segments_set_layout,
            unit_quad_buffer,
            format,
            samples,
        })
    }

    /// The winding-pass pipeline.
    pub fn stencil_handle(&self) -> vk::Pipeline {
        self.stencil.handle()
    }

    /// The solid cover pipeline.
    pub fn cover_solid_handle(&self) -> vk::Pipeline {
        self.cover_solid.handle()
    }

    /// The gradient cover pipeline.
    pub fn cover_gradient_handle(&self) -> vk::Pipeline {
        self.cover_gradient.handle()
    }

    /// The winding pass's pipeline layout (no descriptor sets).
    pub fn stencil_layout(&self) -> vk::PipelineLayout {
        self.stencil_layout.handle()
    }

    /// The solid cover pass's layout (set 0 empty, set 1 segments).
    pub fn cover_layout(&self) -> vk::PipelineLayout {
        self.cover_layout.handle()
    }

    /// The gradient cover pass's layout (set 0 gradient UBO, set 1 segments).
    pub fn cover_gradient_layout(&self) -> vk::PipelineLayout {
        self.cover_gradient_layout.handle()
    }

    #[allow(dead_code)] // held for the cover gradient pipeline's descriptor layout lifetime
    pub fn gradient_set_layout(&self) -> vk::DescriptorSetLayout {
        self.gradient_set_layout.handle()
    }

    /// The flattened-contour storage-buffer set layout (set 1) the cover
    /// shaders read.
    pub fn segments_set_layout(&self) -> vk::DescriptorSetLayout {
        self.segments_set_layout.handle()
    }

    pub fn unit_quad_buffer(&self) -> vk::Buffer {
        self.unit_quad_buffer.buffer()
    }

    pub fn format(&self) -> vk::Format {
        self.format
    }
}

const CONTOUR_BINDINGS: [vk::VertexInputBindingDescription; 1] =
    [vk::VertexInputBindingDescription {
        binding: 0,
        stride: 2 * 4,
        input_rate: vk::VertexInputRate::VERTEX,
    }];
const CONTOUR_ATTRIBUTES: [vk::VertexInputAttributeDescription; 1] =
    [vk::VertexInputAttributeDescription {
        location: 0,
        binding: 0,
        format: vk::Format::R32G32_SFLOAT,
        offset: 0,
    }];

fn contour_vertex_input() -> vk::PipelineVertexInputStateCreateInfo<'static> {
    vk::PipelineVertexInputStateCreateInfo {
        vertex_binding_description_count: CONTOUR_BINDINGS.len() as u32,
        p_vertex_binding_descriptions: CONTOUR_BINDINGS.as_ptr(),
        vertex_attribute_description_count: CONTOUR_ATTRIBUTES.len() as u32,
        p_vertex_attribute_descriptions: CONTOUR_ATTRIBUTES.as_ptr(),
        ..Default::default()
    }
}
