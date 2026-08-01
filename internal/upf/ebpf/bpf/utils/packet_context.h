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
	/* Why the frame was not forwarded, set by drop_with()/abort_with() at the
	 * drop site and read once at the entrypoint that counts the outcome. */
	enum upf_drop_reason drop_reason;
	__u8 interface : 1;
};

/* Do not forward this frame, and say why.
 *
 * Every `return CTX_ACT_DROP` and `return CTX_ACT_ABORTED` in the datapath
 * goes through one of these, so that the reason reaches the counter. The
 * exceptions are the few helpers that decide without a packet_context — their
 * callers translate the returned action into a reason instead.
 *
 * The distinction between the two is the hook verdict, not the accounting:
 * XDP_ABORTED fires the trace_xdp_exception tracepoint, so a datapath failure
 * stays visible to `xdpdump` and friends, while both are counted as drops. */
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
 * mode. Nothing outside them should call ctx_tx_back or ctx_redirect_out. */
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

/* Record the cause without deciding the verdict, for a helper that reports
 * failure through its return value and leaves the drop to its caller. */
static __always_inline void set_drop_reason(struct packet_context *ctx,
					    enum upf_drop_reason reason)
{
	ctx->drop_reason = reason;
}

/* Drop, keeping the specific cause a helper already recorded. `fallback`
 * covers the paths where none did. The inner site knows more than the caller,
 * so it wins; without this the caller's coarse reason would overwrite it. */
static __always_inline enum ctx_action
drop_reported(struct packet_context *ctx, enum upf_drop_reason fallback)
{
	if (ctx->drop_reason == UPF_DROP_UNSPEC)
		ctx->drop_reason = fallback;

	return CTX_ACT_DROP;
}

/* abort_with, keeping an inner helper's cause. Same rule as drop_reported. */
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

/* Record what the datapath decided about this frame and convert it to the
 * hook's verdict.
 *
 * Every program that owns a frame's outcome returns through this exactly
 * once, at its boundary: one increment per frame, and no path can skip the
 * count or take it twice. Forwards and drops go to separate arrays, so the
 * two totals are disjoint and sum to the frames the program handled.
 *
 * The reason is read from the context rather than passed in, so it cannot be
 * sampled before the call that sets it: `record_action(ctx, handle(ctx))`
 * evaluates the action first, and this body reads ctx->drop_reason after. */
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
