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

#pragma once

#include "xdp/utils/flow.h"
#include "xdp/utils/packet_context.h"
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

#include "xdp/utils/nat.h"
#include "xdp/utils/trace.h"

volatile const int n3_ifindex;
volatile const int n3_ifindex = 0;
volatile const int n6_ifindex;
volatile const int n6_ifindex = 0;

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 16384);
} no_neigh_map SEC(".maps");

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
};

static __always_inline enum xdp_action
do_route_ipv4(struct packet_context *ctx, struct bpf_fib_lookup *fib_params,
	      struct route_stat *statistic)
{
	__u32 expected_ifindex;

	if (ctx->interface == INTERFACE_N3) {
		expected_ifindex = n6_ifindex;
	} else {
		expected_ifindex = n3_ifindex;
	}

	if (fib_params->ifindex != expected_ifindex) {
		upf_printk("upf: ifindex mismatch: fib=%d expected=%d",
			   fib_params->ifindex, expected_ifindex);
		statistic->ip4_ifindex_mismatch += 1;
		return XDP_DROP;
	}

	__builtin_memcpy(ctx->eth->h_source, fib_params->smac, ETH_ALEN);
	__builtin_memcpy(ctx->eth->h_dest, fib_params->dmac, ETH_ALEN);

	if (ctx->interface == INTERFACE_N3) {
		if (masquerade) {
			if (!source_nat(ctx, fib_params)) {
				return XDP_DROP;
			}
		}
	}

	if (expected_ifindex == ctx->xdp_ctx->ingress_ifindex)
		return XDP_TX;
	return bpf_redirect(expected_ifindex, 0);
}

static __always_inline enum xdp_action
do_route_ipv6(struct packet_context *ctx, struct bpf_fib_lookup *fib_params,
	      struct route_stat *statistic)
{
	__u32 expected_ifindex;

	if (ctx->interface == INTERFACE_N3) {
		expected_ifindex = n6_ifindex;
	} else {
		expected_ifindex = n3_ifindex;
	}

	if (fib_params->ifindex != expected_ifindex) {
		upf_printk("upf: ifindex mismatch: fib=%d expected=%d",
			   fib_params->ifindex, expected_ifindex);
		statistic->ip6_ifindex_mismatch += 1;
		return XDP_DROP;
	}

	__builtin_memcpy(ctx->eth->h_dest, fib_params->dmac, ETH_ALEN);
	__builtin_memcpy(ctx->eth->h_source, fib_params->smac, ETH_ALEN);

	upf_printk("upf: bpf_redirect: if=%d %lu -> %lu",
		   fib_params->ifindex, fib_params->smac,
		   fib_params->dmac);

	if (expected_ifindex == ctx->xdp_ctx->ingress_ifindex)
		return XDP_TX;
	return bpf_redirect(expected_ifindex, 0);
}

static __always_inline enum xdp_action route_ipv4(struct packet_context *ctx,
						  struct route_stat *statistic)
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
	fib_params.ifindex = ctx->xdp_ctx->ingress_ifindex;

	__u64 flags = BPF_FIB_LOOKUP_DIRECT;
	if (masquerade) {
		flags |= BPF_FIB_LOOKUP_SRC;
	}
	int rc = bpf_fib_lookup(ctx->xdp_ctx, &fib_params, sizeof(fib_params),
				flags);
	switch (rc) {
	case BPF_FIB_LKUP_RET_NO_NEIGH:
		__builtin_memset(fib_params.dmac, 0xFF, 6);
		__be32 daddr = fib_params.ipv4_dst;
		bpf_ringbuf_output(&no_neigh_map, &daddr, sizeof(daddr), 0);
		statistic->fib_lookup_ip4_no_neigh += 1;
		// The fall-through is voluntary here
	case BPF_FIB_LKUP_RET_SUCCESS:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: nexthop: %pI4",
			   &ctx->ip4->saddr, &ctx->ip4->daddr,
			   &fib_params.ipv4_dst);
		if (rc == BPF_FIB_LKUP_RET_SUCCESS)
			statistic->fib_lookup_ip4_success += 1;

		return do_route_ipv4(ctx, &fib_params, statistic);

	case BPF_FIB_LKUP_RET_BLACKHOLE:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_blackhole += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_UNREACHABLE:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_unreachable += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_PROHIBIT:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_prohibit += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_NO_SRC_ADDR:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_no_src_addr += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_FRAG_NEEDED:
		upf_printk("upf: fragmentation needed for %pI4 -> %pI4",
			   &ctx->ip4->saddr, &ctx->ip4->daddr);
		statistic->fib_lookup_ip4_frag_needed += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_NOT_FWDED:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_not_fwded += 1;
		return XDP_PASS;
	case BPF_FIB_LKUP_RET_FWD_DISABLED:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_fwd_disabled += 1;
		return XDP_PASS;
	case BPF_FIB_LKUP_RET_UNSUPP_LWT:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		statistic->fib_lookup_ip4_unsupp_lwt += 1;
		return XDP_PASS;
	default:
		upf_printk("upf: bpf_fib_lookup %pI4 -> %pI4: %d",
			   &ctx->ip4->saddr, &ctx->ip4->daddr, rc);
		return XDP_PASS;
	}
}

