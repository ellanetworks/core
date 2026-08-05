// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/utils/pdr.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/trace.h"
#include "bpf/utils/ip_addr.h"
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct sdf_filter_list);
	__uint(max_entries, MAX_SDF_FILTERS);
} sdf_filters SEC(".maps");

/*
 * match_sdf_filters – evaluate a packet against the filter list for the PDR.
 *
 * Returns:
 *   CTX_ACT_OK  – packet is allowed (default-allow or explicit allow match)
 *   CTX_ACT_DROP  – packet is denied by an explicit deny rule
 *
 * Matching semantics (first-match wins):
 *   1. If filter_map_index == 0, no filtering → allow.
 *   2. Look up the filter list; if not found → allow (fail-open).
 *   3. Iterate rules in order; first match wins.
 *   4. No rule matched → default allow.
 *
 * Direction is implicit: uplink PDRs carry the uplink filter index, downlink
 * PDRs carry the downlink filter index.
 * N3 interface: daddr is the remote; N6 interface: saddr is the remote.
 */
/* Whether the rule would reject any port. Both the SDF_PORT_ANY wildcard and a
 * range spanning every port match regardless of what the ports are. */
static __always_inline bool rule_constrains_ports(const struct sdf_rule *r)
{
	if (r->port_low == SDF_PORT_ANY && r->port_high == SDF_PORT_ANY)
		return false;

	return !(r->port_low == 0 && r->port_high == 65535);
}

#define SDF_VERDICT_PASS 0
#define SDF_VERDICT_DENY 1
#define SDF_VERDICT_UNFILTERABLE 2

struct sdf_query {
	struct in6_addr remote;
	__u32 filter_index;
	__u16 dport;
	__u8 proto;
	__u8 is_ipv4;
	__u8 ports_unreadable;
	__u8 pad[3];
};

__noinline __weak int sdf_match(struct sdf_query *q);

/* Compares one 32-bit word under a prefix of `bits`. */
static __always_inline bool sdf_word_eq(__u32 rule, __u32 pkt, __u32 bits)
{
	if (bits == 0)
		return true;

	if (bits >= 32)
		return rule == pkt;

	__u32 mask = bpf_htonl(~((__u32)0) << (32 - bits));

	return (rule & mask) == (pkt & mask);
}

/* Every word index is a literal: a computed one is a variable-offset read of
 * the stack copy, which the verifier refuses inside a global function. */
static __always_inline int match_ipv6_prefix(const struct in6_addr *rule_ip,
					     __u8 prefix_len,
					     const struct in6_addr *pkt_ip)
{
	const __u32 *rule = rule_ip->in6_u.u6_addr32;
	const __u32 *pkt = pkt_ip->in6_u.u6_addr32;
	__u32 left = prefix_len > 128 ? 128 : prefix_len;

	if (!sdf_word_eq(rule[0], pkt[0], left))
		return -1;

	left = left > 32 ? left - 32 : 0;

	if (!sdf_word_eq(rule[1], pkt[1], left))
		return -1;

	left = left > 32 ? left - 32 : 0;

	if (!sdf_word_eq(rule[2], pkt[2], left))
		return -1;

	left = left > 32 ? left - 32 : 0;

	if (!sdf_word_eq(rule[3], pkt[3], left))
		return -1;

	return 1;
}

