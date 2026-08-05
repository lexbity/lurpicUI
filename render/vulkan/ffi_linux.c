#include <dlfcn.h>
#include <inttypes.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static void *lurpic_render_handle = NULL;

// GEN-FFI-START fn-declarations
static int32_t (*lurpic_render_create_image_fn)(const unsigned char *pixels, uintptr_t len, uint32_t width, uint32_t height, uint32_t stride, uint32_t format, uint64_t *out_handle) = NULL;
static int32_t (*lurpic_render_create_xcb_surface_fn)(uintptr_t instance, uintptr_t connection, uint32_t window, uint32_t width, uint32_t height, uintptr_t *out_surface) = NULL;
static int32_t (*lurpic_render_destroy_image_fn)(uint64_t handle) = NULL;
static uint64_t (*lurpic_render_device_generation_fn)(void) = NULL;
static int32_t (*lurpic_render_init_fn)(void) = NULL;
static uintptr_t (*lurpic_render_instance_handle_fn)(void) = NULL;
static const char * (*lurpic_render_last_error_fn)(void) = NULL;
static int32_t (*lurpic_render_query_capabilities_fn)(void *out) = NULL;
static int32_t (*lurpic_render_query_pipeline_features_fn)(void *out) = NULL;
static void (*lurpic_render_reset_atlas_fn)(void) = NULL;
static int32_t (*lurpic_render_resize_fn)(int32_t width, int32_t height) = NULL;
static int32_t (*lurpic_render_set_validation_fn)(uint32_t enabled) = NULL;
static int32_t (*lurpic_render_shutdown_fn)(void) = NULL;
static int32_t (*lurpic_render_submit_and_readback_fn)(const unsigned char *data, uintptr_t len, uint32_t width, uint32_t height, unsigned char *out_pixels, uintptr_t out_len) = NULL;
static int32_t (*lurpic_render_submit_frame_fn)(const unsigned char *data, uintptr_t len) = NULL;
static uint64_t (*lurpic_render_test_destroy_count_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_drop_count_fn)(void) = NULL;
static int32_t (*lurpic_render_test_error_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_glyph_atlas_count_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_glyph_atlas_evictions_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_handle_create_fn)(void) = NULL;
static int32_t (*lurpic_render_test_handle_destroy_fn)(uint64_t handle) = NULL;
static int32_t (*lurpic_render_test_handle_use_fn)(uint64_t handle) = NULL;
static int32_t (*lurpic_render_test_force_swapped_rendering_fn)(uint32_t enabled) = NULL;
static uint64_t (*lurpic_render_test_image_count_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_image_destroy_count_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_last_batch_count_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_last_command_count_fn)(void) = NULL;
static int32_t (*lurpic_render_test_ok_fn)(void) = NULL;
static int32_t (*lurpic_render_test_panic_fn)(void) = NULL;
static int32_t (*lurpic_render_test_reset_fn)(void) = NULL;
static uint64_t (*lurpic_render_test_validation_error_count_fn)(void) = NULL;
static int32_t (*lurpic_render_upload_glyph_fn)(uint64_t font_id, uint32_t glyph_id, uint32_t size_bits, uint32_t width, uint32_t height, float offset_x, float offset_y, float advance, const unsigned char *pixels, uintptr_t len) = NULL;
static const char * (*lurpic_render_version_fn)(void) = NULL;
// GEN-FFI-END fn-declarations
static void lurpic_render_reset_fns(void) {
// GEN-FFI-START unload
  lurpic_render_create_image_fn = NULL;
  lurpic_render_create_xcb_surface_fn = NULL;
  lurpic_render_destroy_image_fn = NULL;
  lurpic_render_device_generation_fn = NULL;
  lurpic_render_init_fn = NULL;
  lurpic_render_instance_handle_fn = NULL;
  lurpic_render_last_error_fn = NULL;
  lurpic_render_query_capabilities_fn = NULL;
  lurpic_render_query_pipeline_features_fn = NULL;
  lurpic_render_reset_atlas_fn = NULL;
  lurpic_render_resize_fn = NULL;
  lurpic_render_set_validation_fn = NULL;
  lurpic_render_shutdown_fn = NULL;
  lurpic_render_submit_and_readback_fn = NULL;
  lurpic_render_submit_frame_fn = NULL;
  lurpic_render_test_destroy_count_fn = NULL;
  lurpic_render_test_drop_count_fn = NULL;
  lurpic_render_test_error_fn = NULL;
  lurpic_render_test_glyph_atlas_count_fn = NULL;
  lurpic_render_test_glyph_atlas_evictions_fn = NULL;
  lurpic_render_test_handle_create_fn = NULL;
  lurpic_render_test_handle_destroy_fn = NULL;
  lurpic_render_test_handle_use_fn = NULL;
  lurpic_render_test_force_swapped_rendering_fn = NULL;
  lurpic_render_test_image_count_fn = NULL;
  lurpic_render_test_image_destroy_count_fn = NULL;
  lurpic_render_test_last_batch_count_fn = NULL;
  lurpic_render_test_last_command_count_fn = NULL;
  lurpic_render_test_ok_fn = NULL;
  lurpic_render_test_panic_fn = NULL;
  lurpic_render_test_reset_fn = NULL;
  lurpic_render_test_validation_error_count_fn = NULL;
  lurpic_render_upload_glyph_fn = NULL;
  lurpic_render_version_fn = NULL;
// GEN-FFI-END unload
}


