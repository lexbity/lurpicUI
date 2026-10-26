//! ash-based Vulkan integration layer.
//!
//! This module isolates `ash` behind the `GpuContext` trait (FR-15) so a future
//! swap to a different Vulkan binding is bounded to re-implementing this
//! module. It owns entry loading (Q14), validation (Q15), surface creation
//! (Q16), RAII resource guards (Q10), memory allocation (Q16/FR-16), and
//! resource tracking (P9).

pub mod allocator;
pub mod context;
pub mod entry;
pub mod resource_tracker;
pub mod resources;
pub mod surface;
pub mod validation;
