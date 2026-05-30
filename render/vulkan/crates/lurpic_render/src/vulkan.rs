use crate::frame::{decode_frame, DecodedFrame, FrameStats};
use crate::raster::rasterize_frame;
use crate::{clear_last_error, RenderResult};
use std::ffi::{c_char, c_void, CStr, CString};
use std::ptr;
use std::sync::{Mutex, OnceLock};

#[repr(C)]
#[derive(Clone, Copy)]
pub struct VulkanCapabilities {
    pub device_name: [c_char; 256],
    pub device_type: i32,
    pub api_version: u32,
    pub driver_version: u32,
    pub max_texture_dimension_2d: u32,
    pub graphics_queue_family_index: u32,
    pub present_queue_family_index: u32,
    pub transfer_queue_family_index: u32,
}

impl VulkanCapabilities {
    pub fn empty() -> Self {
        Self {
            device_name: [0; 256],
            device_type: VK_PHYSICAL_DEVICE_TYPE_OTHER,
            api_version: 0,
            driver_version: 0,
            max_texture_dimension_2d: 0,
            graphics_queue_family_index: 0,
            present_queue_family_index: 0,
            transfer_queue_family_index: 0,
        }
    }
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkApplicationInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    p_application_name: *const c_char,
    application_version: u32,
    p_engine_name: *const c_char,
    engine_version: u32,
    api_version: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkInstanceCreateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkInstanceCreateFlags,
    p_application_info: *const VkApplicationInfo,
    enabled_layer_count: u32,
    pp_enabled_layer_names: *const *const c_char,
    enabled_extension_count: u32,
    pp_enabled_extension_names: *const *const c_char,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkDeviceQueueCreateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkDeviceQueueCreateFlags,
    queue_family_index: u32,
    queue_count: u32,
    p_queue_priorities: *const f32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkDeviceCreateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkDeviceCreateFlags,
    queue_create_info_count: u32,
    p_queue_create_infos: *const VkDeviceQueueCreateInfo,
    enabled_layer_count: u32,
    pp_enabled_layer_names: *const *const c_char,
    enabled_extension_count: u32,
    pp_enabled_extension_names: *const *const c_char,
    p_enabled_features: *const c_void,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkQueueFamilyProperties {
    queue_flags: VkQueueFlags,
    queue_count: u32,
    timestamp_valid_bits: u32,
    min_image_transfer_granularity: VkExtent3D,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkExtent3D {
    width: u32,
    height: u32,
    depth: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkPhysicalDeviceProperties {
    api_version: u32,
    driver_version: u32,
    vendor_id: u32,
    device_id: u32,
    device_type: VkPhysicalDeviceType,
    device_name: [c_char; VK_MAX_PHYSICAL_DEVICE_NAME_SIZE as usize],
    pipeline_cache_uuid: [u8; VK_UUID_SIZE as usize],
    limits: [u32; 256],
    sparse_properties: [u32; 16],
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkPhysicalDeviceMemoryProperties {
    memory_type_count: u32,
    memory_types: [VkMemoryType; 32],
    memory_heap_count: u32,
    memory_heaps: [VkMemoryHeap; 16],
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkMemoryType {
    property_flags: VkMemoryPropertyFlags,
    heap_index: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkMemoryHeap {
    size: VkDeviceSize,
    flags: VkFlags,
}

type VkInstance = *mut c_void;
type VkPhysicalDevice = *mut c_void;
type VkDevice = *mut c_void;
type VkQueue = *mut c_void;
type VkSurfaceKHR = *mut c_void;
type VkSwapchainKHR = *mut c_void;
type VkImage = *mut c_void;
type VkCommandPool = *mut c_void;
type VkCommandBuffer = *mut c_void;
type VkBuffer = *mut c_void;
type VkDeviceMemory = *mut c_void;
type VkDebugUtilsMessengerEXT = *mut c_void;
type VkFlags = u32;
type VkDeviceSize = u64;
type VkBool32 = u32;
type VkStructureType = i32;
type VkFormat = i32;
type VkColorSpaceKHR = i32;
type VkPresentModeKHR = i32;
type VkSharingMode = i32;
type VkImageLayout = i32;
type VkImageUsageFlags = VkFlags;
type VkImageAspectFlags = VkFlags;
type VkBufferUsageFlags = VkFlags;
type VkMemoryPropertyFlags = VkFlags;
type VkCommandPoolCreateFlags = VkFlags;
type VkCommandBufferUsageFlags = VkFlags;
type VkPipelineStageFlags = VkFlags;
type VkAccessFlags = VkFlags;
type VkCompositeAlphaFlagsKHR = VkFlags;
type VkSurfaceTransformFlagsKHR = VkFlags;
type VkCommandPoolResetFlags = VkFlags;
type VkInstanceCreateFlags = VkFlags;
type VkDeviceQueueCreateFlags = VkFlags;
type VkDeviceCreateFlags = VkFlags;
type VkQueueFlags = VkFlags;
type VkDebugUtilsMessageSeverityFlagsEXT = VkFlags;
type VkDebugUtilsMessageTypeFlagsEXT = VkFlags;
type VkPhysicalDeviceType = i32;
type VkResult = i32;
const VK_STRUCTURE_TYPE_APPLICATION_INFO: VkStructureType = 0;
const VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO: VkStructureType = 1;
const VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO: VkStructureType = 2;
const VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO: VkStructureType = 3;
const VK_STRUCTURE_TYPE_DEBUG_UTILS_MESSENGER_CREATE_INFO_EXT: VkStructureType = 1000128004;
const VK_STRUCTURE_TYPE_XCB_SURFACE_CREATE_INFO_KHR: VkStructureType = 1000005000;
const VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR: VkStructureType = 1000001000;
const VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO: VkStructureType = 39;
const VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO: VkStructureType = 40;
const VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO: VkStructureType = 42;
const VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO: VkStructureType = 12;
const VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO: VkStructureType = 5;
const VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER: VkStructureType = 45;
const VK_STRUCTURE_TYPE_PRESENT_INFO_KHR: VkStructureType = 1000001001;
const VK_STRUCTURE_TYPE_SUBMIT_INFO: VkStructureType = 4;

const VK_API_VERSION_1_0: u32 = 1 << 22;
const VK_MAX_PHYSICAL_DEVICE_NAME_SIZE: u32 = 256;
const VK_UUID_SIZE: u32 = 16;

const VK_QUEUE_GRAPHICS_BIT: VkQueueFlags = 0x0000_0001;
const VK_PHYSICAL_DEVICE_TYPE_OTHER: VkPhysicalDeviceType = 0;
const VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU: VkPhysicalDeviceType = 1;
const VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU: VkPhysicalDeviceType = 2;
const VK_PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU: VkPhysicalDeviceType = 3;
const VK_PHYSICAL_DEVICE_TYPE_CPU: VkPhysicalDeviceType = 4;

const VK_SUCCESS: VkResult = 0;
const VK_ERROR_OUT_OF_HOST_MEMORY: VkResult = -1;
const VK_ERROR_OUT_OF_DEVICE_MEMORY: VkResult = -2;
const VK_ERROR_INITIALIZATION_FAILED: VkResult = -3;
const VK_ERROR_DEVICE_LOST: VkResult = -4;
const VK_ERROR_LAYER_NOT_PRESENT: VkResult = -6;
const VK_ERROR_EXTENSION_NOT_PRESENT: VkResult = -7;
const VK_ERROR_FEATURE_NOT_PRESENT: VkResult = -8;
const VK_ERROR_INCOMPATIBLE_DRIVER: VkResult = -9;
const VK_ERROR_TOO_MANY_OBJECTS: VkResult = -10;
const VK_ERROR_FORMAT_NOT_SUPPORTED: VkResult = -11;
const VK_ERROR_OUT_OF_DATE_KHR: VkResult = -1000001004;
const VK_SUBOPTIMAL_KHR: VkResult = 1000001003;

const VK_DEBUG_UTILS_MESSAGE_SEVERITY_VERBOSE_BIT_EXT: VkDebugUtilsMessageSeverityFlagsEXT =
    0x0000_0001;
const VK_DEBUG_UTILS_MESSAGE_SEVERITY_INFO_BIT_EXT: VkDebugUtilsMessageSeverityFlagsEXT =
    0x0000_0010;
const VK_DEBUG_UTILS_MESSAGE_SEVERITY_WARNING_BIT_EXT: VkDebugUtilsMessageSeverityFlagsEXT =
    0x0000_0100;
const VK_DEBUG_UTILS_MESSAGE_SEVERITY_ERROR_BIT_EXT: VkDebugUtilsMessageSeverityFlagsEXT =
    0x0000_1000;

const VK_DEBUG_UTILS_MESSAGE_TYPE_GENERAL_BIT_EXT: VkDebugUtilsMessageTypeFlagsEXT = 0x0000_0001;
const VK_DEBUG_UTILS_MESSAGE_TYPE_VALIDATION_BIT_EXT: VkDebugUtilsMessageTypeFlagsEXT = 0x0000_0002;
const VK_DEBUG_UTILS_MESSAGE_TYPE_PERFORMANCE_BIT_EXT: VkDebugUtilsMessageTypeFlagsEXT =
    0x0000_0004;
const VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR: VkCompositeAlphaFlagsKHR = 0x0000_0001;
const VK_IMAGE_USAGE_TRANSFER_DST_BIT: VkImageUsageFlags = 0x0000_0002;
const VK_BUFFER_USAGE_TRANSFER_SRC_BIT: VkBufferUsageFlags = 0x0000_0001;
const VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT: VkPipelineStageFlags = 0x0000_0001;
const VK_PIPELINE_STAGE_TRANSFER_BIT: VkPipelineStageFlags = 0x0000_0100;
const VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT: VkPipelineStageFlags = 0x0000_0200;
const VK_ACCESS_TRANSFER_WRITE_BIT: VkAccessFlags = 0x0000_0800;
const VK_IMAGE_ASPECT_COLOR_BIT: VkImageAspectFlags = 0x0000_0001;
const VK_QUEUE_FAMILY_IGNORED: u32 = u32::MAX;
const VK_SHARING_MODE_EXCLUSIVE: VkSharingMode = 0;
const VK_PRESENT_MODE_FIFO_KHR: VkPresentModeKHR = 2;
const VK_PRESENT_MODE_MAILBOX_KHR: VkPresentModeKHR = 1;
const VK_FORMAT_B8G8R8A8_UNORM: VkFormat = 44;
const VK_COLOR_SPACE_SRGB_NONLINEAR_KHR: VkColorSpaceKHR = 0;
const VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL: VkImageLayout = 6;
const VK_IMAGE_LAYOUT_PRESENT_SRC_KHR: VkImageLayout = 1000001002;
const VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT: VkCommandPoolCreateFlags = 0x0000_0002;
const VK_COMMAND_BUFFER_LEVEL_PRIMARY: i32 = 0;
const VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT: VkCommandBufferUsageFlags = 0x0000_0001;
const VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT: VkMemoryPropertyFlags = 0x0000_0002;
const VK_MEMORY_PROPERTY_HOST_COHERENT_BIT: VkMemoryPropertyFlags = 0x0000_0004;

struct VulkanState {
    _loader: VulkanLoader,
    instance: VkInstance,
    physical_device: VkPhysicalDevice,
    device: VkDevice,
    queue: VkQueue,
    debug_messenger: VkDebugUtilsMessengerEXT,
    surface: VkSurfaceKHR,
    swapchain: VkSwapchainKHR,
    command_pool: VkCommandPool,
    command_buffer: VkCommandBuffer,
    requested_width: u32,
    requested_height: u32,
    swapchain_extent_width: u32,
    swapchain_extent_height: u32,
    swapchain_format: VkFormat,
    swapchain_images: Vec<VkImage>,
    swapchain_image_views: Vec<*mut c_void>,
    staging_buffer: VkBuffer,
    staging_memory: VkDeviceMemory,
    staging_mapped: *mut c_void,
    staging_size: usize,
    memory_properties: VkPhysicalDeviceMemoryProperties,
    pending_frame: Option<DecodedFrame>,
    last_frame: Option<DecodedFrame>,
    destroy_device: unsafe extern "system" fn(VkDevice, *const c_void),
    destroy_instance: unsafe extern "system" fn(VkInstance, *const c_void),
    destroy_debug_utils_messenger:
        Option<unsafe extern "system" fn(VkInstance, VkDebugUtilsMessengerEXT, *const c_void)>,
    capabilities: VulkanCapabilities,
}

unsafe impl Send for VulkanState {}
unsafe impl Sync for VulkanState {}

impl VulkanState {
    unsafe fn shutdown(&mut self) {
        self.destroy_swapchain_resources();
        self.pending_frame = None;
        self.last_frame = None;
        self.destroy_staging_resources();
        if !self.surface.is_null() {
            if let Ok(destroy_surface) = self._loader.load_instance_proc::<PfnVkDestroySurfaceKHR>(
                self.instance,
                cstr(b"vkDestroySurfaceKHR\0"),
            ) {
                destroy_surface(self.instance, self.surface, ptr::null());
            }
            self.surface = ptr::null_mut();
        }
        if let Some(destroy_debug_utils_messenger) = self.destroy_debug_utils_messenger {
            if !self.debug_messenger.is_null() {
                destroy_debug_utils_messenger(self.instance, self.debug_messenger, ptr::null());
            }
        }
        (self.destroy_device)(self.device, ptr::null());
        (self.destroy_instance)(self.instance, ptr::null());
        self.debug_messenger = ptr::null_mut();
        self.device = ptr::null_mut();
        self.instance = ptr::null_mut();
        self.physical_device = ptr::null_mut();
        self.queue = ptr::null_mut();
    }

    fn instance_handle(&self) -> usize {
        self.instance as usize
    }

    unsafe fn destroy_swapchain_resources(&mut self) {
        if !self.swapchain_image_views.is_empty() {
            if let Ok(destroy_image_view) = self._loader.load_device_proc::<PfnVkDestroyImageView>(
                self.device,
                cstr(b"vkDestroyImageView\0"),
            ) {
                for view in self.swapchain_image_views.drain(..) {
                    if !view.is_null() {
                        destroy_image_view(self.device, view, ptr::null());
                    }
                }
            } else {
                self.swapchain_image_views.clear();
            }
        }
        if !self.command_pool.is_null() {
            if let Ok(destroy_command_pool) =
                self._loader.load_device_proc::<PfnVkDestroyCommandPool>(
                    self.device,
                    cstr(b"vkDestroyCommandPool\0"),
                )
            {
                destroy_command_pool(self.device, self.command_pool, ptr::null());
            }
            self.command_pool = ptr::null_mut();
            self.command_buffer = ptr::null_mut();
        }
        if !self.swapchain.is_null() {
            if let Ok(destroy_swapchain) =
                self._loader.load_device_proc::<PfnVkDestroySwapchainKHR>(
                    self.device,
                    cstr(b"vkDestroySwapchainKHR\0"),
                )
            {
                destroy_swapchain(self.device, self.swapchain, ptr::null());
            }
            self.swapchain = ptr::null_mut();
            self.swapchain_images.clear();
        }
    }

    unsafe fn destroy_staging_resources(&mut self) {
        if !self.staging_mapped.is_null() {
            if let Ok(unmap_memory) = self
                ._loader
                .load_device_proc::<PfnVkUnmapMemory>(self.device, cstr(b"vkUnmapMemory\0"))
            {
                unmap_memory(self.device, self.staging_memory);
            }
            self.staging_mapped = ptr::null_mut();
        }
        if !self.staging_buffer.is_null() {
            if let Ok(destroy_buffer) = self
                ._loader
                .load_device_proc::<PfnVkDestroyBuffer>(self.device, cstr(b"vkDestroyBuffer\0"))
            {
                destroy_buffer(self.device, self.staging_buffer, ptr::null());
            }
            self.staging_buffer = ptr::null_mut();
        }
        if !self.staging_memory.is_null() {
            if let Ok(free_memory) = self
                ._loader
                .load_device_proc::<PfnVkFreeMemory>(self.device, cstr(b"vkFreeMemory\0"))
            {
                free_memory(self.device, self.staging_memory, ptr::null());
            }
            self.staging_memory = ptr::null_mut();
        }
        self.staging_size = 0;
    }

    unsafe fn ensure_staging_buffer(&mut self, size: usize) -> Result<(), (RenderResult, String)> {
        if size == 0 {
            return Err((
                RenderResult::InitFailed,
                "staging buffer size is zero".to_string(),
            ));
        }
        if self.staging_size >= size
            && !self.staging_buffer.is_null()
            && !self.staging_mapped.is_null()
        {
            return Ok(());
        }

        self.destroy_staging_resources();

        let create_buffer: PfnVkCreateBuffer = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkCreateBuffer\0"))?;
        let destroy_buffer: PfnVkDestroyBuffer = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkDestroyBuffer\0"))?;
        let get_requirements: PfnVkGetBufferMemoryRequirements = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkGetBufferMemoryRequirements\0"))?;
        let allocate_memory: PfnVkAllocateMemory = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkAllocateMemory\0"))?;
        let bind_buffer_memory: PfnVkBindBufferMemory = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkBindBufferMemory\0"))?;
        let map_memory: PfnVkMapMemory = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkMapMemory\0"))?;

        let create_info = VkBufferCreateInfo {
            s_type: VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,
            p_next: ptr::null(),
            flags: 0,
            size: size as VkDeviceSize,
            usage: VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
            sharing_mode: VK_SHARING_MODE_EXCLUSIVE,
            queue_family_index_count: 0,
            p_queue_family_indices: ptr::null(),
        };
        let mut buffer: VkBuffer = ptr::null_mut();
        let rc = create_buffer(self.device, &create_info, ptr::null(), &mut buffer);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkCreateBuffer", rc));
        }

        let mut reqs = VkMemoryRequirements {
            size: 0,
            alignment: 0,
            memory_type_bits: 0,
        };
        get_requirements(self.device, buffer, &mut reqs);

        let memory_type_index = find_memory_type_index(
            &self.memory_properties,
            reqs.memory_type_bits,
            VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
        )
        .ok_or_else(|| {
            (
                RenderResult::InitFailed,
                "no host-visible coherent memory type found".to_string(),
            )
        })?;

        let alloc_info = VkMemoryAllocateInfo {
            s_type: VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
            p_next: ptr::null(),
            allocation_size: reqs.size,
            memory_type_index,
        };
        let mut memory: VkDeviceMemory = ptr::null_mut();
        let rc = allocate_memory(self.device, &alloc_info, ptr::null(), &mut memory);
        if rc != VK_SUCCESS {
            destroy_buffer(self.device, buffer, ptr::null());
            return Err(vk_error("vkAllocateMemory", rc));
        }

        let rc = bind_buffer_memory(self.device, buffer, memory, 0);
        if rc != VK_SUCCESS {
            let free_memory: PfnVkFreeMemory = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkFreeMemory\0"))?;
            free_memory(self.device, memory, ptr::null());
            destroy_buffer(self.device, buffer, ptr::null());
            return Err(vk_error("vkBindBufferMemory", rc));
        }

        let mut mapped: *mut c_void = ptr::null_mut();
        let rc = map_memory(self.device, memory, 0, reqs.size, 0, &mut mapped);
        if rc != VK_SUCCESS {
            let free_memory: PfnVkFreeMemory = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkFreeMemory\0"))?;
            free_memory(self.device, memory, ptr::null());
            destroy_buffer(self.device, buffer, ptr::null());
            return Err(vk_error("vkMapMemory", rc));
        }

        self.staging_buffer = buffer;
        self.staging_memory = memory;
        self.staging_mapped = mapped;
        self.staging_size = size;
        Ok(())
    }

    unsafe fn upload_staging_pixels(
        &mut self,
        pixels: &[u8],
    ) -> Result<(), (RenderResult, String)> {
        if self.staging_mapped.is_null() {
            return Err((
                RenderResult::InitFailed,
                "staging buffer is not mapped".to_string(),
            ));
        }
        if pixels.len() > self.staging_size {
            return Err((
                RenderResult::OutOfMemory,
                "staging buffer is too small".to_string(),
            ));
        }
        ptr::copy_nonoverlapping(
            pixels.as_ptr(),
            self.staging_mapped as *mut u8,
            pixels.len(),
        );
        Ok(())
    }

    unsafe fn create_xcb_surface(
        &mut self,
        connection: *mut c_void,
        window: u32,
        width: u32,
        height: u32,
    ) -> Result<VkSurfaceKHR, (RenderResult, String)> {
        if self.device.is_null() || self.instance.is_null() {
            return Err((
                RenderResult::InitFailed,
                "renderer is not initialized".to_string(),
            ));
        }
        if !self.surface.is_null() {
            return Ok(self.surface);
        }

        let create_xcb_surface: PfnVkCreateXcbSurfaceKHR = self
            ._loader
            .load_instance_proc(self.instance, cstr(b"vkCreateXcbSurfaceKHR\0"))?;
        let get_surface_support: PfnVkGetPhysicalDeviceSurfaceSupportKHR =
            self._loader.load_instance_proc(
                self.instance,
                cstr(b"vkGetPhysicalDeviceSurfaceSupportKHR\0"),
            )?;

        let create_info = VkXcbSurfaceCreateInfoKHR {
            s_type: VK_STRUCTURE_TYPE_XCB_SURFACE_CREATE_INFO_KHR,
            p_next: ptr::null(),
            flags: 0,
            connection,
            window,
        };
        let mut surface: VkSurfaceKHR = ptr::null_mut();
        let rc = create_xcb_surface(self.instance, &create_info, ptr::null(), &mut surface);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkCreateXcbSurfaceKHR", rc));
        }

        let mut supported = 0u32;
        let rc = get_surface_support(
            self.physical_device,
            self.capabilities.graphics_queue_family_index,
            surface,
            &mut supported,
        );
        if rc != VK_SUCCESS {
            if let Ok(destroy_surface) = self._loader.load_instance_proc::<PfnVkDestroySurfaceKHR>(
                self.instance,
                cstr(b"vkDestroySurfaceKHR\0"),
            ) {
                destroy_surface(self.instance, surface, ptr::null());
            }
            return Err(vk_error("vkGetPhysicalDeviceSurfaceSupportKHR", rc));
        }
        if supported == 0 {
            if let Ok(destroy_surface) = self._loader.load_instance_proc::<PfnVkDestroySurfaceKHR>(
                self.instance,
                cstr(b"vkDestroySurfaceKHR\0"),
            ) {
                destroy_surface(self.instance, surface, ptr::null());
            }
            return Err((
                RenderResult::InitFailed,
                "graphics queue family cannot present to the XCB surface".to_string(),
            ));
        }

        self.surface = surface;
        self.requested_width = width;
        self.requested_height = height;
        if let Err(err) = self.recreate_swapchain() {
            if let Ok(destroy_surface) = self._loader.load_instance_proc::<PfnVkDestroySurfaceKHR>(
                self.instance,
                cstr(b"vkDestroySurfaceKHR\0"),
            ) {
                destroy_surface(self.instance, surface, ptr::null());
            }
            self.surface = ptr::null_mut();
            return Err(err);
        }
        Ok(surface)
    }

    unsafe fn resize(&mut self, width: u32, height: u32) -> Result<(), (RenderResult, String)> {
        if self.surface.is_null() {
            self.requested_width = width;
            self.requested_height = height;
            return Ok(());
        }
        self.requested_width = width;
        self.requested_height = height;
        self.recreate_swapchain()
    }

    unsafe fn present(&mut self) -> Result<(), (RenderResult, String)> {
        if self.swapchain.is_null() || self.command_buffer.is_null() {
            return Err((
                RenderResult::InitFailed,
                "swapchain is not ready".to_string(),
            ));
        }
        let frame = if let Some(frame) = self.pending_frame.take() {
            self.last_frame = Some(frame.clone());
            Some(frame)
        } else {
            self.last_frame.clone()
        };
        let pixels = rasterize_frame(
            frame.as_ref(),
            self.swapchain_extent_width,
            self.swapchain_extent_height,
        );
        self.upload_staging_pixels(&pixels)?;

        let acquire_next_image: PfnVkAcquireNextImageKHR = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkAcquireNextImageKHR\0"))?;
        let reset_command_pool: PfnVkResetCommandPool = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkResetCommandPool\0"))?;
        let begin_command_buffer: PfnVkBeginCommandBuffer = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkBeginCommandBuffer\0"))?;
        let end_command_buffer: PfnVkEndCommandBuffer = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkEndCommandBuffer\0"))?;
        let cmd_pipeline_barrier: PfnVkCmdPipelineBarrier = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkCmdPipelineBarrier\0"))?;
        let cmd_copy_buffer_to_image: PfnVkCmdCopyBufferToImage = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkCmdCopyBufferToImage\0"))?;
        let queue_submit: PfnVkQueueSubmit = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkQueueSubmit\0"))?;
        let queue_wait_idle: PfnVkQueueWaitIdle = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkQueueWaitIdle\0"))?;
        let queue_present: PfnVkQueuePresentKHR = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkQueuePresentKHR\0"))?;

        let mut image_index = 0u32;
        let rc = acquire_next_image(
            self.device,
            self.swapchain,
            u64::MAX,
            ptr::null_mut(),
            ptr::null_mut(),
            &mut image_index,
        );
        if rc == VK_ERROR_OUT_OF_DATE_KHR || rc == VK_SUBOPTIMAL_KHR {
            self.recreate_swapchain()?;
            return Ok(());
        }
        if rc != VK_SUCCESS {
            return Err(vk_error("vkAcquireNextImageKHR", rc));
        }
        if image_index as usize >= self.swapchain_images.len() {
            return Err((
                RenderResult::VulkanError,
                "acquired swapchain image index out of range".to_string(),
            ));
        }

        let reset_rc = reset_command_pool(self.device, self.command_pool, 0);
        if reset_rc != VK_SUCCESS {
            return Err(vk_error("vkResetCommandPool", reset_rc));
        }

        let begin_info = VkCommandBufferBeginInfo {
            s_type: VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,
            p_next: ptr::null(),
            flags: VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT,
            p_inheritance_info: ptr::null(),
        };
        let rc = begin_command_buffer(self.command_buffer, &begin_info);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkBeginCommandBuffer", rc));
        }

        let barrier_to_transfer = VkImageMemoryBarrier {
            s_type: VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
            p_next: ptr::null(),
            src_access_mask: 0,
            dst_access_mask: VK_ACCESS_TRANSFER_WRITE_BIT,
            old_layout: VK_IMAGE_LAYOUT_PRESENT_SRC_KHR,
            new_layout: VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
            src_queue_family_index: VK_QUEUE_FAMILY_IGNORED,
            dst_queue_family_index: VK_QUEUE_FAMILY_IGNORED,
            image: self.swapchain_images[image_index as usize],
            subresource_range: VkImageSubresourceRange {
                aspect_mask: VK_IMAGE_ASPECT_COLOR_BIT,
                base_mip_level: 0,
                level_count: 1,
                base_array_layer: 0,
                layer_count: 1,
            },
        };
        cmd_pipeline_barrier(
            self.command_buffer,
            VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT,
            VK_PIPELINE_STAGE_TRANSFER_BIT,
            0,
            0,
            ptr::null(),
            0,
            ptr::null(),
            1,
            &barrier_to_transfer,
        );

        let copy_region = VkBufferImageCopy {
            buffer_offset: 0,
            buffer_row_length: 0,
            buffer_image_height: 0,
            image_subresource: VkImageSubresourceLayers {
                aspect_mask: VK_IMAGE_ASPECT_COLOR_BIT,
                mip_level: 0,
                base_array_layer: 0,
                layer_count: 1,
            },
            image_offset: VkOffset3D { x: 0, y: 0, z: 0 },
            image_extent: VkExtent3D {
                width: self.swapchain_extent_width,
                height: self.swapchain_extent_height,
                depth: 1,
            },
        };
        cmd_copy_buffer_to_image(
            self.command_buffer,
            self.staging_buffer,
            self.swapchain_images[image_index as usize],
            VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
            1,
            &copy_region,
        );

        let barrier_to_present = VkImageMemoryBarrier {
            s_type: VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
            p_next: ptr::null(),
            src_access_mask: VK_ACCESS_TRANSFER_WRITE_BIT,
            dst_access_mask: 0,
            old_layout: VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
            new_layout: VK_IMAGE_LAYOUT_PRESENT_SRC_KHR,
            src_queue_family_index: VK_QUEUE_FAMILY_IGNORED,
            dst_queue_family_index: VK_QUEUE_FAMILY_IGNORED,
            image: self.swapchain_images[image_index as usize],
            subresource_range: barrier_to_transfer.subresource_range,
        };
        cmd_pipeline_barrier(
            self.command_buffer,
            VK_PIPELINE_STAGE_TRANSFER_BIT,
            VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT,
            0,
            0,
            ptr::null(),
            0,
            ptr::null(),
            1,
            &barrier_to_present,
        );

        let rc = end_command_buffer(self.command_buffer);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkEndCommandBuffer", rc));
        }

        let submit_info = VkSubmitInfo {
            s_type: VK_STRUCTURE_TYPE_SUBMIT_INFO,
            p_next: ptr::null(),
            wait_semaphore_count: 0,
            p_wait_semaphores: ptr::null(),
            p_wait_dst_stage_mask: ptr::null(),
            command_buffer_count: 1,
            p_command_buffers: &self.command_buffer,
            signal_semaphore_count: 0,
            p_signal_semaphores: ptr::null(),
        };
        let rc = queue_submit(self.queue, 1, &submit_info, ptr::null());
        if rc != VK_SUCCESS {
            return Err(vk_error("vkQueueSubmit", rc));
        }
        let rc = queue_wait_idle(self.queue);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkQueueWaitIdle", rc));
        }

        let swapchain = self.swapchain;
        let present_info = VkPresentInfoKHR {
            s_type: VK_STRUCTURE_TYPE_PRESENT_INFO_KHR,
            p_next: ptr::null(),
            wait_semaphore_count: 0,
            p_wait_semaphores: ptr::null(),
            swapchain_count: 1,
            p_swapchains: &swapchain,
            p_image_indices: &image_index,
            p_results: ptr::null_mut(),
        };
        let rc = queue_present(self.queue, &present_info);
        if rc == VK_ERROR_OUT_OF_DATE_KHR || rc == VK_SUBOPTIMAL_KHR {
            self.recreate_swapchain()?;
            return Ok(());
        }
        if rc != VK_SUCCESS {
            return Err(vk_error("vkQueuePresentKHR", rc));
        }
        Ok(())
    }

    fn submit_frame(&mut self, frame: DecodedFrame) {
        self.pending_frame = Some(frame);
    }

    fn frame_stats(&self) -> FrameStats {
        self.pending_frame
            .as_ref()
            .or(self.last_frame.as_ref())
            .map(|frame| frame.stats)
            .unwrap_or_default()
    }

    unsafe fn recreate_swapchain(&mut self) -> Result<(), (RenderResult, String)> {
        if self.surface.is_null() {
            return Err((
                RenderResult::InitFailed,
                "Vulkan surface has not been created".to_string(),
            ));
        }

        self.destroy_staging_resources();
        self.destroy_swapchain_resources();

        let get_surface_capabilities: PfnVkGetPhysicalDeviceSurfaceCapabilitiesKHR =
            self._loader.load_instance_proc(
                self.instance,
                cstr(b"vkGetPhysicalDeviceSurfaceCapabilitiesKHR\0"),
            )?;
        let get_surface_formats: PfnVkGetPhysicalDeviceSurfaceFormatsKHR =
            self._loader.load_instance_proc(
                self.instance,
                cstr(b"vkGetPhysicalDeviceSurfaceFormatsKHR\0"),
            )?;
        let get_surface_present_modes: PfnVkGetPhysicalDeviceSurfacePresentModesKHR =
            self._loader.load_instance_proc(
                self.instance,
                cstr(b"vkGetPhysicalDeviceSurfacePresentModesKHR\0"),
            )?;
        let create_swapchain: PfnVkCreateSwapchainKHR = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkCreateSwapchainKHR\0"))?;
        let get_swapchain_images: PfnVkGetSwapchainImagesKHR = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkGetSwapchainImagesKHR\0"))?;
        let create_command_pool: PfnVkCreateCommandPool = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkCreateCommandPool\0"))?;
        let allocate_command_buffers: PfnVkAllocateCommandBuffers = self
            ._loader
            .load_device_proc(self.device, cstr(b"vkAllocateCommandBuffers\0"))?;

        let mut caps = VkSurfaceCapabilitiesKHR {
            min_image_count: 0,
            max_image_count: 0,
            current_extent: VkExtent2D {
                width: 0,
                height: 0,
            },
            min_image_extent: VkExtent2D {
                width: 0,
                height: 0,
            },
            max_image_extent: VkExtent2D {
                width: 0,
                height: 0,
            },
            max_image_array_layers: 0,
            supported_transforms: 0,
            current_transform: 0,
            supported_composite_alpha: 0,
            supported_usage_flags: 0,
        };
        let rc = get_surface_capabilities(self.physical_device, self.surface, &mut caps);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkGetPhysicalDeviceSurfaceCapabilitiesKHR", rc));
        }

        let extent = choose_extent(&caps, self.requested_width, self.requested_height);
        let formats =
            query_surface_formats(get_surface_formats, self.physical_device, self.surface)?;
        let format = choose_surface_format(&formats);
        let present_mode = choose_present_mode(
            query_present_modes(
                get_surface_present_modes,
                self.physical_device,
                self.surface,
            )?
            .as_slice(),
        );

        let desired_image_count = caps.min_image_count.saturating_add(1);
        let image_count = if caps.max_image_count > 0 {
            desired_image_count.min(caps.max_image_count)
        } else {
            desired_image_count
        };

        let create_info = VkSwapchainCreateInfoKHR {
            s_type: VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR,
            p_next: ptr::null(),
            flags: 0,
            surface: self.surface,
            min_image_count: image_count.max(1),
            image_format: format.format,
            image_color_space: format.color_space,
            image_extent: extent,
            image_array_layers: 1,
            image_usage: if caps.supported_usage_flags & VK_IMAGE_USAGE_TRANSFER_DST_BIT != 0 {
                VK_IMAGE_USAGE_TRANSFER_DST_BIT
            } else {
                caps.supported_usage_flags
            },
            image_sharing_mode: VK_SHARING_MODE_EXCLUSIVE,
            queue_family_index_count: 0,
            p_queue_family_indices: ptr::null(),
            pre_transform: caps.current_transform,
            composite_alpha: choose_composite_alpha(caps.supported_composite_alpha),
            present_mode,
            clipped: 1,
            old_swapchain: ptr::null_mut(),
        };

        let mut swapchain: VkSwapchainKHR = ptr::null_mut();
        let rc = create_swapchain(self.device, &create_info, ptr::null(), &mut swapchain);
        if rc != VK_SUCCESS {
            return Err(vk_error("vkCreateSwapchainKHR", rc));
        }

        let mut image_count = 0u32;
        let rc = get_swapchain_images(self.device, swapchain, &mut image_count, ptr::null_mut());
        if rc != VK_SUCCESS {
            let destroy_swapchain: PfnVkDestroySwapchainKHR = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkDestroySwapchainKHR\0"))?;
            destroy_swapchain(self.device, swapchain, ptr::null());
            return Err(vk_error("vkGetSwapchainImagesKHR(count)", rc));
        }
        let mut images = vec![ptr::null_mut(); image_count as usize];
        let rc = get_swapchain_images(
            self.device,
            swapchain,
            &mut image_count,
            images.as_mut_ptr(),
        );
        if rc != VK_SUCCESS {
            let destroy_swapchain: PfnVkDestroySwapchainKHR = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkDestroySwapchainKHR\0"))?;
            destroy_swapchain(self.device, swapchain, ptr::null());
            return Err(vk_error("vkGetSwapchainImagesKHR(list)", rc));
        }

        let command_pool_info = VkCommandPoolCreateInfo {
            s_type: VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
            p_next: ptr::null(),
            flags: VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT,
            queue_family_index: self.capabilities.graphics_queue_family_index,
        };
        let mut command_pool: VkCommandPool = ptr::null_mut();
        let rc = create_command_pool(
            self.device,
            &command_pool_info,
            ptr::null(),
            &mut command_pool,
        );
        if rc != VK_SUCCESS {
            let destroy_swapchain: PfnVkDestroySwapchainKHR = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkDestroySwapchainKHR\0"))?;
            destroy_swapchain(self.device, swapchain, ptr::null());
            return Err(vk_error("vkCreateCommandPool", rc));
        }

        let alloc_info = VkCommandBufferAllocateInfo {
            s_type: VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
            p_next: ptr::null(),
            command_pool,
            level: VK_COMMAND_BUFFER_LEVEL_PRIMARY,
            command_buffer_count: 1,
        };
        let mut command_buffer = ptr::null_mut();
        let rc = allocate_command_buffers(self.device, &alloc_info, &mut command_buffer);
        if rc != VK_SUCCESS {
            let destroy_command_pool: PfnVkDestroyCommandPool = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkDestroyCommandPool\0"))?;
            let destroy_swapchain: PfnVkDestroySwapchainKHR = self
                ._loader
                .load_device_proc(self.device, cstr(b"vkDestroySwapchainKHR\0"))?;
            destroy_command_pool(self.device, command_pool, ptr::null());
            destroy_swapchain(self.device, swapchain, ptr::null());
            return Err(vk_error("vkAllocateCommandBuffers", rc));
        }

        self.swapchain = swapchain;
        self.command_pool = command_pool;
        self.command_buffer = command_buffer;
        self.swapchain_images = images;
        self.swapchain_format = format.format;
        self.swapchain_extent_width = extent.width;
        self.swapchain_extent_height = extent.height;
        self.ensure_staging_buffer((extent.width as usize) * (extent.height as usize) * 4)?;
        Ok(())
    }
}

