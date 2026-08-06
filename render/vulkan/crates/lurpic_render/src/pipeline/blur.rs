//! The shadow blur pipelines (Slice 9): a path-mask pass, a separable
//! horizontal/vertical Gaussian blur pair over the R8 scratch images, and the
//! tinted composite pass that draws the blurred mask into the main target.
//!
//! The mask pass reuses the Slice 7 coverage shader (`shadow_mask.frag` is the
//! cover shader with an optional `1 - coverage` inversion for inner shadows);
//! it writes the path's analytic coverage into the R8 scratch without a stencil
//! attachment (the cover shader is the coverage authority, Q8 amendment). The
//! blur pair and the composite sample the scratch with a NEAREST/CLAMP_TO_EDGE
//! sampler so texel reads land exactly on the software oracle's array indices.

use ash::vk;

use crate::gpu::allocator::{GpuBuffer, MemoryLocation};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{DescriptorSetLayout, Pipeline, PipelineLayout};
use crate::pipeline::{build_graphics_pipeline, quad_vertex_input};
use crate::RenderResult;

const MASK_VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/solid.vert.spv"));
const MASK_FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/shadow_mask.frag.spv"));
// The blur and composite passes reuse the textured vertex shader (instanced
// dest+src quads; the blur quad spans the blur region, the composite quad spans
// the offset region sampling the same region).
const BLUR_VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/textured.vert.spv"));
const BLUR_H_FRAG_SPV: &[u8] =
    include_bytes!(concat!(env!("OUT_DIR"), "/blur_horizontal.frag.spv"));
const BLUR_V_FRAG_SPV: &[u8] =
    include_bytes!(concat!(env!("OUT_DIR"), "/blur_vertical.frag.spv"));
const COMPOSITE_FRAG_SPV: &[u8] =
    include_bytes!(concat!(env!("OUT_DIR"), "/shadow_composite.frag.spv"));

/// Descriptor set layout binding for the blur/composite sampler sets.
pub const SAMPLER_BINDING: u32 = 0;
/// Set 1 binding for the mask pass's contour-edges storage buffer.
pub const SEGMENTS_BINDING: u32 = 0;

/// The R8 format of the blur scratch images (Slice 9). The software oracle's
/// coverage mask is 8-bit too (vector.Rasterizer's Alpha output), so the
/// quantization matches; the separable blur spreads any residual difference.
pub const BLUR_SCRATCH_FORMAT: vk::Format = vk::Format::R8_UNORM;

const UNIT_QUAD: [[f32; 2]; 6] = [
    [0.0, 0.0],
    [1.0, 0.0],
    [1.0, 1.0],
    [0.0, 0.0],
    [1.0, 1.0],
    [0.0, 1.0],
];

/// The four Slice 9 pipelines and their layouts. `format` is the main surface
/// format for the composite; the mask/blur pair always target R8 scratch.
pub struct BlurPipeline {
    #[allow(dead_code)] // held for arena lifetime (dropped with the context)
    device: ash::Device,
    mask: Pipeline,
    blur_h: Pipeline,
    blur_v: Pipeline,
    composite: Pipeline,
    mask_layout: PipelineLayout,
    #[allow(dead_code)] // set 0 of the mask (unused by its shader)
    mask_empty_layout: DescriptorSetLayout,
    #[allow(dead_code)] // held for the mask pipeline's descriptor layout lifetime
    mask_segments_layout: DescriptorSetLayout,
    sampler_layout: PipelineLayout,
    sampler_set_layout: DescriptorSetLayout,
    unit_quad_buffer: GpuBuffer,
    format: vk::Format,
    #[allow(dead_code)] // selected by the solid pipeline's samples
    samples: vk::SampleCountFlags,
}

