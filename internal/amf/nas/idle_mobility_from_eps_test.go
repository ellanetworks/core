// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

const (
	idleEKSI  uint8 = 4
	nativeKSI uint8 = 1
)

func arrivingMMContext(supi etsi.SUPI) interworking.MMContextResponse {
	var kasme [32]byte
	for i := range kasme {
		kasme[i] = byte(i + 1)
	}

	return interworking.MMContextResponse{
		SUPI: supi,
		Security: interworking.EPSSecurityContext{
			KASME:                kasme,
			EKSI:                 nas.KeySetIdentifier{Value: idleEKSI},
			ULNASCount:           nas.MakeCount(0, 7),
			DLNASCount:           nas.MakeCount(0, 2),
			Algorithms:           interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
			UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
		},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		PDNConnections: []interworking.PDNConnection{
			{PDUSessionID: 3, EPSBearerIdentity: 6, APN: "internet", Snssai: models.Snssai{Sst: 1, Sd: "010203"}},
		},
		AMBRUplink:   models.MustParseBitRate("50 Mbps"),
		AMBRDownlink: models.MustParseBitRate("100 Mbps"),
	}
}

func idleArrivalRequest() *fgs.RegistrationRequest {
	return &fgs.RegistrationRequest{
		RegistrationType:       fgs.RegistrationTypeMobilityUpdating,
		NgKSI:                  nas.KeySetIdentifier{Value: idleEKSI, Mapped: true},
		MobileIdentity:         mappedGUTIIdentity(),
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: []byte{uint8(eps.PDEMM), 0x01, 0xde, 0xad, 0xbe, 0xef, 0x00},
		UESecurityCapability:   &fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0},
	}
}

func nativeIdleArrivalRequest() *fgs.RegistrationRequest {
	req := idleArrivalRequest()
	req.NgKSI = nas.KeySetIdentifier{Value: nativeKSI}

	return req
}

func idleArrivalUE(t *testing.T) (*amf.UeContext, *amf.AMF, *fakeEPSPeer, *fakeSmf) {
	t.Helper()

	ue, _, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	peer := &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}
	amfInstance.EPS = peer

	ue.SetSecuredForTest(false)

	return ue, amfInstance, peer, smf
}

// TS 24.501 §5.5.1.3.2 c
func TestMovingFromEPCInIdleModeNeedsTheContainer(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	conn := ue.Conn()
	conn.SetRegistrationType5GS(uint8(fgs.RegistrationTypeMobilityUpdating))

	req := idleArrivalRequest()
	if !movingFromEPCInIdleMode(conn, req) {
		t.Error("a mobility update reporting EMM-REGISTERED with a container is not taken as an idle-mode change")
	}

	req.EPSNASMessageContainer = nil
	if movingFromEPCInIdleMode(conn, req) {
		t.Error("a mobility update with no container is taken as an idle-mode change")
	}

	req = idleArrivalRequest()
	req.UEStatus = nil

	if movingFromEPCInIdleMode(conn, req) {
		t.Error("a mobility update that does not report EMM-REGISTERED is taken as an inter-system change")
	}
}

// TS 33.501 §8.2
func TestRecoverContextFromEPSInstallsTheMappedContext(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), false)

	if len(peer.MMContextRequests) != 1 {
		t.Fatalf("MME context requests = %d, want 1", len(peer.MMContextRequests))
	}

	if !ue.SecurityContextIsValid() {
		t.Fatal("no security context was installed, so the AMF would authenticate a UE the MME already vouched for")
	}

	if got := ue.NgKsi(); got.Tsc != models.ScTypeMapped || got.Ksi != int32(idleEKSI) {
		t.Errorf("ngKSI = %+v, want the eKSI %d of a mapped context", got, idleEKSI)
	}

	raw, err := amf.BuildSecurityModeCommand(ue)
	if err != nil {
		t.Fatalf("BuildSecurityModeCommand: %v", err)
	}

	smc, err := fgs.ParseSecurityModeCommand(raw)
	if err != nil {
		t.Fatalf("parse SecurityModeCommand: %v", err)
	}

	if smc.NgKSI.Value != idleEKSI || !smc.NgKSI.Mapped {
		t.Errorf("security mode command ngKSI = %+v, want the mapped %d", smc.NgKSI, idleEKSI)
	}

	if smc.SelectedEPSNASSecurityAlgorithms == nil {
		t.Fatal("no selected EPS NAS security algorithms: the UE could not derive the same mapped EPS context on its way back")
	}

	if smc.SelectedEPSNASSecurityAlgorithms.Integrity != nas.IntegrityAES ||
		smc.SelectedEPSNASSecurityAlgorithms.Ciphering != nas.CipheringAES {
		t.Errorf("selected EPS NAS algorithms = %+v, want the AES pair already in use", smc.SelectedEPSNASSecurityAlgorithms)
	}

	if ue.Ambr == nil || ue.Ambr.Uplink != models.MustParseBitRate("50 Mbps") {
		t.Errorf("AMBR = %+v, want the subscribed one the MME returned", ue.Ambr)
	}
}

