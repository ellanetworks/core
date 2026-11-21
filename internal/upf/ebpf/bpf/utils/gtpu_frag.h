// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <stdbool.h>

#include "bpf/utils/common.h"
#include "bpf/utils/packet_context.h"

#define IPV6_MIN_MTU 1280

#define FRAG_TX_MAX_PKT 9216

#define FRAG_TX_MIN_PKT 20

#define FRAG_TX_SCRATCH_PAD 8

int frag_tx_enabled;

struct frag_tx_scratch {
	__u8 payload[FRAG_TX_MAX_PKT];
	__u8 pad[FRAG_TX_SCRATCH_PAD];
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 4 * 1024 * 1024);
} frag_tx_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct frag_tx_scratch);
	__uint(max_entries, 1);
} frag_tx_scratch SEC(".maps");

static __always_inline enum ctx_action
fragmented_transport_to_stack(struct packet_context *ctx)
{
	if (!reasm_veth_ifindex || frame_is_reassembled(ctx->ctx_buff))
		return drop_with(ctx, UPF_DROP_FRAGMENTED_TRANSPORT);

	return CTX_ACT_OK;
}

static __always_inline bool
downlink_fragments_outer_ipv4(const struct iphdr *ip4)
{
	return frag_tx_enabled && (ip4->frag_off & bpf_htons(0x4000)) == 0;
}

static __always_inline bool
downlink_fragments_outer_ipv6(const struct ipv6hdr *ip6, __u32 inner_mtu)
{
	if (!frag_tx_enabled || inner_mtu >= IPV6_MIN_MTU)
		return false;

	return bpf_ntohs(ip6->payload_len) + sizeof(struct ipv6hdr) <=
	       IPV6_MIN_MTU;
}

static __always_inline __u32 ipv6_packet_too_big_mtu(__u32 inner_mtu)
{
	if (frag_tx_enabled && inner_mtu < IPV6_MIN_MTU)
		return IPV6_MIN_MTU;

	return inner_mtu;
}

enum frag_tx_result {
	FRAG_TX_FITS = 0,
	FRAG_TX_CAPTURED,
	FRAG_TX_RING_FULL,
	FRAG_TX_TOO_LARGE,
	FRAG_TX_FAILED,
};

__noinline __weak int frag_tx_offload(struct __ctx_buff *ctx_buff, __u32 off,
				      __u32 len, __u32 check_mtu)
{
	__u32 mtu_len = len;

	if (!ctx_buff)
		return FRAG_TX_FAILED;

	if (check_mtu) {
		const long fits = bpf_check_mtu(ctx_buff, n3_mtu_ifindex(),
						&mtu_len, 0, 0);
		if (fits < 0)
			return FRAG_TX_FAILED;
		if (fits == 0)
			return FRAG_TX_FITS;
	}

	const bool too_large = len > FRAG_TX_MAX_PKT;
	const bool too_small = len < FRAG_TX_MIN_PKT;
	if (len > FRAG_TX_MAX_PKT)
		len = FRAG_TX_MAX_PKT;
	if (len < FRAG_TX_MIN_PKT)
		len = FRAG_TX_MIN_PKT;

	if (too_large)
		return FRAG_TX_TOO_LARGE;
	if (too_small)
		return FRAG_TX_FAILED;

	const __u32 cpu = bpf_get_smp_processor_id();
	struct frag_tx_scratch *s = bpf_map_lookup_elem(&frag_tx_scratch, &cpu);
	if (!s)
		return FRAG_TX_FAILED;

	if (ctx_load_bytes(ctx_buff, off, s->payload, len) < 0)
		return FRAG_TX_FAILED;

	if (bpf_ringbuf_output(&frag_tx_map, s->payload, len, 0) != 0)
		return FRAG_TX_RING_FULL;

	return FRAG_TX_CAPTURED;
}

static __always_inline int frag_tx_offer(struct packet_context *ctx,
					 const void *l3)
{
	if (!frag_tx_enabled)
		return FRAG_TX_FITS;

	return frag_tx_offload(
		ctx->ctx_buff, ctx_frame_offset(ctx->ctx_buff, l3),
		ctx_len_from(ctx->ctx_buff, ctx->data_end, l3), 1);
}

static __always_inline enum upf_drop_reason frag_tx_drop_reason(int result)
{
	if (result == FRAG_TX_RING_FULL)
		return UPF_DROP_FRAG_TX_RING_FULL;
	if (result == FRAG_TX_TOO_LARGE)
		return UPF_DROP_FRAG_TX_TOO_LARGE;
	return UPF_DROP_INTERNAL_FRAG_TX_FAILED;
}

static __always_inline enum ctx_action
frag_tx_route_too_small(struct packet_context *ctx, const void *l3)
{
	if (!frag_tx_enabled)
		return drop_with(ctx, UPF_DROP_FIB_FRAG_NEEDED);

	const int offered = frag_tx_offload(
		ctx->ctx_buff, ctx_frame_offset(ctx->ctx_buff, l3),
		ctx_len_from(ctx->ctx_buff, ctx->data_end, l3), 0);
	if (offered == FRAG_TX_CAPTURED)
		return CTX_ACT_SLOW_PATH;
	return drop_with(ctx, frag_tx_drop_reason(offered));
}
