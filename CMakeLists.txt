# LurpicUI keeps Go as the source of truth for package compilation, testing,
# vetting, and module resolution.
# CMake is the orchestration layer for configuration, presets, packaging, and
# convenience targets.
# This build system must not mutate tracked source files or attempt to
# reimplement go test semantics inside CMake.

cmake_minimum_required(VERSION 3.21)

project(lurpicui
  DESCRIPTION "Go-first UI runtime orchestrated by CMake"
  LANGUAGES NONE
)

list(APPEND CMAKE_MODULE_PATH "${CMAKE_CURRENT_SOURCE_DIR}/cmake/Modules")

set(LURPIC_BACKEND_SOFTWARE ON CACHE BOOL "Enable software renderer backend")
set(LURPIC_BACKEND_VULKAN OFF CACHE BOOL "Enable Vulkan renderer backend")
set(LURPIC_PLATFORM_LINUX ON CACHE BOOL "Enable Linux platform build/presets")
set(LURPIC_PLATFORM_ANDROID OFF CACHE BOOL "Enable Android platform build/presets")

set(LURPIC_BUILD_TESTS ON CACHE BOOL "Enable test targets")
set(LURPIC_BUILD_EXAMPLES ON CACHE BOOL "Build example applications")
set(LURPIC_BUILD_BENCHMARKS OFF CACHE BOOL "Enable benchmark targets")
set(LURPIC_BUILD_DOCS OFF CACHE BOOL "Enable docs generation targets")
set(LURPIC_INCLUDE_TEMPLATE_THEMES ON CACHE BOOL "Bundle template theme artifacts")
set(LURPIC_ENABLE_DEBUG_VALIDATION OFF CACHE BOOL "Enable debug-tag and validation runs")
set(LURPIC_UPDATE_GOLDENS OFF CACHE BOOL "Allow golden image update targets")
set(LURPIC_CI_MODE OFF CACHE BOOL "CI-optimized strict target set")
set(LURPIC_RELEASE_PACKAGE OFF CACHE BOOL "Enable packaging/install export")
set(LURPIC_VULKAN_SDK_ROOT "" CACHE PATH "Optional Vulkan SDK root used for backend validation")

function(_lurpic_reject_unknown_options)
  set(_lurpic_allowed_options
    LURPIC_BACKEND_SOFTWARE
    LURPIC_BACKEND_VULKAN
    LURPIC_PLATFORM_LINUX
    LURPIC_PLATFORM_ANDROID
    LURPIC_BUILD_TESTS
    LURPIC_BUILD_EXAMPLES
    LURPIC_BUILD_BENCHMARKS
    LURPIC_BUILD_DOCS
    LURPIC_INCLUDE_TEMPLATE_THEMES
    LURPIC_ENABLE_DEBUG_VALIDATION
    LURPIC_UPDATE_GOLDENS
    LURPIC_CI_MODE
    LURPIC_RELEASE_PACKAGE
    LURPIC_VULKAN_SDK_ROOT
    LURPIC_GO_EXECUTABLE
    LURPIC_GO_MINIMUM_VERSION
    LURPIC_GO_VERSION
    LURPIC_GO_VERSION_SHORT
    LURPIC_GO_VERSION_MINIMUM
    LURPIC_GO_GOMOD
    LURPIC_GO_GOROOT
    LURPIC_GO_GOPATH
    LURPIC_GO_CGO_ENABLED
    LURPIC_HAVE_CGO
    LURPIC_CC_EXECUTABLE
    LURPIC_HAVE_RACE
    LURPIC_RACE_HINT
    LURPIC_LINUX_PLATFORM_ENABLED
    LURPIC_VULKAN_SDK_ROOT
    LURPIC_HAVE_VULKAN
    LURPIC_VULKAN_INCLUDE_DIR
    LURPIC_VULKAN_LIBRARY
    LURPIC_VULKAN_GLSLC
    LURPIC_VULKAN_HINT
    LURPIC_PACKAGE_VERSION
    LURPIC_VCS_REVISION
    LURPIC_BUILD_TIMESTAMP
  )

  get_cmake_property(_lurpic_cache_vars CACHE_VARIABLES)
  set(_lurpic_unknown_options "")
  foreach(_lurpic_cache_var IN LISTS _lurpic_cache_vars)
    if(_lurpic_cache_var MATCHES "^LURPIC_" AND NOT _lurpic_cache_var IN_LIST _lurpic_allowed_options)
      list(APPEND _lurpic_unknown_options "${_lurpic_cache_var}")
    endif()
  endforeach()

  if(_lurpic_unknown_options)
    list(JOIN _lurpic_unknown_options ", " _lurpic_unknown_joined)
    message(FATAL_ERROR "Configuration failure: unknown LurpicUI CMake options: ${_lurpic_unknown_joined}")
  endif()
