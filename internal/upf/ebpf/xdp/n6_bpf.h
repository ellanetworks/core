// Copyright 2025 Ella Networks

#pragma once

#include "xdp/utils/flow.h"
#include "xdp/utils/profiling.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/ipv6.h>

#include "xdp/utils/common.h"
#include "xdp/utils/frag_needed.h"
#include "xdp/utils/gtp.h"
#include "xdp/utils/pdr.h"
#include "xdp/utils/qer.h"
#include "xdp/utils/sdf.h"
#include "xdp/utils/urr.h"
#include "xdp/utils/routing.h"
#include "xdp/utils/statistics.h"
#include "xdp/utils/nocp.h"

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u32);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_DOWNLINK_IPV4_SIZE);
} pdrs_downlink_ip4 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct in6_addr);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_DOWNLINK_IPV4_SIZE);
} pdrs_downlink_ip6 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
} downlink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
} downlink_statistics SEC(".maps");

/*
 * Send an encapsulated downlink packet to the GTP tunnel.
 * Branches on far->outer_header_creation to select IPv4 or IPv6 outer header,
 * then routes via the appropriate FIB lookup.
 *
 * The two branches are kept completely separate (each ends with its own
 * return) so the BPF verifier never merges a state where ctx->ip4 was
 * cleared by context_set_ip6 with a path that calls route_ipv4, or vice
 * versa.  Merging those states would make ctx->ip4 / ctx->ip6 appear as
 * plain scalars to the verifier and cause invalid-mem-access rejections.
 */
static __always_inline enum xdp_action
send_to_gtp_tunnel(struct packet_context *ctx, const struct far_info *far,
		   __u8 tos, __u8 qfi)
{
	if (far->outer_header_creation & OHC_GTP_U_UDP_IPv6) {
		PROFILE_START(PROF_N6_GTP_MANIP);
		__u32 encap_result = add_gtp_over_ip6_headers(
			ctx, &far->localip, &far->remoteip, tos, qfi,
			far->teid);
		if (encap_result != 0) {
			PROFILE_END(PROF_N6_GTP_MANIP);
			return XDP_ABORTED;
		}
		PROFILE_END(PROF_N6_GTP_MANIP);

		ctx->statistics->packet_counters.tx++;

		const __u32 key6 = 0;
		struct route_stat *route_stat6 =
			bpf_map_lookup_elem(&downlink_route_stats, &key6);
		if (!route_stat6)
			return XDP_ABORTED;

		PROFILE_START(PROF_N6_FIB_ROUTING);
		upf_printk("upf: send gtp pdu %pI6c -> %pI6c", &ctx->ip6->saddr,
			   &ctx->ip6->daddr);
		enum xdp_action fib_ret6 = route_ipv6(ctx, route_stat6);
		PROFILE_END(PROF_N6_FIB_ROUTING);
		return fib_ret6;
	} else {
		PROFILE_START(PROF_N6_GTP_MANIP);
		__u32 encap_result = add_gtp_over_ip4_headers(
			ctx, ipv4_from_mapped(&far->localip),
			ipv4_from_mapped(&far->remoteip), tos, qfi, far->teid);
		if (encap_result != 0) {
			PROFILE_END(PROF_N6_GTP_MANIP);
			return XDP_ABORTED;
		}
		PROFILE_END(PROF_N6_GTP_MANIP);

		ctx->statistics->packet_counters.tx++;

		const __u32 key4 = 0;
		struct route_stat *route_stat4 =
			bpf_map_lookup_elem(&downlink_route_stats, &key4);
		if (!route_stat4)
			return XDP_ABORTED;

		PROFILE_START(PROF_N6_FIB_ROUTING);
		upf_printk("upf: send gtp pdu %pI4 -> %pI4", &ctx->ip4->saddr,
			   &ctx->ip4->daddr);
		enum xdp_action fib_ret4 = route_ipv4(ctx, route_stat4);
		PROFILE_END(PROF_N6_FIB_ROUTING);
		return fib_ret4;
	}
}

