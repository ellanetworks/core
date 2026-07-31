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

#include "bpf/utils/flow.h"
#include "bpf/utils/packet_context.h"
#include <linux/bpf.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/types.h>
#include <sys/socket.h>

#include "bpf/utils/nat.h"
#include "bpf/utils/profiling.h"
#include "bpf/utils/trace.h"

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 16384);
} no_neigh_map SEC(".maps");

// The egress ifindex is carried alongside the nexthop because an IPv6
// link-local nexthop is ambiguous: every interface has an fe80::/64.
struct no_neigh_event {
	__u32 ifindex;
	__u32 family;
	__u8 addr[16];
};

// Decoded by hand in parseNoNeighEvent (internal/upf/upf.go).
_Static_assert(sizeof(struct no_neigh_event) == 24,
	       "no_neigh_event layout must match parseNoNeighEvent");

struct route_stat {
	__u64 fib_lookup_ip4_cache;
	__u64 fib_lookup_ip4_success;
	__u64 fib_lookup_ip4_no_neigh;
	__u64 fib_lookup_ip4_blackhole;
	__u64 fib_lookup_ip4_unreachable;
	__u64 fib_lookup_ip4_prohibit;
	__u64 fib_lookup_ip4_no_src_addr;
	__u64 fib_lookup_ip4_frag_needed;
	__u64 fib_lookup_ip4_not_fwded;
	__u64 fib_lookup_ip4_fwd_disabled;
	__u64 fib_lookup_ip4_unsupp_lwt;
	__u64 ip4_ifindex_mismatch;
	__u64 fib_lookup_ip6_cache;
	__u64 fib_lookup_ip6_success;
	__u64 fib_lookup_ip6_no_neigh;
	__u64 fib_lookup_ip6_blackhole;
	__u64 fib_lookup_ip6_unreachable;
	__u64 fib_lookup_ip6_prohibit;
	__u64 fib_lookup_ip6_no_src_addr;
	__u64 fib_lookup_ip6_frag_needed;
	__u64 fib_lookup_ip6_not_fwded;
	__u64 fib_lookup_ip6_fwd_disabled;
	__u64 fib_lookup_ip6_unsupp_lwt;
	__u64 ip6_ifindex_mismatch;
	__u64 fib_lookup_ip4_error;
	__u64 fib_lookup_ip6_error;
};

static __always_inline enum ctx_action
do_route_ipv4(struct packet_context *ctx, struct bpf_fib_lookup *fib_params,
	      struct route_stat *statistic, bool trust_fib)
{
	/*
	 * trust_fib: forward to the egress interface the kernel routing table
	 * chose (used by the veth RA-injection path). The N3<->N6 ifindex
	 * enforcement below assumes ingress on N3 or N6 and does not apply when
	 * the packet originates from the injection veth.
	 */
	if (trust_fib) {
		__builtin_memcpy(ctx->eth->h_source, fib_params->smac,
				 ETH_ALEN);
		__builtin_memcpy(ctx->eth->h_dest, fib_params->dmac, ETH_ALEN);
		return ctx_redirect_out(ctx->ctx_buff, fib_params->ifindex,
					egress_vlan_forwarded(ctx));
	}

	__u32 expected_ifindex;

	if (ctx->interface == INTERFACE_N3) {
		expected_ifindex = n6_ifindex;
	} else {
		expected_ifindex = n3_ifindex;
	}

	if (n3_ifindex != 0 && n6_ifindex != 0 &&
	    fib_params->ifindex != expected_ifindex) {
		upf_printk("upf: ifindex mismatch: fib=%d expected=%d",
			   fib_params->ifindex, expected_ifindex);
		statistic->ip4_ifindex_mismatch += 1;
		return CTX_ACT_DROP;
	}

	__builtin_memcpy(ctx->eth->h_source, fib_params->smac, ETH_ALEN);
	__builtin_memcpy(ctx->eth->h_dest, fib_params->dmac, ETH_ALEN);

	/* GTP-U transport addresses are the UPF's and its peer's (TS 29.281
	 * §4.4.3), and no downlink path reverses a translation of them. */
	if (ctx->interface == INTERFACE_N3 && !ctx->gtp) {
		if (masquerade) {
			PROFILE_START(PROF_N3_NAT);
			int nat_ok = source_nat(ctx, fib_params);
			PROFILE_END(PROF_N3_NAT);
			if (!nat_ok) {
				return CTX_ACT_DROP;
			}
		}
	}

