// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

// buildPDUSessionEstRequestWithType builds a NAS PDU Session Establishment
// Request with the specified PDU session type (IPv4, IPv6, or IPv4v6).
func buildPDUSessionEstRequestWithType(pduSessionType fgs.PDUSessionType) []byte {
	req := &fgs.PDUSessionEstablishmentRequest{
		PDUSessionID:             1,
		PTI:                      10,
		IntegrityProtMaxDataRate: [2]byte{0xff, 0xff},
		PDUSessionType:           &pduSessionType,
	}

	buf, err := req.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request: %v", err))
	}

	return buf
}

// ipv6Fakes returns fakes configured for IPv6-only session tests.
func ipv6Fakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF) {
	pcf := &fakePCF{
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
			QosData: models.QosData{
				Var5qi: 9,
				Arp:    &models.Arp{PriorityLevel: 1},
				QFI:    1,
			},
			DNS:      net.ParseIP("2001:4860:4860::8888"),
			MTU:      1500,
			IPv4Pool: "",
			IPv6Pool: "2001:db8::/48",
		},
	}
	store := &fakeStore{
		allocatedIPv6: netip.MustParseAddr("2001:db8::"),
		releasedIPv6:  netip.MustParseAddr("2001:db8::"),
	}
	upf := &fakeUPF{
		establishResult: &models.EstablishResponse{
			N3TEID: 5000,
			N3IPv4: netip.MustParseAddr("192.168.1.1"),
		},
	}
	amfCb := &fakeAMF{}

	return pcf, store, upf, amfCb
}

func buildPDUSessionResourceSetupResponseTransferIPv6(teid uint32, ip net.IP) ([]byte, error) {
	transfer := libngap.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: libngap.QosFlowPerTNLInformation{
			UPTransportLayerInformation: libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
				TransportLayerAddress: libngap.TransportLayerAddress(ip.To16()),
				GTPTEID:               libngap.GTPTEID(teid),
			}},
			AssociatedQosFlowList: libngap.AssociatedQosFlowList{{QosFlowIdentifier: 1}},
		},
	}

	b, err := transfer.Marshal()

	return b, err
}

// dualStackFakes returns fakes configured for IPv4v6 dual-stack session tests.
func dualStackFakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF) {
	pcf := &fakePCF{
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
			QosData: models.QosData{
				Var5qi: 9,
				Arp:    &models.Arp{PriorityLevel: 1},
				QFI:    1,
			},
			DNS:      net.ParseIP("2001:4860:4860::8888"),
			MTU:      1500,
			IPv4Pool: "10.0.0.0/24",
			IPv6Pool: "2001:db8::/48",
		},
	}
	store := &fakeStore{
		allocatedIP:   netip.MustParseAddr("10.0.0.1"),
		allocatedIPv6: netip.MustParseAddr("2001:db8:abcd::"),
		releasedIP:    netip.MustParseAddr("10.0.0.1"),
		releasedIPv6:  netip.MustParseAddr("2001:db8:abcd::"),
	}
	upf := &fakeUPF{
		establishResult: &models.EstablishResponse{
			N3TEID: 6000,
			N3IPv4: netip.MustParseAddr("192.168.1.1"),
		},
	}
	amfCb := &fakeAMF{}

	return pcf, store, upf, amfCb
}

// ===========================
// IPv6-Only PDU Session
// ===========================

