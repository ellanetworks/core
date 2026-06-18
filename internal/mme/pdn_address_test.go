// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/udm"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// activateFromAccept unprotects an Attach Accept and decodes the embedded
// Activate Default EPS Bearer Context Request.
func activateFromAccept(t *testing.T, m *MME, ue *UeContext) *eps.ActivateDefaultEPSBearerContextRequest {
	t.Helper()

	wire, err := m.buildProtectedAttachAccept(ue, &epsQoS{APN: "internet", QCI: 9, MTU: 1400})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	accept, err := eps.ParseAttachAccept(plain)
	if err != nil {
		t.Fatal(err)
	}

	activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(accept.ESMMessageContainer)
	if err != nil {
		t.Fatal(err)
	}

	return activate
}

// TestAttachAcceptIMSVoPS checks the Attach Accept advertises IMS voice over PS
// session in the EPS network feature support IE (TS 24.301 §9.9.3.12A), so a
// voice-centric UE stays on E-UTRAN (TS 23.221 §7.2a).
func TestAttachAcceptIMSVoPS(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).pdnType = eps.PDNTypeIPv4
	testPDN(ue).ueIP = testUEIP

	wire, err := m.buildProtectedAttachAccept(ue, &epsQoS{APN: "internet", QCI: 9, MTU: 1400})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	accept, err := eps.ParseAttachAccept(plain)
	if err != nil {
		t.Fatal(err)
	}

	if accept.EPSNetworkFeatureSupport == nil || !accept.EPSNetworkFeatureSupport.IMSVoPS {
		t.Fatalf("Attach Accept must advertise IMS VoPS, got %+v", accept.EPSNetworkFeatureSupport)
	}
}

// TestAttachAcceptDNSPCO checks an IPv4 bearer's PCO advertises both the DNS
// server (0x000D) and the IPv4 Link MTU (0x0010), TS 24.008 §10.5.6.3.
func TestAttachAcceptDNSPCO(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).pdnType = eps.PDNTypeIPv4
	testPDN(ue).ueIP = testUEIP
	testPDN(ue).dns = netip.MustParseAddr("8.8.8.8")

	activate := activateFromAccept(t, m, ue)

	want := eps.BuildProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)
	if !bytes.Equal(activate.ProtocolConfigurationOptions, want) {
		t.Fatalf("PCO = %x, want %x", activate.ProtocolConfigurationOptions, want)
	}
}

// TestAttachAcceptIPv6NoLinkMTU checks an IPv6-only bearer's PCO carries the
// IPv6 DNS but no IPv4 Link MTU (there is no IPv6 PCO MTU container; the IPv6
// link MTU is delivered via the Router Advertisement).
func TestAttachAcceptIPv6NoLinkMTU(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).pdnType = eps.PDNTypeIPv6
	testPDN(ue).ueIPv6IID = testUEIPv6IID
	testPDN(ue).dns = netip.MustParseAddr("2001:4860:4860::8888")

	activate := activateFromAccept(t, m, ue)

	dns := netip.MustParseAddr("2001:4860:4860::8888").As16()
	want := eps.BuildProtocolConfigurationOptions([][]byte{dns[:]}, 0)

	if !bytes.Equal(activate.ProtocolConfigurationOptions, want) {
		t.Fatalf("PCO = %x, want %x (IPv6 DNS, no IPv4 Link MTU)", activate.ProtocolConfigurationOptions, want)
	}
}

// TestAttachAcceptDowngradeCause checks an IPv4v6→IPv4 downgrade carries ESM
// cause #50 in the Activate Default (TS 24.301 §6.5.1.3).
func TestAttachAcceptDowngradeCause(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).pdnType = eps.PDNTypeIPv4
	testPDN(ue).ueIP = testUEIP
	testPDN(ue).esmCause = eps.ESMCausePDNTypeIPv4OnlyAllowed

	activate := activateFromAccept(t, m, ue)

	if activate.ESMCause == nil || *activate.ESMCause != eps.ESMCausePDNTypeIPv4OnlyAllowed {
		t.Fatalf("ESM cause = %v, want %d", activate.ESMCause, eps.ESMCausePDNTypeIPv4OnlyAllowed)
	}
}

// TestActivateDefaultBearerRejectsWhen4GNotAllowed checks that a subscriber on a
// profile that forbids 4G is rejected with EMM cause #7 "EPS services not
// allowed" (Core Network type restriction, TS 23.501 §5.3.4 / TS 24.301 §9.9.3.9).
func TestActivateDefaultBearerRejectsWhen4GNotAllowed(t *testing.T) {
	m := New(udm.New(newFakeCredStore(), noopKeyResolver), barredBearerStore{}, &fakeSessionManager{})
	ue, cc := securedUE(t, m)

	m.activateDefaultBearer(ue)

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + UE Context Release Command, got %d", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != emmCauseEPSServicesNotAllowed {
		t.Fatalf("Attach Reject cause = %d, want %d (EPS services not allowed)", rej.Cause, emmCauseEPSServicesNotAllowed)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TestActivateDefaultBearerRejectsOnSessionFailure checks that when the anchor
// cannot establish the default bearer, the attach is rejected with EMM cause
// #19 "ESM failure" and the S1 context is released (TS 24.301 §5.5.1.2.5).
func TestActivateDefaultBearerRejectsOnSessionFailure(t *testing.T) {
	m := New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, &erroringSessionManager{})
	ue, cc := securedUE(t, m)

	m.activateDefaultBearer(ue)

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + UE Context Release Command, got %d", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != emmCauseESMFailure {
		t.Fatalf("Attach Reject cause = %d, want %d (ESM failure)", rej.Cause, emmCauseESMFailure)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TestAttachAcceptPDNAddress checks the Attach Accept encodes the PDN Address per
// the negotiated PDN type (TS 24.301 §9.9.4.9): IPv4 carries the address, IPv6
// the SLAAC interface identifier, IPv4v6 both.
func TestAttachAcceptPDNAddress(t *testing.T) {
	cases := []struct {
		name    string
		pdnType uint8
		wantV4  bool
		wantV6  bool
	}{
		{"IPv4", eps.PDNTypeIPv4, true, false},
		{"IPv6", eps.PDNTypeIPv6, false, true},
		{"IPv4v6", eps.PDNTypeIPv4v6, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, _ := securedUE(t, m)
			testPDN(ue).pdnType = tc.pdnType
			testPDN(ue).ueIP = testUEIP
			testPDN(ue).ueIPv6IID = testUEIPv6IID

			wire, err := m.buildProtectedAttachAccept(ue, &epsQoS{APN: "internet", QCI: 9})
			if err != nil {
				t.Fatal(err)
			}

			plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
				ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
			if err != nil {
				t.Fatal(err)
			}

			accept, err := eps.ParseAttachAccept(plain)
			if err != nil {
				t.Fatal(err)
			}

			activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(accept.ESMMessageContainer)
			if err != nil {
				t.Fatal(err)
			}

			pdn, err := eps.ParsePDNAddress(activate.PDNAddress)
			if err != nil {
				t.Fatal(err)
			}

			if pdn.PDNType != tc.pdnType {
				t.Fatalf("PDN type = %d, want %d", pdn.PDNType, tc.pdnType)
			}

			if tc.wantV4 && pdn.IPv4 != testUEIP.As4() {
				t.Fatalf("IPv4 = %v, want %v", pdn.IPv4, testUEIP.As4())
			}

			if tc.wantV6 && pdn.IPv6IID != testUEIPv6IID {
				t.Fatalf("IPv6 IID = %x, want %x", pdn.IPv6IID, testUEIPv6IID)
			}
		})
	}
}
