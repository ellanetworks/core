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

#include "bpf/n6_bpf.h"
#include "bpf/n3_bpf.h"

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
 * The datapath is split so the verifier checks each stage on its own budget:
 * a thin entry program tail-calls into upf_uplink_func or upf_downlink_func.
 */

/* N3 uplink: GTP-U-encapsulated traffic from the gNB. */
static __always_inline enum ctx_action
handle_uplink_ip4(struct packet_context *ctx)
{
	/* l4_unavailable: a non-first fragment of the tunnel transport has
	 * payload where its UDP header would be, and payload that spells 2152
	 * would be decapsulated as GTP-U (RFC 1858). */
	if (parse_ip4(ctx) == IPPROTO_UDP && !ctx->l4_unavailable) {
		struct udphdr *udp = detect_udp_header(ctx, 0);
		if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT) {
			parse_udp(ctx);
			upf_printk(
				"upf: gtp-u received on N3, src=%pI4 dst=%pI4",
				&ctx->ip4->saddr, &ctx->ip4->daddr);
			return handle_gtpu(ctx);
		}
	}

	/* Non-GTP traffic on N3 is not uplink user-plane; leave it to the stack. */
	return DEFAULT_CTX_ACTION;
}

static __always_inline enum ctx_action
handle_uplink_ip6(struct packet_context *ctx)
{
	/* The tunnel transport; the inner packet is parsed after decap. */
	if (parse_ip6_transport(ctx) == IPPROTO_UDP) {
		struct udphdr *udp = detect_udp_header(ctx, 0);
		if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT) {
			parse_udp(ctx);
			upf_printk("upf: gtp-u received on N3 (IPv6 outer)");
			return handle_gtpu(ctx);
		}
	}

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

/* Every protocol is offered to the session lookup: the uplink carries a
 * subscriber's ESP, GRE and SCTP unfiltered, and the Packet Filter Set has an
 * SPI component for it (TS 23.501 §5.7.6.2). Non-session traffic falls through
 * at the PDR lookup. */
static __always_inline enum ctx_action
handle_downlink_ip4(struct packet_context *ctx)
{
	int relinked = own_frame_relink(ctx);
	if (relinked < 0)
		return abort_with(ctx, UPF_DROP_INTERNAL_PULL_FAILED);

	if (relinked > 0)
		return DEFAULT_CTX_ACTION;

	if (parse_ip4(ctx) < 0) {
		/* A reason means the header was rejected, not merely
		 * unrecognised: drop it rather than hand it to the kernel. */
		if (ctx->drop_reason != UPF_DROP_UNSPEC)
			return drop_with(ctx, ctx->drop_reason);

		return DEFAULT_CTX_ACTION;
	}

	/* Before destination_nat_lookup, which reads ports. */
	frag_resolve4(ctx);

	ctx->statistics->packet_counters.rx++;

	return handle_n6_packet_ipv4(ctx);
}