endfunction()

_lurpic_reject_unknown_options()

include(LurpicSummary)

include(FindGoToolchain)
lurpic_find_go_toolchain()

include(DetectCGO)
lurpic_detect_cgo()

include(DetectRace)
lurpic_detect_race()

include(DetectVulkan)
lurpic_detect_vulkan()

if(NOT LURPIC_BACKEND_SOFTWARE AND NOT LURPIC_BACKEND_VULKAN)
  message(FATAL_ERROR "At least one backend must be enabled. Set LURPIC_BACKEND_SOFTWARE=ON and/or LURPIC_BACKEND_VULKAN=ON.")
endif()
if(LURPIC_BACKEND_VULKAN AND NOT LURPIC_HAVE_VULKAN)
  message(FATAL_ERROR "Configuration failure: Vulkan backend was enabled but Vulkan prerequisites were not found. Set LURPIC_VULKAN_SDK_ROOT to a valid SDK root or install Vulkan headers and libraries.")
endif()

include(GoCommandWrappers)
lurpic_add_go_build_target(
  NAME build-go
  PACKAGES ./...
)
lurpic_add_go_vet_target(
  NAME vet
  PACKAGES ./...
)
lurpic_add_go_test_target(
  NAME test
  PACKAGES ./...
)
if(LURPIC_BACKEND_SOFTWARE)
  lurpic_add_go_build_target(
    NAME build-backend-software
    PACKAGES ./render/software/...
    DESCRIPTION "Go build software backend packages"
  )
endif()
if(LURPIC_BACKEND_VULKAN)
  lurpic_add_go_build_target(
    NAME build-backend-vulkan
    PACKAGES ./render/vulkan/...
    DESCRIPTION "Go build Vulkan backend packages"
  )
endif()

function(_lurpic_add_test_suite_target)
  set(options)
  set(one_value_args NAME DESCRIPTION AVAILABLE UNAVAILABLE_MESSAGE)
  set(multi_value_args PACKAGES)
  cmake_parse_arguments(LURPIC "${options}" "${one_value_args}" "${multi_value_args}" ${ARGN})

  if(NOT LURPIC_NAME)
    message(FATAL_ERROR "_lurpic_add_test_suite_target requires NAME")
  endif()
  if(NOT LURPIC_DESCRIPTION)
    set(LURPIC_DESCRIPTION "Go test target")
  endif()

  lurpic_add_optional_go_test_target(
    NAME "${LURPIC_NAME}"
    DESCRIPTION "${LURPIC_DESCRIPTION}"
    AVAILABLE ${LURPIC_AVAILABLE}
    UNAVAILABLE_MESSAGE "${LURPIC_UNAVAILABLE_MESSAGE}"
    PACKAGES ${LURPIC_PACKAGES}
  )
endfunction()

set(_lurpic_test_all_targets "")

