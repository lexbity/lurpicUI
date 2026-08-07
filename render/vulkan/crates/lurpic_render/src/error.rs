//! Shared error mapping for Vulkan result codes.

use ash::vk;
use crate::RenderResult;

/// Maps a raw Vulkan `VkResult` to the crate's typed render error.
///
/// `VK_ERROR_DEVICE_LOST` maps to the distinct [`RenderResult::DeviceLost`] (not
/// a generic VulkanError) so the Go side can translate it into the typed
/// `render.ErrGPUFatal` (Slice 10, FR-10) and the runtime can fall back to the
/// software backend. Device loss is also a device-generation event: the
/// generation counter bumps so `DeviceGenerationProvider` consumers invalidate
/// GPU-cached texture IDs.
pub fn vk_error(op: &str, code: i32) -> (RenderResult, String) {
    let out_of_memory = vk::Result::ERROR_OUT_OF_HOST_MEMORY.as_raw() == code
        || vk::Result::ERROR_OUT_OF_DEVICE_MEMORY.as_raw() == code;
    let init_failed = vk::Result::ERROR_INITIALIZATION_FAILED.as_raw() == code
        || vk::Result::ERROR_LAYER_NOT_PRESENT.as_raw() == code
        || vk::Result::ERROR_EXTENSION_NOT_PRESENT.as_raw() == code
        || vk::Result::ERROR_FEATURE_NOT_PRESENT.as_raw() == code;
    let incompatible_driver = vk::Result::ERROR_INCOMPATIBLE_DRIVER.as_raw() == code;
    let device_lost = vk::Result::ERROR_DEVICE_LOST.as_raw() == code;
    let too_many = vk::Result::ERROR_TOO_MANY_OBJECTS.as_raw() == code
        || vk::Result::ERROR_FORMAT_NOT_SUPPORTED.as_raw() == code;

    let result = if device_lost {
        crate::bump_device_generation();
        RenderResult::DeviceLost
    } else if out_of_memory {
        RenderResult::OutOfMemory
    } else if init_failed {
        RenderResult::InitFailed
    } else if incompatible_driver {
        RenderResult::Unsupported
    } else if too_many {
        RenderResult::VulkanError
    } else if code == vk::Result::SUCCESS.as_raw() {
        RenderResult::Ok
    } else {
        RenderResult::VulkanError
    };
    (result, format!("{} failed with vkResult {}", op, code))
}
