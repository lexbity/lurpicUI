include_guard(GLOBAL)

function(_lurpic_trim_rust_version out_var raw_value)
  string(REGEX MATCH "([0-9]+\\.[0-9]+\\.[0-9]+)" _lurpic_rust_version_match "${raw_value}")
  if(CMAKE_MATCH_1)
    set(${out_var} "${CMAKE_MATCH_1}" PARENT_SCOPE)
  else()
    set(${out_var} "" PARENT_SCOPE)
  endif()
endfunction()

function(lurpic_find_rust_toolchain)
  set(_lurpic_cargo_executable "")
  set(_lurpic_rustc_executable "")

  find_program(_lurpic_cargo_executable NAMES cargo PATHS /usr/bin /usr/local/bin /opt/homebrew/bin)
  if(NOT _lurpic_cargo_executable)
    execute_process(
      COMMAND /bin/sh -lc "command -v cargo"
      OUTPUT_VARIABLE _lurpic_cargo_command_output
      OUTPUT_STRIP_TRAILING_WHITESPACE
      ERROR_STRIP_TRAILING_WHITESPACE
      RESULT_VARIABLE _lurpic_cargo_command_result
    )
    if(_lurpic_cargo_command_result EQUAL 0 AND NOT _lurpic_cargo_command_output STREQUAL "")
      set(_lurpic_cargo_executable "${_lurpic_cargo_command_output}")
    endif()
  endif()
  if(NOT _lurpic_cargo_executable)
    message(FATAL_ERROR "Rust toolchain not found. Install Rust with rustup and ensure cargo is on PATH.")
  endif()

  find_program(_lurpic_rustc_executable NAMES rustc PATHS /usr/bin /usr/local/bin /opt/homebrew/bin)
  if(NOT _lurpic_rustc_executable)
    execute_process(
      COMMAND /bin/sh -lc "command -v rustc"
      OUTPUT_VARIABLE _lurpic_rustc_command_output
      OUTPUT_STRIP_TRAILING_WHITESPACE
      ERROR_STRIP_TRAILING_WHITESPACE
      RESULT_VARIABLE _lurpic_rustc_command_result
    )
    if(_lurpic_rustc_command_result EQUAL 0 AND NOT _lurpic_rustc_command_output STREQUAL "")
      set(_lurpic_rustc_executable "${_lurpic_rustc_command_output}")
    endif()
  endif()
  if(NOT _lurpic_rustc_executable)
    message(FATAL_ERROR "Rust compiler not found. Install Rust with rustup and ensure rustc is on PATH.")
  endif()

  execute_process(
    COMMAND "${_lurpic_cargo_executable}" --version
    OUTPUT_VARIABLE _lurpic_cargo_version_output
    OUTPUT_STRIP_TRAILING_WHITESPACE
    ERROR_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE _lurpic_cargo_version_result
  )
  if(NOT _lurpic_cargo_version_result EQUAL 0)
    message(FATAL_ERROR "Failed to run '${_lurpic_cargo_executable} --version'.")
  endif()

  execute_process(
    COMMAND "${_lurpic_rustc_executable}" --version
    OUTPUT_VARIABLE _lurpic_rustc_version_output
    OUTPUT_STRIP_TRAILING_WHITESPACE
    ERROR_STRIP_TRAILING_WHITESPACE
    RESULT_VARIABLE _lurpic_rustc_version_result
  )
  if(NOT _lurpic_rustc_version_result EQUAL 0)
    message(FATAL_ERROR "Failed to run '${_lurpic_rustc_executable} --version'.")
  endif()

  _lurpic_trim_rust_version(LURPIC_CARGO_VERSION "${_lurpic_cargo_version_output}")
  _lurpic_trim_rust_version(LURPIC_RUSTC_VERSION "${_lurpic_rustc_version_output}")

  set(LURPIC_HAVE_RUST ON PARENT_SCOPE)
  set(LURPIC_CARGO_EXECUTABLE "${_lurpic_cargo_executable}" PARENT_SCOPE)
  set(LURPIC_RUSTC_EXECUTABLE "${_lurpic_rustc_executable}" PARENT_SCOPE)
  set(LURPIC_CARGO_VERSION "${LURPIC_CARGO_VERSION}" PARENT_SCOPE)
  set(LURPIC_RUSTC_VERSION "${LURPIC_RUSTC_VERSION}" PARENT_SCOPE)

  set(LURPIC_HAVE_RUST ON CACHE BOOL "Whether Rust toolchain prerequisites are available" FORCE)
  set(LURPIC_CARGO_EXECUTABLE "${_lurpic_cargo_executable}" CACHE FILEPATH "Cargo executable used by CMake" FORCE)
  set(LURPIC_RUSTC_EXECUTABLE "${_lurpic_rustc_executable}" CACHE FILEPATH "Rust compiler executable used by CMake" FORCE)
  set(LURPIC_CARGO_VERSION "${LURPIC_CARGO_VERSION}" CACHE STRING "Parsed cargo version" FORCE)
  set(LURPIC_RUSTC_VERSION "${LURPIC_RUSTC_VERSION}" CACHE STRING "Parsed rustc version" FORCE)
endfunction()
