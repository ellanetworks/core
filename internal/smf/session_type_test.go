// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// The two enumerations agree only on IPv4/IPv6/IPv4v6. Above them PDN type 5 is
// non-IP and 6 Ethernet, while PDU session type 4 is Unstructured and 5
// Ethernet, so a numeric cast would read an Ethernet PDU session as a non-IP PDN
// connection. This pins the shared range and that the divergent values are
// refused rather than mistranslated.
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

	// 4 is Unstructured as a PDU session type and unassigned as a PDN type; 5 is
	// Ethernet on one side and non-IP on the other; 6 is Ethernet as a PDN type
	// and reserved as a PDU session type.
	for _, v := range []uint8{0, 4, 5, 6, 7} {
		if got, err := pdnTypeFor(v); err == nil {
			t.Errorf("pdnTypeFor(%d) = %d, want a refusal: the enumerations diverge here", v, got)
		}
	}

	for _, v := range []uint8{0, 4, 5, 6, 7} {
		if got, err := pduSessionTypeFor(v); err == nil {
			t.Errorf("pduSessionTypeFor(%d) = %d, want a refusal: the enumerations diverge here", v, got)
		}
	}
}

// A UE told only "request rejected, unspecified" cannot tell that retrying the
// same PDN type is futile (TS 24.301 §6.5.1.4.1), so EPS names the cause as 5GS
// already does.
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

// The EPS bearer identity is the MME's to allocate from 5..15; 1..4 are reserved
// (TS 24.301 §6.1) and name no session.
func TestSessionIdentityRejectsReservedBearerIdentities(t *testing.T) {
	for ebi := uint8(1); ebi <= 4; ebi++ {
		if (SessionIdentity{EBI: ebi}).valid() {
			t.Errorf("EBI %d accepted, want it refused as reserved", ebi)
		}
	}

	if !(SessionIdentity{EBI: 5}).valid() {
		t.Error("EBI 5 refused, want it accepted")
	}

	// A reserved bearer identity alongside a usable PDU session identity is still
	// a malformed identity, not a degrade.
	if (SessionIdentity{PDUSessionID: 3, EBI: 2}).valid() {
		t.Error("reserved EBI accepted because a PDU session identity was present")
	}
}
