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

#pragma once

#include "bpf/ctx/ctx.h"
#include <stdbool.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include "bpf/utils/statistics.h"
#include "bpf/utils/traffic_class.h"
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/types.h>
#include <linux/udp.h>
#include <linux/tcp.h>
#include <linux/icmp.h>
#include "bpf/utils/gtpu.h"

#define INTERFACE_N3 0x0
#define INTERFACE_N6 0x1

volatile const int n3_ifindex;
volatile const int n3_ifindex = 0;
volatile const int n6_ifindex;
volatile const int n6_ifindex = 0;
volatile const __u32 n3_routing_ifindex = 0;
volatile const __u32 n6_routing_ifindex = 0;
volatile const int n3_vlan;
volatile const int n3_vlan = 0;
volatile const int n6_vlan;
volatile const int n6_vlan = 0;

struct vlan_hdr {
	__be16 h_vlan_TCI;
	__be16 h_vlan_encapsulated_proto;
};

#define MAX_NETWORK_INSTANCES 16

struct network_instance {
	__u32 ifindex;
	__u32 routing_ifindex;
	__u32 vlan;
};

struct network_instance_key {
	__u32 ifindex;
	__u32 vlan;
};

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct network_instance);
	__uint(max_entries, MAX_NETWORK_INSTANCES);
} network_instances SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct network_instance_key);
	__type(value, __u32);
	__uint(max_entries, MAX_NETWORK_INSTANCES);
} network_instance_classifier SEC(".maps");

/* Header cursor to keep track of current parsing position */
struct packet_context {
	void *data;
	const void *data_end;
	struct upf_statistic *statistics;
	struct counters *counter;
	struct __ctx_buff *ctx_buff;
	__u32 routing_ifindex;
	struct ethhdr *eth;
	struct iphdr *ip4;
	struct ipv6hdr *ip6;
	struct udphdr *udp;
	struct tcphdr *tcp;
	struct gtpuhdr *gtp;
	struct icmphdr *icmp;
	struct vlan_hdr *vlan;
	/* Length of the GTP-U header parse_gtp consumed: mandatory header plus the
	 * optional word and any extension headers. Drives uplink decapsulation. */
	__u32 gtp_hdr_len;
	__u16 l3_hdr_len;
	__u8 qfi;
	__u8 l4_proto;
	__u16 l4_sport;
	__u16 l4_dport;
	__be16 frag_nat_id;
	/* IPv6 fragment identification, from the chain walk. */
	__be32 frag_id6;
	/* Set by drop_with()/abort_with(), read once by record_action(). */
	enum upf_drop_reason drop_reason;
	__u8 interface : 1;
	__u8 l4_unavailable : 1;
	__u8 exthdr_invalid : 1;
	/* ctx->data is still on the fixed header. */
	__u8 l4_resolved : 1;
	/* Ports came from the fragment map rather than the packet, so this
	 * fragment has none of its own to record. Distinct from l4_resolved,
	 * which the IPv6 chain walk sets for every packet. */
	__u8 frag_recovered : 1;
	__u8 is_fragment : 1;
	__u8 nat_exempt : 1;
	__u8 l4_ports : 1;
	__u8 ni_id;
};

__noinline __weak __u32
routing_ingress_ifindex(struct __ctx_buff *ctx, __u8 interface)
{
	__u32 ifindex = ctx_ingress_ifindex(ctx);

	if (interface == INTERFACE_N3 && ifindex == (__u32)n3_ifindex)
		return n3_routing_ifindex;
	if (interface == INTERFACE_N6 && ifindex == (__u32)n6_ifindex)
		return n6_routing_ifindex;
	return ifindex;
}

__noinline __weak __u32 network_instance_ifindex(__u32 ni_id)
{
	if (ni_id == 0)
		return n6_ifindex;

	const struct network_instance *ni =
		bpf_map_lookup_elem(&network_instances, &ni_id);

	return ni ? ni->ifindex : 0;
}

__noinline __weak __u32 network_instance_routing_ifindex(__u32 ni_id)
{
	if (ni_id == 0)
		return n6_routing_ifindex;

	const struct network_instance *ni =
		bpf_map_lookup_elem(&network_instances, &ni_id);

	return ni ? ni->routing_ifindex : 0;
}

