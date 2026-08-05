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
#include <linux/types.h>
#include <stdbool.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <linux/icmp.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <linux/tcp.h>

#include "bpf/utils/common.h"
#include "bpf/utils/frag.h"
#include "bpf/utils/ip_addr.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/trace.h"
#include <sys/cdefs.h>

#define ETH_P_IPV6_BE 0xDD86
#define ETH_P_IP_BE 0x0008

static __always_inline int parse_ethernet(struct packet_context *ctx)
{
	struct ethhdr *eth = (struct ethhdr *)ctx->data;
	if ((const void *)(eth + 1) > ctx->data_end) {
		return -1;
	}

	ctx->data += sizeof(*eth);
	ctx->eth = eth;
	ctx->vlan = NULL;

	if (eth->h_proto == bpf_htons(ETH_P_8021Q) ||
	    eth->h_proto == bpf_htons(ETH_P_8021AD)) {
		struct vlan_hdr *vlan = (struct vlan_hdr *)ctx->data;

		if ((const void *)(ctx->data + sizeof(*vlan)) > ctx->data_end) {
			return -1;
		}

		ctx->data += sizeof(*vlan);
		ctx->vlan = vlan;
		return bpf_ntohs(vlan->h_vlan_encapsulated_proto);
	}

	return bpf_ntohs(eth->h_proto);
}

/* frag_off without the flag bits. IP4_FRAG_MASK in nat_ct.h is the wider test
 * that also catches the more-fragments flag. */
#define IP4_FRAG_OFFSET_MASK 0x1FFF
#define IP4_MORE_FRAGMENTS 0x2000

static __always_inline struct iphdr *
detect_ip4_header(struct packet_context *ctx)
{
	struct iphdr *ip4 = (struct iphdr *)ctx->data;
	if ((const void *)(ip4 + 1) > ctx->data_end) {
		return NULL;
	}
	return ip4;
}

static __always_inline int parse_ip4(struct packet_context *ctx)
{
	struct iphdr *ip4 = detect_ip4_header(ctx);
	if (!ip4) {
		return -1;
	}

	/* Below the 20-byte minimum the L4 header would sit inside the IP
	 * header (RFC 791 §3.1). */
	if (ip4->ihl < 5) {
		return -1;
	}

	ctx->data += ip4->ihl * 4; /* header + options */
	ctx->ip4 = ip4;
	ctx->l3_hdr_len = ip4->ihl * 4;
	ctx->l4_proto = ip4->protocol;

	const __u16 frag_off = bpf_ntohs(ip4->frag_off);

	ctx->is_fragment = (frag_off & (IP4_FRAG_OFFSET_MASK | IP4_MORE_FRAGMENTS)) != 0;

	/* Offset field alone (RFC 791 §3.1): more-fragments also marks the first
	 * fragment, which does carry the header. */
	if (frag_off & IP4_FRAG_OFFSET_MASK)
		ctx->l4_unavailable = 1;

	/* RFC 3128 §3: both shapes move the TCP flags out of the first
	 * fragment, past a filter reading them. */
	if (ip4->protocol == IPPROTO_TCP) {
		const __u16 offset = frag_off & IP4_FRAG_OFFSET_MASK;

		if (offset == 1) {
			set_drop_reason(ctx, UPF_DROP_FRAGMENT_MALFORMED);
			return -1;
		}

		if (offset == 0 && (frag_off & IP4_MORE_FRAGMENTS) &&
		    bounded_u16(bpf_ntohs(ip4->tot_len)) <
			    (__u32)ctx->l3_hdr_len + sizeof(struct tcphdr)) {
			set_drop_reason(ctx, UPF_DROP_FRAGMENT_MALFORMED);
			return -1;
		}
	}

	return ip4->protocol;
}

/* RFC 8200 §4.5. The uapi headers do not export this one. */
struct ipv6_frag_hdr {
	__u8 nexthdr;
	__u8 reserved;
	__be16 frag_off;
	__be32 identification;
} __attribute__((packed));

/* The low three bits are reserved bits and the more-fragments flag. */
#define IPV6_FRAG_OFFSET_MASK 0xfff8
#define IPV6_MORE_FRAGMENTS 0x0001

/* RFC 8200 §4.1. ESP and IPPROTO_NONE end the chain: no readable upper
 * layer. */
