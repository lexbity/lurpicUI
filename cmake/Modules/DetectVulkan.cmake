include_guard(GLOBAL)

function(lurpic_detect_vulkan)
  set(_lurpic_have_vulkan OFF)
  set(_lurpic_vulkan_root "")
  set(_lurpic_vulkan_include_dir "")
  set(_lurpic_vulkan_library "")
  set(_lurpic_vulkan_glslc "")
  set(_lurpic_vulkan_hint "Vulkan backend disabled")

  if(LURPIC_BACKEND_VULKAN)
    if(LURPIC_VULKAN_SDK_ROOT)
      set(_lurpic_vulkan_root "${LURPIC_VULKAN_SDK_ROOT}")
      if(NOT EXISTS "${_lurpic_vulkan_root}")
        message(FATAL_ERROR "LURPIC_VULKAN_SDK_ROOT does not exist: ${_lurpic_vulkan_root}")
      endif()
    elseif(DEFINED ENV{VULKAN_SDK} AND NOT "$ENV{VULKAN_SDK}" STREQUAL "")
      set(_lurpic_vulkan_root "$ENV{VULKAN_SDK}")
    endif()

    set(_lurpic_vulkan_include_candidates "")
    set(_lurpic_vulkan_library_candidates "")
    set(_lurpic_vulkan_glslc_candidates "")

    if(_lurpic_vulkan_root)
      list(APPEND _lurpic_vulkan_include_candidates
        "${_lurpic_vulkan_root}/include/vulkan/vulkan.h"
        "${_lurpic_vulkan_root}/Include/vulkan/vulkan.h"
      )
      list(APPEND _lurpic_vulkan_library_candidates
        "${_lurpic_vulkan_root}/lib/libvulkan.so"
        "${_lurpic_vulkan_root}/lib64/libvulkan.so"
        "${_lurpic_vulkan_root}/Lib/libvulkan.so"
      )
      list(APPEND _lurpic_vulkan_glslc_candidates
        "${_lurpic_vulkan_root}/bin/glslc"
        "${_lurpic_vulkan_root}/Bin/glslc"
      )
    endif()

    list(APPEND _lurpic_vulkan_include_candidates
      "/usr/include/vulkan/vulkan.h"
      "/usr/local/include/vulkan/vulkan.h"
      "/usr/include/x86_64-linux-gnu/vulkan/vulkan.h"
      "/usr/include/arm-linux-gnueabihf/vulkan/vulkan.h"
      "/usr/include/aarch64-linux-gnu/vulkan/vulkan.h"
    )
    list(APPEND _lurpic_vulkan_library_candidates
      "/usr/lib/libvulkan.so"
      "/usr/lib64/libvulkan.so"
      "/usr/lib/x86_64-linux-gnu/libvulkan.so"
      "/usr/lib/aarch64-linux-gnu/libvulkan.so"
    )

    foreach(_lurpic_vulkan_include_candidate IN LISTS _lurpic_vulkan_include_candidates)
      if(EXISTS "${_lurpic_vulkan_include_candidate}")
        get_filename_component(_lurpic_vulkan_include_dir "${_lurpic_vulkan_include_candidate}" DIRECTORY)
        break()
      endif()
    endforeach()

    foreach(_lurpic_vulkan_library_candidate IN LISTS _lurpic_vulkan_library_candidates)
      if(EXISTS "${_lurpic_vulkan_library_candidate}")
        set(_lurpic_vulkan_library "${_lurpic_vulkan_library_candidate}")
        break()
      endif()
    endforeach()

    foreach(_lurpic_vulkan_glslc_candidate IN LISTS _lurpic_vulkan_glslc_candidates)
      if(EXISTS "${_lurpic_vulkan_glslc_candidate}")
        set(_lurpic_vulkan_glslc "${_lurpic_vulkan_glslc_candidate}")
        break()
      endif()
    endforeach()

    if(_lurpic_vulkan_include_dir AND _lurpic_vulkan_library)
      set(_lurpic_have_vulkan ON)
      if(NOT _lurpic_vulkan_glslc)
        set(_lurpic_vulkan_hint "Vulkan headers and library were found; glslc was not found and is optional for the current backend stub.")
      else()
        set(_lurpic_vulkan_hint "Vulkan headers, library, and glslc were found.")
      endif()
    else()
      set(_lurpic_vulkan_hint "Vulkan backend requested but headers or library were not found. Install a Vulkan SDK or set LURPIC_VULKAN_SDK_ROOT to the SDK root.")
    endif()
  endif()

  set(LURPIC_HAVE_VULKAN "${_lurpic_have_vulkan}" PARENT_SCOPE)
  set(LURPIC_VULKAN_SDK_ROOT "${_lurpic_vulkan_root}" PARENT_SCOPE)
  set(LURPIC_VULKAN_INCLUDE_DIR "${_lurpic_vulkan_include_dir}" PARENT_SCOPE)
  set(LURPIC_VULKAN_LIBRARY "${_lurpic_vulkan_library}" PARENT_SCOPE)
  set(LURPIC_VULKAN_GLSLC "${_lurpic_vulkan_glslc}" PARENT_SCOPE)
  set(LURPIC_VULKAN_HINT "${_lurpic_vulkan_hint}" PARENT_SCOPE)

  set(LURPIC_HAVE_VULKAN "${_lurpic_have_vulkan}" CACHE BOOL "Whether Vulkan backend prerequisites are available" FORCE)
  set(LURPIC_VULKAN_SDK_ROOT "${_lurpic_vulkan_root}" CACHE PATH "Resolved Vulkan SDK root" FORCE)
  set(LURPIC_VULKAN_INCLUDE_DIR "${_lurpic_vulkan_include_dir}" CACHE PATH "Resolved Vulkan include directory" FORCE)
  set(LURPIC_VULKAN_LIBRARY "${_lurpic_vulkan_library}" CACHE FILEPATH "Resolved Vulkan library" FORCE)
  set(LURPIC_VULKAN_GLSLC "${_lurpic_vulkan_glslc}" CACHE FILEPATH "Resolved Vulkan shader compiler" FORCE)
  set(LURPIC_VULKAN_HINT "${_lurpic_vulkan_hint}" CACHE STRING "Vulkan capability hint" FORCE)
endfunction()
