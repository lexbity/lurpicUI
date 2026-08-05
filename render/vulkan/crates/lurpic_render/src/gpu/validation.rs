//! Validation layers and debug messaging (Q15).
//!
//! `VK_LAYER_KHRONOS_validation` is enabled only when `LURPIC_RENDER_VALIDATION=1`
//! AND the layer is present (graceful degradation). The debug messenger routes
//! validation messages through a callback that counts errors/warnings and logs
//! them; `validation_error_count()` lets tests assert a clean session.

use ash::vk;
use std::ffi::{c_char, c_void, CStr, CString};
use std::sync::atomic::{AtomicUsize, Ordering};

use crate::error::vk_error;
use crate::RenderResult;

static VALIDATION_ERRORS: AtomicUsize = AtomicUsize::new(0);
static VALIDATION_WARNINGS: AtomicUsize = AtomicUsize::new(0);

#[allow(dead_code)] // consumed by the foundation/validation tests
pub fn validation_error_count() -> usize {
    VALIDATION_ERRORS.load(Ordering::Relaxed)
}

#[allow(dead_code)]
pub fn validation_warning_count() -> usize {
    VALIDATION_WARNINGS.load(Ordering::Relaxed)
}

#[allow(dead_code)]
pub fn reset_validation_counts() {
    VALIDATION_ERRORS.store(0, Ordering::Relaxed);
    VALIDATION_WARNINGS.store(0, Ordering::Relaxed);
}

unsafe extern "system" fn debug_utils_callback(
    message_severity: vk::DebugUtilsMessageSeverityFlagsEXT,
    message_types: vk::DebugUtilsMessageTypeFlagsEXT,
    p_callback_data: *const vk::DebugUtilsMessengerCallbackDataEXT,
    _p_user_data: *mut c_void,
) -> vk::Bool32 {
    if message_severity.contains(vk::DebugUtilsMessageSeverityFlagsEXT::ERROR) {
        VALIDATION_ERRORS.fetch_add(1, Ordering::Relaxed);
    }
    if message_severity.contains(vk::DebugUtilsMessageSeverityFlagsEXT::WARNING) {
        VALIDATION_WARNINGS.fetch_add(1, Ordering::Relaxed);
    }
    let message = unsafe { p_callback_data.as_ref() }
        .and_then(|data| (!data.p_message.is_null()).then(|| CStr::from_ptr(data.p_message)))
        .and_then(|cstr| cstr.to_str().ok())
        .unwrap_or("<no validation message>");
    eprintln!(
        "vulkan validation [{:?} {:?}] {}",
        message_severity, message_types, message
    );
    vk::FALSE
}

fn c_char_array_to_string(bytes: &[c_char]) -> String {
    let bytes: Vec<u8> = bytes.iter().map(|&b| b as u8).collect();
    let end = bytes.iter().position(|&b| b == 0).unwrap_or(bytes.len());
    String::from_utf8_lossy(&bytes[..end]).into_owned()
}

fn layer_available(entry: &ash::Entry, name: &str) -> Result<bool, (RenderResult, String)> {
    let props = unsafe { entry.enumerate_instance_layer_properties() }
        .map_err(|e| vk_error("vkEnumerateInstanceLayerProperties", e.as_raw()))?;
    Ok(props
        .iter()
        .any(|p| c_char_array_to_string(&p.layer_name) == name))
}

fn extension_available(entry: &ash::Entry, name: &str) -> Result<bool, (RenderResult, String)> {
    let props = unsafe { entry.enumerate_instance_extension_properties(None) }
        .map_err(|e| vk_error("vkEnumerateInstanceExtensionProperties", e.as_raw()))?;
    Ok(props
        .iter()
        .any(|p| c_char_array_to_string(&p.extension_name) == name))
}

/// Owns the validation layer/extension names and the debug messenger. Dropped
/// before the instance (callers place it as a field ahead of the instance).
pub struct ValidationContext {
    enabled: bool,
    layer_names: Vec<CString>,
    extension_names: Vec<CString>,
    messenger_create_info: Option<vk::DebugUtilsMessengerCreateInfoEXT<'static>>,
    debug_utils: Option<ash::ext::debug_utils::Instance>,
    messenger: Option<vk::DebugUtilsMessengerEXT>,
}

