// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestSessionTypeConversionsCoverOnlyTheSharedRange(t *testing.T) {
	shared := []struct {
		pduSessionType fgs.PDUSessionType
		pdnType        eps.PDNType
	}{
		{fgs.PDUSessionTypeIPv4, eps.PDNTypeIPv4},
		{fgs.PDUSessionTypeIPv6, eps.PDNTypeIPv6},
		{fgs.PDUSessionTypeIPv4v6, eps.PDNTypeIPv4v6},
	}

	for _, tc := range shared {
		got, err := pdnTypeFor(uint8(tc.pduSessionType))
		if err != nil || got != tc.pdnType {
			t.Errorf("pdnTypeFor(%d) = %d, %v; want %d", tc.pduSessionType, got, err, tc.pdnType)
		}

		back, err := pduSessionTypeFor(uint8(tc.pdnType))
		if err != nil || back != tc.pduSessionType {
			t.Errorf("pduSessionTypeFor(%d) = %d, %v; want %d", tc.pdnType, back, err, tc.pduSessionType)
		}
	}

	// The enumerations diverge at 4 and above.
	for _, v := range []uint8{0, 4, 5, 6, 7} {
		if got, err := pdnTypeFor(v); err == nil {
			t.Errorf("pdnTypeFor(%d) = %d, want a refusal", v, got)
		}
	}

	for _, v := range []uint8{0, 4, 5, 6, 7} {
		if got, err := pduSessionTypeFor(v); err == nil {
			t.Errorf("pduSessionTypeFor(%d) = %d, want a refusal", v, got)
		}
	}
}

// TS 24.301 §6.5.1.4.1.
func TestPDNTypeRejectCause(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested eps.PDNType
		policy    *Policy
		want      eps.ESMCause
	}{
		{"IPv6 asked, IPv4-only network", eps.PDNTypeIPv6, &Policy{IPv4Pool: "10.0.0.0/24"}, eps.ESMCausePDNTypeIPv4OnlyAllowed},
		{"IPv4 asked, IPv6-only network", eps.PDNTypeIPv4, &Policy{IPv6Pool: "2001:db8::/48"}, eps.ESMCausePDNTypeIPv6OnlyAllowed},
		{"nothing available", eps.PDNTypeIPv4v6, &Policy{}, eps.ESMCauseUnknownPDNType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pdnTypeRejectCause(uint8(tc.requested), tc.policy); got != tc.want {
				t.Errorf("cause = %s, want %s", got, tc.want)
			}
		})
	}
}

// TS 24.301 §6.1: 1..4 are reserved.
func TestSessionIdentityRejectsReservedBearerIdentities(t *testing.T) {
	for ebi := uint8(1); ebi <= 4; ebi++ {
		if (SessionIdentity{EBI: ebi}).valid() {
			t.Errorf("valid() = true for EBI %d, want false", ebi)
		}
	}

	if !(SessionIdentity{EBI: 5}).valid() {
		t.Error("valid() = false for EBI 5, want true")
	}

	if (SessionIdentity{PDUSessionID: 3, EBI: 2}).valid() {
		t.Error("valid() = true for a reserved EBI with a PDU session identity, want false")
	}
}
