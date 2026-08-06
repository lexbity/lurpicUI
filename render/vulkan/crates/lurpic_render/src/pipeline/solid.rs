//! The solid-quad pipeline: instanced quads for `FillRect`/`StrokeRect`
//! (Slice 3). Each instance is one rect; the vertex shader applies the
//! push-constant transform and the fragment shader applies the clip + opacity.
//! MSAA (Q8) is selected at pipeline build time.

use ash::vk;

use crate::gpu::allocator::{MemoryLocation, GpuBuffer};
use crate::gpu::context::GpuContext;
use crate::gpu::resources::{Pipeline, PipelineLayout};
use crate::pipeline::{build_graphics_pipeline, quad_vertex_input};
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

        let (pipeline, layout) = build_graphics_pipeline(
            ctx,
            VERT_SPV,
            frag_spv,
            &[],
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
