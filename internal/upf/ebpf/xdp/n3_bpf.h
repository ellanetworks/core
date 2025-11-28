// Copyright 2025 Ella Networks

#pragma once

#include "xdp/utils/routing.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "xdp/utils/common.h"
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
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} pdrs_uplink SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} uplink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
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

	ctx->interface = INTERFACE_N3;

	__u32 far_id = pdr->far_id;
	__u32 qer_id = pdr->qer_id;
	__u32 urr_id = pdr->urr_id;
	__u8 outer_header_removal = pdr->outer_header_removal;

	/* If an SDF is configured, match it against the inner packet */
	if (pdr->sdf_mode) {
		struct packet_context inner_context = {
			.data = (char *)(long)ctx->data,
			.data_end = (const char *)(long)ctx->data_end,
		};

		if (inner_context.data + 1 > inner_context.data_end)
			return DEFAULT_XDP_ACTION;
		int eth_protocol = guess_eth_protocol(inner_context.data);
		switch (eth_protocol) {
		case ETH_P_IP_BE: {
			int ip_protocol = parse_ip4(&inner_context);
			if (-1 == ip_protocol) {
				upf_printk("upf: unable to parse IPv4 header");
				return DEFAULT_XDP_ACTION;
			}
			if (-1 == parse_l4(ip_protocol, &inner_context)) {
				upf_printk("upf: unable to parse L4 header");
				return DEFAULT_XDP_ACTION;
			}
			const struct sdf_filter *sdf =
				&pdr->sdf_rules.sdf_filter;
			if (match_sdf_filter_ipv4(&inner_context, sdf)) {
				upf_printk("upf: sdf filter matches teid:%d",
					   teid);
				far_id = pdr->sdf_rules.far_id;
				qer_id = pdr->sdf_rules.qer_id;
				urr_id = pdr->sdf_rules.urr_id;
				outer_header_removal =
					pdr->sdf_rules.outer_header_removal;
			} else {
				upf_printk(
					"upf: sdf filter doesn't match teid:%d",
					teid);
				if (pdr->sdf_mode & 1)
					return DEFAULT_XDP_ACTION;
			}
			break;
		}
		case ETH_P_IPV6_BE: {
			int ip_protocol = parse_ip6(&inner_context);
			if (ip_protocol == -1) {
				upf_printk("upf: unable to parse IPv6 header");
				return DEFAULT_XDP_ACTION;
			}
			if (-1 == parse_l4(ip_protocol, &inner_context)) {
				upf_printk("upf: unable to parse L4 header");
				return DEFAULT_XDP_ACTION;
			}
			const struct sdf_filter *sdf =
				&pdr->sdf_rules.sdf_filter;
			if (match_sdf_filter_ipv6(&inner_context, sdf)) {
				upf_printk("upf: sdf filter matches teid:%d",
					   teid);
				far_id = pdr->sdf_rules.far_id;
				qer_id = pdr->sdf_rules.qer_id;
				urr_id = pdr->sdf_rules.urr_id;
				outer_header_removal =
					pdr->sdf_rules.outer_header_removal;
			} else {
				upf_printk(
					"upf: sdf filter doesn't match teid:%d",
					teid);
				if (pdr->sdf_mode & 1)
					return DEFAULT_XDP_ACTION;
			}
			break;
		}
		default:
			upf_printk(
				"upf: unsupported inner ethernet protocol: %d",
				eth_protocol);
			if (pdr->sdf_mode & 1)
				return DEFAULT_XDP_ACTION;
			break;
		}
	}

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

	const __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
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
		ctx->uplink_statistics->byte_counter.bytes += packet_size;
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