static char lurpic_render_error[512];

static void set_error(const char *message) {
  if (message == NULL || message[0] == '\0') {
    lurpic_render_error[0] = '\0';
    return;
  }
  strncpy(lurpic_render_error, message, sizeof(lurpic_render_error) - 1);
  lurpic_render_error[sizeof(lurpic_render_error) - 1] = '\0';
}

#define LOAD_SYM(field, symbol_name, fn_type) \
  do { \
    void *symbol = dlsym(handle, symbol_name); \
    const char *sym_err = dlerror(); \
    if (sym_err != NULL || symbol == NULL) { \
      if (sym_err != NULL) { \
        set_error(sym_err); \
      } else { \
        snprintf(lurpic_render_error, sizeof(lurpic_render_error), "vulkan: missing symbol %s", symbol_name); \
      } \
      lurpic_render_reset_fns(); \
      dlclose(handle); \
      return -3; \
    } \
    field = (fn_type)symbol; \
  } while (0)

#define LOAD_SYM_OPTIONAL(field, symbol_name, fn_type) \
  do { \
    dlerror(); \
    void *symbol = dlsym(handle, symbol_name); \
    if (symbol != NULL) { \
      field = (fn_type)symbol; \
    } \
  } while (0)

int lurpic_render_load(const char *library_path) {
  if (lurpic_render_handle != NULL) {
    set_error("");
    return 0;
  }
  if (library_path == NULL || library_path[0] == '\0') {
    set_error("vulkan: library path is empty");
    return -1;
  }

  dlerror();
  void *handle = dlopen(library_path, RTLD_NOW | RTLD_LOCAL);
  if (handle == NULL) {
    const char *err = dlerror();
    set_error(err != NULL ? err : "vulkan: dlopen failed");
    return -2;
  }

  dlerror();
// GEN-FFI-START dlsym
  LOAD_SYM(lurpic_render_create_image_fn, "lurpic_render_create_image", int32_t(*)(const unsigned char *pixels, uintptr_t len, uint32_t width, uint32_t height, uint32_t stride, uint32_t format, uint64_t *out_handle));
  LOAD_SYM(lurpic_render_create_xcb_surface_fn, "lurpic_render_create_xcb_surface", int32_t(*)(uintptr_t instance, uintptr_t connection, uint32_t window, uint32_t width, uint32_t height, uintptr_t *out_surface));
  LOAD_SYM(lurpic_render_destroy_image_fn, "lurpic_render_destroy_image", int32_t(*)(uint64_t handle));
  LOAD_SYM(lurpic_render_device_generation_fn, "lurpic_render_device_generation", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_init_fn, "lurpic_render_init", int32_t(*)(void));
  LOAD_SYM(lurpic_render_instance_handle_fn, "lurpic_render_instance_handle", uintptr_t(*)(void));
  LOAD_SYM(lurpic_render_last_error_fn, "lurpic_render_last_error", const char *(*)(void));
  LOAD_SYM(lurpic_render_query_capabilities_fn, "lurpic_render_query_capabilities", int32_t(*)(void *out));
  LOAD_SYM(lurpic_render_query_pipeline_features_fn, "lurpic_render_query_pipeline_features", int32_t(*)(void *out));
  LOAD_SYM(lurpic_render_reset_atlas_fn, "lurpic_render_reset_atlas", void(*)(void));
  LOAD_SYM(lurpic_render_resize_fn, "lurpic_render_resize", int32_t(*)(int32_t width, int32_t height));
  LOAD_SYM(lurpic_render_set_validation_fn, "lurpic_render_set_validation", int32_t(*)(uint32_t enabled));
  LOAD_SYM(lurpic_render_shutdown_fn, "lurpic_render_shutdown", int32_t(*)(void));
  LOAD_SYM(lurpic_render_submit_and_readback_fn, "lurpic_render_submit_and_readback", int32_t(*)(const unsigned char *data, uintptr_t len, uint32_t width, uint32_t height, unsigned char *out_pixels, uintptr_t out_len));
  LOAD_SYM(lurpic_render_submit_frame_fn, "lurpic_render_submit_frame", int32_t(*)(const unsigned char *data, uintptr_t len));
  LOAD_SYM(lurpic_render_test_destroy_count_fn, "lurpic_render_test_destroy_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_drop_count_fn, "lurpic_render_test_drop_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_error_fn, "lurpic_render_test_error", int32_t(*)(void));
  LOAD_SYM(lurpic_render_test_glyph_atlas_count_fn, "lurpic_render_test_glyph_atlas_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_glyph_atlas_evictions_fn, "lurpic_render_test_glyph_atlas_evictions", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_handle_create_fn, "lurpic_render_test_handle_create", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_handle_destroy_fn, "lurpic_render_test_handle_destroy", int32_t(*)(uint64_t handle));
  LOAD_SYM(lurpic_render_test_handle_use_fn, "lurpic_render_test_handle_use", int32_t(*)(uint64_t handle));
  LOAD_SYM(lurpic_render_test_force_swapped_rendering_fn, "lurpic_render_test_force_swapped_rendering", int32_t(*)(uint32_t enabled));
  LOAD_SYM(lurpic_render_test_image_count_fn, "lurpic_render_test_image_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_image_destroy_count_fn, "lurpic_render_test_image_destroy_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_last_batch_count_fn, "lurpic_render_test_last_batch_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_last_command_count_fn, "lurpic_render_test_last_command_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_test_ok_fn, "lurpic_render_test_ok", int32_t(*)(void));
  LOAD_SYM(lurpic_render_test_panic_fn, "lurpic_render_test_panic", int32_t(*)(void));
  LOAD_SYM(lurpic_render_test_reset_fn, "lurpic_render_test_reset", int32_t(*)(void));
  LOAD_SYM(lurpic_render_test_validation_error_count_fn, "lurpic_render_test_validation_error_count", uint64_t(*)(void));
  LOAD_SYM(lurpic_render_upload_glyph_fn, "lurpic_render_upload_glyph", int32_t(*)(uint64_t font_id, uint32_t glyph_id, uint32_t size_bits, uint32_t width, uint32_t height, float offset_x, float offset_y, float advance, const unsigned char *pixels, uintptr_t len));
  LOAD_SYM(lurpic_render_version_fn, "lurpic_render_version", const char *(*)(void));
// GEN-FFI-END dlsym
#undef LOAD_SYM
#undef LOAD_SYM_OPTIONAL

  lurpic_render_handle = handle;
  set_error("");
  return 0;
}

// GEN-FFI-START wrappers
int32_t lurpic_render_create_image(const unsigned char *pixels, uintptr_t len, uint32_t width, uint32_t height, uint32_t stride, uint32_t format, uint64_t *out_handle) {
  if (lurpic_render_create_image_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_create_image_fn(pixels, len, width, height, stride, format, out_handle);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_create_xcb_surface(uintptr_t instance, uintptr_t connection, uint32_t window, uint32_t width, uint32_t height, uintptr_t *out_surface) {
  if (lurpic_render_create_xcb_surface_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_create_xcb_surface_fn(instance, connection, window, width, height, out_surface);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_destroy_image(uint64_t handle) {
  if (lurpic_render_destroy_image_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_destroy_image_fn(handle);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
uint64_t lurpic_render_device_generation(void) {
  if (lurpic_render_device_generation_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_device_generation_fn();
}
int32_t lurpic_render_init(void) {
  if (lurpic_render_init_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_init_fn();
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
uintptr_t lurpic_render_instance_handle(void) {
  if (lurpic_render_instance_handle_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_instance_handle_fn();
}
const char * lurpic_render_last_error(void) {
  if (lurpic_render_last_error_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return NULL;
  }
  set_error("");
  return lurpic_render_last_error_fn();
}
int32_t lurpic_render_query_capabilities(void *out) {
  if (lurpic_render_query_capabilities_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_query_capabilities_fn(out);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_query_pipeline_features(void *out) {
  if (lurpic_render_query_pipeline_features_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_query_pipeline_features_fn(out);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
void lurpic_render_reset_atlas(void) {
  if (lurpic_render_reset_atlas_fn != NULL) {
    lurpic_render_reset_atlas_fn();
  }
}
int32_t lurpic_render_resize(int32_t width, int32_t height) {
  if (lurpic_render_resize_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_resize_fn(width, height);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_set_validation(uint32_t enabled) {
  if (lurpic_render_set_validation_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_set_validation_fn(enabled);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_shutdown(void) {
  if (lurpic_render_shutdown_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_shutdown_fn();
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_submit_and_readback(const unsigned char *data, uintptr_t len, uint32_t width, uint32_t height, unsigned char *out_pixels, uintptr_t out_len) {
  if (lurpic_render_submit_and_readback_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_submit_and_readback_fn(data, len, width, height, out_pixels, out_len);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_submit_frame(const unsigned char *data, uintptr_t len) {
  if (lurpic_render_submit_frame_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_submit_frame_fn(data, len);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
uint64_t lurpic_render_test_destroy_count(void) {
  if (lurpic_render_test_destroy_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_destroy_count_fn();
}
uint64_t lurpic_render_test_drop_count(void) {
  if (lurpic_render_test_drop_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_drop_count_fn();
}
int32_t lurpic_render_test_error(void) {
  if (lurpic_render_test_error_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_error_fn();
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
uint64_t lurpic_render_test_glyph_atlas_count(void) {
  if (lurpic_render_test_glyph_atlas_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_glyph_atlas_count_fn();
}
uint64_t lurpic_render_test_glyph_atlas_evictions(void) {
  if (lurpic_render_test_glyph_atlas_evictions_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_glyph_atlas_evictions_fn();
}
uint64_t lurpic_render_test_handle_create(void) {
  if (lurpic_render_test_handle_create_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_handle_create_fn();
}
int32_t lurpic_render_test_handle_destroy(uint64_t handle) {
  if (lurpic_render_test_handle_destroy_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_handle_destroy_fn(handle);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_test_handle_use(uint64_t handle) {
  if (lurpic_render_test_handle_use_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_handle_use_fn(handle);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_test_force_swapped_rendering(uint32_t enabled) {
  if (lurpic_render_test_force_swapped_rendering_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_force_swapped_rendering_fn(enabled);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
uint64_t lurpic_render_test_image_count(void) {
  if (lurpic_render_test_image_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_image_count_fn();
}
uint64_t lurpic_render_test_image_destroy_count(void) {
  if (lurpic_render_test_image_destroy_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_image_destroy_count_fn();
}
uint64_t lurpic_render_test_last_batch_count(void) {
  if (lurpic_render_test_last_batch_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_last_batch_count_fn();
}
uint64_t lurpic_render_test_last_command_count(void) {
  if (lurpic_render_test_last_command_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_last_command_count_fn();
}
int32_t lurpic_render_test_ok(void) {
  if (lurpic_render_test_ok_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_ok_fn();
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_test_panic(void) {
  if (lurpic_render_test_panic_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_panic_fn();
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
int32_t lurpic_render_test_reset(void) {
  if (lurpic_render_test_reset_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_test_reset_fn();
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
uint64_t lurpic_render_test_validation_error_count(void) {
  if (lurpic_render_test_validation_error_count_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return 0;
  }
  set_error("");
  return lurpic_render_test_validation_error_count_fn();
}
int32_t lurpic_render_upload_glyph(uint64_t font_id, uint32_t glyph_id, uint32_t size_bits, uint32_t width, uint32_t height, float offset_x, float offset_y, float advance, const unsigned char *pixels, uintptr_t len) {
  if (lurpic_render_upload_glyph_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return -1;
  }
  int32_t result = lurpic_render_upload_glyph_fn(font_id, glyph_id, size_bits, width, height, offset_x, offset_y, advance, pixels, len);
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}
const char * lurpic_render_version(void) {
  if (lurpic_render_version_fn == NULL) {
    set_error("vulkan: Rust library not loaded");
    return NULL;
  }
  set_error("");
  return lurpic_render_version_fn();
}
// GEN-FFI-END wrappers
void lurpic_render_unload(void) {
  if (lurpic_render_handle != NULL) {
    dlclose(lurpic_render_handle);
  }
  lurpic_render_handle = NULL;
  lurpic_render_reset_fns();
  set_error("");
}
