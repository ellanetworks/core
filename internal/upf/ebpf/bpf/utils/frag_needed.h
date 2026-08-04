// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "linux/bpf.h"
#include "bpf/bpf_helpers.h"

#include "bpf/utils/csum.h"
#include "bpf/utils/gtp.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"
#include "bpf/utils/routing.h"
#include <bpf/bpf_endian.h>
#include <linux/icmp.h>
#include <linux/icmpv6.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <stdint.h>

static __always_inline int vlan_to_insert(struct packet_context *ctx)
{
	/* The reply leaves the way the frame arrived, and with a shared master
	 * the ingress ifindex names both sides. */
	return egress_vlan_reflected(ctx);
}

/* Source address for a self-generated reply: the preferred source the FIB
 * would pick. BPF_FIB_LOOKUP_SRC is what writes ipv4_src (net/core/filter.c,
 * fib_result_prefsrc), and tot_len stays 0 so the MTU check does not abort the
 * lookup before it does. The seed is what remains if it never gets there. */
static __always_inline __be32 get_src_ip_addr(struct packet_context *ctx)
{
	struct bpf_fib_lookup fib_params = {};
	fib_params.family = AF_INET;
	fib_params.l4_protocol = ctx->ip4->protocol;
	fib_params.tot_len = 0;
	fib_params.ipv4_src = ctx->ip4->daddr;
	fib_params.ipv4_dst = ctx->ip4->saddr;
	fib_params.ifindex = ctx_ingress_ifindex(ctx->ctx_buff);

	bpf_fib_lookup(ctx->ctx_buff, &fib_params, sizeof(fib_params),
		       BPF_FIB_LOOKUP_DIRECT | BPF_FIB_LOOKUP_SRC);
	return fib_params.ipv4_src;
}

/* IPv6 counterpart. RFC 4443 §2.2 requires the source to be an address of the
 * node; the UE address the trigger was sent to is neither ours nor routable
 * back through N6 under BCP38. */
/* `orig` is re-derived after the resize: ctx->ip6 still points into the
 * pre-resize frame, which the verifier treats as a scalar. */
static __always_inline void get_src_ip6_addr(struct packet_context *ctx,
					     const struct ipv6hdr *orig,
					     struct in6_addr *out)
{
	struct bpf_fib_lookup fib_params = {};
	fib_params.family = AF_INET6;
	fib_params.l4_protocol = orig->nexthdr;
	fib_params.tot_len = 0;
	__builtin_memcpy(fib_params.ipv6_src, &orig->daddr,
			 sizeof(fib_params.ipv6_src));
	__builtin_memcpy(fib_params.ipv6_dst, &orig->saddr,
			 sizeof(fib_params.ipv6_dst));
	fib_params.ifindex = ctx_ingress_ifindex(ctx->ctx_buff);

	bpf_fib_lookup(ctx->ctx_buff, &fib_params, sizeof(fib_params),
		       BPF_FIB_LOOKUP_DIRECT | BPF_FIB_LOOKUP_SRC);
	__builtin_memcpy(out, fib_params.ipv6_src, sizeof(*out));
}

