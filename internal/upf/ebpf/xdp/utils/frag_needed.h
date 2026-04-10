// Copyright 2025 Ella Networks

#pragma once

#include "linux/bpf.h"
#include "bpf/bpf_helpers.h"

#include "xdp/utils/csum.h"
#include "xdp/utils/gtp.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/routing.h"
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
	if (ctx->xdp_ctx->ingress_ifindex == n3_ifindex) {
		return n3_vlan;
	} else if (ctx->xdp_ctx->ingress_ifindex == n6_ifindex) {
		return n6_vlan;
	}
	return 0;
}

static __always_inline __be32 get_src_ip_addr(struct packet_context *ctx)
{
	struct bpf_fib_lookup fib_params = {};
	fib_params.family = AF_INET;
	fib_params.tos = ctx->ip4->tos;
	fib_params.l4_protocol = ctx->ip4->protocol;
	fib_params.sport = 0;
	fib_params.dport = 0;
	fib_params.tot_len = bpf_ntohs(ctx->ip4->tot_len);
	fib_params.ipv4_src = ctx->ip4->daddr;
	fib_params.ipv4_dst = ctx->ip4->saddr;
	fib_params.ifindex = ctx->xdp_ctx->ingress_ifindex;

	__u64 flags = BPF_FIB_LOOKUP_DIRECT;
	bpf_fib_lookup(ctx->xdp_ctx, &fib_params, sizeof(fib_params), flags);
	return fib_params.ipv4_src;
}

