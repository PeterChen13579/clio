# Hacked by Alex
#
# Original copyright notice: Copyright (c) 2020 Kristian Rumberg (kristianrumberg@gmail.com)
#
# Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated
# documentation files (the "Software"), to deal in the Software without restriction, including without limitation the
# rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit
# persons to whom the Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all copies or substantial portions of the
# Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
# WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
# COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
# OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

set(CGO_RESOLVE_BLACKLIST pthread rt gcov systemd)

set(CGO_CXXFLAGS_BLACKLIST
    "-Werror"
    "-Wall"
    "-Wextra"
    "-Wold-style-definition"
    "-fdiagnostics-color=always"
    "-Wformat-nonliteral"
    "-Wformat=2"
)

macro (add_cgo_executable GO_MOD_NAME GO_FILES CGO_DEPS GO_BIN)
  cgo_fetch_cflags_and_ldflags()
  cgo_build_envs()

  set(CGO_BUILT_FLAG ${CMAKE_CURRENT_BINARY_DIR}/${GO_MOD_NAME}.cgo.module)
  add_custom_command(
    OUTPUT ${CGO_BUILT_FLAG}
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
    COMMAND echo Building CGO modules for ${GO_BIN}
    COMMAND ${CMAKE_COMMAND} -E remove ${CGO_BUILT_FLAG}
    COMMAND env ${CGO_ENVS} go build -a -o ${GO_BIN} ./...
    COMMAND touch ${CGO_BUILT_FLAG}
    DEPENDS SHAMap clio_etl
  )
  add_custom_target(${GO_MOD_NAME}_cgo ALL DEPENDS ${CGO_BUILT_FLAG})

  set(GO_BUILT_FLAG ${CMAKE_CURRENT_BINARY_DIR}/${GO_MOD_NAME}.go.module)
  add_custom_command(
    OUTPUT ${GO_BUILT_FLAG}
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
    COMMAND echo Building GO modules for ${GO_BIN}
    COMMAND env ${CGO_ENVS} go build -o ${GO_BIN} ./...
    COMMAND touch ${GO_BUILT_FLAG}
    DEPENDS ${CGO_BUILT_FLAG} ${${GO_FILES}}
  )
  add_custom_target(${GO_MOD_NAME}_go ALL DEPENDS ${GO_BUILT_FLAG})

endmacro ()

macro (cgo_build_envs)
  set(CGO_ENVS CGO_CXXFLAGS="${CGO_CXXFLAGS}" CGO_LDFLAGS="${CGO_LDFLAGS}")

  # Assume ARM7 if on ARM
  if ("${CMAKE_SYSTEM_PROCESSOR}" STREQUAL "arm")
    list(APPEND CGO_ENVS GOARCH=arm GOARM=7)
  endif ()

  message(STATUS "CGO_LDFLAGS: ${CGO_LDFLAGS}")
  message(STATUS "CGO_CXXFLAGS: ${CGO_CXXFLAGS}")
endmacro ()

macro (cgo_fetch_cflags_and_ldflags)
  set(LIB_PATHS "${CMAKE_LIBRARY_PATH}" "${CMAKE_BINARY_DIR}/src/etl")
  set(INCLUDES "${CMAKE_INCLUDE_PATH}")

  foreach (P ${LIB_PATHS})
    list(APPEND CGO_LDFLAGS "-L${P}")
  endforeach ()

  foreach (P ${INCLUDES})
    list(APPEND CGO_CXXFLAGS "-isystem${P}")
  endforeach ()

  list(
    APPEND
    CGO_LDFLAGS
    "-L."
    "-L${CMAKE_BINARY_DIR}"
    "-lSHAMap"
    "-lfmt"
    "-lclio_etl"
    "-lxrpl"
    "-led25519"
    "-lsecp256k1"
    "-lssl"
    "-lboost_thread"
    "-lcrypto"
    "-ldl"
    "-lxxhash"
  )
  list(
    APPEND
    CGO_CXXFLAGS
    "-I${CMAKE_CURRENT_SOURCE_DIR}/../../src"
    "-I${CMAKE_CURRENT_SOURCE_DIR}"
    "-I."
    "-std=c++20"
    "-D_GNU_SOURCE=1"
  )

  # Need to remove warnings for CGo to work
  foreach (F ${CGO_CXXFLAGS_BLACKLIST})
    list(REMOVE_ITEM CGO_CXXFLAGS "${F}")
  endforeach ()
endmacro ()
