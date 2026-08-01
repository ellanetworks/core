/**
 * SPDX-FileCopyrightText: Ella Networks Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/types.h>
#include <linux/udp.h>

#include "bpf/utils/csum.h"
#include "bpf/utils/gtp.h"
#include "bpf/utils/gtpu.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"
#include "bpf/utils/trace.h"

/* GTP-U path management: the messages the UPF answers itself, never
 * forwarding (TS 29.281 §7). Separated from the encapsulation helpers in
 * gtp.h because these rewrite a frame in place and send it back, where those
 * resize a frame in flight. */

static __always_inline void swap_ip6(struct ipv6hdr *ip6)
{
	struct in6_addr tmp;
	__builtin_memcpy(&tmp, &ip6->saddr, sizeof(struct in6_addr));
	__builtin_memcpy(&ip6->saddr, &ip6->daddr, sizeof(struct in6_addr));
	__builtin_memcpy(&ip6->daddr, &tmp, sizeof(struct in6_addr));
}

/* GTP-U Recovery information element type (TS 29.281 §8.2). */
#define GTPU_IE_RECOVERY (14)

/* An Echo Response is a 12-octet GTP-U header (mandatory header plus the
 * optional word; the S flag is set as required for Echo messages, TS 29.281
 * §5.1) followed by the mandatory Recovery IE (TV format, 2 octets). */
#define GTPU_ECHO_RESPONSE_LEN (14)

/* Answer a GTP-U Echo Request by rewriting it in place into an Echo Response
 * carrying the mandatory Recovery IE (TS 29.281 §7.2.2, Table 7.2.2-1). The
 * response repeats the request's sequence number (§7.2.2) and is emitted at a
 * fixed length, so a request bearing extension headers or a private extension
 * is answered with the canonical form; its tail is not reflected. */
static __always_inline __u32 handle_echo_request(struct packet_context *ctx)
{
	struct gtpuhdr *gtp = ctx->gtp;

	if (!ctx->eth || !ctx->udp || !gtp)
		return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

	if (!ctx->ip4 && !ctx->ip6)
		return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

	const int is_ip4 = ctx->ip4 != NULL;

	/* The sequence number is only present when the request set the S flag. */
	__be16 seq = 0;
	if (gtp->s) {
		const __u8 *opt = (const __u8 *)(gtp + 1);
		if ((const void *)(opt + sizeof(seq)) > ctx->data_end)
			return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

		__builtin_memcpy(&seq, opt, sizeof(seq));
	}

	/* Resize the frame so it ends exactly after the canonical response: a
	 * short request is grown, a longer one has its tail dropped. */
	long delta = -ctx_tail_excess(ctx->ctx_buff, ctx->data_end, gtp,
				      GTPU_ECHO_RESPONSE_LEN);
	if (delta != 0 && ctx_adjust_tail(ctx->ctx_buff, (int)delta) < 0)
		return drop_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);

	/* ctx_adjust_tail invalidates every packet pointer, and an offset
	 * saved across the call is not provably in-bounds to the verifier. Re-walk
	 * the headers from data instead: each is a bounds-checked constant step
	 * from the last, which the verifier tracks precisely. */
	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);

	struct ethhdr *eth = data;
	if ((const void *)(eth + 1) > data_end)
		return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

	/* parse_ethernet accepts both C-VLAN and S-VLAN tags (parsers.h); match both
	 * so an 802.1ad-tagged frame is not re-walked 4 octets short. */
	void *l3 = eth + 1;
	if (eth->h_proto == bpf_htons(ETH_P_8021Q) ||
	    eth->h_proto == bpf_htons(ETH_P_8021AD)) {
		struct vlan_hdr *vlan = l3;
		if ((const void *)(vlan + 1) > data_end)
			return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

		l3 = vlan + 1;
	}

	struct udphdr *udp;
	if (is_ip4) {
		struct iphdr *ip = l3;
		if ((const void *)(ip + 1) > data_end)
			return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

		/* The re-walk steps a fixed 20 octets to L4, so an IPv4 header
		 * carrying options (ihl > 5) — which parse_ip4 accepts — would be
		 * rewritten inside the options. Drop; emitting a corrupt
		 * frame; options on a GTP-U echo do not occur in practice. */
		if (ip->ihl != 5)
			return drop_with(ctx, UPF_DROP_MALFORMED_HEADER);

		udp = (struct udphdr *)(ip + 1);
	} else {
		struct ipv6hdr *ip6 = l3;
		if ((const void *)(ip6 + 1) > data_end)
			return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

		udp = (struct udphdr *)(ip6 + 1);
	}

	if ((const void *)(udp + 1) > data_end)
		return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

	__u8 *p = (__u8 *)(udp + 1);
	if ((const void *)(p + GTPU_ECHO_RESPONSE_LEN) > data_end)
		return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

	gtp = (struct gtpuhdr *)p;

	const __u16 udp_len = sizeof(*udp) + GTPU_ECHO_RESPONSE_LEN;

	*p = GTP_FLAGS; /* version 1, PT 1 */
	gtp->s = 1;
	gtp->message_type = GTPU_ECHO_RESPONSE;
	gtp->message_length =
		bpf_htons(GTPU_ECHO_RESPONSE_LEN - sizeof(struct gtpuhdr));
	gtp->teid = 0;

	/* Optional word: sequence number, N-PDU number, next-extension type. */
	__builtin_memcpy(p + 8, &seq, sizeof(seq));
	p[10] = 0;
	p[11] = 0;

	/* Recovery (TS 29.281 §8.2, TV format). The restart counter shall be set
	 * to zero by the sender and ignored by the receiver (§7.2.2). */
	p[12] = GTPU_IE_RECOVERY;
	p[13] = 0;

	swap_port(udp);
	udp->len = bpf_htons(udp_len);

	if (is_ip4) {
		struct iphdr *ip = l3;
		if ((const void *)(ip + 1) > data_end)
			return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

		swap_ip(ip);
		ip->tot_len = bpf_htons(sizeof(*ip) + udp_len);
		recompute_ipv4_csum(ip);

		/* Rewriting the header invalidated the UDP checksum; zero means
		 * "not computed" over IPv4 (RFC 768 p.2). */
		udp->check = 0;

		upf_printk("upf: send gtp echo response [ %pI4 -> %pI4 ]",
			   &ip->saddr, &ip->daddr);
	} else {
		struct ipv6hdr *ip6 = l3;
		if ((const void *)(ip6 + 1) > data_end)
			return drop_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

		swap_ip6(ip6);
		ip6->payload_len = bpf_htons(udp_len);

		/* The UDP checksum is mandatory over IPv6 (a wrong one is dropped
		 * by the receiver). */
		udp->check = 0;
		__u32 udp_off = (__u32)((__u8 *)udp - (__u8 *)data);
		int cs = udpv6_csum(&ip6->saddr, &ip6->daddr, udp_off, udp_len,
				    ctx->ctx_buff);
		if (cs < 0)
			return drop_with(ctx, UPF_DROP_INTERNAL_CSUM_FAILED);

		udp->check = (__u16)cs;
	}

	swap_mac(eth);

	return tx_back(ctx, egress_vlan_reflected(ctx));
}

