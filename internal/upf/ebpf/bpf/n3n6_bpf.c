// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include <linux/in.h>
#include <linux/if_ether.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <sys/socket.h>

#include "bpf/n3_bpf.h"
#include "bpf/n6_bpf.h"

#include "bpf/utils/statistics.h"
#include "bpf/utils/common.h"
#include "bpf/utils/routing.h"

#include "bpf/utils/trace.h"
#include "bpf/utils/profiling.h"
#include "bpf/utils/gtp_control.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"
#include "bpf/utils/nat.h"

/*
 * The datapath is split so the verifier checks each stage on its own budget. A
 * thin entry program (upf_entry_func) classifies by packet type and tail-calls
 * into a stage program: upf_uplink_func (GTP-U decap) or upf_downlink_func
 * (downlink forwarding). Classifying by packet type (not by interface) keeps a
 * single entry working whether N3 and N6 are separate interfaces or the same
 * one. The downlink stage is also the reuse point for UE-to-UE local switching
 * later. See bpf_datapath_split_plan.md.
 */

/* N3 uplink: GTP-U-encapsulated traffic from the gNB. */
static __always_inline enum ctx_action
handle_uplink_ip4(struct packet_context *ctx)
{
	if (parse_ip4(ctx) == IPPROTO_UDP) {
		struct udphdr *udp = detect_udp_header(ctx, 0);
		if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT) {
			parse_udp(ctx);
			upf_printk(
				"upf: gtp-u received on N3, src=%pI4 dst=%pI4",
				&ctx->ip4->saddr, &ctx->ip4->daddr);
			enum ctx_action action = handle_gtpu(ctx);
			ctx->statistics->xdp_actions[ctx_stat_action(action) &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return action;
		}
	}

	/* Non-GTP traffic on N3 is not uplink user-plane; leave it to the stack. */
	ctx->statistics->xdp_actions[ctx_stat_action(DEFAULT_CTX_ACTION) &
				     EUPF_MAX_XDP_ACTION_MASK] += 1;
	return DEFAULT_CTX_ACTION;
}

static __always_inline enum ctx_action
handle_uplink_ip6(struct packet_context *ctx)
{
	if (parse_ip6(ctx) == IPPROTO_UDP) {
		struct udphdr *udp = detect_udp_header(ctx, 0);
		if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT) {
			parse_udp(ctx);
			upf_printk("upf: gtp-u received on N3 (IPv6 outer)");
			enum ctx_action action = handle_gtpu(ctx);
			ctx->statistics->xdp_actions[ctx_stat_action(action) &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return action;
		}
	}

	ctx->statistics->xdp_actions[ctx_stat_action(DEFAULT_CTX_ACTION) &
				     EUPF_MAX_XDP_ACTION_MASK] += 1;
	return DEFAULT_CTX_ACTION;
}

static __always_inline enum ctx_action
process_uplink(struct packet_context *ctx)
{
	switch (parse_ethernet(ctx)) {
	case ETH_P_IP:
		return handle_uplink_ip4(ctx);
	case ETH_P_IPV6:
		return handle_uplink_ip6(ctx);
	case ETH_P_ARP:
		upf_printk("upf: arp received on N3. passing to kernel");
		return CTX_ACT_OK;
	}
	return DEFAULT_CTX_ACTION;
}

/* N6 downlink: plain IP traffic from the data network toward a UE. */
static __always_inline enum ctx_action
handle_downlink_ip4(struct packet_context *ctx)
{
	int l4_protocol = parse_ip4(ctx);
	if (l4_protocol != IPPROTO_UDP && l4_protocol != IPPROTO_ICMP &&
	    l4_protocol != IPPROTO_TCP) {
		ctx->statistics
			->xdp_actions[ctx_stat_action(DEFAULT_CTX_ACTION) &
				      EUPF_MAX_XDP_ACTION_MASK] += 1;
		return DEFAULT_CTX_ACTION;
	}

	ctx->statistics->packet_counters.rx++;
	enum ctx_action action = handle_n6_packet_ipv4(ctx);
	ctx->statistics->xdp_actions[ctx_stat_action(action) &
				     EUPF_MAX_XDP_ACTION_MASK] += 1;
	return action;
}

