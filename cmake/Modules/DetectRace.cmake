include_guard(GLOBAL)

function(lurpic_detect_race)
  set(_lurpic_have_race OFF)
  set(_lurpic_race_hint "Race detector unavailable in this Go toolchain")

  execute_process(
    COMMAND "${LURPIC_GO_EXECUTABLE}" test -race -run ^$ runtime
    WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
    OUTPUT_VARIABLE _lurpic_race_stdout
    ERROR_VARIABLE _lurpic_race_stderr
    RESULT_VARIABLE _lurpic_race_result
  )

  if(_lurpic_race_result EQUAL 0)
    set(_lurpic_have_race ON)
    set(_lurpic_race_hint "Race detector supported by this Go toolchain")
  else()
    if(NOT _lurpic_race_stderr STREQUAL "")
      string(STRIP "${_lurpic_race_stderr}" _lurpic_race_stderr)
      set(_lurpic_race_hint "${_lurpic_race_stderr}")
    endif()
  endif()

  set(LURPIC_HAVE_RACE "${_lurpic_have_race}" PARENT_SCOPE)
  set(LURPIC_RACE_HINT "${_lurpic_race_hint}" PARENT_SCOPE)
  set(LURPIC_HAVE_RACE "${_lurpic_have_race}" CACHE BOOL "Whether the Go race detector is supported" FORCE)
  set(LURPIC_RACE_HINT "${_lurpic_race_hint}" CACHE STRING "Race detector capability hint" FORCE)
endfunction()