static __always_inline bool ipv6_is_ext_hdr(__u8 nexthdr)
{
	switch (nexthdr) {
	case IPPROTO_HOPOPTS:
	case IPPROTO_ROUTING:
	case IPPROTO_DSTOPTS:
	case IPPROTO_FRAGMENT:
	case IPPROTO_AH:
		return true;
	default:
		return false;
	}
}

/* The outer IPv6 header of a GTP-U tunnel. No chain walk: nothing filters on
 * the tunnel transport. */
static __always_inline int parse_ip6_transport(struct packet_context *ctx)
{
	struct ipv6hdr *ip6 = (struct ipv6hdr *)ctx->data;
	if ((const void *)(ip6 + 1) > ctx->data_end) {
		return -1;
	}

	ctx->data += sizeof(*ip6);
	ctx->ip6 = ip6;
	ctx->l3_hdr_len = sizeof(*ip6);
	ctx->l4_proto = ip6->nexthdr;

	return ip6->nexthdr;
}

static __always_inline bool ipv6_load_l4_ports(struct __ctx_buff *ctx_buff,
					       __u32 l4_off, __u8 proto,
					       __u16 *sport, __u16 *dport)
{
	__be16 ports[2];

	if (proto != IPPROTO_TCP && proto != IPPROTO_UDP)
		return true;

	if (ctx_load_bytes(ctx_buff, l4_off, ports, sizeof(ports)) < 0)
		return false;

	*sport = bpf_ntohs(ports[0]);
	*dport = bpf_ntohs(ports[1]);

	return true;
}

struct ipv6_chain {
	__u16 l3_hdr_len;
	__u16 sport;
	__u16 dport;
	__u8 proto;
	/* Upper-layer header exists but is not in this packet. */
	__u8 unavailable;
	/* Chain truncated, malformed, or longer than the walk covers. */
	__u8 invalid;
	/* A fragment of a larger datagram, first or later. */
	__u8 is_fragment;
	__be32 id;
};

/* Global, so it is verified once: inlined, its paths are re-explored for every
 * state the caller reaches, which put upf_downlink_func over the verifier's
 * instruction budget. That fixes the scalar-only interface, and the offset
 * form: a packet pointer cannot cross the call, and one advanced by a variable
 * length leaves each downstream state with its own range.
 *
 * Mixing BPF-to-BPF calls with tail calls needs kernel 5.10; we require 6.8. */
__noinline __weak int ipv6_walk_chain(struct __ctx_buff *ctx_buff,
				      __u32 l3_off, __u32 first_nexthdr,
				      struct ipv6_chain *out)
{
	/* A global function's pointer argument is nullable to the verifier. */
	if (!out)
		return -1;

	const __u32 chain_off = l3_off + sizeof(struct ipv6hdr);
	__u32 chain_len = 0;
	__u8 nexthdr = (__u8)first_nexthdr;

	out->sport = 0;
	out->dport = 0;
	out->proto = IPPROTO_NONE;
	out->unavailable = 0;
	out->invalid = 0;
	out->is_fragment = 0;
	out->id = 0;

#pragma clang loop unroll(disable)
	for (int i = 0; i < IPV6_MAX_EXT_HEADERS; i++) {
		/* RFC 8200 §4.7: the chain ends and nothing follows it. */
		if (nexthdr == IPPROTO_NONE)
			goto no_upper_layer;

		if (!ipv6_is_ext_hdr(nexthdr))
			goto resolved;

		struct ipv6_opt_hdr ext;

		if (ctx_load_bytes(ctx_buff, chain_off + chain_len, &ext,
				   sizeof(ext)) < 0)
			goto unparsable;

		__u32 ext_len;

		if (nexthdr == IPPROTO_FRAGMENT) {
			struct ipv6_frag_hdr frag;

			if (ctx_load_bytes(ctx_buff, chain_off + chain_len,
					   &frag, sizeof(frag)) < 0)
				goto unparsable;

			/* RFC 6946: offset 0 with M clear is a whole datagram. */
			if (frag.frag_off &
			    bpf_htons(IPV6_FRAG_OFFSET_MASK | IPV6_MORE_FRAGMENTS))
				out->is_fragment = 1;

			out->id = frag.identification;

			/* Past this the bytes are payload. The fragment
			 * header's next-header is the same in every fragment so
			 * it still names the protocol — unless it names another
			 * extension header, which is in the payload. */
			if (frag.frag_off & bpf_htons(IPV6_FRAG_OFFSET_MASK)) {
				out->unavailable = 1;
				chain_len += sizeof(frag);
				nexthdr = frag.nexthdr;

				if (ipv6_is_ext_hdr(nexthdr) ||
				    nexthdr == IPPROTO_NONE)
					goto unparsable;

				goto resolved;
			}

			ext_len = sizeof(frag);
		} else if (nexthdr == IPPROTO_AH) {
			/* RFC 4302 §2.2: length in 4-octet units, less two. */
			ext_len = ((__u32)ext.hdrlen + 2) * 4;
		} else {
			/* RFC 8200: length in 8-octet units, not counting the
			 * first. */
			ext_len = ((__u32)ext.hdrlen + 1) * 8;
		}

		if (ext_len < sizeof(ext) ||
		    chain_len + ext_len > IPV6_MAX_EXT_CHAIN_LEN)
			goto unparsable;

		nexthdr = ext.nexthdr;
		chain_len += ext_len;
	}

