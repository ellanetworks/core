// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
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

// rejectCauseCode decodes a PDU Session Establishment Reject NAS message and
// returns the 5GSM cause value.
func rejectCauseCode(t *testing.T, raw []byte) uint8 {
	t.Helper()

	m := new(nas.Message)

	if err := m.PlainNasDecode(&raw); err != nil {
		t.Fatalf("failed to decode reject NAS: %v", err)
	}

	if m.PDUSessionEstablishmentReject == nil {
		t.Fatal("expected PDUSessionEstablishmentReject, got nil")
	}

	return m.PDUSessionEstablishmentReject.GetCauseValue()
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

func buildPDUSessionModificationRequest(pduSessionID, pti uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionModificationRequest)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationRequest = nasMessage.NewPDUSessionModificationRequest(0)
	m.PDUSessionModificationRequest.SetMessageType(nas.MsgTypePDUSessionModificationRequest)
	m.PDUSessionModificationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationRequest.SetPDUSessionID(pduSessionID)
	m.PDUSessionModificationRequest.SetPTI(pti)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Modification Request: %v", err))
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

	dlPdr.FAR.ApplyAction = models.ApplyAction{Forw: true}
	dlPdr.FAR.ForwardingParameters = &models.ForwardingParameters{
		OuterHeaderCreation: &models.OuterHeaderCreation{
			Description: models.OuterHeaderCreationGtpUUdpIpv4,
			TEID:        6000,
			IPv4Address: net.ParseIP("10.0.0.100").To4(),
		},
	}

	policy := &smf.Policy{
		Ambr:    models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		QosData: models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1},
	}

	qer, err := s.NewQER(policy)
	if err != nil {
		t.Fatalf("NewQER: %v", err)
	}

	ulPdr.QER = qer
	dlPdr.QER = qer

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
	smCtx.PDUIPV4Address = net.ParseIP("10.0.0.1").To4()

	smCtx.PolicyData = policy

	return smCtx, smf.CanonicalName(supi, 1)
}

func modificationSMPayload(t *testing.T, n1Msg []byte) []byte {
	t.Helper()

	msg := nas.NewMessage()
	if err := msg.GsmMessageDecode(&n1Msg); err == nil && msg.PDUSessionModificationCommand != nil {
		return n1Msg
	}

	msg = nas.NewMessage()
	if err := msg.PlainNasDecode(&n1Msg); err != nil {
		t.Fatalf("decode DL NAS Transport: %v", err)
	}

	if msg.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport || msg.DLNASTransport == nil {
		t.Fatal("expected DL NAS Transport carrying N1 SM information")
	}

	if msg.DLNASTransport.GetPayloadContainerType() != nasMessage.PayloadContainerTypeN1SMInfo {
		t.Fatal("expected DL NAS Transport carrying N1 SM information")
	}

	return msg.DLNASTransport.GetPayloadContainerContents()
}

// ===========================
// DataPath tests
// ===========================

func TestActivateTunnelAndPDR_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pduAddr := netip.MustParseAddr("10.0.0.1")

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

	if !smCtx.Tunnel.DataPath.UpLinkTunnel.PDR.FAR.ApplyAction.Forw {
		t.Fatal("UL FAR should forward")
	}
}