func TestCreateSmContext_IPv6Only_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err != nil {
		t.Fatalf("CreateSmContext (IPv6) failed: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("expected no reject, got %d bytes", len(rejectN1))
	}

	if ref == "" {
		t.Fatal("expected non-empty context ref")
	}

	smCtx := s.GetSession(ref)
	if smCtx == nil {
		t.Fatal("session should be in pool")
	}

	if fgs.PDUSessionType(smCtx.PDUSessionType) != fgs.PDUSessionTypeIPv6 {
		t.Fatalf("expected PDU session type IPv6 (%d), got %d", fgs.PDUSessionTypeIPv6, smCtx.PDUSessionType)
	}

	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set")
	}

	expectedPrefix := net.ParseIP("2001:db8::")
	if !smCtx.PDUIPV6Prefix.Equal(expectedPrefix) {
		t.Fatalf("expected PDUAddressIPv6 %s, got %s", expectedPrefix, smCtx.PDUIPV6Prefix)
	}

	var zeroIID [8]byte
	if smCtx.IPv6IID == zeroIID {
		t.Fatal("expected non-zero IPv6 IID")
	}

	if smCtx.PDUIPV4Address != nil {
		t.Fatalf("expected nil PDUAddress for IPv6-only, got %s", smCtx.PDUIPV4Address)
	}

	upf.mu.Lock()
	if upf.lastEstablish == nil {
		upf.mu.Unlock()
		t.Fatal("expected PFCP establishment call")
	}

	if upf.lastEstablish.IMSI != testIMSI {
		upf.mu.Unlock()
		t.Fatalf("expected IMSI %s in establish request, got %s", testIMSI, upf.lastEstablish.IMSI)
	}
	upf.mu.Unlock()

	amfCb.mu.Lock()
	if len(amfCb.n1n2Calls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 N1N2 transfer call, got %d", len(amfCb.n1n2Calls))
	}
	amfCb.mu.Unlock()
}

func TestCreateSmContext_IPv6Only_AllocationFailure(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	store.allocatedIPv6 = netip.Addr{}
	store.allocateIPv6Err = fmt.Errorf("IPv6 pool exhausted")
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv6)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err == nil {
		t.Fatal("expected error when IPv6 pool exhausted")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); fgs.GSMCause(got) != fgs.GSMCauseInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", fgs.GSMCauseInsufficientResources, got)
	}
}

// ===========================
// Dual-Stack (IPv4v6) PDU Session
// ===========================

func TestCreateSmContext_DualStack_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv4v6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err != nil {
		t.Fatalf("CreateSmContext (IPv4v6) failed: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("expected no reject, got %d bytes", len(rejectN1))
	}

	smCtx := s.GetSession(ref)
	if smCtx == nil {
		t.Fatal("session should be in pool")
	}

	if fgs.PDUSessionType(smCtx.PDUSessionType) != fgs.PDUSessionTypeIPv4v6 {
		t.Fatalf("expected PDU session type IPv4v6 (%d), got %d", fgs.PDUSessionTypeIPv4v6, smCtx.PDUSessionType)
	}

	if smCtx.PDUIPV4Address == nil {
		t.Fatal("expected PDUAddress to be set for dual-stack")
	}

	expectedIPv4 := net.ParseIP("10.0.0.1").To4()
	if !smCtx.PDUIPV4Address.Equal(expectedIPv4) {
		t.Fatalf("expected PDUAddress %s, got %s", expectedIPv4, smCtx.PDUIPV4Address)
	}

	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set for dual-stack")
	}

	expectedIPv6 := net.ParseIP("2001:db8:abcd::")
	if !smCtx.PDUIPV6Prefix.Equal(expectedIPv6) {
		t.Fatalf("expected PDUAddressIPv6 %s, got %s", expectedIPv6, smCtx.PDUIPV6Prefix)
	}

	var zeroIID [8]byte
	if smCtx.IPv6IID == zeroIID {
		t.Fatal("expected non-zero IPv6 IID")
	}
}