	/* Loop ran out before the chain did. */
	if (ipv6_is_ext_hdr(nexthdr))
		goto unparsable;

resolved:
	out->l3_hdr_len = sizeof(struct ipv6hdr) + chain_len;
	out->proto = nexthdr;

	if (!out->unavailable &&
	    !ipv6_load_l4_ports(ctx_buff, chain_off + chain_len, nexthdr,
				&out->sport, &out->dport))
		out->unavailable = 1;

	return 0;

no_upper_layer:
	out->l3_hdr_len = sizeof(struct ipv6hdr) + chain_len;
	out->proto = IPPROTO_NONE;
	out->unavailable = 1;

	return 0;

unparsable:
	out->invalid = 1;
	out->unavailable = 1;
	out->proto = IPPROTO_NONE;
	out->l3_hdr_len = sizeof(struct ipv6hdr) + chain_len;

	return 0;
}

/* Reported on the context, not as a parse failure: the packet is still IPv6 and
 * may belong to a session, which only the owning stage can decide. */
static __always_inline void ipv6_walk_ext_chain(struct packet_context *ctx,
						__u8 nexthdr, __u32 l3_off)
{
	/* The verifier checks caller and callee separately: the callee's writes
	 * do not count as the caller initializing this. */
	struct ipv6_chain chain = {};

	ctx->l4_resolved = 1;

	if (ipv6_walk_chain(ctx->ctx_buff, l3_off, nexthdr, &chain) < 0) {
		ctx->exthdr_invalid = 1;
		ctx->l4_unavailable = 1;
		ctx->l4_proto = IPPROTO_NONE;
		ctx->l3_hdr_len = sizeof(struct ipv6hdr);

		return;
	}

	ctx->l3_hdr_len = chain.l3_hdr_len;
	ctx->l4_proto = chain.proto;
	ctx->l4_sport = chain.sport;
	ctx->l4_dport = chain.dport;
	ctx->l4_unavailable = chain.unavailable ? 1 : 0;
	ctx->exthdr_invalid = chain.invalid ? 1 : 0;
	ctx->is_fragment = chain.is_fragment ? 1 : 0;
	ctx->frag_id6 = chain.id;

	/* Recording waits for a session; recovering cannot, the ports are what
	 * the SDF rules match on. */
	frag_resolve6(ctx);
}

/* The subscriber-packet parse: resolves the upper-layer protocol and ports a
 * scoped SDF rule matches on (RFC 7045). */
static __always_inline int parse_ip6(struct packet_context *ctx)
{
	int nexthdr = parse_ip6_transport(ctx);
	if (nexthdr < 0) {
		return -1;
	}

	if (nexthdr == IPPROTO_NONE) {
		ctx->l4_unavailable = 1;
		ctx->l4_resolved = 1;

		return nexthdr;
	}

	if (!ipv6_is_ext_hdr((__u8)nexthdr)) {
		return nexthdr;
	}

	ipv6_walk_ext_chain(ctx, (__u8)nexthdr,
			    ctx_frame_offset(ctx->ctx_buff, ctx->ip6));

	return ctx->l4_proto;
}

