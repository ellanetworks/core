// Copyright 2025 Ella Networks

#pragma once

#include "xdp/utils/routing.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "xdp/utils/pdr.h"

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u32);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_UPLINK_SIZE);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} pdrs_uplink SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} uplink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} uplink_statistics SEC(".maps");