/* GTP-U Error Indication information element types (TS 29.281 §8.1). */
#define GTPU_IE_TEID_DATA_I (16)
#define GTPU_IE_PEER_ADDRESS (133)

/* Reflect a GTP-U Error Indication to the sender of a G-PDU received for a TEID
 * with no PDU session, over IPv4 N3 transport (TS 29.281 §7.3.1). The message
 * carries the triggering TEID (Tunnel Endpoint Identifier Data I, §8.3) and this
 * UPF's address (GTP-U Peer Address, §8.4); the S flag is set as required for
 * Error Indication messages (§5.1). */
static __always_inline enum ctx_action
send_error_indication_ipv4(struct packet_context *ctx)
{
	struct ethhdr *eth = ctx->eth;
	struct iphdr *ip = ctx->ip4;
	struct udphdr *udp = ctx->udp;
	struct gtpuhdr *gtp = ctx->gtp;

	if (!eth || !ip || !udp || !gtp)
		return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

	/* 12-octet GTP-U header (mandatory + optional word) followed by the two
	 * mandatory IEs: TEID Data I (5) and GTP-U Peer Address (7) = 24 octets. */
	__u8 *p = (__u8 *)gtp;
	if ((const void *)(p + 24) > ctx->data_end)
		return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

	__be32 trigger_teid = gtp->teid;
	__be32 peer_addr = ip->daddr; /* destination of the triggering packet */

	swap_mac(eth);
	swap_ip(ip);
	ip->tot_len = bpf_htons(sizeof(*ip) + sizeof(*udp) + 24);
	recompute_ipv4_csum(ip);

	udp->source = bpf_htons(GTP_UDP_PORT);
	udp->dest = bpf_htons(GTP_UDP_PORT);
	udp->len = bpf_htons(sizeof(*udp) + 24);
	udp->check = 0;

	*p = GTP_FLAGS; /* version 1, PT 1 */
	gtp->s = 1;
	gtp->e = 0;
	gtp->pn = 0;
	gtp->message_type = GTPU_ERROR_INDICATION;
	gtp->message_length = bpf_htons(16); /* optional word (4) + IEs (12) */
	gtp->teid = 0;

	/* Optional word: sequence number, N-PDU number, next-extension type. */
	p[8] = 0;
	p[9] = 0;
	p[10] = 0;
	p[11] = 0;

	/* Tunnel Endpoint Identifier Data I (TS 29.281 §8.3, TV format). */
	p[12] = GTPU_IE_TEID_DATA_I;
	__builtin_memcpy(p + 13, &trigger_teid, sizeof(trigger_teid));

	/* GTP-U Peer Address (TS 29.281 §8.4, TLV; IPv4 length 4). */
	p[17] = GTPU_IE_PEER_ADDRESS;
	p[18] = 0;
	p[19] = 4;
	__builtin_memcpy(p + 20, &peer_addr, sizeof(peer_addr));

	/* Drop the trailing T-PDU so the frame ends after the IEs. */
	long trim = ctx_tail_excess(ctx->ctx_buff, ctx->data_end, p, 24);
	if (trim > 0 && ctx_adjust_tail(ctx->ctx_buff, (int)-trim) < 0)
		return drop_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);

	return tx_back(ctx, egress_vlan_reflected(ctx));
}

