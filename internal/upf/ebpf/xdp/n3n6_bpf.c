/**
 * Copyright 2023 Edgecom LLC
 * SPDX-FileCopyrightText: Ella Networks Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Modified by Ella Networks.
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

#include "xdp/n3_bpf.h"
#include "xdp/n6_bpf.h"

#include "xdp/utils/statistics.h"
#include "xdp/utils/common.h"
#include "xdp/utils/routing.h"

#include "xdp/utils/trace.h"
#include "xdp/utils/profiling.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/nat.h"

/*
 * The datapath is split so the verifier checks each stage on its own budget. A
 * thin entry program (upf_entry_func) classifies by packet type and tail-calls
 * into a stage program: upf_uplink_func (GTP-U decap) or upf_downlink_func
 * (downlink forwarding). Classifying by packet type (not by interface) keeps a
 * single entry working whether N3 and N6 are separate interfaces or the same
 * one. The downlink stage is also the reuse point for UE-to-UE local switching
 * later. See bpf_datapath_split_plan.md.
 */

/* Tail-call program array populated at load with the stage programs. */
#define UPF_CALL_UPLINK 0
#define UPF_CALL_DOWNLINK 1

struct {
	__uint(type, BPF_MAP_TYPE_PROG_ARRAY);
	__type(key, __u32);
	__type(value, __u32);
	__uint(max_entries, 4);
} upf_calls SEC(".maps");

/* N3 uplink: GTP-U-encapsulated traffic from the gNB. */
static __always_inline enum xdp_action handle_uplink_ip4(struct packet_context *ctx)
{
	if (parse_ip4(ctx) == IPPROTO_UDP) {
		struct udphdr *udp = detect_udp_header(ctx, 0);
		if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT) {
			parse_udp(ctx);
			upf_printk("upf: gtp-u received on N3, src=%pI4 dst=%pI4",
				   &ctx->ip4->saddr, &ctx->ip4->daddr);
			enum xdp_action action = handle_gtpu(ctx);
			ctx->statistics->xdp_actions[action &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return action;
		}
	}

	/* Non-GTP traffic on N3 is not uplink user-plane; leave it to the stack. */
	ctx->statistics->xdp_actions[DEFAULT_XDP_ACTION & EUPF_MAX_XDP_ACTION_MASK] +=
		1;
	return DEFAULT_XDP_ACTION;
}

static __always_inline enum xdp_action handle_uplink_ip6(struct packet_context *ctx)
{
	if (parse_ip6(ctx) == IPPROTO_UDP) {
		struct udphdr *udp = detect_udp_header(ctx, 0);
		if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT) {
			parse_udp(ctx);
			upf_printk("upf: gtp-u received on N3 (IPv6 outer)");
			enum xdp_action action = handle_gtpu(ctx);
			ctx->statistics->xdp_actions[action &
						     EUPF_MAX_XDP_ACTION_MASK] +=
				1;
			return action;
		}
	}

	ctx->statistics->xdp_actions[DEFAULT_XDP_ACTION & EUPF_MAX_XDP_ACTION_MASK] +=
		1;
	return DEFAULT_XDP_ACTION;
}

static __always_inline enum xdp_action process_uplink(struct packet_context *ctx)
{
	switch (parse_ethernet(ctx)) {
	case ETH_P_IP:
		return handle_uplink_ip4(ctx);
	case ETH_P_IPV6:
		return handle_uplink_ip6(ctx);
	case ETH_P_ARP:
		upf_printk("upf: arp received on N3. passing to kernel");
		return XDP_PASS;
	}
	return DEFAULT_XDP_ACTION;
}

