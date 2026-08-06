//! The textured-quad pipeline (Slice 4): sampler-bound instanced quads for
//! `DrawImage`/`DrawTexture`. Each instance is one quad carrying the dest rect
//! (local xywh) and the src rect (texture-space xywh); the vertex shader
//! applies the push-constant transform and derives the UV, the fragment shader
//! samples a combined image sampler bound per draw group via a descriptor set.

use ash::vk;

use crate::gpu::allocator::{GpuBuffer, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{DescriptorSetLayout, Pipeline, PipelineLayout, ShaderModule};
use crate::RenderResult;

const VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/textured.vert.spv"));
const FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/textured.frag.spv"));

/// The static unit-quad corners as two triangles (TRIANGLE_LIST), shared by
/// every textured instance (identical geometry to the solid pipeline's quad).
const UNIT_QUAD: [[f32; 2]; 6] = [
    [0.0, 0.0],
    [1.0, 0.0],
    [1.0, 1.0],
    [0.0, 0.0],
    [1.0, 1.0],
    [0.0, 1.0],
];

/// Descriptor set layout binding for the textured pipeline: set 0 binding 0 is
/// the combined image sampler the fragment shader samples.
pub const SAMPLER_BINDING: u32 = 0;

pub struct TexturedPipeline {
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

impl TexturedPipeline {
    pub fn new(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        let device = ctx.device();

        let vert = ShaderModule::new(device, &spv_words(VERT_SPV))?;
        let frag = ShaderModule::new(device, &spv_words(FRAG_SPV))?;

        let set_layout = DescriptorSetLayout::new(
            device,
            &[vk::DescriptorSetLayoutBinding {
                binding: SAMPLER_BINDING,
                descriptor_type: vk::DescriptorType::COMBINED_IMAGE_SAMPLER,
                descriptor_count: 1,
                stage_flags: vk::ShaderStageFlags::FRAGMENT,
                ..Default::default()
            }],
        )?;
        let layout = PipelineLayout::new(
            device,
            &[set_layout.handle()],
            &[super::PushConstants::layout()],
        )?;

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
        // location 1 (instance): dest_rect, vec4 (local xywh).
        // location 2 (instance): src_rect, vec4 (texture xywh).
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

        // Premultiplied-alpha "over" (identical to the solid pipeline): the
        // fragment shader emits premultiplied color, so src factor is ONE.
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
        let mut rendering_info =
            vk::PipelineRenderingCreateInfo::default().color_attachment_formats(&color_formats);

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

    /// The descriptor set layout the frame encoder allocates per-draw-group
    /// sets from.
    pub fn set_layout(&self) -> vk::DescriptorSetLayout {
        self.set_layout.handle()
    }

    pub fn unit_quad_buffer(&self) -> vk::Buffer {
        self.unit_quad_buffer.buffer()
    }

    #[allow(dead_code)] // MSAA is selected on the solid pipeline's samples
    pub fn samples(&self) -> vk::SampleCountFlags {
        self.samples
    }

    pub fn format(&self) -> vk::Format {
        self.format
    }
}

fn spv_words(bytes: &[u8]) -> Vec<u32> {
    bytes
        .chunks_exact(4)
        .map(|c| u32::from_le_bytes([c[0], c[1], c[2], c[3]]))
        .collect()
}
