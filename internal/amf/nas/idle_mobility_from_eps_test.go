// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

const idleEKSI uint8 = 4

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

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest())

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

// TS 24.501 §5.5.1.3.5 b
func TestRecoverContextFromEPSFallsBackWhenTheMMEHasNoContext(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)
	peer.MMContextErr = interworking.ErrUnknownUEContext

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest())

	if ue.SecurityContextIsValid() {
		t.Error("a security context was installed though the MME returned none")
	}

	if ue.Conn().ArrivingFromEPS != nil {
		t.Error("the connection holds an arriving context the MME never returned")
	}
}

func TestRecoverContextFromEPSFallsBackOnAFailedIntegrityCheck(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)
	peer.MMContextErr = interworking.ErrIntegrityCheckFailed

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest())

	if ue.SecurityContextIsValid() {
		t.Error("a security context was installed though the MME could not verify the TAU")
	}
}

// TS 24.501 §5.5.1.3.4 case a
func TestRecoverContextFromEPSKeepsANativeContext(t *testing.T) {
	ue, amfInstance, peer, _ := idleArrivalUE(t)

	ue.SetSecuredForTest(true)

	native := ue.NgKsi()

	recoverContextFromEPS(context.Background(), amfInstance, ue, idleArrivalRequest())

	if got := ue.NgKsi(); got != native {
		t.Errorf("ngKSI = %+v, want the native %+v: the UE holds no mapped context to answer under", got, native)
	}

	if ue.Conn().ArrivingFromEPS == nil {
		t.Fatal("the PDN connections were discarded with the EPS security parameters")
	}

	if len(peer.MMContextRequests) != 1 {
		t.Error("the MME was not asked for the UE's PDN connections")
	}
}

// TS 23.502 §4.11.1.3.3 steps 8 and 14
func TestAdoptArrivingSessionsMovesThemAndAcksTheMME(t *testing.T) {
	ue, amfInstance, peer, smf := idleArrivalUE(t)

	resp := arrivingMMContext(ue.Supi())
	ue.Conn().ArrivingFromEPS = &resp

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

	if ue.Conn().ArrivingFromEPS != nil {
		t.Error("the arriving context outlived the adoption, so a later registration would move it again")
	}
}

// TS 23.502 §4.11.1.3.3 step 8
func TestAdoptArrivingSessionsLeavesBehindWhatCannotMove(t *testing.T) {
	ue, amfInstance, peer, smf := idleArrivalUE(t)
	smf.IdleTransferErr = context.DeadlineExceeded

	resp := arrivingMMContext(ue.Supi())
	ue.Conn().ArrivingFromEPS = &resp

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

// TS 23.502 §4.11.1.3.3 step 17
func TestIdleArrivalAcceptReportsTheAdoptedSessions(t *testing.T) {
	ue, ngapSender, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	peer := &fakeEPSPeer{MMContextResponse: arrivingMMContext(ue.Supi())}
	amfInstance.EPS = peer

	resp := arrivingMMContext(ue.Supi())
	conn := ue.Conn()
	conn.ArrivingFromEPS = &resp
	conn.ArrivedFromEPS = true
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
