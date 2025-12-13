// Copyright 2024 Ella Networks

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include "xdp/utils/pdr.h"
#include "xdp/utils/trace.h"
#include "xdp/utils/packet_context.h"

#define URR_MAP_SIZE MAX_PDU_SESSIONS

/* URR ID -> Byte count */
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, __u32);
	__type(value, __u64);
	__uint(max_entries, URR_MAP_SIZE);
} urr_map SEC(".maps");

static __always_inline void
update_urr_bytes(struct packet_context *ctx, __u32 urr_id)
{
	if (!urr_id) {
		upf_printk("upf: urr_id is 0 - no URR associated with packet");
		return;
	}
	upf_printk("upf: update URR found for urr_id:%d", urr_id);
	__u64 *byte_count = bpf_map_lookup_elem(&urr_map, &urr_id);
	if (!byte_count) {
		upf_printk("upf: no URR found for urr_id:%d", urr_id);
		return;
	}
	__u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
	__sync_fetch_and_add(byte_count, packet_size);
}