	if (expected_ifindex == ctx_ingress_ifindex(ctx->ctx_buff))
		return ctx_tx_back(ctx->ctx_buff, egress_vlan_forwarded(ctx));
	return ctx_redirect_out(ctx->ctx_buff, expected_ifindex,
				egress_vlan_forwarded(ctx));
}

static __always_inline enum ctx_action
do_route_ipv6(struct packet_context *ctx, struct bpf_fib_lookup *fib_params,
	      struct route_stat *statistic, bool trust_fib)
{
	if (trust_fib) {
		__builtin_memcpy(ctx->eth->h_source, fib_params->smac,
				 ETH_ALEN);
		__builtin_memcpy(ctx->eth->h_dest, fib_params->dmac, ETH_ALEN);
		return ctx_redirect_out(ctx->ctx_buff, fib_params->ifindex,
					egress_vlan_forwarded(ctx));
	}

	__u32 expected_ifindex;

	if (ctx->interface == INTERFACE_N3) {
		expected_ifindex = n6_ifindex;
	} else {
		expected_ifindex = n3_ifindex;
	}

	if (n3_ifindex != 0 && n6_ifindex != 0 &&
	    fib_params->ifindex != expected_ifindex) {
		upf_printk("upf: ifindex mismatch: fib=%d expected=%d",
			   fib_params->ifindex, expected_ifindex);
		statistic->ip6_ifindex_mismatch += 1;
		return CTX_ACT_DROP;
	}

	__builtin_memcpy(ctx->eth->h_dest, fib_params->dmac, ETH_ALEN);
	__builtin_memcpy(ctx->eth->h_source, fib_params->smac, ETH_ALEN);

	upf_printk("upf: bpf_redirect: if=%d %lu -> %lu", fib_params->ifindex,
		   fib_params->smac, fib_params->dmac);

	if (expected_ifindex == ctx_ingress_ifindex(ctx->ctx_buff) &&
	    expected_ifindex != 0)
		return ctx_tx_back(ctx->ctx_buff, egress_vlan_forwarded(ctx));
	upf_printk("upf: bpf_redirect: if=%d %lu -> %lu", fib_params->ifindex,
		   fib_params->smac, fib_params->dmac);
	return ctx_redirect_out(ctx->ctx_buff, fib_params->ifindex,
				egress_vlan_forwarded(ctx));
}

static __always_inline enum ctx_action route_ipv4(struct packet_context *ctx,
						  struct route_stat *statistic,
						  bool trust_fib)
{
	struct bpf_fib_lookup fib_params = {};
	fib_params.family = AF_INET;
	fib_params.tos = ctx->ip4->tos;
	fib_params.l4_protocol = ctx->ip4->protocol;
	fib_params.sport = 0;
	fib_params.dport = 0;
	fib_params.tot_len = bpf_ntohs(ctx->ip4->tot_len);
	fib_params.ipv4_src = ctx->ip4->saddr;
	fib_params.ipv4_dst = ctx->ip4->daddr;
	fib_params.ifindex = ctx_ingress_ifindex(ctx->ctx_buff);

	/* Only source_nat reads the derived address, and trust_fib skips it.
	 * Asking anyway adds BPF_FIB_LKUP_RET_NO_SRC_ADDR as a drop reason. */
	__u64 flags = BPF_FIB_LOOKUP_DIRECT;
	if (!trust_fib && masquerade) {
		flags |= BPF_FIB_LOOKUP_SRC;
	}
	int rc = bpf_fib_lookup(ctx->ctx_buff, &fib_params, sizeof(fib_params),
				flags);
	switch (rc) {
	case BPF_FIB_LKUP_RET_NO_NEIGH: {
		// smac is unset on this branch, so the frame cannot be completed
		// here. The lookup rewrites family and ipv6_dst for an IPv4
		// route with an IPv6 nexthop (RFC 5549).
		struct no_neigh_event ev = {
			.ifindex = fib_params.ifindex,
			.family = fib_params.family,
		};

		__builtin_memcpy(ev.addr, fib_params.ipv6_dst,
				 sizeof(fib_params.ipv6_dst));
		bpf_ringbuf_output(&no_neigh_map, &ev, sizeof(ev), 0);
		statistic->fib_lookup_ip4_no_neigh += 1;

		return CTX_ACT_DROP;
	}
	case BPF_FIB_LKUP_RET_SUCCESS:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: nexthop: %pI4",
			   &ctx->ip4->saddr, &ctx->ip4->daddr,
			   &fib_params.ipv4_dst);
		statistic->fib_lookup_ip4_success += 1;

		return do_route_ipv4(ctx, &fib_params, statistic, trust_fib);

	case BPF_FIB_LKUP_RET_BLACKHOLE:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_blackhole += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_UNREACHABLE:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_unreachable += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_PROHIBIT:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_prohibit += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_NO_SRC_ADDR:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_no_src_addr += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_FRAG_NEEDED:
		upf_printk("upf: fragmentation needed for %pI4 -> %pI4",
			   &ctx->ip4->saddr, &ctx->ip4->daddr);
		statistic->fib_lookup_ip4_frag_needed += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_NOT_FWDED:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_not_fwded += 1;
		return CTX_ACT_OK;
	case BPF_FIB_LKUP_RET_FWD_DISABLED:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_fwd_disabled += 1;
		return CTX_ACT_OK;
	case BPF_FIB_LKUP_RET_UNSUPP_LWT:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_unsupp_lwt += 1;
		return CTX_ACT_OK;
	default:
		/* A negative return is helper misuse, not a routing verdict;
		 * passing hands the stack an untranslated UE address. */
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		if (rc < 0) {
			statistic->fib_lookup_ip4_error += 1;
			return CTX_ACT_DROP;
		}
		return CTX_ACT_OK;
	}
}