/* IPv6-transport counterpart of send_error_indication_ipv4 (TS 29.281 §7.3.1).
 * The GTP-U Peer Address IE carries the 16-octet IPv6 address (§8.4), and the
 * UDP checksum is mandatory over IPv6. */
static __always_inline enum ctx_action
send_error_indication_ipv6(struct packet_context *ctx)
{
	struct ethhdr *eth = ctx->eth;
	struct ipv6hdr *ip6 = ctx->ip6;
	struct udphdr *udp = ctx->udp;
	struct gtpuhdr *gtp = ctx->gtp;

	if (!eth || !ip6 || !udp || !gtp)
		return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

	/* 12-octet GTP-U header (mandatory + optional word) followed by the two
	 * mandatory IEs: TEID Data I (5) and GTP-U Peer Address (1+2+16 = 19) = 36
	 * octets. */
	__u8 *p = (__u8 *)gtp;
	if ((const void *)(p + 36) > ctx->data_end)
		return drop_with(ctx, UPF_DROP_MALFORMED_GTP);

	__be32 trigger_teid = gtp->teid;
	struct in6_addr peer_addr =
		ip6->daddr; /* destination of the triggering packet */

	swap_mac(eth);
	swap_ip6(ip6);

	const __u16 udp_len = sizeof(*udp) + 36;
	ip6->payload_len = bpf_htons(udp_len);

	udp->source = bpf_htons(GTP_UDP_PORT);
	udp->dest = bpf_htons(GTP_UDP_PORT);
	udp->len = bpf_htons(udp_len);
	udp->check = 0;

	*p = GTP_FLAGS; /* version 1, PT 1 */
	gtp->s = 1;
	gtp->e = 0;
	gtp->pn = 0;
	gtp->message_type = GTPU_ERROR_INDICATION;
	gtp->message_length = bpf_htons(28); /* optional word (4) + IEs (24) */
	gtp->teid = 0;

	/* Optional word: sequence number, N-PDU number, next-extension type. */
	p[8] = 0;
	p[9] = 0;
	p[10] = 0;
	p[11] = 0;

	/* Tunnel Endpoint Identifier Data I (TS 29.281 §8.3, TV format). */
	p[12] = GTPU_IE_TEID_DATA_I;
	__builtin_memcpy(p + 13, &trigger_teid, sizeof(trigger_teid));

	/* GTP-U Peer Address (TS 29.281 §8.4, TLV; IPv6 length 16). */
	p[17] = GTPU_IE_PEER_ADDRESS;
	p[18] = 0;
	p[19] = 16;
	__builtin_memcpy(p + 20, &peer_addr, sizeof(peer_addr));

	/* UDP checksum is mandatory over IPv6. */
	__u32 udp_off = (__u32)((__u8 *)udp - (__u8 *)ctx_data(ctx->ctx_buff));
	int csum = udpv6_csum(&ip6->saddr, &ip6->daddr, udp_off, udp_len,
			      ctx->ctx_buff);
	if (csum < 0)
		return drop_with(ctx, UPF_DROP_INTERNAL_CSUM_FAILED);
	udp->check = (__u16)csum;

	/* Drop the trailing T-PDU so the frame ends after the IEs. */
	long trim = ctx_tail_excess(ctx->ctx_buff, ctx->data_end, p, 36);
	if (trim > 0 && ctx_adjust_tail(ctx->ctx_buff, (int)-trim) < 0)
		return drop_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);

	return tx_back(ctx, egress_vlan_reflected(ctx));
}
