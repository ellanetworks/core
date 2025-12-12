// Copyright 2025 Ella Networks

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/ipv6.h>

#include "xdp/utils/common.h"
#include "xdp/utils/gtp.h"
#include "xdp/utils/pdr.h"
#include "xdp/utils/qer.h"
#include "xdp/utils/urr.h"
#include "xdp/utils/routing.h"
#include "xdp/utils/statistics.h"
#include "xdp/utils/nocp.h"

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u32);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_DOWNLINK_IPV4_SIZE);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} pdrs_downlink_ip4 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct in6_addr);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_DOWNLINK_IPV4_SIZE);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} pdrs_downlink_ip6 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} downlink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} downlink_statistics SEC(".maps");

/*
 * This function adds the necessary outer headers for downlink encapsulation
 * and then routes the packet. Note that the transmit counter is now updated
 * using the downlink counter (tx_n6).
 */
static __always_inline enum xdp_action
send_to_gtp_tunnel(struct packet_context *ctx, int srcip, int dstip, __u8 tos,
		   __u8 qfi, int teid)
{
	if (-1 == add_gtp_over_ip4_headers(ctx, srcip, dstip, tos, qfi, teid))
		return XDP_ABORTED;
	upf_printk("upf: send gtp pdu %pI4 -> %pI4", &ctx->ip4->saddr,
		   &ctx->ip4->daddr);
	ctx->downlink_statistics->packet_counters.tx++;

	const __u32 key = 0;
	struct route_stat *route_statistic =
		bpf_map_lookup_elem(&downlink_route_stats, &key);
	if (!route_statistic)
		return XDP_ABORTED;
	return route_ipv4(ctx, route_statistic);
}

/*
 * Downlink processing for IPv4 packets.
 * Looks up the downlink session using the destination IP address.
 */
static __always_inline __u16 handle_n6_packet_ipv4(struct packet_context *ctx)
{
	if (masquerade) {
		destination_nat(ctx);
	}
	const struct iphdr *ip4 = ctx->ip4;
	struct pdr_info *pdr =
		bpf_map_lookup_elem(&pdrs_downlink_ip4, &ip4->daddr);
	if (!pdr) {
		upf_printk("upf: no downlink session for ip:%pI4", &ip4->daddr);
		return DEFAULT_XDP_ACTION;
	}

	ctx->interface = INTERFACE_N6;

	__u32 far_id = pdr->far_id;
	__u32 qer_id = pdr->qer_id;
	__u32 urr_id = pdr->urr_id;
	if (pdr->sdf_mode) {
		struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
		if (match_sdf_filter_ipv4(ctx, sdf)) {
			upf_printk(
				"Packet with source ip:%pI4 and destination ip:%pI4 matches SDF filter",
				&ip4->saddr, &ip4->daddr);
			far_id = pdr->sdf_rules.far_id;
			qer_id = pdr->sdf_rules.qer_id;
			urr_id = pdr->sdf_rules.urr_id;
		} else if (pdr->sdf_mode & 1) {
			return DEFAULT_XDP_ACTION;
		}
	}

	struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
	if (!far) {
		upf_printk("upf: no downlink session far for ip:%pI4 far:%d",
			   &ip4->daddr, far_id);
		return XDP_DROP;
	}

	upf_printk("upf: downlink session for ip:%pI4 far:%d action:%d",
		   &ip4->daddr, far_id, far->action);

	struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
	if (!qer) {
		upf_printk("upf: no downlink session qer for ip:%pI4 qer:%d",
			   &ip4->daddr, qer_id);
		return XDP_DROP;
	}
	upf_printk("upf: qer:%d gate_status:%d mbr:%d", qer_id,
		   qer->dl_gate_status, qer->dl_maximum_bitrate);

	if (far->action & (FAR_BUFF | FAR_NOCP)) {
		upf_printk("upf: need to notify CP for pdr:%d and qfi:%d", pdr->pdr_id, qer->qfi);
		struct nocp notif = { .local_seid = pdr->local_seid, .pdr_id = pdr->pdr_id, .qfi = qer->qfi };
		bpf_ringbuf_output(&nocp_map, (void *)&notif, sizeof(struct nocp), 0);

		/* Technically, we need to buffer the packet here, but this is not
		 * implemented yet. */
		return XDP_DROP;
	}
	if (!(far->action & FAR_FORW)) {
		return XDP_DROP;
	}
	if (!(far->outer_header_creation & OHC_GTP_U_UDP_IPv4)) {
		return XDP_DROP;
	}

	if (qer->dl_gate_status != GATE_STATUS_OPEN)
		return XDP_DROP;

	const __u64 packet_size = ctx->data_end - (void *)ctx->ip4;
	if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start,
						  qer->dl_maximum_bitrate))
		return XDP_DROP;

	__u8 tos = far->transport_level_marking >> 8;
	upf_printk("upf: use mapping %pI4 -> TEID:%d", &ip4->daddr, far->teid);

	/* Update downlink traffic counter */
	{
		__u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
		ctx->downlink_statistics->byte_counter.bytes +=
			packet_size; // Count downlink traffic
	}

	update_urr_bytes(ctx, urr_id);

	return send_to_gtp_tunnel(ctx, far->localip, far->remoteip, tos,
				  qer->qfi, far->teid);
}

