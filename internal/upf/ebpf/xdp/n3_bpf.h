// Copyright 2025 Ella Networks

#pragma once

#include "xdp/utils/routing.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "xdp/utils/common.h"
#include "xdp/utils/frag_needed.h"
#include "xdp/utils/gtp.h"
#include "xdp/utils/pdr.h"
#include "xdp/utils/qer.h"
#include "xdp/utils/urr.h"
#include "xdp/utils/statistics.h"

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u32);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_UPLINK_SIZE);
} pdrs_uplink SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
} uplink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
} uplink_statistics SEC(".maps");

static __always_inline enum xdp_action
handle_gtp_packet(struct packet_context *ctx)
{
	if (!ctx->gtp) {
		upf_printk("upf: unexpected packet context. no gtp header");
		return DEFAULT_XDP_ACTION;
	}

	__u32 teid = bpf_htonl(ctx->gtp->teid);
	/* Lookup uplink session using the TEID */
	struct pdr_info *pdr = bpf_map_lookup_elem(&pdrs_uplink, &teid);
	if (!pdr) {
		upf_printk("upf: no session for teid:%d", teid);
		return DEFAULT_XDP_ACTION;
	}

	__u32 mtu_len = 0;
	long ret = bpf_check_mtu(ctx->xdp_ctx, n6_ifindex, &mtu_len, -GTP_ENCAP_SIZE, 0);
	if (ret < 0) {
		ctx->statistics->xdp_actions[XDP_ABORTED & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return XDP_ABORTED;
	}
	if (ret > 0) {
		bpf_printk("upf: n3 packet too large");
		return frag_needed(ctx, mtu_len);
	}

	ctx->interface = INTERFACE_N3;

	__u32 far_id = pdr->far_id;
	__u32 qer_id = pdr->qer_id;
	__u32 urr_id = pdr->urr_id;
	__u8 outer_header_removal = pdr->outer_header_removal;

	/* Lookup FAR and QER (both expected to be for uplink) */
	struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
	if (!far) {
		upf_printk("upf: no session far for teid:%d far:%d", teid,
			   far_id);
		return XDP_DROP;
	}
	upf_printk("upf: far:%d action:%d outer_header_creation:%d", far_id,
		   far->action, far->outer_header_creation);
	if (!(far->action & FAR_FORW))
		return XDP_DROP;
	struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
	if (!qer) {
		upf_printk("upf: no session qer for teid:%d qer:%d", teid,
			   qer_id);
		return XDP_DROP;
	}
	upf_printk("upf: qer:%d gate_status:%d mbr:%d", qer_id,
		   qer->ul_gate_status, qer->ul_maximum_bitrate);
	if (qer->ul_gate_status != GATE_STATUS_OPEN)
		return XDP_DROP;

	const __u64 packet_size = ctx->data_end - ctx->data;
	if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->ul_start,
						  qer->ul_maximum_bitrate))
		return XDP_DROP;

	upf_printk("upf: session for teid:%d far:%d outer_header_removal:%d",
		   teid, pdr->far_id, outer_header_removal);
	if (far->outer_header_creation & OHC_GTP_U_UDP_IPv4) {
		upf_printk("upf: session for teid:%d -> %d remote:%pI4", teid,
			   far->teid, &far->remoteip);
		update_gtp_tunnel(ctx, far->localip, far->remoteip, 0,
				  far->teid);
	} else if (outer_header_removal == OHR_GTP_U_UDP_IPv4) {
		long result = remove_gtp_header(ctx);
		if (result) {
			upf_printk(
				"upf: handle_gtp_packet: can't remove gtp header: %d",
				result);
			return XDP_ABORTED;
		}
	}

	/* Account uplink traffic */
	{
		__u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
		ctx->statistics->byte_counter.bytes += packet_size;
	}

	update_urr_bytes(ctx, urr_id);

	const __u32 key = 0;
	struct route_stat *route_statistic =
		bpf_map_lookup_elem(&uplink_route_stats, &key);
	if (!route_statistic)
		return XDP_ABORTED;

	if (ctx->ip4)
		return route_ipv4(ctx, route_statistic);
	else if (ctx->ip6)
		return route_ipv6(ctx, route_statistic);
	else
		return XDP_ABORTED;
}

static __always_inline enum xdp_action handle_gtpu(struct packet_context *ctx)
{
	int pdu_type = parse_gtp(ctx);
	switch (pdu_type) {
	case GTPU_G_PDU:
		return handle_gtp_packet(ctx);
	case GTPU_ECHO_REQUEST:
		upf_printk("upf: gtp echo request [ %pI4 -> %pI4 ]",
			   &ctx->ip4->saddr, &ctx->ip4->daddr);
		return handle_echo_request(ctx);
	case GTPU_ECHO_RESPONSE:
		return XDP_PASS;
	case GTPU_ERROR_INDICATION:
	case GTPU_SUPPORTED_EXTENSION_HEADERS_NOTIFICATION:
	case GTPU_END_MARKER:
		return DEFAULT_XDP_ACTION;
	default:
		upf_printk("upf: unexpected gtp message: type=%d", pdu_type);
		return DEFAULT_XDP_ACTION;
	}
}