func TestCreateSmContext_DownlinkPDRsPerPDUSessionType(t *testing.T) {
	tests := []struct {
		name        string
		fakes       func() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF)
		sessionType fgs.PDUSessionType
		wantPDRs    int
		wantIPv4    bool
		wantIPv6    bool
	}{
		{
			name:        "IPv4 only",
			fakes:       defaultFakes,
			sessionType: fgs.PDUSessionTypeIPv4,
			wantPDRs:    1,
			wantIPv4:    true,
		},
		{
			name:        "IPv6 only",
			fakes:       ipv6Fakes,
			sessionType: fgs.PDUSessionTypeIPv6,
			wantPDRs:    1,
			wantIPv6:    true,
		},
		{
			name:        "dual stack",
			fakes:       dualStackFakes,
			sessionType: fgs.PDUSessionTypeIPv4v6,
			wantPDRs:    2,
			wantIPv4:    true,
			wantIPv6:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcf, store, upf, amfCb := tt.fakes()
			s := newTestSMF(pcf, store, upf, amfCb)

			n1Msg := buildPDUSessionEstRequestWithType(tt.sessionType)

			_, _, err := s.CreateSmContext(context.Background(), testSUPI(), 1, testDNN, testSnssai,
				fgs.RequestTypeInitialRequest, n1Msg, 0)
			if err != nil {
				t.Fatalf("CreateSmContext: %v", err)
			}

			upf.mu.Lock()
			defer upf.mu.Unlock()

			if upf.lastEstablish == nil {
				t.Fatal("expected PFCP establishment call")
			}

			var (
				count            int
				hasIPv4, hasIPv6 bool
			)

			for _, pdr := range upf.lastEstablish.PDRs {
				if !pdr.PDI.UEIPAddress.IsValid() || pdr.PDI.LocalFTEID != nil {
					continue
				}

				count++

				addr, err := netip.ParseAddr(pdr.PDI.UEIPAddress.String())
				if err != nil {
					t.Fatalf("invalid UE IP in downlink PDR: %s", pdr.PDI.UEIPAddress)
				}

				if addr.Is4() {
					hasIPv4 = true
				} else {
					hasIPv6 = true
				}
			}

			if count != tt.wantPDRs {
				t.Fatalf("downlink PDRs = %d, want %d", count, tt.wantPDRs)
			}

			if hasIPv4 != tt.wantIPv4 {
				t.Errorf("downlink PDR with an IPv4 UE address = %v, want %v", hasIPv4, tt.wantIPv4)
			}

			if hasIPv6 != tt.wantIPv6 {
				t.Errorf("downlink PDR with an IPv6 UE address = %v, want %v", hasIPv6, tt.wantIPv6)
			}
		})
	}
}

func TestCreateSmContext_DualStack_IPv6AllocationFails_RollsBackIPv4(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	store.allocateIPv6Err = fmt.Errorf("IPv6 pool exhausted")
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv4v6)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err == nil {
		t.Fatal("expected error when IPv6 allocation fails in dual-stack")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); fgs.GSMCause(got) != fgs.GSMCauseInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", fgs.GSMCauseInsufficientResources, got)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) == 0 {
		t.Fatal("expected IPv4 address to be released (rolled back) after IPv6 allocation failure")
	}
}

func TestCreateSmContext_DualStack_IPv4AllocationFails(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	store.allocateIPErr = fmt.Errorf("IPv4 pool exhausted")
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv4v6)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err == nil {
		t.Fatal("expected error when IPv4 allocation fails in dual-stack")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); fgs.GSMCause(got) != fgs.GSMCauseInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", fgs.GSMCauseInsufficientResources, got)
	}

	// IPv6 should NOT have been allocated (IPv4 fails first).
	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPv6s) != 0 {
		t.Fatal("expected no IPv6 release since IPv6 was never allocated")
	}
}

// ===========================
// Session Release
// ===========================

// setupIPv6SessionWithTunnel creates a session with a fully populated tunnel
// for IPv6-only testing.
func setupIPv6SessionWithTunnel(t *testing.T, s *smf.SMF) (*smf.SMContext, string) {
	t.Helper()

	supi := testSUPI()
	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)

	seid := s.AllocateSEID()
	s.AssignPFCPSession(smCtx, seid)
	smCtx.PFCPContext.SEID = 100
	smCtx.PFCPContext.Established = true

	smCtx.Tunnel = &smf.UPTunnel{
		N3TEID: 5000,
		N3IPv4: netip.MustParseAddr("192.168.1.1"),
	}
	smCtx.Tunnel.UEIPv6 = netip.MustParseAddr("2001:db8::")
	smCtx.Tunnel.AN = smf.AnchorBinding{TEID: 6000, IPv4: net.ParseIP("10.0.0.100").To4()}
	smCtx.Tunnel.Downlink = smf.DownlinkForwarding
	smCtx.PDUIPV6Prefix = net.ParseIP("2001:db8::").To16()
	smCtx.PDUSessionType = uint8(fgs.PDUSessionTypeIPv6)

	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
		QosData: models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1},
	}

	return smCtx, smCtx.Ref
}