static __always_inline enum ctx_action route_ipv6(struct packet_context *ctx,
						  struct route_stat *statistic,
						  bool trust_fib)
{
	struct bpf_fib_lookup fib_params = {};
	fib_params.family = AF_INET6;
	// fib_params.tos = ip6->flow_lbl;
	fib_params.l4_protocol = ctx->ip6->nexthdr;
	fib_params.sport = 0;
	fib_params.dport = 0;
	fib_params.tot_len = bpf_ntohs(ctx->ip6->payload_len);
	__builtin_memcpy(fib_params.ipv6_src, &ctx->ip6->saddr,
			 sizeof(ctx->ip6->saddr));
	__builtin_memcpy(fib_params.ipv6_dst, &ctx->ip6->daddr,
			 sizeof(ctx->ip6->daddr));
	fib_params.ifindex = ctx_ingress_ifindex(ctx->ctx_buff);

	int rc = bpf_fib_lookup(ctx->ctx_buff, &fib_params, sizeof(fib_params),
				0 /*BPF_FIB_LOOKUP_OUTPUT*/);
	switch (rc) {
	case BPF_FIB_LKUP_RET_NO_NEIGH: {
		// smac is unset on this branch, so the frame cannot be completed
		// here. The lookup rewrites family and ipv6_dst for an IPv4
		// route with an IPv6 nexthop (RFC 5549).
		struct no_neigh_event ev = {
			.ifindex = fib_params.ifindex,
			.family = fib_params.family,
		};

		__builtin_memcpy(ev.addr, fib_params.ipv6_dst,
				 sizeof(fib_params.ipv6_dst));
		bpf_ringbuf_output(&no_neigh_map, &ev, sizeof(ev), 0);
		statistic->fib_lookup_ip6_no_neigh += 1;

		return CTX_ACT_DROP;
	}
	case BPF_FIB_LKUP_RET_SUCCESS:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: nexthop: %pI6c",
			   &ctx->ip6->saddr, &ctx->ip6->daddr,
			   fib_params.ipv6_dst);
		statistic->fib_lookup_ip6_success += 1;
		//_decr_ttl(ether_proto, l3hdr);

		return do_route_ipv6(ctx, &fib_params, statistic, trust_fib);
	case BPF_FIB_LKUP_RET_BLACKHOLE:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_blackhole += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_UNREACHABLE:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_unreachable += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_PROHIBIT:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_prohibit += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_NO_SRC_ADDR:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_no_src_addr += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_FRAG_NEEDED:
		upf_printk("upf: fragmentation needed %pI6c -> %pI6c",
			   &ctx->ip6->saddr, &ctx->ip6->daddr);
		statistic->fib_lookup_ip6_frag_needed += 1;
		return CTX_ACT_DROP;
	case BPF_FIB_LKUP_RET_NOT_FWDED:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_not_fwded += 1;
		return CTX_ACT_OK;
	case BPF_FIB_LKUP_RET_FWD_DISABLED:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_fwd_disabled += 1;
		return CTX_ACT_OK;
	case BPF_FIB_LKUP_RET_UNSUPP_LWT:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_unsupp_lwt += 1;
		return CTX_ACT_OK;
	default:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		if (rc < 0) {
			statistic->fib_lookup_ip6_error += 1;
		}
		return CTX_ACT_OK;
	}
}
