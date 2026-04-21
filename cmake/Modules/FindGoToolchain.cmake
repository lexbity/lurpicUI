include_guard(GLOBAL)

function(_lurpic_parse_go_module_version out_var)
  set(_go_mod_path "${CMAKE_CURRENT_SOURCE_DIR}/go.mod")
  if(NOT EXISTS "${_go_mod_path}")
    if(DEFINED _lurpic_go_minimum_fallback AND NOT _lurpic_go_minimum_fallback STREQUAL "")
      set(${out_var} "${_lurpic_go_minimum_fallback}" PARENT_SCOPE)
    else()
      set(${out_var} "1.22" PARENT_SCOPE)
    endif()
    return()
  endif()

  file(STRINGS "${_go_mod_path}" _lurpic_go_mod_lines REGEX "^go [0-9]+\\.[0-9]+(\\.[0-9]+)?$")
  if(_lurpic_go_mod_lines)
    list(GET _lurpic_go_mod_lines 0 _lurpic_go_mod_line)
    string(REGEX REPLACE "^go ([0-9]+\\.[0-9]+(\\.[0-9]+)?)$" "\\1" _lurpic_go_mod_version "${_lurpic_go_mod_line}")
    if(NOT _lurpic_go_mod_version STREQUAL "")
      set(${out_var} "${_lurpic_go_mod_version}" PARENT_SCOPE)
      return()
    endif()
  endif()

  if(DEFINED _lurpic_go_minimum_fallback AND NOT _lurpic_go_minimum_fallback STREQUAL "")
    set(${out_var} "${_lurpic_go_minimum_fallback}" PARENT_SCOPE)
  else()
    set(${out_var} "1.22" PARENT_SCOPE)
  endif()
endfunction()

function(_lurpic_trim_go_version out_var raw_value)
  string(REGEX MATCH "go([0-9]+\\.[0-9]+(\\.[0-9]+)?)" _lurpic_go_version_match "${raw_value}")
  if(CMAKE_MATCH_1)
    set(${out_var} "${CMAKE_MATCH_1}" PARENT_SCOPE)
  else()
    set(${out_var} "" PARENT_SCOPE)
  endif()
endfunction()

set(_lurpic_go_minimum_fallback "1.22")
_lurpic_parse_go_module_version(_lurpic_go_module_version)
if(NOT DEFINED LURPIC_GO_MINIMUM_VERSION OR LURPIC_GO_MINIMUM_VERSION STREQUAL "")
  set(LURPIC_GO_MINIMUM_VERSION "${_lurpic_go_module_version}" CACHE STRING "Minimum supported Go version" FORCE)
else()
  set(LURPIC_GO_MINIMUM_VERSION "${LURPIC_GO_MINIMUM_VERSION}" CACHE STRING "Minimum supported Go version" FORCE)
endif()

