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
#include "bpf/utils/profiling.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/in6.h>
#include <linux/ipv6.h>

#include "bpf/utils/common.h"
#include "bpf/utils/frag_needed.h"
#include "bpf/utils/gtp.h"
#include "bpf/utils/pdr.h"
#include "bpf/utils/qer.h"
#include "bpf/utils/sdf.h"
#include "bpf/utils/urr.h"
#include "bpf/utils/routing.h"
#include "bpf/utils/statistics.h"
#include "bpf/utils/nocp.h"

#include "bpf/utils/pdr_maps.h"

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct route_stat);
	__uint(max_entries, 1);
} downlink_route_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct upf_statistic);
	__uint(max_entries, 1);
} downlink_statistics SEC(".maps");

/*
 * The IPv4 and IPv6 branches each end in their own return so the verifier
 * never merges a state where context_set_ip6 cleared ctx->ip4 with one that
 * calls route_ipv4: merged, both appear as plain scalars and every
 * dereference is rejected.
 */
static __always_inline enum ctx_action
send_to_gtp_tunnel(struct packet_context *ctx, const struct far_info *far,
		   __u8 tos, __u8 qfi)
{
	if (far->outer_header_creation & OHC_GTP_U_UDP_IPv6) {
		PROFILE_START(PROF_N6_GTP_MANIP);
		__u32 encap_result =
			(far->outer_header_creation & OHC_NO_PSC) ?
				add_gtp_over_ip6_headers_s1u(ctx, &far->localip,
							     &far->remoteip,
							     tos, far->teid) :
				add_gtp_over_ip6_headers(ctx, &far->localip,
							 &far->remoteip, tos,
							 qfi, far->teid);
		if (encap_result != 0) {
			PROFILE_END(PROF_N6_GTP_MANIP);
			return abort_with(ctx, UPF_DROP_INTERNAL_ENCAP_FAILED);
		}
		PROFILE_END(PROF_N6_GTP_MANIP);

		ctx->statistics->packet_counters.tx++;

		const __u32 key6 = 0;
		struct route_stat *route_stat6 =
			bpf_map_lookup_elem(&downlink_route_stats, &key6);
		if (!route_stat6)
			return abort_with(ctx, UPF_DROP_INTERNAL_MAP_LOOKUP_FAILED);

		PROFILE_START(PROF_N6_FIB_ROUTING);
		upf_printk("upf: send gtp pdu %pI6c -> %pI6c", &ctx->ip6->saddr,
			   &ctx->ip6->daddr);
		enum ctx_action fib_ret6 = route_ipv6(ctx, route_stat6, false);
		PROFILE_END(PROF_N6_FIB_ROUTING);
		return fib_ret6;
	} else {
		PROFILE_START(PROF_N6_GTP_MANIP);
		__u32 encap_result =
			(far->outer_header_creation & OHC_NO_PSC) ?
				add_gtp_over_ip4_headers_s1u(
					ctx, ipv4_from_mapped(&far->localip),
					ipv4_from_mapped(&far->remoteip), tos,
					far->teid) :
				add_gtp_over_ip4_headers(
					ctx, ipv4_from_mapped(&far->localip),
					ipv4_from_mapped(&far->remoteip), tos,
					qfi, far->teid);
		if (encap_result != 0) {
			PROFILE_END(PROF_N6_GTP_MANIP);
			return abort_with(ctx, UPF_DROP_INTERNAL_ENCAP_FAILED);
		}
		PROFILE_END(PROF_N6_GTP_MANIP);

		ctx->statistics->packet_counters.tx++;

		const __u32 key4 = 0;
		struct route_stat *route_stat4 =
			bpf_map_lookup_elem(&downlink_route_stats, &key4);
		if (!route_stat4)
			return abort_with(ctx, UPF_DROP_INTERNAL_MAP_LOOKUP_FAILED);

		PROFILE_START(PROF_N6_FIB_ROUTING);
		upf_printk("upf: send gtp pdu %pI4 -> %pI4", &ctx->ip4->saddr,
			   &ctx->ip4->daddr);
		enum ctx_action fib_ret4 = route_ipv4(ctx, route_stat4, false);
		PROFILE_END(PROF_N6_FIB_ROUTING);
		return fib_ret4;
	}
}

