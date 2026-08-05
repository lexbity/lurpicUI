//! `VkPipelineCache` wrapper with optional disk persistence.
//!
//! Retires the created-and-never-used pipeline-cache debt: the cache is built
//! once, fed to every pipeline build, and persisted to `LURPIC_PIPELINE_CACHE_DIR`
//! so shader compilation state survives process restarts.

use std::path::Path;

use ash::vk;

use crate::error::vk_error;
use crate::RenderResult;

pub struct PipelineCache {
    device: ash::Device,
    handle: vk::PipelineCache,
    save_path: Option<std::path::PathBuf>,
}

impl PipelineCache {
    /// Creates a pipeline cache, seeding it from `path` if present. `save_path`
    /// is where `save()` writes the cache data.
    pub fn new(
        device: &ash::Device,
        seed_path: Option<&Path>,
        save_path: Option<&Path>,
    ) -> Result<Self, (RenderResult, String)> {
        let (initial_data, initial_size): (Vec<u8>, usize) = match seed_path {
            Some(path) => match std::fs::read(path) {
                Ok(data) => {
                    let len = data.len();
                    (data, len)
                }
                Err(_) => (Vec::new(), 0),
            },
            None => (Vec::new(), 0),
        };
        let info = vk::PipelineCacheCreateInfo {
            initial_data_size: initial_size,
            p_initial_data: if initial_data.is_empty() {
                std::ptr::null()
            } else {
                initial_data.as_ptr().cast()
            },
            ..Default::default()
        };
        let handle = unsafe { device.create_pipeline_cache(&info, None) }
            .map_err(|e| vk_error("vkCreatePipelineCache", e.as_raw()))?;
        Ok(Self {
            device: device.clone(),
            handle,
            save_path: save_path.map(|p| p.to_path_buf()),
        })
    }

    pub fn handle(&self) -> vk::PipelineCache {
        self.handle
    }

    /// Persists the cache data to disk. Failures are non-fatal (caching is an
    /// optimization); the caller decides whether to propagate.
    pub fn save(&self) -> Result<(), (RenderResult, String)> {
        let Some(path) = &self.save_path else {
            return Ok(());
        };
        let data = unsafe { self.device.get_pipeline_cache_data(self.handle) }
            .map_err(|e| vk_error("vkGetPipelineCacheData", e.as_raw()))?;
        std::fs::create_dir_all(path.parent().unwrap_or(Path::new(".")))
            .map_err(|e| (RenderResult::InitFailed, format!("pipeline cache dir: {}", e)))?;
        std::fs::write(path, data)
            .map_err(|e| (RenderResult::InitFailed, format!("pipeline cache write: {}", e)))
    }
}

impl Drop for PipelineCache {
    fn drop(&mut self) {
        unsafe {
            self.device.destroy_pipeline_cache(self.handle, None);
        }
    }
}

/// Resolves the pipeline cache directory from `LURPIC_PIPELINE_CACHE_DIR`, or
/// `None` when unset (caching disabled).
pub fn cache_dir() -> Option<std::path::PathBuf> {
    std::env::var("LURPIC_PIPELINE_CACHE_DIR")
        .ok()
        .filter(|s| !s.is_empty())
        .map(std::path::PathBuf::from)
}

/// The cache file path inside the cache directory.
pub fn cache_path() -> Option<std::path::PathBuf> {
    cache_dir().map(|dir| dir.join("lurpic_render_pipeline.cache"))
}