_lurpic_add_test_suite_target(
  NAME test-pure
  DESCRIPTION "Go test pure package suite"
  AVAILABLE ON
  PACKAGES
    ./gfx/...
    ./signal/...
    ./store/...
    ./job/...
    ./layout/...
    ./text/...
    ./theme/...
    ./facet/...
    ./runtime/...
    ./graph/index/...
    ./internal/hashutil/...
    ./internal/renderutil/...
    ./render/vulkan/...
    ./marks
    ./marks/basic/...
    ./marks/structure/...
    ./marks/uiinput/...
    ./marks/uinav/...
    ./marks/uinotification/...
    ./marks/annotation/...
    ./marks/chart/...
)
list(APPEND _lurpic_test_all_targets test-pure)

_lurpic_add_test_suite_target(
  NAME test-headless
  DESCRIPTION "Go test headless harness suite"
  AVAILABLE ON
  PACKAGES
    ./internal/testkit/...
    ./projection/...
    ./diagnostics/...
    ./graph/canvas/...
)
list(APPEND _lurpic_test_all_targets test-headless)

_lurpic_add_test_suite_target(
  NAME test-render-software
  DESCRIPTION "Go test render/software suite"
  AVAILABLE ${LURPIC_BACKEND_SOFTWARE}
  UNAVAILABLE_MESSAGE "render/software tests are unavailable because LURPIC_BACKEND_SOFTWARE is OFF."
  PACKAGES ./render/software/...
)
if(LURPIC_BACKEND_SOFTWARE)
  list(APPEND _lurpic_test_all_targets test-render-software)
endif()

_lurpic_add_test_suite_target(
  NAME test-integration
  DESCRIPTION "Go test integration suite"
  AVAILABLE ON
  PACKAGES ./marks/integration/...
)
list(APPEND _lurpic_test_all_targets test-integration)

add_custom_target(test-all
  DEPENDS ${_lurpic_test_all_targets}
  COMMENT "Aggregate Go test suite for the current configuration"
  VERBATIM
)

set(_lurpic_nonplatform_test_packages
  ./gfx/...
  ./signal/...
  ./store/...
  ./job/...
  ./layout/...
  ./text/...
  ./theme/...
  ./facet/...
  ./runtime/...
  ./graph/index/...
  ./graph/canvas/...
  ./internal/hashutil/...
  ./internal/renderutil/...
  ./internal/testkit/...
  ./projection/...
  ./diagnostics/...
  ./render/software/...
  ./render/vulkan/...
  ./marks/...
)

set(_lurpic_debug_test_packages
  ./internal/syncutil/...
  ./runtime/...
)

set(_lurpic_benchmark_packages
  ./layout/...
  ./graph/index/...
  ./graph/canvas/...
  ./marks/chart/...
  ./render/software/...
  ./internal/renderutil/...
  ./theme/...
)

set(LURPIC_BUNDLE_STAGE_ROOT "${CMAKE_CURRENT_BINARY_DIR}/staging")
set(LURPIC_BUNDLE_EXAMPLES_DIR "${LURPIC_BUNDLE_STAGE_ROOT}/examples")
set(LURPIC_BUNDLE_THEMES_DIR "${LURPIC_BUNDLE_STAGE_ROOT}/themes")
set(LURPIC_BUNDLE_DOCS_DIR "${LURPIC_BUNDLE_STAGE_ROOT}/docs")
set(LURPIC_BUNDLE_PRESETS_DIR "${LURPIC_BUNDLE_STAGE_ROOT}/presets")

file(MAKE_DIRECTORY
  "${LURPIC_BUNDLE_STAGE_ROOT}"
  "${LURPIC_BUNDLE_EXAMPLES_DIR}"
  "${LURPIC_BUNDLE_THEMES_DIR}"
  "${LURPIC_BUNDLE_DOCS_DIR}"
  "${LURPIC_BUNDLE_PRESETS_DIR}"
)

configure_file(
  "${CMAKE_CURRENT_SOURCE_DIR}/cmake/Scripts/BundleLayout.txt.in"
  "${LURPIC_BUNDLE_DOCS_DIR}/bundle-layout.txt"
  @ONLY
)
configure_file(
  "${CMAKE_CURRENT_SOURCE_DIR}/cmake/Scripts/BundleSummary.txt.in"
  "${LURPIC_BUNDLE_PRESETS_DIR}/config-summary.txt"
  @ONLY
)

