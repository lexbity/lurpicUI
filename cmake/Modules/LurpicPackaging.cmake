include_guard(GLOBAL)

if(NOT DEFINED CMAKE_INSTALL_LIBDIR)
  set(CMAKE_INSTALL_LIBDIR "lib" CACHE STRING "Library install directory" FORCE)
endif()
if(NOT DEFINED CMAKE_INSTALL_DATADIR)
  set(CMAKE_INSTALL_DATADIR "share" CACHE STRING "Data install directory" FORCE)
endif()
include(GNUInstallDirs)

function(lurpic_setup_packaging)
  set(_lurpic_script_dir "${CMAKE_CURRENT_FUNCTION_LIST_DIR}/../Scripts")
  set(_lurpic_package_version "unknown")
  set(_lurpic_vcs_revision "unknown")
  set(_lurpic_build_timestamp "")

  execute_process(
    COMMAND git describe --always --dirty --tags
    WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
    OUTPUT_VARIABLE _lurpic_git_describe
    ERROR_QUIET
    OUTPUT_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE _lurpic_git_describe_result
  )
  if(_lurpic_git_describe_result EQUAL 0 AND NOT _lurpic_git_describe STREQUAL "")
    set(_lurpic_vcs_revision "${_lurpic_git_describe}")
    set(_lurpic_package_version "${_lurpic_git_describe}")
  else()
    execute_process(
      COMMAND git rev-parse --short=12 HEAD
      WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
      OUTPUT_VARIABLE _lurpic_git_rev
      ERROR_QUIET
      OUTPUT_STRIP_TRAILING_WHITESPACE
      RESULT_VARIABLE _lurpic_git_rev_result
    )
    if(_lurpic_git_rev_result EQUAL 0 AND NOT _lurpic_git_rev STREQUAL "")
      set(_lurpic_vcs_revision "${_lurpic_git_rev}")
    endif()
  endif()

  string(TIMESTAMP _lurpic_build_timestamp "%Y-%m-%dT%H:%M:%SZ" UTC)

  set(LURPIC_PACKAGE_VERSION "${_lurpic_package_version}" PARENT_SCOPE)
  set(LURPIC_VCS_REVISION "${_lurpic_vcs_revision}" PARENT_SCOPE)
  set(LURPIC_BUILD_TIMESTAMP "${_lurpic_build_timestamp}" PARENT_SCOPE)
  set(LURPIC_PACKAGE_VERSION "${_lurpic_package_version}" CACHE STRING "LurpicUI package version" FORCE)
  set(LURPIC_VCS_REVISION "${_lurpic_vcs_revision}" CACHE STRING "LurpicUI VCS revision" FORCE)
  set(LURPIC_BUILD_TIMESTAMP "${_lurpic_build_timestamp}" CACHE STRING "LurpicUI build timestamp" FORCE)

  configure_file(
    "${_lurpic_script_dir}/LurpicConfig.cmake.in"
    "${CMAKE_CURRENT_BINARY_DIR}/LurpicConfig.cmake"
    @ONLY
  )
  configure_file(
    "${_lurpic_script_dir}/LurpicTargets.cmake.in"
    "${CMAKE_CURRENT_BINARY_DIR}/LurpicTargets.cmake"
    @ONLY
  )
  configure_file(
    "${_lurpic_script_dir}/BuildSummary.txt.in"
    "${CMAKE_CURRENT_BINARY_DIR}/lurpic-build-summary.txt"
    @ONLY
  )

  install(FILES
    "${CMAKE_CURRENT_BINARY_DIR}/LurpicConfig.cmake"
    RENAME "lurpicConfig.cmake"
    DESTINATION "${CMAKE_INSTALL_LIBDIR}/cmake/lurpic"
    COMPONENT metadata
  )
  install(FILES
    "${CMAKE_CURRENT_BINARY_DIR}/LurpicTargets.cmake"
    RENAME "lurpicTargets.cmake"
    DESTINATION "${CMAKE_INSTALL_LIBDIR}/cmake/lurpic"
    COMPONENT metadata
  )
  install(FILES
    "${CMAKE_CURRENT_BINARY_DIR}/lurpic-build-summary.txt"
    DESTINATION "${CMAKE_INSTALL_DATADIR}/lurpic"
    COMPONENT metadata
  )
  install(FILES
    "${CMAKE_CURRENT_SOURCE_DIR}/CMakePresets.json"
    DESTINATION "${CMAKE_INSTALL_DATADIR}/lurpic/presets"
    COMPONENT metadata
  )
  install(FILES
    "${CMAKE_CURRENT_SOURCE_DIR}/docs/cmake-presets.md"
    DESTINATION "${CMAKE_INSTALL_DATADIR}/lurpic/docs"
    COMPONENT docs
  )
  if(LURPIC_RELEASE_PACKAGE)
    install(DIRECTORY
      "${LURPIC_BUNDLE_STAGE_ROOT}/"
      DESTINATION "${CMAKE_INSTALL_DATADIR}/lurpic"
      COMPONENT bundle
    )
  endif()

  set(CPACK_PACKAGE_NAME "lurpicui")
  set(CPACK_PACKAGE_VENDOR "LurpicUI")
  set(CPACK_PACKAGE_CONTACT "LurpicUI Maintainers")
  set(CPACK_PACKAGE_VERSION "${_lurpic_package_version}")
  set(CPACK_PACKAGE_FILE_NAME "lurpicui-${_lurpic_package_version}")
  set(CPACK_GENERATOR "TGZ")
  set(CPACK_INCLUDE_TOPLEVEL_DIRECTORY ON)
  set(CPACK_ARCHIVE_COMPONENT_INSTALL ON)
  set(_lurpic_cpack_components metadata docs)
  if(LURPIC_RELEASE_PACKAGE)
    list(APPEND _lurpic_cpack_components bundle)
  endif()
  set(CPACK_COMPONENTS_ALL ${_lurpic_cpack_components})
  include(CPack)
endfunction()
