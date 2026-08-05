// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

/*
 * Why the datapath did not forward a frame, as one dimension of
 * app_upf_datapath_drop_total. Direction is not in this enum: the uplink and
 * downlink pipelines keep separate statistics maps.
 *
 * Values are dense and never reused, so a historical series keeps its meaning:
 * an entry that becomes unreachable is kept and marked.
 */
enum upf_drop_reason {
	/* A drop site that skipped drop_with()/abort_with(). */
	UPF_DROP_UNSPEC = 0,

	/* Session and policy: expected in normal operation. */
	UPF_DROP_NO_UPLINK_SESSION,
	UPF_DROP_NO_DOWNLINK_SESSION,
	UPF_DROP_FAR_NO_FORWARD,
	UPF_DROP_FAR_NO_ENCAP,
	/* An uplink FAR requesting GTP-to-GTP forwarding, which n3_bpf.h
	 * refuses. */
	UPF_DROP_FAR_UNSUPPORTED,
	UPF_DROP_QER_GATE_CLOSED,
	UPF_DROP_QER_RATE_LIMIT,
	UPF_DROP_SDF_FILTER,
	UPF_DROP_NOCP_BUFFER,
	/* Split by family rather than carrying a label that would be empty on
	 * every other reason. */
	UPF_DROP_SOURCE_SPOOF_IPV4,
	UPF_DROP_SOURCE_SPOOF_IPV6,
	UPF_DROP_DECAP_FAMILY_MISMATCH,
	UPF_DROP_ENCAP_GSO,
	UPF_DROP_DECAP_GSO,
	UPF_DROP_DF_NOT_SET,
	/* Not a failure: the Router Solicitation went to the RA responder over
	 * a ring buffer. Counted like any other non-forward so the totals stay
	 * closed. */
	UPF_DROP_RS_INTERCEPTED,

	/* NAT. */
	UPF_DROP_NAT_UNSOLICITED,
	UPF_DROP_NAT_FRAGMENT,
	UPF_DROP_NAT_PORT_EXHAUSTED,
	UPF_DROP_NAT_UNSUPPORTED_PROTO,
	UPF_DROP_NAT_MALFORMED,
	UPF_DROP_NAT_TRANSLATE_FAILED,

	/* Routing: the kernel FIB declined to give us a nexthop. */
	UPF_DROP_FIB_NO_NEIGH,
	UPF_DROP_FIB_BLACKHOLE,
	UPF_DROP_FIB_UNREACHABLE,
	UPF_DROP_FIB_PROHIBIT,
	UPF_DROP_FIB_NO_SRC_ADDR,
	UPF_DROP_FIB_FRAG_NEEDED,
	UPF_DROP_FIB_ERROR,
	UPF_DROP_IFINDEX_MISMATCH,

	/* Malformed input: the peer sent something the datapath cannot parse. */
	UPF_DROP_MALFORMED_GTP,
	UPF_DROP_MALFORMED_HEADER,

	/* Datapath failures: one alert watches the prefix, and they should be
	 * zero forever. */
	UPF_DROP_INTERNAL_PULL_FAILED,
	UPF_DROP_INTERNAL_MTU_CHECK_FAILED,
	UPF_DROP_INTERNAL_ENCAP_FAILED,
	UPF_DROP_INTERNAL_DECAP_FAILED,
	UPF_DROP_INTERNAL_CSUM_FAILED,
	UPF_DROP_INTERNAL_RESIZE_FAILED,
	UPF_DROP_INTERNAL_WRITE_FAILED,
	UPF_DROP_INTERNAL_MAP_LOOKUP_FAILED,
	UPF_DROP_INTERNAL_TX_FAILED,

	/* Appended so existing reasons keep their values. */

	/* Non-first fragment, for a policy that scopes on ports. */
	UPF_DROP_FRAGMENT_UNFILTERABLE,
	/* Chain truncated, or longer than the parser walks. */
	UPF_DROP_EXTHDR_INVALID,
	/* An ICMP error whose quoted datagram has no NAT mapping to reverse. */
	UPF_DROP_NAT_QUOTE_NO_MAPPING,
	/* Shaped to move header fields out of the first fragment (RFC 3128). */
	UPF_DROP_FRAGMENT_MALFORMED,

	UPF_DROP_REASON_COUNT,
};

/* A power of two so the verifier accepts the masked index without a bounds
 * branch. */
#define UPF_DROP_REASON_MAX 64
#define UPF_DROP_REASON_MASK 0x3f

_Static_assert(UPF_DROP_REASON_COUNT <= UPF_DROP_REASON_MAX,
	       "drop reason enum outgrew drop_reasons[]; raise "
	       "UPF_DROP_REASON_MAX and its mask together");