impl BlurPipeline {
    pub fn new(
        ctx: &dyn GpuContext,
        format: vk::Format,
        samples: vk::SampleCountFlags,
    ) -> Result<Self, (RenderResult, String)> {
        let device = ctx.device();

        // The mask pass binds the contour-edges storage buffer at set 1; set 0
        // is an empty layout (the mask shader accesses only set 1, mirroring the
        // solid path-fill cover).
        let mask_empty_layout = DescriptorSetLayout::new(device, &[])?;
        let mask_segments_layout = DescriptorSetLayout::new(
            device,
            &[vk::DescriptorSetLayoutBinding {
                binding: SEGMENTS_BINDING,
                descriptor_type: vk::DescriptorType::STORAGE_BUFFER,
                descriptor_count: 1,
                stage_flags: vk::ShaderStageFlags::FRAGMENT,
                ..Default::default()
            }],
        )?;
        let (mask, mask_layout) = build_graphics_pipeline(
            ctx,
            MASK_VERT_SPV,
            MASK_FRAG_SPV,
            &[mask_empty_layout.handle(), mask_segments_layout.handle()],
            quad_vertex_input(),
            BLUR_SCRATCH_FORMAT,
            samples,
            vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
            None,
        )?;

        // Blur + composite sample the scratch via a combined image sampler.
        let sampler_set_layout = DescriptorSetLayout::new(
            device,
            &[vk::DescriptorSetLayoutBinding {
                binding: SAMPLER_BINDING,
                descriptor_type: vk::DescriptorType::COMBINED_IMAGE_SAMPLER,
                descriptor_count: 1,
                stage_flags: vk::ShaderStageFlags::FRAGMENT,
                ..Default::default()
            }],
        )?;
        let (blur_h, sampler_layout) = build_graphics_pipeline(
            ctx,
            BLUR_VERT_SPV,
            BLUR_H_FRAG_SPV,
            &[sampler_set_layout.handle()],
            quad_vertex_input(),
            BLUR_SCRATCH_FORMAT,
            samples,
            vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
            None,
        )?;
        let (blur_v, _) = build_graphics_pipeline(
            ctx,
            BLUR_VERT_SPV,
            BLUR_V_FRAG_SPV,
            &[sampler_set_layout.handle()],
            quad_vertex_input(),
            BLUR_SCRATCH_FORMAT,
            samples,
            vk::ColorComponentFlags::R
                | vk::ColorComponentFlags::G
                | vk::ColorComponentFlags::B
                | vk::ColorComponentFlags::A,
            None,
        )?;
        let (composite, _) = build_graphics_pipeline(
            ctx,
            BLUR_VERT_SPV,
            COMPOSITE_FRAG_SPV,
            &[sampler_set_layout.handle()],
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
            mask,
            blur_h,
            blur_v,
            composite,
            mask_layout,
            mask_empty_layout,
            mask_segments_layout,
            sampler_layout,
            sampler_set_layout,
            unit_quad_buffer,
            format,
            samples,
        })
    }

    /// The mask pass pipeline (path coverage into the R8 scratch).
    pub fn mask_handle(&self) -> vk::Pipeline {
        self.mask.handle()
    }

    /// The mask pass layout (set 0 empty, set 1 segments).
    pub fn mask_layout(&self) -> vk::PipelineLayout {
        self.mask_layout.handle()
    }

    /// The horizontal Gaussian blur pipeline (R8 scratch -> R8 scratch).
    pub fn blur_h_handle(&self) -> vk::Pipeline {
        self.blur_h.handle()
    }

    /// The vertical Gaussian blur pipeline (R8 scratch -> R8 scratch).
    pub fn blur_v_handle(&self) -> vk::Pipeline {
        self.blur_v.handle()
    }

    /// The shadow composite pipeline (R8 scratch -> main target).
    pub fn composite_handle(&self) -> vk::Pipeline {
        self.composite.handle()
    }

    /// The shared blur/composite layout (set 0 sampler).
    pub fn sampler_layout(&self) -> vk::PipelineLayout {
        self.sampler_layout.handle()
    }

    /// The combined-image-sampler set layout the blur/composite passes bind.
    pub fn sampler_set_layout(&self) -> vk::DescriptorSetLayout {
        self.sampler_set_layout.handle()
    }

    /// The static unit-quad buffer shared by all four pipelines.
    pub fn unit_quad_buffer(&self) -> vk::Buffer {
        self.unit_quad_buffer.buffer()
    }

    /// The main surface format the composite pipeline targets.
    pub fn format(&self) -> vk::Format {
        self.format
    }
}
