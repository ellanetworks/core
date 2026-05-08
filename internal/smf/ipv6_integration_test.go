// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
)

// buildPDUSessionEstRequestWithType builds a NAS PDU Session Establishment
// Request with the specified PDU session type (IPv4, IPv6, or IPv4v6).
func buildPDUSessionEstRequestWithType(pduSessionType uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentRequest)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentRequest = nasMessage.NewPDUSessionEstablishmentRequest(0)
	m.PDUSessionEstablishmentRequest.SetMessageType(nas.MsgTypePDUSessionEstablishmentRequest)
	m.PDUSessionEstablishmentRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentRequest.SetPDUSessionID(1)
	m.PDUSessionEstablishmentRequest.SetPTI(10)
	m.PDUSessionEstablishmentRequest.IntegrityProtectionMaximumDataRate. //nolint:staticcheck // full path needed to avoid ambiguous selector
										SetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForUpLink(0xff)
	m.PDUSessionEstablishmentRequest.IntegrityProtectionMaximumDataRate. //nolint:staticcheck // full path needed to avoid ambiguous selector
										SetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForDownLink(0xff)
	m.PDUSessionEstablishmentRequest.PDUSessionType = nasType.NewPDUSessionType( //nolint:staticcheck // full path needed to avoid ambiguous selector
		nasMessage.PDUSessionEstablishmentRequestPDUSessionTypeType)
	m.PDUSessionEstablishmentRequest.PDUSessionType.SetPDUSessionTypeValue(pduSessionType) //nolint:staticcheck // full path needed to avoid ambiguous selector

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request: %v", err))
	}

	return buf
}

// ipv6Fakes returns fakes configured for IPv6-only session tests.
func ipv6Fakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF) {
	pcf := &fakePCF{
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
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
			RemoteSEID: 100,
			CreatedPDRs: []models.CreatedPDR{
				{PDRID: 1, TEID: 5000, N3IPv4: netip.MustParseAddr("192.168.1.1")},
			},
		},
	}
	amfCb := &fakeAMF{}

	return pcf, store, upf, amfCb
}

func buildPDUSessionResourceSetupResponseTransferIPv6(teid uint32, ip net.IP) ([]byte, error) {
	transfer := ngapType.PDUSessionResourceSetupResponseTransfer{}

	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel = &ngapType.GTPTunnel{}

	teidBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(teidBytes, teid)
	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.GTPTEID.Value = teidBytes
	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ip.To16(),
		BitLength: 128,
	}

	transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List = append(
		transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List,
		ngapType.AssociatedQosFlowItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
		},
	)

	return aper.MarshalWithParams(transfer, "valueExt")
}

// dualStackFakes returns fakes configured for IPv4v6 dual-stack session tests.
func dualStackFakes() (*fakePCF, *fakeStore, *fakeUPF, *fakeAMF) {
	pcf := &fakePCF{
		policy: &smf.Policy{
			Ambr: models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
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
			RemoteSEID: 200,
			CreatedPDRs: []models.CreatedPDR{
				{PDRID: 1, TEID: 6000, N3IPv4: netip.MustParseAddr("192.168.1.1")},
			},
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

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
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

	// Verify IPv6 session type.
	if smCtx.PDUSessionType != nasMessage.PDUSessionTypeIPv6 {
		t.Fatalf("expected PDU session type IPv6 (%d), got %d", nasMessage.PDUSessionTypeIPv6, smCtx.PDUSessionType)
	}

	// Verify IPv6 prefix was allocated.
	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set")
	}

	expectedPrefix := net.ParseIP("2001:db8::")
	if !smCtx.PDUIPV6Prefix.Equal(expectedPrefix) {
		t.Fatalf("expected PDUAddressIPv6 %s, got %s", expectedPrefix, smCtx.PDUIPV6Prefix)
	}

	// Verify IID was generated (non-zero).
	var zeroIID [8]byte
	if smCtx.IPv6IID == zeroIID {
		t.Fatal("expected non-zero IPv6 IID")
	}

	// Verify no IPv4 address was allocated.
	if smCtx.PDUIPV4Address != nil {
		t.Fatalf("expected nil PDUAddress for IPv6-only, got %s", smCtx.PDUIPV4Address)
	}

	// Verify PFCP establishment was called.
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

	// Verify N1N2 transfer was called (session accept).
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

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv6)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when IPv6 pool exhausted")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", nasMessage.Cause5GSMInsufficientResources, got)
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

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv4IPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
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

	// Verify dual-stack session type.
	if smCtx.PDUSessionType != nasMessage.PDUSessionTypeIPv4IPv6 {
		t.Fatalf("expected PDU session type IPv4v6 (%d), got %d", nasMessage.PDUSessionTypeIPv4IPv6, smCtx.PDUSessionType)
	}

	// Verify IPv4 address was allocated.
	if smCtx.PDUIPV4Address == nil {
		t.Fatal("expected PDUAddress to be set for dual-stack")
	}

	expectedIPv4 := net.ParseIP("10.0.0.1").To4()
	if !smCtx.PDUIPV4Address.Equal(expectedIPv4) {
		t.Fatalf("expected PDUAddress %s, got %s", expectedIPv4, smCtx.PDUIPV4Address)
	}

	// Verify IPv6 prefix was allocated.
	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set for dual-stack")
	}

	expectedIPv6 := net.ParseIP("2001:db8:abcd::")
	if !smCtx.PDUIPV6Prefix.Equal(expectedIPv6) {
		t.Fatalf("expected PDUAddressIPv6 %s, got %s", expectedIPv6, smCtx.PDUIPV6Prefix)
	}

	// Verify IID was generated (non-zero).
	var zeroIID [8]byte
	if smCtx.IPv6IID == zeroIID {
		t.Fatal("expected non-zero IPv6 IID")
	}
}

