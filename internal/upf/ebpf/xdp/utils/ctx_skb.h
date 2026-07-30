// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/pkt_cls.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>

#define __ctx_buff __sk_buff

enum ctx_action {
	CTX_ACT_OK = TC_ACT_OK,
	CTX_ACT_DROP = TC_ACT_SHOT,
	CTX_ACT_ABORTED = TC_ACT_SHOT,
};

/* GTP-U encap: the tunnel headers grown by ctx_encap are outer L3 + UDP, and
 * the GTP-U header takes the inner-L2 slot of BPF_F_ADJ_ROOM_ENCAP_L2 so
 * segmentation replays it per segment. */
#define CTX_ENCAP_FLAGS_IPV4(inner_l2_len)          \
	(BPF_F_ADJ_ROOM_ENCAP_L3_IPV4 |             \
	 BPF_F_ADJ_ROOM_ENCAP_L4_UDP |              \
	 BPF_F_ADJ_ROOM_ENCAP_L2(inner_l2_len))
#define CTX_ENCAP_FLAGS_IPV6(inner_l2_len)          \
	(BPF_F_ADJ_ROOM_ENCAP_L3_IPV6 |             \
	 BPF_F_ADJ_ROOM_ENCAP_L4_UDP |              \
	 BPF_F_ADJ_ROOM_ENCAP_L2(inner_l2_len))

static __always_inline void *ctx_data(struct __ctx_buff *ctx)
{
	return (void *)(long)ctx->data;
}

static __always_inline const void *ctx_data_end(struct __ctx_buff *ctx)
{
	return (const void *)(long)ctx->data_end;
}

/* Whole-frame length: data_end bounds only the linear head of a non-linear or
 * GSO skb, so skb->len is authoritative. */
static __always_inline __u64 ctx_full_len(struct __ctx_buff *ctx)
{
	return ctx->len;
}

/* Bytes from `from` (a pointer into the linear head) to the end of the frame,
 * frags included; `data_end` is unused here but keeps one call shape across
 * both contexts. */
static __always_inline __u64 ctx_len_from(struct __ctx_buff *ctx,
					  const void *data_end, const void *from)
{
	return ctx->len - (__u64)(from - (const void *)(long)ctx->data);
}

static __always_inline __u32 ctx_ingress_ifindex(struct __ctx_buff *ctx)
{
	return ctx->ingress_ifindex;
}

/* Guarantee `len` bytes are linear and writable: pulls frags into the head and
 * unclones (a cloned skb rejects direct writes). Clamped because
 * bpf_skb_pull_data fails outright when len exceeds skb->len. */
static __always_inline long ctx_pull(struct __ctx_buff *ctx, __u32 len)
{
	if (len > ctx->len)
		len = ctx->len;

	return bpf_skb_pull_data(ctx, len);
}

/* Remove `bytes` of encapsulation between the L2 and L3 headers; the caller's
 * L2 save/rewrite around the call finds the header already in place and
 * rewrites it with identical bytes.
 *
 * BPF_F_ADJ_ROOM_NO_CSUM_RESET must not be added: the shrink path runs
 * skb_postpull_rcsum, which downgrades CHECKSUM_PARTIAL to CHECKSUM_NONE when
 * the removed span crosses csum_start — GTP-U decap (≥36 bytes, outer-UDP
 * csum_start at 34) relies on that downgrade for correct checksums on veth
 * traffic. */
static __always_inline long ctx_decap(struct __ctx_buff *ctx, __s32 bytes)
{
	long ret = bpf_skb_adjust_room(ctx, -bytes, BPF_ADJ_ROOM_MAC,
				       BPF_F_ADJ_ROOM_FIXED_GSO);
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* Open `bytes` of room for encapsulation headers between the L2 and L3
 * headers. BPF_F_ADJ_ROOM_FIXED_GSO keeps gso_size/gso_segs consistent on
 * GSO super-frames across every resize. */
static __always_inline long ctx_encap(struct __ctx_buff *ctx, __s32 bytes,
				      __u64 encap_flags)
{
	long ret = bpf_skb_adjust_room(ctx, bytes, BPF_ADJ_ROOM_MAC,
				       encap_flags | BPF_F_ADJ_ROOM_FIXED_GSO);
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* Original L2 header after ctx_encap opened `encap_size` bytes of room: the
 * room opened after the L2 header, so it is still at the frame front and the
 * caller's copy to the front collapses to a self-copy. The caller
 * bounds-checks the returned pointer. */
static __always_inline struct ethhdr *ctx_encap_saved_eth(void *data,
							  __u64 encap_size)
{
	return (struct ethhdr *)data;
}

/* Grow `bytes` at the very front of the frame; the previous frame start (its
 * L2 header included) moves to offset `bytes`. */
static __always_inline long ctx_prepend(struct __ctx_buff *ctx, __s32 bytes)
{
	long ret = bpf_skb_change_head(ctx, bytes, 0);
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* Move the frame end by `delta` (negative trims, positive grows). */
static __always_inline long ctx_adjust_tail(struct __ctx_buff *ctx, __s32 delta)
{
	long ret = bpf_skb_change_tail(ctx, ctx->len + delta, 0);
	if (ret)
		return ret;

	return ctx_pull(ctx, CTX_PULL_LEN);
}

/* Copy `len` bytes at frame offset `off` into `to`; reads past the linear
 * head. */
static __always_inline long ctx_load_bytes(struct __ctx_buff *ctx, __u32 off,
					   void *to, __u32 len)
{
	return bpf_skb_load_bytes(ctx, off, to, len);
}

/* Transmit the frame back out its ingress interface; TC has no XDP_TX-style
 * verdict, the redirect helper's return value is the verdict. */
static __always_inline enum ctx_action ctx_tx_back(struct __ctx_buff *ctx)
{
	return bpf_redirect(ctx->ingress_ifindex, 0);
}

/* Transmit the frame out `ifindex`. */
static __always_inline enum ctx_action ctx_redirect_out(__u32 ifindex)
{
	return bpf_redirect(ifindex, 0);
}
