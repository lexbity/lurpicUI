include_guard(GLOBAL)

function(lurpic_detect_cgo)
  set(_lurpic_have_cgo OFF)
  set(_lurpic_cc_executable "")

  if(DEFINED LURPIC_GO_CGO_ENABLED AND LURPIC_GO_CGO_ENABLED STREQUAL "1")
    find_program(_lurpic_cc_executable NAMES cc clang gcc PATHS /usr/bin /usr/local/bin /opt/homebrew/bin)
    if(NOT _lurpic_cc_executable)
      execute_process(
        COMMAND /bin/sh -lc "command -v cc || command -v clang || command -v gcc"
        OUTPUT_VARIABLE _lurpic_cc_command_output
        OUTPUT_STRIP_TRAILING_WHITESPACE
        ERROR_STRIP_TRAILING_WHITESPACE
        RESULT_VARIABLE _lurpic_cc_command_result
      )
      if(_lurpic_cc_command_result EQUAL 0 AND NOT _lurpic_cc_command_output STREQUAL "")
        set(_lurpic_cc_executable "${_lurpic_cc_command_output}")
      endif()
    endif()
    if(_lurpic_cc_executable)
      set(_lurpic_have_cgo ON)
    endif()
  endif()

  set(LURPIC_HAVE_CGO "${_lurpic_have_cgo}" PARENT_SCOPE)
  set(LURPIC_CC_EXECUTABLE "${_lurpic_cc_executable}" PARENT_SCOPE)
  set(LURPIC_HAVE_CGO "${_lurpic_have_cgo}" CACHE BOOL "Whether cgo is usable for this configuration" FORCE)
  set(LURPIC_CC_EXECUTABLE "${_lurpic_cc_executable}" CACHE FILEPATH "C compiler used for cgo validation" FORCE)

  if(LURPIC_PLATFORM_LINUX AND NOT LURPIC_HAVE_CGO)
    message(WARNING "Linux platform is enabled but cgo is unavailable. platform/linux targets may be unavailable until a C compiler is present.")
  endif()
endfunction()
