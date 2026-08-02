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
#include <bpf/bpf_helpers.h>
#include "bpf/utils/statistics.h"
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
volatile const int n3_vlan;
volatile const int n3_vlan = 0;
volatile const int n6_vlan;
volatile const int n6_vlan = 0;

struct vlan_hdr {
	__be16 h_vlan_TCI;
	__be16 h_vlan_encapsulated_proto;
};

/* Header cursor to keep track of current parsing position */
struct packet_context {
	void *data;
	const void *data_end;
	struct upf_statistic *statistics;
	struct counters *counter;
	struct __ctx_buff *ctx_buff;
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
	/* Set by drop_with()/abort_with(), read once by record_action(). */
	enum upf_drop_reason drop_reason;
	__u8 interface : 1;
};

/* A merged buffer holds several datagrams behind one set of headers, so
 * neither encapsulation nor decapsulation can produce a correct frame from
 * it. The kernel merges on receive with GRO, and a veth or virtio peer can
 * deliver one without any merge on this side. */
static __always_inline int frame_is_merged(const struct packet_context *ctx)
{
	return ctx_gso_segs(ctx->ctx_buff) > 1;
}

/* Every drop in the datapath returns through one of these, so the reason
 * reaches the counter; helpers that decide without a packet_context leave the
 * translation to their caller. The two differ only in the hook verdict —
 * XDP_ABORTED fires trace_xdp_exception — and are counted alike. */
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

/* The transmit helpers decide without a packet_context, so the datapath calls
 * them through these wrappers, which attach the cause of their one failure
 * mode. */
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

/* Same rule as drop_reported. */
static __always_inline enum ctx_action
abort_reported(struct packet_context *ctx, enum upf_drop_reason fallback)
{
	if (ctx->drop_reason == UPF_DROP_UNSPEC)
		ctx->drop_reason = fallback;

	return CTX_ACT_ABORTED;
}

/* VLAN id a frame leaves with, keyed on the logical side it is bound for
 * and not the egress ifindex: when N3 and N6 are sub-interfaces of one
 * NIC, both resolve to the same master and the ifindex cannot tell the two
 * directions apart. */
static __always_inline int
egress_vlan_forwarded(const struct packet_context *ctx)
{
	return ctx->interface == INTERFACE_N3 ? n6_vlan : n3_vlan;
}

/* Counterpart for a frame the datapath answers itself, which leaves by the
 * side it arrived on. */
static __always_inline int
egress_vlan_reflected(const struct packet_context *ctx)
{
	return ctx->interface == INTERFACE_N3 ? n3_vlan : n6_vlan;
}

/* Every program that owns a frame's outcome returns through this exactly
 * once, at its boundary, so the frame is counted neither twice nor not at
 * all.
 *
 * The reason is read from the context rather than passed in because C does
 * not order argument evaluation: `record_action(ctx, handle(ctx))` would
 * otherwise be free to sample it before handle() sets it. */
static __always_inline int record_action(struct packet_context *ctx,
					 enum ctx_action action)
{
	if (action == CTX_ACT_DROP || action == CTX_ACT_ABORTED)
		ctx->statistics->drop_reasons[ctx->drop_reason &
					      UPF_DROP_REASON_MASK] += 1;
	else
		ctx->statistics->forwarded_actions[action & UPF_ACTION_MASK] +=
			1;

	return ctx_verdict(action);
}
