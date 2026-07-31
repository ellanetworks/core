/**
 * Copyright 2023 Edgecom LLC
 * SPDX-FileCopyrightText: Ella Networks Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Modified by Ella Networks.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/types.h>
#include <linux/udp.h>
#include <linux/icmp.h>

#include "bpf/utils/csum.h"
#include "bpf/utils/gtpu.h"
#include "bpf/utils/ip_addr.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"
#include "bpf/utils/pdr.h"
#include "bpf/utils/trace.h"

/* ---------------------------------------------------------------------------
 * GTP-U encapsulation overhead constants.
 *
 * IPv4 outer: IPv4 header (20) + UDP (8) + GTP-U (8) + PDU session ext (8) = 44
 * IPv6 outer: IPv6 header (40) + UDP (8) + GTP-U (8) + PDU session ext (8) = 64
 */
#define GTP_ENCAP_SIZE_IPV4                             \
	(sizeof(struct iphdr) + sizeof(struct udphdr) + \
	 sizeof(struct gtpuhdr) + 8)
#define GTP_ENCAP_SIZE_IPV6                               \
	(sizeof(struct ipv6hdr) + sizeof(struct udphdr) + \
	 sizeof(struct gtpuhdr) + 8)
/* GTP_PSC_EXT_SIZE is the GTP extension chain (gtp_hdr_ext + PDU session
 * container) included in the encap sizes above; a 4G S1-U bearer omits it. */
#define GTP_PSC_EXT_SIZE 8

/* Upper bound on the GTP-U extension-header chain the parser walks, so the
 * verifier sees a bounded loop. N3 traffic carries at most the PDU Session
 * Container; the margin tolerates a short chain. */
#define GTP_MAX_EXT_HEADERS 4

/* Upper bound on the total parsed GTP-U header length (mandatory header +
 * optional word + extension headers). Bounds the value for the verifier and
 * caps pathological chains; far above any N3 header (typically 16 octets). */
#define GTP_MAX_HDR_LEN 64

/* Deepest span the datapath parses or writes behind a single pull. Two paths
 * compete for it: the uplink GTP-U chain, and the dNAT translation of an ICMP
 * error, which reaches the L4 ports inside the quoted datagram. A pull shorter
 * than either turns a bounds check into a pass-to-stack with no counter, so
 * the bound is asserted against the parse depth it has to cover. */
/* eth 14 + VLAN 4 + IPv4 with options 60 */
#define CTX_PARSE_DEPTH_L2_L3 78
/* + UDP 8 + GTP-U 64 */
#define CTX_PARSE_DEPTH_GTPU (CTX_PARSE_DEPTH_L2_L3 + 8 + GTP_MAX_HDR_LEN)
/* + ICMP 8 + quoted IPv4 with options 60 + quoted TCP 20 */
#define CTX_PARSE_DEPTH_ICMP_QUOTE (CTX_PARSE_DEPTH_L2_L3 + 8 + 60 + 20)

_Static_assert(CTX_PULL_LEN >= CTX_PARSE_DEPTH_GTPU,
	       "CTX_PULL_LEN must cover the uplink GTP-U parse depth");
_Static_assert(CTX_PULL_LEN >= CTX_PARSE_DEPTH_ICMP_QUOTE,
	       "CTX_PULL_LEN must cover the dNAT ICMP-quote parse depth");