static __always_inline enum xdp_action route_ipv6(struct packet_context *ctx,
						  struct route_stat *statistic)
{
	struct bpf_fib_lookup fib_params = {};
	fib_params.family = AF_INET;
	// fib_params.tos = ip6->flow_lbl;
	fib_params.l4_protocol = ctx->ip6->nexthdr;
	fib_params.sport = 0;
	fib_params.dport = 0;
	fib_params.tot_len = bpf_ntohs(ctx->ip6->payload_len);
	__builtin_memcpy(fib_params.ipv6_src, &ctx->ip6->saddr,
			 sizeof(ctx->ip6->saddr));
	__builtin_memcpy(fib_params.ipv6_dst, &ctx->ip6->daddr,
			 sizeof(ctx->ip6->daddr));
	fib_params.ifindex = ctx->xdp_ctx->ingress_ifindex;

	int rc = bpf_fib_lookup(ctx->xdp_ctx, &fib_params, sizeof(fib_params),
				0 /*BPF_FIB_LOOKUP_OUTPUT*/);
	switch (rc) {
	case BPF_FIB_LKUP_RET_NO_NEIGH:
		__builtin_memset(fib_params.dmac, 0xFF, 6);
		struct in6_addr daddr = {};
		__builtin_memcpy(&daddr, &fib_params.ipv6_dst, sizeof(daddr));
		bpf_ringbuf_output(&no_neigh_map, &daddr, sizeof(daddr), 0);
		statistic->fib_lookup_ip6_no_neigh += 1;
		// The fall-through is voluntary here
	case BPF_FIB_LKUP_RET_SUCCESS:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: nexthop: %pI4",
			   &ctx->ip6->saddr, &ctx->ip6->daddr,
			   fib_params.ipv4_dst);
		if (rc == BPF_FIB_LKUP_RET_SUCCESS)
			statistic->fib_lookup_ip6_success += 1;
		//_decr_ttl(ether_proto, l3hdr);

		return do_route_ipv6(ctx, &fib_params, statistic);
	case BPF_FIB_LKUP_RET_BLACKHOLE:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_blackhole += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_UNREACHABLE:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_unreachable += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_PROHIBIT:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_prohibit += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_NO_SRC_ADDR:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_no_src_addr += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_FRAG_NEEDED:
		upf_printk("upf: fragmentation needed %pI6c -> %pI6c",
			   &ctx->ip6->saddr, &ctx->ip6->daddr);
		statistic->fib_lookup_ip6_frag_needed += 1;
		return XDP_DROP;
	case BPF_FIB_LKUP_RET_NOT_FWDED:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_not_fwded += 1;
		return XDP_PASS;
	case BPF_FIB_LKUP_RET_FWD_DISABLED:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_fwd_disabled += 1;
		return XDP_PASS;
	case BPF_FIB_LKUP_RET_UNSUPP_LWT:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		statistic->fib_lookup_ip6_unsupp_lwt += 1;
		return XDP_PASS;
	default:
		upf_printk("upf: bpf_fib_lookup %pI6c -> %pI6c: %d",
			   &ctx->ip6->saddr, &ctx->ip6->daddr, rc);
		return XDP_PASS;
	}
}