static __always_inline enum ctx_action
handle_downlink_ip6(struct packet_context *ctx)
{
	int l4_protocol = parse_ip6(ctx);
	if (l4_protocol != IPPROTO_UDP && l4_protocol != IPPROTO_ICMPV6 &&
	    l4_protocol != IPPROTO_TCP) {
		ctx->statistics
			->xdp_actions[ctx_stat_action(DEFAULT_CTX_ACTION) &
				      EUPF_MAX_XDP_ACTION_MASK] += 1;
		return DEFAULT_CTX_ACTION;
	}

	ctx->statistics->packet_counters.rx++;
	enum ctx_action action = handle_n6_packet_ipv6(ctx);
	ctx->statistics->xdp_actions[ctx_stat_action(action) &
				     EUPF_MAX_XDP_ACTION_MASK] += 1;
	return action;
}

static __always_inline enum ctx_action
process_downlink(struct packet_context *ctx)
{
	switch (parse_ethernet(ctx)) {
	case ETH_P_IP:
		return handle_downlink_ip4(ctx);
	case ETH_P_IPV6:
		return handle_downlink_ip6(ctx);
	case ETH_P_ARP:
		upf_printk("upf: arp received on N6. passing to kernel");
		return CTX_ACT_OK;
	}
	return DEFAULT_CTX_ACTION;
}

/* The lookup fails only to the verifier: the statistics maps are single-entry
 * per-CPU arrays, so index 0 always exists. */
static __always_inline struct upf_statistic *get_stats(void *stats_map)
{
	const __u32 key = 0;

	return bpf_map_lookup_elem(stats_map, &key);
}

/* upf_gtpu_control_func: tail-call stage for the GTP-U messages the UPF answers
 * itself — echo requests (TS 29.281 §7.2) and error indications for an unknown
 * TEID (§7.3.1). Re-parses from its own ctx (the stack does not survive a tail
 * call). */
CTX_DP_SEC("upf_gtpu_control")
int upf_gtpu_control_func(struct __ctx_buff *ctx)
{
	long vlan_ret = ctx_vlan_ingress(ctx);
	if (vlan_ret)
		return vlan_ret < 0 ? CTX_ACT_ABORTED : CTX_ACT_OK;

	struct upf_statistic *statistics = get_stats(&uplink_statistics);
	if (!statistics)
		return CTX_ACT_ABORTED;

	/* This stage answers echo requests and error indications by rewriting
	 * the frame in place, so the headers must be writable first. The pull
	 * precedes the context so the parse below starts from fresh pointers. */
	if (CTX_NEEDS_PULL && ctx_pull(ctx, CTX_PULL_LEN) < 0)
		return DEFAULT_CTX_ACTION;

	struct packet_context context = {
		.data = ctx_data(ctx),
		.data_end = ctx_data_end(ctx),
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N3,
	};

	if (context_reinit(&context, context.data, context.data_end) != 0)
		return DEFAULT_CTX_ACTION;

	/* Each transport is dispatched to its own return: a shared tail lets the
	 * verifier merge the ip4 and ip6 states, after which it rejects a
	 * dereference of either. */
	if (context.ip4) {
		if (parse_udp(&context) != GTP_UDP_PORT)
			return DEFAULT_CTX_ACTION;

		int pdu_type = parse_gtp(&context);
		if (!context.gtp)
			return DEFAULT_CTX_ACTION;

		if (pdu_type == GTPU_ECHO_REQUEST)
			return handle_echo_request(&context);

		if (pdu_type != GTPU_G_PDU || context.gtp->teid == 0)
			return DEFAULT_CTX_ACTION;

		/* The stage is independently reachable, so the absent session is
		 * confirmed here. */
		__u32 teid4 = bpf_htonl(context.gtp->teid);
		if (bpf_map_lookup_elem(&pdrs_uplink, &teid4))
			return DEFAULT_CTX_ACTION;

		return send_error_indication_ipv4(&context);
	}

	if (context.ip6) {
		if (parse_udp(&context) != GTP_UDP_PORT)
			return DEFAULT_CTX_ACTION;

		int pdu_type = parse_gtp(&context);
		if (!context.gtp)
			return DEFAULT_CTX_ACTION;

		if (pdu_type == GTPU_ECHO_REQUEST)
			return handle_echo_request(&context);

		if (pdu_type != GTPU_G_PDU || context.gtp->teid == 0)
			return DEFAULT_CTX_ACTION;

		__u32 teid6 = bpf_htonl(context.gtp->teid);
		if (bpf_map_lookup_elem(&pdrs_uplink, &teid6))
			return DEFAULT_CTX_ACTION;

		return send_error_indication_ipv6(&context);
	}

	return DEFAULT_CTX_ACTION;
}