static __always_inline __u16 handle_n6_packet_ipv4(struct packet_context *ctx)
{
	if (!ctx->ip4)
		return abort_with(ctx, UPF_DROP_INTERNAL_PULL_FAILED);

	bool translated = false;
	bool counted = false;
	struct nat_xlate xlate = {};
	/* A later fragment reaches here with the ports its first fragment
	 * recorded, so it resolves to the same mapping. One whose datagram was
	 * never recorded has none, and cannot be translated. */
	const bool fragment = ctx->l4_unavailable;
	if (masquerade && !fragment) {
		PROFILE_START(PROF_N6_NAT);
		translated = destination_nat_lookup(ctx, &xlate, &counted);
		PROFILE_END(PROF_N6_NAT);
	}
	const struct iphdr *ip4 = ctx->ip4;
	/* Read before destination_nat_apply rewrites them: frag_resolve4 runs
	 * ahead of translation, so the record has to key on what the next
	 * fragment will arrive with, not on the UE's. The ports matter as much
	 * as the addresses — under the skb build destination_nat_apply
	 * re-parses the frame, so by the record site they read post-translation.
	 * They are meaningful only once destination_nat_lookup has parsed them,
	 * which is exactly when translation can rewrite them. */
	const __u32 wire_saddr = ip4->saddr;
	const __u32 wire_daddr = ip4->daddr;
	const __u16 wire_sport = ctx->l4_sport;
	const __u16 wire_dport = ctx->l4_dport;
	__u32 ue_addr = translated ? xlate.daddr : ip4->daddr;

	PROFILE_START(PROF_N6_PDR_LOOKUP);
	struct pdr_info *pdr =
		bpf_map_lookup_elem(&pdrs_downlink_ip4, &ue_addr);
	PROFILE_END(PROF_N6_PDR_LOOKUP);
	if (!pdr) {
		/* Not a UE address: try the framed-route table (TS 29.244 §5.16).
		 * The entry redirects to the owning UE address so the live downlink
		 * PDR is the single source of truth. */
		struct framed_ip4_key fk = { .prefixlen = 32, .addr = ue_addr };
		__u32 *ue_ip = bpf_map_lookup_elem(&framed_downlink_ip4, &fk);
		if (ue_ip) {
			pdr = bpf_map_lookup_elem(&pdrs_downlink_ip4, ue_ip);
		}
		if (!pdr) {
			upf_printk("upf: no downlink session for ip:%pI4",
				   &ue_addr);
			/* A conntrack hit makes this the UE's packet; the host
			 * stack has no session to answer it with. */
			if (translated)
				return drop_with(ctx, UPF_DROP_NAT_UNSOLICITED);

			return DEFAULT_CTX_ACTION;
		}
	}

	/* With NAT the UE address is not visible on N6 (TS 23.501
	 * §5.8.2.2.1): untranslated downlink to a UE is unsolicited. */
	if (masquerade && !translated) {
		upf_printk("upf: unsolicited downlink for ip:%pI4",
			   &ip4->daddr);
		if (fragment)
			set_drop_reason(ctx, UPF_DROP_NAT_FRAGMENT);
		else if (!counted)
			set_drop_reason(ctx, UPF_DROP_NAT_UNSOLICITED);

		account_flow(ctx, n3_ifindex, pdr->imsi, IPV4, FLOW_DOWNLINK, DROP);

		return drop_reported(ctx, UPF_DROP_NO_DOWNLINK_SESSION);
	}

	struct far_info *far = &pdr->far;
	struct qer_info *qer = &pdr->qer;

	PROFILE_START(PROF_N6_MTU_CHECK);
	__u32 mtu_len = 0;
	long ret = 0;
	int encap_size = (far->outer_header_creation & OHC_GTP_U_UDP_IPv6) ?
				 GTP_ENCAP_SIZE_IPV6 :
				 GTP_ENCAP_SIZE_IPV4;
	if (far->outer_header_creation & OHC_NO_PSC) {
		encap_size -=
			GTP_PSC_EXT_SIZE; /* S1-U: no PDU session container */
	}
	ret = bpf_check_mtu(ctx->ctx_buff, n3_ifindex, &mtu_len, encap_size, 0);
	PROFILE_END(PROF_N6_MTU_CHECK);
	if (ret < 0) {
		return abort_with(ctx, UPF_DROP_INTERNAL_MTU_CHECK_FAILED);
	}
	if (ret > 0) {
		upf_printk("upf: n6 packet too large");
		mtu_len -= encap_size;
		/* The error must quote the packet as the sender sent it for the
		 * sender to match it to a socket (RFC 792). */
		return frag_needed(ctx, mtu_len);
	}

	PROFILE_START(PROF_N6_NAT);
	destination_nat_apply(ctx, &xlate);
	PROFILE_END(PROF_N6_NAT);

	if (CTX_L4_CSUM_VIA_HELPERS) {
		/* The frame parsed as IPv4 above, so a missing header here is a
		 * failed re-parse inside destination_nat_apply. */
		ip4 = ctx->ip4;
		if (!ip4)
			return abort_with(ctx, UPF_DROP_MALFORMED_HEADER);
	}

	ctx->interface = INTERFACE_N6;

	__u32 urr_id = pdr->urr_id;

	upf_printk("upf: downlink session for ip:%pI4 action:%d", &ip4->daddr,
		   far->action);

	if (far->action & (FAR_BUFF | FAR_NOCP)) {
		upf_printk("upf: need to notify CP for pdr:%d and qfi:%d",
			   pdr->pdr_id, qer->qfi);
		struct nocp notif = { .local_seid = pdr->local_seid,
				      .pdr_id = pdr->pdr_id,
				      .qfi = qer->qfi };
		bpf_ringbuf_output(&nocp_map, (void *)&notif,
				   sizeof(struct nocp), 0);

		/* Technically, we need to buffer the packet here, but this is not
		 * implemented yet. */
		return drop_with(ctx, UPF_DROP_NOCP_BUFFER);
	}
	if (!(far->action & FAR_FORW)) {
		upf_printk("upf: far not set to forward, dropping packet");
		return drop_with(ctx, UPF_DROP_FAR_NO_FORWARD);
	}
	if (!(far->outer_header_creation &
	      (OHC_GTP_U_UDP_IPv4 | OHC_GTP_U_UDP_IPv6))) {
		upf_printk(
			"upf: far not set to encapsulate in gtp, dropping packet");
		return drop_with(ctx, UPF_DROP_FAR_NO_ENCAP);
	}
	/* Segmentation replays the tunnel span verbatim, so every segment would
	 * carry the merged frame's GTP-U message_length (TS 29.281 §5.1) and,
	 * over IPv6, its outer UDP checksum. */
	if (frame_is_merged(ctx)) {
		upf_printk("upf: merged frame on the encap path, dropping");
		return drop_with(ctx, UPF_DROP_ENCAP_GSO);
	}

	PROFILE_START(PROF_N6_QER_RATELIMIT);
	upf_printk("upf: qer gate_status:%d mbr:%d", qer->dl_gate_status,
		   qer->dl_maximum_bitrate);
	if (qer->dl_gate_status != GATE_STATUS_OPEN) {
		PROFILE_END(PROF_N6_QER_RATELIMIT);
		return drop_with(ctx, UPF_DROP_QER_GATE_CLOSED);
	}

	/* Shared with this session's other downlink PDR. */
	if (qer->dl_maximum_bitrate != 0) {
		const __u64 packet_size =
			ctx_len_from(ctx->ctx_buff, ctx->data_end, ctx->ip4);
		struct qer_window *window =
			qer_window_for(pdr->local_seid, pdr->qer_id);

		if (window &&
		    CTX_ACT_DROP == limit_rate_sliding_window(
					    packet_size, &window->dl_start,
					    qer->dl_maximum_bitrate)) {
			PROFILE_END(PROF_N6_QER_RATELIMIT);
			return drop_with(ctx, UPF_DROP_QER_RATE_LIMIT);
		}
	}
	PROFILE_END(PROF_N6_QER_RATELIMIT);

	/* Parse inner L4 so match_sdf_filters can inspect protocol/ports */
	parse_l4(ip4->protocol, ctx);

	/* A later fragment whose ports came from the map has nothing of its
	 * own to record. */
	if (ctx->is_fragment && !ctx->l4_unavailable && !ctx->frag_recovered)
		frag_record4(ip4, wire_saddr, wire_daddr,
			     translated ? wire_sport : ctx->l4_sport,
			     translated ? wire_dport : ctx->l4_dport);

	/* SDF filter enforcement (downlink) */
	{
		PROFILE_START(PROF_N6_SDF_FILTER);
		enum ctx_action sdf_verdict =
			match_sdf_filters(ctx, pdr->filter_map_index);
		PROFILE_END(PROF_N6_SDF_FILTER);
		if (sdf_verdict == CTX_ACT_DROP) {
			upf_printk("upf: downlink SDF drop ip:%pI4",
				   &ip4->daddr);
			account_flow(ctx, n3_ifindex, pdr->imsi, IPV4, FLOW_DOWNLINK, DROP);
			return drop_reported(ctx, UPF_DROP_SDF_FILTER);
		}
	}

	__u8 tos = far->transport_level_marking >> 8;
	upf_printk("upf: use mapping %pI4 -> TEID:%d", &ip4->daddr, far->teid);

	/* Captured before encapsulation resizes the frame. */
	const __u64 billed_bytes = ctx_full_len(ctx->ctx_buff);

	account_flow(ctx, n3_ifindex, pdr->imsi, IPV4, FLOW_DOWNLINK, ALLOW);

	/* Only if the frame leaves: encapsulation and routing can still fail. */
	enum ctx_action tunnel_ret = send_to_gtp_tunnel(ctx, far, tos, qer->qfi);

	if (ctx_action_forwards(tunnel_ret)) {
		/* Exported throughput follows the verdict, as billing does. */
		ctx->statistics->byte_counter.bytes += billed_bytes;
		update_urr_bytes(ctx, pdr->local_seid, urr_id, billed_bytes);
	}

	return tunnel_ret;
}

