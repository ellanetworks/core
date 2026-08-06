// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// The EPS PDN type (TS 24.301 §9.9.4.10) and the 5GS PDU session type
// (TS 24.501 §9.11.4.11) agree on IPv4, IPv6 and IPv4v6 and diverge above them:
// PDN type 5 is non-IP and 6 is Ethernet, while PDU session type 4 is
// Unstructured and 5 is Ethernet. Converting one to the other by numeric cast is
// therefore right only inside the range the two share, and silently wrong
// outside it — a PDU session type of Ethernet would read as the PDN type non-IP.
// TS 24.501 §6.1.4.2 b) gives the real mapping in both directions, including
// that non-IP resolves to Unstructured absent local state; the pair below refuse
// anything that needs it rather than guess, because this core establishes only
// IP sessions (negotiatePDUSessionType admits nothing else).

// pdnTypeFor maps a session's PDU session type to the PDN type that carries it
// over EPS (TS 24.501 §6.1.4.2 b).
func pdnTypeFor(pduSessionType uint8) (eps.PDNType, error) {
	switch fgs.PDUSessionType(pduSessionType) {
	case fgs.PDUSessionTypeIPv4:
		return eps.PDNTypeIPv4, nil
	case fgs.PDUSessionTypeIPv6:
		return eps.PDNTypeIPv6, nil
	case fgs.PDUSessionTypeIPv4v6:
		return eps.PDNTypeIPv4v6, nil
	}

	return 0, fmt.Errorf("PDU session type %d has no PDN type this core serves", pduSessionType)
}

// pduSessionTypeFor maps a session's PDN type to the PDU session type that
// carries it over 5GS (TS 24.501 §6.1.4.2 b).
func pduSessionTypeFor(pdnType uint8) (fgs.PDUSessionType, error) {
	switch eps.PDNType(pdnType) {
	case eps.PDNTypeIPv4:
		return fgs.PDUSessionTypeIPv4, nil
	case eps.PDNTypeIPv6:
		return fgs.PDUSessionTypeIPv6, nil
	case eps.PDNTypeIPv4v6:
		return fgs.PDUSessionTypeIPv4v6, nil
	}

	return 0, fmt.Errorf("PDN type %d has no PDU session type this core serves", pdnType)
}

// pdnTypeRejectCause maps a failed PDN type negotiation to the ESM cause
// TS 24.301 §6.5.1.4.1 prescribes, mirroring pduSessionTypeRejectCause on the
// 5GS side. Without it the UE is told only "request rejected, unspecified" and
// cannot tell that retrying the same type is futile.
//
//   - IPv6 requested, only IPv4 supported → #50 PDN type IPv4 only allowed
//   - IPv4 requested, only IPv6 supported → #51 PDN type IPv6 only allowed
//   - nothing the UE asked for can be served → #28 unknown PDN type
func pdnTypeRejectCause(requested uint8, policy *Policy) eps.ESMCause {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch eps.PDNType(requested) {
	case eps.PDNTypeIPv6:
		if hasIPv4 && !hasIPv6 {
			return eps.ESMCausePDNTypeIPv4OnlyAllowed
		}
	case eps.PDNTypeIPv4:
		if !hasIPv4 && hasIPv6 {
			return eps.ESMCausePDNTypeIPv6OnlyAllowed
		}
	}

	return eps.ESMCauseUnknownPDNType
}
