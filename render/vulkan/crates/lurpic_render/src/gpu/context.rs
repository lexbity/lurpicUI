//! The GPU context: instance + device + queue + allocator + pipeline cache
//! behind the [`GpuContext`] isolation trait (FR-15).
//!
//! RAII drop order is enforced by field declaration order: resources and the
//! allocator drop first, then the device, then the instance. Destroying a
//! parent before children is a validation error, so the arena guarantees the
//! reverse.

use std::ffi::{c_char, CString};

use ash::vk;

use crate::error::vk_error;
use crate::gpu::allocator::{Allocator, GpuAllocator};
use crate::gpu::resources::CommandPool;
use crate::gpu::validation::ValidationContext;
use crate::pipeline_cache::PipelineCache;
use crate::RenderResult;

/// Advertised GPU capabilities for honest backend selection (FR-11).
#[derive(Clone, Copy, Debug, Default)]
#[allow(dead_code)] // api_version/device_type consumed by capability reporting
pub struct PhysicalDeviceFeatures {
    pub api_version: u32,
    pub device_type: vk::PhysicalDeviceType,
    pub dynamic_rendering: bool,
    pub synchronization2: bool,
    pub extended_dynamic_state: bool,
    pub msaa_2x: bool,
    pub msaa_4x: bool,
    pub msaa_8x: bool,
    pub stencil_fill: bool,
}

/// The isolation layer pipeline modules program against. A future swap to a
/// different Vulkan binding re-implements this trait (bus-factor mitigation,
/// Q5).
pub trait GpuContext {
    fn device(&self) -> &ash::Device;
    fn instance(&self) -> &ash::Instance;
    fn physical_device(&self) -> vk::PhysicalDevice;
    fn queue(&self) -> vk::Queue;
    fn queue_family(&self) -> u32;
    fn command_pool(&self) -> vk::CommandPool;
    #[allow(dead_code)] // consumed by pipeline builds (Slice 3)
    fn pipeline_cache(&self) -> vk::PipelineCache;
    fn features(&self) -> &PhysicalDeviceFeatures;
    fn allocator(&self) -> &dyn Allocator;
}

struct InstanceGuard {
    instance: ash::Instance,
}

impl Drop for InstanceGuard {
    fn drop(&mut self) {
        unsafe {
            self.instance.destroy_instance(None);
        }
    }
}

struct DeviceGuard {
    device: ash::Device,
}

impl Drop for DeviceGuard {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_device(None);
        }
    }
}

/// Concrete GPU context owned by the renderer.
pub struct AshContext {
    // Drop order (field order): validation messenger, surface loader, allocator,
    // pipeline cache, command pool, device, instance.
    // `validation`/`pipeline_cache` are held for RAII drop ordering even though
    // the pipeline-cache handle is only read by later pipeline builds.
    #[allow(dead_code)]
    validation: ValidationContext,
    surface_loader: Option<ash::khr::surface::Instance>,
    allocator: GpuAllocator,
    #[allow(dead_code)]
    pipeline_cache: PipelineCache,
    command_pool: CommandPool,
    features: PhysicalDeviceFeatures,
    physical_device: vk::PhysicalDevice,
    queue_family: u32,
    queue: vk::Queue,
    device_guard: DeviceGuard,
    instance_guard: InstanceGuard,
    entry: ash::Entry,
}

impl AshContext {
    /// Initializes instance + device + queue + command pool + pipeline cache
    /// + allocator. `validation_enabled` gates the Khronos validation layer.
    pub fn init(validation_enabled: bool) -> Result<Self, (RenderResult, String)> {
        let entry = crate::gpu::entry::load_entry()?;
        let validation = ValidationContext::new(&entry, validation_enabled)?;

        let instance = create_instance(&entry, &validation)?;

        let mut validation = validation;
        validation.attach(&entry, &instance)?;

        let surface_loader = Some(ash::khr::surface::Instance::new(&entry, &instance));

        let (physical_device, device_props) = pick_physical_device(&instance)?;
        let queue_family = pick_queue_family(&instance, physical_device)?;
        let device = create_device(&instance, physical_device, queue_family)?;
        let queue = unsafe { device.get_device_queue(queue_family, 0) };

        let features = query_features(&instance, physical_device, &device_props);

        let command_pool = CommandPool::new(&device, queue_family, true)?;

        let cache_path = crate::pipeline_cache::cache_path();
        let seed = cache_path.as_deref();
        let pipeline_cache = PipelineCache::new(&device, seed, seed)?;

        let allocator = GpuAllocator::new(&device, &instance, physical_device)?;

        let entry = entry;
        Ok(Self {
            validation,
            surface_loader,
            allocator,
            pipeline_cache,
            command_pool,
            features,
            physical_device,
            queue_family,
            queue,
            device_guard: DeviceGuard { device },
            instance_guard: InstanceGuard { instance },
            entry,
        })
    }

