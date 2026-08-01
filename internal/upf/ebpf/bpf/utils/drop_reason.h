// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

/*
 * Why the datapath did not forward a frame.
 *
 * The reason is a dimension of one counter, not a counter of its own: the
 * datapath records drop_reasons[reason] and userspace publishes a single
 * app_upf_datapath_drop_total{direction,reason} series per value. Adding a
 * reason is therefore an enum entry and a name — no new map field, accessor,
 * metric or dashboard panel. That cost is why reasons get recorded at all.
 *
 * Direction is not part of this enum. The uplink and downlink pipelines keep
 * separate statistics maps, so it is already known at read time.
 *
 * Names starting with UPF_DROP_INTERNAL_ mean the datapath itself failed —
 * a helper returned an error, or a bounds check the verifier requires but
 * that cannot fail at runtime did. They are published with an `internal_`
 * prefix so one alert can watch all of them: they should be zero forever, and
 * any of them being non-zero is a datapath bug rather than a network event.
 *
 * Values are dense and never reused: an entry that becomes unreachable is
 * kept and marked, so a historical series keeps its meaning.
 */
enum upf_drop_reason {
	/* Not set by the drop site. Reaching this means a `return
	 * CTX_ACT_DROP` somewhere skipped drop_with()/abort_with(); the count
	 * is a visible bug, not a silent gap. */
	UPF_DROP_UNSPEC = 0,

	/* Session and policy: expected in normal operation. */
	UPF_DROP_NO_UPLINK_SESSION,
	UPF_DROP_NO_DOWNLINK_SESSION,
	UPF_DROP_FAR_NO_FORWARD,
	UPF_DROP_FAR_NO_ENCAP,
	UPF_DROP_FAR_UNSUPPORTED,
	UPF_DROP_QER_GATE_CLOSED,
	UPF_DROP_QER_RATE_LIMIT,
	UPF_DROP_SDF_FILTER,
	UPF_DROP_NOCP_BUFFER,
	/* Split by family rather than carrying an address-family label: the
	 * distinction is meaningful for this reason and for no other, and a
	 * label that is empty on 39 of 40 series is worse than two names. */
	UPF_DROP_SOURCE_SPOOF_IPV4,
	UPF_DROP_SOURCE_SPOOF_IPV6,
	UPF_DROP_DECAP_FAMILY_MISMATCH,
	UPF_DROP_ENCAP_GSO,
	UPF_DROP_DF_NOT_SET,
	/* Not a failure: the frame was consumed by the datapath and answered
	 * out of band. A Router Solicitation is handed to the RA responder
	 * over a ring buffer, so it stops here by design. Counted like any
	 * other non-forward so the totals stay closed. */
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

	/* Datapath failures. Should be zero forever. */
	UPF_DROP_INTERNAL_PULL_FAILED,
	UPF_DROP_INTERNAL_MTU_CHECK_FAILED,
	UPF_DROP_INTERNAL_ENCAP_FAILED,
	UPF_DROP_INTERNAL_DECAP_FAILED,
	UPF_DROP_INTERNAL_CSUM_FAILED,
	UPF_DROP_INTERNAL_RESIZE_FAILED,
	UPF_DROP_INTERNAL_WRITE_FAILED,
	UPF_DROP_INTERNAL_MAP_LOOKUP_FAILED,
	/* The frame could not be handed to the egress path: the VLAN tag could
	 * not be rewritten, or the redirect helper refused the target. */
	UPF_DROP_INTERNAL_TX_FAILED,

	UPF_DROP_REASON_COUNT,
};

/* Array width and index mask. A power of two so the verifier accepts the
 * masked index without a bounds branch; it must stay >= UPF_DROP_REASON_COUNT,
 * which the build asserts below. */
#define UPF_DROP_REASON_MAX 64
#define UPF_DROP_REASON_MASK 0x3f

_Static_assert(UPF_DROP_REASON_COUNT <= UPF_DROP_REASON_MAX,
	       "drop reason enum outgrew drop_reasons[]; raise "
	       "UPF_DROP_REASON_MAX and its mask together");