// setupDualStackSessionWithTunnel creates a session with a fully populated tunnel
// for IPv4v6 dual-stack testing.
func setupDualStackSessionWithTunnel(t *testing.T, s *smf.SMF) (*smf.SMContext, string) {
	t.Helper()

	smCtx, ref := setupIPv6SessionWithTunnel(t, s)
	smCtx.PDUIPV4Address = net.ParseIP("10.0.0.1").To4()
	smCtx.PDUSessionType = uint8(fgs.PDUSessionTypeIPv4v6)

	return smCtx, ref
}

func TestReleaseSmContext_IPv6Only(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupIPv6SessionWithTunnel(t, s)

	err := s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext (IPv6) failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	store.mu.Lock()
	if len(store.releasedIPv6s) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv6 prefix to be released")
	}
	store.mu.Unlock()

	store.mu.Lock()
	if len(store.releasedIPs) != 0 {
		store.mu.Unlock()
		t.Fatal("should not release IPv4 for IPv6-only session")
	}
	store.mu.Unlock()

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()
}

func TestReleaseSmContext_DualStack(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupDualStackSessionWithTunnel(t, s)

	err := s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext (dual-stack) failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	store.mu.Lock()
	if len(store.releasedIPs) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv4 address to be released")
	}

	if len(store.releasedIPv6s) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv6 prefix to be released")
	}
	store.mu.Unlock()

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()
}

func TestRemoveSession_IPv6Only_ReleasesIPv6(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()
	ctx := context.Background()

	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	smCtx.PDUIPV6Prefix = net.ParseIP("2001:db8::").To16()

	removeSession(s, ctx, smCtx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPv6s) != 1 || store.releasedIPv6s[0] != testIMSI {
		t.Fatalf("expected IPv6 release for %s, got %v", testIMSI, store.releasedIPv6s)
	}

	if len(store.releasedIPs) != 0 {
		t.Fatal("should not release IPv4 for IPv6-only session")
	}
}

func TestRemoveSession_DualStack_ReleasesBoth(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()
	ctx := context.Background()

	smCtx, _ := s.NewSession(supi, smf.Access5G, smf.SessionIdentity{PDUSessionID: 1}, testDNN, testSnssai)
	smCtx.PDUIPV4Address = net.ParseIP("10.0.0.1").To4()
	smCtx.PDUIPV6Prefix = net.ParseIP("2001:db8:abcd::").To16()

	removeSession(s, ctx, smCtx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 1 || store.releasedIPs[0] != testIMSI {
		t.Fatalf("expected IPv4 release for %s, got %v", testIMSI, store.releasedIPs)
	}

	if len(store.releasedIPv6s) != 1 || store.releasedIPv6s[0] != testIMSI {
		t.Fatalf("expected IPv6 release for %s, got %v", testIMSI, store.releasedIPv6s)
	}
}

// ===========================
// NAS Release via UpdateSmContextN1Msg
// ===========================

func TestUpdateSmContextN1Msg_IPv6Release(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupIPv6SessionWithTunnel(t, s)
	n1Msg := buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg (IPv6 release) failed: %v", err)
	}

	// The release is signaled to the UE/gNB via the AMF (TS 24.501 §6.3.3).
	if rsp != nil {
		t.Fatalf("expected no UpdateResult, got %+v", rsp)
	}

	amfCb.mu.Lock()
	if len(amfCb.releaseCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 ReleaseSession call, got %d", len(amfCb.releaseCalls))
	}
	amfCb.mu.Unlock()

	// The user plane is torn down on Release Complete, not on the request
	// (TS 23.502 §4.3.4).
	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseComplete(smCtx.PDUSessionID, 5)); err != nil {
		t.Fatalf("release complete: %v", err)
	}

	store.mu.Lock()
	if len(store.releasedIPv6s) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv6 prefix to be released")
	}
	store.mu.Unlock()

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()
}

