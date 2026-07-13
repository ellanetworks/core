// SPDX-FileCopyrightText: Ella Networks Inc.
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

/* Framed-route LPM keys: prefixlen (in bits) followed by the address, per the
 * BPF_MAP_TYPE_LPM_TRIE key convention. */
struct framed_ip4_key {
	__u32 prefixlen;
	__u32 addr; /* network byte order, matching iphdr->daddr */
};

struct framed_ip6_key {
	__u32 prefixlen;
	struct in6_addr addr;
};

/* Framed routes (TS 29.244 §5.16) reuse the owning session's downlink
 * forwarding. To keep a single source of truth, the value is the owning UE
 * address (the pdrs_downlink_* key), not a copy of the PDR: the datapath
 * redirects a framed hit to the live downlink PDR, so it tracks every
 * downlink-PDR change automatically. */
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__type(key, struct framed_ip4_key);
	__type(value, __u32); /* owning UE IPv4, key into pdrs_downlink_ip4 */
	__uint(max_entries, FRAMED_MAP_IPV4_SIZE);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} framed_downlink_ip4 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__type(key, struct framed_ip6_key);
	__type(value, struct in6_addr); /* owning UE IPv6, key into pdrs_downlink_ip6 */
	__uint(max_entries, FRAMED_MAP_IPV6_SIZE);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} framed_downlink_ip6 SEC(".maps");