static __always_inline __u32 parse_gtp(struct packet_context *ctx)
{
	struct gtpuhdr *gtp = (struct gtpuhdr *)ctx->data;
	if ((const void *)(gtp + 1) > ctx->data_end)
		return -1;

	ctx->data += sizeof(*gtp);
	__u32 hdr_len = sizeof(*gtp);

	/* The optional word (sequence number, N-PDU number, next-extension-header
	 * type) is present if any of E/S/PN is set. Extension headers follow it
	 * only when E is set (TS 29.281 §5.1, §5.2). */
	if (gtp->e || gtp->s || gtp->pn) {
		struct gtp_hdr_ext *opt = (struct gtp_hdr_ext *)ctx->data;
		if ((const void *)(opt + 1) > ctx->data_end)
			return -1;

		__u8 next_ext = opt->next_ext;
		ctx->data += sizeof(struct gtp_hdr_ext);
		hdr_len += sizeof(struct gtp_hdr_ext);

		if (gtp->e) {
#pragma unroll
			for (int i = 0; i < GTP_MAX_EXT_HEADERS; i++) {
				if (next_ext == 0)
					break;

				__u8 *ext = (__u8 *)ctx->data;
				if ((const void *)(ext + 1) > ctx->data_end)
					return -1;

				/* Length is in 4-octet units; the extension header's
				 * last octet is the next-extension-header type. */
				__u32 ext_len = (__u32)ext[0] * 4;
				if (ext_len == 0 ||
				    hdr_len + ext_len > GTP_MAX_HDR_LEN ||
				    (const void *)(ext + ext_len) >
					    ctx->data_end)
					return -1;

				next_ext = ext[ext_len - 1];
				ctx->data += ext_len;
				hdr_len += ext_len;
			}

			if (next_ext != 0)
				return -1;
		}
	}

	ctx->gtp = gtp;
	ctx->gtp_hdr_len = hdr_len;
	return gtp->message_type;
}

/* Bytes to strip when decapsulating an uplink GTP-U packet, excluding any VLAN
 * tag: outer IP + UDP + the GTP-U header parse_gtp actually consumed. The
 * Ethernet header is preserved (rewritten in place), so it is not counted.
 * Returns 0 when the parsed header length is out of range. */
static __always_inline __u32 gtp_decap_size_no_vlan(
	const struct packet_context *ctx, __u8 outer_header_removal)
{
	__u32 gtp_hdr_len = ctx->gtp_hdr_len;
	if (gtp_hdr_len < sizeof(struct gtpuhdr) ||
	    gtp_hdr_len > GTP_MAX_HDR_LEN)
		return 0;

	__u32 outer_ip_size = (outer_header_removal == OHR_GTP_U_UDP_IPv6) ?
				      sizeof(struct ipv6hdr) :
				      sizeof(struct iphdr);

	return outer_ip_size + sizeof(struct udphdr) + gtp_hdr_len;
}

static __always_inline int guess_eth_protocol(const void *data)
{
	const __u8 ip_version = (*(const __u8 *)data) >> 4;
	switch (ip_version) {
	case 6: {
		return ETH_P_IPV6_BE;
	}
	case 4: {
		return ETH_P_IP_BE;
	}
	default:
		/* do nothing with non-ip packets */
		upf_printk("upf: can't process non-IP packet: %d", ip_version);
		return -1;
	}
}