/* N6 downlink: plain IP traffic from the data network toward a UE. */
static __always_inline enum xdp_action handle_downlink_ip4(struct packet_context *ctx)
{
	int l4_protocol = parse_ip4(ctx);
	if (l4_protocol != IPPROTO_UDP && l4_protocol != IPPROTO_ICMP &&
	    l4_protocol != IPPROTO_TCP) {
		ctx->statistics
			->xdp_actions[DEFAULT_XDP_ACTION & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return DEFAULT_XDP_ACTION;
	}

	ctx->statistics->packet_counters.rx++;
	enum xdp_action action = handle_n6_packet_ipv4(ctx);
	ctx->statistics->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return action;
}

static __always_inline enum xdp_action handle_downlink_ip6(struct packet_context *ctx)
{
	int l4_protocol = parse_ip6(ctx);
	if (l4_protocol != IPPROTO_UDP && l4_protocol != IPPROTO_ICMPV6 &&
	    l4_protocol != IPPROTO_TCP) {
		ctx->statistics
			->xdp_actions[DEFAULT_XDP_ACTION & EUPF_MAX_XDP_ACTION_MASK] +=
			1;
		return DEFAULT_XDP_ACTION;
	}

	ctx->statistics->packet_counters.rx++;
	enum xdp_action action = handle_n6_packet_ipv6(ctx);
	ctx->statistics->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
	return action;
}

static __always_inline enum xdp_action process_downlink(struct packet_context *ctx)
{
	switch (parse_ethernet(ctx)) {
	case ETH_P_IP:
		return handle_downlink_ip4(ctx);
	case ETH_P_IPV6:
		return handle_downlink_ip6(ctx);
	case ETH_P_ARP:
		upf_printk("upf: arp received on N6. passing to kernel");
		return XDP_PASS;
	}
	return DEFAULT_XDP_ACTION;
}

/* get_or_init_stats returns the singleton statistics record for a map, creating
 * it on first use. */
static __always_inline struct upf_statistic *get_or_init_stats(void *stats_map)
{
	const __u32 key = 0;
	struct upf_statistic *statistics = bpf_map_lookup_elem(stats_map, &key);
	if (!statistics) {
		const struct upf_statistic initval = {};
		bpf_map_update_elem(stats_map, &key, &initval, BPF_ANY);
		statistics = bpf_map_lookup_elem(stats_map, &key);
	}

	return statistics;
}

/* upf_uplink_func: tail-call stage for GTP-U uplink traffic. Re-parses from its
 * own ctx (the stack does not survive a tail call). */
SEC("xdp/upf_uplink")
int upf_uplink_func(struct xdp_md *ctx)
{
	struct upf_statistic *statistics = get_or_init_stats(&uplink_statistics);
	if (!statistics)
		return XDP_ABORTED;

	struct packet_context context = {
		.data = (void *)(long)ctx->data,
		.data_end = (const void *)(long)ctx->data_end,
		.xdp_ctx = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N3,
	};

	PROFILE_START(PROF_N3_TOTAL);
	enum xdp_action ret = process_uplink(&context);
	PROFILE_END(PROF_N3_TOTAL);
	return ret;
}

/* upf_downlink_func: tail-call stage for plain downlink traffic toward a UE. */
SEC("xdp/upf_downlink")
int upf_downlink_func(struct xdp_md *ctx)
{
	struct upf_statistic *statistics = get_or_init_stats(&downlink_statistics);
	if (!statistics)
		return XDP_ABORTED;

	struct packet_context context = {
		.data = (void *)(long)ctx->data,
		.data_end = (const void *)(long)ctx->data_end,
		.xdp_ctx = ctx,
		.statistics = statistics,
		.interface = INTERFACE_N6,
	};

	PROFILE_START(PROF_N6_TOTAL);
	enum xdp_action ret = process_downlink(&context);
	PROFILE_END(PROF_N6_TOTAL);
	return ret;
}

/* upf_entry_func: attached to the N3/N6 interface(s). Classifies by packet type
 * — GTP-U (UDP :2152) is uplink, everything else is downlink — and tail-calls
 * the matching stage. Packet-type classification (not interface) keeps this
 * correct when N3 and N6 share one interface. */
SEC("xdp/upf_entry")
int upf_entry_func(struct xdp_md *ctx)
{
	struct packet_context context = {
		.data = (void *)(long)ctx->data,
		.data_end = (const void *)(long)ctx->data_end,
		.xdp_ctx = ctx,
	};

	__u16 l3_protocol = parse_ethernet(&context);
	__u32 index = UPF_CALL_DOWNLINK;

	if (l3_protocol == ETH_P_ARP) {
		upf_printk("upf: arp received. passing to kernel");
		return XDP_PASS;
	}

	if (l3_protocol == ETH_P_IP) {
		if (parse_ip4(&context) == IPPROTO_UDP) {
			struct udphdr *udp = detect_udp_header(&context, 0);
			if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT)
				index = UPF_CALL_UPLINK;
		}
	} else if (l3_protocol == ETH_P_IPV6) {
		if (parse_ip6(&context) == IPPROTO_UDP) {
			struct udphdr *udp = detect_udp_header(&context, 0);
			if (udp && bpf_ntohs(udp->dest) == GTP_UDP_PORT)
				index = UPF_CALL_UPLINK;
		}
	} else {
		return DEFAULT_XDP_ACTION;
	}

	bpf_tail_call(ctx, &upf_calls, index);

	/* Only reached if the stage program is not populated in upf_calls. */
	return DEFAULT_XDP_ACTION;
}

char _license[] SEC("license") = "GPL";
