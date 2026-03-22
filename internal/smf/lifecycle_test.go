// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
)

func TestMain(m *testing.M) {
	smf.RegisterMetrics(nil)
	os.Exit(m.Run())
}

// --- NAS message helpers ---

func buildPDUSessionEstRequest() []byte {
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
	m.PDUSessionEstablishmentRequest.PDUSessionType.SetPDUSessionTypeValue(nasMessage.PDUSessionTypeIPv4) //nolint:staticcheck // full path needed to avoid ambiguous selector

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request: %v", err))
	}

	return buf
}

func buildPDUSessionReleaseRequest(pduSessionID, pti uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseRequest)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseRequest = nasMessage.NewPDUSessionReleaseRequest(0)
	m.PDUSessionReleaseRequest.SetMessageType(nas.MsgTypePDUSessionReleaseRequest)
	m.PDUSessionReleaseRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseRequest.SetPDUSessionID(pduSessionID)
	m.PDUSessionReleaseRequest.SetPTI(pti)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Release Request: %v", err))
	}

	return buf
}

// setupSessionWithTunnel creates a session with a fully populated tunnel / data path,
// simulating a session that has already been established.
func setupSessionWithTunnel(t *testing.T, s *smf.SMF) (*smf.SMContext, string) {
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

	dlPdr.FAR.ApplyAction = smf.ApplyAction{Forw: true}
	dlPdr.FAR.ForwardingParameters = &smf.ForwardingParameters{
		DestinationInterface: smf.DestinationInterface{InterfaceValue: smf.DestinationInterfaceAccess},
		NetworkInstance:      testDNN,
		OuterHeaderCreation: &smf.OuterHeaderCreation{
			OuterHeaderCreationDescription: smf.OuterHeaderCreationGtpUUdpIpv4,
			TeID:                           6000,
			IPv4Address:                    net.ParseIP("10.0.0.100").To4(),
		},
	}

	smCtx.Tunnel = &smf.UPTunnel{
		DataPath: &smf.DataPath{
			UpLinkTunnel: &smf.GTPTunnel{
				PDR:  ulPdr,
				TEID: 5000,
				N3IP: net.ParseIP("192.168.1.1").To4(),
			},
			DownLinkTunnel: &smf.GTPTunnel{
				PDR: dlPdr,
			},
			Activated: true,
		},
	}
	smCtx.Tunnel.ANInformation.IPAddress = net.ParseIP("10.0.0.100").To4()
	smCtx.Tunnel.ANInformation.TEID = 6000

	smCtx.PolicyData = &smf.Policy{
		Ambr:    models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		QosData: models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1},
	}

	return smCtx, smf.CanonicalName(supi, 1)
}

// ===========================
// DataPath tests
// ===========================

func TestActivateTunnelAndPDR_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	supi := testSUPI()

	smCtx := s.NewSession(supi, 1, testDNN, testSnssai)
	smCtx.Tunnel = &smf.UPTunnel{
		DataPath: &smf.DataPath{
			UpLinkTunnel:   &smf.GTPTunnel{},
			DownLinkTunnel: &smf.GTPTunnel{},
		},
	}

	policy := &smf.Policy{
		Ambr:    models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		QosData: models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1},
	}
	pduAddr := net.ParseIP("10.0.0.1").To4()

	err := smCtx.Tunnel.DataPath.ActivateTunnelAndPDR(s, smCtx, policy, pduAddr)
	if err != nil {
		t.Fatalf("ActivateTunnelAndPDR failed: %v", err)
	}

	if !smCtx.Tunnel.DataPath.Activated {
		t.Fatal("expected DataPath to be Activated")
	}

	if smCtx.PFCPContext == nil {
		t.Fatal("expected PFCPContext to be set")
	}

	if smCtx.Tunnel.DataPath.UpLinkTunnel.PDR == nil {
		t.Fatal("expected UL PDR to be set")
	}

	if smCtx.Tunnel.DataPath.DownLinkTunnel.PDR == nil {
		t.Fatal("expected DL PDR to be set")
	}

	if smCtx.Tunnel.DataPath.UpLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue != smf.SourceInterfaceAccess {
		t.Fatal("UL PDR source interface should be Access")
	}

	if !smCtx.Tunnel.DataPath.UpLinkTunnel.PDR.FAR.ApplyAction.Forw {
		t.Fatal("UL FAR should forward")
	}
}