static __always_inline long remove_gtp_header(struct packet_context *ctx,
					      __u8 outer_header_removal)
{
	if (!ctx->gtp) {
		upf_printk("upf: remove_gtp_header: not a gtp packet");
		return -1;
	}

	const __u32 gtp_encap_size_no_vlan =
		gtp_decap_size_no_vlan(ctx, outer_header_removal);
	if (gtp_encap_size_no_vlan == 0) {
		upf_printk("upf: remove_gtp_header: bad gtp header length");
		return -1;
	}

	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);
	struct ethhdr *eth = (struct ethhdr *)data;
	if ((const void *)(eth + 1) > data_end) {
		upf_printk("upf: remove_gtp_header: can't parse eth");
		return -1;
	}

	/* Preserve the L2 addresses; the rewritten header below carries them. */
	struct ethhdr saved_eth;
	__builtin_memcpy(&saved_eth, eth, sizeof(saved_eth));

	__u32 in_vlan_size = 0;
	if (eth->h_proto == bpf_htons(ETH_P_8021Q) ||
	    eth->h_proto == bpf_htons(ETH_P_8021AD)) {
		upf_printk("upf: remove_gtp_header: detected vlan header");
		in_vlan_size = sizeof(struct vlan_hdr);
	}
	__u32 out_vlan_size =
		(CTX_INBAND_VLAN && n6_vlan) ? sizeof(struct vlan_hdr) : 0;

	/* Strip the input VLAN tag (if any) and the outer IP/UDP/GTP headers,
	 * keeping headroom for the Ethernet header and an optional output VLAN
	 * tag. Resizing first lets every rewrite below use a fixed offset from
	 * the new packet start, which the verifier can bound even though the
	 * stripped GTP header length varies. */
	long result = ctx_decap(
		ctx->ctx_buff,
		(__s32)(in_vlan_size + gtp_encap_size_no_vlan - out_vlan_size));
	if (result)
		return result;

	data = ctx_data(ctx->ctx_buff);
	data_end = ctx_data_end(ctx->ctx_buff);

	struct ethhdr *new_eth = (struct ethhdr *)data;
	if ((const void *)(new_eth + 1) > data_end) {
		upf_printk("upf: remove_gtp_header: can't set new eth");
		return -1;
	}
	__builtin_memcpy(new_eth, &saved_eth, sizeof(*new_eth));

	if (CTX_INBAND_VLAN && n6_vlan) {
		struct vlan_hdr *vlan = (struct vlan_hdr *)(new_eth + 1);
		const __u8 *inner = (const __u8 *)(vlan + 1);
		if ((const void *)(inner + 1) > data_end) {
			upf_printk(
				"upf: remove_gtp_header: can't set new vlan");
			return -1;
		}
		int eth_proto = guess_eth_protocol(inner);
		if (eth_proto == -1)
			return -1;
		vlan->h_vlan_TCI = bpf_htons(n6_vlan & 0x0FFF);
		vlan->h_vlan_encapsulated_proto = eth_proto;
		new_eth->h_proto = bpf_htons(ETH_P_8021Q);
	} else {
		const __u8 *inner = (const __u8 *)(new_eth + 1);
		if ((const void *)(inner + 1) > data_end)
			return -1;
		int eth_proto = guess_eth_protocol(inner);
		if (eth_proto == -1)
			return -1;
		new_eth->h_proto = eth_proto;
	}

	return context_reinit(ctx, data, data_end);
}

static __always_inline void fill_ip_header(struct iphdr *ip, int saddr,
					   int daddr, __u8 tos, int tot_len)
{
	ip->version = 4;
	ip->ihl = 5; /* No options */
	ip->tos = tos;
	ip->tot_len = bpf_htons(tot_len);
	ip->id = 0; /* No fragmentation */
	ip->frag_off = 0x0040; /* Don't fragment; Fragment offset = 0 */
	ip->ttl = 64;
	ip->protocol = IPPROTO_UDP;
	ip->check = 0;
	ip->saddr = saddr;
	ip->daddr = daddr;
}

static __always_inline void fill_ip6_header(struct ipv6hdr *ip6,
					    const struct in6_addr *saddr,
					    const struct in6_addr *daddr,
					    __u8 traffic_class, int payload_len)
{
	ip6->version = 6;
	ip6->priority = traffic_class >> 4;
	ip6->flow_lbl[0] = (traffic_class & 0x0f) << 4;
	ip6->flow_lbl[1] = 0;
	ip6->flow_lbl[2] = 0;
	ip6->payload_len = bpf_htons(payload_len);
	ip6->nexthdr = IPPROTO_UDP;
	ip6->hop_limit = 64;
	__builtin_memcpy(&ip6->saddr, saddr, sizeof(struct in6_addr));
	__builtin_memcpy(&ip6->daddr, daddr, sizeof(struct in6_addr));
}

static __always_inline void fill_udp_header(struct udphdr *udp, int port,
					    int len)
{
	udp->source = bpf_htons(port);
	udp->dest = udp->source;
	udp->len = bpf_htons(len);
	udp->check = 0;
}

static __always_inline void fill_gtp_header(struct gtpuhdr *gtp, int teid,
					    int len)
{
	*(__u8 *)gtp = GTP_FLAGS;
	gtp->e = 1;
	gtp->message_type = GTPU_G_PDU;
	gtp->message_length = bpf_htons(len);
	gtp->teid = bpf_htonl(teid);
}

/* fill_gtp_header_plain writes a bare G-PDU header with no extension headers
 * (E=0), as used on 4G S1-U where the PDU Session Container is absent
 * (TS 29.281; the container is N3/N9-only per TS 38.415). GTP_FLAGS already
 * clears the E/S/PN bits. */
