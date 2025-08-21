cmake_minimum_required(VERSION 3.28)
project(ellacore)

set(CMAKE_C_COMPILER clang)
set(CMAKE_C_FLAGS "-target bpf -std=gnu11 -Wall -Werror -g -O2 -I.")
set(CMAKE_EXPORT_COMPILE_COMMANDS on)

add_library(n3entrypoint OBJECT xdp/n3_bpf.c)

add_custom_command(OUTPUT n3entrypoint_bpf.o
	COMMAND bpftool gen object n3entrypoint_bpf.o n3entrypoint.o
	DEPENDS n3entrypoint.o
	VERBATIM)

add_library(n6entrypoint OBJECT xdp/n6_bpf.c)

add_custom_command(OUTPUT n6entrypoint_bpf.o
	COMMAND bpftool gen object n6entrypoint_bpf.o n6entrypoint.o
	DEPENDS n6entrypoint.o
	VERBATIM)