#[derive(Clone)]
struct VulkanLoader {
    handle: *mut c_void,
    vk_create_instance: unsafe extern "system" fn(
        *const VkInstanceCreateInfo,
        *const c_void,
        *mut VkInstance,
    ) -> VkResult,
    vk_enumerate_instance_extension_properties:
        unsafe extern "system" fn(*const c_char, *mut u32, *mut VkExtensionProperties) -> VkResult,
    vk_enumerate_instance_layer_properties:
        unsafe extern "system" fn(*mut u32, *mut VkLayerProperties) -> VkResult,
    vk_get_instance_proc_addr:
        unsafe extern "system" fn(VkInstance, *const c_char) -> PfnVkVoidFunction,
    vk_get_device_proc_addr:
        unsafe extern "system" fn(VkDevice, *const c_char) -> PfnVkVoidFunction,
}

unsafe impl Send for VulkanLoader {}
unsafe impl Sync for VulkanLoader {}

type PfnVkVoidFunction = Option<unsafe extern "system" fn()>;
type PfnVkCreateDebugUtilsMessengerEXT = unsafe extern "system" fn(
    VkInstance,
    *const VkDebugUtilsMessengerCreateInfoEXT,
    *const c_void,
    *mut VkDebugUtilsMessengerEXT,
) -> VkResult;
type PfnVkDestroyDebugUtilsMessengerEXT =
    unsafe extern "system" fn(VkInstance, VkDebugUtilsMessengerEXT, *const c_void);