static __always_inline enum ctx_action
handle_downlink_ip6(struct packet_context *ctx)
{
	int relinked = own_frame_relink(ctx);
	if (relinked < 0)
		return abort_with(ctx, UPF_DROP_INTERNAL_PULL_FAILED);

	if (relinked > 0)
		return DEFAULT_CTX_ACTION;

	if (parse_ip6(ctx) < 0)
		return DEFAULT_CTX_ACTION;

	ctx->statistics->packet_counters.rx++;

	return handle_n6_packet_ipv6(ctx);
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

/* Fails only to the verifier: the maps are single-entry per-CPU arrays. */
static __always_inline struct upf_statistic *get_stats(void *stats_map)
{
	const __u32 key = 0;

	return bpf_map_lookup_elem(stats_map, &key);
}

/* The GTP-U messages the UPF answers itself: echo requests (TS 29.281 §7.2)
 * and error indications for an unknown TEID (§7.3.1). Re-parses from its own
 * ctx; the stack does not survive a tail call. */
CTX_DP_SEC("upf_gtpu_control")
int upf_gtpu_control_func(struct __ctx_buff *ctx)
{
	struct upf_statistic *statistics = get_stats(&uplink_statistics);
	if (!statistics)
		return ctx_verdict(CTX_ACT_ABORTED);

	struct packet_context context = {
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N3,
	};

	if (ctx_vlan_ingress(ctx))
		return record_action(&context, CTX_ACT_OK);

	/* This stage rewrites the frame in place, so the headers must be
	 * writable first. */
	if (CTX_NEEDS_PULL && ctx_pull(ctx, CTX_PULL_LEN) < 0)
		return record_action(&context, DEFAULT_CTX_ACTION);

	context.data = ctx_data(ctx);
	context.data_end = ctx_data_end(ctx);

	if (context_reinit(&context, context.data, context.data_end) != 0)
		return record_action(&context, DEFAULT_CTX_ACTION);

	/* Each transport is dispatched to its own return: a shared tail lets the
	 * verifier merge the ip4 and ip6 states, after which it rejects a
	 * dereference of either. */
	if (context.ip4) {
		if (parse_udp(&context) != GTP_UDP_PORT)
			return record_action(&context, DEFAULT_CTX_ACTION);

		int pdu_type = parse_gtp(&context);
		if (!context.gtp)
			return record_action(&context, DEFAULT_CTX_ACTION);

		if (pdu_type == GTPU_ECHO_REQUEST)
			return record_action(&context, handle_echo_request(&context));

		if (pdu_type != GTPU_G_PDU || context.gtp->teid == 0)
			return record_action(&context, DEFAULT_CTX_ACTION);

		/* The stage is independently reachable, so the absent session is
		 * confirmed here. */
		__u32 teid4 = bpf_htonl(context.gtp->teid);
		if (bpf_map_lookup_elem(&pdrs_uplink, &teid4))
			return record_action(&context, DEFAULT_CTX_ACTION);

		return record_action(&context, send_error_indication_ipv4(&context));
	}

	if (context.ip6) {
		if (parse_udp(&context) != GTP_UDP_PORT)
			return record_action(&context, DEFAULT_CTX_ACTION);

		int pdu_type = parse_gtp(&context);
		if (!context.gtp)
			return record_action(&context, DEFAULT_CTX_ACTION);

		if (pdu_type == GTPU_ECHO_REQUEST)
			return record_action(&context, handle_echo_request(&context));

		if (pdu_type != GTPU_G_PDU || context.gtp->teid == 0)
			return record_action(&context, DEFAULT_CTX_ACTION);

		__u32 teid6 = bpf_htonl(context.gtp->teid);
		if (bpf_map_lookup_elem(&pdrs_uplink, &teid6))
			return record_action(&context, DEFAULT_CTX_ACTION);

		return record_action(&context, send_error_indication_ipv6(&context));
	}

	return record_action(&context, DEFAULT_CTX_ACTION);
}

/* Re-parses from its own ctx; the stack does not survive a tail call. */
CTX_DP_SEC("upf_uplink")
int upf_uplink_func(struct __ctx_buff *ctx)
{
	struct upf_statistic *statistics = get_stats(&uplink_statistics);
	if (!statistics)
		return ctx_verdict(CTX_ACT_ABORTED);

	struct packet_context context = {
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N3,
	};

	if (ctx_vlan_ingress(ctx))
		return record_action(&context, CTX_ACT_OK);

	/* Safe before parsing because upf_entry_func only tail-calls here for
	 * UDP to the GTP-U port, which the datapath owns. data_end bounds only
	 * the linear head, which a header-splitting NIC sizes with
	 * eth_get_headlen: __skb_get_poff stops at thoff + 8 for UDP
	 * (net/core/flow_dissector.c), leaving the GTP-U header in a fragment,
	 * so parsing first would hand the G-PDU to the stack undecapsulated. */
	if (CTX_NEEDS_PULL && ctx_pull(ctx, CTX_PULL_LEN) < 0)
		return record_action(&context,
				     abort_with(&context, UPF_DROP_INTERNAL_PULL_FAILED));

	context.data = ctx_data(ctx);
	context.data_end = ctx_data_end(ctx);

	PROFILE_START(PROF_N3_TOTAL);
	enum ctx_action ret = process_uplink(&context);
	PROFILE_END(PROF_N3_TOTAL);

	return record_action(&context, ret);
}

CTX_DP_SEC("upf_downlink")
int upf_downlink_func(struct __ctx_buff *ctx)
{
	struct upf_statistic *statistics = get_stats(&downlink_statistics);
	if (!statistics)
		return ctx_verdict(CTX_ACT_ABORTED);

	struct packet_context context = {
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N6,
	};

	if (ctx_vlan_ingress(ctx))
		return record_action(&context, CTX_ACT_OK);

	/* No pull here: this stage is reached by every frame that is not GTP-U,
	 * most of which the datapath does not own. handle_downlink_ip4/ip6 pull
	 * once the ethertype says the frame could be ours. */
	context.data = ctx_data(ctx);
	context.data_end = ctx_data_end(ctx);

	/* Only the buffer responder injects on this veth, and everything it
	 * sends is for a UE: a frame the datapath does not forward has no
	 * legitimate consumer, and passing it to the host would only have it
	 * routed back out N6. */
	const bool reinject = frame_is_reinjected(ctx);

	PROFILE_START(PROF_N6_TOTAL);
	enum ctx_action ret = process_downlink(&context);
	PROFILE_END(PROF_N6_TOTAL);

	if (reinject && ret == CTX_ACT_OK)
		ret = drop_with(&context, UPF_DROP_REINJECT_UNOWNED);

	return record_action(&context, ret);
}

CTX_DP_SEC("upf_local_switch")
int upf_local_switch_func(struct __ctx_buff *ctx)
{
	struct upf_statistic *statistics = get_stats(&uplink_statistics);
	if (!statistics)
		return ctx_verdict(CTX_ACT_ABORTED);

	struct packet_context context = {
		.ctx_buff = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N3,
	};

	if (ctx_vlan_ingress(ctx))
		return record_action(&context, CTX_ACT_OK);

	if (CTX_NEEDS_PULL && ctx_pull(ctx, CTX_PULL_LEN) < 0)
		return record_action(&context,
				     abort_with(&context, UPF_DROP_INTERNAL_PULL_FAILED));

	context.data = ctx_data(ctx);
	context.data_end = ctx_data_end(ctx);

	__u16 l3 = parse_ethernet(&context);

	if (l3 == ETH_P_IP) {
		if (parse_ip4(&context) < 0)
			return record_action(&context, DEFAULT_CTX_ACTION);

		frag_resolve4(&context);
	} else if (l3 == ETH_P_IPV6) {
		if (parse_ip6(&context) < 0)
			return record_action(&context, DEFAULT_CTX_ACTION);

		parse_l4(context.l4_proto, &context);
	} else {
		return record_action(&context, DEFAULT_CTX_ACTION);
	}

	struct pdr_info *dl_pdr = try_local_switch(&context);
	if (!dl_pdr)
		return record_action(&context, DEFAULT_CTX_ACTION);

	const __u32 lskey = 0;
	struct pdr_info *ul_pdr =
		bpf_map_lookup_elem(&local_switch_ul_pdr, &lskey);
	if (!ul_pdr)
		return record_action(&context, DEFAULT_CTX_ACTION);

	enum ctx_action ret = local_switch_to_ue(&context, dl_pdr, ul_pdr);

	if (ctx_action_forwards(ret)) {
		const __u64 billed_bytes = ctx_full_len(ctx);
		const __u32 dlkey = 0;
		struct upf_statistic *dl_stats =
			bpf_map_lookup_elem(&downlink_statistics, &dlkey);
		if (dl_stats) {
			dl_stats->packet_counters.tx++;
			dl_stats->byte_counter.bytes += billed_bytes;
		}
	}

	return record_action(&context, ret);
}

/* Classifying by packet shape rather than by interface keeps this correct when
 * N3 and N6 share one. The shape alone is not enough, though: uplink traffic
 * is attributed to a subscriber and source-NATed on its behalf, so a
 * GTP-U-shaped packet arriving on N6 must not claim that treatment. */
CTX_DP_SEC("upf_entry")
int upf_entry_func(struct __ctx_buff *ctx)
{
	/* Returns here are not counted: the frame is not classified yet, so
	 * neither statistics map is the right one. */

	if (ctx_vlan_ingress(ctx))
		return ctx_verdict(CTX_ACT_OK);

	struct packet_context context = {
		.data = ctx_data(ctx),
		.data_end = ctx_data_end(ctx),
		.ctx_buff = ctx,
	};

	__u16 l3_protocol = parse_ethernet(&context);
	__u32 index = UPF_CALL_DOWNLINK;

	if (l3_protocol == ETH_P_ARP) {
		upf_printk("upf: arp received. passing to kernel");
		return ctx_verdict(CTX_ACT_OK);
	}

	const bool split_interfaces = n3_ifindex != 0 && n6_ifindex != 0 &&
				      n3_ifindex != n6_ifindex;
	const bool ingress_is_n3 =
		ctx_ingress_ifindex(ctx) == (__u32)n3_ifindex;
	const bool gtpu_allowed = !split_interfaces || ingress_is_n3;

	if (l3_protocol == ETH_P_IP) {
		if (parse_ip4(&context) == IPPROTO_UDP &&
		    !context.l4_unavailable) {
			struct udphdr *udp = detect_udp_header(&context, 0);
			if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT &&
			    gtpu_allowed)
				index = UPF_CALL_UPLINK;
		}
	} else if (l3_protocol == ETH_P_IPV6) {
		/* Classification only; the owning stage parses properly. */
		if (parse_ip6_transport(&context) == IPPROTO_UDP) {
			struct udphdr *udp = detect_udp_header(&context, 0);
			if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT &&
			    gtpu_allowed)
				index = UPF_CALL_UPLINK;
		}
	} else {
		return ctx_verdict(DEFAULT_CTX_ACTION);
	}

	/* The mirror of gtpu_allowed: downlink tunnels to a UE on its destination
	 * address alone, so it belongs to frames that arrived on N6. Inactive on
	 * a shared interface, which the classification above covers. */
	if (index == UPF_CALL_DOWNLINK && split_interfaces && ingress_is_n3) {
		upf_printk("upf: non-GTP frame on N3, passing to kernel");
		return ctx_verdict(DEFAULT_CTX_ACTION);
	}

	bpf_tail_call(ctx, &upf_calls, index);

	/* Only reached if the stage program is not populated in upf_calls. */
	return ctx_verdict(DEFAULT_CTX_ACTION);
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
	if (ctx_vlan_ingress(ctx))
		return ctx_verdict(CTX_ACT_OK);

	void *data = ctx_data(ctx);
	const void *data_end = ctx_data_end(ctx);

	struct ethhdr *eth = data;
	if ((void *)(eth + 1) > data_end)
		return ctx_verdict(CTX_ACT_DROP);

	if (eth->h_proto != bpf_htons(ETH_P_IPV6)) {
		return ctx_verdict(CTX_ACT_DROP);
	}

	struct ipv6hdr *ip6 = (struct ipv6hdr *)(eth + 1);
	if ((void *)(ip6 + 1) > data_end) {
		return ctx_verdict(CTX_ACT_DROP);
	}

	struct veth_tunnel_info *tun =
		bpf_map_lookup_elem(&veth_tunnels, &ip6->daddr);
	if (!tun) {
		upf_printk("upf: veth XDP tunnel miss dst=%pI6c", &ip6->daddr);
		return ctx_verdict(CTX_ACT_DROP);
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
			return ctx_verdict(CTX_ACT_ABORTED);
		}

		const __u32 key4 = 0;
		struct route_stat *route_stat4 =
			bpf_map_lookup_elem(&downlink_route_stats, &key4);
		if (!route_stat4)
			return ctx_verdict(CTX_ACT_ABORTED);

		return ctx_verdict(route_ipv4(&pkt_ctx, route_stat4, true));
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
			return ctx_verdict(CTX_ACT_ABORTED);
		}

		const __u32 key6 = 0;
		struct route_stat *route_stat6 =
			bpf_map_lookup_elem(&downlink_route_stats, &key6);
		if (!route_stat6)
			return ctx_verdict(CTX_ACT_ABORTED);

		return ctx_verdict(route_ipv6(&pkt_ctx, route_stat6, true));
	}

	return ctx_verdict(CTX_ACT_ABORTED);
}

char _license[] SEC("license") = "GPL";