static __always_inline enum ctx_action
match_sdf_filters(struct packet_context *ctx, __u32 filter_map_index)
{
	__u8 pkt_proto;
	__u16 pkt_dport = 0;
	__u8 pkt_is_ipv4;
	struct in6_addr pkt_remote = {};

	if (filter_map_index == 0)
		return CTX_ACT_OK;

	if (ctx->ip4) {
		pkt_is_ipv4 = 1;
		ipv4_to_mapped(&pkt_remote, (ctx->interface == INTERFACE_N3) ?
						    ctx->ip4->daddr :
						    ctx->ip4->saddr);
	} else if (ctx->ip6) {
		pkt_is_ipv4 = 0;
		pkt_remote = (ctx->interface == INTERFACE_N3) ?
				     ctx->ip6->daddr :
				     ctx->ip6->saddr;
	} else {
		return CTX_ACT_OK;
	}

	/* The upper-layer protocol: matching ip6->nexthdr would match a rule
	 * against the extension-header type. */
	pkt_proto = ctx->l4_proto;
	pkt_dport = (ctx->interface == INTERFACE_N3) ? ctx->l4_dport :
						       ctx->l4_sport;

	upf_printk("upf: filter packet for %08X:%d, proto %d",
		   bpf_ntohl(ipv4_from_mapped(&pkt_remote)), pkt_dport,
		   pkt_proto);

	struct sdf_query q = {
		.remote = pkt_remote,
		.filter_index = filter_map_index,
		.dport = pkt_dport,
		.proto = pkt_proto,
		.is_ipv4 = pkt_is_ipv4,
		.ports_unreadable = ctx->l4_unavailable,
	};

	int verdict = sdf_match(&q);

	if (verdict == SDF_VERDICT_PASS)
		return CTX_ACT_OK;

	set_drop_reason(ctx, verdict == SDF_VERDICT_UNFILTERABLE ?
				     UPF_DROP_FRAGMENT_UNFILTERABLE :
				     UPF_DROP_SDF_FILTER);

	return CTX_ACT_DROP;
}

__noinline __weak int sdf_match(struct sdf_query *q)
{
	if (!q)
		return SDF_VERDICT_PASS;

	struct sdf_filter_list *flist =
		bpf_map_lookup_elem(&sdf_filters, &q->filter_index);
	if (!flist)
		return SDF_VERDICT_PASS;

	__u8 num = flist->num_rules;

	if (num > MAX_RULES_PER_FILTER)
		num = MAX_RULES_PER_FILTER;

	const __u8 pkt_proto = q->proto;
	const __u16 pkt_dport = q->dport;
	const __u8 pkt_is_ipv4 = q->is_ipv4;
	const __u8 ports_unreadable = q->ports_unreadable;
	const struct in6_addr pkt_remote = q->remote;

#pragma clang loop unroll(disable)
	for (__u8 i = 0; i < num; i++) {
		const struct sdf_rule *r = &flist->rules[i];

		if (r->protocol != SDF_PROTO_ANY && r->protocol != pkt_proto)
			continue;

		if (r->prefix_len != 0) {
			__u8 rule_is_ipv4 = is_ipv4_mapped_ipv6(&r->remote_ip);

			if (pkt_is_ipv4 != rule_is_ipv4)
				continue;

			if (rule_is_ipv4) {
				__u32 rule_ip = bpf_ntohl(
					ipv4_from_mapped(&r->remote_ip));
				__u32 pkt_ip = bpf_ntohl(
					ipv4_from_mapped(&pkt_remote));
				__u8 prefix = r->prefix_len;

				if (prefix > 32)
					prefix = 32;

				if (prefix > 0) {
					__u32 mask = ~((__u32)0)
						     << (32 - prefix);
					if ((rule_ip & mask) != (pkt_ip & mask))
						continue;
				}
			} else {
				if (r->prefix_len > 128)
					continue;

				if (match_ipv6_prefix(&r->remote_ip,
						      r->prefix_len,
						      &pkt_remote) < 0)
					continue;
			}
		}

		/* Reaching here means protocol and address already matched, so
		 * this rule is one that could apply to the packet. */
		if (rule_constrains_ports(r)) {
			/* Skipping it is how a deny is evaded (RFC 1858). */
			if (ports_unreadable)
				return SDF_VERDICT_UNFILTERABLE;

			if (pkt_dport < r->port_low || pkt_dport > r->port_high)
				continue;
		}

		if (r->action == 1)
			return SDF_VERDICT_DENY;

		return SDF_VERDICT_PASS;
	}

	return SDF_VERDICT_PASS;
}
