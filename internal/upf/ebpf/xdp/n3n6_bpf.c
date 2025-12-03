/**
 * Copyright 2023 Edgecom LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include "xdp/utils/common.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include <linux/in.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <sys/socket.h>

#include "xdp/n3_bpf.h"
#include "xdp/n6_bpf.h"
#include "xdp/utils/statistics.h"

#include "xdp/utils/trace.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/nat.h"

static __always_inline enum xdp_action handle_ip4(struct packet_context *ctx)
{
	enum xdp_action action;
	int l4_protocol = parse_ip4(ctx);
	if (l4_protocol == IPPROTO_UDP && GTP_UDP_PORT == parse_udp(ctx)) {
		upf_printk("upf: gtp-u received");
		action = handle_gtpu(ctx);
		ctx->uplink_statistics
			->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return action;
	} else if (l4_protocol != IPPROTO_ICMP && l4_protocol != IPPROTO_UDP &&
		   l4_protocol != IPPROTO_TCP) {
		action = DEFAULT_XDP_ACTION;
		ctx->downlink_statistics
			->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
		return DEFAULT_XDP_ACTION;
	}
	ctx->downlink_statistics->packet_counters.rx++;
	action = handle_n6_packet_ipv4(ctx);
	ctx->downlink_statistics
		->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return action;
}

/*
 * IPv6 handler.
 */
static __always_inline enum xdp_action handle_ip6(struct packet_context *ctx)
{
	int l4_protocol = parse_ip6(ctx);
	switch (l4_protocol) {
	case IPPROTO_ICMPV6:
		upf_printk("upf: icmp received. passing to kernel");
		return XDP_PASS;
	case IPPROTO_UDP:
		break;
	case IPPROTO_TCP:
		break;
	default:
		return DEFAULT_XDP_ACTION;
	}
	return handle_n6_packet_ipv6(ctx);
}

static __always_inline enum xdp_action
process_packet(struct packet_context *ctx)
{
	__u16 l3_protocol = parse_ethernet(ctx);
	switch (l3_protocol) {
	case ETH_P_IP:
		return handle_ip4(ctx);
	case ETH_P_IPV6:
		return handle_ip6(ctx);
	case ETH_P_ARP:
		upf_printk("upf: arp received. passing to kernel");
		return XDP_PASS;
	}
	return DEFAULT_XDP_ACTION;
}

SEC("xdp/upf_n3_n6_entrypoint")
int upf_n3_n6_entrypoint_func(struct xdp_md *ctx)
{
	const __u32 key = 0;
	struct upf_statistic *uplink_statistic =
		bpf_map_lookup_elem(&uplink_statistics, &key);
	if (!uplink_statistic) {
		const struct upf_statistic initval = {};
		bpf_map_update_elem(&uplink_statistics, &key, &initval,
				    BPF_ANY);
		uplink_statistic =
			bpf_map_lookup_elem(&uplink_statistics, &key);
		if (!uplink_statistic)
			return XDP_ABORTED;
	}
	struct upf_statistic *downlink_statistic =
		bpf_map_lookup_elem(&downlink_statistics, &key);
	if (!downlink_statistic) {
		const struct upf_statistic initval = {};
		bpf_map_update_elem(&downlink_statistics, &key, &initval,
				    BPF_ANY);
		downlink_statistic =
			bpf_map_lookup_elem(&downlink_statistics, &key);
		if (!downlink_statistic)
			return XDP_ABORTED;
	}

	struct packet_context context = {
		.data = (void *)(long)ctx->data,
		.data_end = (const void *)(long)ctx->data_end,
		.xdp_ctx = ctx,
		.downlink_statistics = downlink_statistic,
		.uplink_statistics = uplink_statistic,
	};

	return process_packet(&context);
}

char _license[] SEC("license") = "GPL";