func TestCreateSmContext_DualStack_SendsTwoDownlinkPDRs(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv4IPv6)

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err != nil {
		t.Fatalf("CreateSmContext (IPv4v6) failed: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish == nil {
		t.Fatal("expected PFCP establishment call")
	}

	req := upf.lastEstablish

	var (
		downlinkPDRCount int
		downlinkUEIPs    []string
	)

	for _, pdr := range req.PDRs {
		if pdr.PDI.UEIPAddress.IsValid() && pdr.PDI.LocalFTEID == nil {
			downlinkPDRCount++

			downlinkUEIPs = append(downlinkUEIPs, pdr.PDI.UEIPAddress.String())
		}
	}

	if downlinkPDRCount != 2 {
		t.Fatalf("expected 2 downlink PDRs for dual-stack, got %d", downlinkPDRCount)
	}

	hasIPv4 := false
	hasIPv6 := false

	for _, ip := range downlinkUEIPs {
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			t.Fatalf("invalid UE IP in downlink PDR: %s", ip)
		}

		if addr.Is4() {
			hasIPv4 = true
		} else {
			hasIPv6 = true
		}
	}

	if !hasIPv4 {
		t.Error("expected downlink PDR with IPv4 UE address")
	}

	if !hasIPv6 {
		t.Error("expected downlink PDR with IPv6 UE address")
	}
}

func TestCreateSmContext_IPv4Only_SendsOneDownlinkPDR(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv4)

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err != nil {
		t.Fatalf("CreateSmContext (IPv4) failed: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish == nil {
		t.Fatal("expected PFCP establishment call")
	}

	req := upf.lastEstablish

	var downlinkPDRCount int

	for _, pdr := range req.PDRs {
		if pdr.PDI.UEIPAddress.IsValid() && pdr.PDI.LocalFTEID == nil {
			downlinkPDRCount++
		}
	}

	if downlinkPDRCount != 1 {
		t.Fatalf("expected 1 downlink PDR for IPv4-only, got %d", downlinkPDRCount)
	}
}

func TestCreateSmContext_IPv6Only_SendsOneDownlinkPDR(t *testing.T) {
	pcf, store, upf, amfCb := ipv6Fakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv6)

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err != nil {
		t.Fatalf("CreateSmContext (IPv6) failed: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish == nil {
		t.Fatal("expected PFCP establishment call")
	}

	req := upf.lastEstablish

	var downlinkPDRCount int

	for _, pdr := range req.PDRs {
		if pdr.PDI.UEIPAddress.IsValid() && pdr.PDI.LocalFTEID == nil {
			downlinkPDRCount++
		}
	}

	if downlinkPDRCount != 1 {
		t.Fatalf("expected 1 downlink PDR for IPv6-only, got %d", downlinkPDRCount)
	}
}

