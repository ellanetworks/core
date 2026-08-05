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
#include "bpf/utils/routing.h"
#include "bpf/utils/trace.h"
#include "bpf/utils/profiling.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "bpf/utils/common.h"
#include "bpf/utils/frag_needed.h"
#include "bpf/utils/gtp.h"
#include "bpf/utils/tailcall.h"
#include "bpf/utils/pdr.h"
#include "bpf/utils/pdr_maps.h"
#include "bpf/utils/qer.h"
#include "bpf/utils/sdf.h"
#include "bpf/utils/urr.h"
#include "bpf/utils/statistics.h"
#include "bpf/utils/rs_event.h"

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u32);
	__type(value, struct pdr_info);
	__uint(max_entries, PDR_MAP_UPLINK_SIZE);
} pdrs_uplink SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
} uplink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
} uplink_statistics SEC(".maps");

/*
 * fe80::/10 is rejected, so a future link-local-sourced feature (NS/NA proxy,
 * stateless DHCPv6) must be intercepted before this, as RS already is. IPv4 and
 * IPv6 use separate return paths to avoid a verifier ip4/ip6 state merge.
 */
static __always_inline bool source_allowed(struct packet_context *ctx,
					   const struct pdr_info *pdr)
{
	if (ctx->ip4) {
		__u32 ue4 = ipv4_from_mapped(&pdr->ue_ipv4);
		if (ue4 == 0)
			return false;

		__u32 src = ctx->ip4->saddr;
		if (src == ue4)
			return true;

		struct framed_ip4_key fk = { .prefixlen = 32, .addr = src };
		__u32 *owner = bpf_map_lookup_elem(&framed_downlink_ip4, &fk);
		return owner && *owner == ue4;
	}

	if (ctx->ip6) {
		const struct in6_addr *src = &ctx->ip6->saddr;
		if (src->in6_u.u6_addr8[0] == 0xfe &&
		    (src->in6_u.u6_addr8[1] & 0xc0) == 0x80)
			return false;

		if ((pdr->ue_ipv6.in6_u.u6_addr32[0] |
		     pdr->ue_ipv6.in6_u.u6_addr32[1]) == 0)
			return false;

		if (match_ipv6_prefix(&pdr->ue_ipv6, 64, src) == 1)
			return true;

		struct framed_ip6_key fk = { .prefixlen = 128 };
		fk.addr = *src;
		struct in6_addr *owner =
			bpf_map_lookup_elem(&framed_downlink_ip6, &fk);
		/* /64: owner is a /64 downlink key, robust even if ue_ipv6 is a full address */
		return owner &&
		       match_ipv6_prefix(owner, 64, &pdr->ue_ipv6) == 1;
	}

	return false;
}

