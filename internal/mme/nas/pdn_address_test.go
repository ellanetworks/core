// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

var testSnssai = models.Snssai{Sst: 1, Sd: "102030"}

func wantSnssaiContainer(t *testing.T) nas.PCOContainer {
	t.Helper()

	c, err := nas.NewSNSSAIContainer([]byte{0x01, 0x10, 0x20, 0x30}, nas.PLMN{MCC: "001", MNC: "01"})
	if err != nil {
		t.Fatal(err)
	}

	return c
}

// activateFromAccept unprotects an Attach Accept and decodes the embedded
// Activate Default EPS Bearer Context Request.
func activateFromAccept(t *testing.T, m *mme.MME, ue *mme.UeContext) *eps.ActivateDefaultEPSBearerContextRequest {
	t.Helper()

	wire, err := buildProtectedAttachAccept(context.Background(), m, ue, &mme.EpsQoS{APN: "internet", QCI: 9, MTU: 1400, Snssai: testSnssai})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
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
	testPDN(ue).PdnType = eps.PDNTypeIPv4
	testPDN(ue).UeIP = testUEIP

	wire, err := buildProtectedAttachAccept(context.Background(), m, ue, &mme.EpsQoS{APN: "internet", QCI: 9, MTU: 1400, Snssai: testSnssai})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatal(err)
	}

	accept, err := eps.ParseAttachAccept(plain)
	if err != nil {
		t.Fatal(err)
	}

	if accept.NetworkFeatureSupport == nil || !accept.NetworkFeatureSupport.IMSVoPS {
		t.Fatalf("Attach Accept must advertise IMS VoPS, got %+v", accept.NetworkFeatureSupport)
	}
}

// TestAttachAcceptDNSPCO checks an IPv4 bearer's PCO advertises both the DNS
// server (0x000D) and the IPv4 Link MTU (0x0010), TS 24.008 §10.5.6.3.
func TestAttachAcceptDNSPCO(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).PdnType = eps.PDNTypeIPv4
	testPDN(ue).UeIP = testUEIP
	testPDN(ue).Dns = netip.MustParseAddr("8.8.8.8")

	activate := activateFromAccept(t, m, ue)

	want := nas.NewProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)
	want.Containers = append(want.Containers, wantSnssaiContainer(t))

	if !reflect.DeepEqual(activate.ProtocolConfigurationOptions, &want) {
		t.Fatalf("PCO = %+v, want %+v", activate.ProtocolConfigurationOptions, want)
	}
}

// TestAttachAcceptIPv6NoLinkMTU checks an IPv6-only bearer's PCO carries the
// IPv6 DNS but no IPv4 Link MTU (there is no IPv6 PCO MTU container; the IPv6
// link MTU is delivered via the Router Advertisement).
func TestAttachAcceptIPv6NoLinkMTU(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).PdnType = eps.PDNTypeIPv6
	testPDN(ue).UeIPv6IID = testUEIPv6IID
	testPDN(ue).Dns = netip.MustParseAddr("2001:4860:4860::8888")

	activate := activateFromAccept(t, m, ue)

	dns := netip.MustParseAddr("2001:4860:4860::8888").As16()
	want := nas.NewProtocolConfigurationOptions([][]byte{dns[:]}, 0)
	want.Containers = append(want.Containers, wantSnssaiContainer(t))

	if !reflect.DeepEqual(activate.ProtocolConfigurationOptions, &want) {
		t.Fatalf("PCO = %+v, want %+v (IPv6 DNS, no IPv4 Link MTU)", activate.ProtocolConfigurationOptions, want)
	}
}

// TestAttachAcceptDowngradeCause checks an IPv4v6→IPv4 downgrade carries ESM
// cause #50 in the Activate Default (TS 24.301 §6.5.1.3).
func TestAttachAcceptDowngradeCause(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).PdnType = eps.PDNTypeIPv4
	testPDN(ue).UeIP = testUEIP
	testPDN(ue).EsmCause = eps.ESMCausePDNTypeIPv4OnlyAllowed

	activate := activateFromAccept(t, m, ue)

	if activate.Cause == nil || *activate.Cause != eps.ESMCausePDNTypeIPv4OnlyAllowed {
		t.Fatalf("ESM cause = %v, want %d", activate.Cause, eps.ESMCausePDNTypeIPv4OnlyAllowed)
	}
}