type PfnVkDebugUtilsMessengerCallbackEXT = Option<
    unsafe extern "system" fn(
        VkDebugUtilsMessageSeverityFlagsEXT,
        VkDebugUtilsMessageTypeFlagsEXT,
        *const VkDebugUtilsMessengerCallbackDataEXT,
        *mut c_void,
    ) -> VkBool32,
>;
type PfnVkCreateXcbSurfaceKHR = unsafe extern "system" fn(
    VkInstance,
    *const VkXcbSurfaceCreateInfoKHR,
    *const c_void,
    *mut VkSurfaceKHR,
) -> VkResult;
type PfnVkDestroySurfaceKHR = unsafe extern "system" fn(VkInstance, VkSurfaceKHR, *const c_void);
type PfnVkGetPhysicalDeviceSurfaceSupportKHR =
    unsafe extern "system" fn(VkPhysicalDevice, u32, VkSurfaceKHR, *mut VkBool32) -> VkResult;
type PfnVkGetPhysicalDeviceSurfaceCapabilitiesKHR = unsafe extern "system" fn(
    VkPhysicalDevice,
    VkSurfaceKHR,
    *mut VkSurfaceCapabilitiesKHR,
) -> VkResult;
type PfnVkGetPhysicalDeviceSurfaceFormatsKHR = unsafe extern "system" fn(
    VkPhysicalDevice,
    VkSurfaceKHR,
    *mut u32,
    *mut VkSurfaceFormatKHR,
) -> VkResult;
type PfnVkGetPhysicalDeviceSurfacePresentModesKHR = unsafe extern "system" fn(
    VkPhysicalDevice,
    VkSurfaceKHR,
    *mut u32,
    *mut VkPresentModeKHR,
) -> VkResult;
type PfnVkDestroyImageView = unsafe extern "system" fn(VkDevice, *mut c_void, *const c_void);
type PfnVkGetPhysicalDeviceMemoryProperties =
    unsafe extern "system" fn(VkPhysicalDevice, *mut VkPhysicalDeviceMemoryProperties);