/*
 * Downlink processing for IPv4 packets.
 * Looks up the downlink session using the destination IP address.
 */
static __always_inline __u16 handle_n6_packet_ipv4(struct packet_context *ctx)
{
	if (masquerade) {
		PROFILE_START(PROF_N6_NAT);
		destination_nat(ctx);
		PROFILE_END(PROF_N6_NAT);
	}
	const struct iphdr *ip4 = ctx->ip4;

	PROFILE_START(PROF_N6_PDR_LOOKUP);
	struct pdr_info *pdr =
		bpf_map_lookup_elem(&pdrs_downlink_ip4, &ip4->daddr);
	PROFILE_END(PROF_N6_PDR_LOOKUP);
	if (!pdr) {
		upf_printk("upf: no downlink session for ip:%pI4", &ip4->daddr);
		return DEFAULT_XDP_ACTION;
	}

	struct far_info *far = &pdr->far;
	struct qer_info *qer = &pdr->qer;

	PROFILE_START(PROF_N6_MTU_CHECK);
	__u32 mtu_len = 0;
	long ret = 0;
	int encap_size = (far->outer_header_creation & OHC_GTP_U_UDP_IPv6) ?
				 GTP_ENCAP_SIZE_IPV6 :
				 GTP_ENCAP_SIZE_IPV4;
	ret = bpf_check_mtu(ctx->xdp_ctx, n3_ifindex, &mtu_len, encap_size, 0);
	PROFILE_END(PROF_N6_MTU_CHECK);
	if (ret < 0) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	if (ret > 0) {
		upf_printk("upf: n6 packet too large");
		mtu_len -= encap_size;
		return frag_needed(ctx, mtu_len);
	}

	ctx->interface = INTERFACE_N6;

	__u32 urr_id = pdr->urr_id;

	upf_printk("upf: downlink session for ip:%pI4 action:%d", &ip4->daddr,
		   far->action);

	if (far->action & (FAR_BUFF | FAR_NOCP)) {
		upf_printk("upf: need to notify CP for pdr:%d and qfi:%d",
			   pdr->pdr_id, qer->qfi);
		struct nocp notif = { .local_seid = pdr->local_seid,
				      .pdr_id = pdr->pdr_id,
				      .qfi = qer->qfi };
		bpf_ringbuf_output(&nocp_map, (void *)&notif,
				   sizeof(struct nocp), 0);

		/* Technically, we need to buffer the packet here, but this is not
		 * implemented yet. */
		return XDP_DROP;
	}
	if (!(far->action & FAR_FORW)) {
		upf_printk("upf: far not set to forward, dropping packet");
		return XDP_DROP;
	}
	if (!(far->outer_header_creation &
	      (OHC_GTP_U_UDP_IPv4 | OHC_GTP_U_UDP_IPv6))) {
		upf_printk(
			"upf: far not set to encapsulate in gtp, dropping packet");
		return XDP_DROP;
	}

	PROFILE_START(PROF_N6_QER_RATELIMIT);
	upf_printk("upf: qer gate_status:%d mbr:%d", qer->dl_gate_status,
		   qer->dl_maximum_bitrate);
	if (qer->dl_gate_status != GATE_STATUS_OPEN) {
		PROFILE_END(PROF_N6_QER_RATELIMIT);
		return XDP_DROP;
	}

	const __u64 packet_size = ctx->data_end - (void *)ctx->ip4;
	if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start,
						  qer->dl_maximum_bitrate)) {
		PROFILE_END(PROF_N6_QER_RATELIMIT);
		return XDP_DROP;
	}
	PROFILE_END(PROF_N6_QER_RATELIMIT);

	/* Parse inner L4 so match_sdf_filters can inspect protocol/ports */
	parse_l4(ip4->protocol, ctx);

	/* SDF filter enforcement (downlink) */
	{
		PROFILE_START(PROF_N6_SDF_FILTER);
		enum xdp_action sdf_verdict =
			match_sdf_filters(ctx, pdr->filter_map_index);
		PROFILE_END(PROF_N6_SDF_FILTER);
		if (sdf_verdict == XDP_DROP) {
			upf_printk("upf: downlink SDF drop ip:%pI4",
				   &ip4->daddr);
			ctx->statistics->xdp_actions[XDP_DROP &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			account_flow(ctx, n3_ifindex, pdr->imsi, DROP);
			return XDP_DROP;
		}
	}

	__u8 tos = far->transport_level_marking >> 8;
	upf_printk("upf: use mapping %pI4 -> TEID:%d", &ip4->daddr, far->teid);

	/* Update downlink traffic counter */
	{
		__u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
		ctx->statistics->byte_counter.bytes +=
			packet_size; // Count downlink traffic
	}

	update_urr_bytes(ctx, urr_id);
	account_flow(ctx, n3_ifindex, pdr->imsi, ALLOW);

	return send_to_gtp_tunnel(ctx, far, tos, qer->qfi);
}