static __always_inline enum ctx_action
handle_gtp_packet(struct packet_context *ctx)
{
	if (!ctx->gtp) {
		upf_printk("upf: unexpected packet context. no gtp header");
		return DEFAULT_CTX_ACTION;
	}

	__u32 teid = bpf_htonl(ctx->gtp->teid);

	/* Lookup uplink session using the TEID */
	PROFILE_START(PROF_N3_PDR_LOOKUP);
	struct pdr_info *pdr = bpf_map_lookup_elem(&pdrs_uplink, &teid);
	PROFILE_END(PROF_N3_PDR_LOOKUP);
	if (!pdr) {
		/* No PDU session for this TEID: discard the G-PDU and, for a non-zero
		 * TEID, return a GTP-U Error Indication to the sender over the same
		 * IP transport (TS 29.281 §7.3.1). */
		if (ctx->gtp->teid != 0) {
			return gtpu_control_tail_call(ctx);
		}

		upf_printk("upf: no uplink PDR for teid:%d, discarding", teid);
		return drop_with(ctx, UPF_DROP_NO_UPLINK_SESSION);
	}

	__u32 urr_id = pdr->urr_id;
	__u8 outer_header_removal = pdr->outer_header_removal;

	struct far_info *far = &pdr->far;
	struct qer_info *qer = &pdr->qer;

	PROFILE_START(PROF_N3_MTU_CHECK);
	__u32 mtu_len = 0;
	__u32 decap_no_vlan = gtp_decap_size_no_vlan(ctx);
	if (decap_no_vlan == 0) {
		PROFILE_END(PROF_N3_MTU_CHECK);
		return abort_with(ctx, UPF_DROP_MALFORMED_GTP);
	}
	/* The same span remove_gtp_header strips: the input tag goes with the
	 * outer headers, and an output tag is added back. Both terms are 0 on
	 * TCX, where tags are out of band. */
	int decap_size = (int)decap_no_vlan +
			 (CTX_INBAND_VLAN && ctx->vlan ? (int)sizeof(struct vlan_hdr) : 0) -
			 (CTX_INBAND_VLAN && n6_vlan ? (int)sizeof(struct vlan_hdr) : 0);
	long ret = bpf_check_mtu(ctx->ctx_buff, n6_ifindex, &mtu_len,
				 -decap_size, 0);
	PROFILE_END(PROF_N3_MTU_CHECK);
	if (ret < 0) {
		return abort_with(ctx, UPF_DROP_INTERNAL_MTU_CHECK_FAILED);
	}
	/* Acted on after decapsulation. Here ctx still describes the transport
	 * header, so DF and the address family are the gNB's, not the
	 * subscriber's, and neither can be read yet. */
	const bool inner_exceeds_n6_mtu = ret > 0;

	ctx->interface = INTERFACE_N3;

	upf_printk("upf: far action:%d outer_header_creation:%d", far->action,
		   far->outer_header_creation);
	if (!(far->action & FAR_FORW)) {
		return drop_with(ctx, UPF_DROP_FAR_NO_FORWARD);
	}

	/* Decap would strip the first datagram's outer headers and leave the
	 * rest in the payload as intact GTP-U. */
	if (frame_is_merged(ctx)) {
		upf_printk("upf: merged frame on the decap path, dropping");
		return drop_with(ctx, UPF_DROP_DECAP_GSO);
	}

	PROFILE_START(PROF_N3_QER_RATELIMIT);
	upf_printk("upf: qer gate_status:%d mbr:%d", qer->ul_gate_status,
		   qer->ul_maximum_bitrate);
	if (qer->ul_gate_status != GATE_STATUS_OPEN) {
		PROFILE_END(PROF_N3_QER_RATELIMIT);
		return drop_with(ctx, UPF_DROP_QER_GATE_CLOSED);
	}

	/* An unlimited QER costs no lookup. */
	if (qer->ul_maximum_bitrate != 0) {
		const __u64 packet_size =
			ctx_len_from(ctx->ctx_buff, ctx->data_end, ctx->data);
		struct qer_window *window =
			qer_window_for(pdr->local_seid, pdr->qer_id);

		if (window &&
		    CTX_ACT_DROP == limit_rate_sliding_window(
					    packet_size, &window->ul_start,
					    qer->ul_maximum_bitrate)) {
			PROFILE_END(PROF_N3_QER_RATELIMIT);
			return drop_with(ctx, UPF_DROP_QER_RATE_LIMIT);
		}
	}
	PROFILE_END(PROF_N3_QER_RATELIMIT);

	upf_printk("upf: session for teid:%d outer_header_removal:%d", teid,
		   outer_header_removal);
	PROFILE_START(PROF_N3_GTP_MANIP);
	/* GTP-to-GTP forwarding (N9, S5/S8) is not supported: the frame would
	 * leave with a stale outer UDP checksum over rewritten addresses, and
	 * would bypass the SDF and anti-spoof checks the decap path runs. */
	if (far->outer_header_creation &
	    (OHC_GTP_U_UDP_IPv4 | OHC_GTP_U_UDP_IPv6)) {
		PROFILE_END(PROF_N3_GTP_MANIP);
		return drop_with(ctx, UPF_DROP_FAR_UNSUPPORTED);
	}

	if (outer_header_removal == OHR_GTP_U_UDP_IPv4 ||
	    outer_header_removal == OHR_GTP_U_UDP_IPv6) {
		long result = remove_gtp_header(ctx, outer_header_removal);
		if (result) {
			PROFILE_END(PROF_N3_GTP_MANIP);
			upf_printk(
				"upf: handle_gtp_packet: can't remove gtp header: %d",
				result);
			return abort_reported(ctx, UPF_DROP_INTERNAL_DECAP_FAILED);
		}

		/* Before anything reads ports. Not in parse_ip4, which the
		 * tunnel transport also uses. */
		frag_resolve4(ctx);

		/* For match_sdf_filters. */
		if (ctx->ip4 || ctx->ip6)
			parse_l4(ctx->l4_proto, ctx);

		/* The RA is built in Go and injected through the veth path.
		 *
		 * At l3_hdr_len from ctx->ip6: an egress VLAN tag and a chain
		 * both move the ICMPv6 header. */
		if (ctx->ip6 && ctx->l4_proto == IPPROTO_ICMPV6 &&
		    !ctx->l4_unavailable) {
			const __u32 icmp6_off =
				ctx_frame_offset(ctx->ctx_buff, ctx->ip6) +
				ctx->l3_hdr_len;
			__u8 icmp6_type;

			if (ctx_load_bytes(ctx->ctx_buff, icmp6_off,
					   &icmp6_type, sizeof(icmp6_type)) ==
				    0 &&
			    icmp6_type == ICMPV6_TYPE_ROUTER_SOLICITATION) {
				upf_printk("upf: RS detected for teid:%d",
					   teid);
				struct rs_event ev = {
					.teid = teid,
				};
				__builtin_memcpy(&ev.ue_ipv6, &ctx->ip6->saddr,
						 sizeof(struct in6_addr));
				bpf_ringbuf_output(&rs_event_map, &ev,
						   sizeof(ev), 0);
				PROFILE_END(PROF_N3_GTP_MANIP);
				return drop_with(ctx,
						 UPF_DROP_RS_INTERCEPTED);
			}
		}

		/* after RS interception, inside the decap branch — both load-bearing */
		if (!source_allowed(ctx, pdr)) {
			upf_printk("upf: uplink source spoof drop teid:%d",
				   teid);
			PROFILE_END(PROF_N3_GTP_MANIP);
			return drop_with(ctx, ctx->ip6 ?
						      UPF_DROP_SOURCE_SPOOF_IPV6 :
						      UPF_DROP_SOURCE_SPOOF_IPV4);
		}

		/* Deferred from the MTU check above, which ran before
		 * decapsulation. The subscriber's packet is in the context now,
		 * so DF and the family are its own.
		 *
		 * The packet is dropped either way — the datapath does not
		 * fragment. DF clear keeps its own reason because a router that
		 * did fragment would have delivered it, which is worth telling
		 * apart from a packet whose sender asked for this. No ICMP is
		 * generated: reaching the UE means building the error from this
		 * header and encapsulating it into the session's downlink
		 * tunnel, which this path cannot do. */
		if (inner_exceeds_n6_mtu) {
			upf_printk("upf: n3 inner packet exceeds n6 mtu");

			if (ctx->ip4 &&
			    (ctx->ip4->frag_off & bpf_htons(0x4000)) == 0)
				return drop_with(ctx, UPF_DROP_DF_NOT_SET);

			return drop_with(ctx, UPF_DROP_MTU_EXCEEDED);
		}

		/* After source_allowed: a spoofed address must not be able to
		 * plant an entry. Uplink is not translated until routing, so
		 * the header still holds the addresses the next fragment will
		 * carry. */
		if (ctx->is_fragment && ctx->ip4 && !ctx->l4_unavailable &&
		    !ctx->frag_recovered)
			frag_record4(ctx->ip4, ctx->ip4->saddr, ctx->ip4->daddr,
				     ctx->l4_sport, ctx->l4_dport);

		frag_record6(ctx);
	}
	PROFILE_END(PROF_N3_GTP_MANIP);

	/* Without decapsulation the context still holds the tunnel headers,
	 * whose addresses and ports are the UPF's and its peer's. */
	if (!ctx->gtp) {
		/* No policy is evaluable against an unwalkable chain, and the
		 * session is known by now (RFC 7112 §5). */
		if (ctx->exthdr_invalid) {
			upf_printk("upf: uplink unparsable exthdr chain teid:%d",
				   teid);
			account_flow(ctx, n6_ifindex, pdr->imsi, IPV6, FLOW_UPLINK, DROP);
			return drop_with(ctx, UPF_DROP_EXTHDR_INVALID);
		}

		PROFILE_START(PROF_N3_SDF_FILTER);
		enum ctx_action sdf_verdict =
			match_sdf_filters(ctx, pdr->filter_map_index);
		PROFILE_END(PROF_N3_SDF_FILTER);
		if (sdf_verdict == CTX_ACT_DROP) {
			upf_printk("upf: uplink SDF drop teid:%d", teid);
			account_flow(ctx, n6_ifindex, pdr->imsi,
				     ctx->ip4 ? IPV4 : IPV6, FLOW_UPLINK, DROP);
			return drop_reported(ctx, UPF_DROP_SDF_FILTER);
		}
	}

	/* Before routing resizes the frame. */
	const __u64 billed_bytes = ctx_full_len(ctx->ctx_buff);

	/* Account uplink traffic */
	ctx->statistics->byte_counter.bytes += billed_bytes;

	const __u32 key = 0;
	struct route_stat *route_statistic =
		bpf_map_lookup_elem(&uplink_route_stats, &key);
	if (!route_statistic)
		return abort_with(ctx, UPF_DROP_INTERNAL_MAP_LOOKUP_FAILED);

	/* After routing: the subscriber is billed on this, and every FIB failure
	 * is reachable from here — fib_no_neigh is ordinary while a neighbour
	 * resolves. The counter and flow record stay ahead of it because the
	 * inner tuple does not survive encapsulation. */
	if (ctx->ip4) {
		account_flow(ctx, n6_ifindex, pdr->imsi, IPV4, FLOW_UPLINK, ALLOW);
		PROFILE_START(PROF_N3_FIB_ROUTING);
		enum ctx_action fib_ret =
			route_ipv4(ctx, route_statistic, false);
		PROFILE_END(PROF_N3_FIB_ROUTING);

		if (ctx_action_forwards(fib_ret))
			update_urr_bytes(ctx, pdr->local_seid, urr_id,
					 billed_bytes);

		return fib_ret;
	} else if (ctx->ip6) {
		account_flow(ctx, n6_ifindex, pdr->imsi, IPV6, FLOW_UPLINK, ALLOW);
		PROFILE_START(PROF_N3_FIB_ROUTING);
		enum ctx_action fib_ret =
			route_ipv6(ctx, route_statistic, false);
		PROFILE_END(PROF_N3_FIB_ROUTING);

		if (ctx_action_forwards(fib_ret))
			update_urr_bytes(ctx, pdr->local_seid, urr_id,
					 billed_bytes);

		return fib_ret;
	} else {
		return abort_with(ctx, UPF_DROP_MALFORMED_HEADER);
	}
}

static __always_inline enum ctx_action handle_gtpu(struct packet_context *ctx)
{
	int pdu_type = parse_gtp(ctx);
	switch (pdu_type) {
	case GTPU_G_PDU:
		return handle_gtp_packet(ctx);
	case GTPU_ECHO_REQUEST:
		if (ctx->ip4)
			upf_printk("upf: gtp echo request [ %pI4 -> %pI4 ]",
				   &ctx->ip4->saddr, &ctx->ip4->daddr);
		return gtpu_control_tail_call(ctx);
	case GTPU_ECHO_RESPONSE:
		return CTX_ACT_OK;
	case GTPU_ERROR_INDICATION:
	case GTPU_SUPPORTED_EXTENSION_HEADERS_NOTIFICATION:
	case GTPU_END_MARKER:
		return DEFAULT_CTX_ACTION;
	default:
		upf_printk("upf: unexpected gtp message: type=%d", pdu_type);
		return DEFAULT_CTX_ACTION;
	}
}
