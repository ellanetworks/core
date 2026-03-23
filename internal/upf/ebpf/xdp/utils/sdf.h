#pragma once

#include "xdp/utils/pdr.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/trace.h"
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
 *   XDP_PASS  – packet is permitted (default-permit or explicit permit match)
 *   XDP_DROP  – packet is denied by an explicit deny rule
 *
 * Matching semantics (first-match wins):
 *   1. If filter_map_index == 0, no filtering → permit.
 *   2. Look up the filter list; if not found → permit (fail-open).
 *   3. Iterate rules in order; first match wins.
 *   4. No rule matched → default permit.
 *
 * Direction is implicit: uplink PDRs carry the uplink filter index, downlink
 * PDRs carry the downlink filter index.
 * N3 interface: daddr is the remote; N6 interface: saddr is the remote.
 */
static __always_inline enum xdp_action
match_sdf_filters(struct packet_context *ctx, __u32 filter_map_index)
{
	if (filter_map_index == 0)
		return XDP_PASS;

	struct sdf_filter_list *flist =
		bpf_map_lookup_elem(&sdf_filters, &filter_map_index);
	if (!flist)
		return XDP_PASS; /* fail-open */

	__u8 num = flist->num_rules;
	if (num > MAX_RULES_PER_FILTER)
		num = MAX_RULES_PER_FILTER; /* bound for the verifier */

	if (!ctx->ip4)
		return XDP_PASS; /* IPv6 not filtered yet */

	__u8  pkt_proto  = ctx->ip4->protocol;
	__u32 pkt_remote = (ctx->interface == INTERFACE_N3)
	                       ? bpf_ntohl(ctx->ip4->daddr)
	                       : bpf_ntohl(ctx->ip4->saddr);
	__u16 pkt_dport  = 0;

	if (ctx->tcp)
		pkt_dport = (ctx->interface == INTERFACE_N3)
				? bpf_ntohs(ctx->tcp->dest)
				: bpf_ntohs(ctx->tcp->source);
	else if (ctx->udp)
		pkt_dport = (ctx->interface == INTERFACE_N3)
				? bpf_ntohs(ctx->udp->dest)
				: bpf_ntohs(ctx->udp->source);

	upf_printk("upf: filter packet for %08X:%d, proto %d", pkt_remote, pkt_dport, pkt_proto);
	for (__u8 i = 0; i < MAX_RULES_PER_FILTER; i++) {
		if (i >= num)
			break;

		const struct sdf_rule *r = &flist->rules[i];

		upf_printk("upf: checking protocol: %d", r->protocol);
		/* Protocol check */
		if (r->protocol != SDF_PROTO_ANY && r->protocol != pkt_proto)
			continue;

		upf_printk("upf: checking prefix: %08X/%08X", r->remote_ip, r->remote_mask);
		/* Remote IP/prefix check */
		if (r->remote_mask != 0) {
			__u32 rule_net   = r->remote_ip & r->remote_mask;
			__u32 pkt_masked = pkt_remote & r->remote_mask;
			if (pkt_masked != rule_net)
				continue;
		}

		upf_printk("upf: checking ports: %d-%d", r->port_low, r->port_high);
		/* Port range check */
		if (r->port_low != 0) {
			if (pkt_dport < r->port_low || pkt_dport > r->port_high)
				continue;
		}

		upf_printk("upf: rule matched: action: %d", r->action);

		/* First match wins */
		if (r->action == 1)
			return XDP_DROP;

		return XDP_PASS;
	}

	upf_printk("upf: default permit");

	/* default-permit */
	return XDP_PASS;
}
