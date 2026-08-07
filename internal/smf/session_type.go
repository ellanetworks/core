// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// The EPS PDN type (TS 24.301 §9.9.4.10) and the 5GS PDU session type
// (TS 24.501 §9.11.4.11) agree on IPv4, IPv6 and IPv4v6, and diverge above them.

// TS 24.501 §6.1.4.2 b.
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

// narrow5GSMCause is the cause an accept carries when the UE asked for IPv4v6
// and the data network offers one family: 5GSM #50/#51 (TS 24.501 §6.4.1.3).
// nil when the type was not narrowed.
func narrow5GSMCause(requested, negotiated uint8) *fgs.GSMCause {
	switch narrowPDUType(requested, negotiated) {
	case narrowIPv4Only:
		return new(fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed)
	case narrowIPv6Only:
		return new(fgs.GSMCausePDUSessionTypeIPv6OnlyAllowed)
	}

	return nil
}

// narrowESMCause is the same narrowing on the EPS side: ESM #50/#51
// (TS 24.301 §6.5.1.3). 0 when the type was not narrowed.
func narrowESMCause(requested, negotiated uint8) eps.ESMCause {
	switch narrowPDUType(requested, negotiated) {
	case narrowIPv4Only:
		return eps.ESMCausePDNTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		return eps.ESMCausePDNTypeIPv6OnlyAllowed
	}

	return 0
}

// TS 24.301 §6.5.1.4.1.
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
