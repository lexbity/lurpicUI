//! Native surface creation via ash (Q16).
//!
//! The window lives in Go (XCB connection/window on Linux, `ANativeWindow` on
//! Android); we keep the existing FFI entry points and reimplement them with
//! `ash::vk::XcbSurfaceCreateInfoKHR` / `AndroidSurfaceCreateInfoKHR`. No
//! `raw-window-handle` fabrication is required.

use ash::vk;
use std::ffi::c_void;

use crate::error::vk_error;
use crate::RenderResult;

/// Creates an XCB surface and verifies the graphics queue family can present
/// to it.
#[cfg(not(target_os = "android"))]
pub fn create_xcb_surface(
    entry: &ash::Entry,
    instance: &ash::Instance,
    surface_loader: &ash::khr::surface::Instance,
    physical_device: vk::PhysicalDevice,
    queue_family: u32,
    connection: usize,
    window: u32,
) -> Result<vk::SurfaceKHR, (RenderResult, String)> {
    let xcb_loader = ash::khr::xcb_surface::Instance::new(entry, instance);
    let create_info = vk::XcbSurfaceCreateInfoKHR {
        connection: connection as *mut c_void,
        window,
        ..Default::default()
    };
    let surface = unsafe { xcb_loader.create_xcb_surface(&create_info, None) }
        .map_err(|e| vk_error("vkCreateXcbSurfaceKHR", e.as_raw()))?;
    if let Err(err) = verify_present_support(surface_loader, physical_device, queue_family, surface) {
        unsafe {
            surface_loader.destroy_surface(surface, None);
        }
        return Err(err);
    }
    Ok(surface)
}

/// Creates an Android surface and verifies the graphics queue family can
/// present to it.
#[cfg(target_os = "android")]
pub fn create_android_surface(
    entry: &ash::Entry,
    instance: &ash::Instance,
    surface_loader: &ash::khr::surface::Instance,
    physical_device: vk::PhysicalDevice,
    queue_family: u32,
    window: *mut c_void,
) -> Result<vk::SurfaceKHR, (RenderResult, String)> {
    let android_loader = ash::khr::android_surface::Instance::new(entry, instance);
    let create_info = vk::AndroidSurfaceCreateInfoKHR {
        window,
        ..Default::default()
    };
    let surface = unsafe { android_loader.create_android_surface(&create_info, None) }
        .map_err(|e| vk_error("vkCreateAndroidSurfaceKHR", e.as_raw()))?;
    if let Err(err) = verify_present_support(surface_loader, physical_device, queue_family, surface) {
        unsafe {
            surface_loader.destroy_surface(surface, None);
        }
        return Err(err);
    }
    Ok(surface)
}

fn verify_present_support(
    surface_loader: &ash::khr::surface::Instance,
    physical_device: vk::PhysicalDevice,
    queue_family: u32,
    surface: vk::SurfaceKHR,
) -> Result<(), (RenderResult, String)> {
    let supported = unsafe {
        surface_loader.get_physical_device_surface_support(physical_device, queue_family, surface)
    }
    .map_err(|e| vk_error("vkGetPhysicalDeviceSurfaceSupportKHR", e.as_raw()))?;
    if !supported {
        return Err((
            RenderResult::InitFailed,
            "graphics queue family cannot present to this surface".to_string(),
        ));
    }
    Ok(())
}
