// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

const (
	epsIPv4Pool = "10.45.0.0/22"
	epsIPv6Pool = "2001:db8::/48"
)

func epsTestSMF() (*fakeStore, *fakeUPF) {
	store := &fakeStore{
		allocatedIP:   netip.AddrFrom4([4]byte{10, 45, 0, 7}),
		allocatedIPv6: netip.MustParseAddr("2001:db8:2::"),
		releasedIP:    netip.AddrFrom4([4]byte{10, 45, 0, 7}),
	}
	upf := &fakeUPF{establishResult: &models.EstablishResponse{
		RemoteSEID:  0x99,
		CreatedPDRs: []models.CreatedPDR{{PDRID: 1, TEID: 0xABCD, N3IPv4: netip.AddrFrom4([4]byte{10, 3, 0, 2})}},
	}}

	return store, upf
}

// epsTestEBI is the default bearer's EPS bearer identity the EPS session tests
// key sessions by (TS 24.301).
const epsTestEBI uint8 = 5

func epsRequest(pdnType uint8) models.EPSBearerRequest {
	return models.EPSBearerRequest{
		IMSI:              "001010000000001",
		EPSBearerIdentity: epsTestEBI,
		APN:               "internet",
		AMBRUplink:        "1 Gbps",
		AMBRDownlink:      "1 Gbps",
		IPv4Pool:          epsIPv4Pool,
		IPv6Pool:          epsIPv6Pool,
		MTU:               1400,
		RequestedPDNType:  pdnType,
	}
}

func TestCreateEPSSessionIPv4(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	if bearer.PDNType != 1 || bearer.IPv4 != store.allocatedIP {
		t.Fatalf("bearer = %+v, want IPv4 %v", bearer, store.allocatedIP)
	}

	want := models.FTEID{TEID: 0xABCD, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 2})}
	if bearer.SGW != want {
		t.Fatalf("S-GW F-TEID = %+v, want %+v", bearer.SGW, want)
	}

	if upf.lastEstablish == nil || len(upf.lastEstablish.PDRs) < 2 {
		t.Fatalf("expected an establish with >=2 PDRs, got %+v", upf.lastEstablish)
	}
}

// TestCreateEPSSessionBindsPolicyID checks the EPS session carries the policy ID
// into the UPF establish request, so the data path binds the session to the
// policy's network rules (SDF filters), as the 5G path does.
func TestCreateEPSSessionBindsPolicyID(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	req := epsRequest(1)
	req.PolicyID = "policy-uuid-123"

	if _, err := s.CreateEPSSession(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	if upf.lastEstablish == nil {
		t.Fatal("no UPF establish request captured")
	}

	if upf.lastEstablish.PolicyID != "policy-uuid-123" {
		t.Fatalf("UPF establish PolicyID = %q, want %q", upf.lastEstablish.PolicyID, "policy-uuid-123")
	}
}

func TestCreateEPSSessionIPv6(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(2))
	if err != nil {
		t.Fatal(err)
	}

	if bearer.PDNType != 2 {
		t.Fatalf("PDN type = %d, want 2 (IPv6)", bearer.PDNType)
	}

	if bearer.IPv6Prefix != store.allocatedIPv6 {
		t.Fatalf("IPv6 prefix = %v, want %v", bearer.IPv6Prefix, store.allocatedIPv6)
	}

	if bearer.IPv6IID == [8]byte{} {
		t.Fatal("IPv6 IID not assigned")
	}

	if bearer.IPv4.IsValid() {
		t.Fatalf("IPv6-only bearer has an IPv4 address: %v", bearer.IPv4)
	}
}

func TestCreateEPSSessionIPv4v6(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(3))
	if err != nil {
		t.Fatal(err)
	}

	if bearer.PDNType != 3 || bearer.IPv4 != store.allocatedIP || bearer.IPv6Prefix != store.allocatedIPv6 {
		t.Fatalf("dual-stack bearer = %+v", bearer)
	}

	// Dual-stack adds the second (IPv6) downlink PDR: uplink + downlink-v4 + v6.
	if upf.lastEstablish == nil || len(upf.lastEstablish.PDRs) < 3 {
		t.Fatalf("expected a dual-stack establish with >=3 PDRs, got %d", len(upf.lastEstablish.PDRs))
	}
}

// TestCreateEPSSessionSGWN3Family checks the S-GW S1-U endpoint advertised to the
// eNB carries whichever N3 family the UPF reports — IPv4, IPv6, or both — so a 4G
// bearer can run its user plane over IPv6 the same way 5G N3 does.
func TestCreateEPSSessionSGWN3Family(t *testing.T) {
	v4 := netip.AddrFrom4([4]byte{10, 3, 0, 2})
	v6 := netip.MustParseAddr("2001:db8:3::10")

	tests := []struct {
		name   string
		n3v4   netip.Addr
		n3v6   netip.Addr
		wantV4 netip.Addr
		wantV6 netip.Addr
	}{
		{name: "ipv4 n3", n3v4: v4, wantV4: v4},
		{name: "ipv6 n3", n3v6: v6, wantV6: v6},
		{name: "dual n3", n3v4: v4, n3v6: v6, wantV4: v4, wantV6: v6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := epsTestSMF()
			upf := &fakeUPF{establishResult: &models.EstablishResponse{
				RemoteSEID:  0x99,
				CreatedPDRs: []models.CreatedPDR{{PDRID: 1, TEID: 0xABCD, N3IPv4: tc.n3v4, N3IPv6: tc.n3v6}},
			}}
			s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

			bearer, err := s.CreateEPSSession(context.Background(), epsRequest(3))
			if err != nil {
				t.Fatal(err)
			}

			if bearer.SGW.Addr != tc.wantV4 {
				t.Fatalf("S-GW IPv4 N3 = %v, want %v", bearer.SGW.Addr, tc.wantV4)
			}

			if bearer.SGWN3IPv6 != tc.wantV6 {
				t.Fatalf("S-GW IPv6 N3 = %v, want %v", bearer.SGWN3IPv6, tc.wantV6)
			}
		})
	}
}

