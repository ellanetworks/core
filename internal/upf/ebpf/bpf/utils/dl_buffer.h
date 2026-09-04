// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <stdbool.h>

#include "bpf/utils/common.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/pdr.h"

/* Ifindex of the veth re-injected packets arrive on, 0 when the buffer responder is not running. */
int buffer_veth_ifindex;

// True when ctx ingressed on the buffer injection veth.
static __always_inline bool
frame_is_reinjected(struct __ctx_buff *ctx)
{
	return buffer_veth_ifindex != 0 &&
	       ctx_ingress_ifindex(ctx) == (__u32)buffer_veth_ifindex;
}

// Largest capturable L3 packet.
#define DL_BUFFER_MAX_PKT 9000

// Smallest capturable L3 packet: the bare IPv4 header.
#define DL_BUFFER_MIN_PKT 20

/* Scratch slack so the length clamp and pointer arithmetic verify. */
#define DL_BUFFER_SCRATCH_PAD 8

/* One record: the 16-byte header followed by the packet. */
struct dl_buffer_hdr {
	__u64 local_seid;
	__u16 pdr_id;
	__u16 len;
	__u8 qfi;
	__u8 family; /* 4 or 6 */
	__u16 pad;
};

struct dl_buffer_scratch {
	struct dl_buffer_hdr hdr;
	__u8 payload[DL_BUFFER_MAX_PKT];
	__u8 pad[DL_BUFFER_SCRATCH_PAD];
};

/* 2 MiB ring buffer carrying whole packets; must be a power of two >= PAGE_SIZE. */
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 2 * 1024 * 1024);
} dl_buffer_map SEC(".maps");

/* Per-CPU-indexed regular ARRAY; the Go loader sets max_entries to the CPU count. */
struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct dl_buffer_scratch);
	__uint(max_entries, 1);
} dl_buffer_scratch SEC(".maps");

/* Capture counters, kept separate from drop reasons. */
struct dl_buffer_counters {
	__u64 captured;
	__u64 ring_full;
	__u64 too_large;
	__u64 gso;
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct dl_buffer_counters);
	__uint(max_entries, 1);
} dl_buffer_counters_map SEC(".maps");

/* Copy the L3 packet at l3 into the ring buffer, prefixed by the session identity. */
static __always_inline bool
dl_buffer_capture(struct packet_context *ctx, const struct pdr_info *pdr,
		  const struct qer_info *qer, const void *l3, __u8 family)
{
	const __u32 zero = 0;
	struct dl_buffer_counters *ctrs =
		bpf_map_lookup_elem(&dl_buffer_counters_map, &zero);

	if (frame_is_merged(ctx)) {
		if (ctrs)
			ctrs->gso++;
		return false;
	}

	__u32 len = ctx_len_from(ctx->ctx_buff, ctx->data_end, l3);

	// Two-sided clamp to help the verifier.
	const bool too_large = len > DL_BUFFER_MAX_PKT;
	if (len > DL_BUFFER_MAX_PKT)
		len = DL_BUFFER_MAX_PKT;
	if (len < DL_BUFFER_MIN_PKT)
		len = DL_BUFFER_MIN_PKT;

	if (too_large) {
		if (ctrs)
			ctrs->too_large++;
		return false;
	}

	const __u32 cpu = bpf_get_smp_processor_id();
	struct dl_buffer_scratch *s =
		bpf_map_lookup_elem(&dl_buffer_scratch, &cpu);
	if (!s)
		return false;

	s->hdr = (struct dl_buffer_hdr){
		.local_seid = pdr->local_seid,
		.pdr_id = pdr->pdr_id,
		.len = len,
		.qfi = qer->qfi,
		.family = family,
	};

	if (ctx_load_bytes(ctx->ctx_buff, ctx_frame_offset(ctx->ctx_buff, l3),
			   s->payload, len) < 0)
		return false;

	if (bpf_ringbuf_output(&dl_buffer_map, s,
			       sizeof(s->hdr) + len, 0) != 0) {
		if (ctrs)
			ctrs->ring_full++;
		return false;
	}

	if (ctrs)
		ctrs->captured++;

	return true;
}
