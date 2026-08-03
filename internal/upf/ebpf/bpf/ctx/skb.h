// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/pkt_cls.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>

#include "bpf/ctx/action.h"
#include <bpf/bpf_endian.h>

#define __ctx_buff __sk_buff

/* cilium/ebpf resolves the "tcx/ingress" prefix to BPF_TCX_INGRESS; the bare
 * "tc"/"classifier" names carry attach type 0. */
#define CTX_DP_SEC(name) SEC("tcx/ingress/" name)

/* Covers the deepest direct write: Ethernet (14) + VLAN (4) + IPv4 with
 * options (60) + TCP with options (60). Past this bound, use ctx_load_bytes
 * or another ctx_pull. */
#define CTX_PULL_LEN 192

/* A TC skb may be non-linear or cloned. */
#define CTX_NEEDS_PULL 1

#define VLAN_TAG_LEN 4

/* An abort is a drop here, and both forwarding actions are TC_ACT_REDIRECT
 * because the redirect helper has already recorded the target. */
static __always_inline int ctx_verdict(enum ctx_action action)
{
	switch (action) {
	case CTX_ACT_OK:
		return TC_ACT_OK;
	case CTX_ACT_TX:
	case CTX_ACT_REDIRECT:
		return TC_ACT_REDIRECT;
	case CTX_ACT_DROP:
	case CTX_ACT_ABORTED:
		break;
	}

	return TC_ACT_SHOT;
}

/* No BPF_F_ADJ_ROOM_ENCAP_L2: GTP-U carries bare IP, so inner_mac stays equal
 * to inner_net and the GTP-U header falls inside the tunnel span segmentation
 * replays verbatim per segment (tnl_hlen, net/ipv4/udp_offload.c). */
#define CTX_ENCAP_FLAGS_IPV4 \
	(BPF_F_ADJ_ROOM_ENCAP_L3_IPV4 | BPF_F_ADJ_ROOM_ENCAP_L4_UDP)
#define CTX_ENCAP_FLAGS_IPV6 \
	(BPF_F_ADJ_ROOM_ENCAP_L3_IPV6 | BPF_F_ADJ_ROOM_ENCAP_L4_UDP)

static __always_inline void *ctx_data(struct __ctx_buff *ctx)
{
	return (void *)(long)ctx->data;
}

static __always_inline const void *ctx_data_end(struct __ctx_buff *ctx)
{
	return (const void *)(long)ctx->data_end;
}

/* data_end bounds only the linear head of a non-linear or GSO skb, so
 * skb->len is authoritative. It excludes a tag the kernel moved to metadata,
 * which XDP counts as frame bytes: added back so both builds meter the same
 * traffic identically. */
static __always_inline __u64 ctx_full_len(struct __ctx_buff *ctx)
{
	return ctx->len + (ctx->vlan_present ? VLAN_TAG_LEN : 0);
}

/* `from` must lie in the linear head. `data_end` is unused; it keeps one call
 * shape across both contexts. */
static __always_inline __u64 ctx_len_from(struct __ctx_buff *ctx,
					  const void *data_end,
					  const void *from)
{
	__u64 off = (__u64)(from - (const void *)(long)ctx->data);

	/* Opaque to the optimizer: the pointer difference must reach the
	 * verifier as one scalar — re-associated to (len - from) + data it is
	 * a pointer subtracted from a scalar, which the verifier rejects. */
	asm volatile("" : "+r"(off));

	/* A `from` that predates a decap sits ahead of ctx->data, and the
	 * unsigned difference would wrap into a length that admits any
	 * bounds check. */
	if (off > ctx->len)
		return 0;

	return ctx->len - off;
}

static __always_inline __u32 ctx_ingress_ifindex(struct __ctx_buff *ctx)
{
	return ctx->ingress_ifindex;
}

/* Non-zero exactly when the buffer is GSO: it is what skb_is_gso() reads. */
static __always_inline __u32 ctx_gso_size(struct __ctx_buff *ctx)
{
	return ctx->gso_size;
}

