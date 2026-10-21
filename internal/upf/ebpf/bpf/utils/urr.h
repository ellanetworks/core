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

enum urr_direction {
	URR_UPLINK,
	URR_DOWNLINK,
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct urr_key);
	__type(value, struct urr_counters);
	__uint(max_entries, URR_MAP_SIZE);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} urr_map SEC(".maps");

static __always_inline __u64 urr_packet_len(struct packet_context *ctx)
{
	const void *l3;
	__u64 len;
	__u64 min_len;

	if (ctx->ip4) {
		l3 = ctx->ip4;
		len = bounded_u16(bpf_ntohs(ctx->ip4->tot_len));
		min_len = ctx->ip4->ihl * 4;
	} else if (ctx->ip6) {
		l3 = ctx->ip6;
		len = bounded_u16(bpf_ntohs(ctx->ip6->payload_len));
		if (!len)
			return 0;
		len += sizeof(struct ipv6hdr);
		min_len = sizeof(struct ipv6hdr);
	} else {
		return 0;
	}

	if (len < min_len ||
	    len > ctx_len_from(ctx->ctx_buff, ctx->data_end, l3))
		return 0;

	return len;
}

/* bytes is passed in: the caller charges after the routing verdict, by which
 * time encapsulation may have resized the frame. */
static __always_inline void update_urr_bytes(struct packet_context *ctx,
					     __u64 seid, __u32 urr_id,
					     __u64 bytes, __u64 now,
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
	if (!counters->first_packet_ns)
		counters->first_packet_ns = now;
	/* Per-CPU value, and BPF runs with preemption disabled, so nothing else
	 * can reach this slot: an atomic would only add a locked round trip.
	 * Userspace sums across CPUs when it reads. */
	if (direction == URR_DOWNLINK) {
		counters->downlink += bytes;
		counters->downlink_packets++;
	} else {
		counters->uplink += bytes;
		counters->uplink_packets++;
	}
	counters->last_packet_ns = now;
}