func TestDeactivateTunnelAndPDR_CleansUp(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)
	smCtx := s.GetSession(ref)

	smCtx.Tunnel.DataPath.DeactivateTunnelAndPDR(s)

	if smCtx.Tunnel.DataPath.Activated {
		t.Fatal("expected DataPath to be deactivated")
	}
}

// ===========================
// ActivateSmContext tests
// ===========================

func TestActivateSmContext_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	n2Buf, err := s.ActivateSmContext(context.Background(), ref)
	if err != nil {
		t.Fatalf("ActivateSmContext failed: %v", err)
	}

	if len(n2Buf) == 0 {
		t.Fatal("expected non-empty N2 buffer")
	}
}

func TestActivateSmContext_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.ActivateSmContext(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestActivateSmContext_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.ActivateSmContext(context.Background(), "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// DeactivateSmContext tests
// ===========================

func TestDeactivateSmContext_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.DeactivateSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("DeactivateSmContext failed: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 ModifySession call, got %d", len(upf.modifyCalls))
	}

	smCtx := s.GetSession(ref)
	dlFar := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR

	if dlFar.ApplyAction.Forw {
		t.Fatal("expected Forw to be false after deactivation")
	}

	if !dlFar.ApplyAction.Buff {
		t.Fatal("expected Buff to be true after deactivation")
	}

	if !dlFar.ApplyAction.Nocp {
		t.Fatal("expected Nocp to be true after deactivation")
	}
}

func TestDeactivateSmContext_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.DeactivateSmContext(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestDeactivateSmContext_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.DeactivateSmContext(context.Background(), "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestDeactivateSmContext_NilPFCPContext(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PFCPContext = nil

	err := s.DeactivateSmContext(ctx, ref)
	if err == nil {
		t.Fatal("expected error when PFCPContext is nil")
	}
}

func TestDeactivateSmContext_ModifyError(t *testing.T) {
	store, _, amfCb := defaultFakes()
	upf := &fakeUPF{err: fmt.Errorf("PFCP modify failed")}
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.DeactivateSmContext(ctx, ref)
	if err == nil {
		t.Fatal("expected error when ModifySession fails")
	}
}

// ===========================
// ReleaseSmContext tests
// ===========================

func TestReleaseSmContext_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after release")
	}

	store.mu.Lock()
	if len(store.releasedIPs) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IP to be released")
	}
	store.mu.Unlock()

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()
}

func TestReleaseSmContext_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.ReleaseSmContext(context.Background(), "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestReleaseSmContext_NoTunnel(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	s.NewSession(supi, 1, testDNN, testSnssai)
	ref := smf.CanonicalName(supi, 1)

	err := s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext without tunnel failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed")
	}

	upf.mu.Lock()
	if len(upf.deleteCalls) != 0 {
		upf.mu.Unlock()
		t.Fatal("should not call DeleteSession when there is no tunnel")
	}
	upf.mu.Unlock()
}

func TestReleaseSmContext_DeleteSessionFails(t *testing.T) {
	store, _, amfCb := defaultFakes()
	upf := &fakeUPF{err: fmt.Errorf("PFCP delete failed")}
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReleaseSmContext(ctx, ref)
	if err == nil {
		t.Fatal("expected error when DeleteSession fails")
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed even on PFCP failure")
	}
}

// ===========================
// CreateSmContext tests
// ===========================

func TestCreateSmContext_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequest()

	ref, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
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

	upf.mu.Lock()
	if upf.lastEstablish == nil {
		upf.mu.Unlock()
		t.Fatal("expected PFCP establishment call")
	}

	if upf.lastEstablish.SUPI != testIMSI {
		upf.mu.Unlock()
		t.Fatalf("expected SUPI %s in PFCP request, got %s", testIMSI, upf.lastEstablish.SUPI)
	}
	upf.mu.Unlock()

	amfCb.mu.Lock()
	if len(amfCb.n1n2Calls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 N1N2 transfer call, got %d", len(amfCb.n1n2Calls))
	}
	amfCb.mu.Unlock()
}

