// Copyright 2025 Ella Networks

#include "linux/bpf.h"
#include "bpf/bpf_helpers.h"

#include "xdp/utils/csum.h"
#include "xdp/utils/gtp.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/routing.h"
#include <bpf/bpf_endian.h>
#include <linux/icmp.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/ip.h>
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
	bpf_fib_lookup(ctx->xdp_ctx, &fib_params, sizeof(fib_params),
		       flags);
	return fib_params.ipv4_src;
}

static __always_inline enum xdp_action
frag_needed_ipv4(struct packet_context *ctx, __be16 mtu)
{
	upf_printk("upf: preparing fragmention needed error");
	int ip_proto = parse_ip4(ctx);
	if (ip_proto < 0) {
		upf_printk("upf: packet was not IPv4");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}
	ctx->statistics->packet_counters.rx++;
	if ((ctx->ip4->frag_off & bpf_htons(0x4000)) == 0) {
		// Don't Fragment is not set, drop the packet
		upf_printk("upf: DF not set, dropping: %04X", ctx->ip4->frag_off);
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
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}

	// Reinitialize pointers to satisfy the verifier
	void *data = (void *)(long)ctx->xdp_ctx->data;
	void *data_end = (void *)(long)ctx->xdp_ctx->data_end;
	ctx->eth = (struct ethhdr *)(data + adj_size);
	if (((const void *)(ctx->eth) > data_end) || ((const void *)(ctx->eth + 1) > data_end)) {
		upf_printk("upf: could not find original eth header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
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
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}
	if (((const void *)(ctx->ip4) > data_end) || ((const void *)(ctx->ip4 + 1) > data_end)) {
		upf_printk("upf: could not find original ip header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}

	struct ethhdr *new_eth = (struct ethhdr *)(data);

	if ((const void *)(new_eth + 1) > data_end) {
		upf_printk("upf: could not write new eth header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}

	__builtin_memcpy(new_eth->h_dest, ctx->eth->h_source, ETH_ALEN);
	__builtin_memcpy(new_eth->h_source, ctx->eth->h_dest, ETH_ALEN);
	new_eth->h_proto = ctx->eth->h_proto;

	struct iphdr *new_ip = (struct iphdr *)(new_eth + 1);

	if ((const void *)(new_ip + 1) > data_end) {
		upf_printk("upf: could not write new ip header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}

	if (incoming_vlan) {
		struct vlan_hdr *new_vlan = (struct vlan_hdr *)(new_eth + 1);
		if ((const void *)(new_vlan + 1) > data_end) {
			upf_printk("upf: could not write new vlan header");
			ctx->statistics
				->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
			return XDP_ABORTED;
		}

		new_ip = (struct iphdr *)(new_vlan + 1);
		if ((const void *)(new_ip + 1) > data_end) {
			upf_printk("upf: could not write new ip header");
			ctx->statistics
				->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
			return XDP_ABORTED;
		}

		if (ctx->vlan) {
			__builtin_memcpy(new_vlan, ctx->vlan, sizeof(*new_vlan));
		} else {
			new_vlan->h_vlan_TCI = bpf_htons(incoming_vlan & 0x0FFF);
			new_vlan->h_vlan_encapsulated_proto = ctx->eth->h_proto;
			new_eth->h_proto = bpf_htons(ETH_P_8021Q);
		}
	}

	__builtin_memcpy(new_ip, ctx->ip4, sizeof(*new_ip));
	new_ip->daddr = ctx->ip4->saddr;
	new_ip->saddr = ctx->ip4->daddr;
	new_ip->protocol = IPPROTO_ICMP;
	new_ip->ttl = 64;
	new_ip->tot_len = bpf_htons(sizeof(struct iphdr) + sizeof(struct icmphdr) + sizeof(struct iphdr) + 8);
	new_ip->saddr = get_src_ip_addr(ctx);
	recompute_ipv4_csum(new_ip);

	struct icmphdr *new_icmp = (struct icmphdr *)(new_ip + 1);
	if ((const void *)(new_icmp + 1) > data_end) {
		upf_printk("upf: could not write new icmp header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}
	new_icmp->type = ICMP_DEST_UNREACH;
	new_icmp->code = ICMP_FRAG_NEEDED;
	new_icmp->un.frag.mtu = mtu;

	int pkt_size = data_end - data;
	int icmp_pkt_size = sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct icmphdr) + sizeof(struct iphdr) + 8;
	if (incoming_vlan) {
		icmp_pkt_size += sizeof(struct vlan_hdr);
	}
	if ((data + icmp_pkt_size) > data_end) {
		upf_printk("upf: could not write new icmp header");
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}
	recompute_icmp_csum(new_icmp, sizeof(struct icmphdr) + sizeof(struct iphdr) + 8);
	if (pkt_size != icmp_pkt_size) {
		adj_size = icmp_pkt_size - pkt_size;
		int ret = bpf_xdp_adjust_tail(ctx->xdp_ctx, adj_size);
		if (ret < 0) {
			upf_printk("upf: could not adjust tail by: %d", adj_size);
			upf_printk("upf: pkt_size: %d", pkt_size);
			upf_printk("upf: icmp_pkt_size: %X", icmp_pkt_size);
			upf_printk("upf: data_end: %X", data_end);
			ctx->statistics
				->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
			return XDP_ABORTED;
		}
	}
	upf_printk("upf: sending fragmentation needed error");
	ctx->statistics
		->xdp_actions[XDP_TX & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return XDP_TX;
}

static __always_inline enum xdp_action
frag_needed(struct packet_context *ctx, __u32 mtu_len)
{
	__u16 l3_protocol = parse_ethernet(ctx);
	__be16 mtu = bpf_htons(mtu_len);
	switch (l3_protocol) {
	case ETH_P_IP:
		return frag_needed_ipv4(ctx, mtu);
	}
	ctx->statistics->xdp_actions[XDP_DROP & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return XDP_DROP;
}