static __always_inline struct udphdr *
detect_udp_header(struct packet_context *ctx, int offset)
{
	struct udphdr *udp = (struct udphdr *)(ctx->data + offset);
	if ((const void *)(udp + 1) > ctx->data_end) {
		return NULL;
	}
	return udp;
}

static __always_inline int parse_udp(struct packet_context *ctx)
{
	if (ctx->udp)
		return bpf_ntohs(ctx->udp->dest);

	struct udphdr *udp = detect_udp_header(ctx, 0);
	if (!udp) {
		return -1;
	}

	ctx->data += sizeof(*udp);
	ctx->udp = udp;
	ctx->l4_sport = bpf_ntohs(udp->source);
	ctx->l4_dport = bpf_ntohs(udp->dest);

	return bpf_ntohs(udp->dest);
}

static __always_inline struct tcphdr *
detect_tcp_header(struct packet_context *ctx, int offset)
{
	struct tcphdr *tcp = (struct tcphdr *)(ctx->data + offset);
	if ((const void *)(tcp + 1) > ctx->data_end) {
		return NULL;
	}
	return tcp;
}

/* RFC 792 guarantees only 8 quoted octets: the port pair, not the checksum. */
static __always_inline struct tcphdr *
detect_tcp_ports(struct packet_context *ctx, int offset)
{
	struct tcphdr *tcp = (struct tcphdr *)(ctx->data + offset);
	if ((const void *)((__u8 *)tcp + 8) > ctx->data_end) {
		return NULL;
	}
	return tcp;
}

/* The checksum ends at byte 18, short of the full header. */
static __always_inline struct tcphdr *
detect_tcp_check(struct packet_context *ctx, int offset)
{
	struct tcphdr *tcp = (struct tcphdr *)(ctx->data + offset);
	if ((const void *)((__u8 *)tcp + 18) > ctx->data_end) {
		return NULL;
	}
	return tcp;
}

static __always_inline struct icmphdr *
detect_icmp_header(struct packet_context *ctx, int offset)
{
	struct icmphdr *icmp = (struct icmphdr *)(ctx->data + offset);
	if ((const void *)(icmp + 1) > ctx->data_end) {
		return NULL;
	}
	return icmp;
}

static __always_inline int parse_tcp(struct packet_context *ctx)
{
	if (ctx->tcp)
		return bpf_ntohs(ctx->tcp->dest);

	struct tcphdr *tcp = detect_tcp_header(ctx, 0);
	if (!tcp) {
		return -1;
	}

	// TODO: parse header lenght correctly (tcp options)

	ctx->data += sizeof(*tcp);
	ctx->tcp = tcp;
	ctx->l4_sport = bpf_ntohs(tcp->source);
	ctx->l4_dport = bpf_ntohs(tcp->dest);

	return bpf_ntohs(tcp->dest);
}

static __always_inline int parse_icmp(struct packet_context *ctx)
{
	if (ctx->icmp)
		return ctx->icmp->type;

	/* As in parse_l4. */
	if (ctx->l4_resolved || ctx->l4_unavailable)
		return -1;

	struct icmphdr *icmp = (struct icmphdr *)ctx->data;
	if ((const void *)(icmp + 1) > ctx->data_end) {
		return -1;
	}

	ctx->data += sizeof(*icmp);
	ctx->icmp = icmp;
	return icmp->type;
}

static __always_inline int parse_l4(int ip_protocol, struct packet_context *ctx)
{
	/* ctx->data is on the fixed header, so the chain would parse as L4. */
	if (ctx->l4_resolved)
		return 0;

	/* Payload, not a header: reading it as one is how an attacker supplies a
	 * port (RFC 1858). */
	if (ctx->l4_unavailable)
		return 0;

	int ret;

	switch (ip_protocol) {
	case IPPROTO_UDP:
		ret = parse_udp(ctx);
		break;
	case IPPROTO_TCP:
		ret = parse_tcp(ctx);
		break;
	default:
		return 0;
	}

	/* Truncated below its port pair: the zeros left behind are not observed
	 * ports. */
	if (ret < 0) {
		ctx->l4_unavailable = 1;
		return ret;
	}

	return ret;
}

static __always_inline void swap_mac(struct ethhdr *eth)
{
	__u8 mac[6];
	__builtin_memcpy(mac, eth->h_source, sizeof(mac));
	__builtin_memcpy(eth->h_source, eth->h_dest, sizeof(eth->h_source));
	__builtin_memcpy(eth->h_dest, mac, sizeof(eth->h_dest));
}

