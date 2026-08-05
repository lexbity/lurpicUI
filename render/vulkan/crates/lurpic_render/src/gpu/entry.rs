//! Entry-point loading per-platform (Q14): runtime dlopen on desktop via
//! `Entry::load()`/`Entry::load_from()`, static link on Android via
//! `Entry::linked()`. Honours `LURPIC_RENDER_VULKAN_LIBRARY` for an explicit
//! loader path override.

use crate::RenderResult;

pub fn load_entry() -> Result<ash::Entry, (RenderResult, String)> {
    #[cfg(target_os = "android")]
    {
        // Vulkan is a mandatory Android system library since API 24; static
        // linking removes a runtime failure mode.
        Ok(ash::Entry::linked())
    }
    #[cfg(not(target_os = "android"))]
    {
        if let Ok(path) = std::env::var("LURPIC_RENDER_VULKAN_LIBRARY") {
            if !path.is_empty() {
                return unsafe { ash::Entry::load_from(&path) }.map_err(|e| {
                    (
                        RenderResult::Unsupported,
                        format!("load Vulkan loader from {}: {}", path, e),
                    )
                });
            }
        }
        unsafe { ash::Entry::load() }.map_err(|e| {
            (
                RenderResult::Unsupported,
                format!("load Vulkan loader: {}", e),
            )
        })
    }
}
