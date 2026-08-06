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

// servedPDUSessionType vets a stored 5GS PDU session type against the range this
// core serves.
func servedPDUSessionType(pduSessionType uint8) (fgs.PDUSessionType, error) {
	switch t := fgs.PDUSessionType(pduSessionType); t {
	case fgs.PDUSessionTypeIPv4, fgs.PDUSessionTypeIPv6, fgs.PDUSessionTypeIPv4v6:
		return t, nil
	}

	return 0, fmt.Errorf("PDU session type %d is not served", pduSessionType)
}

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

// TS 24.501 §6.1.4.2 b.
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