/*
 * Downlink processing for IPv6 packets.
 */
static __always_inline enum xdp_action
handle_n6_packet_ipv6(struct packet_context *ctx)
{
	const struct ipv6hdr *ip6 = ctx->ip6;

	PROFILE_START(PROF_N6_PDR_LOOKUP);
	struct pdr_info *pdr =
		bpf_map_lookup_elem(&pdrs_downlink_ip6, &ip6->daddr);
	PROFILE_END(PROF_N6_PDR_LOOKUP);
	if (!pdr) {
		upf_printk("upf: no downlink session for ip:%pI6c",
			   &ip6->daddr);
		return DEFAULT_XDP_ACTION;
	}

	struct far_info *far = &pdr->far;
	struct qer_info *qer = &pdr->qer;

	int encap_size = (far->outer_header_creation & OHC_GTP_U_UDP_IPv6) ?
				 GTP_ENCAP_SIZE_IPV6 :
				 GTP_ENCAP_SIZE_IPV4;
	__u32 mtu_len = 0;
	long ret = bpf_check_mtu(ctx->xdp_ctx, n3_ifindex, &mtu_len, encap_size,
				 0);
	if (ret < 0) {
		ctx->statistics
			->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return XDP_ABORTED;
	}
	if (ret > 0) {
		upf_printk("upf: n6 ipv6 packet too large");
		mtu_len -= encap_size;
		return frag_needed(ctx, mtu_len);
	}

	ctx->interface = INTERFACE_N6;

	upf_printk("upf: downlink session for ip:%pI6c action:%d", &ip6->daddr,
		   far->action);

	if (far->action & (FAR_BUFF | FAR_NOCP)) {
		upf_printk("upf: need to notify CP for pdr:%d and qfi:%d",
			   pdr->pdr_id, qer->qfi);
		struct nocp notif = { .local_seid = pdr->local_seid,
				      .pdr_id = pdr->pdr_id,
				      .qfi = qer->qfi };
		bpf_ringbuf_output(&nocp_map, (void *)&notif,
				   sizeof(struct nocp), 0);

		/* Technically, we need to buffer the packet here, but this is not
		 * implemented yet. */
		return XDP_DROP;
	}
	if (!(far->action & FAR_FORW))
		return XDP_DROP;
	if (!(far->outer_header_creation &
	      (OHC_GTP_U_UDP_IPv4 | OHC_GTP_U_UDP_IPv6)))
		return XDP_DROP;

	upf_printk("upf: qer gate_status:%d mbr:%d", qer->dl_gate_status,
		   qer->dl_maximum_bitrate);
	if (qer->dl_gate_status != GATE_STATUS_OPEN)
		return XDP_DROP;

	const __u64 packet_size = ctx->data_end - (void *)ctx->ip6;
	if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start,
						  qer->dl_maximum_bitrate))
		return XDP_DROP;

	__u8 tos = far->transport_level_marking >> 8;
	upf_printk("upf: use mapping %pI6c -> TEID:%d", &ip6->daddr, far->teid);
	return send_to_gtp_tunnel(ctx, far, tos, qer->qfi);
}