func TestCreateSmContext_PolicyNotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	store.policy = nil
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequest()

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when policy not found")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}
}

func TestCreateSmContext_DNNNotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	store.dnnInfo = nil
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequest()

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when DNN not found")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}
}

func TestCreateSmContext_IPExhaustion(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	store.allocatedIP = nil
	store.err = fmt.Errorf("no IP available")
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequest()

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when IP exhausted")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}
}

func TestCreateSmContext_PFCPEstablishmentFailure(t *testing.T) {
	store, _, amfCb := defaultFakes()
	upf := &fakeUPF{err: fmt.Errorf("PFCP establishment failed")}
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	n1Msg := buildPDUSessionEstRequest()

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error when PFCP establishment fails")
	}

	amfCb.mu.Lock()
	if len(amfCb.n1Calls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 TransferN1 call (reject), got %d", len(amfCb.n1Calls))
	}
	amfCb.mu.Unlock()
}

func TestCreateSmContext_InvalidNAS(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	_, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, []byte{0x00})
	if err == nil {
		t.Fatal("expected error for invalid NAS message")
	}
}

func TestCreateSmContext_ReplacesExistingSession(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()
	n1Msg := buildPDUSessionEstRequest()

	ref1, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err != nil {
		t.Fatalf("first CreateSmContext failed: %v", err)
	}

	ref2, _, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err != nil {
		t.Fatalf("second CreateSmContext failed: %v", err)
	}

	if ref1 != ref2 {
		t.Fatalf("expected same canonical name, got %s and %s", ref1, ref2)
	}

	if s.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", s.SessionCount())
	}
}

// ===========================
// UpdateSmContextN1Msg tests
// ===========================

func TestUpdateSmContextN1Msg_ReleaseRequest(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	n1Msg := buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected non-nil response")
	}

	if rsp.N1Msg == nil {
		t.Fatal("expected N1 release command in response")
	}

	if !rsp.ReleaseN2 {
		t.Fatal("expected ReleaseN2 to be true")
	}

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()

	store.mu.Lock()
	if len(store.releasedIPs) == 0 {
		store.mu.Unlock()
		t.Fatal("expected IP to be released")
	}
	store.mu.Unlock()
}

