// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>

/* Macros rather than inline wrappers: the XDP object's BTF and instruction
 * stream must stay what the pre-shim source produced, and even an
 * always_inline wrapper can shift how LLVM re-associates address arithmetic. */
#define __ctx_buff xdp_md

#define CTX_DP_SEC(name) SEC("xdp/" name)

#define CTX_PULL_LEN 192
#define CTX_NEEDS_PULL 0

/* Identity: enum ctx_action carries the XDP verdict encoding. */
#define ctx_verdict(action) ((int)(action))

/* XDP moves raw bytes and carries no offload state to describe. */
#define CTX_ENCAP_FLAGS_IPV4 0
#define CTX_ENCAP_FLAGS_IPV6 0

#define ctx_data(ctx) ((void *)(long)(ctx)->data)

#define ctx_data_end(ctx) ((const void *)(long)(ctx)->data_end)

#define ctx_full_len(ctx) ((__u64)((ctx)->data_end - (ctx)->data))

#define ctx_len_from(ctx, data_end, from) \
	((__u64)((const void *)(data_end) - (const void *)(from)))

#define ctx_frame_holds(ctx, data_end, from, len) \
	((const void *)(from) + (len) <= (const void *)(data_end))

#define ctx_tail_excess(ctx, data_end, from, keep) \
	((long)(data_end) - (long)((const __u8 *)(from) + (keep)))

#define ctx_ingress_ifindex(ctx) ((ctx)->ingress_ifindex)

/* xdp_md carries no GSO metadata, so the count is unavailable rather than
 * zero: generic XDP runs downstream of GRO and can be handed a merged frame
 * it cannot count. See the segmentation offload section of
 * docs/explanation/user_plane_packet_processing_with_ebpf.md. */
#define ctx_gso_segs(ctx) ((__u32)0)

/* XDP frames are already linear and writable. */
#define ctx_pull(ctx, len) ((long)0)

#define ctx_decap(ctx, bytes, inner_is_ipv6) \
	((void)(inner_is_ipv6), bpf_xdp_adjust_head(ctx, bytes))

#define ctx_encap(ctx, bytes, encap_flags) \
	bpf_xdp_adjust_head(ctx, (__s32) - (bytes))

#define ctx_encap_saved_eth(data, encap_size) \
	((struct ethhdr *)((data) + (encap_size)))

#define ctx_prepend(ctx, bytes) bpf_xdp_adjust_head(ctx, -(bytes))

#define ctx_adjust_tail(ctx, delta) bpf_xdp_adjust_tail(ctx, delta)

#define ctx_load_bytes(ctx, off, to, len) bpf_xdp_load_bytes(ctx, off, to, len)

/* XDP frames are never CHECKSUM_PARTIAL, so the check field always holds a
 * full checksum and RFC 1624 arithmetic applies directly. */
#define CTX_L4_CSUM_VIA_HELPERS 0
/* Unreachable: every call site is behind CTX_L4_CSUM_VIA_HELPERS. Traps
 * rather than returning success, which would skip the update silently. */
#define ctx_l4_csum_replace(ctx, off, from, to, flags)                \
	({                                                            \
		(void)(off), (void)(from), (void)(to), (void)(flags); \
		__builtin_trap();                                     \
		(long)0;                                              \
	})

/* XDP sees VLAN tags as frame bytes. */
#define CTX_INBAND_VLAN 1

#define ctx_vlan_ingress(ctx) ((long)0)

/* egress_vid is unevaluated: XDP frames carry their tag in-band, and the vid
 * expressions read volatile config the object must not load for nothing. */
#define ctx_tx_back(ctx, egress_vid) (CTX_ACT_TX)

/* bpf_redirect returns XDP_REDIRECT, which is CTX_ACT_REDIRECT. */
#define ctx_redirect_out(ctx, ifindex, egress_vid) \
	((enum ctx_action)bpf_redirect(ifindex, 0))