type PfnVkCreateBuffer = unsafe extern "system" fn(
    VkDevice,
    *const VkBufferCreateInfo,
    *const c_void,
    *mut VkBuffer,
) -> VkResult;
type PfnVkDestroyBuffer = unsafe extern "system" fn(VkDevice, VkBuffer, *const c_void);
type PfnVkGetBufferMemoryRequirements =
    unsafe extern "system" fn(VkDevice, VkBuffer, *mut VkMemoryRequirements);
type PfnVkAllocateMemory = unsafe extern "system" fn(
    VkDevice,
    *const VkMemoryAllocateInfo,
    *const c_void,
    *mut VkDeviceMemory,
) -> VkResult;
type PfnVkFreeMemory = unsafe extern "system" fn(VkDevice, VkDeviceMemory, *const c_void);
type PfnVkBindBufferMemory =
    unsafe extern "system" fn(VkDevice, VkBuffer, VkDeviceMemory, VkDeviceSize) -> VkResult;
type PfnVkMapMemory = unsafe extern "system" fn(
    VkDevice,
    VkDeviceMemory,
    VkDeviceSize,
    VkDeviceSize,
    VkFlags,
    *mut *mut c_void,
) -> VkResult;
type PfnVkUnmapMemory = unsafe extern "system" fn(VkDevice, VkDeviceMemory);
type PfnVkCmdCopyBufferToImage = unsafe extern "system" fn(
    VkCommandBuffer,
    VkBuffer,
    VkImage,
    VkImageLayout,
    u32,
    *const VkBufferImageCopy,
);
type PfnVkCreateSwapchainKHR = unsafe extern "system" fn(
    VkDevice,
    *const VkSwapchainCreateInfoKHR,
    *const c_void,
    *mut VkSwapchainKHR,
) -> VkResult;
type PfnVkDestroySwapchainKHR = unsafe extern "system" fn(VkDevice, VkSwapchainKHR, *const c_void);
type PfnVkGetSwapchainImagesKHR =
    unsafe extern "system" fn(VkDevice, VkSwapchainKHR, *mut u32, *mut VkImage) -> VkResult;