func TestCreateSmContext_DualStack_IPv6AllocationFails_RollsBackIPv4(t *testing.T) {
	pcf, store, upf, amfCb := dualStackFakes()
	store.allocateIPv6Err = fmt.Errorf("IPv6 pool exhausted")
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv4IPv6)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when IPv6 allocation fails in dual-stack")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", nasMessage.Cause5GSMInsufficientResources, got)
	}

	// Verify IPv4 was rolled back.
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

	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv4IPv6)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when IPv4 allocation fails in dual-stack")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", nasMessage.Cause5GSMInsufficientResources, got)
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
	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)

	seid := s.AllocateLocalSEID()
	smCtx.SetPFCPSession(seid)
	smCtx.PFCPContext.RemoteSEID = 100

	ulPdr, err := s.NewPDR()
	if err != nil {
		t.Fatalf("NewPDR (UL): %v", err)
	}

	dlPdr, err := s.NewPDR()
	if err != nil {
		t.Fatalf("NewPDR (DL): %v", err)
	}

	dlPdr.FAR.ApplyAction = models.ApplyAction{Forw: true}
	dlPdr.FAR.ForwardingParameters = &models.ForwardingParameters{
		OuterHeaderCreation: &models.OuterHeaderCreation{
			Description: models.OuterHeaderCreationGtpUUdpIpv4,
			TEID:        6000,
			IPv4Address: net.ParseIP("10.0.0.100").To4(),
		},
	}

	smCtx.Tunnel = &smf.UPTunnel{
		DataPath: &smf.DataPath{
			UpLinkTunnel: &smf.GTPTunnel{
				PDR:    ulPdr,
				TEID:   5000,
				N3IPv4: netip.MustParseAddr("192.168.1.1"),
			},
			DownLinkTunnel: &smf.GTPTunnel{
				PDR: dlPdr,
			},
			Activated: true,
		},
	}
	smCtx.Tunnel.ANInformation.IPv4Address = net.ParseIP("10.0.0.100").To4()
	smCtx.Tunnel.ANInformation.TEID = 6000
	smCtx.PDUIPV6Prefix = net.ParseIP("2001:db8::").To16()
	smCtx.PDUSessionType = nasMessage.PDUSessionTypeIPv6

	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		QosData: models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1},
	}

	return smCtx, smf.CanonicalName(supi, 1)
}

// setupDualStackSessionWithTunnel creates a session with a fully populated tunnel
// for IPv4v6 dual-stack testing.
func setupDualStackSessionWithTunnel(t *testing.T, s *smf.SMF) (*smf.SMContext, string) {
	t.Helper()

	smCtx, ref := setupIPv6SessionWithTunnel(t, s)
	smCtx.PDUIPV4Address = net.ParseIP("10.0.0.1").To4()
	smCtx.PDUSessionType = nasMessage.PDUSessionTypeIPv4IPv6

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

	// Session should be removed.
	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	// Verify IPv6 was released.
	store.mu.Lock()
	if len(store.releasedIPv6s) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv6 prefix to be released")
	}
	store.mu.Unlock()

	// Verify no IPv4 release was attempted.
	store.mu.Lock()
	if len(store.releasedIPs) != 0 {
		store.mu.Unlock()
		t.Fatal("should not release IPv4 for IPv6-only session")
	}
	store.mu.Unlock()

	// Verify UPF session was deleted.
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

	// Session should be removed.
	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	// Verify both IPv4 and IPv6 were released.
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

	// Verify UPF session was deleted.
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

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.PDUIPV6Prefix = net.ParseIP("2001:db8::").To16()

	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(ctx, ref)

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

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.PDUIPV4Address = net.ParseIP("10.0.0.1").To4()
	smCtx.PDUIPV6Prefix = net.ParseIP("2001:db8:abcd::").To16()

	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(ctx, ref)

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

	if rsp == nil || rsp.N1Msg == nil {
		t.Fatal("expected N1 release command in response")
	}

	if !rsp.ReleaseN2 {
		t.Fatal("expected ReleaseN2 to be true")
	}

	// Verify IPv6 was released.
	store.mu.Lock()
	if len(store.releasedIPv6s) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IPv6 prefix to be released")
	}
	store.mu.Unlock()

	// Verify UPF session was deleted.
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

	if rsp == nil || rsp.N1Msg == nil {
		t.Fatal("expected N1 release command in response")
	}

	// Verify both IPv4 and IPv6 were released.
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

	// Verify UPF session was deleted.
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

	n1 := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1)
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

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, smCtx.CanonicalName(), n2Data); err != nil {
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

	// Create IPv6 session.
	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
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

	// Verify session state.
	if smCtx.PDUSessionType != nasMessage.PDUSessionTypeIPv6 {
		t.Fatalf("expected IPv6 session type, got %d", smCtx.PDUSessionType)
	}

	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set")
	}

	if smCtx.PDUIPV4Address != nil {
		t.Fatal("expected no IPv4 address for IPv6-only")
	}

	// Release session.
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

	// Verify store received IPv6 release.
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

	// Create dual-stack session.
	n1Msg := buildPDUSessionEstRequestWithType(nasMessage.PDUSessionTypeIPv4IPv6)

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
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

	// Verify session state.
	if smCtx.PDUSessionType != nasMessage.PDUSessionTypeIPv4IPv6 {
		t.Fatalf("expected IPv4v6 session type, got %d", smCtx.PDUSessionType)
	}

	if smCtx.PDUIPV4Address == nil {
		t.Fatal("expected PDUAddress to be set for dual-stack")
	}

	if smCtx.PDUIPV6Prefix == nil {
		t.Fatal("expected PDUAddressIPv6 to be set for dual-stack")
	}

	// Release session.
	err = s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	// Verify both addresses were released.
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

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error for unsupported PDU session type")
	}
}