func TestUpdateSmContextN1Msg_DualStackRelease(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupDualStackSessionWithTunnel(t, s)
	n1Msg := buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg (dual-stack release) failed: %v", err)
	}

	// The release is signaled to the UE/gNB via the AMF (TS 24.501 §6.3.3).
	if rsp != nil {
		t.Fatalf("expected no UpdateResult, got %+v", rsp)
	}

	amfCb.mu.Lock()
	if len(amfCb.releaseCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 ReleaseSession call, got %d", len(amfCb.releaseCalls))
	}
	amfCb.mu.Unlock()

	// The user plane is torn down on Release Complete, not on the request
	// (TS 23.502 §4.3.4).
	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseComplete(smCtx.PDUSessionID, 5)); err != nil {
		t.Fatalf("release complete: %v", err)
	}

	store.mu.Lock()
	if len(store.releasedIPs) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv4 address to be released")
	}

	if len(store.releasedIPv6s) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv6 prefix to be released")
	}
	store.mu.Unlock()

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()
}

func TestUpdateSmContextN2InfoPduResSetupRsp_IPv6RegistersIPv6GnbAddress(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1 := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1, 0)
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("expected accept path, got reject N1: %x", rejectN1)
	}

	smCtx := s.GetSession(ref)
	if smCtx == nil {
		t.Fatal("expected SM context to exist")
	}

	gnbIPv6 := net.ParseIP("2001:db8::200").To16()
	gnbTEID := uint32(7001)

	n2Data, err := buildPDUSessionResourceSetupResponseTransferIPv6(gnbTEID, gnbIPv6)
	if err != nil {
		t.Fatalf("build IPv6 N2 payload: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, smCtx.Ref, n2Data); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastIPv6Reg == nil {
		t.Fatal("expected IPv6 session registration")
	}

	if got, want := upf.lastIPv6Reg.GnbN3Addr, netip.MustParseAddr("2001:db8::200"); got != want {
		t.Fatalf("registered gNB N3 address = %s, want %s", got, want)
	}
}

// ===========================
// CreateSmContext → ReleaseSmContext full round-trip
// ===========================

func TestIPv6Session_CreateAndRelease_RoundTrip(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("unexpected reject: %d bytes", len(rejectN1))
	}

	smCtx := s.GetSession(ref)
	if smCtx == nil {
		t.Fatal("session not found after create")
	}

	if fgs.PDUSessionType(smCtx.PDUSessionType) != fgs.PDUSessionTypeIPv6 {
		t.Fatalf("expected IPv6 session type, got %d", smCtx.PDUSessionType)
	}

	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set")
	}

	if smCtx.PDUIPV4Address != nil {
		t.Fatal("expected no IPv4 address for IPv6-only")
	}

	err = s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	if s.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions, got %d", s.SessionCount())
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPv6s) == 0 {
		t.Fatal("expected IPv6 prefix to be released in store")
	}
}

func TestDualStackSession_CreateAndRelease_RoundTrip(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(fgs.PDUSessionTypeIPv4v6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("unexpected reject: %d bytes", len(rejectN1))
	}

	smCtx := s.GetSession(ref)
	if smCtx == nil {
		t.Fatal("session not found after create")
	}

	if fgs.PDUSessionType(smCtx.PDUSessionType) != fgs.PDUSessionTypeIPv4v6 {
		t.Fatalf("expected IPv4v6 session type, got %d", smCtx.PDUSessionType)
	}

	if smCtx.PDUIPV4Address == nil {
		t.Fatal("expected PDUAddress to be set for dual-stack")
	}

	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set for dual-stack")
	}

	err = s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) == 0 {
		t.Fatal("expected IPv4 address to be released")
	}

	if len(store.releasedIPv6s) == 0 {
		t.Fatal("expected IPv6 prefix to be released")
	}
}

// ===========================
// Unsupported PDU Session Type
// ===========================

func TestCreateSmContext_UnsupportedPDUSessionType(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	// PDU session type 4 is "Unstructured" which is not supported.
	n1Msg := buildPDUSessionEstRequestWithType(4)

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1Msg, 0)
	if err == nil {
		t.Fatal("expected error for unsupported PDU session type")
	}
}