    /// The surface loader (present-support queries, surface destruction).
    pub fn surface_loader(&self) -> Option<&ash::khr::surface::Instance> {
        self.surface_loader.as_ref()
    }

    /// Persists the pipeline cache to disk.
    #[allow(dead_code)] // consumed at teardown by later slices
    pub fn save_pipeline_cache(&self) -> Result<(), (RenderResult, String)> {
        self.pipeline_cache.save()
    }

    pub fn entry(&self) -> &ash::Entry {
        &self.entry
    }
}

impl GpuContext for AshContext {
    fn device(&self) -> &ash::Device {
        &self.device_guard.device
    }

    fn instance(&self) -> &ash::Instance {
        &self.instance_guard.instance
    }

    fn physical_device(&self) -> vk::PhysicalDevice {
        self.physical_device
    }

    fn queue(&self) -> vk::Queue {
        self.queue
    }

    fn queue_family(&self) -> u32 {
        self.queue_family
    }

    fn command_pool(&self) -> vk::CommandPool {
        self.command_pool.handle()
    }

    fn pipeline_cache(&self) -> vk::PipelineCache {
        self.pipeline_cache.handle()
    }

    fn features(&self) -> &PhysicalDeviceFeatures {
        &self.features
    }

    fn allocator(&self) -> &dyn Allocator {
        &self.allocator
    }
}

fn create_instance(
    entry: &ash::Entry,
    validation: &ValidationContext,
) -> Result<ash::Instance, (RenderResult, String)> {
    let app_name = CString::new("lurpic_render").unwrap();
    let engine_name = CString::new("lurpic_render").unwrap();
    let app_info = vk::ApplicationInfo {
        api_version: vk::API_VERSION_1_3,
        p_application_name: app_name.as_ptr(),
        p_engine_name: engine_name.as_ptr(),
        ..Default::default()
    };

    let mut extension_names: Vec<CString> = vec![CString::new("VK_KHR_surface").unwrap()];
    #[cfg(not(target_os = "android"))]
    extension_names.push(CString::new("VK_KHR_xcb_surface").unwrap());
    #[cfg(target_os = "android")]
    extension_names.push(CString::new("VK_KHR_android_surface").unwrap());
    if validation.enabled() {
        extension_names.push(CString::new("VK_EXT_debug_utils").unwrap());
    }

    let layer_ptrs: Vec<*const c_char> = validation.layer_ptrs();
    let extension_ptrs: Vec<*const c_char> = extension_names.iter().map(|s| s.as_ptr()).collect();

    let create_info = vk::InstanceCreateInfo {
        p_application_info: &app_info,
        enabled_layer_count: layer_ptrs.len() as u32,
        pp_enabled_layer_names: layer_ptrs.as_ptr(),
        enabled_extension_count: extension_ptrs.len() as u32,
        pp_enabled_extension_names: extension_ptrs.as_ptr(),
        ..Default::default()
    };
    // Chain the debug messenger create info so validation emits during instance
    // creation itself. The clone lives until `create_instance` returns.
    let mut messenger_info = validation.messenger_create_info().cloned();
    let create_info = match messenger_info.as_mut() {
        Some(info) => create_info.push_next(info),
        None => create_info,
    };

    unsafe { entry.create_instance(&create_info, None) }
        .map_err(|e| vk_error("vkCreateInstance", e.as_raw()))
}

fn pick_physical_device(
    instance: &ash::Instance,
) -> Result<(vk::PhysicalDevice, vk::PhysicalDeviceProperties), (RenderResult, String)> {
    let devices = unsafe { instance.enumerate_physical_devices() }
        .map_err(|e| vk_error("vkEnumeratePhysicalDevices", e.as_raw()))?;
    if devices.is_empty() {
        return Err((
            RenderResult::Unsupported,
            "no Vulkan physical devices found".to_string(),
        ));
    }

    let mut best: Option<(i32, vk::PhysicalDevice, vk::PhysicalDeviceProperties)> = None;
    for device in devices {
        let props = unsafe { instance.get_physical_device_properties(device) };
        if props.api_version < vk::API_VERSION_1_3 {
            // Q7: Vulkan 1.3 core, no extension fallback.
            continue;
        }
        let score = match props.device_type {
            vk::PhysicalDeviceType::DISCRETE_GPU => 400,
            vk::PhysicalDeviceType::INTEGRATED_GPU => 300,
            vk::PhysicalDeviceType::VIRTUAL_GPU => 200,
            vk::PhysicalDeviceType::CPU => 100,
            _ => 50,
        };
        if pick_queue_family(instance, device).is_err() {
            continue;
        }
        let score = score + 10;
        if best
            .as_ref()
            .map_or(true, |(best_score, _, _)| score > *best_score)
        {
            best = Some((score, device, props));
        }
    }

    best.map(|(_, device, props)| (device, props)).ok_or_else(|| {
        (
            RenderResult::Unsupported,
            "no suitable Vulkan 1.3 physical device found".to_string(),
        )
    })
}

