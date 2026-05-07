// Copyright 2026 Ella Networks
//

// XDP program for the veth-xdp side of the veth pair used for
// SMF -> UPF packet injection (Router Advertisements, etc.).
//
// For IPv6 packets whose destination matches a veth_tunnels map entry, the
// program reuses the N6 GTP-U encapsulation path (add_gtp_over_ip6_headers
// or add_gtp_over_ip4_headers) and redirects the packet to the N3 interface.

#include "xdp/utils/routing.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/udp.h>

#include "xdp/n6_bpf.h"
#include "xdp/utils/gtp.h"
#include "xdp/utils/trace.h"

/* ------------------------------------------------------------------ */
/* Tunnel encapsulation map                                            */
/* ------------------------------------------------------------------ */

// Tunnel metadata for GTP-U encapsulation of packets injected via veth.
// Key: inner IPv6 destination address (struct in6_addr).
struct veth_tunnel_info {
	__u32 teid;
	struct in6_addr local_addr;
	struct in6_addr remote_addr;
	__u8 qfi;
	__u8 pad[3];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct in6_addr);
	__type(value, struct veth_tunnel_info);
	__uint(max_entries, 256);
} veth_tunnels SEC(".maps");

/* ------------------------------------------------------------------ */
/* XDP entrypoint                                                      */
/* ------------------------------------------------------------------ */

SEC("xdp/veth_xdp")
int veth_xdp_func(struct xdp_md *ctx)
{
	void *data = (void *)(long)ctx->data;
	void *data_end = (void *)(long)ctx->data_end;

	struct ethhdr *eth = data;
	if ((void *)(eth + 1) > data_end)
		return XDP_DROP;

	// Non-IPv6 dropped.
	if (eth->h_proto != bpf_htons(ETH_P_IPV6)) {
		return XDP_DROP;
	}

	// Short IPv6 header.
	struct ipv6hdr *ip6 = (struct ipv6hdr *)(eth + 1);
	if ((void *)(ip6 + 1) > data_end) {
		return XDP_DROP;
	}

	// Tunnel map lookup.
	struct veth_tunnel_info *tun =
		bpf_map_lookup_elem(&veth_tunnels, &ip6->daddr);
	if (!tun) {
		upf_printk("upf: veth XDP tunnel miss dst=%pI6c", &ip6->daddr);
		return XDP_DROP;
	}

	upf_printk("upf: veth received RA for dest %pI6c", &ip6->daddr);

	// Set up packet_context for reuse of N6 encapsulation functions.
	struct packet_context pkt_ctx = {
		.xdp_ctx = ctx,
		.interface = n6_ifindex,
		.eth = eth,
		.ip6 = ip6,
	};

	int ret;
	if (is_ipv4_mapped_ipv6(&tun->local_addr) &&
	    is_ipv4_mapped_ipv6(&tun->remote_addr)) {
		upf_printk("upf: encapsulating over IPv4");
		__u32 saddr = ipv4_from_mapped(&tun->local_addr);
		__u32 daddr = ipv4_from_mapped(&tun->remote_addr);
		ret = add_gtp_over_ip4_headers(&pkt_ctx, saddr, daddr, 0,
					       tun->qfi, tun->teid);
		if (ret != 0) {
			return XDP_ABORTED;
		}

		const __u32 key4 = 0;
		struct route_stat *route_stat4 =
			bpf_map_lookup_elem(&downlink_route_stats, &key4);
		if (!route_stat4)
			return XDP_ABORTED;

		enum xdp_action fib_ret4 = route_ipv4(&pkt_ctx, route_stat4);
		return fib_ret4;
	} else {
		upf_printk("upf: encapsulating over IPv6");
		ret = add_gtp_over_ip6_headers(&pkt_ctx, &tun->local_addr,
					       &tun->remote_addr, 0, tun->qfi,
					       tun->teid);
		if (ret != 0) {
			return XDP_ABORTED;
		}

		const __u32 key6 = 0;
		struct route_stat *route_stat6 =
			bpf_map_lookup_elem(&downlink_route_stats, &key6);
		if (!route_stat6)
			return XDP_ABORTED;

		enum xdp_action fib_ret6 = route_ipv6(&pkt_ctx, route_stat6);
		return fib_ret6;
	}

	return XDP_ABORTED;
}

char _license[] SEC("license") = "GPL";
