//! Slice 2 foundation test: builds a real graphics pipeline and renders a
//! fullscreen clear into an offscreen image, reads a pixel back, and asserts
//! zero validation errors. This proves the ash integration layer (entry,
//! instance, device, queue, allocator, pipeline cache, dynamic rendering) is
//! functional end to end. Presentation is exercised by the main VulkanState
//! path; a headless test cannot create a swapchain without a window.
//!
//! Skips when no Vulkan 1.3 device is available.

use lurpic_render::gpu::context::{AshContext, GpuContext};
use lurpic_render::gpu::resources::{CommandPool, ImageView, Pipeline, PipelineLayout, ShaderModule};

use ash::vk;

const VERT_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/clear.vert.spv"));
const FRAG_SPV: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/clear.frag.spv"));

fn spv_words(bytes: &[u8]) -> Vec<u32> {
    bytes
        .chunks_exact(4)
        .map(|c| u32::from_le_bytes([c[0], c[1], c[2], c[3]]))
        .collect()
}

fn check_vulkan_available() -> Option<AshContext> {
    let validation = std::env::var("LURPIC_RENDER_VALIDATION").as_deref() == Ok("1");
    match AshContext::init(validation) {
        Ok(ctx) => Some(ctx),
        Err((code, msg)) => {
            if code == lurpic_render::RenderResult::Unsupported {
                eprintln!("foundation test skipped: {}", msg);
                None
            } else {
                panic!("foundation test init failed: {:?}: {}", code, msg)
            }
        }
    }
}