fn pick_queue_family(
    instance: &ash::Instance,
    physical_device: vk::PhysicalDevice,
) -> Result<u32, (RenderResult, String)> {
    let families = unsafe { instance.get_physical_device_queue_family_properties(physical_device) };
    for (index, family) in families.iter().enumerate() {
        if family.queue_count > 0 && family.queue_flags.contains(vk::QueueFlags::GRAPHICS) {
            return Ok(index as u32);
        }
    }
    Err((
        RenderResult::InitFailed,
        "no suitable Vulkan graphics queue family found".to_string(),
    ))
}

fn create_device(
    instance: &ash::Instance,
    physical_device: vk::PhysicalDevice,
    queue_family: u32,
) -> Result<ash::Device, (RenderResult, String)> {
    let queue_priority = 1.0f32;
    let queue_info = vk::DeviceQueueCreateInfo {
        queue_family_index: queue_family,
        queue_count: 1,
        p_queue_priorities: &queue_priority,
        ..Default::default()
    };

    // Enable the 1.2/1.3 features the renderer relies on (all promoted to core
    // in 1.3). dynamic_rendering, synchronization2 and extended_dynamic_state
    // are all 1.3; shader_sampled_image_array_non_uniform_indexing is 1.2.
    let mut features12 = vk::PhysicalDeviceVulkan12Features::default();
    let mut features13 = vk::PhysicalDeviceVulkan13Features::default();
    let mut features_ext_dynamic_state = vk::PhysicalDeviceExtendedDynamicStateFeaturesEXT::default();
    features13.dynamic_rendering = vk::TRUE;
    features13.synchronization2 = vk::TRUE;
    // VK_EXT_extended_dynamic_state is core in 1.3; the feature struct keeps its
    // extension name (per the promotion rules).
    features_ext_dynamic_state.extended_dynamic_state = vk::TRUE;

    let queue_infos = [queue_info];
    let create_info = vk::DeviceCreateInfo::default()
        .queue_create_infos(&queue_infos)
        .push_next(&mut features12)
        .push_next(&mut features13)
        .push_next(&mut features_ext_dynamic_state);

    unsafe { instance.create_device(physical_device, &create_info, None) }
        .map_err(|e| vk_error("vkCreateDevice", e.as_raw()))
}

fn query_features(
    instance: &ash::Instance,
    physical_device: vk::PhysicalDevice,
    props: &vk::PhysicalDeviceProperties,
) -> PhysicalDeviceFeatures {
    let mut features12 = vk::PhysicalDeviceVulkan12Features::default();
    let mut features13 = vk::PhysicalDeviceVulkan13Features::default();
    let mut features_ext_dynamic_state = vk::PhysicalDeviceExtendedDynamicStateFeaturesEXT::default();
    let mut features2 = vk::PhysicalDeviceFeatures2::default()
        .push_next(&mut features12)
        .push_next(&mut features13)
        .push_next(&mut features_ext_dynamic_state);
    unsafe {
        instance.get_physical_device_features2(physical_device, &mut features2);
    }

    let sample_counts = unsafe { instance.get_physical_device_properties(physical_device) }
        .limits
        .framebuffer_color_sample_counts;

    PhysicalDeviceFeatures {
        api_version: props.api_version,
        device_type: props.device_type,
        dynamic_rendering: features13.dynamic_rendering == vk::TRUE,
        synchronization2: features13.synchronization2 == vk::TRUE,
        extended_dynamic_state: features_ext_dynamic_state.extended_dynamic_state == vk::TRUE,
        msaa_2x: sample_counts.contains(vk::SampleCountFlags::TYPE_2),
        msaa_4x: sample_counts.contains(vk::SampleCountFlags::TYPE_4),
        msaa_8x: sample_counts.contains(vk::SampleCountFlags::TYPE_8),
        stencil_fill: true, // stencil is core Vulkan (usage flag, not a feature)
    }
}