type PfnVkAcquireNextImageKHR = unsafe extern "system" fn(
    VkDevice,
    VkSwapchainKHR,
    u64,
    *mut c_void,
    *mut c_void,
    *mut u32,
) -> VkResult;
type PfnVkQueuePresentKHR = unsafe extern "system" fn(VkQueue, *const VkPresentInfoKHR) -> VkResult;
type PfnVkCreateCommandPool = unsafe extern "system" fn(
    VkDevice,
    *const VkCommandPoolCreateInfo,
    *const c_void,
    *mut VkCommandPool,
) -> VkResult;
type PfnVkDestroyCommandPool = unsafe extern "system" fn(VkDevice, VkCommandPool, *const c_void);
type PfnVkAllocateCommandBuffers = unsafe extern "system" fn(
    VkDevice,
    *const VkCommandBufferAllocateInfo,
    *mut VkCommandBuffer,
) -> VkResult;
type PfnVkBeginCommandBuffer =
    unsafe extern "system" fn(VkCommandBuffer, *const VkCommandBufferBeginInfo) -> VkResult;
type PfnVkEndCommandBuffer = unsafe extern "system" fn(VkCommandBuffer) -> VkResult;
type PfnVkResetCommandPool =
    unsafe extern "system" fn(VkDevice, VkCommandPool, VkCommandPoolResetFlags) -> VkResult;
type PfnVkCmdPipelineBarrier = unsafe extern "system" fn(
    VkCommandBuffer,
    VkPipelineStageFlags,
    VkPipelineStageFlags,
    VkFlags,
    u32,
    *const c_void,
    u32,
    *const c_void,
    u32,
    *const VkImageMemoryBarrier,
);
type PfnVkQueueSubmit =
    unsafe extern "system" fn(VkQueue, u32, *const VkSubmitInfo, *const c_void) -> VkResult;
type PfnVkQueueWaitIdle = unsafe extern "system" fn(VkQueue) -> VkResult;