func TestUpdateSmContextN1Msg_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextN1Msg(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN1Msg_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextN1Msg(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2InfoPduResSetupFail tests
// ===========================

func TestUpdateSmContextN2InfoPduResSetupFail_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupFail(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2InfoPduResSetupFail_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupFail(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2InfoPduResRelRsp tests
// ===========================

func TestUpdateSmContextN2InfoPduResRelRsp_NotDuplicate(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.UpdateSmContextN2InfoPduResRelRsp(ctx, ref)
	if err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed from pool after N2 release response")
	}
}

func TestUpdateSmContextN2InfoPduResRelRsp_DuplicatePDU(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PDUSessionReleaseDueToDupPduID = true

	err := s.UpdateSmContextN2InfoPduResRelRsp(ctx, ref)
	if err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("session should be removed after duplicate PDU release response")
	}
}

func TestUpdateSmContextN2InfoPduResRelRsp_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2InfoPduResRelRsp_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextCauseDuplicatePDUSessionID tests
// ===========================

func TestUpdateSmContextCauseDuplicatePDUSessionID_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	n2Rsp, err := s.UpdateSmContextCauseDuplicatePDUSessionID(ctx, ref)
	if err != nil {
		t.Fatalf("UpdateSmContextCauseDuplicatePDUSessionID failed: %v", err)
	}

	if len(n2Rsp) == 0 {
		t.Fatal("expected non-empty N2 response")
	}

	if !smCtx.PDUSessionReleaseDueToDupPduID {
		t.Fatal("expected PDUSessionReleaseDueToDupPduID to be true")
	}

	upf.mu.Lock()
	if len(upf.deleteCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 DeleteSession call, got %d", len(upf.deleteCalls))
	}
	upf.mu.Unlock()
}

func TestUpdateSmContextCauseDuplicatePDUSessionID_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextCauseDuplicatePDUSessionID(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextCauseDuplicatePDUSessionID_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextCauseDuplicatePDUSessionID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2HandoverPreparing tests
// ===========================

func TestUpdateSmContextN2HandoverPreparing_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPreparing(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2HandoverPreparing_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPreparing(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2HandoverPrepared tests
// ===========================

func TestUpdateSmContextN2HandoverPrepared_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPrepared(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2HandoverPrepared_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPrepared(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextXnHandoverPathSwitchReq tests
// ===========================

func TestUpdateSmContextXnHandoverPathSwitchReq_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextXnHandoverPathSwitchReq(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextXnHandoverPathSwitchReq_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	_, err := s.UpdateSmContextXnHandoverPathSwitchReq(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextHandoverFailed tests
// ===========================

func TestUpdateSmContextHandoverFailed_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextHandoverFailed(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextHandoverFailed_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextHandoverFailed(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2InfoPduResSetupRsp tests
// ===========================

func TestUpdateSmContextN2InfoPduResSetupRsp_EmptyRef(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2InfoPduResSetupRsp_NotFound(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestUpdateSmContextN2InfoPduResSetupRsp_NilPFCPContext(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PFCPContext = nil

	err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, nil)
	if err == nil {
		t.Fatal("expected error for nil N2 data or nil PFCPContext")
	}
}

// ===========================
// HandlePfcpSessionReportRequest tests
// ===========================

func TestHandlePfcpSessionReportRequest_DLDR(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, _ := setupSessionWithTunnel(t, s)

	msg, err := core.BuildPfcpSessionReportRequestForDownlinkData(
		smCtx.PFCPContext.LocalSEID, 1,
		smCtx.Tunnel.DataPath.UpLinkTunnel.PDR.PDRID,
		smCtx.PolicyData.QosData.QFI,
	)
	if err != nil {
		t.Fatalf("build DLDR report: %v", err)
	}

	rsp, err := s.HandlePfcpSessionReportRequest(ctx, msg)
	if err != nil {
		t.Fatalf("HandlePfcpSessionReportRequest DLDR failed: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected non-nil response")
	}

	amfCb.mu.Lock()
	if len(amfCb.pageCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 page call, got %d", len(amfCb.pageCalls))
	}
	amfCb.mu.Unlock()
}

func TestHandlePfcpSessionReportRequest_USAR(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, _ := setupSessionWithTunnel(t, s)

	msg, err := core.BuildPfcpSessionReportRequestForUsage(
		smCtx.PFCPContext.LocalSEID, 1,
		1, 1,
		500, 300,
	)
	if err != nil {
		t.Fatalf("build USAR report: %v", err)
	}

	rsp, err := s.HandlePfcpSessionReportRequest(ctx, msg)
	if err != nil {
		t.Fatalf("HandlePfcpSessionReportRequest USAR failed: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected non-nil response")
	}

	store.mu.Lock()
	if len(store.usageLog) != 1 {
		store.mu.Unlock()
		t.Fatalf("expected 1 usage entry, got %d", len(store.usageLog))
	}

	entry := store.usageLog[0]
	store.mu.Unlock()

	if entry.imsi != testIMSI {
		t.Fatalf("expected IMSI %s, got %s", testIMSI, entry.imsi)
	}

	if entry.uplinkBytes != 500 {
		t.Fatalf("expected 500 uplink bytes, got %d", entry.uplinkBytes)
	}

	if entry.downlinkBytes != 300 {
		t.Fatalf("expected 300 downlink bytes, got %d", entry.downlinkBytes)
	}
}

func TestHandlePfcpSessionReportRequest_UnknownSEID(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	msg, err := core.BuildPfcpSessionReportRequestForDownlinkData(999, 1, 1, 1)
	if err != nil {
		t.Fatalf("build DLDR report: %v", err)
	}

	rsp, err := s.HandlePfcpSessionReportRequest(ctx, msg)
	if err == nil {
		t.Fatal("expected error for unknown SEID")
	}

	if rsp == nil {
		t.Fatal("expected reject response even on error")
	}
}

// ===========================
// SendFlowReport tests
// ===========================

func TestSendFlowReport_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	req := &pfcp_dispatcher.FlowReportRequest{
		IMSI:            testIMSI,
		SourceIP:        "10.0.0.1",
		DestinationIP:   "8.8.8.8",
		SourcePort:      12345,
		DestinationPort: 443,
		Protocol:        6,
		Packets:         100,
		Bytes:           50000,
		StartTime:       time.Now().Format(time.RFC3339),
		EndTime:         time.Now().Add(time.Minute).Format(time.RFC3339),
		Direction:       "uplink",
	}

	err := s.SendFlowReport(ctx, req)
	if err != nil {
		t.Fatalf("SendFlowReport failed: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.flowLog) != 1 {
		t.Fatalf("expected 1 flow report, got %d", len(store.flowLog))
	}

	if store.flowLog[0].IMSI != testIMSI {
		t.Fatalf("expected IMSI %s, got %s", testIMSI, store.flowLog[0].IMSI)
	}

	if store.flowLog[0].SourceIP != "10.0.0.1" {
		t.Fatalf("expected source IP 10.0.0.1, got %s", store.flowLog[0].SourceIP)
	}

	if store.flowLog[0].Bytes != 50000 {
		t.Fatalf("expected 50000 bytes, got %d", store.flowLog[0].Bytes)
	}
}

func TestSendFlowReport_NilRequest(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	err := s.SendFlowReport(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSendFlowReport_MissingIMSI(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)

	req := &pfcp_dispatcher.FlowReportRequest{
		SourceIP: "10.0.0.1",
	}

	err := s.SendFlowReport(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing IMSI")
	}
}

func TestSendFlowReport_StoreError(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	store.err = fmt.Errorf("database error")
	s := newTestSMF(store, upf, amfCb)

	req := &pfcp_dispatcher.FlowReportRequest{
		IMSI:     testIMSI,
		SourceIP: "10.0.0.1",
	}

	err := s.SendFlowReport(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when store fails")
	}
}

// ===========================
// IncrementDailyUsage tests
// ===========================

func TestIncrementDailyUsage_DelegatesToStore(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	err := s.IncrementDailyUsage(ctx, testIMSI, 1000, 2000)
	if err != nil {
		t.Fatalf("IncrementDailyUsage failed: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 1 {
		t.Fatalf("expected 1 usage entry, got %d", len(store.usageLog))
	}

	if store.usageLog[0].uplinkBytes != 1000 {
		t.Fatalf("expected 1000 uplink bytes, got %d", store.usageLog[0].uplinkBytes)
	}

	if store.usageLog[0].downlinkBytes != 2000 {
		t.Fatalf("expected 2000 downlink bytes, got %d", store.usageLog[0].downlinkBytes)
	}
}

// ===========================
// NGAP N2 payload builders for happy-path tests
// ===========================

// buildPDUSessionResourceSetupResponseTransfer builds an APER-encoded
// PDUSessionResourceSetupResponseTransfer with the given gNB DL GTP tunnel info.
func buildPDUSessionResourceSetupResponseTransfer(teid uint32, ip net.IP) ([]byte, error) {
	transfer := ngapType.PDUSessionResourceSetupResponseTransfer{}

	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel = &ngapType.GTPTunnel{}

	teidBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(teidBytes, teid)
	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.GTPTEID.Value = teidBytes
	transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ip.To4(),
		BitLength: 32,
	}

	transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List = append(
		transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List,
		ngapType.AssociatedQosFlowItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
		},
	)

	return aper.MarshalWithParams(transfer, "valueExt")
}

// buildPathSwitchRequestTransfer builds an APER-encoded PathSwitchRequestTransfer
// with the given target gNB DL GTP tunnel info.
func buildPathSwitchRequestTransfer(teid uint32, ip net.IP) ([]byte, error) {
	transfer := ngapType.PathSwitchRequestTransfer{}

	transfer.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	teidBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(teidBytes, teid)
	transfer.DLNGUUPTNLInformation.GTPTunnel.GTPTEID.Value = teidBytes
	transfer.DLNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ip.To4(),
		BitLength: 32,
	}

	transfer.QosFlowAcceptedList.List = append(transfer.QosFlowAcceptedList.List,
		ngapType.QosFlowAcceptedItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
		},
	)

	return aper.MarshalWithParams(transfer, "valueExt")
}

// ===========================
// UpdateSmContextN2InfoPduResSetupRsp happy-path
// ===========================

func TestUpdateSmContextN2InfoPduResSetupRsp_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	// Build N2 payload: gNB reports its DL tunnel endpoint.
	gnbIP := net.ParseIP("10.0.0.200").To4()
	gnbTEID := uint32(7000)

	n2Data, err := buildPDUSessionResourceSetupResponseTransfer(gnbTEID, gnbIP)
	if err != nil {
		t.Fatalf("build N2 payload: %v", err)
	}

	err = s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2Data)
	if err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	// Verify the session's ANInformation was updated.
	if !smCtx.Tunnel.ANInformation.IPAddress.Equal(gnbIP) {
		t.Fatalf("expected AN IP %s, got %s", gnbIP, smCtx.Tunnel.ANInformation.IPAddress)
	}

	if smCtx.Tunnel.ANInformation.TEID != gnbTEID {
		t.Fatalf("expected AN TEID %d, got %d", gnbTEID, smCtx.Tunnel.ANInformation.TEID)
	}

	// Verify DL FAR was updated with the gNB's outer header creation.
	dlFAR := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if dlFAR.ForwardingParameters.OuterHeaderCreation.TeID != gnbTEID {
		t.Fatalf("expected DL FAR TEID %d, got %d", gnbTEID, dlFAR.ForwardingParameters.OuterHeaderCreation.TeID)
	}

	if !dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address.Equal(gnbIP) {
		t.Fatalf("expected DL FAR IP %s, got %s", gnbIP, dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address)
	}

	// Verify a PFCP modification was sent.
	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}
}

// ===========================
// UpdateSmContextXnHandoverPathSwitchReq happy-path
// ===========================

func TestUpdateSmContextXnHandoverPathSwitchReq_HappyPath(t *testing.T) {
	store, upf, amfCb := defaultFakes()
	s := newTestSMF(store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	// Build N2 payload: target gNB reports its DL tunnel endpoint.
	targetGnbIP := net.ParseIP("10.0.0.201").To4()
	targetTEID := uint32(8000)

	n2Data, err := buildPathSwitchRequestTransfer(targetTEID, targetGnbIP)
	if err != nil {
		t.Fatalf("build N2 payload: %v", err)
	}

	n2Rsp, err := s.UpdateSmContextXnHandoverPathSwitchReq(ctx, ref, n2Data)
	if err != nil {
		t.Fatalf("UpdateSmContextXnHandoverPathSwitchReq: %v", err)
	}

	// Verify the N2 response (PathSwitchRequestAcknowledgeTransfer) is non-nil.
	if n2Rsp == nil {
		t.Fatal("expected non-nil N2 response")
	}

	// Verify the session's ANInformation was updated to the target gNB.
	if !smCtx.Tunnel.ANInformation.IPAddress.Equal(targetGnbIP) {
		t.Fatalf("expected AN IP %s, got %s", targetGnbIP, smCtx.Tunnel.ANInformation.IPAddress)
	}

	if smCtx.Tunnel.ANInformation.TEID != targetTEID {
		t.Fatalf("expected AN TEID %d, got %d", targetTEID, smCtx.Tunnel.ANInformation.TEID)
	}

	// Verify DL FAR was updated to forward to the target gNB.
	dlFAR := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if dlFAR.ForwardingParameters.OuterHeaderCreation.TeID != targetTEID {
		t.Fatalf("expected DL FAR TEID %d, got %d", targetTEID, dlFAR.ForwardingParameters.OuterHeaderCreation.TeID)
	}

	if !dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address.Equal(targetGnbIP) {
		t.Fatalf("expected DL FAR IP %s, got %s", targetGnbIP, dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address)
	}

	// Verify SNDEM flag was set for path switch.
	if dlFAR.ForwardingParameters.PFCPSMReqFlags == nil || !dlFAR.ForwardingParameters.PFCPSMReqFlags.Sndem {
		t.Fatal("expected PFCPSMReqFlags.Sndem to be set after path switch")
	}

	// Verify a PFCP modification was sent.
	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}
}