function(lurpic_find_go_toolchain)
  set(_lurpic_go_executable "")
  if(DEFINED LURPIC_GO_EXECUTABLE AND NOT LURPIC_GO_EXECUTABLE STREQUAL "" AND NOT LURPIC_GO_EXECUTABLE MATCHES "-NOTFOUND$")
    set(_lurpic_go_executable "${LURPIC_GO_EXECUTABLE}")
    if(NOT EXISTS "${_lurpic_go_executable}")
      message(FATAL_ERROR "Requested Go executable does not exist: ${_lurpic_go_executable}")
    endif()
  else()
    find_program(_lurpic_go_executable NAMES go PATHS /usr/bin /usr/local/bin /opt/homebrew/bin)
    if(NOT _lurpic_go_executable)
      execute_process(
        COMMAND /bin/sh -lc "command -v go"
        OUTPUT_VARIABLE _lurpic_go_command_output
        OUTPUT_STRIP_TRAILING_WHITESPACE
        ERROR_STRIP_TRAILING_WHITESPACE
        RESULT_VARIABLE _lurpic_go_command_result
      )
      if(_lurpic_go_command_result EQUAL 0 AND NOT _lurpic_go_command_output STREQUAL "")
        set(_lurpic_go_executable "${_lurpic_go_command_output}")
      endif()
    endif()
    if(NOT _lurpic_go_executable)
      message(FATAL_ERROR "Go toolchain not found. Install Go ${LURPIC_GO_MINIMUM_VERSION} or newer and ensure 'go' is on PATH.")
    endif()
  endif()

  execute_process(
    COMMAND "${_lurpic_go_executable}" version
    WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
    OUTPUT_VARIABLE _lurpic_go_version_output
    OUTPUT_STRIP_TRAILING_WHITESPACE
    ERROR_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE _lurpic_go_version_result
  )
  if(NOT _lurpic_go_version_result EQUAL 0)
    message(FATAL_ERROR "Failed to run '${_lurpic_go_executable} version'.")
  endif()

  _lurpic_trim_go_version(LURPIC_GO_VERSION_SHORT "${_lurpic_go_version_output}")
  if(LURPIC_GO_VERSION_SHORT STREQUAL "")
    message(FATAL_ERROR "Unable to parse Go version from output: ${_lurpic_go_version_output}")
  endif()

  if(LURPIC_GO_VERSION_SHORT VERSION_LESS LURPIC_GO_MINIMUM_VERSION)
    message(FATAL_ERROR "Go ${LURPIC_GO_MINIMUM_VERSION} or newer is required, but found ${LURPIC_GO_VERSION_SHORT} (${_lurpic_go_version_output}).")
  endif()

  execute_process(
    COMMAND "${_lurpic_go_executable}" env GOMOD GOROOT GOPATH CGO_ENABLED
    WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
    OUTPUT_VARIABLE _lurpic_go_env_output
    OUTPUT_STRIP_TRAILING_WHITESPACE
    ERROR_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE _lurpic_go_env_result
  )
  if(NOT _lurpic_go_env_result EQUAL 0)
    message(FATAL_ERROR "Failed to query Go environment from '${_lurpic_go_executable}'.")
  endif()

  string(REPLACE "\r\n" "\n" _lurpic_go_env_output "${_lurpic_go_env_output}")
  string(REPLACE "\r" "\n" _lurpic_go_env_output "${_lurpic_go_env_output}")
  string(REGEX REPLACE "\n+$" "" _lurpic_go_env_output "${_lurpic_go_env_output}")
  string(REPLACE "\n" ";" _lurpic_go_env_lines "${_lurpic_go_env_output}")
  list(LENGTH _lurpic_go_env_lines _lurpic_go_env_count)
  if(NOT _lurpic_go_env_count EQUAL 4)
    message(FATAL_ERROR "Unexpected Go env output from '${LURPIC_GO_EXECUTABLE} env GOMOD GOROOT GOPATH CGO_ENABLED'.")
  endif()

  list(GET _lurpic_go_env_lines 0 LURPIC_GO_GOMOD)
  list(GET _lurpic_go_env_lines 1 LURPIC_GO_GOROOT)
  list(GET _lurpic_go_env_lines 2 LURPIC_GO_GOPATH)
  list(GET _lurpic_go_env_lines 3 LURPIC_GO_CGO_ENABLED)

  set(LURPIC_GO_VERSION "${_lurpic_go_version_output}")
  set(LURPIC_GO_VERSION_SHORT "${LURPIC_GO_VERSION_SHORT}")
  set(LURPIC_GO_VERSION_MINIMUM "${LURPIC_GO_MINIMUM_VERSION}")
  set(LURPIC_GO_EXECUTABLE "${_lurpic_go_executable}")
  set(LURPIC_GO_GOMOD "${LURPIC_GO_GOMOD}")
  set(LURPIC_GO_GOROOT "${LURPIC_GO_GOROOT}")
  set(LURPIC_GO_GOPATH "${LURPIC_GO_GOPATH}")
  set(LURPIC_GO_CGO_ENABLED "${LURPIC_GO_CGO_ENABLED}")

  set(LURPIC_GO_EXECUTABLE "${LURPIC_GO_EXECUTABLE}" CACHE FILEPATH "Go executable used by CMake" FORCE)
  set(LURPIC_GO_VERSION "${LURPIC_GO_VERSION}" CACHE STRING "Full go version output" FORCE)
  set(LURPIC_GO_VERSION_SHORT "${LURPIC_GO_VERSION_SHORT}" CACHE STRING "Parsed Go version number" FORCE)
  set(LURPIC_GO_VERSION_MINIMUM "${LURPIC_GO_VERSION_MINIMUM}" CACHE STRING "Minimum supported Go version" FORCE)
  set(LURPIC_GO_GOMOD "${LURPIC_GO_GOMOD}" CACHE FILEPATH "Resolved Go module file" FORCE)
  set(LURPIC_GO_GOROOT "${LURPIC_GO_GOROOT}" CACHE PATH "Resolved Go GOROOT" FORCE)
  set(LURPIC_GO_GOPATH "${LURPIC_GO_GOPATH}" CACHE PATH "Resolved Go GOPATH" FORCE)
  set(LURPIC_GO_CGO_ENABLED "${LURPIC_GO_CGO_ENABLED}" CACHE STRING "Resolved Go CGO_ENABLED" FORCE)
endfunction()