// TS 33.501 §6.7.2 step 2a, §8.2
func TestIdleArrivalReplaysTheUEsOwn5GSecurityCapability(t *testing.T) {
	ue, amfInstance, _, _ := idleArrivalUE(t)

	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000001"]`,
			Ciphering:     `["NULL","AES"]`,
			Integrity:     `["AES"]`,
		},
	}

	req := idleArrivalRequest()
	req.UESecurityCapability = &fgs.UESecurityCapability{EA: 0x20, IA: 0x20}

	ue.SetUESecurityCapabilityForTest(req.UESecurityCapability)

	recoverContextFromEPS(context.Background(), amfInstance, ue, req, false)

	raw, err := amf.BuildSecurityModeCommand(ue)
	if err != nil {
		t.Fatalf("BuildSecurityModeCommand: %v", err)
	}

	smc, err := fgs.ParseSecurityModeCommand(raw)
	if err != nil {
		t.Fatalf("parse SecurityModeCommand: %v", err)
	}

	if !smc.ReplayedUESecurityCapability.Equal(*req.UESecurityCapability) {
		t.Errorf("replayed UE security capability = %+v, want the %+v the UE sent: the UE answers SECURITY MODE REJECT on a mismatch",
			smc.ReplayedUESecurityCapability, *req.UESecurityCapability)
	}

	if smc.CipheringAlgorithm == nas.CipheringNull {
		t.Error("the AMF selected NEA0 for a UE that advertised NEA2, so it negotiated against a capability set the UE never sent")
	}
}

// TS 24.501 §5.5.1.3.5 b
func TestRecoverContextFromEPSFallsBackWhenTheMMEHasNoContext(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)
	peer.MMContextErr = interworking.ErrUnknownUEContext

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), false)

	if ue.SecurityContextIsValid() {
		t.Error("a security context was installed though the MME returned none")
	}

	if ue.Conn().EPSArrival.ArrivingSessions() != nil {
		t.Error("the connection holds an arriving context the MME never returned")
	}
}

func TestRecoverContextFromEPSFallsBackOnAFailedIntegrityCheck(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)
	peer.MMContextErr = interworking.ErrIntegrityCheckFailed

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), false)

	if ue.SecurityContextIsValid() {
		t.Error("a security context was installed though the MME could not verify the TAU")
	}
}

// TS 24.501 §5.5.1.3.4 case a
func TestRecoverContextFromEPSKeepsANativeContext(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)

	ue.SetSecuredForTest(true)

	native := ue.NgKsi()

	recoverContextFromEPS(context.Background(), amfInstance, ue, nativeIdleArrivalRequest(), true)

	if got := ue.NgKsi(); got != native {
		t.Errorf("ngKSI = %+v, want the native %+v: the UE holds no mapped context to answer under", got, native)
	}

	if ue.Conn().EPSArrival.ArrivingSessions() == nil {
		t.Fatal("the PDN connections were discarded with the EPS security parameters")
	}

	if len(peer.MMContextRequests) != 1 {
		t.Error("the MME was not asked for the UE's PDN connections")
	}
}

// TS 33.501 §8.2, TS 24.501 §4.4.2.5
func TestRecoverContextFromEPSDoesNotResumeOnAnUnverifiedRegistrationRequest(t *testing.T) {
	ue, amfInstance, _, _ := idleArrivalUE(t)

	ue.SetSecuredForTest(true)

	recoverContextFromEPS(context.Background(), amfInstance, ue, nativeIdleArrivalRequest(), false)

	if !ue.Conn().ArrivalNeedsSecurityModeControl() {
		t.Fatal("the AMF resumed on a native context without verifying the registration request that claimed it")
	}

	if got := ue.NgKsi(); got.Tsc != models.ScTypeMapped {
		t.Errorf("ngKSI = %+v, want a mapped context derived from the EPS one", got)
	}
}

