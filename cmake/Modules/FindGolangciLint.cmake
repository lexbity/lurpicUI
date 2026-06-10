# FindGolangciLint — locate a pinned golangci-lint v2 binary
#
# The module searches for golangci-lint in the following order:
#   1. LURPIC_GOLANGCI_EXECUTABLE cache/override variable
#   2. PATH (via find_program)
#   3. $(go env GOPATH)/bin
#
# If a binary is found, its version is checked against the v2 major version
# (2.x).  A v1 binary or missing binary produces a FATAL_ERROR with an
# actionable install hint pinned to the project's required version.
#
# Variables set (all cached):
#   LURPIC_GOLANGCI_EXECUTABLE — path to the golangci-lint binary
#   LURPIC_GOLANGCI_VERSION    — full version string from `golangci-lint version`
#   LURPIC_GOLANGCI_VERSION_SHORT — parsed semver (e.g. "2.12.2")
#
# The required minimum major version is defined by
# _lurpic_golangci_minimum_major and can be overridden before including
# this module.

include_guard(GLOBAL)

# ---- Pinned minimum version ------------------------------------------------
set(_lurpic_golangci_minimum_major "2"
  CACHE INTERNAL "Minimum golangci-lint major version required by this project")

# ---- Helper: trim a golangci-lint version string to just MAJOR.MINOR.PATCH --
function(_lurpic_trim_golangci_version out_var raw_value)
  string(REGEX MATCH "version ([0-9]+\\.[0-9]+\\.[0-9]+)" _match "${raw_value}")
  if(CMAKE_MATCH_1)
    set(${out_var} "${CMAKE_MATCH_1}" PARENT_SCOPE)
  else()
    set(${out_var} "" PARENT_SCOPE)
  endif()
endfunction()

# ---- Main discovery --------------------------------------------------------
function(lurpic_find_golangci_lint)
  set(_lurpic_exe "")

  # 1. Override variable.
  if(DEFINED LURPIC_GOLANGCI_EXECUTABLE
      AND NOT LURPIC_GOLANGCI_EXECUTABLE STREQUAL ""
      AND NOT LURPIC_GOLANGCI_EXECUTABLE MATCHES "-NOTFOUND$")
    set(_lurpic_exe "${LURPIC_GOLANGCI_EXECUTABLE}")
    if(NOT EXISTS "${_lurpic_exe}")
      message(FATAL_ERROR
        "LURPIC_GOLANGCI_EXECUTABLE points to a non-existent file:\n"
        "  ${_lurpic_exe}\n"
        "Unset the variable or point it at a valid golangci-lint binary.")
    endif()
  endif()

  # 2. PATH search.
  if(NOT _lurpic_exe)
    find_program(_lurpic_exe NAMES golangci-lint)
  endif()

  # 3. GOPATH/bin fallback.
  if(NOT _lurpic_exe AND DEFINED LURPIC_GO_GOPATH AND NOT LURPIC_GO_GOPATH STREQUAL "")
    set(_lurpic_gopath_bin "${LURPIC_GO_GOPATH}/bin/golangci-lint")
    if(EXISTS "${_lurpic_gopath_bin}")
      set(_lurpic_exe "${_lurpic_gopath_bin}")
    endif()
  endif()
  if(NOT _lurpic_exe)
    # If LURPIC_GO_GOPATH is not yet available, try go env.
    execute_process(
      COMMAND /bin/sh -lc "command -v golangci-lint"
      OUTPUT_VARIABLE _lurpic_sh_output
      OUTPUT_STRIP_TRAILING_WHITESPACE
      ERROR_STRIP_TRAILING_WHITESPACE
      RESULT_VARIABLE _lurpic_sh_result
    )
    if(_lurpic_sh_result EQUAL 0 AND NOT _lurpic_sh_output STREQUAL "")
      set(_lurpic_exe "${_lurpic_sh_output}")
    endif()
  endif()

  # Not found — fatal.
  if(NOT _lurpic_exe)
    message(FATAL_ERROR
      "golangci-lint v${_lurpic_golangci_minimum_major} not found.\n"
      "Install it with:\n"
      "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v${_lurpic_golangci_minimum_major}\n"
      "Or download from https://golangci-lint.run/welcome/install/\n"
      "Then ensure 'golangci-lint' is on PATH, or set LURPIC_GOLANGCI_EXECUTABLE.")
  endif()

  # ---- Version check -------------------------------------------------------
  execute_process(
    COMMAND "${_lurpic_exe}" version
    WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
    OUTPUT_VARIABLE _lurpic_version_output
    OUTPUT_STRIP_TRAILING_WHITESPACE
    ERROR_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE _lurpic_version_result
  )
  if(NOT _lurpic_version_result EQUAL 0)
    message(FATAL_ERROR "Failed to run '${_lurpic_exe} version'.")
  endif()

  _lurpic_trim_golangci_version(LURPIC_GOLANGCI_VERSION_SHORT "${_lurpic_version_output}")
  if(LURPIC_GOLANGCI_VERSION_SHORT STREQUAL "")
    message(FATAL_ERROR
      "Unable to parse golangci-lint version from output:\n"
      "  ${_lurpic_version_output}\n"
      "Ensure the binary is a valid golangci-lint release.")
  endif()

  # Assert major version >= minimum.
  string(REGEX MATCH "^[0-9]+" _lurpic_major "${LURPIC_GOLANGCI_VERSION_SHORT}")
  if(_lurpic_major VERSION_LESS "${_lurpic_golangci_minimum_major}")
    message(FATAL_ERROR
      "golangci-lint v${_lurpic_golangci_minimum_major}+ required, but found:\n"
      "  ${_lurpic_exe} → ${_lurpic_version_output}\n"
      "The project configuration (${CMAKE_CURRENT_SOURCE_DIR}/.golangci.yaml) "
      "requires version ${_lurpic_golangci_minimum_major}.x.\n"
      "Install the correct version with:\n"
      "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v${_lurpic_golangci_minimum_major}")
  endif()

  # ---- Export cached variables ---------------------------------------------
  set(LURPIC_GOLANGCI_VERSION "${_lurpic_version_output}")
  set(LURPIC_GOLANGCI_VERSION_SHORT "${LURPIC_GOLANGCI_VERSION_SHORT}")

  set(LURPIC_GOLANGCI_EXECUTABLE "${_lurpic_exe}"
    CACHE FILEPATH "golangci-lint binary used by CMake" FORCE)
  set(LURPIC_GOLANGCI_VERSION "${_lurpic_version_output}"
    CACHE STRING "Full golangci-lint version output" FORCE)
  set(LURPIC_GOLANGCI_VERSION_SHORT "${LURPIC_GOLANGCI_VERSION_SHORT}"
    CACHE STRING "Parsed golangci-lint version number" FORCE)

  message(STATUS "FindGolangciLint: ${_lurpic_exe} (${LURPIC_GOLANGCI_VERSION_SHORT})")
endfunction()
