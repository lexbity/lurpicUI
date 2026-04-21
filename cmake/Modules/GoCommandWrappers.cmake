include_guard(GLOBAL)

function(_lurpic_add_go_target)
  set(options)
  set(one_value_args NAME SUBCOMMAND WORKING_DIRECTORY DESCRIPTION)
  set(multi_value_args PACKAGES ARGS ENV)
  cmake_parse_arguments(LURPIC "${options}" "${one_value_args}" "${multi_value_args}" ${ARGN})

  if(NOT LURPIC_NAME)
    message(FATAL_ERROR "lurpic_add_go_target requires NAME")
  endif()
  if(NOT LURPIC_SUBCOMMAND)
    message(FATAL_ERROR "lurpic_add_go_target requires SUBCOMMAND")
  endif()
  if(NOT DEFINED LURPIC_WORKING_DIRECTORY OR LURPIC_WORKING_DIRECTORY STREQUAL "")
    set(LURPIC_WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}")
  endif()
  if(NOT LURPIC_DESCRIPTION)
    set(LURPIC_DESCRIPTION "Go ${LURPIC_SUBCOMMAND} target")
  endif()
  if(NOT LURPIC_PACKAGES)
    set(LURPIC_PACKAGES ./...)
  endif()

  set(_lurpic_go_cache_dir "${CMAKE_CURRENT_BINARY_DIR}/go-cache")
  set(_lurpic_go_tmp_dir "${CMAKE_CURRENT_BINARY_DIR}/go-tmp")
  file(MAKE_DIRECTORY "${_lurpic_go_cache_dir}" "${_lurpic_go_tmp_dir}")

  set(_lurpic_command "${CMAKE_COMMAND}" -E env)
  list(APPEND _lurpic_command
    "GOCACHE=${_lurpic_go_cache_dir}"
    "GOTMPDIR=${_lurpic_go_tmp_dir}"
  )
  foreach(_lurpic_env_entry IN LISTS LURPIC_ENV)
    list(APPEND _lurpic_command "${_lurpic_env_entry}")
  endforeach()
  list(APPEND _lurpic_command "${LURPIC_GO_EXECUTABLE}" "${LURPIC_SUBCOMMAND}")
  foreach(_lurpic_arg IN LISTS LURPIC_ARGS)
    list(APPEND _lurpic_command "${_lurpic_arg}")
  endforeach()
  foreach(_lurpic_package IN LISTS LURPIC_PACKAGES)
    list(APPEND _lurpic_command "${_lurpic_package}")
  endforeach()

  add_custom_target("${LURPIC_NAME}"
    COMMAND ${_lurpic_command}
    WORKING_DIRECTORY "${LURPIC_WORKING_DIRECTORY}"
    COMMENT "${LURPIC_DESCRIPTION}"
    VERBATIM
  )
endfunction()

function(lurpic_add_go_build_target)
  _lurpic_add_go_target(SUBCOMMAND build ${ARGN})
endfunction()

function(lurpic_add_go_vet_target)
  _lurpic_add_go_target(SUBCOMMAND vet ${ARGN})
endfunction()

function(lurpic_add_go_test_target)
  _lurpic_add_go_target(SUBCOMMAND test ${ARGN})
endfunction()

function(lurpic_add_optional_go_test_target)
  set(options)
  set(one_value_args NAME WORKING_DIRECTORY DESCRIPTION AVAILABLE UNAVAILABLE_MESSAGE)
  set(multi_value_args PACKAGES ARGS ENV)
  cmake_parse_arguments(LURPIC "${options}" "${one_value_args}" "${multi_value_args}" ${ARGN})

  if(LURPIC_AVAILABLE)
    lurpic_add_go_test_target(
      NAME "${LURPIC_NAME}"
      WORKING_DIRECTORY "${LURPIC_WORKING_DIRECTORY}"
      DESCRIPTION "${LURPIC_DESCRIPTION}"
      PACKAGES ${LURPIC_PACKAGES}
      ARGS ${LURPIC_ARGS}
      ENV ${LURPIC_ENV}
    )
  else()
    if(NOT LURPIC_UNAVAILABLE_MESSAGE)
      set(LURPIC_UNAVAILABLE_MESSAGE "Target ${LURPIC_NAME} is unavailable in this configuration.")
    endif()
    add_custom_target("${LURPIC_NAME}"
      COMMAND ${CMAKE_COMMAND} -E echo "${LURPIC_UNAVAILABLE_MESSAGE}"
      COMMAND ${CMAKE_COMMAND} -E false
      COMMENT "${LURPIC_UNAVAILABLE_MESSAGE}"
      VERBATIM
    )
  endif()
endfunction()

function(lurpic_add_go_list_target)
  _lurpic_add_go_target(SUBCOMMAND list ${ARGN})
endfunction()