// TS 24.501 §4.4.2.5
func TestRecoverContextFromEPSDoesNotResumeOnAnNgKSIItDoesNotHold(t *testing.T) {
	ue, amfInstance, _, _ := idleArrivalUE(t)

	ue.SetSecuredForTest(true)

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), true)

	if !ue.Conn().ArrivalNeedsSecurityModeControl() {
		t.Fatal("the AMF resumed on a context the ngKSI in the registration request does not identify")
	}
}

// TS 23.502 §4.11.1.3.3 steps 8 and 14
func TestAdoptArrivingSessionsMovesThemAndAcksTheMME(t *testing.T) {
	ue, amfInstance, peer, smf := idleArrivalUE(t)

	resp := arrivingMMContext(ue.Supi())
	ue.Conn().EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}

	adoptArrivingSessions(context.Background(), amfInstance, ue, ue.Conn())

	if len(smf.IdleTransfers) != 1 {
		t.Fatalf("idle transfers = %d, want 1", len(smf.IdleTransfers))
	}

	if got := smf.IdleTransfers[0]; got.PDUSessionID != 3 || got.EPSBearerIdentity != 6 || got.Dnn != "internet" {
		t.Errorf("idle transfer = %+v, want PDU session 3 as EBI 6 on internet", got)
	}

	sm, ok := ue.SmContextFindByPDUSessionID(3)
	if !ok {
		t.Fatal("the arriving PDN connection opened no SM context")
	}

	if sm.Ref != "idle-ref-3" {
		t.Errorf("SM context ref = %q, want the moved session's", sm.Ref)
	}

	if ebi := ue.EPSBearerIdentities()[3]; ebi != 6 {
		t.Errorf("EPS bearer identity = %d, want the 6 it keeps on 5GS", ebi)
	}

	if !peer.Acked {
		t.Fatal("the MME was never acknowledged, so it keeps serving a UE that has left")
	}

	if len(peer.Transferred) != 1 || peer.Transferred[0] != 3 {
		t.Errorf("acknowledged sessions = %v, want the adopted PDU session 3", peer.Transferred)
	}

	if ue.Conn().EPSArrival.ArrivingSessions() != nil {
		t.Error("the arriving context outlived the adoption, so a later registration would move it again")
	}
}

// TS 23.502 §4.11.1.3.3 step 8
func TestAdoptArrivingSessionsLeavesBehindWhatCannotMove(t *testing.T) {
	ue, amfInstance, peer, smf := idleArrivalUE(t)
	smf.IdleTransferErr = context.DeadlineExceeded

	resp := arrivingMMContext(ue.Supi())
	ue.Conn().EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}

	adoptArrivingSessions(context.Background(), amfInstance, ue, ue.Conn())

	if _, ok := ue.SmContextFindByPDUSessionID(3); ok {
		t.Error("an SM context was opened for a session that never moved")
	}

	if !peer.Acked {
		t.Fatal("the MME was never acknowledged")
	}

	if len(peer.Transferred) != 0 {
		t.Errorf("acknowledged sessions = %v, want none", peer.Transferred)
	}
}

// TS 23.502 §4.11.1.3.3 step 8
func TestAdoptArrivingSessionsReleasesASessionItCannotAddress(t *testing.T) {
	ue, amfInstance, peer, smf := idleArrivalUE(t)

	resp := arrivingMMContext(ue.Supi())
	resp.PDNConnections[0].PDUSessionID = 16
	ue.Conn().EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}

	adoptArrivingSessions(context.Background(), amfInstance, ue, ue.Conn())

	if len(smf.IdleTransfers) != 1 {
		t.Fatalf("idle transfers = %d, want the session to have moved before the SM context failed", len(smf.IdleTransfers))
	}

	if len(smf.ReleaseSmContextCalls) != 1 {
		t.Fatalf("released sessions = %d, want 1: the MME dropped its PDN connection when the session moved, so nobody else releases it",
			len(smf.ReleaseSmContextCalls))
	}

	if got := smf.ReleaseSmContextCalls[0].SmContextRef; got != "idle-ref-16" {
		t.Errorf("released SM context ref = %q, want the moved session's", got)
	}

	if len(peer.Transferred) != 0 {
		t.Errorf("acknowledged sessions = %v, want none", peer.Transferred)
	}
}