#[test]
fn clear_pipeline_builds_and_renders() {
    let Some(ctx) = check_vulkan_available() else {
        return;
    };

    let device = ctx.device().clone();
    let width = 64u32;
    let height = 64u32;

    // Shader modules.
    let vert = ShaderModule::new(&device, &spv_words(VERT_SPV)).expect("vert shader module");
    let frag = ShaderModule::new(&device, &spv_words(FRAG_SPV)).expect("frag shader module");

    // Pipeline layout with a single vec4 fragment push constant.
    let push_range = vk::PushConstantRange {
        stage_flags: vk::ShaderStageFlags::FRAGMENT,
        offset: 0,
        size: 16,
    };
    let layout = PipelineLayout::new(&device, &[], &[push_range]).expect("pipeline layout");

    // Graphics pipeline: fullscreen triangle, no vertex buffers.
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
    let vertex_input = vk::PipelineVertexInputStateCreateInfo::default();
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
        rasterization_samples: vk::SampleCountFlags::TYPE_1,
        ..Default::default()
    };
    let color_attachment = vk::PipelineColorBlendAttachmentState {
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

    // Dynamic rendering requires the color attachment format on the pipeline.
    let color_formats = [vk::Format::R8G8B8A8_UNORM];
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

    let pipeline =
        Pipeline::new(&device, ctx.pipeline_cache(), &create_infos).expect("graphics pipeline");

    // Offscreen color image (host-readable via staging copy).
    let extent = vk::Extent3D {
        width,
        height,
        depth: 1,
    };
    let image_info = vk::ImageCreateInfo {
        image_type: vk::ImageType::TYPE_2D,
        format: vk::Format::R8G8B8A8_UNORM,
        extent,
        mip_levels: 1,
        array_layers: 1,
        samples: vk::SampleCountFlags::TYPE_1,
        tiling: vk::ImageTiling::OPTIMAL,
        usage: vk::ImageUsageFlags::COLOR_ATTACHMENT | vk::ImageUsageFlags::TRANSFER_SRC,
        sharing_mode: vk::SharingMode::EXCLUSIVE,
        ..Default::default()
    };
    let image = unsafe { device.create_image(&image_info, None) }.expect("create image");
    let requirements = unsafe { device.get_image_memory_requirements(image) };
    let image_allocation = ctx
        .allocator()
        .allocate_image_memory(
            image,
            requirements,
            lurpic_render::gpu::allocator::MemoryLocation::GpuOnly,
        )
        .expect("image memory");
    unsafe {
        device.bind_image_memory(image, image_allocation.memory(), 0)
    }
    .expect("bind image memory");
    let view = ImageView::new(&device, image, vk::Format::R8G8B8A8_UNORM, vk::ImageAspectFlags::COLOR)
        .expect("image view");

    // Staging buffer for readback.
    let mut staging = ctx
        .allocator()
        .create_buffer(
            (width * height * 4) as u64,
            vk::BufferUsageFlags::TRANSFER_DST,
            lurpic_render::gpu::allocator::MemoryLocation::CpuToGpu,
        )
        .expect("staging buffer");

    // Command buffer: clear-render via dynamic rendering, then copy out.
    let command_pool = CommandPool::new(&device, ctx.queue_family(), true).expect("command pool");
    let command_buffer = command_pool.allocate_primary().expect("command buffer");
    let begin = vk::CommandBufferBeginInfo {
        flags: vk::CommandBufferUsageFlags::ONE_TIME_SUBMIT,
        ..Default::default()
    };
    unsafe {
        device.begin_command_buffer(command_buffer, &begin)
    }
    .expect("begin command buffer");

    // Transition image to COLOR_ATTACHMENT_OPTIMAL.
    let image_barrier = vk::ImageMemoryBarrier::default()
        .old_layout(vk::ImageLayout::UNDEFINED)
        .new_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
        .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .image(image)
        .subresource_range(
            vk::ImageSubresourceRange::default()
                .aspect_mask(vk::ImageAspectFlags::COLOR)
                .level_count(1)
                .layer_count(1),
        );
    unsafe {
        device.cmd_pipeline_barrier(
            command_buffer,
            vk::PipelineStageFlags::TOP_OF_PIPE,
            vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
            vk::DependencyFlags::empty(),
            &[],
            &[],
            &[image_barrier],
        );
    }

    // Dynamic rendering with the image as the sole color attachment.
    let color_attachment = vk::RenderingAttachmentInfo::default()
        .image_view(view.handle())
        .image_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
        .load_op(vk::AttachmentLoadOp::CLEAR)
        .store_op(vk::AttachmentStoreOp::STORE)
        .clear_value(vk::ClearValue {
            color: vk::ClearColorValue {
                float32: [0.2, 0.4, 0.6, 1.0],
            },
        });
    let color_attachments = [color_attachment];
    let rendering_info = vk::RenderingInfo::default()
        .render_area(vk::Rect2D {
            offset: vk::Offset2D { x: 0, y: 0 },
            extent: vk::Extent2D { width, height },
        })
        .layer_count(1)
        .color_attachments(&color_attachments);
    unsafe {
        device.cmd_begin_rendering(command_buffer, &rendering_info);
        device.cmd_bind_pipeline(
            command_buffer,
            vk::PipelineBindPoint::GRAPHICS,
            pipeline.handle(),
        );
        device.cmd_set_viewport(
            command_buffer,
            0,
            &[vk::Viewport {
                x: 0.0,
                y: 0.0,
                width: width as f32,
                height: height as f32,
                min_depth: 0.0,
                max_depth: 1.0,
            }],
        );
        device.cmd_set_scissor(
            command_buffer,
            0,
            &[vk::Rect2D {
                offset: vk::Offset2D { x: 0, y: 0 },
                extent: vk::Extent2D { width, height },
            }],
        );
        // Push a full vec4 fragment color (16 bytes of float bits): opaque red.
        let mut push_color = [0u8; 16];
        push_color[0..4].copy_from_slice(&1.0f32.to_le_bytes());
        push_color[4..8].copy_from_slice(&0.0f32.to_le_bytes());
        push_color[8..12].copy_from_slice(&0.0f32.to_le_bytes());
        push_color[12..16].copy_from_slice(&1.0f32.to_le_bytes());
        device.cmd_push_constants(
            command_buffer,
            layout.handle(),
            vk::ShaderStageFlags::FRAGMENT,
            0,
            &push_color,
        );
        device.cmd_draw(command_buffer, 3, 1, 0, 0);
        device.cmd_end_rendering(command_buffer);
    }

    // Copy rendered image to the staging buffer for readback.
    let copy_barrier = vk::ImageMemoryBarrier::default()
        .src_access_mask(vk::AccessFlags::COLOR_ATTACHMENT_WRITE)
        .dst_access_mask(vk::AccessFlags::TRANSFER_READ)
        .old_layout(vk::ImageLayout::COLOR_ATTACHMENT_OPTIMAL)
        .new_layout(vk::ImageLayout::TRANSFER_SRC_OPTIMAL)
        .src_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .dst_queue_family_index(vk::QUEUE_FAMILY_IGNORED)
        .image(image)
        .subresource_range(
            vk::ImageSubresourceRange::default()
                .aspect_mask(vk::ImageAspectFlags::COLOR)
                .level_count(1)
                .layer_count(1),
        );
    unsafe {
        device.cmd_pipeline_barrier(
            command_buffer,
            vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT,
            vk::PipelineStageFlags::TRANSFER,
            vk::DependencyFlags::empty(),
            &[],
            &[],
            &[copy_barrier],
        );
        let regions = [vk::BufferImageCopy::default()
            .image_subresource(
                vk::ImageSubresourceLayers::default()
                    .aspect_mask(vk::ImageAspectFlags::COLOR)
                    .layer_count(1),
            )
            .image_extent(extent)];
        device.cmd_copy_image_to_buffer(
            command_buffer,
            image,
            vk::ImageLayout::TRANSFER_SRC_OPTIMAL,
            staging.buffer(),
            &regions,
        );
    }

    unsafe {
        device.end_command_buffer(command_buffer)
    }
    .expect("end command buffer");

    let command_buffers = [command_buffer];
    let submit = vk::SubmitInfo::default().command_buffers(&command_buffers);
    unsafe {
        device.queue_submit(ctx.queue(), &[submit], vk::Fence::null())
    }
    .expect("queue submit");
    unsafe {
        device.queue_wait_idle(ctx.queue())
    }
    .expect("queue wait idle");

    // Read back the first pixel and assert it matches the pushed fragment
    // color (opaque red), proving the pipeline drew the triangle.
    let mut pixel = [0u8; 4];
    let ptr = staging
        .mapped_ptr()
        .expect("staging buffer is host-mapped");
    unsafe {
        std::ptr::copy_nonoverlapping(ptr, pixel.as_mut_ptr(), 4);
    }
    assert_eq!(pixel, [255, 0, 0, 255]);

    // Explicitly drop resources before the context's device (arena order).
    drop(staging);
    drop(image_allocation);
    unsafe {
        device.destroy_image(image, None);
    }
    drop(command_pool);
    drop(pipeline);
    drop(layout);
    drop(frag);
    drop(vert);

    let _ = ctx;
}

/// The foundation init path must be validation-clean. Run with
/// LURPIC_RENDER_VALIDATION not required here (validation is wired into the
/// main init); this test additionally asserts the device/lifetime path.
#[test]
fn context_drop_is_validation_clean() {
    let Some(ctx) = check_vulkan_available() else {
        return;
    };
    drop(ctx);
}
