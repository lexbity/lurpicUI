//! The solid-quad pipeline: instanced quads for `FillRect`/`StrokeRect`
//! (Slice 3). Each instance is one rect; the vertex shader applies the
//! push-constant transform and the fragment shader applies the clip + opacity.
//! MSAA (Q8) is selected at pipeline build time.

use ash::vk;

use crate::gpu::allocator::{MemoryLocation, GpuBuffer};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{Pipeline, PipelineLayout, ShaderModule};
use crate::RenderResult;

const VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/solid.vert.spv"));

/// The test-only RG-swapped fragment shader (negative control). Compiled only
/// into test builds (test-exports feature).
#[cfg(feature = "test-exports")]
const SWAPPED_FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/solid_swapped.frag.spv"));

const FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/solid.frag.spv"));

/// The static unit-quad corners as two triangles (TRIANGLE_LIST), shared by
/// every rect instance.
const UNIT_QUAD: [[f32; 2]; 6] = [
    [0.0, 0.0],
    [1.0, 0.0],
    [1.0, 1.0],
    [0.0, 0.0],
    [1.0, 1.0],
    [0.0, 1.0],
];

pub struct SolidPipeline {
    #[allow(dead_code)] // held for arena lifetime (dropped with the context)
    device: ash::Device,
    pipeline: Pipeline,
    layout: PipelineLayout,
    unit_quad_buffer: GpuBuffer,
    format: vk::Format,
    samples: vk::SampleCountFlags,
    /// True when the pipeline was built from the RG-swapped test shader.
    #[cfg(feature = "test-exports")]
    swapped: bool,
}

impl SolidPipeline {
    pub fn new(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        Self::new_with_frag(ctx, format, samples, FRAG_SPV, false)
    }

    /// Builds the pipeline from an explicit fragment SPV. `swapped` marks the
    /// test-only RG-swapped variant so the cache can rebuild on toggle.
    #[cfg(feature = "test-exports")]
    pub fn new_swapped(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        Self::new_with_frag(ctx, format, samples, SWAPPED_FRAG_SPV, true)
    }

    #[cfg_attr(not(feature = "test-exports"), allow(unused_variables))]
    fn new_with_frag(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
        frag_spv: &[u8],
        swapped: bool,
    ) -> Result<Self, (RenderResult, String)> {
        let device = ctx.device();

        let vert = ShaderModule::new(device, &spv_words(VERT_SPV))?;
        let frag = ShaderModule::new(device, &spv_words(frag_spv))?;

        let layout = PipelineLayout::new(device, &[], &[super::PushConstants::layout()])?;

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

        // location 0 (vertex): unit_quad_pos, vec2.
        // location 1 (instance): rect, vec4.
        // location 2 (instance): color, vec4.
        let bindings = [
            vk::VertexInputBindingDescription {
                binding: 0,
                stride: 2 * 4,
                input_rate: vk::VertexInputRate::VERTEX,
            },
            vk::VertexInputBindingDescription {
                binding: 1,
                stride: 8 * 4,
                input_rate: vk::VertexInputRate::INSTANCE,
            },
        ];
        let attributes = [
            vk::VertexInputAttributeDescription {
                location: 0,
                binding: 0,
                format: vk::Format::R32G32_SFLOAT,
                offset: 0,
            },
            vk::VertexInputAttributeDescription {
                location: 1,
                binding: 1,
                format: vk::Format::R32G32B32A32_SFLOAT,
                offset: 0,
            },
            vk::VertexInputAttributeDescription {
                location: 2,
                binding: 1,
                format: vk::Format::R32G32B32A32_SFLOAT,
                offset: 16,
            },
        ];
        let vertex_input = vk::PipelineVertexInputStateCreateInfo {
            vertex_binding_description_count: bindings.len() as u32,
            p_vertex_binding_descriptions: bindings.as_ptr(),
            vertex_attribute_description_count: attributes.len() as u32,
            p_vertex_attribute_descriptions: attributes.as_ptr(),
            ..Default::default()
        };

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
            color_write_mask: vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
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

        let color_formats = [format];
        let mut rendering_info = vk::PipelineRenderingCreateInfo::default()
            .color_attachment_formats(&color_formats);

        let create_infos = [vk::GraphicsPipelineCreateInfo::default()
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
            .subpass(0)];
        let pipeline = Pipeline::new(device, ctx.pipeline_cache(), &create_infos)?;

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
            pipeline,
            layout,
            unit_quad_buffer,
            format,
            samples,
            #[cfg(feature = "test-exports")]
            swapped,
        })
    }

    pub fn handle(&self) -> vk::Pipeline {
        self.pipeline.handle()
    }

    pub fn layout(&self) -> vk::PipelineLayout {
        self.layout.handle()
    }

    pub fn unit_quad_buffer(&self) -> vk::Buffer {
        self.unit_quad_buffer.buffer()
    }

    pub fn samples(&self) -> vk::SampleCountFlags {
        self.samples
    }

    pub fn format(&self) -> vk::Format {
        self.format
    }

    /// True when built from the RG-swapped test shader.
    #[cfg(feature = "test-exports")]
    pub fn swapped(&self) -> bool {
        self.swapped
    }
}

fn spv_words(bytes: &[u8]) -> Vec<u32> {
    bytes
        .chunks_exact(4)
        .map(|c| u32::from_le_bytes([c[0], c[1], c[2], c[3]]))
        .collect()
}
