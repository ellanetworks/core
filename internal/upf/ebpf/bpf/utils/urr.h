// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include "bpf/utils/pdr.h"
#include "bpf/utils/trace.h"
#include "bpf/utils/packet_context.h"

/* Up to two URRs (uplink, downlink) per session. */
#define URR_MAP_SIZE (2 * MAX_PDU_SESSIONS)

/* URR IDs are scoped to their PFCP session, so the map key is (SEID, URR ID);
 * this keeps IDs per-session and needs no cross-session allocation. */
struct urr_key {
	__u64 seid;
	__u32 urr_id;
	__u32 _pad;
};

/* (SEID, URR ID) -> Byte count */
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct urr_key);
	__type(value, __u64);
	__uint(max_entries, URR_MAP_SIZE);
} urr_map SEC(".maps");

/* bytes is passed in: the caller charges after the routing verdict, by which
 * time encapsulation may have resized the frame. */
static __always_inline void update_urr_bytes(struct packet_context *ctx,
					     __u64 seid, __u32 urr_id,
					     __u64 bytes)
{
	if (!urr_id) {
		upf_printk("upf: urr_id is 0 - no URR associated with packet");
		return;
	}
	upf_printk("upf: update URR found for urr_id:%d", urr_id);
	struct urr_key key = { .seid = seid, .urr_id = urr_id };
	__u64 *byte_count = bpf_map_lookup_elem(&urr_map, &key);
	if (!byte_count) {
		upf_printk("upf: no URR found for urr_id:%d", urr_id);
		return;
	}
	/* Per-CPU value, and BPF runs with preemption disabled, so nothing else
	 * can reach this slot: an atomic would only add a locked round trip.
	 * Userspace sums across CPUs when it reads. */
	*byte_count += bytes;
}