static __always_inline enum xdp_action
frag_needed_ipv4(struct packet_context *ctx, __be16 mtu)
{
	upf_printk("upf: preparing fragmention needed error");
	if (ctx->ip4->protocol < 0) {
		upf_printk("upf: packet was not IPv4");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	ctx->statistics->packet_counters.rx++;
	if ((ctx->ip4->frag_off & bpf_htons(0x4000)) == 0) {
		// Don't Fragment is not set, drop the packet
		upf_printk("upf: DF not set, dropping: %04X",
			   ctx->ip4->frag_off);
		ctx->statistics
			->xdp_actions[XDP_DROP & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_DROP;
	}

	int adj_size = sizeof(struct icmphdr) + sizeof(struct iphdr);

	int incoming_vlan = vlan_to_insert(ctx);
	if (!ctx->vlan && incoming_vlan) {
		adj_size += sizeof(struct vlan_hdr);
	}

	int ret = bpf_xdp_adjust_head(ctx->xdp_ctx, -adj_size);
	if (ret < 0) {
		upf_printk("upf: could not adjust head");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	// Reinitialize pointers to satisfy the verifier
	void *data = (void *)(long)ctx->xdp_ctx->data;
	void *data_end = (void *)(long)ctx->xdp_ctx->data_end;
	ctx->eth = (struct ethhdr *)(data + adj_size);
	if (((const void *)(ctx->eth) > data_end) ||
	    ((const void *)(ctx->eth + 1) > data_end)) {
		upf_printk("upf: could not find original eth header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	ctx->vlan = NULL;
	ctx->ip4 = (struct iphdr *)(ctx->eth + 1);
	if (ctx->eth->h_proto == bpf_htons(ETH_P_8021Q)) {
		ctx->vlan = (struct vlan_hdr *)(ctx->eth + 1);
		ctx->ip4 = (struct iphdr *)(ctx->vlan + 1);
	}
	if (ctx->vlan && (const void *)(ctx->vlan + 1) > data_end) {
		upf_printk("upf: could not find original vlan header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	if (((const void *)(ctx->ip4) > data_end) ||
	    ((const void *)(ctx->ip4 + 1) > data_end)) {
		upf_printk("upf: could not find original ip header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	struct ethhdr *new_eth = (struct ethhdr *)(data);

	if ((const void *)(new_eth + 1) > data_end) {
		upf_printk("upf: could not write new eth header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	__builtin_memcpy(new_eth->h_dest, ctx->eth->h_source, ETH_ALEN);
	__builtin_memcpy(new_eth->h_source, ctx->eth->h_dest, ETH_ALEN);
	new_eth->h_proto = ctx->eth->h_proto;

	struct iphdr *new_ip = (struct iphdr *)(new_eth + 1);

	if ((const void *)(new_ip + 1) > data_end) {
		upf_printk("upf: could not write new ip header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	if (incoming_vlan) {
		struct vlan_hdr *new_vlan = (struct vlan_hdr *)(new_eth + 1);
		if ((const void *)(new_vlan + 1) > data_end) {
			upf_printk("upf: could not write new vlan header");
			ctx->statistics->xdp_actions[XDP_ABORTED &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return XDP_ABORTED;
		}

		new_ip = (struct iphdr *)(new_vlan + 1);
		if ((const void *)(new_ip + 1) > data_end) {
			upf_printk("upf: could not write new ip header");
			ctx->statistics->xdp_actions[XDP_ABORTED &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return XDP_ABORTED;
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

	__builtin_memcpy(new_ip, ctx->ip4, sizeof(*new_ip));
	new_ip->daddr = ctx->ip4->saddr;
	new_ip->saddr = ctx->ip4->daddr;
	new_ip->protocol = IPPROTO_ICMP;
	new_ip->ttl = 64;
	new_ip->tot_len =
		bpf_htons(sizeof(struct iphdr) + sizeof(struct icmphdr) +
			  sizeof(struct iphdr) + 8);
	new_ip->saddr = get_src_ip_addr(ctx);
	recompute_ipv4_csum(new_ip);

	struct icmphdr *new_icmp = (struct icmphdr *)(new_ip + 1);
	if ((const void *)(new_icmp + 1) > data_end) {
		upf_printk("upf: could not write new icmp header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	new_icmp->type = ICMP_DEST_UNREACH;
	new_icmp->code = ICMP_FRAG_NEEDED;
	new_icmp->un.frag.mtu = mtu;

	int pkt_size = data_end - data;
	int icmp_pkt_size = sizeof(struct ethhdr) + sizeof(struct iphdr) +
			    sizeof(struct icmphdr) + sizeof(struct iphdr) + 8;
	if (incoming_vlan) {
		icmp_pkt_size += sizeof(struct vlan_hdr);
	}
	if ((data + icmp_pkt_size) > data_end) {
		upf_printk("upf: could not write new icmp header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	recompute_icmp_csum(new_icmp,
			    sizeof(struct icmphdr) + sizeof(struct iphdr) + 8);
	if (pkt_size != icmp_pkt_size) {
		adj_size = icmp_pkt_size - pkt_size;
		int ret = bpf_xdp_adjust_tail(ctx->xdp_ctx, adj_size);
		if (ret < 0) {
			upf_printk("upf: could not adjust tail by: %d",
				   adj_size);
			upf_printk("upf: pkt_size: %d", pkt_size);
			upf_printk("upf: icmp_pkt_size: %X", icmp_pkt_size);
			upf_printk("upf: data_end: %X", data_end);
			ctx->statistics->xdp_actions[XDP_ABORTED &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return XDP_ABORTED;
		}
	}
	upf_printk("upf: sending fragmentation needed error");
	ctx->statistics->xdp_actions[XDP_TX & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return XDP_TX;
}

/*
 * send_packet_too_big - generate an ICMPv6 Packet Too Big (Type 2, Code 0)
 * error and send it back towards the originator of the oversized inner IPv6
 * packet.
 *
 * The new packet structure is:
 *   ETH (14) | IPv6 (40) | ICMPv6 PTB (8) | orig IPv6 hdr (40) | 8 bytes
 * = 110 bytes (+ 4 bytes if VLAN is added).
 *
 * We prepend sizeof(icmp6hdr)+sizeof(ipv6hdr) = 48 bytes in front of the
 * original packet, reuse the original IPv6 header as the ICMPv6 payload,
 * swap src/dst addresses, and compute the ICMPv6 checksum.
 *
 * @mtu: effective MTU in network byte order (16-bit).
 */
static __always_inline enum xdp_action
send_packet_too_big(struct packet_context *ctx, __be16 mtu)
{
	upf_printk("upf: preparing packet too big error");

	/* Space to prepend: new ICMPv6 header + new outer IPv6 header */
	int adj_size = (int)(sizeof(struct icmp6hdr) + sizeof(struct ipv6hdr));

	int incoming_vlan = vlan_to_insert(ctx);
	if (!ctx->vlan && incoming_vlan)
		adj_size += (int)sizeof(struct vlan_hdr);

	int ret = bpf_xdp_adjust_head(ctx->xdp_ctx, -adj_size);
	if (ret < 0) {
		upf_printk("upf: could not adjust head");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	/* Re-read data/data_end after adjust_head */
	void *data = (void *)(long)ctx->xdp_ctx->data;
	void *data_end = (void *)(long)ctx->xdp_ctx->data_end;

	/* Original ETH header is at data + adj_size */
	struct ethhdr *orig_eth = (struct ethhdr *)(data + adj_size);
	if (((const void *)orig_eth > data_end) ||
	    ((const void *)(orig_eth + 1) > data_end)) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	/* Locate original IPv6 header (past original ETH, optional VLAN) */
	ctx->vlan = NULL;
	struct ipv6hdr *orig_ip6 = (struct ipv6hdr *)(orig_eth + 1);
	if (orig_eth->h_proto == bpf_htons(ETH_P_8021Q)) {
		ctx->vlan = (struct vlan_hdr *)(orig_eth + 1);
		if ((const void *)(ctx->vlan + 1) > data_end) {
			ctx->statistics->xdp_actions[XDP_ABORTED &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return XDP_ABORTED;
		}
		orig_ip6 = (struct ipv6hdr *)(ctx->vlan + 1);
	}
	if ((const void *)(orig_ip6 + 1) > data_end) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	/* --- Write new Ethernet header at data --- */
	struct ethhdr *new_eth = (struct ethhdr *)data;
	if ((const void *)(new_eth + 1) > data_end) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	__builtin_memcpy(new_eth->h_dest, orig_eth->h_source, ETH_ALEN);
	__builtin_memcpy(new_eth->h_source, orig_eth->h_dest, ETH_ALEN);
	new_eth->h_proto = bpf_htons(ETH_P_IPV6);

	/* --- Optional VLAN header --- */
	struct ipv6hdr *new_ip6 = (struct ipv6hdr *)(new_eth + 1);
	if (incoming_vlan) {
		struct vlan_hdr *new_vlan = (struct vlan_hdr *)(new_eth + 1);
		if ((const void *)(new_vlan + 1) > data_end) {
			ctx->statistics->xdp_actions[XDP_ABORTED &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return XDP_ABORTED;
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
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}

	/* --- Write new IPv6 header (swap src/dst) --- */
	static const __u16 icmp6_msg_len =
		sizeof(struct icmp6hdr) + sizeof(struct ipv6hdr) + 8;
	new_ip6->version = 6;
	new_ip6->priority = 0;
	new_ip6->flow_lbl[0] = 0;
	new_ip6->flow_lbl[1] = 0;
	new_ip6->flow_lbl[2] = 0;
	new_ip6->payload_len = bpf_htons(icmp6_msg_len);
	new_ip6->nexthdr = IPPROTO_ICMPV6;
	new_ip6->hop_limit = 64;
	__builtin_memcpy(&new_ip6->saddr, &orig_ip6->daddr,
			 sizeof(struct in6_addr));
	__builtin_memcpy(&new_ip6->daddr, &orig_ip6->saddr,
			 sizeof(struct in6_addr));

	/* --- Write ICMPv6 Packet Too Big header --- */
	struct icmp6hdr *new_icmp6 = (struct icmp6hdr *)(new_ip6 + 1);
	if ((const void *)(new_icmp6 + 1) > data_end) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	new_icmp6->icmp6_type = ICMPV6_PKT_TOOBIG;
	new_icmp6->icmp6_code = 0;
	new_icmp6->icmp6_cksum = 0;
	new_icmp6->icmp6_mtu = bpf_htonl((__u32)bpf_ntohs(mtu));

	/* Verify the full ICMPv6 message (header + payload) is in bounds */
	if ((const void *)((void *)new_icmp6 + icmp6_msg_len) > data_end) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
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
	int pkt_size = (int)(data_end - data);
	if (pkt_size != icmp_pkt_size) {
		int adj_tail = icmp_pkt_size - pkt_size;
		if (bpf_xdp_adjust_tail(ctx->xdp_ctx, adj_tail) < 0) {
			upf_printk("upf: could not adjust tail for PTB");
			ctx->statistics->xdp_actions[XDP_ABORTED &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return XDP_ABORTED;
		}
	}

	upf_printk("upf: sending packet too big error");
	ctx->statistics->xdp_actions[XDP_TX & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return XDP_TX;
}

static __always_inline enum xdp_action
frag_needed_ipv6(struct packet_context *ctx, __be16 mtu)
{
	return send_packet_too_big(ctx, mtu);
}

/*
 * frag_needed - dispatch to the correct MTU-exceeded handler.
 *
 * Use ctx->ip4 / ctx->ip6 (set during initial packet parsing) to decide:
 *   IPv4 inner → ICMP Fragmentation Needed
 *   IPv6 inner → ICMPv6 Packet Too Big
 *
 * We deliberately avoid re-reading eth->h_proto here: a fresh memory load
 * would give the BPF verifier an unconstrained scalar and cause it to explore
 * the wrong branch on paths where ctx->ip4 is known to be NULL.
 */
static __always_inline enum xdp_action frag_needed(struct packet_context *ctx,
						   __u32 mtu_len)
{
	__be16 mtu = bpf_htons(mtu_len);
	if (ctx->ip4)
		return frag_needed_ipv4(ctx, mtu);
	if (ctx->ip6)
		return send_packet_too_big(ctx, mtu);
	ctx->statistics->xdp_actions[XDP_DROP & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return XDP_DROP;
}
