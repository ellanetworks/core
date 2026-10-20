// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include "bpf/utils/common.h"
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

struct urr_counters {
	__u64 uplink;
	__u64 downlink;
};

enum urr_direction {
	URR_UPLINK,
	URR_DOWNLINK,
};

/* (SEID, URR ID) -> Uplink and downlink byte counts */
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct urr_key);
	__type(value, struct urr_counters);
	__uint(max_entries, URR_MAP_SIZE);
} urr_map SEC(".maps");

static __always_inline __u64 urr_packet_len(struct packet_context *ctx)
{
	const void *l3;
	__u64 len;

	if (ctx->ip4) {
		l3 = ctx->ip4;
		len = bounded_u16(bpf_ntohs(ctx->ip4->tot_len));
	} else if (ctx->ip6) {
		l3 = ctx->ip6;
		len = bounded_u16(bpf_ntohs(ctx->ip6->payload_len)) +
		      sizeof(struct ipv6hdr);
	} else {
		return 0;
	}

	const __u64 held = ctx_len_from(ctx->ctx_buff, ctx->data_end, l3);

	return len < held ? len : held;
}

/* bytes is passed in: the caller charges after the routing verdict, by which
 * time encapsulation may have resized the frame. */
static __always_inline void update_urr_bytes(struct packet_context *ctx,
					     __u64 seid, __u32 urr_id,
					     __u64 bytes,
					     enum urr_direction direction)
{
	if (!urr_id) {
		upf_printk("upf: urr_id is 0 - no URR associated with packet");
		return;
	}
	upf_printk("upf: update URR found for urr_id:%d", urr_id);
	struct urr_key key = { .seid = seid, .urr_id = urr_id };
	struct urr_counters *counters = bpf_map_lookup_elem(&urr_map, &key);
	if (!counters) {
		upf_printk("upf: no URR found for urr_id:%d", urr_id);
		return;
	}
	/* Per-CPU value, and BPF runs with preemption disabled, so nothing else
	 * can reach this slot: an atomic would only add a locked round trip.
	 * Userspace sums across CPUs when it reads. */
	if (direction == URR_DOWNLINK)
		counters->downlink += bytes;
	else
		counters->uplink += bytes;
}