// TS 23.502 §4.11.1.3.3 step 14
func TestIdleArrivalIsAbortedWhenTheIdentityCannotBeCommitted(t *testing.T) {
	ue, ngapSender, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	peer := &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}
	amfInstance.EPS = peer

	resp := arrivingMMContext(ue.Supi())
	conn := ue.Conn()
	conn.EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}
	conn.RegistrationRequest = &fgs.RegistrationRequest{
		RegistrationType: fgs.RegistrationTypeMobilityUpdating,
		UEStatus:         &fgs.UEStatus{S1ModeReg: true},
	}
	conn.MarkICSCompleted()

	ue.SetSupiForTest(etsi.SUPI{})

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(smf.IdleTransfers) != 0 {
		t.Errorf("idle transfers = %d, want none: the AMF moved sessions for a UE it could not index", len(smf.IdleTransfers))
	}

	if peer.Acked {
		t.Error("the MME was acknowledged, so it released PDN connections the AMF never adopted")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Errorf("downlink NAS messages = %d, want none: the UE was accepted on 5GS with no session while the MME still holds them all",
			len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Errorf("UE context release commands = %d, want 1: the half-built registration was left in place",
			len(ngapSender.SentUEContextReleaseCommand))
	}
}

// TS 23.502 §4.11.1.3.3 step 17
func TestIdleArrivalAcceptReportsTheAdoptedSessions(t *testing.T) {
	ue, ngapSender, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	peer := &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}
	amfInstance.EPS = peer

	resp := arrivingMMContext(ue.Supi())
	conn := ue.Conn()
	conn.EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}
	conn.RegistrationRequest = &fgs.RegistrationRequest{
		RegistrationType: fgs.RegistrationTypeMobilityUpdating,
		UEStatus:         &fgs.UEStatus{S1ModeReg: true},
		PDUSessionStatus: &fgs.PSIBitmap{PSI: [16]bool{3: true}},
	}
	conn.MarkICSCompleted()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(smf.IdleTransfers) != 1 {
		t.Fatalf("idle transfers = %d, want the arriving PDN connection to have moved", len(smf.IdleTransfers))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS transports = %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	plain := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)

	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("could not parse RegistrationAccept: %v", err)
	}

	if accept.PDUSessionStatus == nil || !accept.PDUSessionStatus.PSI[3] {
		t.Errorf("PDU session status = %+v, want PDU session 3 active", accept.PDUSessionStatus)
	}

	if accept.EPSBearerContextStatus == nil || !accept.EPSBearerContextStatus.Active[6] {
		t.Errorf("EPS bearer context status = %+v, want EBI 6 active", accept.EPSBearerContextStatus)
	}
}

// TS 33.501 §8.2
func TestIdleArrivalFromEPSAlwaysRunsTheSecurityModeProcedure(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}

	ue.SetSecuredForTest(false)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), false)

	if !ue.SecurityContextIsValid() {
		t.Fatal("no mapped context was installed")
	}

	conn := ue.Conn()
	conn.SetRegistrationType5GS(uint8(fgs.RegistrationTypeMobilityUpdating))
	conn.RegistrationRequest = idleArrivalRequest()

	securityMode(context.Background(), amfInstance, ue)

	if ue.RegStep() != amf.RegStepSecurityMode {
		t.Fatalf("registration step = %v, want the security mode sub-phase: the AMF skipped the command that activates the mapped context",
			ue.RegStep())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS messages = %d, want the SECURITY MODE COMMAND", len(ngapSender.SentDownlinkNASTransport))
	}

	wire := ngapSender.SentDownlinkNASTransport[0].NASPDU
	if len(wire) < 7 {
		t.Fatalf("downlink NAS PDU is %d octets, too short to carry a SECURITY MODE COMMAND", len(wire))
	}

	smc, err := fgs.ParseSecurityModeCommand(wire[7:])
	if err != nil {
		t.Fatalf("the AMF sent something other than a SECURITY MODE COMMAND: %v", err)
	}

	if !smc.NgKSI.Mapped {
		t.Errorf("security mode command ngKSI = %+v, want the mapped context's", smc.NgKSI)
	}
}