func TestDeactivateTunnelAndPDR_CleansUp(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.ActivateSmContext(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestActivateSmContext_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.ActivateSmContext(context.Background(), "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// DeactivateSmContext tests
// ===========================

func TestDeactivateSmContext_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.DeactivateSmContext(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestDeactivateSmContext_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.DeactivateSmContext(context.Background(), "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestDeactivateSmContext_NilPFCPContext(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PFCPContext = nil

	err := s.DeactivateSmContext(ctx, ref)
	if err == nil {
		t.Fatal("expected error when PFCPContext is nil")
	}
}

func TestDeactivateSmContext_ModifyError(t *testing.T) {
	pcf, store, _, amfCb := defaultFakes()
	upf := &fakeUPF{err: fmt.Errorf("PFCP modify failed")}
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.ReleaseSmContext(context.Background(), "nonexistent-ref")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestReleaseSmContext_NoTunnel(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, _, amfCb := defaultFakes()
	upf := &fakeUPF{err: fmt.Errorf("PFCP delete failed")}
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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

func TestCreateSmContext_PolicyNotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	pcf.policy = nil
	s := newTestSMF(pcf, store, upf, amfCb)
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

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMRequestRejectedUnspecified {
		t.Fatalf("expected cause %d (RequestRejectedUnspecified), got %d", nasMessage.Cause5GSMRequestRejectedUnspecified, got)
	}
}

func TestCreateSmContext_DNNNotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	pcf.policy = nil
	pcf.err = fmt.Errorf("get session policy: data network not found: %w", smf.ErrDNNNotFound)
	s := newTestSMF(pcf, store, upf, amfCb)
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

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMMissingOrUnknownDNN {
		t.Fatalf("expected 5GSM cause %d (#27 missing or unknown DNN), got %d", nasMessage.Cause5GSMMissingOrUnknownDNN, got)
	}
}

// TestCreateSmContext_DNNNotInSlice verifies that when the slice is served but
// no policy provides the requested DNN, the SMF rejects with 5GSM cause #70
// "missing or unknown DNN in a slice" (TS 24.501 §9.11.4.2).
func TestCreateSmContext_DNNNotInSlice(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	pcf.policy = nil
	pcf.err = fmt.Errorf("get session policy: %w", smf.ErrDNNNotInSlice)
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, buildPDUSessionEstRequest())
	if err == nil {
		t.Fatal("expected error when DNN not in slice")
	}

	if rejectN1 == nil {
		t.Fatal("expected reject N1 message")
	}

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMMissingOrUnknownDNNInASlice {
		t.Fatalf("expected 5GSM cause %d (#70 missing or unknown DNN in a slice), got %d", nasMessage.Cause5GSMMissingOrUnknownDNNInASlice, got)
	}
}

func TestCreateSmContext_IPExhaustion(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	store.allocatedIP = netip.Addr{}
	store.allocateIPErr = fmt.Errorf("no IP available")
	s := newTestSMF(pcf, store, upf, amfCb)
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

	if got := rejectCauseCode(t, rejectN1); got != nasMessage.Cause5GSMInsufficientResources {
		t.Fatalf("expected cause %d (InsufficientResources), got %d", nasMessage.Cause5GSMInsufficientResources, got)
	}
}

func TestCreateSmContext_PFCPEstablishmentFailure(t *testing.T) {
	pcf, store, _, amfCb := defaultFakes()
	upf := &fakeUPF{err: fmt.Errorf("PFCP establishment failed")}
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, []byte{0x00})
	if err == nil {
		t.Fatal("expected error for invalid NAS message")
	}

	if rejectN1 == nil {
		t.Fatal("expected SMF to build a PDU Session Establishment Reject for malformed NAS")
	}

	if cause := rejectCauseCode(t, rejectN1); cause != nasMessage.Cause5GSMProtocolErrorUnspecified {
		t.Fatalf("expected cause %d (protocol error unspecified), got %d", nasMessage.Cause5GSMProtocolErrorUnspecified, cause)
	}

	if s.SessionCount() != 0 {
		t.Fatalf("expected no sessions to be created, got %d", s.SessionCount())
	}
}

func TestCreateSmContext_WrongNASMessageType(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	// A well-formed but inappropriate GSM message (release request rather than establishment).
	n1Msg := buildPDUSessionReleaseRequest(1, 10)

	_, rejectN1, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, n1Msg)
	if err == nil {
		t.Fatal("expected error for unexpected NAS message type")
	}

	if rejectN1 == nil {
		t.Fatal("expected SMF to build a PDU Session Establishment Reject for wrong NAS type")
	}

	if cause := rejectCauseCode(t, rejectN1); cause != nasMessage.Cause5GSMMessageTypeNotCompatibleWithTheProtocolState {
		t.Fatalf("expected cause %d (message type not compatible with protocol state), got %d", nasMessage.Cause5GSMMessageTypeNotCompatibleWithTheProtocolState, cause)
	}

	if s.SessionCount() != 0 {
		t.Fatalf("expected no sessions to be created, got %d", s.SessionCount())
	}
}

func TestCreateSmContext_ReplacesExistingSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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

// CreateSmContext/ReleaseSmContext BGP-announcement tests were removed
// when the SMF→BGP coupling was deleted. Route announce/withdraw is now
// driven by the BGP reconciler reading the replicated ip_leases table.

func TestReleaseSmContext_NilPDUAddress_SkipsIPRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PDUIPV4Address = nil // simulate a session that never had an IP allocated

	err := s.ReleaseSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ReleaseSmContext failed: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 0 {
		t.Fatal("should not call ReleaseIP when PDUAddress is nil")
	}
}

func TestRemoveSession_NilPDUAddress_SkipsIPRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	supi := testSUPI()
	bgCtx := context.Background()

	s.NewSession(supi, 1, testDNN, testSnssai) // PDUAddress is nil by default
	ref := smf.CanonicalName(supi, 1)

	s.RemoveSession(bgCtx, ref)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.releasedIPs) != 0 {
		t.Fatal("should not call ReleaseIP when PDUAddress is nil")
	}
}

// ===========================
// UpdateSmContextN1Msg tests
// ===========================

func TestUpdateSmContextN1Msg_ReleaseRequest(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	n1Msg := buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	// A UE-requested release runs the network-requested release procedure: the
	// Release Command goes to the UE/gNB via the AMF, not back as an
	// UpdateResult (TS 24.501 §6.4.3.3 → §6.3.3).
	if rsp != nil {
		t.Fatalf("expected no UpdateResult, got %+v", rsp)
	}

	amfCb.mu.Lock()
	if len(amfCb.releaseCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 ReleaseSession call, got %d", len(amfCb.releaseCalls))
	}
	amfCb.mu.Unlock()

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

	// The session is retained until the UE confirms with Release Complete or
	// T3592 expires (TS 24.501 §6.3.3.3).
	if s.GetSession(ref) == nil {
		t.Fatal("expected session to be retained while T3592 is running")
	}
}