__noinline __weak __u32 network_instance_vlan(__u32 ni_id)
{
	if (ni_id == 0)
		return n6_vlan;

	const struct network_instance *ni =
		bpf_map_lookup_elem(&network_instances, &ni_id);

	return ni ? ni->vlan : 0;
}

__noinline __weak bool network_instance_available(__u32 ni_id)
{
	return ni_id == 0 || network_instance_ifindex(ni_id) != 0;
}

__noinline __weak bool network_instance_isolated(__u32 ni_id)
{
	return ni_id != 0;
}

__noinline __weak __u32 uplink_routing_ifindex(__u32 ni_id, __u32 ingress)
{
	return ni_id ? network_instance_routing_ifindex(ni_id) : ingress;
}

static __always_inline bool ctx_select_ni(struct packet_context *ctx,
					  __u32 ni_id)
{
	ctx->ni_id = (__u8)ni_id;

	return network_instance_available(ni_id);
}

static __always_inline __u32 ctx_n6_ifindex(const struct packet_context *ctx)
{
	return network_instance_ifindex(ctx->ni_id);
}

static __always_inline __u32
ctx_n6_routing_ifindex(const struct packet_context *ctx)
{
	return network_instance_routing_ifindex(ctx->ni_id);
}

static __always_inline int ctx_n6_vlan(const struct packet_context *ctx)
{
	return (int)network_instance_vlan(ctx->ni_id);
}

static __always_inline __u32 frame_vlan_id(const struct packet_context *ctx)
{
#if CTX_INBAND_VLAN
	if (ctx->vlan && (const void *)(ctx->vlan + 1) <= ctx->data_end)
		return bpf_ntohs(ctx->vlan->h_vlan_TCI) & 0x0fff;
	return 0;
#else
	return ctx->ctx_buff->vlan_present ? ctx->ctx_buff->vlan_tci & 0x0fff : 0;
#endif
}

__noinline __weak int ingress_network_instance(__u32 ifindex, __u32 vlan)
{
	struct network_instance_key key = { .ifindex = ifindex, .vlan = vlan };
	const __u32 *ni_id =
		bpf_map_lookup_elem(&network_instance_classifier, &key);
	if (ni_id)
		return (int)*ni_id;

	if (!n3_ifindex || !n6_ifindex || ifindex == (__u32)n6_ifindex ||
	    ifindex == (__u32)n3_ifindex)
		return 0;

	return -1;
}

__noinline __weak int downlink_ingress_ni(struct __ctx_buff *ctx)
{
	__u32 vlan = 0;

#if CTX_INBAND_VLAN
	void *data = ctx_data(ctx);
	const void *data_end = ctx_data_end(ctx);
	struct ethhdr *eth = data;

	if ((void *)(eth + 1) > data_end)
		return -1;

	if (eth->h_proto == bpf_htons(ETH_P_8021Q) ||
	    eth->h_proto == bpf_htons(ETH_P_8021AD)) {
		struct vlan_hdr *vh = (struct vlan_hdr *)(eth + 1);

		if ((void *)(vh + 1) > data_end)
			return -1;

		vlan = bpf_ntohs(vh->h_vlan_TCI) & 0x0fff;
	}
#else
	vlan = ctx->vlan_present ? ctx->vlan_tci & 0x0fff : 0;
#endif

	return ingress_network_instance(ctx_ingress_ifindex(ctx), vlan);
}

__noinline __weak int downlink_session_ni(struct __ctx_buff *ctx)
{
	return downlink_ingress_ni(ctx);
}

__noinline __weak int downlink_reply_vlan(struct __ctx_buff *ctx)
{
	int ni_id = downlink_ingress_ni(ctx);

	return ni_id > 0 ? (int)network_instance_vlan((__u32)ni_id) : n6_vlan;
}

/* A merged buffer holds several datagrams behind one set of headers, so
 * neither encapsulation nor decapsulation can produce a correct frame from it.
 * gso_size only says the buffer is GSO; gso_segs is what answers the question.
 * A source that reports no count (SKB_GSO_DODGY) counts as merged: it sets
 * gso_size only for a buffer it had already segmented. */