/*
 * Downlink processing for IPv6 packets.
 */
static __always_inline enum xdp_action
handle_n6_packet_ipv6(struct packet_context *ctx)
{
	const struct ipv6hdr *ip6 = ctx->ip6;
	struct pdr_info *pdr =
		bpf_map_lookup_elem(&pdrs_downlink_ip6, &ip6->daddr);
	if (!pdr) {
		upf_printk("upf: no downlink session for ip:%pI6c",
			   &ip6->daddr);
		return DEFAULT_XDP_ACTION;
	}

	ctx->interface = INTERFACE_N6;

	__u32 far_id = pdr->far_id;
	__u32 qer_id = pdr->qer_id;
	if (pdr->sdf_mode) {
		struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
		if (match_sdf_filter_ipv6(ctx, sdf)) {
			upf_printk(
				"Packet with source ip:%pI6c and destination ip:%pI6c matches SDF filter",
				&ip6->saddr, &ip6->daddr);
			far_id = pdr->sdf_rules.far_id;
			qer_id = pdr->sdf_rules.qer_id;
		} else if (pdr->sdf_mode & 1) {
			return DEFAULT_XDP_ACTION;
		}
	}

	struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
	if (!far) {
		upf_printk("upf: no downlink session far for ip:%pI6c far:%d",
			   &ip6->daddr, far_id);
		return XDP_DROP;
	}
	upf_printk("upf: downlink session for ip:%pI6c far:%d action:%d",
		   &ip6->daddr, far_id, far->action);

	struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
	if (!qer) {
		upf_printk("upf: no downlink session qer for ip:%pI6c qer:%d",
			   &ip6->daddr, qer_id);
		return XDP_DROP;
	}
	upf_printk("upf: qer:%d gate_status:%d mbr:%d", qer_id,
		   qer->dl_gate_status, qer->dl_maximum_bitrate);

	if (far->action & (FAR_BUFF | FAR_NOCP)) {
		upf_printk("upf: need to notify CP for pdr:%d and qfi:%d", pdr->pdr_id, qer->qfi);
		struct nocp notif = { .local_seid = pdr->local_seid, .pdr_id = pdr->pdr_id, .qfi = qer->qfi };
		bpf_ringbuf_output(&nocp_map, (void *)&notif, sizeof(struct nocp), 0);

		/* Technically, we need to buffer the packet here, but this is not
		 * implemented yet. */
		return XDP_DROP;
	}
	if (!(far->action & FAR_FORW))
		return XDP_DROP;
	if (!(far->outer_header_creation & OHC_GTP_U_UDP_IPv4))
		return XDP_DROP;

	if (qer->dl_gate_status != GATE_STATUS_OPEN)
		return XDP_DROP;

	const __u64 packet_size = ctx->data_end - (void *)ctx->ip6;
	if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start,
						  qer->dl_maximum_bitrate))
		return XDP_DROP;

	__u8 tos = far->transport_level_marking >> 8;
	upf_printk("upf: use mapping %pI6c -> TEID:%d", &ip6->daddr, far->teid);
	return send_to_gtp_tunnel(ctx, far->localip, far->remoteip, tos,
				  qer->qfi, far->teid);
}
