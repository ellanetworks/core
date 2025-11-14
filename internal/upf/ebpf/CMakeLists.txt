cmake_minimum_required(VERSION 3.28)
project(ellacore)

set(CMAKE_C_COMPILER clang)
set(CMAKE_C_FLAGS "-target bpf -std=gnu11 -Wall -Werror -g -O2 -I.")
set(CMAKE_EXPORT_COMPILE_COMMANDS on)

add_library(n3n6entrypoint OBJECT xdp/n3n6_bpf.c)

add_custom_command(OUTPUT n3n6entrypoint_bpf.o
	COMMAND bpftool gen object n3n6entrypoint_bpf.o n3n6entrypoint.o
	DEPENDS n3n6entrypoint.o
	VERBATIM)