static __always_inline enum ctx_action
handle_n6_packet_ipv6(struct packet_context *ctx)
{
	if (!ctx->ip6)
		return abort_with(ctx, UPF_DROP_INTERNAL_PULL_FAILED);

	const struct ipv6hdr *ip6 = ctx->ip6;

	PROFILE_START(PROF_N6_PDR_LOOKUP);
	/* Each IPv6 PDU session gets a /64, and the prefix is the PDR key. */
	struct in6_addr prefix = ip6->daddr;
	__builtin_memset(((void *)&prefix) + 8, 0, 8);

	struct pdr_info *pdr = bpf_map_lookup_elem(&pdrs_downlink_ip6, &prefix);
	PROFILE_END(PROF_N6_PDR_LOOKUP);
	if (!pdr) {
		/* Not a UE /64: try the framed-route table on the full destination
		 * (TS 29.244 §5.16), longest-prefix matched. The entry redirects to
		 * the owning UE /64 so the live downlink PDR is the single source of
		 * truth. */
		struct framed_ip6_key fk = { .prefixlen = 128,
					     .addr = ip6->daddr };
		struct in6_addr *ue_prefix =
			bpf_map_lookup_elem(&framed_downlink_ip6, &fk);
		if (ue_prefix) {
			pdr = bpf_map_lookup_elem(&pdrs_downlink_ip6,
						  ue_prefix);
		}
		if (!pdr) {
			upf_printk("upf: no downlink session for ip:%pI6c",
				   &prefix);
			return DEFAULT_CTX_ACTION;
		}
	}

	struct far_info *far = &pdr->far;
	struct qer_info *qer = &pdr->qer;

	int encap_size = (far->outer_header_creation & OHC_GTP_U_UDP_IPv6) ?
				 GTP_ENCAP_SIZE_IPV6 :
				 GTP_ENCAP_SIZE_IPV4;
	if (far->outer_header_creation & OHC_NO_PSC) {
		encap_size -=
			GTP_PSC_EXT_SIZE; /* S1-U: no PDU session container */
	}
	__u32 mtu_len = 0;
	long ret = bpf_check_mtu(ctx->ctx_buff, n3_ifindex, &mtu_len,
				 encap_size, 0);
	if (ret < 0) {
		return abort_with(ctx, UPF_DROP_INTERNAL_MTU_CHECK_FAILED);
	}
	if (ret > 0) {
		upf_printk("upf: n6 ipv6 packet too large");
		mtu_len -= encap_size;
		return frag_needed(ctx, mtu_len);
	}

	ctx->interface = INTERFACE_N6;

	/* For match_sdf_filters. */
	parse_l4(ctx->l4_proto, ctx);

	frag_record6(ctx);

	// IPv6 is not NATed (each UE owns its /64), so the inner L4 checksum is
	// unchanged; the outer GTP-over-IPv6 UDP checksum is built during
	// encapsulation in gtp.h.

	/* No policy is evaluable against an unwalkable chain, and the session is
	 * known by now (RFC 7112 §5). */
	if (ctx->exthdr_invalid) {
		upf_printk("upf: downlink unparsable exthdr chain ip:%pI6c",
			   &ip6->daddr);
		account_flow(ctx, n3_ifindex, pdr->imsi, IPV6, FLOW_DOWNLINK, DROP);
		return drop_with(ctx, UPF_DROP_EXTHDR_INVALID);
	}

	/* SDF filter enforcement (downlink) */
	{
		PROFILE_START(PROF_N6_SDF_FILTER);
		enum ctx_action sdf_verdict =
			match_sdf_filters(ctx, pdr->filter_map_index);
		PROFILE_END(PROF_N6_SDF_FILTER);
		if (sdf_verdict == CTX_ACT_DROP) {
			upf_printk("upf: downlink SDF drop ip:%pI6c",
				   &ip6->daddr);
			account_flow(ctx, n3_ifindex, pdr->imsi, IPV6, FLOW_DOWNLINK, DROP);
			return drop_reported(ctx, UPF_DROP_SDF_FILTER);
		}
	}

	upf_printk("upf: downlink session for ip:%pI6c action:%d", &ip6->daddr,
		   far->action);

	if (far->action & (FAR_BUFF | FAR_NOCP)) {
		upf_printk("upf: need to notify CP for pdr:%d and qfi:%d",
			   pdr->pdr_id, qer->qfi);
		struct nocp notif = { .local_seid = pdr->local_seid,
				      .pdr_id = pdr->pdr_id,
				      .qfi = qer->qfi };
		bpf_ringbuf_output(&nocp_map, (void *)&notif,
				   sizeof(struct nocp), 0);

		/* Technically, we need to buffer the packet here, but this is not
		 * implemented yet. */
		return drop_with(ctx, UPF_DROP_NOCP_BUFFER);
	}
	if (!(far->action & FAR_FORW)) {
		return drop_with(ctx, UPF_DROP_FAR_NO_FORWARD);
	}
	if (!(far->outer_header_creation &
	      (OHC_GTP_U_UDP_IPv4 | OHC_GTP_U_UDP_IPv6))) {
		return drop_with(ctx, UPF_DROP_FAR_NO_ENCAP);
	}
	if (frame_is_merged(ctx)) {
		return drop_with(ctx, UPF_DROP_ENCAP_GSO);
	}

	upf_printk("upf: qer gate_status:%d mbr:%d", qer->dl_gate_status,
		   qer->dl_maximum_bitrate);
	if (qer->dl_gate_status != GATE_STATUS_OPEN) {
		return drop_with(ctx, UPF_DROP_QER_GATE_CLOSED);
	}

	/* Shared with this session's IPv4 downlink PDR: see the IPv4 path. */
	if (qer->dl_maximum_bitrate != 0) {
		const __u64 packet_size =
			ctx_len_from(ctx->ctx_buff, ctx->data_end, ctx->ip6);
		struct qer_window *window =
			qer_window_for(pdr->local_seid, pdr->qer_id);

		if (window &&
		    CTX_ACT_DROP == limit_rate_sliding_window(
					    packet_size, &window->dl_start,
					    qer->dl_maximum_bitrate)) {
			return drop_with(ctx, UPF_DROP_QER_RATE_LIMIT);
		}
	}

	__u8 tos = far->transport_level_marking >> 8;

	/* Captured before encapsulation resizes the frame. */
	const __u64 billed_bytes = ctx_full_len(ctx->ctx_buff);

	__u32 urr_id = pdr->urr_id;

	account_flow(ctx, n3_ifindex, pdr->imsi, IPV6, FLOW_DOWNLINK, ALLOW);

	/* As in the IPv4 path: billing follows the verdict. */
	enum ctx_action tunnel_ret = send_to_gtp_tunnel(ctx, far, tos, qer->qfi);

	if (ctx_action_forwards(tunnel_ret)) {
		/* Exported throughput follows the verdict, as billing does. */
		ctx->statistics->byte_counter.bytes += billed_bytes;
		update_urr_bytes(ctx, pdr->local_seid, urr_id, billed_bytes);
	}

	return tunnel_ret;
}