// TestActivateDefaultBearerRejectsWhen4GNotAllowed checks that a subscriber on a
// profile that forbids 4G is rejected with EMM cause #7 "EPS services not
// allowed" (Core Network type restriction, TS 23.501 §5.3.4 / TS 24.301 §9.9.3.9).
func TestActivateDefaultBearerRejectsWhen4GNotAllowed(t *testing.T) {
	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), barredBearerStore{}, &fakeSessionManager{})
	ue, cc := securedUE(t, m)

	activateDefaultBearer(context.Background(), m, ue)

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + UE Context Release Command, got %d", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseEPSServicesNotAllowed {
		t.Fatalf("Attach Reject cause = %d, want %d (EPS services not allowed)", rej.Cause, eps.EMMCauseEPSServicesNotAllowed)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TestActivateDefaultBearerRejectsOnSessionFailure checks that when the anchor
// cannot establish the default bearer, the attach is rejected with EMM cause
// #19 "ESM failure" and the S1 context is released (TS 24.301 §5.5.1.2.5).
func TestActivateDefaultBearerRejectsOnSessionFailure(t *testing.T) {
	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, &erroringSessionManager{})
	ue, cc := securedUE(t, m)

	activateDefaultBearer(context.Background(), m, ue)

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + UE Context Release Command, got %d", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseESMFailure {
		t.Fatalf("Attach Reject cause = %d, want %d (ESM failure)", rej.Cause, eps.EMMCauseESMFailure)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TestAttachAcceptPDNAddress checks the Attach Accept encodes the PDN Address per
// the negotiated PDN type (TS 24.301 §9.9.4.9): IPv4 carries the address, IPv6
// the SLAAC interface identifier, IPv4v6 both.
func TestAttachAcceptPDNAddress(t *testing.T) {
	cases := []struct {
		name    string
		pdnType eps.PDNType
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
			testPDN(ue).PdnType = tc.pdnType
			testPDN(ue).UeIP = testUEIP
			testPDN(ue).UeIPv6IID = testUEIPv6IID

			wire, err := buildProtectedAttachAccept(context.Background(), m, ue, &mme.EpsQoS{APN: "internet", QCI: 9})
			if err != nil {
				t.Fatal(err)
			}

			plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
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

			pdn := activate.PDNAddress
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

func TestAttachAcceptIWKN26FollowsN1ModeSupport(t *testing.T) {
	// N1 mode is octet 9 bit 6 of the UE network capability (TS 24.301
	// §9.9.3.34); Rest starts at octet 7.
	withN1 := eps.UENetworkCapability{EEA: 0xF0, EIA: 0x70, Rest: []byte{0x00, 0x00, 0x20}}
	withoutN1 := eps.UENetworkCapability{EEA: 0xF0, EIA: 0x70, Rest: []byte{0x00, 0x00, 0x00}}

	for _, tc := range []struct {
		name  string
		uecap eps.UENetworkCapability
		want  bool
	}{
		{"UE supports N1 mode", withN1, true},
		{"UE does not support N1 mode", withoutN1, false},
		{"UE sent no feature octets", eps.UENetworkCapability{EEA: 0xF0, EIA: 0x70}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, _ := securedUE(t, m)
			ue.SetUESecurityCapability(tc.uecap, nil, mme.MintAuthProofForAttachRequest())
			testPDN(ue).PdnType = eps.PDNTypeIPv4
			testPDN(ue).UeIP = testUEIP

			wire, err := buildProtectedAttachAccept(context.Background(), m, ue, &mme.EpsQoS{APN: "internet", QCI: 9, MTU: 1400, Snssai: testSnssai})
			if err != nil {
				t.Fatal(err)
			}

			plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
			if err != nil {
				t.Fatal(err)
			}

			accept, err := eps.ParseAttachAccept(plain)
			if err != nil {
				t.Fatal(err)
			}

			if accept.NetworkFeatureSupport == nil {
				t.Fatal("Attach Accept carries no EPS network feature support")
			}

			if accept.NetworkFeatureSupport.IWKN26 != tc.want {
				t.Errorf("IWK N26 = %v, want %v", accept.NetworkFeatureSupport.IWKN26, tc.want)
			}
		})
	}
}