static __always_inline void swap_port(struct udphdr *udp)
{
	if (!udp)
		return;
	__u16 tmp = udp->dest;
	udp->dest = udp->source;
	udp->source = tmp;
	/* Update UDP checksum */
	udp->check = 0;
	// cs = 0;
	// ipv4_l4_csum(udp, sizeof(*udp), &cs, iph);
	// udp->check = cs;
}

static __always_inline void swap_ip(struct iphdr *iph)
{
	__u32 tmp_ip = iph->daddr;
	iph->daddr = iph->saddr;
	iph->saddr = tmp_ip;

	/* Don't need to recalc csum in case of ip swap */
	// ip->check = ipv4_csum(ip, sizeof(*ip));
}

static __always_inline void
context_set_ip4(struct packet_context *ctx, void *data, const void *data_end,
		struct ethhdr *eth, struct vlan_hdr *vlan, struct iphdr *ip4,
		struct udphdr *udp, struct gtpuhdr *gtp)
{
	ctx->data = data;
	ctx->data_end = data_end;
	ctx->eth = eth;
	ctx->vlan = vlan;
	ctx->ip4 = ip4;
	ctx->ip6 = NULL;
	ctx->udp = udp;
	ctx->gtp = gtp;
}

static __always_inline void
context_set_ip6(struct packet_context *ctx, void *data, const void *data_end,
		struct ethhdr *eth, struct vlan_hdr *vlan, struct ipv6hdr *ip6,
		struct udphdr *udp, struct gtpuhdr *gtp)
{
	ctx->data = data;
	ctx->data_end = data_end;
	ctx->eth = eth;
	ctx->vlan = vlan;
	ctx->ip4 = NULL;
	ctx->ip6 = ip6;
	ctx->udp = udp;
	ctx->gtp = gtp;
}

static __always_inline void context_reset(struct packet_context *ctx,
					  void *data, const void *data_end)
{
	ctx->data = data;
	ctx->data_end = data_end;
	ctx->eth = NULL;
	ctx->vlan = NULL;
	ctx->ip4 = NULL;
	ctx->ip6 = NULL;
	ctx->udp = NULL;
	ctx->tcp = NULL;
	ctx->gtp = NULL;
	ctx->icmp = NULL;
	ctx->gtp_hdr_len = 0;
	ctx->l3_hdr_len = 0;
	ctx->l4_proto = 0;
	ctx->l4_sport = 0;
	ctx->l4_dport = 0;
	ctx->l4_unavailable = 0;
	ctx->exthdr_invalid = 0;
	ctx->l4_resolved = 0;
	ctx->frag_recovered = 0;
	ctx->is_fragment = 0;
	ctx->frag_nat_id = 0;
	ctx->frag_id6 = 0;
}

static __always_inline long context_reinit(struct packet_context *ctx,
					   void *data, const void *data_end)
{
	context_reset(ctx, data, data_end);

	int ethertype = parse_ethernet(ctx);
	switch (ethertype) {
	case ETH_P_IPV6: {
		if (-1 == parse_ip6(ctx)) {
			upf_printk("upf: can't parse ip6");
			return -1;
		}
		return 0;
	}
	case ETH_P_IP: {
		if (-1 == parse_ip4(ctx)) {
			upf_printk("upf: can't parse ip4");
			return -1;
		}
		return 0;
	}

	default:
		/* do nothing with non-ip packets */
		upf_printk("upf: can't process not an ip packet: %d",
			   ethertype);
		return -1;
	}
}

/* Linearize before the L3 parse rather than after it: a pull invalidates the
 * context, and re-deriving it repeats every parse already done — including the
 * extension-header walk.
 *
 * Returns 0 when the frame is ready to parse, -1 when the pull itself failed,
 * and 1 when the relinked frame no longer parses. */
static __always_inline int own_frame_relink(struct packet_context *ctx)
{
	if (!CTX_NEEDS_PULL)
		return 0;

	if (ctx_pull(ctx->ctx_buff, CTX_PULL_LEN) < 0)
		return -1;

	context_reset(ctx, ctx_data(ctx->ctx_buff),
		      ctx_data_end(ctx->ctx_buff));

	return parse_ethernet(ctx) < 0 ? 1 : 0;
}