/* upf_uplink_func: tail-call stage for GTP-U uplink traffic. Re-parses from its
 * own ctx (the stack does not survive a tail call). */
CTX_DP_SEC("upf_uplink")
int upf_uplink_func(struct __ctx_buff *ctx)
{
	long vlan_ret = ctx_vlan_ingress(ctx);
	if (vlan_ret)
		return vlan_ret < 0 ? CTX_ACT_ABORTED : CTX_ACT_OK;

	struct upf_statistic *statistics = get_stats(&uplink_statistics);
	if (!statistics)
		return CTX_ACT_ABORTED;

	struct packet_context context = {
		.data = ctx_data(ctx),
		.data_end = ctx_data_end(ctx),
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N3,
	};

	PROFILE_START(PROF_N3_TOTAL);
	enum ctx_action ret = process_uplink(&context);
	PROFILE_END(PROF_N3_TOTAL);
	return ret;
}

/* upf_downlink_func: tail-call stage for plain downlink traffic toward a UE. */
CTX_DP_SEC("upf_downlink")
int upf_downlink_func(struct __ctx_buff *ctx)
{
	long vlan_ret = ctx_vlan_ingress(ctx);
	if (vlan_ret)
		return vlan_ret < 0 ? CTX_ACT_ABORTED : CTX_ACT_OK;

	struct upf_statistic *statistics = get_stats(&downlink_statistics);
	if (!statistics)
		return CTX_ACT_ABORTED;

	struct packet_context context = {
		.data = ctx_data(ctx),
		.data_end = ctx_data_end(ctx),
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N6,
	};

	PROFILE_START(PROF_N6_TOTAL);
	enum ctx_action ret = process_downlink(&context);
	PROFILE_END(PROF_N6_TOTAL);
	return ret;
}

/* upf_entry_func: attached to the N3/N6 interface(s). Classifies by packet type
 * — GTP-U (UDP :2152) is uplink, everything else is downlink — and tail-calls
 * the matching stage. Packet-type classification (not interface) keeps this
 * correct when N3 and N6 share one interface.
 *
 * With distinct interfaces the shape alone is not enough: uplink traffic is
 * attributed to a subscriber and source-NATed on its behalf, so a GTP-U-shaped
 * packet arriving on N6 must not claim that treatment. */
CTX_DP_SEC("upf_entry")
int upf_entry_func(struct __ctx_buff *ctx)
{
	long vlan_ret = ctx_vlan_ingress(ctx);
	if (vlan_ret)
		return vlan_ret < 0 ? CTX_ACT_ABORTED : CTX_ACT_OK;

	struct packet_context context = {
		.data = ctx_data(ctx),
		.data_end = ctx_data_end(ctx),
		.ctx_buff = ctx,
	};

	__u16 l3_protocol = parse_ethernet(&context);
	__u32 index = UPF_CALL_DOWNLINK;

	if (l3_protocol == ETH_P_ARP) {
		upf_printk("upf: arp received. passing to kernel");
		return CTX_ACT_OK;
	}

	const bool split_interfaces = n3_ifindex != 0 && n6_ifindex != 0 &&
				      n3_ifindex != n6_ifindex;
	const bool gtpu_allowed = !split_interfaces ||
				  ctx_ingress_ifindex(ctx) == (__u32)n3_ifindex;

	if (l3_protocol == ETH_P_IP) {
		if (parse_ip4(&context) == IPPROTO_UDP) {
			struct udphdr *udp = detect_udp_header(&context, 0);
			if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT &&
			    gtpu_allowed)
				index = UPF_CALL_UPLINK;
		}
	} else if (l3_protocol == ETH_P_IPV6) {
		if (parse_ip6(&context) == IPPROTO_UDP) {
			struct udphdr *udp = detect_udp_header(&context, 0);
			if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT &&
			    gtpu_allowed)
				index = UPF_CALL_UPLINK;
		}
	} else {
		return DEFAULT_CTX_ACTION;
	}

	bpf_tail_call(ctx, &upf_calls, index);

	/* Only reached if the stage program is not populated in upf_calls. */
	return DEFAULT_CTX_ACTION;
}

