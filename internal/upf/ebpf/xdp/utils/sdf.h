#pragma once

#include "xdp/utils/pdr.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/trace.h"
#include "xdp/utils/ip_addr.h"
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
 *   XDP_PASS  – packet is allowed (default-allow or explicit allow match)
 *   XDP_DROP  – packet is denied by an explicit deny rule
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
static __always_inline enum xdp_action
match_sdf_rules(struct sdf_filter_list *flist, __u8 num, __u8 pkt_proto,
		__u16 pkt_dport, __u8 pkt_is_ipv4,
		const struct in6_addr pkt_remote);

static __always_inline int match_ipv6_prefix(const struct in6_addr *rule_ip,
					     __u8 prefix_len,
					     const struct in6_addr *pkt_ip)
{
	const __u32 *rule = rule_ip->in6_u.u6_addr32;
	const __u32 *pkt = pkt_ip->in6_u.u6_addr32;
	__u8 words = prefix_len >> 5;
	__u8 bits = prefix_len & 31;

	if (words > 0) {
		if (rule[0] != pkt[0])
			return -1;
	}
	if (words > 1) {
		if (rule[1] != pkt[1])
			return -1;
	}
	if (words > 2) {
		if (rule[2] != pkt[2])
			return -1;
	}
	if (words > 3) {
		if (rule[3] != pkt[3])
			return -1;
	}

	if (words < 4 && bits > 0) {
		__u32 mask = bpf_htonl(~((__u32)0) << (32 - bits));
		if ((rule[words] & mask) != (pkt[words] & mask))
			return -1;
	}

	return 1;
}

static __always_inline enum xdp_action
match_sdf_filters(struct packet_context *ctx, __u32 filter_map_index)
{
	__u8 pkt_proto;
	__u16 pkt_dport = 0;
	__u8 pkt_is_ipv4;
	struct in6_addr pkt_remote = {};

	if (filter_map_index == 0)
		return XDP_PASS;

	struct sdf_filter_list *flist =
		bpf_map_lookup_elem(&sdf_filters, &filter_map_index);
	if (!flist)
		return XDP_PASS; /* fail-open */

	__u8 num = flist->num_rules;
	if (num > MAX_RULES_PER_FILTER)
		num = MAX_RULES_PER_FILTER; /* bound for the verifier */

	if (ctx->ip4) {
		pkt_is_ipv4 = 1;
		pkt_proto = ctx->ip4->protocol;
		ipv4_to_mapped(&pkt_remote, (ctx->interface == INTERFACE_N3) ?
						    ctx->ip4->daddr :
						    ctx->ip4->saddr);
	} else if (ctx->ip6) {
		pkt_is_ipv4 = 0;
		pkt_proto = ctx->ip6->nexthdr;
		pkt_remote = (ctx->interface == INTERFACE_N3) ?
				     ctx->ip6->daddr :
				     ctx->ip6->saddr;
	} else {
		return XDP_PASS;
	}

	if (ctx->tcp)
		pkt_dport = (ctx->interface == INTERFACE_N3) ?
				    bpf_ntohs(ctx->tcp->dest) :
				    bpf_ntohs(ctx->tcp->source);
	else if (ctx->udp)
		pkt_dport = (ctx->interface == INTERFACE_N3) ?
				    bpf_ntohs(ctx->udp->dest) :
				    bpf_ntohs(ctx->udp->source);

	upf_printk("upf: filter packet for %08X:%d, proto %d",
		   bpf_ntohl(ipv4_from_mapped(&pkt_remote)), pkt_dport,
		   pkt_proto);

	return match_sdf_rules(flist, num, pkt_proto, pkt_dport, pkt_is_ipv4,
			       pkt_remote);
}

static __always_inline enum xdp_action
match_sdf_rules(struct sdf_filter_list *flist, __u8 num, __u8 pkt_proto,
		__u16 pkt_dport, __u8 pkt_is_ipv4,
		const struct in6_addr pkt_remote)
{
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

				if (!match_ipv6_prefix(&r->remote_ip,
						       r->prefix_len,
						       &pkt_remote))
					continue;
			}
		}

		if (r->port_low != 0) {
			if (pkt_dport < r->port_low || pkt_dport > r->port_high)
				continue;
		}

		if (r->action == 1)
			return XDP_DROP;

		return XDP_PASS;
	}

	return XDP_PASS;
}
