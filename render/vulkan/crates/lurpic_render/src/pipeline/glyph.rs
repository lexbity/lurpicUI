//! The glyph pipeline (Slice 5): instanced quads sampling the packed glyph
//! atlas. Two fragment-shader variants select the rendering mode — bitmap
//! (coverage mask in the atlas R channel) for sizes < 24 px and SDF
//! (signed-distance in the atlas G channel) for larger sizes. Both share the
//! same vertex shader, pipeline layout, and combined-image-sampler set layout.

use ash::vk;

use crate::gpu::allocator::{GpuBuffer, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{DescriptorSetLayout, Pipeline, PipelineLayout};
use crate::RenderResult;

const VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/glyph.vert.spv"));
const BITMAP_FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/glyph_bitmap.frag.spv"));
const SDF_FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/glyph_sdf.frag.spv"));

const UNIT_QUAD: [[f32; 2]; 6] = [
    [0.0, 0.0],
    [1.0, 0.0],
    [1.0, 1.0],
    [0.0, 0.0],
    [1.0, 1.0],
    [0.0, 1.0],
];

/// Descriptor set layout binding for the glyph pipeline: set 0 binding 0 is
/// the combined image sampler bound to the packed glyph atlas.
pub const ATLAS_BINDING: u32 = 0;

pub struct GlyphPipeline {
    #[allow(dead_code)] // held for arena lifetime (dropped with the context)
    device: ash::Device,
    bitmap: Pipeline,
    sdf: Pipeline,
    layout: PipelineLayout,
    #[allow(dead_code)] // identical to the textured pipeline's set layout; the encoder reuses that one
    set_layout: DescriptorSetLayout,
    unit_quad_buffer: GpuBuffer,
    format: vk::Format,
    #[allow(dead_code)] // MSAA is selected on the solid pipeline's samples
    samples: vk::SampleCountFlags,
}

impl GlyphPipeline {
    pub fn new(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        let device = ctx.device();

        let set_layout = DescriptorSetLayout::new(device, &[vk::DescriptorSetLayoutBinding {
            binding: ATLAS_BINDING,
            descriptor_type: vk::DescriptorType::COMBINED_IMAGE_SAMPLER,
            descriptor_count: 1,
            stage_flags: vk::ShaderStageFlags::FRAGMENT,
            ..Default::default()
        }])?;

        let build = |frag_spv: &[u8]| {
            crate::pipeline::build_graphics_pipeline(
                ctx,
                VERT_SPV,
                frag_spv,
                &[set_layout.handle()],
                crate::pipeline::quad_vertex_input(),
                format,
                samples,
                vk::ColorComponentFlags::R
                    | vk::ColorComponentFlags::G
                    | vk::ColorComponentFlags::B
                    | vk::ColorComponentFlags::A,
                None,
            )
        };

        let (bitmap, layout) = build(BITMAP_FRAG_SPV)?;
        let (sdf, _) = build(SDF_FRAG_SPV)?;

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
            bitmap,
            sdf,
            layout,
            set_layout,
            unit_quad_buffer,
            format,
            samples,
        })
    }

    /// The bitmap-mode (coverage-mask) pipeline.
    pub fn bitmap_handle(&self) -> vk::Pipeline {
        self.bitmap.handle()
    }

    /// The SDF-mode pipeline.
    pub fn sdf_handle(&self) -> vk::Pipeline {
        self.sdf.handle()
    }

    pub fn layout(&self) -> vk::PipelineLayout {
        self.layout.handle()
    }

    #[allow(dead_code)] // identical to the textured pipeline's set layout
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