/* Segments the buffer holds, or 0 when the source did not count them: a
 * virtio, tap or vhost header sets gso_size and leaves gso_segs at 0, flagged
 * SKB_GSO_DODGY (include/linux/virtio_net.h). */
static __always_inline __u32 ctx_gso_segs(struct __ctx_buff *ctx)
{
	return ctx->gso_segs;
}

/* Counts frags, so it validates lengths on non-linear frames. Not a substitute
 * for a data_end check before direct access. */
static __always_inline int ctx_frame_holds(struct __ctx_buff *ctx,
					   const void *data_end,
					   const void *from, __u64 len)
{
	return len <= ctx_len_from(ctx, data_end, from);
}

/* Negative when the frame ends short of `keep`. */
static __always_inline long ctx_tail_excess(struct __ctx_buff *ctx,
					    const void *data_end,
					    const void *from, __u32 keep)
{
	return (long)ctx_len_from(ctx, data_end, from) - (long)keep;
}

/* Pulls frags into the head and unclones; a cloned skb rejects direct writes.
 * Clamped because bpf_skb_pull_data fails when len exceeds skb->len. */
static __always_inline long ctx_pull(struct __ctx_buff *ctx, __u32 len)
{
	if (len > ctx->len)
		len = ctx->len;

	return bpf_skb_pull_data(ctx, len);
}

/* Remove `bytes` of encapsulation between the L2 and L3 headers; the caller
 * saves and rewrites the L2 header around the call.
 *
 * Stripping the outer L3+UDP+GTP span always drives
 * skb_checksum_start_offset below zero, where skb_postpull_rcsum downgrades
 * CHECKSUM_PARTIAL to CHECKSUM_NONE (include/linux/skbuff.h) — which decap
 * needs for correct checksums on veth traffic. Without
 * BPF_F_ADJ_ROOM_FIXED_GSO, bpf_skb_net_shrink refuses any non-TCP GSO frame
 * with -ENOTSUPP (net/core/filter.c). */
