// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/pkt_cls.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define __ctx_buff __sk_buff

/* Datapath program section: SCHED_CLS with the BPF_TCX_INGRESS attach type
 * (cilium/ebpf resolves the "tcx/ingress" prefix; the bare "tc"/"classifier"
 * names carry attach type 0). */
#define CTX_DP_SEC(name) SEC("tcx/ingress/" name)

/* Bytes made linear and writable ahead of parsing and direct header writes.
 * Covers the deepest write: Ethernet (14) + VLAN (4) + IPv4 with options (60)
 * + TCP with options (60). Reads and writes past this bound need
 * ctx_load_bytes or another ctx_pull. */
#define CTX_PULL_LEN 192

/* Whether the datapath must pull before parsing and writing. A TC skb may be non-linear or cloned. */
#define CTX_NEEDS_PULL 1

enum ctx_action {
	CTX_ACT_OK = TC_ACT_OK,
	CTX_ACT_DROP = TC_ACT_SHOT,
	CTX_ACT_ABORTED = TC_ACT_SHOT,
};

/* GTP-U carries bare IP, so inner_mac stays equal to inner_net (the ipip
 * model) and the GTP-U header lives inside the opaque tunnel span that UDP
 * tunnel segmentation replays verbatim per segment (tnl_hlen in
 * net/ipv4/udp_offload.c runs from the outer transport header to
 * inner_mac). BPF_F_ADJ_ROOM_ENCAP_L2 is for Ethernet-inner tunnels. */
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
	 * subsequent bounds check. */
	if (off > ctx->len)
		return 0;

	return ctx->len - off;
}

static __always_inline __u32 ctx_ingress_ifindex(struct __ctx_buff *ctx)
{
	return ctx->ingress_ifindex;
}

/* Segment count of a GSO super-frame (0 or 1 for wire-sized frames). */
static __always_inline __u32 ctx_gso_segs(struct __ctx_buff *ctx)
{
	return ctx->gso_segs;
}

/* True when the frame holds `len` bytes starting at `from` — frags included,
 * so this validates datagram lengths on non-linear frames. Not a substitute
 * for a data_end check before direct access. */
static __always_inline int ctx_frame_holds(struct __ctx_buff *ctx,
					   const void *data_end,
					   const void *from, __u64 len)
{
	return len <= ctx_len_from(ctx, data_end, from);
}

/* Signed count of frame bytes past the first `keep` bytes starting at `from`
 * (negative when the frame ends short of that), counting frags. `from` must
 * lie in the linear head. */