set(_lurpic_build_examples_commands
  COMMAND ${CMAKE_COMMAND} -E make_directory
    "${LURPIC_BUNDLE_STAGE_ROOT}"
    "${LURPIC_BUNDLE_EXAMPLES_DIR}"
    "${LURPIC_BUNDLE_THEMES_DIR}"
    "${LURPIC_BUNDLE_DOCS_DIR}"
    "${LURPIC_BUNDLE_PRESETS_DIR}"
)
if(LURPIC_INCLUDE_TEMPLATE_THEMES)
  list(APPEND _lurpic_build_examples_commands
    COMMAND ${CMAKE_COMMAND} -E touch "${LURPIC_BUNDLE_THEMES_DIR}/template-themes.enabled"
  )
endif()
if(LURPIC_BUILD_EXAMPLES)
  foreach(_lurpic_example_name IN ITEMS basic ui chart graph)
    set(_lurpic_example_dir "${CMAKE_CURRENT_SOURCE_DIR}/examples/${_lurpic_example_name}")
    if(EXISTS "${_lurpic_example_dir}/main.go")
      list(APPEND _lurpic_build_examples_commands
        COMMAND ${CMAKE_COMMAND} -E env ${LURPIC_GO_EXECUTABLE} build ./examples/${_lurpic_example_name}
      )
      add_custom_target("run-example-${_lurpic_example_name}"
        COMMAND ${CMAKE_COMMAND} -E env ${LURPIC_GO_EXECUTABLE} run ./examples/${_lurpic_example_name}
        WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
        COMMENT "Run example ${_lurpic_example_name}"
        VERBATIM
      )
    endif()
  endforeach()
endif()

add_custom_target(build-examples
  ${_lurpic_build_examples_commands}
  COMMENT "Stage optional examples and template assets"
  VERBATIM
)

lurpic_add_optional_go_test_target(
  NAME test-race
  DESCRIPTION "Go test race detector suite"
  AVAILABLE ${LURPIC_HAVE_RACE}
  UNAVAILABLE_MESSAGE "Environment capability failure: test-race is unavailable because the Go race detector is not supported by this toolchain: ${LURPIC_RACE_HINT}"
  PACKAGES ${_lurpic_nonplatform_test_packages}
  ARGS -race -count=1
)

lurpic_add_go_test_target(
  NAME test-debug
  DESCRIPTION "Go test lurpic_debug suite"
  PACKAGES ${_lurpic_debug_test_packages}
  ARGS -tags lurpic_debug -count=1
)

lurpic_add_go_test_target(
  NAME bench
  DESCRIPTION "Go benchmark aggregate suite"
  PACKAGES ${_lurpic_benchmark_packages}
  ARGS -run ^$ -bench . -benchmem -count=1
)

lurpic_add_go_test_target(
  NAME bench-graph
  DESCRIPTION "Go benchmark graph suite"
  PACKAGES
    ./graph/index/...
    ./graph/canvas/...
    ./marks/chart/...
  ARGS -run ^$ -bench . -benchmem -count=1
)

lurpic_add_go_test_target(
  NAME bench-render
  DESCRIPTION "Go benchmark render suite"
  PACKAGES
    ./render/software/...
    ./internal/renderutil/...
  ARGS -run ^$ -bench . -benchmem -count=1
)

lurpic_add_go_test_target(
  NAME golden-verify
  DESCRIPTION "Go golden verification suite"
  PACKAGES ./internal/testkit/...
)

if(LURPIC_UPDATE_GOLDENS)
  lurpic_add_go_test_target(
    NAME golden-update
    DESCRIPTION "Go golden update suite"
    PACKAGES ./internal/testkit/...
    ARGS -update-golden
  )
endif()

include(LurpicPackaging)
lurpic_setup_packaging()

lurpic_emit_configuration_summary()
lurpic_add_configuration_summary_target()