// TestCreateEPSSessionUPFFailureReleasesTunnel verifies that when the UPF
// establish fails mid-create, the abort path releases the tunnel — freeing the
// PDR/FAR/QER/URR IDs that ActivateTunnelAndPDR allocated before establish, even
// though RemoteSEID is still 0 (regression for the leaked-rule-ID bug, F2).
// releaseTunnel running is observable via the UPF DeleteSession call it issues.
func TestCreateEPSSessionUPFFailureReleasesTunnel(t *testing.T) {
	store, upf := epsTestSMF()
	upf.err = errors.New("upf establish failed")
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err == nil {
		t.Fatal("expected create to fail when UPF establish fails")
	}

	if len(upf.deleteCalls) == 0 {
		t.Fatal("aborted EPS session did not release the tunnel; the PDR/FAR/QER/URR IDs would leak")
	}
}

func TestCreateEPSSessionNoIPv6Pool(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	req := epsRequest(2)
	req.IPv6Pool = "" // data network offers no IPv6

	if _, err := s.CreateEPSSession(context.Background(), req); err == nil {
		t.Fatal("expected IPv6 request to fail with no IPv6 pool")
	}
}

// TestCreateEPSSessionDowngradeCause checks an IPv4v6 request on a single-stack
// data network is assigned the available family with the matching ESM cause
// (#50/#51, TS 24.301 §6.5.1.3).
func TestCreateEPSSessionDowngradeCause(t *testing.T) {
	for _, tc := range []struct {
		name      string
		dropPool  func(*models.EPSBearerRequest)
		wantType  uint8
		wantCause uint8
	}{
		{"IPv6-only DN", func(r *models.EPSBearerRequest) { r.IPv4Pool = "" }, 2, 51},
		{"IPv4-only DN", func(r *models.EPSBearerRequest) { r.IPv6Pool = "" }, 1, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, upf := epsTestSMF()
			s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

			req := epsRequest(3) // IPv4v6
			tc.dropPool(&req)

			bearer, err := s.CreateEPSSession(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			if bearer.PDNType != tc.wantType || bearer.ESMCause != tc.wantCause {
				t.Fatalf("bearer type=%d cause=%d, want type=%d cause=%d", bearer.PDNType, bearer.ESMCause, tc.wantType, tc.wantCause)
			}
		})
	}
}

// TestCreateEPSSessionDNS checks the configured data-network DNS is returned on
// the bearer for the MME to advertise via PCO.
func TestCreateEPSSessionDNS(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	req := epsRequest(1)
	req.DNS = "8.8.4.4"

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if bearer.DNS != netip.MustParseAddr("8.8.4.4") {
		t.Fatalf("bearer DNS = %v, want 8.8.4.4", bearer.DNS)
	}
}

func TestModifyEPSSessionRegistersIPv6(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(3)); err != nil {
		t.Fatal(err)
	}

	enb := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	if err := s.ModifyEPSSession(context.Background(), "001010000000001", epsTestEBI, enb); err != nil {
		t.Fatal(err)
	}

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 ModifySession, got %d", len(upf.modifyCalls))
	}

	// The downlink FAR encaps toward the eNB, PSC-less.
	var ohc *models.OuterHeaderCreation

	for _, far := range upf.modifyCalls[0].UpdateFARs {
		if far.ForwardingParameters != nil && far.ForwardingParameters.OuterHeaderCreation != nil {
			ohc = far.ForwardingParameters.OuterHeaderCreation
		}
	}

	if ohc == nil || ohc.TEID != 0x55 || !ohc.S1U {
		t.Fatalf("downlink OuterHeaderCreation = %+v, want TEID 0x55 with S1U", ohc)
	}

	// The IPv6 session is registered so the RA responder can answer the UE's RS.
	if upf.lastIPv6Reg == nil {
		t.Fatal("IPv6 session not registered with the UPF after modify")
	}

	if upf.lastIPv6Reg.Prefix.Addr() != store.allocatedIPv6 || upf.lastIPv6Reg.Prefix.Bits() != 64 {
		t.Fatalf("registered IPv6 prefix = %v, want %v/64", upf.lastIPv6Reg.Prefix, store.allocatedIPv6)
	}

	// The RA must ride the S1-U bearer PSC-less, matching the data path.
	if !upf.lastIPv6Reg.S1U {
		t.Fatal("IPv6 session registered without the S1-U (PSC-less) marking")
	}
}

func TestReleaseEPSSession(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	if _, err := s.CreateEPSSession(context.Background(), epsRequest(1)); err != nil {
		t.Fatal(err)
	}

	if err := s.ReleaseEPSSession(context.Background(), "001010000000001", epsTestEBI); err != nil {
		t.Fatal(err)
	}

	if len(upf.deleteCalls) != 1 || upf.deleteCalls[0].remoteSEID != 0x99 {
		t.Fatalf("expected 1 DeleteSession for SEID 0x99, got %+v", upf.deleteCalls)
	}

	if len(store.releasedIPs) != 1 {
		t.Fatalf("expected the UE IP to be released, got %+v", store.releasedIPs)
	}
}
