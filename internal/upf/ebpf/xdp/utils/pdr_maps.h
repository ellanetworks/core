// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/in6.h>

#include "xdp/utils/pdr.h"

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u32);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_DOWNLINK_IPV4_SIZE);
} pdrs_downlink_ip4 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct in6_addr);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_DOWNLINK_IPV6_SIZE);
} pdrs_downlink_ip6 SEC(".maps");
