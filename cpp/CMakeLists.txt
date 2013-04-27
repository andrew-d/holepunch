cmake_minimum_required (VERSION 2.6)
project (holepunch)

set (holepunch_VERSION_MAJOR 0)
set (holepunch_VERSION_MINOR 0)
set (holepunch_VERSION_PATCH 1)

configure_file (
    "${PROJECT_SOURCE_DIR}/config.h.in"
    "${PROJECT_BINARY_DIR}/config.h"
    )

include_directories("${PROJECT_BINARY_DIR}", "${PROJECT_SOURCE_DIR}/vendor")

find_package(Boost 1.48 COMPONENTS thread REQUIRED)
include_directories( ${Boost_INCLUDEDIR} )

add_executable(holepunch main.cpp fdutil.cpp tcp.cpp)
target_link_libraries(holepunch ${Boost_LIBRARIES})