#[allow(dead_code)] // configured by init; used by validation tests
impl ValidationContext {
    /// A disabled context (no layers, no messenger).
    pub fn none() -> Self {
        Self {
            enabled: false,
            layer_names: Vec::new(),
            extension_names: Vec::new(),
            messenger_create_info: None,
            debug_utils: None,
            messenger: None,
        }
    }

    /// Builds the layer/extension lists and the messenger create info. Called
    /// before instance creation; `attach()` creates the messenger afterwards.
    pub fn new(entry: &ash::Entry, enabled: bool) -> Result<Self, (RenderResult, String)> {
        let mut layer_names = Vec::new();
        let mut extension_names = Vec::new();
        let mut messenger_create_info = None;

        if enabled {
            if layer_available(entry, "VK_LAYER_KHRONOS_validation")? {
                layer_names.push(CString::new("VK_LAYER_KHRONOS_validation").unwrap());
            } else {
                eprintln!(
                    "lurpic_render: validation requested but VK_LAYER_KHRONOS_validation \
                     is not installed"
                );
            }
            if extension_available(entry, "VK_EXT_debug_utils")? {
                extension_names.push(CString::new("VK_EXT_debug_utils").unwrap());
                messenger_create_info = Some(vk::DebugUtilsMessengerCreateInfoEXT {
                    message_severity: vk::DebugUtilsMessageSeverityFlagsEXT::WARNING
                        | vk::DebugUtilsMessageSeverityFlagsEXT::ERROR,
                    message_type: vk::DebugUtilsMessageTypeFlagsEXT::GENERAL
                        | vk::DebugUtilsMessageTypeFlagsEXT::VALIDATION
                        | vk::DebugUtilsMessageTypeFlagsEXT::PERFORMANCE,
                    pfn_user_callback: Some(debug_utils_callback),
                    ..Default::default()
                });
            }
        }

        let enabled = enabled && messenger_create_info.is_some();
        Ok(Self {
            enabled,
            layer_names,
            extension_names,
            messenger_create_info,
            debug_utils: None,
            messenger: None,
        })
    }

    pub fn enabled(&self) -> bool {
        self.enabled
    }

    pub fn layer_ptrs(&self) -> Vec<*const c_char> {
        self.layer_names.iter().map(|s| s.as_ptr()).collect()
    }

    #[allow(dead_code)]
    pub fn extension_ptrs(&self) -> Vec<*const c_char> {
        self.extension_names.iter().map(|s| s.as_ptr()).collect()
    }

    /// The messenger create info chained into `InstanceCreateInfo` so the layer
    /// can emit during instance creation itself.
    pub fn messenger_create_info(&self) -> Option<&vk::DebugUtilsMessengerCreateInfoEXT<'static>> {
        self.messenger_create_info.as_ref()
    }

    /// Creates the debug messenger against a live instance.
    pub fn attach(
        &mut self,
        entry: &ash::Entry,
        instance: &ash::Instance,
    ) -> Result<(), (RenderResult, String)> {
        if !self.enabled {
            return Ok(());
        }
        let Some(create_info) = self.messenger_create_info.as_ref() else {
            return Ok(());
        };
        let debug_utils = ash::ext::debug_utils::Instance::new(entry, instance);
        let messenger = unsafe { debug_utils.create_debug_utils_messenger(create_info, None) }
            .map_err(|e| vk_error("vkCreateDebugUtilsMessengerEXT", e.as_raw()))?;
        self.debug_utils = Some(debug_utils);
        self.messenger = Some(messenger);
        Ok(())
    }
}

impl Drop for ValidationContext {
    fn drop(&mut self) {
        if let (Some(debug_utils), Some(messenger)) =
            (self.debug_utils.take(), self.messenger.take())
        {
            unsafe {
                debug_utils.destroy_debug_utils_messenger(messenger, None);
            }
        }
    }
}
