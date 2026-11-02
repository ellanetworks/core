// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include "bpf/utils/pdr.h"

#define MAX_URR_PER_SESSION 8

#define URR_MAP_SIZE (MAX_URR_PER_SESSION * MAX_PDU_SESSIONS)

/* URR IDs are scoped to their PFCP session, so the map key is (SEID, URR ID);
 * this keeps IDs per-session and needs no cross-session allocation. */
struct urr_key {
	__u64 seid;
	__u32 urr_id;
	__u32 _pad;
};

struct urr_counters {
	__u64 uplink;
	__u64 downlink;
	__u64 uplink_packets;
	__u64 downlink_packets;
	__u64 first_packet_ns;
	__u64 last_packet_ns;
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct urr_key);
	__type(value, struct urr_counters);
	__uint(max_entries, URR_MAP_SIZE);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} urr_map SEC(".maps");
