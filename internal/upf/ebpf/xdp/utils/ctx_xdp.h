// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>

/* Macros, not typedefs or wrapper functions: the XDP object's BTF and
 * instruction stream must be token-for-token what the pre-shim source
 * produced, and even an always_inline wrapper can shift how LLVM
 * re-associates address arithmetic. */
#define __ctx_buff xdp_md
#define ctx_action xdp_action

/* ctx_pull discards its length here — XDP frames are linear and writable —
 * but call sites name it, so the knob stays defined in both variants. */
#define CTX_PULL_LEN 192

#define CTX_ACT_OK XDP_PASS
#define CTX_ACT_DROP XDP_DROP
#define CTX_ACT_ABORTED XDP_ABORTED

/* TC encap needs BPF_F_ADJ_ROOM_ENCAP_* on bpf_skb_adjust_room; XDP moves raw
 * bytes and carries no offload state to describe. */
#define CTX_ENCAP_FLAGS_IPV4(inner_l2_len) 0
#define CTX_ENCAP_FLAGS_IPV6(inner_l2_len) 0

#define ctx_data(ctx) ((void *)(long)(ctx)->data)

#define ctx_data_end(ctx) ((const void *)(long)(ctx)->data_end)

/* Whole-frame length. On TC data_end bounds only the linear head of a GSO
 * super-frame, so the skb length is authoritative there; XDP frames are
 * always linear. */
#define ctx_full_len(ctx) ((__u64)((ctx)->data_end - (ctx)->data))

/* Bytes from `from` (a pointer into the frame) to the end of the frame; same
 * linear-head caveat as ctx_full_len. `data_end` is the caller's
 * already-derived bound. */
#define ctx_len_from(ctx, data_end, from) \
	((__u64)((const void *)(data_end) - (const void *)(from)))

#define ctx_ingress_ifindex(ctx) ((ctx)->ingress_ifindex)

/* Guarantee `len` bytes are linear and writable. XDP packets already are;
 * on TC this pulls frags and unclones (a cloned skb rejects direct writes). */
#define ctx_pull(ctx, len) ((long)0)

/* Remove `bytes` of encapsulation. The caller preserves the L2 header around
 * the call (saved before, rewritten after): XDP shrinks at the frame front,
 * TC shrinks between the L2 and L3 headers, and the caller's rewrite is
 * correct for both. */
#define ctx_decap(ctx, bytes) bpf_xdp_adjust_head(ctx, bytes)

/* Open `bytes` of room for encapsulation headers. XDP grows at the frame
 * front, TC between the L2 and L3 headers; ctx_encap_saved_eth resolves the
 * difference for the caller's L2 rewrite. */
#define ctx_encap(ctx, bytes, encap_flags) \
	bpf_xdp_adjust_head(ctx, (__s32) - (bytes))

/* Original L2 header after ctx_encap opened `encap_size` bytes of room: XDP
 * grew the frame at the front, so it sits at offset `encap_size` and the
 * caller copies it to the frame front; on TC it is already at the front and
 * the caller's copy collapses to a self-copy. The caller bounds-checks the
 * returned pointer. */
#define ctx_encap_saved_eth(data, encap_size) \
	((struct ethhdr *)((data) + (encap_size)))

/* Grow `bytes` at the very front of the frame; the previous frame start (its
 * L2 header included) moves to offset `bytes`. */
#define ctx_prepend(ctx, bytes) bpf_xdp_adjust_head(ctx, -(bytes))

/* Move the frame end by `delta` (negative trims, positive grows). */
#define ctx_adjust_tail(ctx, delta) bpf_xdp_adjust_tail(ctx, delta)

/* Copy `len` bytes at frame offset `off` into `to`; unlike direct access this
 * reads past the linear head on TC. */
#define ctx_load_bytes(ctx, off, to, len) bpf_xdp_load_bytes(ctx, off, to, len)

/* Transmit the frame back out its ingress interface. */
#define ctx_tx_back(ctx) ((enum ctx_action)XDP_TX)

/* Transmit the frame out `ifindex`. */
#define ctx_redirect_out(ifindex) \
	((enum ctx_action)bpf_redirect(ifindex, 0))