// TS 24.501 §4.4.2.5, §5.4.2.2
func TestIdleArrivalOnANativeContextRunsNoSecurityModeProcedure(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}

	ue.SetSecuredForTest(true)
	ue.SetDLCountForTest(7)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	native := ue.NgKsi()

	conn := ue.Conn()
	conn.SetRegistrationType5GS(uint8(fgs.RegistrationTypeMobilityUpdating))

	req := nativeIdleArrivalRequest()

	wire, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the arrival request: %v", err)
	}

	if err := handleRegistrationRequestMessage(context.Background(), amfInstance, ue, req, wire, true, false); err != nil {
		t.Fatalf("handleRegistrationRequestMessage: %v", err)
	}

	recoverContextFromEPS(context.Background(), amfInstance, ue, req, true)

	if conn.ArrivalNeedsSecurityModeControl() {
		t.Fatal("the AMF mapped the EPS context over a native one it could verify and resume on")
	}

	if conn.EPSArrival.ArrivingSessions() == nil {
		t.Fatal("the PDN connections were discarded with the EPS security parameters")
	}

	securityMode(context.Background(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS messages = %d, want the registration accept alone", len(ngapSender.SentDownlinkNASTransport))
	}

	sent := ngapSender.SentDownlinkNASTransport[0].NASPDU
	if len(sent) < 7 {
		t.Fatalf("downlink NAS PDU is %d octets, too short to be security protected", len(sent))
	}

	if sht := fgs.SecurityHeaderType(sent[1]); sht == fgs.SHTIntegrityProtectedNewContext {
		t.Error("the AMF sent a SECURITY MODE COMMAND to take into use a context the UE was already using")
	}

	if sent[6] != 7 {
		t.Errorf("the reply carries NAS sequence number %d, want the 7 the context was already at", sent[6])
	}

	if got := ue.DLCountForTest(); got != 8 {
		t.Errorf("downlink NAS COUNT = %d, want 8: the count carries on across the inter-system change", got)
	}

	if got := ue.NgKsi(); got != native {
		t.Errorf("ngKSI = %+v, want the native %+v it arrived on", got, native)
	}

	if ue.RegStep() != amf.RegStepContextSetup {
		t.Errorf("registration step = %v, want the context setup sub-phase: the registration should carry straight on", ue.RegStep())
	}
}

// TS 24.501 §5.5.1.3.4, TS 23.502 §4.11.1.3.3 step 14
func TestAnArrivalThatMovesNothingIsStillAccepted(t *testing.T) {
	ue, amfInstance, peer, smf := idleArrivalUE(t)
	smf.IdleTransferErr = context.DeadlineExceeded

	conn := ue.Conn()
	conn.EPSArrival = &amf.EPSArrival{}

	resp := arrivingMMContext(ue.Supi())
	conn.EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}

	if !adoptArrivingSessions(context.Background(), amfInstance, ue, conn) {
		t.Fatal("the registration was abandoned, so the UE gets no 5GS service until it retries")
	}

	if !peer.Acked {
		t.Error("the MME was not acknowledged, so it keeps serving a UE that has left EPS")
	}

	plain, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil,
		models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept: %v", err)
	}

	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("parse the registration accept: %v", err)
	}

	if accept.EPSBearerContextStatus == nil {
		t.Fatal("the accept carries no EPS bearer context status, so the UE keeps QoS rules for bearers that no longer exist")
	}

	for ebi, active := range accept.EPSBearerContextStatus.Active {
		if active {
			t.Errorf("EPS bearer %d reported active though no PDN connection moved onto 5GS", ebi)
		}
	}
}

// TS 24.501 §5.4.2.2
func TestASecondRegistrationDoesNotInheritTheMappedArrival(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}

	ue.SetSecuredForTest(false)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	conn := ue.Conn()
	conn.SetRegistrationType5GS(uint8(fgs.RegistrationTypeMobilityUpdating))

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest(), false)

	if !conn.ArrivalNeedsSecurityModeControl() {
		t.Fatal("the arrival did not map the EPS context, so this test proves nothing")
	}

	plain := idleArrivalRequest()
	plain.UEStatus = nil
	plain.EPSNASMessageContainer = nil

	wire, err := plain.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the second registration: %v", err)
	}

	before := ue.DLCountForTest()

	if err := handleRegistrationRequestMessage(context.Background(), amfInstance, ue, plain, wire, true, false); err != nil {
		t.Fatalf("handleRegistrationRequestMessage: %v", err)
	}

	if conn.ArrivedFromEPS() {
		t.Fatal("the second registration inherited the first one's arrival from EPS")
	}

	sentBefore := len(ngapSender.SentDownlinkNASTransport)

	securityMode(context.Background(), amfInstance, ue)

	for _, sent := range ngapSender.SentDownlinkNASTransport[sentBefore:] {
		if len(sent.NASPDU) > 1 && fgs.SecurityHeaderType(sent.NASPDU[1]) == fgs.SHTIntegrityProtectedNewContext {
			t.Error("the second registration ran a security mode procedure on a context that was already current")
		}
	}

	if got := ue.DLCountForTest(); got < before {
		t.Errorf("downlink NAS COUNT went from %d back to %d: the second registration restarted a context the UE is still counting on", before, got)
	}
}
