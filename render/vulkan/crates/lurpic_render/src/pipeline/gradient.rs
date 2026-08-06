//! The gradient pipeline (Slice 6): instanced quads for rect fills/strokes
//! with a `BrushLinearGradient`. The vertex shader is shared with the solid
//! pipeline (instanced rect + push-constant transform); the fragment shader
//! computes the gradient color per pixel from the push-constant start/end and a
//! per-group gradient UBO (set 0 binding 0). The premultiplied brush color in
//! the instance record is unused.

use ash::vk;

use crate::gpu::allocator::{GpuBuffer, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{DescriptorSetLayout, Pipeline, PipelineLayout};
use crate::pipeline::{build_graphics_pipeline, quad_vertex_input};
use crate::RenderResult;

// The gradient pipeline reuses the solid vertex shader (identical instance
// layout + push-constant transform).
const VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/solid.vert.spv"));
const FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/gradient.frag.spv"));

/// Descriptor set layout binding for the gradient pipeline: set 0 binding 0 is
/// the gradient-stops uniform buffer.
pub const UBO_BINDING: u32 = 0;

pub struct GradientPipeline {
    #[allow(dead_code)] // held for arena lifetime (dropped with the context)
    device: ash::Device,
    pipeline: Pipeline,
    layout: PipelineLayout,
    set_layout: DescriptorSetLayout,
    unit_quad_buffer: GpuBuffer,
    format: vk::Format,
    #[allow(dead_code)] // MSAA is selected on the solid pipeline's samples
    samples: vk::SampleCountFlags,
}

impl GradientPipeline {
    pub fn new(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        let device = ctx.device();

        let set_layout = DescriptorSetLayout::new(device, &[vk::DescriptorSetLayoutBinding {
            binding: UBO_BINDING,
            descriptor_type: vk::DescriptorType::UNIFORM_BUFFER,
            descriptor_count: 1,
            stage_flags: vk::ShaderStageFlags::FRAGMENT,
            ..Default::default()
        }])?;
        let (pipeline, layout) = build_graphics_pipeline(
            ctx,
            VERT_SPV,
            FRAG_SPV,
            &[set_layout.handle()],
            quad_vertex_input(),
            format,
            samples,
            vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
            None,
        )?;


        let mut quad_bytes = [0u8; 48];
        let unit_quad: [[f32; 2]; 6] = [
            [0.0, 0.0],
            [1.0, 0.0],
            [1.0, 1.0],
            [0.0, 0.0],
            [1.0, 1.0],
            [0.0, 1.0],
        ];
        for (i, corner) in unit_quad.iter().enumerate() {
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
            pipeline,
            layout,
            set_layout,
            unit_quad_buffer,
            format,
            samples,
        })
    }

    pub fn handle(&self) -> vk::Pipeline {
        self.pipeline.handle()
    }

    pub fn layout(&self) -> vk::PipelineLayout {
        self.layout.handle()
    }

    /// The descriptor set layout the frame encoder allocates gradient UBO sets
    /// from.
    pub fn set_layout(&self) -> vk::DescriptorSetLayout {
        self.set_layout.handle()
    }

    pub fn unit_quad_buffer(&self) -> vk::Buffer {
        self.unit_quad_buffer.buffer()
    }

    pub fn format(&self) -> vk::Format {
        self.format
    }
}