/* Keyed by the inner IPv6 destination address. */
struct veth_tunnel_info {
	__u32 teid;
	struct in6_addr local_addr;
	struct in6_addr remote_addr;
	__u8 qfi;
	/* 4G S1-U carries plain GTP-U with no PDU Session Container (PSC is
	 * N3/N9 only, TS 38.415); encapsulate PSC-less when set. */
	__u8 no_psc;
	__u8 pad[2];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct in6_addr);
	__type(value, struct veth_tunnel_info);
	__uint(max_entries, 256);
} veth_tunnels SEC(".maps");

/* veth_xdp_func: attached to the veth-xdp end of the pair the SMF injects
 * Router Advertisements on. */
CTX_DP_SEC("veth_xdp")
int veth_xdp_func(struct __ctx_buff *ctx)
{
	long vlan_ret = ctx_vlan_ingress(ctx);
	if (vlan_ret)
		return vlan_ret < 0 ? CTX_ACT_ABORTED : CTX_ACT_OK;

	void *data = ctx_data(ctx);
	const void *data_end = ctx_data_end(ctx);

	struct ethhdr *eth = data;
	if ((void *)(eth + 1) > data_end)
		return CTX_ACT_DROP;

	if (eth->h_proto != bpf_htons(ETH_P_IPV6)) {
		return CTX_ACT_DROP;
	}

	struct ipv6hdr *ip6 = (struct ipv6hdr *)(eth + 1);
	if ((void *)(ip6 + 1) > data_end) {
		return CTX_ACT_DROP;
	}

	struct veth_tunnel_info *tun =
		bpf_map_lookup_elem(&veth_tunnels, &ip6->daddr);
	if (!tun) {
		upf_printk("upf: veth XDP tunnel miss dst=%pI6c", &ip6->daddr);
		return CTX_ACT_DROP;
	}

	upf_printk("upf: veth received RA for dest %pI6c", &ip6->daddr);

	struct packet_context pkt_ctx = {
		.ctx_buff = ctx,
		.interface = INTERFACE_N6,
		.eth = eth,
		.ip6 = ip6,
	};

	int ret;
	if (is_ipv4_mapped_ipv6(&tun->local_addr) &&
	    is_ipv4_mapped_ipv6(&tun->remote_addr)) {
		upf_printk("upf: encapsulating over IPv4");
		__u32 saddr = ipv4_from_mapped(&tun->local_addr);
		__u32 daddr = ipv4_from_mapped(&tun->remote_addr);
		if (tun->no_psc) {
			ret = add_gtp_over_ip4_headers_s1u(&pkt_ctx, saddr,
							   daddr, 0, tun->teid);
		} else {
			ret = add_gtp_over_ip4_headers(&pkt_ctx, saddr, daddr,
						       0, tun->qfi, tun->teid);
		}
		if (ret != 0) {
			return CTX_ACT_ABORTED;
		}

		const __u32 key4 = 0;
		struct route_stat *route_stat4 =
			bpf_map_lookup_elem(&downlink_route_stats, &key4);
		if (!route_stat4)
			return CTX_ACT_ABORTED;

		return route_ipv4(&pkt_ctx, route_stat4, true);
	} else {
		upf_printk("upf: encapsulating over IPv6");
		if (tun->no_psc) {
			ret = add_gtp_over_ip6_headers_s1u(&pkt_ctx,
							   &tun->local_addr,
							   &tun->remote_addr, 0,
							   tun->teid);
		} else {
			ret = add_gtp_over_ip6_headers(&pkt_ctx,
						       &tun->local_addr,
						       &tun->remote_addr, 0,
						       tun->qfi, tun->teid);
		}
		if (ret != 0) {
			return CTX_ACT_ABORTED;
		}

		const __u32 key6 = 0;
		struct route_stat *route_stat6 =
			bpf_map_lookup_elem(&downlink_route_stats, &key6);
		if (!route_stat6)
			return CTX_ACT_ABORTED;

		return route_ipv6(&pkt_ctx, route_stat6, true);
	}

	return CTX_ACT_ABORTED;
}

char _license[] SEC("license") = "GPL";