static __always_inline long ctx_tail_excess(struct __ctx_buff *ctx,
					    const void *data_end,
					    const void *from, __u32 keep)
{
	return (long)ctx_len_from(ctx, data_end, from) - (long)keep;
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
 * The skb_postpull_rcsum in the shrink path downgrades CHECKSUM_PARTIAL to
 * CHECKSUM_NONE only once skb_checksum_start_offset drops below zero
 * (include/linux/skbuff.h, __skb_postpull_rcsum), which GTP-U decap needs for
 * correct checksums on veth traffic. Decap always strips the outer L3+UDP+GTP
 * span, which exceeds the offset of the outer UDP header by 8 + gtp_hdr_len
 * minus the 14-byte L2 header, so the downgrade fires for every valid header.
 * BPF_F_ADJ_ROOM_NO_CSUM_RESET would only preserve CHECKSUM_UNNECESSARY
 * validation state, which nothing after decap consumes: the packet leaves via
 * redirect, where only CHECKSUM_PARTIAL matters at transmit.
 *
 * Unlike the encap path this has no gso_segs guard, because uplink GTP-U is
 * assumed never to arrive merged. Merging it needs rx-udp-gro-forwarding
 * (net/ipv4/udp_offload.c, the NETIF_F_GRO_UDP_FWD branch of
 * udp_gro_receive), and that feature sits in NETIF_F_SOFT_FEATURES_OFF, which
 * register_netdevice transfers to hw_features but never to wanted_features
 * (net/core/dev.c), so it is off unless an operator turns it on.
 *
 * BPF_F_ADJ_ROOM_FIXED_GSO is not optional here: without it
 * bpf_skb_net_shrink refuses any non-TCP GSO frame with -ENOTSUPP
 * (net/core/filter.c). It keeps gso_size unchanged across the shrink, so if
 * the assumption above ever breaks the segments come out short by the
 * stripped header, not malformed. */
static __always_inline long ctx_decap(struct __ctx_buff *ctx, __s32 bytes,
				      __u8 inner_is_ipv6)
{
	/* Without a DECAP_L3 flag bpf_skb_net_shrink leaves skb->protocol at the
	 * outer family (net/core/filter.c), so the stack delivers a decapsulated
	 * frame to the wrong packet_type and egress picks offloads off the wrong
	 * protocol. The grow side sets it from ENCAP_L3_*, which is why encap
	 * needs no equivalent. */
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
 * BPF_F_ADJ_ROOM_FIXED_GSO suppresses the skb_decrease_gso_size that
 * bpf_skb_net_grow would otherwise apply (net/core/filter.c), so a segmented
 * frame would overshoot the original max segment size by the encap overhead
 * rather than keep the inner payload within it. That is a deliberate choice
 * of a possible MTU overshoot over a silently reduced inner MSS, and it is
 * inert on this path: the datapath drops GSO super-frames before they reach
 * encapsulation (see encap_would_be_malformed in n6_bpf.h), so the flag only
 * ever applies to frames the kernel will not segment.
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

/* Move the frame end by `delta` (negative trims, positive grows). No pull
 * needed after: bpf_skb_change_tail makes the whole skb linear and writable
 * (__bpf_try_make_writable over skb->len). */
static __always_inline long ctx_adjust_tail(struct __ctx_buff *ctx, __s32 delta)
{
	return bpf_skb_change_tail(ctx, ctx->len + delta, 0);
}

/* Copy `len` bytes at frame offset `off` into `to`; reads past the linear
 * head. */
static __always_inline long ctx_load_bytes(struct __ctx_buff *ctx, __u32 off,
					   void *to, __u32 len)
{
	return bpf_skb_load_bytes(ctx, off, to, len);
}

/* A TC skb can be CHECKSUM_PARTIAL (veth RX), where the L4 check field holds
 * only the pseudo-header sum; `bpf_l4_csum_replace` →
 * `inet_proto_csum_replace{2,4}` applies a delta correctly for every
 * ip_summed state (pseudo-header deltas need BPF_F_PSEUDO_HDR, others are
 * skipped on PARTIAL). The helper makes the skb writable, so it invalidates
 * every packet pointer: call it only after all direct field writes, and
 * re-derive pointers afterwards. */
#define CTX_L4_CSUM_VIA_HELPERS 1
static __always_inline long ctx_l4_csum_replace(struct __ctx_buff *ctx,
						__u32 off, __u64 from, __u64 to,
						__u64 flags)
{
	return bpf_l4_csum_replace(ctx, off, from, to, flags);
}

/* Logical action index for the xdp_actions statistics array, which uses the
 * XDP verdict encoding. TC verdicts collide with it (TC_ACT_SHOT == 2 ==
 * XDP_PASS), and TC folds aborted into drop (both TC_ACT_SHOT). */
static __always_inline __u32 ctx_stat_action(enum ctx_action action)
{
	switch ((int)action) {
	case TC_ACT_OK:
		return XDP_PASS;
	case TC_ACT_SHOT:
		return XDP_DROP;
	case TC_ACT_REDIRECT:
		return XDP_REDIRECT;
	}

	return XDP_ABORTED;
}

/* The kernel moves a VLAN tag out of the frame bytes before the TCX hook
 * (skb_vlan_untag), so tags are handled via metadata here and the in-band
 * branches compile out. */
#define CTX_INBAND_VLAN 0

/* QinQ guard. The kernel strips only the outer tag before the hook, so an
 * ethertype that is still a VLAN tag means a second one remains in the frame
 * bytes; the datapath hands those to the stack in every attach mode. Returns
 * 0 to proceed, > 0 to pass the frame to the stack.
 *
 * The ingress tag itself is deliberately left in place: a frame the datapath
 * does not own is returned with TC_ACT_OK and demultiplexed to its VLAN
 * sub-device by the stack, which runs after this hook. Popping here would
 * deliver ARP, ND and host traffic to the master netdev instead. Redirected
 * frames clear it in ctx_vlan_egress, before the tag they leave with is set.
 */
static __always_inline long ctx_vlan_ingress(struct __ctx_buff *ctx)
{
	if (ctx->protocol == bpf_htons(ETH_P_8021Q) ||
	    ctx->protocol == bpf_htons(ETH_P_8021AD))
		return 1;

	return 0;
}

/* Set the metadata tag the frame leaves with; on an untagged skb this writes
 * no packet bytes, and segmentation copies it to every segment. Invalidates
 * packet pointers. */
static __always_inline long ctx_vlan_egress(struct __ctx_buff *ctx,
					    int egress_vid)
{
	/* The ingress tag rides in the metadata and dev_queue_xmit would
	 * re-insert it on the egress interface, so it goes before the egress
	 * tag is set: pushing onto a tagged skb writes the old tag in-band and
	 * produces QinQ. */
	if (ctx->vlan_present && bpf_skb_vlan_pop(ctx))
		return -1;

	if (!egress_vid)
		return 0;

	return bpf_skb_vlan_push(ctx, bpf_htons(ETH_P_8021Q),
				 (__u16)egress_vid);
}

/* Transmit the frame back out its ingress interface, tagged `egress_vid`
 * (0 for untagged); TC has no XDP_TX-style verdict, the redirect helper's
 * return value is the verdict. */
static __always_inline enum ctx_action ctx_tx_back(struct __ctx_buff *ctx,
						   int egress_vid)
{
	if (ctx_vlan_egress(ctx, egress_vid) < 0)
		return CTX_ACT_ABORTED;

	return bpf_redirect(ctx->ingress_ifindex, 0);
}

/* Transmit the frame out `ifindex`, tagged `egress_vid` (0 for untagged). */
static __always_inline enum ctx_action
ctx_redirect_out(struct __ctx_buff *ctx, __u32 ifindex, int egress_vid)
{
	if (ctx_vlan_egress(ctx, egress_vid) < 0)
		return CTX_ACT_ABORTED;

	return bpf_redirect(ifindex, 0);
}