static __always_inline enum ctx_action
frag_needed_ipv4(struct packet_context *ctx, __be16 mtu)
{
	upf_printk("upf: preparing fragmention needed error");
	ctx->statistics->packet_counters.rx++;
	if ((ctx->ip4->frag_off & bpf_htons(0x4000)) == 0) {
		// Don't Fragment is not set, drop the packet
		upf_printk("upf: DF not set, dropping: %04X",
			   ctx->ip4->frag_off);
		return drop_with(ctx, UPF_DROP_DF_NOT_SET);
	}

	int adj_size = sizeof(struct icmphdr) + sizeof(struct iphdr);

	int incoming_vlan = CTX_INBAND_VLAN ? vlan_to_insert(ctx) : 0;
	if (!ctx->vlan && incoming_vlan) {
		adj_size += sizeof(struct vlan_hdr);
	}

	int ret = ctx_prepend(ctx->ctx_buff, adj_size);
	if (ret < 0) {
		upf_printk("upf: could not adjust head");
		return abort_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);
	}

	// Reinitialize pointers to satisfy the verifier
	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);
	ctx->eth = (struct ethhdr *)(data + adj_size);
	if (((const void *)(ctx->eth) > data_end) ||
	    ((const void *)(ctx->eth + 1) > data_end)) {
		upf_printk("upf: could not find original eth header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}
	ctx->vlan = NULL;
	ctx->ip4 = (struct iphdr *)(ctx->eth + 1);
	if (ctx->eth->h_proto == bpf_htons(ETH_P_8021Q)) {
		ctx->vlan = (struct vlan_hdr *)(ctx->eth + 1);
		ctx->ip4 = (struct iphdr *)(ctx->vlan + 1);
	}
	if (ctx->vlan && (const void *)(ctx->vlan + 1) > data_end) {
		upf_printk("upf: could not find original vlan header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}
	if (((const void *)(ctx->ip4) > data_end) ||
	    ((const void *)(ctx->ip4 + 1) > data_end)) {
		upf_printk("upf: could not find original ip header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	struct ethhdr *new_eth = (struct ethhdr *)(data);

	if ((const void *)(new_eth + 1) > data_end) {
		upf_printk("upf: could not write new eth header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	__builtin_memcpy(new_eth->h_dest, ctx->eth->h_source, ETH_ALEN);
	__builtin_memcpy(new_eth->h_source, ctx->eth->h_dest, ETH_ALEN);
	new_eth->h_proto = ctx->eth->h_proto;

	struct iphdr *new_ip = (struct iphdr *)(new_eth + 1);

	if ((const void *)(new_ip + 1) > data_end) {
		upf_printk("upf: could not write new ip header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	if (incoming_vlan) {
		struct vlan_hdr *new_vlan = (struct vlan_hdr *)(new_eth + 1);
		if ((const void *)(new_vlan + 1) > data_end) {
			upf_printk("upf: could not write new vlan header");
			return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
		}

		new_ip = (struct iphdr *)(new_vlan + 1);
		if ((const void *)(new_ip + 1) > data_end) {
			upf_printk("upf: could not write new ip header");
			return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
		}

		if (ctx->vlan) {
			__builtin_memcpy(new_vlan, ctx->vlan,
					 sizeof(*new_vlan));
		} else {
			new_vlan->h_vlan_TCI =
				bpf_htons(incoming_vlan & 0x0FFF);
			new_vlan->h_vlan_encapsulated_proto = ctx->eth->h_proto;
			new_eth->h_proto = bpf_htons(ETH_P_8021Q);
		}
	}

	/* Built field by field: copying the trigger would inherit its ihl, tos
	 * (ECN included), id and frag_off, and an ihl claiming options this
	 * header does not carry lands the receiver's parse in the payload. */
	__builtin_memset(new_ip, 0, sizeof(*new_ip));
	new_ip->version = 4;
	new_ip->ihl = sizeof(struct iphdr) >> 2;
	new_ip->tos = IPTOS_PREC_INTERNETCONTROL;
	new_ip->frag_off = 0;
	new_ip->protocol = IPPROTO_ICMP;
	new_ip->ttl = 64;
	new_ip->tot_len = bpf_htons(sizeof(struct iphdr) +
				    sizeof(struct icmphdr) + ICMP_QUOTE_LEN);
	new_ip->daddr = ctx->ip4->saddr;
	new_ip->saddr = get_src_ip_addr(ctx);
	recompute_ipv4_csum(new_ip);

	struct icmphdr *new_icmp = (struct icmphdr *)(new_ip + 1);
	if ((const void *)(new_icmp + 1) > data_end) {
		upf_printk("upf: could not write new icmp header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}
	new_icmp->type = ICMP_DEST_UNREACH;
	new_icmp->code = ICMP_FRAG_NEEDED;
	new_icmp->un.frag.mtu = mtu;
	/* RFC 1191 §4, and it is covered by the checksum computed below. */
	new_icmp->un.frag.__unused = 0;

	int pkt_size = (int)ctx_len_from(ctx->ctx_buff, data_end, data);
	if (pkt_size <= 0)
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

	int icmp_pkt_size = sizeof(struct ethhdr) + sizeof(struct iphdr) +
			    sizeof(struct icmphdr) + ICMP_QUOTE_LEN;
	if (incoming_vlan) {
		icmp_pkt_size += sizeof(struct vlan_hdr);
	}
	if ((data + icmp_pkt_size) > data_end) {
		upf_printk("upf: could not write new icmp header");
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}
	recompute_icmp_csum(new_icmp, sizeof(struct icmphdr) + ICMP_QUOTE_LEN);
	if (pkt_size != icmp_pkt_size) {
		adj_size = icmp_pkt_size - pkt_size;
		int ret = ctx_adjust_tail(ctx->ctx_buff, adj_size);
		if (ret < 0) {
			upf_printk("upf: could not adjust tail by: %d",
				   adj_size);
			upf_printk("upf: pkt_size: %d", pkt_size);
			upf_printk("upf: icmp_pkt_size: %X", icmp_pkt_size);
			return abort_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);
		}
	}
	upf_printk("upf: sending fragmentation needed error");
	return tx_back(ctx, egress_vlan_reflected(ctx));
}

/* ICMPv6 Packet Too Big (RFC 4443 §3.2), built in place: the room prepended in
 * front of the trigger holds the new headers, and the trigger itself becomes
 * the quote. @mtu is in network byte order. */
static __always_inline enum ctx_action
send_packet_too_big(struct packet_context *ctx, __be16 mtu)
{
	upf_printk("upf: preparing packet too big error");

	/* Space to prepend: new ICMPv6 header + new outer IPv6 header */
	int adj_size = (int)(sizeof(struct icmp6hdr) + sizeof(struct ipv6hdr));

	int incoming_vlan = CTX_INBAND_VLAN ? vlan_to_insert(ctx) : 0;
	if (!ctx->vlan && incoming_vlan)
		adj_size += (int)sizeof(struct vlan_hdr);

	int ret = ctx_prepend(ctx->ctx_buff, adj_size);
	if (ret < 0) {
		upf_printk("upf: could not adjust head");
		return abort_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);
	}

	/* Re-read data/data_end after adjust_head */
	void *data = ctx_data(ctx->ctx_buff);
	const void *data_end = ctx_data_end(ctx->ctx_buff);

	/* Original ETH header is at data + adj_size */
	struct ethhdr *orig_eth = (struct ethhdr *)(data + adj_size);
	if (((const void *)orig_eth > data_end) ||
	    ((const void *)(orig_eth + 1) > data_end)) {
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	/* Locate original IPv6 header (past original ETH, optional VLAN) */
	ctx->vlan = NULL;
	struct ipv6hdr *orig_ip6 = (struct ipv6hdr *)(orig_eth + 1);
	if (orig_eth->h_proto == bpf_htons(ETH_P_8021Q)) {
		ctx->vlan = (struct vlan_hdr *)(orig_eth + 1);
		if ((const void *)(ctx->vlan + 1) > data_end) {
			return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
		}
		orig_ip6 = (struct ipv6hdr *)(ctx->vlan + 1);
	}
	if ((const void *)(orig_ip6 + 1) > data_end) {
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	/* --- Write new Ethernet header at data --- */
	struct ethhdr *new_eth = (struct ethhdr *)data;
	if ((const void *)(new_eth + 1) > data_end) {
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}
	__builtin_memcpy(new_eth->h_dest, orig_eth->h_source, ETH_ALEN);
	__builtin_memcpy(new_eth->h_source, orig_eth->h_dest, ETH_ALEN);
	new_eth->h_proto = bpf_htons(ETH_P_IPV6);

	/* --- Optional VLAN header --- */
	struct ipv6hdr *new_ip6 = (struct ipv6hdr *)(new_eth + 1);
	if (incoming_vlan) {
		struct vlan_hdr *new_vlan = (struct vlan_hdr *)(new_eth + 1);
		if ((const void *)(new_vlan + 1) > data_end) {
			return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
		}
		if (ctx->vlan) {
			__builtin_memcpy(new_vlan, ctx->vlan,
					 sizeof(*new_vlan));
		} else {
			new_vlan->h_vlan_TCI =
				bpf_htons(incoming_vlan & 0x0FFF);
			new_vlan->h_vlan_encapsulated_proto =
				bpf_htons(ETH_P_IPV6);
			new_eth->h_proto = bpf_htons(ETH_P_8021Q);
		}
		new_ip6 = (struct ipv6hdr *)(new_vlan + 1);
	}
	if ((const void *)(new_ip6 + 1) > data_end) {
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	/* --- Write new IPv6 header (swap src/dst) --- */
	static const __u16 icmp6_msg_len =
		sizeof(struct icmp6hdr) + ICMP_QUOTE_LEN;
	new_ip6->version = 6;
	new_ip6->priority = 0;
	new_ip6->flow_lbl[0] = 0;
	new_ip6->flow_lbl[1] = 0;
	new_ip6->flow_lbl[2] = 0;
	new_ip6->payload_len = bpf_htons(icmp6_msg_len);
	new_ip6->nexthdr = IPPROTO_ICMPV6;
	new_ip6->hop_limit = 64;
	get_src_ip6_addr(ctx, orig_ip6, &new_ip6->saddr);
	__builtin_memcpy(&new_ip6->daddr, &orig_ip6->saddr,
			 sizeof(struct in6_addr));

	/* --- Write ICMPv6 Packet Too Big header --- */
	struct icmp6hdr *new_icmp6 = (struct icmp6hdr *)(new_ip6 + 1);
	if ((const void *)(new_icmp6 + 1) > data_end) {
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}
	new_icmp6->icmp6_type = ICMPV6_PKT_TOOBIG;
	new_icmp6->icmp6_code = 0;
	new_icmp6->icmp6_cksum = 0;
	new_icmp6->icmp6_mtu = bpf_htonl((__u32)bpf_ntohs(mtu));

	/* Verify the full ICMPv6 message (header + payload) is in bounds */
	if ((const void *)((void *)new_icmp6 + icmp6_msg_len) > data_end) {
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);
	}

	/* Compute ICMPv6 checksum (pseudo-header + 56 bytes of ICMPv6 data) */
	new_icmp6->icmp6_cksum =
		icmpv6_ptb_csum(&new_ip6->saddr, &new_ip6->daddr, new_icmp6);

	/* Trim or extend the packet tail to the exact ICMPv6 packet size */
	int eth_hdr_len = (int)sizeof(struct ethhdr);
	if (incoming_vlan)
		eth_hdr_len += (int)sizeof(struct vlan_hdr);
	int icmp_pkt_size =
		eth_hdr_len + (int)sizeof(struct ipv6hdr) + icmp6_msg_len;
	int pkt_size = (int)ctx_len_from(ctx->ctx_buff, data_end, data);
	if (pkt_size <= 0)
		return abort_with(ctx, UPF_DROP_INTERNAL_WRITE_FAILED);

	if (pkt_size != icmp_pkt_size) {
		if (ctx_adjust_tail(ctx->ctx_buff, icmp_pkt_size - pkt_size) <
		    0) {
			upf_printk("upf: could not adjust tail for PTB");
			return abort_with(ctx, UPF_DROP_INTERNAL_RESIZE_FAILED);
		}
	}

	upf_printk("upf: sending packet too big error");
	return tx_back(ctx, egress_vlan_reflected(ctx));
}

static __always_inline enum ctx_action
frag_needed_ipv6(struct packet_context *ctx, __be16 mtu)
{
	return send_packet_too_big(ctx, mtu);
}

/* Dispatches on the parsed pointers rather than re-reading eth->h_proto: a
 * fresh load is an unconstrained scalar to the verifier, which then explores
 * the branch where ctx->ip4 is known to be NULL. */
static __always_inline enum ctx_action frag_needed(struct packet_context *ctx,
						   __u32 mtu_len)
{
	__be16 mtu = bpf_htons(mtu_len);
	if (ctx->ip4)
		return frag_needed_ipv4(ctx, mtu);
	if (ctx->ip6)
		return send_packet_too_big(ctx, mtu);
	return drop_with(ctx, UPF_DROP_MALFORMED_HEADER);
}