#[repr(C)]
#[derive(Clone, Copy)]
struct VkExtensionProperties {
    extension_name: [c_char; VK_MAX_EXTENSION_NAME_SIZE as usize],
    spec_version: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkLayerProperties {
    layer_name: [c_char; VK_MAX_EXTENSION_NAME_SIZE as usize],
    spec_version: u32,
    implementation_version: u32,
    description: [c_char; VK_MAX_DESCRIPTION_SIZE as usize],
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkDebugUtilsMessengerCreateInfoEXT {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkFlags,
    message_severity: VkDebugUtilsMessageSeverityFlagsEXT,
    message_type: VkDebugUtilsMessageTypeFlagsEXT,
    pfn_user_callback: PfnVkDebugUtilsMessengerCallbackEXT,
    p_user_data: *mut c_void,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkDebugUtilsMessengerCallbackDataEXT {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkFlags,
    p_message_id_name: *const c_char,
    message_id_number: i32,
    p_message: *const c_char,
    queue_label_count: u32,
    p_queue_labels: *const c_void,
    cmd_buf_label_count: u32,
    p_cmd_buf_labels: *const c_void,
    object_count: u32,
    p_objects: *const c_void,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkExtent2D {
    width: u32,
    height: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkXcbSurfaceCreateInfoKHR {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkFlags,
    connection: *mut c_void,
    window: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkSurfaceCapabilitiesKHR {
    min_image_count: u32,
    max_image_count: u32,
    current_extent: VkExtent2D,
    min_image_extent: VkExtent2D,
    max_image_extent: VkExtent2D,
    max_image_array_layers: u32,
    supported_transforms: VkSurfaceTransformFlagsKHR,
    current_transform: VkSurfaceTransformFlagsKHR,
    supported_composite_alpha: VkCompositeAlphaFlagsKHR,
    supported_usage_flags: VkImageUsageFlags,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkSurfaceFormatKHR {
    format: VkFormat,
    color_space: VkColorSpaceKHR,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkSwapchainCreateInfoKHR {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkFlags,
    surface: VkSurfaceKHR,
    min_image_count: u32,
    image_format: VkFormat,
    image_color_space: VkColorSpaceKHR,
    image_extent: VkExtent2D,
    image_array_layers: u32,
    image_usage: VkImageUsageFlags,
    image_sharing_mode: VkSharingMode,
    queue_family_index_count: u32,
    p_queue_family_indices: *const u32,
    pre_transform: VkSurfaceTransformFlagsKHR,
    composite_alpha: VkCompositeAlphaFlagsKHR,
    present_mode: VkPresentModeKHR,
    clipped: VkBool32,
    old_swapchain: VkSwapchainKHR,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkImageSubresourceRange {
    aspect_mask: VkImageAspectFlags,
    base_mip_level: u32,
    level_count: u32,
    base_array_layer: u32,
    layer_count: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkImageMemoryBarrier {
    s_type: VkStructureType,
    p_next: *const c_void,
    src_access_mask: VkAccessFlags,
    dst_access_mask: VkAccessFlags,
    old_layout: VkImageLayout,
    new_layout: VkImageLayout,
    src_queue_family_index: u32,
    dst_queue_family_index: u32,
    image: VkImage,
    subresource_range: VkImageSubresourceRange,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkCommandPoolCreateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkCommandPoolCreateFlags,
    queue_family_index: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkCommandBufferAllocateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    command_pool: VkCommandPool,
    level: i32,
    command_buffer_count: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkCommandBufferBeginInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkCommandBufferUsageFlags,
    p_inheritance_info: *const c_void,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkSubmitInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    wait_semaphore_count: u32,
    p_wait_semaphores: *const c_void,
    p_wait_dst_stage_mask: *const VkPipelineStageFlags,
    command_buffer_count: u32,
    p_command_buffers: *const VkCommandBuffer,
    signal_semaphore_count: u32,
    p_signal_semaphores: *const c_void,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkBufferCreateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    flags: VkFlags,
    size: VkDeviceSize,
    usage: VkBufferUsageFlags,
    sharing_mode: VkSharingMode,
    queue_family_index_count: u32,
    p_queue_family_indices: *const u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkMemoryAllocateInfo {
    s_type: VkStructureType,
    p_next: *const c_void,
    allocation_size: VkDeviceSize,
    memory_type_index: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkMemoryRequirements {
    size: VkDeviceSize,
    alignment: VkDeviceSize,
    memory_type_bits: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkOffset3D {
    x: i32,
    y: i32,
    z: i32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkImageSubresourceLayers {
    aspect_mask: VkImageAspectFlags,
    mip_level: u32,
    base_array_layer: u32,
    layer_count: u32,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkBufferImageCopy {
    buffer_offset: VkDeviceSize,
    buffer_row_length: u32,
    buffer_image_height: u32,
    image_subresource: VkImageSubresourceLayers,
    image_offset: VkOffset3D,
    image_extent: VkExtent3D,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct VkPresentInfoKHR {
    s_type: VkStructureType,
    p_next: *const c_void,
    wait_semaphore_count: u32,
    p_wait_semaphores: *const c_void,
    swapchain_count: u32,
    p_swapchains: *const VkSwapchainKHR,
    p_image_indices: *const u32,
    p_results: *mut VkResult,
}

const VK_MAX_EXTENSION_NAME_SIZE: u32 = 256;
const VK_MAX_DESCRIPTION_SIZE: u32 = 256;

impl VulkanLoader {
    fn load() -> Result<Self, (RenderResult, String)> {
        let path = library_path()?;
        unsafe {
            let handle = dlopen(path.as_ptr(), RTLD_NOW | RTLD_LOCAL);
            if handle.is_null() {
                return Err(last_dl_error("failed to open Vulkan loader"));
            }
            let loader = match Self::from_handle(handle) {
                Ok(loader) => loader,
                Err(err) => {
                    dlclose(handle);
                    return Err(err);
                }
            };
            Ok(loader)
        }
    }

    unsafe fn from_handle(handle: *mut c_void) -> Result<Self, (RenderResult, String)> {
        let vk_create_instance = load_symbol(handle, "vkCreateInstance")?;
        let vk_enumerate_instance_extension_properties =
            load_symbol(handle, "vkEnumerateInstanceExtensionProperties")?;
        let vk_enumerate_instance_layer_properties =
            load_symbol(handle, "vkEnumerateInstanceLayerProperties")?;
        let vk_get_instance_proc_addr = load_symbol(handle, "vkGetInstanceProcAddr")?;
        let vk_get_device_proc_addr = load_symbol(handle, "vkGetDeviceProcAddr")?;
        Ok(Self {
            handle,
            vk_create_instance,
            vk_enumerate_instance_extension_properties,
            vk_enumerate_instance_layer_properties,
            vk_get_instance_proc_addr,
            vk_get_device_proc_addr,
        })
    }

    unsafe fn load_instance_proc<T: Copy>(
        &self,
        instance: VkInstance,
        name: &CStr,
    ) -> Result<T, (RenderResult, String)> {
        let proc = (self.vk_get_instance_proc_addr)(instance, name.as_ptr());
        match proc {
            Some(proc) => Ok(transmute_copy_fn(proc)),
            None => Err((
                RenderResult::InitFailed,
                format!("vulkan: missing function {}", name.to_string_lossy()),
            )),
        }
    }

    unsafe fn load_device_proc<T: Copy>(
        &self,
        device: VkDevice,
        name: &CStr,
    ) -> Result<T, (RenderResult, String)> {
        let proc = (self.vk_get_device_proc_addr)(device, name.as_ptr());
        match proc {
            Some(proc) => Ok(transmute_copy_fn(proc)),
            None => Err((
                RenderResult::InitFailed,
                format!("vulkan: missing function {}", name.to_string_lossy()),
            )),
        }
    }
}

impl Drop for VulkanLoader {
    fn drop(&mut self) {
        unsafe {
            if !self.handle.is_null() {
                dlclose(self.handle);
            }
        }
    }
}

extern "C" {
    fn dlopen(filename: *const c_char, flag: i32) -> *mut c_void;
    fn dlclose(handle: *mut c_void) -> i32;
    fn dlsym(handle: *mut c_void, symbol: *const c_char) -> *mut c_void;
    fn dlerror() -> *const c_char;
}

const RTLD_NOW: i32 = 2;
const RTLD_LOCAL: i32 = 0;

pub fn init() -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    if guard.is_some() {
        clear_last_error();
        return Ok(());
    }

    let loader = VulkanLoader::load()?;
    let state = unsafe { init_with_loader(loader)? };
    *guard = Some(state);
    clear_last_error();
    Ok(())
}

pub fn shutdown() -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    if let Some(mut state) = guard.take() {
        unsafe {
            state.shutdown();
        }
    }
    clear_last_error();
    Ok(())
}

pub fn query_capabilities(out: *mut VulkanCapabilities) -> Result<(), (RenderResult, String)> {
    if out.is_null() {
        return Err((
            RenderResult::InvalidHandle,
            "output pointer is null".to_string(),
        ));
    }
    let guard = state_lock();
    let Some(state) = guard.as_ref() else {
        return Err((
            RenderResult::InitFailed,
            "renderer is not initialized".to_string(),
        ));
    };
    unsafe {
        ptr::write(out, state.capabilities);
    }
    clear_last_error();
    Ok(())
}

pub fn instance_handle() -> usize {
    let guard = state_lock();
    guard.as_ref().map_or(0, VulkanState::instance_handle)
}

pub fn create_xcb_surface(
    instance: usize,
    connection: usize,
    window: u32,
    width: u32,
    height: u32,
) -> Result<usize, (RenderResult, String)> {
    let mut guard = state_lock();
    let Some(state) = guard.as_mut() else {
        return Err((
            RenderResult::InitFailed,
            "renderer is not initialized".to_string(),
        ));
    };
    if state.instance_handle() != instance {
        return Err((
            RenderResult::InvalidHandle,
            "instance handle does not match the active renderer".to_string(),
        ));
    }
    unsafe {
        let surface = state.create_xcb_surface(connection as *mut c_void, window, width, height)?;
        Ok(surface as usize)
    }
}

pub fn resize(width: i32, height: i32) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let Some(state) = guard.as_mut() else {
        return Err((
            RenderResult::InitFailed,
            "renderer is not initialized".to_string(),
        ));
    };
    let width = width.max(1) as u32;
    let height = height.max(1) as u32;
    unsafe { state.resize(width, height) }
}

pub fn present() -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let Some(state) = guard.as_mut() else {
        return Err((
            RenderResult::InitFailed,
            "renderer is not initialized".to_string(),
        ));
    };
    unsafe { state.present() }
}

pub fn submit_frame(data: *const u8, len: usize) -> Result<(), (RenderResult, String)> {
    let mut guard = state_lock();
    let Some(state) = guard.as_mut() else {
        return Err((
            RenderResult::InitFailed,
            "renderer is not initialized".to_string(),
        ));
    };
    if len == 0 {
        state.pending_frame = None;
        return Ok(());
    }
    if data.is_null() {
        return Err((
            RenderResult::InvalidHandle,
            "frame packet pointer is null".to_string(),
        ));
    }
    let bytes = unsafe { std::slice::from_raw_parts(data, len) };
    let frame = decode_frame(bytes)?;
    state.submit_frame(frame);
    Ok(())
}

pub fn frame_stats() -> FrameStats {
    let guard = state_lock();
    guard
        .as_ref()
        .map_or_else(FrameStats::default, VulkanState::frame_stats)
}

unsafe fn init_with_loader(loader: VulkanLoader) -> Result<VulkanState, (RenderResult, String)> {
    let app_name = CString::new("lurpic_render").unwrap();
    let engine_name = CString::new("lurpic_render").unwrap();
    let app = VkApplicationInfo {
        s_type: VK_STRUCTURE_TYPE_APPLICATION_INFO,
        p_next: ptr::null(),
        p_application_name: app_name.as_ptr(),
        application_version: 1,
        p_engine_name: engine_name.as_ptr(),
        engine_version: 1,
        api_version: VK_API_VERSION_1_0,
    };

    let mut enabled_layers: Vec<*const c_char> = Vec::new();
    let debug_create_info = VkDebugUtilsMessengerCreateInfoEXT {
        s_type: VK_STRUCTURE_TYPE_DEBUG_UTILS_MESSENGER_CREATE_INFO_EXT,
        p_next: ptr::null(),
        flags: 0,
        message_severity: VK_DEBUG_UTILS_MESSAGE_SEVERITY_WARNING_BIT_EXT
            | VK_DEBUG_UTILS_MESSAGE_SEVERITY_ERROR_BIT_EXT,
        message_type: VK_DEBUG_UTILS_MESSAGE_TYPE_GENERAL_BIT_EXT
            | VK_DEBUG_UTILS_MESSAGE_TYPE_VALIDATION_BIT_EXT
            | VK_DEBUG_UTILS_MESSAGE_TYPE_PERFORMANCE_BIT_EXT,
        pfn_user_callback: Some(vulkan_debug_callback),
        p_user_data: ptr::null_mut(),
    };
    let mut instance_pnext: *const c_void = ptr::null();
    #[cfg(debug_assertions)]
    {
        let validation_layer = cstr(b"VK_LAYER_KHRONOS_validation\0");
        if layer_available(&loader, validation_layer)? {
            enabled_layers.push(validation_layer.as_ptr());
            if extension_available(&loader, None, "VK_EXT_debug_utils")? {
                instance_pnext = &debug_create_info as *const _ as *const c_void;
            }
        }
    }

    let mut enabled_extensions: Vec<*const c_char> = Vec::new();
    enabled_extensions.push(cstr_ptr("VK_KHR_surface"));
    enabled_extensions.push(cstr_ptr("VK_KHR_xcb_surface"));
    if extension_available(&loader, None, "VK_EXT_debug_utils")? {
        enabled_extensions.push(cstr_ptr("VK_EXT_debug_utils"));
    }

    let create_info = VkInstanceCreateInfo {
        s_type: VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
        p_next: instance_pnext,
        flags: 0,
        p_application_info: &app,
        enabled_layer_count: enabled_layers.len() as u32,
        pp_enabled_layer_names: if enabled_layers.is_empty() {
            ptr::null()
        } else {
            enabled_layers.as_ptr()
        },
        enabled_extension_count: enabled_extensions.len() as u32,
        pp_enabled_extension_names: if enabled_extensions.is_empty() {
            ptr::null()
        } else {
            enabled_extensions.as_ptr()
        },
    };

    let mut instance: VkInstance = ptr::null_mut();
    let rc = (loader.vk_create_instance)(&create_info, ptr::null(), &mut instance);
    if rc != VK_SUCCESS {
        return Err(vk_error("vkCreateInstance", rc));
    }

    let destroy_instance: unsafe extern "system" fn(VkInstance, *const c_void) =
        match loader.load_instance_proc(instance, cstr(b"vkDestroyInstance\0")) {
            Ok(proc) => proc,
            Err(err) => {
                return Err(err);
            }
        };
    let enumerate_physical_devices: unsafe extern "system" fn(
        VkInstance,
        *mut u32,
        *mut VkPhysicalDevice,
    ) -> VkResult = match loader.load_instance_proc(instance, cstr(b"vkEnumeratePhysicalDevices\0"))
    {
        Ok(proc) => proc,
        Err(err) => {
            destroy_instance(instance, ptr::null());
            return Err(err);
        }
    };
    let get_physical_device_properties: unsafe extern "system" fn(
        VkPhysicalDevice,
        *mut VkPhysicalDeviceProperties,
    ) = match loader.load_instance_proc(instance, cstr(b"vkGetPhysicalDeviceProperties\0")) {
        Ok(proc) => proc,
        Err(err) => {
            destroy_instance(instance, ptr::null());
            return Err(err);
        }
    };
    let get_physical_device_memory_properties: PfnVkGetPhysicalDeviceMemoryProperties =
        match loader.load_instance_proc(instance, cstr(b"vkGetPhysicalDeviceMemoryProperties\0")) {
            Ok(proc) => proc,
            Err(err) => {
                destroy_instance(instance, ptr::null());
                return Err(err);
            }
        };
    let get_queue_family_properties: unsafe extern "system" fn(
        VkPhysicalDevice,
        *mut u32,
        *mut VkQueueFamilyProperties,
    ) = match loader.load_instance_proc(
        instance,
        cstr(b"vkGetPhysicalDeviceQueueFamilyProperties\0"),
    ) {
        Ok(proc) => proc,
        Err(err) => {
            destroy_instance(instance, ptr::null());
            return Err(err);
        }
    };
    let create_device: unsafe extern "system" fn(
        VkPhysicalDevice,
        *const VkDeviceCreateInfo,
        *const c_void,
        *mut VkDevice,
    ) -> VkResult = match loader.load_instance_proc(instance, cstr(b"vkCreateDevice\0")) {
        Ok(proc) => proc,
        Err(err) => {
            destroy_instance(instance, ptr::null());
            return Err(err);
        }
    };
    let get_device_queue: unsafe extern "system" fn(VkDevice, u32, u32, *mut VkQueue) =
        match loader.load_instance_proc(instance, cstr(b"vkGetDeviceQueue\0")) {
            Ok(proc) => proc,
            Err(err) => {
                destroy_instance(instance, ptr::null());
                return Err(err);
            }
        };
    let destroy_device: unsafe extern "system" fn(VkDevice, *const c_void) =
        match loader.load_instance_proc(instance, cstr(b"vkDestroyDevice\0")) {
            Ok(proc) => proc,
            Err(err) => {
                destroy_instance(instance, ptr::null());
                return Err(err);
            }
        };

    let physical_device = match pick_physical_device(
        enumerate_physical_devices,
        get_physical_device_properties,
        get_queue_family_properties,
        instance,
    ) {
        Ok(device) => device,
        Err(err) => {
            destroy_instance(instance, ptr::null());
            return Err(err);
        }
    };
    let properties = physical_properties(get_physical_device_properties, physical_device);
    let memory_properties =
        physical_memory_properties(get_physical_device_memory_properties, physical_device);
    let graphics_queue_family = match pick_queue_family(
        get_queue_family_properties,
        physical_device,
        VK_QUEUE_GRAPHICS_BIT,
    ) {
        Ok(index) => index,
        Err(err) => {
            destroy_instance(instance, ptr::null());
            return Err(err);
        }
    };
    let queue_priority = 1.0f32;
    let queue_info = VkDeviceQueueCreateInfo {
        s_type: VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
        p_next: ptr::null(),
        flags: 0,
        queue_family_index: graphics_queue_family,
        queue_count: 1,
        p_queue_priorities: &queue_priority,
    };
    let device_info = VkDeviceCreateInfo {
        s_type: VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
        p_next: ptr::null(),
        flags: 0,
        queue_create_info_count: 1,
        p_queue_create_infos: &queue_info,
        enabled_layer_count: 0,
        pp_enabled_layer_names: ptr::null(),
        enabled_extension_count: 0,
        pp_enabled_extension_names: ptr::null(),
        p_enabled_features: ptr::null(),
    };
    let mut device: VkDevice = ptr::null_mut();
    let rc = create_device(physical_device, &device_info, ptr::null(), &mut device);
    if rc != VK_SUCCESS {
        unsafe {
            destroy_instance(instance, ptr::null());
        }
        return Err(vk_error("vkCreateDevice", rc));
    }

    let mut queue: VkQueue = ptr::null_mut();
    get_device_queue(device, graphics_queue_family, 0, &mut queue);
    let capabilities = capabilities_from_properties(&properties, graphics_queue_family);
    let mut debug_messenger = ptr::null_mut();
    let mut destroy_debug_utils_messenger = None;
    if !instance_pnext.is_null() {
        let create_debug_utils_messenger: PfnVkCreateDebugUtilsMessengerEXT =
            match loader.load_instance_proc(instance, cstr(b"vkCreateDebugUtilsMessengerEXT\0")) {
                Ok(proc) => proc,
                Err(err) => {
                    destroy_device(device, ptr::null());
                    destroy_instance(instance, ptr::null());
                    return Err(err);
                }
            };
        let destroy_debug_utils_messenger_fn: PfnVkDestroyDebugUtilsMessengerEXT =
            match loader.load_instance_proc(instance, cstr(b"vkDestroyDebugUtilsMessengerEXT\0")) {
                Ok(proc) => proc,
                Err(err) => {
                    destroy_device(device, ptr::null());
                    destroy_instance(instance, ptr::null());
                    return Err(err);
                }
            };
        let rc = create_debug_utils_messenger(
            instance,
            &debug_create_info,
            ptr::null(),
            &mut debug_messenger,
        );
        if rc != VK_SUCCESS {
            destroy_device(device, ptr::null());
            destroy_instance(instance, ptr::null());
            return Err(vk_error("vkCreateDebugUtilsMessengerEXT", rc));
        }
        destroy_debug_utils_messenger = Some(destroy_debug_utils_messenger_fn);
    }

    Ok(VulkanState {
        _loader: loader,
        instance,
        physical_device,
        device,
        queue,
        destroy_device,
        destroy_instance,
        debug_messenger,
        surface: ptr::null_mut(),
        swapchain: ptr::null_mut(),
        command_pool: ptr::null_mut(),
        command_buffer: ptr::null_mut(),
        requested_width: 1,
        requested_height: 1,
        swapchain_extent_width: 0,
        swapchain_extent_height: 0,
        swapchain_format: 0,
        swapchain_images: Vec::new(),
        swapchain_image_views: Vec::new(),
        staging_buffer: ptr::null_mut(),
        staging_memory: ptr::null_mut(),
        staging_mapped: ptr::null_mut(),
        staging_size: 0,
        memory_properties,
        pending_frame: None,
        last_frame: None,
        destroy_debug_utils_messenger,
        capabilities,
    })
}

fn state_lock() -> std::sync::MutexGuard<'static, Option<VulkanState>> {
    static STATE: OnceLock<Mutex<Option<VulkanState>>> = OnceLock::new();
    STATE
        .get_or_init(|| Mutex::new(None))
        .lock()
        .expect("vulkan state mutex poisoned")
}

fn capabilities_from_properties(
    props: &VkPhysicalDeviceProperties,
    graphics_queue_family: u32,
) -> VulkanCapabilities {
    let mut caps = VulkanCapabilities::empty();
    caps.device_type = props.device_type;
    caps.api_version = props.api_version;
    caps.driver_version = props.driver_version;
    caps.max_texture_dimension_2d = props.limits[1];
    caps.graphics_queue_family_index = graphics_queue_family;
    caps.present_queue_family_index = graphics_queue_family;
    caps.transfer_queue_family_index = graphics_queue_family;

    let raw = unsafe { CStr::from_ptr(props.device_name.as_ptr()) };
    let bytes = raw.to_bytes();
    let len = bytes.len().min(caps.device_name.len().saturating_sub(1));
    for (dst, src) in caps
        .device_name
        .iter_mut()
        .take(len)
        .zip(bytes.iter().take(len))
    {
        *dst = *src as c_char;
    }
    caps
}

fn physical_properties(
    get_physical_device_properties: unsafe extern "system" fn(
        VkPhysicalDevice,
        *mut VkPhysicalDeviceProperties,
    ),
    device: VkPhysicalDevice,
) -> VkPhysicalDeviceProperties {
    let mut props = VkPhysicalDeviceProperties {
        api_version: 0,
        driver_version: 0,
        vendor_id: 0,
        device_id: 0,
        device_type: VK_PHYSICAL_DEVICE_TYPE_OTHER,
        device_name: [0; VK_MAX_PHYSICAL_DEVICE_NAME_SIZE as usize],
        pipeline_cache_uuid: [0; VK_UUID_SIZE as usize],
        limits: [0; 256],
        sparse_properties: [0; 16],
    };
    unsafe {
        get_physical_device_properties(device, &mut props);
    }
    props
}

fn physical_memory_properties(
    get_physical_device_memory_properties: PfnVkGetPhysicalDeviceMemoryProperties,
    device: VkPhysicalDevice,
) -> VkPhysicalDeviceMemoryProperties {
    let mut props = VkPhysicalDeviceMemoryProperties {
        memory_type_count: 0,
        memory_types: [VkMemoryType {
            property_flags: 0,
            heap_index: 0,
        }; 32],
        memory_heap_count: 0,
        memory_heaps: [VkMemoryHeap { size: 0, flags: 0 }; 16],
    };
    unsafe {
        get_physical_device_memory_properties(device, &mut props);
    }
    props
}

fn find_memory_type_index(
    props: &VkPhysicalDeviceMemoryProperties,
    type_bits: u32,
    required: VkMemoryPropertyFlags,
) -> Option<u32> {
    let count = props.memory_type_count.min(32);
    for i in 0..count {
        let bit = 1u32 << i;
        if type_bits & bit == 0 {
            continue;
        }
        let mem_type = props.memory_types[i as usize];
        if mem_type.property_flags & required == required {
            return Some(i);
        }
    }
    None
}

fn pick_physical_device(
    enumerate: unsafe extern "system" fn(VkInstance, *mut u32, *mut VkPhysicalDevice) -> VkResult,
    get_props: unsafe extern "system" fn(VkPhysicalDevice, *mut VkPhysicalDeviceProperties),
    get_queues: unsafe extern "system" fn(VkPhysicalDevice, *mut u32, *mut VkQueueFamilyProperties),
    instance: VkInstance,
) -> Result<VkPhysicalDevice, (RenderResult, String)> {
    let mut count = 0u32;
    let rc = unsafe { enumerate(instance, &mut count, ptr::null_mut()) };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkEnumeratePhysicalDevices(count)", rc));
    }
    if count == 0 {
        return Err((
            RenderResult::Unsupported,
            "no Vulkan physical devices found".to_string(),
        ));
    }
    let mut devices = vec![ptr::null_mut(); count as usize];
    let rc = unsafe { enumerate(instance, &mut count, devices.as_mut_ptr()) };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkEnumeratePhysicalDevices(list)", rc));
    }

    let mut best: Option<(i32, VkPhysicalDevice)> = None;
    for device in devices {
        let props = physical_properties(get_props, device);
        let score = match props.device_type {
            VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU => 400,
            VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU => 300,
            VK_PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU => 200,
            VK_PHYSICAL_DEVICE_TYPE_CPU => 100,
            _ => 50,
        };
        let _graphics_queue = match pick_queue_family(get_queues, device, VK_QUEUE_GRAPHICS_BIT) {
            Ok(index) => index,
            Err(_) => continue,
        };
        let score = score + 10;
        if best.map_or(true, |(best_score, _)| score > best_score) {
            best = Some((score, device));
        }
    }

    best.map(|(_, device)| device).ok_or_else(|| {
        (
            RenderResult::Unsupported,
            "no suitable Vulkan physical device found".to_string(),
        )
    })
}

fn pick_queue_family(
    get_queues: unsafe extern "system" fn(VkPhysicalDevice, *mut u32, *mut VkQueueFamilyProperties),
    device: VkPhysicalDevice,
    required: VkQueueFlags,
) -> Result<u32, (RenderResult, String)> {
    let mut count = 0u32;
    unsafe {
        get_queues(device, &mut count, ptr::null_mut());
    }
    if count == 0 {
        return Err((
            RenderResult::InitFailed,
            "no Vulkan queue families found".to_string(),
        ));
    }
    let mut families = vec![
        VkQueueFamilyProperties {
            queue_flags: 0,
            queue_count: 0,
            timestamp_valid_bits: 0,
            min_image_transfer_granularity: VkExtent3D {
                width: 0,
                height: 0,
                depth: 0,
            },
        };
        count as usize
    ];
    unsafe {
        get_queues(device, &mut count, families.as_mut_ptr());
    }
    for (index, family) in families.iter().enumerate() {
        if family.queue_count > 0 && (family.queue_flags & required) != 0 {
            return Ok(index as u32);
        }
    }
    Err((
        RenderResult::InitFailed,
        "no suitable Vulkan queue family found".to_string(),
    ))
}

fn layer_available(loader: &VulkanLoader, layer: &CStr) -> Result<bool, (RenderResult, String)> {
    let mut count = 0u32;
    let rc =
        unsafe { (loader.vk_enumerate_instance_layer_properties)(&mut count, ptr::null_mut()) };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkEnumerateInstanceLayerProperties(count)", rc));
    }
    if count == 0 {
        return Ok(false);
    }
    let mut layers = vec![
        VkLayerProperties {
            layer_name: [0; VK_MAX_EXTENSION_NAME_SIZE as usize],
            spec_version: 0,
            implementation_version: 0,
            description: [0; VK_MAX_DESCRIPTION_SIZE as usize],
        };
        count as usize
    ];
    let rc =
        unsafe { (loader.vk_enumerate_instance_layer_properties)(&mut count, layers.as_mut_ptr()) };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkEnumerateInstanceLayerProperties(list)", rc));
    }
    Ok(layers
        .iter()
        .any(|p| unsafe { CStr::from_ptr(p.layer_name.as_ptr()) } == layer))
}

fn extension_available(
    loader: &VulkanLoader,
    layer: Option<&CStr>,
    name: &str,
) -> Result<bool, (RenderResult, String)> {
    let c_name = CString::new(name).unwrap();
    let layer_ptr = layer.map_or(ptr::null(), |l| l.as_ptr());
    let mut count = 0u32;
    let rc = unsafe {
        (loader.vk_enumerate_instance_extension_properties)(layer_ptr, &mut count, ptr::null_mut())
    };
    if rc != VK_SUCCESS {
        return Err(vk_error(
            "vkEnumerateInstanceExtensionProperties(count)",
            rc,
        ));
    }
    if count == 0 {
        return Ok(false);
    }
    let mut ext = vec![
        VkExtensionProperties {
            extension_name: [0; VK_MAX_EXTENSION_NAME_SIZE as usize],
            spec_version: 0,
        };
        count as usize
    ];
    let rc = unsafe {
        (loader.vk_enumerate_instance_extension_properties)(layer_ptr, &mut count, ext.as_mut_ptr())
    };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkEnumerateInstanceExtensionProperties(list)", rc));
    }
    Ok(ext
        .iter()
        .any(|p| unsafe { CStr::from_ptr(p.extension_name.as_ptr()) } == c_name.as_c_str()))
}

fn cstr(s: &'static [u8]) -> &'static CStr {
    unsafe { CStr::from_bytes_with_nul_unchecked(s) }
}

fn cstr_ptr(s: &'static str) -> *const c_char {
    match s {
        "VK_KHR_surface" => cstr(b"VK_KHR_surface\0").as_ptr(),
        "VK_KHR_xcb_surface" => cstr(b"VK_KHR_xcb_surface\0").as_ptr(),
        "VK_EXT_debug_utils" => cstr(b"VK_EXT_debug_utils\0").as_ptr(),
        _ => ptr::null(),
    }
}

fn library_path() -> Result<CString, (RenderResult, String)> {
    if let Ok(override_path) = std::env::var("LURPIC_RENDER_VULKAN_LIBRARY") {
        return CString::new(override_path).map_err(|_| {
            (
                RenderResult::InitFailed,
                "LURPIC_RENDER_VULKAN_LIBRARY contains an interior NUL byte".to_string(),
            )
        });
    }

    for candidate in ["libvulkan.so.1", "libvulkan.so"] {
        if let Ok(path) = CString::new(candidate) {
            return Ok(path);
        }
    }
    Err((
        RenderResult::InitFailed,
        "no Vulkan loader path available".to_string(),
    ))
}

fn load_symbol<T: Copy>(handle: *mut c_void, name: &str) -> Result<T, (RenderResult, String)> {
    let c_name = CString::new(name).unwrap();
    unsafe {
        dlerror();
        let sym = dlsym(handle, c_name.as_ptr());
        if sym.is_null() {
            return Err(last_dl_error(format!("missing Vulkan symbol {}", name)));
        }
        Ok(transmute_copy_ptr(sym))
    }
}

fn vk_error(op: &str, code: VkResult) -> (RenderResult, String) {
    let result = match code {
        VK_ERROR_OUT_OF_HOST_MEMORY | VK_ERROR_OUT_OF_DEVICE_MEMORY => RenderResult::OutOfMemory,
        VK_ERROR_INITIALIZATION_FAILED
        | VK_ERROR_LAYER_NOT_PRESENT
        | VK_ERROR_EXTENSION_NOT_PRESENT
        | VK_ERROR_FEATURE_NOT_PRESENT => RenderResult::InitFailed,
        VK_ERROR_INCOMPATIBLE_DRIVER => RenderResult::Unsupported,
        VK_ERROR_DEVICE_LOST => RenderResult::VulkanError,
        VK_ERROR_TOO_MANY_OBJECTS | VK_ERROR_FORMAT_NOT_SUPPORTED => RenderResult::VulkanError,
        _ if code == VK_SUCCESS => RenderResult::Ok,
        _ => RenderResult::VulkanError,
    };
    (result, format!("{} failed with vkResult {}", op, code))
}

fn last_dl_error(context: impl AsRef<str>) -> (RenderResult, String) {
    unsafe {
        let ptr = dlerror();
        if ptr.is_null() {
            return (RenderResult::Unsupported, context.as_ref().to_string());
        }
        let msg = CStr::from_ptr(ptr).to_string_lossy().into_owned();
        (
            RenderResult::Unsupported,
            format!("{}: {}", context.as_ref(), msg),
        )
    }
}

unsafe fn transmute_copy_fn<T: Copy>(proc: unsafe extern "system" fn()) -> T {
    std::mem::transmute_copy(&proc)
}

unsafe fn transmute_copy_ptr<T: Copy>(sym: *mut c_void) -> T {
    std::mem::transmute_copy(&sym)
}

unsafe extern "system" fn vulkan_debug_callback(
    severity: VkDebugUtilsMessageSeverityFlagsEXT,
    message_types: VkDebugUtilsMessageTypeFlagsEXT,
    callback_data: *const VkDebugUtilsMessengerCallbackDataEXT,
    _user_data: *mut c_void,
) -> VkBool32 {
    let severity_name = debug_severity_name(severity);
    let type_name = debug_message_type_name(message_types);
    let message = if callback_data.is_null() {
        "<no validation message>".to_string()
    } else {
        let message_ptr = (*callback_data).p_message;
        if message_ptr.is_null() {
            "<validation message was null>".to_string()
        } else {
            CStr::from_ptr(message_ptr).to_string_lossy().into_owned()
        }
    };
    eprintln!("vulkan [{}|{}] {}", severity_name, type_name, message);
    0
}

fn debug_severity_name(severity: VkDebugUtilsMessageSeverityFlagsEXT) -> &'static str {
    if severity & VK_DEBUG_UTILS_MESSAGE_SEVERITY_ERROR_BIT_EXT != 0 {
        "error"
    } else if severity & VK_DEBUG_UTILS_MESSAGE_SEVERITY_WARNING_BIT_EXT != 0 {
        "warning"
    } else if severity & VK_DEBUG_UTILS_MESSAGE_SEVERITY_INFO_BIT_EXT != 0 {
        "info"
    } else if severity & VK_DEBUG_UTILS_MESSAGE_SEVERITY_VERBOSE_BIT_EXT != 0 {
        "verbose"
    } else {
        "unknown"
    }
}

fn debug_message_type_name(message_types: VkDebugUtilsMessageTypeFlagsEXT) -> &'static str {
    if message_types & VK_DEBUG_UTILS_MESSAGE_TYPE_VALIDATION_BIT_EXT != 0 {
        "validation"
    } else if message_types & VK_DEBUG_UTILS_MESSAGE_TYPE_PERFORMANCE_BIT_EXT != 0 {
        "performance"
    } else if message_types & VK_DEBUG_UTILS_MESSAGE_TYPE_GENERAL_BIT_EXT != 0 {
        "general"
    } else {
        "unknown"
    }
}

fn choose_surface_format(formats: &[VkSurfaceFormatKHR]) -> VkSurfaceFormatKHR {
    if formats.is_empty() {
        return VkSurfaceFormatKHR {
            format: VK_FORMAT_B8G8R8A8_UNORM,
            color_space: VK_COLOR_SPACE_SRGB_NONLINEAR_KHR,
        };
    }
    for format in formats {
        if format.format == VK_FORMAT_B8G8R8A8_UNORM
            && format.color_space == VK_COLOR_SPACE_SRGB_NONLINEAR_KHR
        {
            return *format;
        }
    }
    formats[0]
}

fn choose_present_mode(modes: &[VkPresentModeKHR]) -> VkPresentModeKHR {
    if modes
        .iter()
        .any(|mode| *mode == VK_PRESENT_MODE_MAILBOX_KHR)
    {
        VK_PRESENT_MODE_MAILBOX_KHR
    } else {
        VK_PRESENT_MODE_FIFO_KHR
    }
}

fn choose_extent(
    caps: &VkSurfaceCapabilitiesKHR,
    requested_width: u32,
    requested_height: u32,
) -> VkExtent2D {
    if caps.current_extent.width != u32::MAX {
        return caps.current_extent;
    }
    let width = requested_width
        .max(caps.min_image_extent.width)
        .min(caps.max_image_extent.width.max(caps.min_image_extent.width));
    let height = requested_height.max(caps.min_image_extent.height).min(
        caps.max_image_extent
            .height
            .max(caps.min_image_extent.height),
    );
    VkExtent2D { width, height }
}

fn choose_composite_alpha(flags: VkCompositeAlphaFlagsKHR) -> VkCompositeAlphaFlagsKHR {
    if flags & VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR != 0 {
        VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR
    } else {
        flags & (!flags + 1)
    }
}

fn query_surface_formats(
    get_surface_formats: PfnVkGetPhysicalDeviceSurfaceFormatsKHR,
    device: VkPhysicalDevice,
    surface: VkSurfaceKHR,
) -> Result<Vec<VkSurfaceFormatKHR>, (RenderResult, String)> {
    let mut count = 0u32;
    let rc = unsafe { get_surface_formats(device, surface, &mut count, ptr::null_mut()) };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkGetPhysicalDeviceSurfaceFormatsKHR(count)", rc));
    }
    if count == 0 {
        return Ok(Vec::new());
    }
    let mut formats = vec![
        VkSurfaceFormatKHR {
            format: VK_FORMAT_B8G8R8A8_UNORM,
            color_space: VK_COLOR_SPACE_SRGB_NONLINEAR_KHR,
        };
        count as usize
    ];
    let rc = unsafe { get_surface_formats(device, surface, &mut count, formats.as_mut_ptr()) };
    if rc != VK_SUCCESS {
        return Err(vk_error("vkGetPhysicalDeviceSurfaceFormatsKHR(list)", rc));
    }
    Ok(formats)
}

fn query_present_modes(
    get_present_modes: PfnVkGetPhysicalDeviceSurfacePresentModesKHR,
    device: VkPhysicalDevice,
    surface: VkSurfaceKHR,
) -> Result<Vec<VkPresentModeKHR>, (RenderResult, String)> {
    let mut count = 0u32;
    let rc = unsafe { get_present_modes(device, surface, &mut count, ptr::null_mut()) };
    if rc != VK_SUCCESS {
        return Err(vk_error(
            "vkGetPhysicalDeviceSurfacePresentModesKHR(count)",
            rc,
        ));
    }
    if count == 0 {
        return Ok(Vec::new());
    }
    let mut modes = vec![VK_PRESENT_MODE_FIFO_KHR; count as usize];
    let rc = unsafe { get_present_modes(device, surface, &mut count, modes.as_mut_ptr()) };
    if rc != VK_SUCCESS {
        return Err(vk_error(
            "vkGetPhysicalDeviceSurfacePresentModesKHR(list)",
            rc,
        ));
    }
    Ok(modes)
}