static __always_inline int frame_is_merged(const struct packet_context *ctx)
{
	if (ctx_gso_size(ctx->ctx_buff) == 0)
		return 0;

	return ctx_gso_segs(ctx->ctx_buff) != 1;
}

static __always_inline bool ctx_action_forwards(enum ctx_action action)
{
	return action != CTX_ACT_DROP && action != CTX_ACT_ABORTED;
}

/* Every drop returns through one of these, so the reason reaches the counter.
 * The two differ only in the hook verdict — XDP_ABORTED fires
 * trace_xdp_exception — and are counted alike. */
static __always_inline enum ctx_action drop_with(struct packet_context *ctx,
						 enum upf_drop_reason reason)
{
	ctx->drop_reason = reason;

	return CTX_ACT_DROP;
}

static __always_inline enum ctx_action abort_with(struct packet_context *ctx,
						  enum upf_drop_reason reason)
{
	ctx->drop_reason = reason;

	return CTX_ACT_ABORTED;
}

/* The transmit helpers take no packet_context, so these wrappers attach the
 * cause of their one failure mode. */
static __always_inline enum ctx_action tx_failed(struct packet_context *ctx,
						 enum ctx_action action)
{
	if (action == CTX_ACT_ABORTED)
		return abort_with(ctx, UPF_DROP_INTERNAL_TX_FAILED);

	return action;
}

static __always_inline enum ctx_action tx_back(struct packet_context *ctx,
					       int egress_vid)
{
	return tx_failed(ctx, ctx_tx_back(ctx->ctx_buff, egress_vid));
}

static __always_inline enum ctx_action
redirect_out(struct packet_context *ctx, __u32 ifindex, int egress_vid)
{
	return tx_failed(ctx, ctx_redirect_out(ctx->ctx_buff, ifindex,
					       egress_vid));
}

/* For a helper that reports failure through its return value. */
static __always_inline void set_drop_reason(struct packet_context *ctx,
					    enum upf_drop_reason reason)
{
	ctx->drop_reason = reason;
}

/* Keep the specific cause an inner helper already recorded; `fallback` covers
 * the paths where none did. */
static __always_inline enum ctx_action
drop_reported(struct packet_context *ctx, enum upf_drop_reason fallback)
{
	if (ctx->drop_reason == UPF_DROP_UNSPEC)
		ctx->drop_reason = fallback;

	return CTX_ACT_DROP;
}

static __always_inline enum ctx_action
abort_reported(struct packet_context *ctx, enum upf_drop_reason fallback)
{
	if (ctx->drop_reason == UPF_DROP_UNSPEC)
		ctx->drop_reason = fallback;

	return CTX_ACT_ABORTED;
}

/* Keyed on the logical side the frame is bound for, not the egress ifindex:
 * with N3 and N6 on sub-interfaces of one NIC both resolve to the same master,
 * which cannot tell the two directions apart. */
static __always_inline int
egress_vlan_forwarded(const struct packet_context *ctx)
{
	return ctx->interface == INTERFACE_N3 ? ctx_n6_vlan(ctx) : n3_vlan;
}

/* Counterpart for a frame the datapath answers itself. */
static __always_inline int
egress_vlan_reflected(const struct packet_context *ctx)
{
	return ctx->interface == INTERFACE_N3 ?
		       n3_vlan :
		       downlink_reply_vlan(ctx->ctx_buff);
}

/* Every program that owns a frame's outcome returns through this exactly once,
 * so the frame is counted neither twice nor not at all.
 *
 * The reason is read from the context rather than passed in because C does not
 * order argument evaluation: `record_action(ctx, handle(ctx))` would otherwise
 * be free to sample it before handle() sets it. */
static __always_inline int record_action(struct packet_context *ctx,
					 enum ctx_action action)
{
	if (!ctx_action_forwards(action)) {
		ctx->statistics->drop_reasons[ctx->drop_reason &
					      UPF_DROP_REASON_MASK] += 1;
		traffic_class_settle(ctx->drop_reason);
	} else {
		ctx->statistics->forwarded_actions[action & UPF_ACTION_MASK] +=
			1;
		traffic_class_release();
	}

	return ctx_verdict(action);
}