static __always_inline void fill_gtp_header_plain(struct gtpuhdr *gtp, int teid,
						  int len)
{
	*(__u8 *)gtp = GTP_FLAGS;
	gtp->message_type = GTPU_G_PDU;
	gtp->message_length = bpf_htons(len);
	gtp->teid = bpf_htonl(teid);
}

static __always_inline void fill_gtp_ext_header(struct gtp_hdr_ext *gtp_ext)
{
	gtp_ext->sqn = 0;
	gtp_ext->npdu = 0;
	gtp_ext->next_ext = GTPU_EXT_TYPE_PDU_SESSION_CONTAINER;
}

static __always_inline void
fill_gtp_ext_header_psc(struct gtp_hdr_ext_pdu_session_container *gtp_ext,
			int qfi, int pdu_type)
{
	gtp_ext->length = 1;
	gtp_ext->pdu_type = pdu_type;
	gtp_ext->spare1 = 0;
	gtp_ext->spare2 = 0;
	gtp_ext->rqi = 0;
	gtp_ext->qfi = qfi;
	gtp_ext->next_ext = 0;
}

static __always_inline __u32
add_gtp_over_ip4_headers(struct packet_context *ctx, int saddr, int daddr,
			 __u8 tos, __u8 qfi, int teid)
{
	static const size_t gtp_ext_hdr_size =
		sizeof(struct gtp_hdr_ext) +
		sizeof(struct gtp_hdr_ext_pdu_session_container);
	static const size_t gtp_full_hdr_size =
		sizeof(struct gtpuhdr) + gtp_ext_hdr_size;
	static const size_t gtp_encap_size_no_vlan = sizeof(struct iphdr) +
						     sizeof(struct udphdr) +
						     gtp_full_hdr_size;
	size_t n3_vlan_hdr_size = 0;
	if (CTX_INBAND_VLAN && n3_vlan) {
		n3_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	size_t n6_vlan_hdr_size = 0;
	if (ctx->vlan) {
		n6_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	const size_t gtp_encap_size =
		n3_vlan_hdr_size - n6_vlan_hdr_size + gtp_encap_size_no_vlan;

	int ip_packet_len = 0;
	if (ctx->ip4) {
		ip_packet_len = bpf_ntohs(ctx->ip4->tot_len);
	} else if (ctx->ip6) {
		ip_packet_len = bpf_ntohs(ctx->ip6->payload_len) +
				sizeof(struct ipv6hdr);
	} else {
		return -1;
	}

	int result =
		ctx_encap(ctx->ctx_buff, gtp_encap_size, CTX_ENCAP_FLAGS_IPV4);
	if (result) {
		return -1;
	}

	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);

	struct ethhdr *orig_eth = ctx_encap_saved_eth(data, gtp_encap_size);
	if ((const void *)(orig_eth + 1) > data_end) {
		return -1;
	}

	struct ethhdr *eth = (struct ethhdr *)data;
	__builtin_memcpy(eth, orig_eth, sizeof(*eth));
	eth->h_proto = bpf_htons(ETH_P_IP);

	struct iphdr *ip = (struct iphdr *)(eth + 1);
	if ((const void *)(ip + 1) > data_end) {
		return -1;
	}

	struct vlan_hdr *vlan = NULL;
	if (CTX_INBAND_VLAN && n3_vlan) {
		eth->h_proto = bpf_htons(ETH_P_8021Q);
		vlan = (struct vlan_hdr *)ip;
		vlan->h_vlan_TCI = bpf_htons(n3_vlan & 0x0FFF);
		vlan->h_vlan_encapsulated_proto = bpf_htons(ETH_P_IP);
		ip = (struct iphdr *)((void *)ip + sizeof(struct vlan_hdr));
		if ((const void *)(ip + 1) > data_end) {
			return -1;
		}
	}

	/* Add the outer IP header */
	fill_ip_header(ip, saddr, daddr, tos,
		       ip_packet_len + gtp_encap_size_no_vlan);

	/* Add the UDP header */
	struct udphdr *udp = (struct udphdr *)(ip + 1);
	if ((const void *)(udp + 1) > data_end)
		return -1;

	fill_udp_header(udp, GTP_UDP_PORT,
			ip_packet_len + sizeof(*udp) + gtp_full_hdr_size);

	/* Add the GTP header */
	struct gtpuhdr *gtp = (struct gtpuhdr *)(udp + 1);
	if ((const void *)(gtp + 1) > data_end)
		return -1;

	fill_gtp_header(gtp, teid, gtp_ext_hdr_size + ip_packet_len);

	/* Add the GTP ext header */
	struct gtp_hdr_ext *gtp_ext = (struct gtp_hdr_ext *)(gtp + 1);
	if ((const void *)(gtp_ext + 1) > data_end)
		return -1;

	fill_gtp_ext_header(gtp_ext);

	/* Add the GTP PDU session container header */
	struct gtp_hdr_ext_pdu_session_container *gtp_psc =
		(struct gtp_hdr_ext_pdu_session_container *)(gtp_ext + 1);
	if ((const void *)(gtp_psc + 1) > data_end)
		return -1;

	fill_gtp_ext_header_psc(gtp_psc, qfi,
				PDU_SESSION_CONTAINER_PDU_TYPE_DL_PSU);

	ip->check = ipv4_csum(ip, sizeof(*ip));

	/* GTP-U tunnel outer UDP, IPv4: RFC 768 allows check=0. TS 29.281
	 * §4.4 only constrains the IPv6 case (forbids zero except when the
	 * peer is known to accept it), which is handled in the IPv6 branch. */
	udp->check = 0;

	/* Update packet pointers */
	context_set_ip4(ctx, ctx_data(ctx->ctx_buff),
			ctx_data_end(ctx->ctx_buff), eth, vlan, ip, udp, gtp);
	return 0;
}

/* add_gtp_over_ip4_headers_s1u encapsulates into a plain GTP-U/UDP/IPv4 G-PDU
 * with no PDU Session Container (4G S1-U). It mirrors add_gtp_over_ip4_headers
 * exactly but for the 8-byte-smaller header (no GTP extension chain). */
static __always_inline __u32 add_gtp_over_ip4_headers_s1u(
	struct packet_context *ctx, int saddr, int daddr, __u8 tos, int teid)
{
	static const size_t gtp_full_hdr_size = sizeof(struct gtpuhdr);
	static const size_t gtp_encap_size_no_vlan = sizeof(struct iphdr) +
						     sizeof(struct udphdr) +
						     gtp_full_hdr_size;
	size_t n3_vlan_hdr_size = 0;
	if (CTX_INBAND_VLAN && n3_vlan) {
		n3_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	size_t n6_vlan_hdr_size = 0;
	if (ctx->vlan) {
		n6_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	const size_t gtp_encap_size =
		n3_vlan_hdr_size - n6_vlan_hdr_size + gtp_encap_size_no_vlan;

	int ip_packet_len = 0;
	if (ctx->ip4) {
		ip_packet_len = bpf_ntohs(ctx->ip4->tot_len);
	} else if (ctx->ip6) {
		ip_packet_len = bpf_ntohs(ctx->ip6->payload_len) +
				sizeof(struct ipv6hdr);
	} else {
		return -1;
	}

	int result =
		ctx_encap(ctx->ctx_buff, gtp_encap_size, CTX_ENCAP_FLAGS_IPV4);
	if (result) {
		return -1;
	}

	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);

	struct ethhdr *orig_eth = ctx_encap_saved_eth(data, gtp_encap_size);
	if ((const void *)(orig_eth + 1) > data_end) {
		return -1;
	}

	struct ethhdr *eth = (struct ethhdr *)data;
	__builtin_memcpy(eth, orig_eth, sizeof(*eth));
	eth->h_proto = bpf_htons(ETH_P_IP);

	struct iphdr *ip = (struct iphdr *)(eth + 1);
	if ((const void *)(ip + 1) > data_end) {
		return -1;
	}

	struct vlan_hdr *vlan = NULL;
	if (CTX_INBAND_VLAN && n3_vlan) {
		eth->h_proto = bpf_htons(ETH_P_8021Q);
		vlan = (struct vlan_hdr *)ip;
		vlan->h_vlan_TCI = bpf_htons(n3_vlan & 0x0FFF);
		vlan->h_vlan_encapsulated_proto = bpf_htons(ETH_P_IP);
		ip = (struct iphdr *)((void *)ip + sizeof(struct vlan_hdr));
		if ((const void *)(ip + 1) > data_end) {
			return -1;
		}
	}

	fill_ip_header(ip, saddr, daddr, tos,
		       ip_packet_len + gtp_encap_size_no_vlan);

	struct udphdr *udp = (struct udphdr *)(ip + 1);
	if ((const void *)(udp + 1) > data_end)
		return -1;

	fill_udp_header(udp, GTP_UDP_PORT,
			ip_packet_len + sizeof(*udp) + gtp_full_hdr_size);

	struct gtpuhdr *gtp = (struct gtpuhdr *)(udp + 1);
	if ((const void *)(gtp + 1) > data_end)
		return -1;

	fill_gtp_header_plain(gtp, teid, ip_packet_len);

	ip->check = ipv4_csum(ip, sizeof(*ip));
	udp->check = 0;

	context_set_ip4(ctx, ctx_data(ctx->ctx_buff),
			ctx_data_end(ctx->ctx_buff), eth, vlan, ip, udp, gtp);
	return 0;
}

static __always_inline void update_gtp_tunnel(struct packet_context *ctx,
					      struct iphdr *ip4, int srcip,
					      int dstip, __u8 tos, int teid)
{
	ctx->gtp->teid = bpf_htonl(teid);
	ip4->saddr = srcip;
	ip4->daddr = dstip;
	ip4->check = 0;
	ip4->check = ipv4_csum(ip4, sizeof(*ip4));
}

static __always_inline void update_gtp_tunnel_ipv6(struct packet_context *ctx,
						   struct ipv6hdr *ip6,
						   const struct in6_addr *srcip,
						   const struct in6_addr *dstip,
						   int teid)
{
	ctx->gtp->teid = bpf_htonl(teid);
	__builtin_memcpy(&ip6->saddr, srcip, sizeof(struct in6_addr));
	__builtin_memcpy(&ip6->daddr, dstip, sizeof(struct in6_addr));
	/* IPv6 has no header checksum */
}

static __always_inline __u32 add_gtp_over_ip6_headers(
	struct packet_context *ctx, const struct in6_addr *saddr,
	const struct in6_addr *daddr, __u8 traffic_class, __u8 qfi, int teid)
{
	static const size_t gtp_ext_hdr_size =
		sizeof(struct gtp_hdr_ext) +
		sizeof(struct gtp_hdr_ext_pdu_session_container);
	static const size_t gtp_full_hdr_size =
		sizeof(struct gtpuhdr) + gtp_ext_hdr_size;
	static const size_t gtp_encap_size_no_vlan = sizeof(struct ipv6hdr) +
						     sizeof(struct udphdr) +
						     gtp_full_hdr_size;
	size_t n3_vlan_hdr_size = 0;
	if (CTX_INBAND_VLAN && n3_vlan) {
		n3_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	size_t n6_vlan_hdr_size = 0;
	if (ctx->vlan) {
		n6_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	const size_t gtp_encap_size =
		n3_vlan_hdr_size - n6_vlan_hdr_size + gtp_encap_size_no_vlan;

	int ip_packet_len = 0;
	if (ctx->ip4) {
		ip_packet_len = bpf_ntohs(ctx->ip4->tot_len);
	} else if (ctx->ip6) {
		ip_packet_len = bpf_ntohs(ctx->ip6->payload_len) +
				sizeof(struct ipv6hdr);
	} else {
		upf_printk("upf: not ip4 or ip6?");
		return -1;
	}

	int result =
		ctx_encap(ctx->ctx_buff, gtp_encap_size, CTX_ENCAP_FLAGS_IPV6);
	if (result) {
		upf_printk("upf: could not adjust head");
		return -1;
	}

	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);

	struct ethhdr *orig_eth = ctx_encap_saved_eth(data, gtp_encap_size);
	if ((const void *)(orig_eth + 1) > data_end) {
		upf_printk("upf: eth overflows data_end");
		return -1;
	}

	struct ethhdr *eth = (struct ethhdr *)data;
	__builtin_memcpy(eth, orig_eth, sizeof(*eth));
	eth->h_proto = bpf_htons(ETH_P_IPV6);

	struct ipv6hdr *ip6 = (struct ipv6hdr *)(eth + 1);
	if ((const void *)(ip6 + 1) > data_end) {
		upf_printk("upf: ip6 overflows data_end");
		return -1;
	}

	struct vlan_hdr *vlan = NULL;
	if (CTX_INBAND_VLAN && n3_vlan) {
		upf_printk("upf: including vlan header for n3");
		eth->h_proto = bpf_htons(ETH_P_8021Q);
		vlan = (struct vlan_hdr *)ip6;
		vlan->h_vlan_TCI = bpf_htons(n3_vlan & 0x0FFF);
		vlan->h_vlan_encapsulated_proto = bpf_htons(ETH_P_IPV6);
		ip6 = (struct ipv6hdr *)((void *)ip6 + sizeof(struct vlan_hdr));
		if ((const void *)(ip6 + 1) > data_end) {
			upf_printk("upf: ip6 overflows data_end");
			return -1;
		}
	}

	/* IPv6 payload_len = everything after the IPv6 header */
	int ipv6_payload_len =
		ip_packet_len + sizeof(struct udphdr) + gtp_full_hdr_size;
	fill_ip6_header(ip6, saddr, daddr, traffic_class, ipv6_payload_len);

	/* Add the UDP header */
	struct udphdr *udp = (struct udphdr *)(ip6 + 1);
	if ((const void *)(udp + 1) > data_end) {
		upf_printk("upf: udp overflows data_end");
		return -1;
	}

	fill_udp_header(udp, GTP_UDP_PORT,
			ip_packet_len + sizeof(*udp) + gtp_full_hdr_size);

	/* Add the GTP header */
	struct gtpuhdr *gtp = (struct gtpuhdr *)(udp + 1);
	if ((const void *)(gtp + 1) > data_end) {
		upf_printk("upf: gtp overflows data_end");
		return -1;
	}

	fill_gtp_header(gtp, teid, gtp_ext_hdr_size + ip_packet_len);

	/* Add the GTP ext header */
	struct gtp_hdr_ext *gtp_ext = (struct gtp_hdr_ext *)(gtp + 1);
	if ((const void *)(gtp_ext + 1) > data_end) {
		upf_printk("upf: gtp_ext overflows data_end");
		return -1;
	}

	fill_gtp_ext_header(gtp_ext);

	/* Add the GTP PDU session container header */
	struct gtp_hdr_ext_pdu_session_container *gtp_psc =
		(struct gtp_hdr_ext_pdu_session_container *)(gtp_ext + 1);
	if ((const void *)(gtp_psc + 1) > data_end) {
		upf_printk("upf: gtp_psc overflows data_end");
		return -1;
	}

	fill_gtp_ext_header_psc(gtp_psc, qfi,
				PDU_SESSION_CONTAINER_PDU_TYPE_DL_PSU);

	/* GTP-U over IPv6 requires UDP checksum (RFC 6936) */
	if (ip6) {
		__u32 udp_off =
			(__u32)((__u8 *)udp - (__u8 *)ctx_data(ctx->ctx_buff));
		int csum_ret = udpv6_csum(&ip6->saddr, &ip6->daddr, udp_off,
					  bpf_ntohs(udp->len), ctx->ctx_buff);
		if (csum_ret < 0) {
			upf_printk("upf: udpv6_csum failed");
			return -1;
		}
		udp->check = (__u16)csum_ret;
	}

	/* Update packet pointers */
	context_set_ip6(ctx, ctx_data(ctx->ctx_buff),
			ctx_data_end(ctx->ctx_buff), eth, vlan, ip6, udp, gtp);
	return 0;
}

/* add_gtp_over_ip6_headers_s1u encapsulates into a plain GTP-U/UDP/IPv6 G-PDU
 * with no PDU Session Container (4G S1-U). It mirrors add_gtp_over_ip6_headers
 * but for the 8-byte-smaller header (no GTP extension chain). */
static __always_inline __u32 add_gtp_over_ip6_headers_s1u(
	struct packet_context *ctx, const struct in6_addr *saddr,
	const struct in6_addr *daddr, __u8 traffic_class, int teid)
{
	static const size_t gtp_full_hdr_size = sizeof(struct gtpuhdr);
	static const size_t gtp_encap_size_no_vlan = sizeof(struct ipv6hdr) +
						     sizeof(struct udphdr) +
						     gtp_full_hdr_size;
	size_t n3_vlan_hdr_size = 0;
	if (CTX_INBAND_VLAN && n3_vlan) {
		n3_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	size_t n6_vlan_hdr_size = 0;
	if (ctx->vlan) {
		n6_vlan_hdr_size += sizeof(struct vlan_hdr);
	}
	const size_t gtp_encap_size =
		n3_vlan_hdr_size - n6_vlan_hdr_size + gtp_encap_size_no_vlan;

	int ip_packet_len = 0;
	if (ctx->ip4) {
		ip_packet_len = bpf_ntohs(ctx->ip4->tot_len);
	} else if (ctx->ip6) {
		ip_packet_len = bpf_ntohs(ctx->ip6->payload_len) +
				sizeof(struct ipv6hdr);
	} else {
		return -1;
	}

	int result =
		ctx_encap(ctx->ctx_buff, gtp_encap_size, CTX_ENCAP_FLAGS_IPV6);
	if (result) {
		return -1;
	}

	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);

	struct ethhdr *orig_eth = ctx_encap_saved_eth(data, gtp_encap_size);
	if ((const void *)(orig_eth + 1) > data_end) {
		return -1;
	}

	struct ethhdr *eth = (struct ethhdr *)data;
	__builtin_memcpy(eth, orig_eth, sizeof(*eth));
	eth->h_proto = bpf_htons(ETH_P_IPV6);

	struct ipv6hdr *ip6 = (struct ipv6hdr *)(eth + 1);
	if ((const void *)(ip6 + 1) > data_end) {
		return -1;
	}

	struct vlan_hdr *vlan = NULL;
	if (CTX_INBAND_VLAN && n3_vlan) {
		eth->h_proto = bpf_htons(ETH_P_8021Q);
		vlan = (struct vlan_hdr *)ip6;
		vlan->h_vlan_TCI = bpf_htons(n3_vlan & 0x0FFF);
		vlan->h_vlan_encapsulated_proto = bpf_htons(ETH_P_IPV6);
		ip6 = (struct ipv6hdr *)((void *)ip6 + sizeof(struct vlan_hdr));
		if ((const void *)(ip6 + 1) > data_end) {
			return -1;
		}
	}

	int ipv6_payload_len =
		ip_packet_len + sizeof(struct udphdr) + gtp_full_hdr_size;
	fill_ip6_header(ip6, saddr, daddr, traffic_class, ipv6_payload_len);

	struct udphdr *udp = (struct udphdr *)(ip6 + 1);
	if ((const void *)(udp + 1) > data_end) {
		return -1;
	}

	fill_udp_header(udp, GTP_UDP_PORT,
			ip_packet_len + sizeof(*udp) + gtp_full_hdr_size);

	struct gtpuhdr *gtp = (struct gtpuhdr *)(udp + 1);
	if ((const void *)(gtp + 1) > data_end) {
		return -1;
	}

	fill_gtp_header_plain(gtp, teid, ip_packet_len);

	/* GTP-U over IPv6 requires UDP checksum (RFC 6936) */
	if (ip6) {
		__u32 udp_off =
			(__u32)((__u8 *)udp - (__u8 *)ctx_data(ctx->ctx_buff));
		int csum_ret = udpv6_csum(&ip6->saddr, &ip6->daddr, udp_off,
					  bpf_ntohs(udp->len), ctx->ctx_buff);
		if (csum_ret < 0) {
			return -1;
		}
		udp->check = (__u16)csum_ret;
	}

	context_set_ip6(ctx, ctx_data(ctx->ctx_buff),
			ctx_data_end(ctx->ctx_buff), eth, vlan, ip6, udp, gtp);
	return 0;
}