func TestUpdateSmContextN1Msg_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextN1Msg(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN1Msg_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextN1Msg(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2InfoPduResSetupFail tests
// ===========================

func TestUpdateSmContextN2InfoPduResSetupFail_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupFail(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2InfoPduResSetupFail_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupFail(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2InfoPduResRelRsp tests
// ===========================

func TestUpdateSmContextN2InfoPduResRelRsp_NotDuplicate(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2InfoPduResRelRsp_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	// Idempotent: returns nil when session already removed (e.g. slice-mismatch release).
	err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected nil for already-removed session, got error: %v", err)
	}
}

// ===========================
// UpdateSmContextCauseDuplicatePDUSessionID tests
// ===========================

func TestUpdateSmContextCauseDuplicatePDUSessionID_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextCauseDuplicatePDUSessionID(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextCauseDuplicatePDUSessionID_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextCauseDuplicatePDUSessionID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2HandoverPreparing tests
// ===========================

func TestUpdateSmContextN2HandoverPreparing_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPreparing(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2HandoverPreparing_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPreparing(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2HandoverPrepared tests
// ===========================

func TestUpdateSmContextN2HandoverPrepared_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPrepared(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2HandoverPrepared_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextN2HandoverPrepared(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextXnHandoverPathSwitchReq tests
// ===========================

func TestUpdateSmContextXnHandoverPathSwitchReq_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextXnHandoverPathSwitchReq(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextXnHandoverPathSwitchReq_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, err := s.UpdateSmContextXnHandoverPathSwitchReq(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextHandoverFailed tests
// ===========================

func TestUpdateSmContextHandoverFailed_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextHandoverFailed(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextHandoverFailed_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextHandoverFailed(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ===========================
// UpdateSmContextN2InfoPduResSetupRsp tests
// ===========================

func TestUpdateSmContextN2InfoPduResSetupRsp_EmptyRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestUpdateSmContextN2InfoPduResSetupRsp_NotFound(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestUpdateSmContextN2InfoPduResSetupRsp_NilPFCPContext(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PFCPContext = nil

	err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, nil)
	if err == nil {
		t.Fatal("expected error for nil N2 data or nil PFCPContext")
	}
}

func TestUpdateSmContextN2InfoPduResSetupRsp_TunnelReleased(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.Tunnel = nil
	smCtx.PFCPContext = nil

	gnbIP := net.ParseIP("10.0.0.200").To4()

	n2Data, err := buildPDUSessionResourceSetupResponseTransfer(7000, gnbIP)
	if err != nil {
		t.Fatalf("build N2 payload: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2Data); err == nil {
		t.Fatal("expected error when tunnel was released, got nil")
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 0 {
		t.Fatalf("expected no PFCP modify calls after tunnel release, got %d", len(upf.modifyCalls))
	}
}

// ===========================
// HandleDownlinkDataReport tests
// ===========================

func TestHandleDownlinkDataReport(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, _ := setupSessionWithTunnel(t, s)

	err := s.HandleDownlinkDataReport(ctx, &models.DownlinkDataReport{
		SEID:  smCtx.PFCPContext.LocalSEID,
		PDRID: smCtx.Tunnel.DataPath.UpLinkTunnel.PDR.PDRID,
		QFI:   smCtx.PolicyData.QosData.QFI,
	})
	if err != nil {
		t.Fatalf("HandleDownlinkDataReport failed: %v", err)
	}

	amfCb.mu.Lock()
	if len(amfCb.pageCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 page call, got %d", len(amfCb.pageCalls))
	}
	amfCb.mu.Unlock()
}

func TestReconcileSmContext_UsesNewPolicyForPFCPAndN1N2(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "200 Mbps",
			SessionAmbrDownlink: "300 Mbps",
			Var5qi:              8,
			Arp:                 14,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	upf.mu.Lock()
	if len(upf.modifyCalls) != 1 {
		upf.mu.Unlock()
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}

	modifyReq := upf.modifyCalls[0]
	upf.mu.Unlock()

	if len(modifyReq.UpdateQERs) != 1 {
		t.Fatalf("expected 1 QER update, got %d", len(modifyReq.UpdateQERs))
	}

	qer := modifyReq.UpdateQERs[0]
	if qer.MBR == nil {
		t.Fatal("expected QER MBR")
	}

	if qer.MBR.ULMBR != 200000 || qer.MBR.DLMBR != 300000 {
		t.Fatalf("QER MBR = %d/%d, want 200000/300000", qer.MBR.ULMBR, qer.MBR.DLMBR)
	}

	amfCb.mu.Lock()
	if len(amfCb.modifyCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 N1N2 modify call, got %d", len(amfCb.modifyCalls))
	}

	call := amfCb.modifyCalls[0]
	amfCb.mu.Unlock()

	oldPayload, err := smfNas.BuildPDUSessionModificationCommand(smCtx.PDUSessionID, &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"}, &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}, nil)
	if err != nil {
		t.Fatalf("build old policy modification command: %v", err)
	}

	newPayload, err := smfNas.BuildPDUSessionModificationCommand(smCtx.PDUSessionID, &models.Ambr{Uplink: "200 Mbps", Downlink: "300 Mbps"}, &models.QosData{Var5qi: 8, Arp: &models.Arp{PriorityLevel: 14}, QFI: 1}, nil)
	if err != nil {
		t.Fatalf("build new policy modification command: %v", err)
	}

	gotPayload := modificationSMPayload(t, call.n1Msg)
	if string(gotPayload) == string(oldPayload) {
		t.Fatal("N1 modification command used old policy")
	}

	if string(gotPayload) != string(newPayload) {
		t.Fatalf("N1 modification command did not use expected new policy")
	}

	if smCtx.PolicyData.Ambr.Uplink != "200 Mbps" || smCtx.PolicyData.Ambr.Downlink != "300 Mbps" {
		t.Fatalf("stored AMBR = %s/%s", smCtx.PolicyData.Ambr.Uplink, smCtx.PolicyData.Ambr.Downlink)
	}

	if smCtx.PolicyData.QosData.Var5qi != 8 {
		t.Fatalf("stored 5QI = %d, want 8", smCtx.PolicyData.QosData.Var5qi)
	}
}

func TestReconcileSmContext_AmbrOnly(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "300 Mbps",
			SessionAmbrDownlink: "400 Mbps",
			Var5qi:              9,
			Arp:                 1,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	qer := upf.modifyCalls[0].UpdateQERs[0]
	if qer.MBR.ULMBR != 300000 || qer.MBR.DLMBR != 400000 {
		t.Fatalf("QER MBR = %d/%d, want 300000/400000", qer.MBR.ULMBR, qer.MBR.DLMBR)
	}

	expectedPayload, err := smfNas.BuildPDUSessionModificationCommand(smCtx.PDUSessionID, &models.Ambr{Uplink: "300 Mbps", Downlink: "400 Mbps"}, nil, nil)
	if err != nil {
		t.Fatalf("build expected N1: %v", err)
	}

	gotPayload := modificationSMPayload(t, amfCb.modifyCalls[0].n1Msg)
	if string(gotPayload) != string(expectedPayload) {
		t.Fatal("N1 modification command mismatch")
	}
}

func TestReconcileSmContext_QoSOnly(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              8,
			Arp:                 14,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	qer := upf.modifyCalls[0].UpdateQERs[0]
	if qer.MBR.ULMBR != 100000 || qer.MBR.DLMBR != 200000 {
		t.Fatalf("QER MBR = %d/%d, want 100000/200000", qer.MBR.ULMBR, qer.MBR.DLMBR)
	}

	expectedPayload, err := smfNas.BuildPDUSessionModificationCommand(smCtx.PDUSessionID, nil, &models.QosData{Var5qi: 8, Arp: &models.Arp{PriorityLevel: 14}, QFI: 1}, nil)
	if err != nil {
		t.Fatalf("build expected N1: %v", err)
	}

	gotPayload := modificationSMPayload(t, amfCb.modifyCalls[0].n1Msg)
	if string(gotPayload) != string(expectedPayload) {
		t.Fatal("N1 modification command mismatch")
	}
}

// TestReconcileSmContext_SliceMismatchFullCleanup verifies that a slice-mismatch
// release performs full data-plane cleanup (IP release, PFCP deletion) before
// signaling the UE (TS 23.502 §4.3.4.2 Step 2), and retains the session while
// T3592 awaits the UE's Release Complete (TS 24.501 §6.3.3); the N2 release
// response then removes it.
func TestReconcileSmContext_SliceMismatchFullCleanup(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcileSliceMismatch,
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	// IP addresses should have been released.
	store.mu.Lock()
	releasedIPs := len(store.releasedIPs)
	store.mu.Unlock()

	if releasedIPs == 0 {
		t.Fatal("expected IP release during slice-mismatch release, got 0")
	}

	// PFCP session should have been deleted.
	upf.mu.Lock()
	deleteCalls := len(upf.deleteCalls)
	upf.mu.Unlock()

	if deleteCalls == 0 {
		t.Fatal("expected PFCP session deletion during slice-mismatch release, got 0")
	}

	// AMF release signaling should have been sent.
	amfCb.mu.Lock()
	releaseCalls := len(amfCb.releaseCalls)
	amfCb.mu.Unlock()

	if releaseCalls != 1 {
		t.Fatalf("expected 1 release signaling call, got %d", releaseCalls)
	}

	// The session is retained until the release is confirmed.
	if s.GetSession(ref) == nil {
		t.Fatal("expected session to be retained while T3592 is running")
	}

	if err := s.UpdateSmContextN2InfoPduResRelRsp(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("expected session to be removed after N2 release response")
	}
}

func TestReconcileSmContext_ModifyIdleUE_CommitsPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	// Simulate idle UE: ModifyN1N2 returns ErrUENotReachable.
	amfCb.err = smf.ErrUENotReachable
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "500 Mbps",
			SessionAmbrDownlink: "600 Mbps",
			Var5qi:              7,
			Arp:                 10,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext should succeed for idle UE, got: %v", err)
	}

	// PFCP should still have been updated.
	upf.mu.Lock()
	pfcpModifyCalls := len(upf.modifyCalls)
	upf.mu.Unlock()

	if pfcpModifyCalls != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", pfcpModifyCalls)
	}

	// Policy should have been committed despite N1N2 skip.
	if smCtx.PolicyData.Ambr.Uplink != "500 Mbps" || smCtx.PolicyData.Ambr.Downlink != "600 Mbps" {
		t.Fatalf("policy not committed: AMBR = %s/%s", smCtx.PolicyData.Ambr.Uplink, smCtx.PolicyData.Ambr.Downlink)
	}

	if smCtx.PolicyData.QosData.Var5qi != 7 {
		t.Fatalf("policy not committed: 5QI = %d, want 7", smCtx.PolicyData.QosData.Var5qi)
	}
}

func TestReconcileSmContext_ReleaseIdleUE_RemovesSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	// Simulate idle UE: ReleaseSession returns ErrUENotReachable.
	amfCb.err = smf.ErrUENotReachable
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcileSliceMismatch,
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext should succeed for idle UE release, got: %v", err)
	}

	// Session should be removed even though UE is idle.
	if s.GetSession(ref) != nil {
		t.Fatal("expected session to be removed after idle-UE slice-mismatch release")
	}

	// IP should still have been released.
	store.mu.Lock()
	releasedIPs := len(store.releasedIPs)
	store.mu.Unlock()

	if releasedIPs == 0 {
		t.Fatal("expected IP release during idle-UE release")
	}

	// PFCP session should still have been deleted.
	upf.mu.Lock()
	deleteCalls := len(upf.deleteCalls)
	upf.mu.Unlock()

	if deleteCalls == 0 {
		t.Fatal("expected PFCP deletion during idle-UE release")
	}
}

// TestReconcileSmContext_DNSChange verifies that a DNS change triggers a PDU
// Session Modification Command carrying the new DNS in Extended PCO (TS 24.501
// §6.3.2), without releasing the session.
func TestReconcileSmContext_DNSChange(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
			DNS:                 "8.8.4.4",
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	// No PFCP modify should be called for DNS-only change.
	upf.mu.Lock()
	if len(upf.modifyCalls) != 0 {
		upf.mu.Unlock()
		t.Fatalf("expected 0 PFCP modify calls for DNS-only change, got %d", len(upf.modifyCalls))
	}
	upf.mu.Unlock()

	// AMF ModifyN1N2 should have been called with DNS in PCO.
	amfCb.mu.Lock()
	if len(amfCb.modifyCalls) != 1 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 1 N1N2 modify call, got %d", len(amfCb.modifyCalls))
	}

	call := amfCb.modifyCalls[0]
	amfCb.mu.Unlock()

	// N2 must be nil for DNS-only changes: no gNB resource modification is
	// needed; the NAS message is delivered via DL NAS Transport per
	// TS 23.502 §4.3.3.2.
	if call.n2Msg != nil {
		t.Fatalf("expected nil N2 message for DNS-only change, got %d bytes", len(call.n2Msg))
	}

	n1Payload := modificationSMPayload(t, call.n1Msg)

	// Decode and verify Extended PCO contains DNS server IE.
	msg := nas.NewMessage()
	if err := msg.PlainNasDecode(&n1Payload); err != nil {
		t.Fatalf("decode N1 modification command: %v", err)
	}

	if msg.PDUSessionModificationCommand == nil {
		t.Fatal("PDUSessionModificationCommand is nil")
	}

	pco := msg.PDUSessionModificationCommand.ExtendedProtocolConfigurationOptions
	if pco == nil {
		t.Fatal("ExtendedProtocolConfigurationOptions is nil; DNS should be in PCO")
	}

	contents := pco.GetExtendedProtocolConfigurationOptionsContents()
	if len(contents) == 0 {
		t.Fatal("PCO contents is empty")
	}

	// Verify the session policy was updated with new DNS.
	smCtx.Mutex.Lock()
	if smCtx.PolicyData.DNS == nil || !smCtx.PolicyData.DNS.Equal(net.ParseIP("8.8.4.4")) {
		smCtx.Mutex.Unlock()
		t.Fatalf("expected DNS 8.8.4.4, got %v", smCtx.PolicyData.DNS)
	}
	smCtx.Mutex.Unlock()
}

// TestReconcileSmContext_InvalidDNS verifies that an invalid DNS address in the
// new policy is rejected with an error rather than silently producing a nil IP.
func TestReconcileSmContext_InvalidDNS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
			DNS:                 "not-a-valid-ip",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid DNS address, got nil")
	}

	// No PFCP or AMF calls should have been made.
	upf.mu.Lock()
	if len(upf.modifyCalls) != 0 {
		upf.mu.Unlock()
		t.Fatalf("expected 0 PFCP modify calls, got %d", len(upf.modifyCalls))
	}
	upf.mu.Unlock()

	amfCb.mu.Lock()
	if len(amfCb.modifyCalls) != 0 {
		amfCb.mu.Unlock()
		t.Fatalf("expected 0 AMF modify calls, got %d", len(amfCb.modifyCalls))
	}
	amfCb.mu.Unlock()
}

// TestReconcileSmContext_MTUChange verifies that an MTU change triggers a
// session release with cause #39 (TS 23.501 §5.6.10.4 NOTE 3).
func TestReconcileSmContext_MTUChange(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
			MTU:                 1400,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	// Session is retained until the release is confirmed (TS 24.501 §6.3.3).
	if s.GetSession(ref) == nil {
		t.Fatal("expected session to be retained while T3592 is running")
	}

	// IP addresses should have been released.
	store.mu.Lock()
	releasedIPs := len(store.releasedIPs)
	store.mu.Unlock()

	if releasedIPs == 0 {
		t.Fatal("expected IP release during MTU-change release, got 0")
	}

	// PFCP session should have been deleted.
	upf.mu.Lock()
	deleteCalls := len(upf.deleteCalls)
	upf.mu.Unlock()

	if deleteCalls == 0 {
		t.Fatal("expected PFCP session deletion during MTU-change release, got 0")
	}

	// AMF release signaling should have been sent.
	amfCb.mu.Lock()
	releaseCalls := len(amfCb.releaseCalls)
	amfCb.mu.Unlock()

	if releaseCalls != 1 {
		t.Fatalf("expected 1 release signaling call, got %d", releaseCalls)
	}
}

// TestReconcileSmContext_PoolChange verifies that an IP pool change triggers a
// session release with cause #39.
func TestReconcileSmContext_PoolChange(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
			IPv4Pool:            "10.0.1.0/24",
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	// Session is retained until the release is confirmed (TS 24.501 §6.3.3).
	if s.GetSession(ref) == nil {
		t.Fatal("expected session to be retained while T3592 is running")
	}

	// IP addresses should have been released.
	store.mu.Lock()
	releasedIPs := len(store.releasedIPs)
	store.mu.Unlock()

	if releasedIPs == 0 {
		t.Fatal("expected IP release during pool-change release, got 0")
	}

	// PFCP session should have been deleted.
	upf.mu.Lock()
	deleteCalls := len(upf.deleteCalls)
	upf.mu.Unlock()

	if deleteCalls == 0 {
		t.Fatal("expected PFCP session deletion during pool-change release, got 0")
	}

	// AMF release signaling should have been sent.
	amfCb.mu.Lock()
	releaseCalls := len(amfCb.releaseCalls)
	amfCb.mu.Unlock()

	if releaseCalls != 1 {
		t.Fatalf("expected 1 release signaling call, got %d", releaseCalls)
	}
}

// TestReconcileSmContext_DNSUnchanged verifies that no N1N2 call is made when
// nothing in the delta actually changed.
func TestReconcileSmContext_DNSUnchanged(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	// Send the same values that are already in the session (no actual change).
	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}

	// No session modification should be called when nothing changed.
	amfCb.mu.Lock()
	modifyCalls := len(amfCb.modifyCalls)
	amfCb.mu.Unlock()

	if modifyCalls != 0 {
		t.Fatalf("expected 0 modify calls when nothing changed, got %d", modifyCalls)
	}
}

// TestReconcileSmContext_DNSIdleUE verifies that DNS policy is committed even
// when the UE is idle (N1N2 delivery skipped).
func TestReconcileSmContext_DNSIdleUE(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	amfCb.err = smf.ErrUENotReachable
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	err := s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 1,
			DNS:                 "1.1.1.1",
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext should succeed for idle UE, got: %v", err)
	}

	// Policy should have been committed despite N1N2 skip.
	smCtx.Mutex.Lock()
	if smCtx.PolicyData.DNS == nil || !smCtx.PolicyData.DNS.Equal(net.ParseIP("1.1.1.1")) {
		smCtx.Mutex.Unlock()
		t.Fatalf("policy not committed: DNS = %v", smCtx.PolicyData.DNS)
	}
	smCtx.Mutex.Unlock()
}

// ===========================
// HandleUsageReport tests
// ===========================

func TestHandleUsageReport(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, _ := setupSessionWithTunnel(t, s)

	err := s.HandleUsageReport(ctx, &models.UsageReport{
		SEID:           smCtx.PFCPContext.LocalSEID,
		UplinkVolume:   500,
		DownlinkVolume: 300,
	})
	if err != nil {
		t.Fatalf("HandleUsageReport failed: %v", err)
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

func TestHandleDownlinkDataReport_UnknownSEID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	err := s.HandleDownlinkDataReport(ctx, &models.DownlinkDataReport{
		SEID:  999,
		PDRID: 1,
		QFI:   1,
	})
	if err == nil {
		t.Fatal("expected error for unknown SEID")
	}
}

// ===========================
// SendFlowReports tests
// ===========================

func TestSendFlowReports_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	req := &models.FlowReportRequest{
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
		Direction:       models.DirectionUplink,
	}

	err := s.SendFlowReports(ctx, []*models.FlowReportRequest{req})
	if err != nil {
		t.Fatalf("SendFlowReports failed: %v", err)
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

func TestSendFlowReports_NilRequestSkipped(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	err := s.SendFlowReports(context.Background(), []*models.FlowReportRequest{nil})
	if err != nil {
		t.Fatalf("expected nil request to be skipped, got error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.flowLog) != 0 {
		t.Fatalf("expected 0 flow reports, got %d", len(store.flowLog))
	}
}

func TestSendFlowReports_MissingIMSISkipped(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	req := &models.FlowReportRequest{
		SourceIP: "10.0.0.1",
	}

	err := s.SendFlowReports(context.Background(), []*models.FlowReportRequest{req})
	if err != nil {
		t.Fatalf("expected empty-IMSI request to be skipped, got error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.flowLog) != 0 {
		t.Fatalf("expected 0 flow reports, got %d", len(store.flowLog))
	}
}

func TestSendFlowReports_StoreError(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	store.err = fmt.Errorf("database error")
	s := newTestSMF(pcf, store, upf, amfCb)

	req := &models.FlowReportRequest{
		IMSI:      testIMSI,
		SourceIP:  "10.0.0.1",
		Direction: models.DirectionUplink,
	}

	err := s.SendFlowReports(context.Background(), []*models.FlowReportRequest{req})
	if err == nil {
		t.Fatal("expected error when store fails")
	}
}

// ===========================
// IncrementDailyUsage tests
// ===========================

func TestIncrementDailyUsage_DelegatesToStore(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	if !smCtx.Tunnel.ANInformation.IPv4Address.Equal(gnbIP) {
		t.Fatalf("expected AN IP %s, got %s", gnbIP, smCtx.Tunnel.ANInformation.IPv4Address)
	}

	if smCtx.Tunnel.ANInformation.TEID != gnbTEID {
		t.Fatalf("expected AN TEID %d, got %d", gnbTEID, smCtx.Tunnel.ANInformation.TEID)
	}

	// Verify DL FAR was updated with the gNB's outer header creation.
	dlFAR := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if dlFAR.ForwardingParameters.OuterHeaderCreation.TEID != gnbTEID {
		t.Fatalf("expected DL FAR TEID %d, got %d", gnbTEID, dlFAR.ForwardingParameters.OuterHeaderCreation.TEID)
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
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
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
	if !smCtx.Tunnel.ANInformation.IPv4Address.Equal(targetGnbIP) {
		t.Fatalf("expected AN IP %s, got %s", targetGnbIP, smCtx.Tunnel.ANInformation.IPv4Address)
	}

	if smCtx.Tunnel.ANInformation.TEID != targetTEID {
		t.Fatalf("expected AN TEID %d, got %d", targetTEID, smCtx.Tunnel.ANInformation.TEID)
	}

	// Verify DL FAR was updated to forward to the target gNB.
	dlFAR := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if dlFAR.ForwardingParameters.OuterHeaderCreation.TEID != targetTEID {
		t.Fatalf("expected DL FAR TEID %d, got %d", targetTEID, dlFAR.ForwardingParameters.OuterHeaderCreation.TEID)
	}

	if !dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address.Equal(targetGnbIP) {
		t.Fatalf("expected DL FAR IP %s, got %s", targetGnbIP, dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address)
	}

	// Verify a PFCP modification was sent.
	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}
}

// ===========================
// TestUpdateSmContextN2HandoverPrepared_Complete verifies that UpdateSmContextN2HandoverPrepared
// only updates in-memory state (no PFCP calls), and that UpdateSmContextN2HandoverComplete
// performs the PFCP N4 Session Modification as required by 3GPP TS 23.502 §4.9.1.3.3 step 7.

func TestUpdateSmContextN2HandoverPrepared_Complete(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	// Phase 1: "Preparing" — SMF receives HandoverRequiredTransfer.
	hoRequiredTransfer := ngapType.HandoverRequiredTransfer{}

	hoRequiredData, err := aper.MarshalWithParams(hoRequiredTransfer, "valueExt")
	if err != nil {
		t.Fatalf("marshal HandoverRequiredTransfer: %v", err)
	}

	n2Rsp, err := s.UpdateSmContextN2HandoverPreparing(ctx, ref, hoRequiredData)
	if err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPreparing: %v", err)
	}

	if n2Rsp == nil {
		t.Fatal("expected non-nil N2 response from Preparing phase")
	}

	// Phase 2: "Prepared" — SMF receives HandoverRequestAcknowledgeTransfer
	// from the target gNB with new DL tunnel info.
	targetGnbIP := net.ParseIP("10.0.0.201").To4()
	targetTEID := uint32(8000)

	hoAckData, err := buildHandoverRequestAcknowledgeTransfer(targetTEID, targetGnbIP)
	if err != nil {
		t.Fatalf("build HandoverRequestAcknowledgeTransfer: %v", err)
	}

	n2Rsp2, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, hoAckData)
	if err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if n2Rsp2 == nil {
		t.Fatal("expected non-nil N2 response (HandoverCommandTransfer) from Prepared phase")
	}

	// Verify the session's ANInformation was updated to the target gNB.
	if !smCtx.Tunnel.ANInformation.IPv4Address.Equal(targetGnbIP) {
		t.Fatalf("expected AN IP %s, got %s", targetGnbIP, smCtx.Tunnel.ANInformation.IPv4Address)
	}

	if smCtx.Tunnel.ANInformation.TEID != targetTEID {
		t.Fatalf("expected AN TEID %d, got %d", targetTEID, smCtx.Tunnel.ANInformation.TEID)
	}

	// Verify DL FAR was updated in memory.
	dlFAR := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if dlFAR.ForwardingParameters.OuterHeaderCreation.TEID != targetTEID {
		t.Fatalf("expected DL FAR TEID %d, got %d", targetTEID, dlFAR.ForwardingParameters.OuterHeaderCreation.TEID)
	}

	if !dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address.Equal(targetGnbIP) {
		t.Fatalf("expected DL FAR IP %s, got %s", targetGnbIP, dlFAR.ForwardingParameters.OuterHeaderCreation.IPv4Address)
	}

	// Verify UpdateSmContextN2HandoverPrepared did NOT call ModifySession.
	// Per 3GPP TS 23.502 §4.9.1.3.3, the N4 modification happens after HandoverNotify.
	upf.mu.Lock()
	if len(upf.modifyCalls) != 0 {
		t.Fatalf("expected 0 PFCP modify calls after N2 handover prepared, got %d", len(upf.modifyCalls))
	}
	upf.mu.Unlock()

	// Phase 3: "Complete" — AMF calls UpdateSmContextN2HandoverComplete after
	// the UE has successfully moved to the target gNB.
	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	// Verify the SMF sent exactly one PFCP modification to the UPF during completion.
	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call after N2 handover complete, got %d", len(upf.modifyCalls))
	}
}

// buildHandoverRequestAcknowledgeTransfer builds an APER-encoded
// HandoverRequestAcknowledgeTransfer with the given target gNB DL GTP tunnel info.
func buildHandoverRequestAcknowledgeTransfer(teid uint32, ip net.IP) ([]byte, error) {
	teidBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(teidBytes, teid)

	transfer := ngapType.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngapType.UPTransportLayerInformation{
			Present: ngapType.UPTransportLayerInformationPresentGTPTunnel,
			GTPTunnel: &ngapType.GTPTunnel{
				TransportLayerAddress: ngapType.TransportLayerAddress{
					Value: aper.BitString{
						Bytes:     ip.To4(),
						BitLength: 32,
					},
				},
				GTPTEID: ngapType.GTPTEID{
					Value: teidBytes,
				},
			},
		},
		QosFlowSetupResponseList: ngapType.QosFlowListWithDataForwarding{
			List: []ngapType.QosFlowItemWithDataForwarding{
				{
					QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
				},
			},
		},
	}

	return aper.MarshalWithParams(transfer, "valueExt")
}

// TestUpdateSmContextN1Msg_ModificationRejected verifies that a UE-requested PDU
// Session Modification Request is answered with a PDU Session Modification Reject
// echoing the request's PTI (TS 24.501 §6.4.2.4, §7.3.1), and that the session is
// not torn down.
func TestUpdateSmContextN1Msg_ModificationRejected(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 7

	n1Msg := buildPDUSessionModificationRequest(smCtx.PDUSessionID, pti)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg (modification) failed: %v", err)
	}

	if rsp == nil || rsp.N1Msg == nil {
		t.Fatal("expected a Modification Reject N1 message (TS 24.501 §6.4.2.4), got none")
	}

	if rsp.ReleaseN2 {
		t.Error("modification reject must not signal N2 release")
	}

	m := new(nas.Message)
	if err := m.PlainNasDecode(&rsp.N1Msg); err != nil {
		t.Fatalf("decode N1 response: %v", err)
	}

	if m.PDUSessionModificationReject == nil {
		t.Fatalf("expected PDUSessionModificationReject, got message type %d", m.GsmHeader.GetMessageType())
	}

	if got := m.PDUSessionModificationReject.GetPTI(); got != pti {
		t.Errorf("reject PTI = %d, want %d (echoed from request)", got, pti)
	}

	if got := m.PDUSessionModificationReject.GetCauseValue(); got != nasMessage.Cause5GSMRequestRejectedUnspecified {
		t.Errorf("reject cause = %d, want %d (request rejected, unspecified)", got, nasMessage.Cause5GSMRequestRejectedUnspecified)
	}
}