static __always_inline long ctx_decap(struct __ctx_buff *ctx, __s32 bytes,
				      __u8 inner_is_ipv6)
{
	/* Without a DECAP_L3 flag skb->protocol keeps the outer family
	 * (net/core/filter.c), so the frame is delivered to the wrong
	 * packet_type and egress picks offloads off the wrong protocol. */
	long ret = bpf_skb_adjust_room(
		ctx, -bytes, BPF_ADJ_ROOM_MAC,
		BPF_F_ADJ_ROOM_FIXED_GSO |
			(inner_is_ipv6 ? BPF_F_ADJ_ROOM_DECAP_L3_IPV6 :
					 BPF_F_ADJ_ROOM_DECAP_L3_IPV4));
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* Open `bytes` of room for encapsulation headers between the L2 and L3
 * headers.
 *
 * BPF_F_ADJ_ROOM_FIXED_GSO suppresses skb_decrease_gso_size
 * (net/core/filter.c), so segments overshoot the original max segment size by
 * the encap overhead rather than carry a silently reduced inner MSS. Inert
 * here: frame_is_merged drops merged frames before this point.
 *
 * This sets skb->encapsulation, after which the kernel rejects a further
 * ENCAP grow (-EALREADY) and bpf_skb_change_tail (-ENOTSUPP): encap must be
 * the last resize before the redirect. */
static __always_inline long ctx_encap(struct __ctx_buff *ctx, __s32 bytes,
				      __u64 encap_flags)
{
	long ret = bpf_skb_adjust_room(ctx, bytes, BPF_ADJ_ROOM_MAC,
				       encap_flags | BPF_F_ADJ_ROOM_FIXED_GSO);
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* The room opened after the L2 header, so the caller's copy to the frame front
 * collapses to a self-copy. */
static __always_inline struct ethhdr *ctx_encap_saved_eth(void *data,
							  __u64 encap_size)
{
	return (struct ethhdr *)data;
}

/* The previous frame start, L2 header included, moves to offset `bytes`. */
static __always_inline long ctx_prepend(struct __ctx_buff *ctx, __s32 bytes)
{
	long ret = bpf_skb_change_head(ctx, bytes, 0);
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* No pull needed after: bpf_skb_change_tail makes the whole skb linear and
 * writable (__bpf_try_make_writable over skb->len). */
static __always_inline long ctx_adjust_tail(struct __ctx_buff *ctx, __s32 delta)
{
	return bpf_skb_change_tail(ctx, ctx->len + delta, 0);
}

/* Reads past the linear head. */
static __always_inline long ctx_load_bytes(struct __ctx_buff *ctx, __u32 off,
					   void *to, __u32 len)
{
	return bpf_skb_load_bytes(ctx, off, to, len);
}

/* A TC skb can be CHECKSUM_PARTIAL (veth RX), where the check field holds only
 * the pseudo-header sum and direct RFC 1624 arithmetic is wrong.
 * inet_proto_csum_replace{2,4} behind this helper applies a delta correctly
 * for every ip_summed state. It invalidates every packet pointer, so it runs
 * after all direct field writes and the caller re-derives them. */
#define CTX_L4_CSUM_VIA_HELPERS 1
static __always_inline long ctx_l4_csum_replace(struct __ctx_buff *ctx,
						__u32 off, __u64 from, __u64 to,
						__u64 flags)
{
	return bpf_l4_csum_replace(ctx, off, from, to, flags);
}

/* skb_vlan_untag moves the tag out of the frame bytes before the hook, so the
 * in-band branches compile out. */
#define CTX_INBAND_VLAN 0

/* QinQ guard: the kernel strips only the outer tag before the hook, so an
 * ethertype that is still a VLAN tag means a second one remains in the frame
 * bytes. The tag is left in place: the stack needs it to demultiplex a frame
 * the datapath does not own to its VLAN sub-device. */
static __always_inline long ctx_vlan_ingress(struct __ctx_buff *ctx)
{
	if (ctx->protocol == bpf_htons(ETH_P_8021Q) ||
	    ctx->protocol == bpf_htons(ETH_P_8021AD))
		return 1;

	return 0;
}

/* Invalidates packet pointers. */
static __always_inline long ctx_vlan_egress(struct __ctx_buff *ctx,
					    int egress_vid)
{
	/* dev_queue_xmit would re-insert the ingress tag on the egress
	 * interface, and pushing onto a tagged skb writes the old tag in-band
	 * as QinQ, so it goes first. */
	if (ctx->vlan_present && bpf_skb_vlan_pop(ctx))
		return -1;

	if (!egress_vid)
		return 0;

	return bpf_skb_vlan_push(ctx, bpf_htons(ETH_P_8021Q),
				 (__u16)egress_vid);
}

/* bpf_redirect only records the target; ctx_verdict returns the verdict that
 * acts on it. The check keeps a rejected redirect from being reported as a
 * transmit. */
static __always_inline int ctx_redirect_recorded(__u32 ifindex)
{
	return bpf_redirect(ifindex, 0) == TC_ACT_REDIRECT;
}

static __always_inline enum ctx_action ctx_tx_back(struct __ctx_buff *ctx,
						   int egress_vid)
{
	if (ctx_vlan_egress(ctx, egress_vid) < 0)
		return CTX_ACT_ABORTED;

	if (!ctx_redirect_recorded(ctx->ingress_ifindex))
		return CTX_ACT_ABORTED;

	return CTX_ACT_TX;
}

static __always_inline enum ctx_action
ctx_redirect_out(struct __ctx_buff *ctx, __u32 ifindex, int egress_vid)
{
	if (ctx_vlan_egress(ctx, egress_vid) < 0)
		return CTX_ACT_ABORTED;

	if (!ctx_redirect_recorded(ifindex))
		return CTX_ACT_ABORTED;

	return CTX_ACT_REDIRECT;
}
