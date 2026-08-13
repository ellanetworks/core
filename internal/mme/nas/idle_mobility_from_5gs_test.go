// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type fakeFiveGSPeer struct {
	Requests    []interworking.EPSContextRequest
	Response    interworking.EPSContextResponse
	Err         error
	Acked       bool
	Transferred []uint8
}

func (*fakeFiveGSPeer) ForwardRelocation(context.Context, interworking.FiveGSRelocationRequest) (interworking.FiveGSRelocationResponse, error) {
	return interworking.FiveGSRelocationResponse{}, nil
}

func (*fakeFiveGSPeer) RelocationCancel(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*fakeFiveGSPeer) RelocationComplete(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (p *fakeFiveGSPeer) EPSContext(_ context.Context, req interworking.EPSContextRequest) (interworking.EPSContextResponse, error) {
	p.Requests = append(p.Requests, req)

	if p.Err != nil {
		return interworking.EPSContextResponse{}, p.Err
	}

	return p.Response, nil
}

func (p *fakeFiveGSPeer) EPSContextAck(_ context.Context, _ etsi.SUPI, transferred []uint8) error {
	p.Acked, p.Transferred = true, transferred

	return nil
}

func arrivingEPSContext(t *testing.T, algorithms interworking.EPSNASAlgorithms) interworking.EPSContextResponse {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(testSubscriber.IMSI)
	if err != nil {
		t.Fatal(err)
	}

	var kasme [32]byte
	for i := range kasme {
		kasme[i] = byte(i + 1)
	}

	return interworking.EPSContextResponse{
		SUPI: supi,
		Security: interworking.EPSSecurityContext{
			KASME:                kasme,
			EKSI:                 nas.KeySetIdentifier{Value: 4, Mapped: true},
			ULNASCount:           nas.MakeCount(0, 7),
			DLNASCount:           nas.MakeCount(0, 2),
			Algorithms:           algorithms,
			UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
		},
		PDNConnections: []interworking.PDNConnection{
			{PDUSessionID: 3, EPSBearerIdentity: 6, APN: "internet", Snssai: models.Snssai{Sst: 1, Sd: "010203"}},
		},
		AMBRUplink:   models.MustParseBitRate("50 Mbps"),
		AMBRDownlink: models.MustParseBitRate("100 Mbps"),
	}
}

func interSystemTAU(t *testing.T, mutate func(*eps.TrackingAreaUpdateRequest)) []byte {
	t.Helper()

	native := eps.GUTITypeNative

	req := &eps.TrackingAreaUpdateRequest{
		EPSUpdateType: 0,
		OldGUTI: eps.GUTIIdentity(eps.GUTI{
			PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0x0100, MMECode: 0x40,
			TMSI: [4]byte{0x00, 0x00, 0xde, 0xad},
		}),
		OldGUTIType: &native,
		UEStatus:    &eps.UEStatus{N1ModeReg: true},
	}

	if mutate != nil {
		mutate(req)
	}

	plain, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("encode TAU: %v", err)
	}

	return append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x00}, plain...)
}

func TestMovingFrom5GSInIdleModeNeedsTheMappedIdentity(t *testing.T) {
	native := eps.GUTITypeNative
	mapped := eps.GUTIType(1)

	base := func() *eps.TrackingAreaUpdateRequest {
		return &eps.TrackingAreaUpdateRequest{
			OldGUTI:     eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}}),
			OldGUTIType: &native,
			UEStatus:    &eps.UEStatus{N1ModeReg: true},
		}
	}

	if !movingFrom5GSInIdleMode(base()) {
		t.Error("a TAU with a native-typed GUTI and 5GMM-REGISTERED is not taken as an inter-system change")
	}

	noStatus := base()
	noStatus.UEStatus = nil

	if movingFrom5GSInIdleMode(noStatus) {
		t.Error("a TAU with no UE status is taken as an inter-system change")
	}

	notRegistered := base()
	notRegistered.UEStatus = &eps.UEStatus{S1ModeReg: true}

	if movingFrom5GSInIdleMode(notRegistered) {
		t.Error("a TAU reporting no 5GMM registration is taken as an inter-system change")
	}

	mappedType := base()
	mappedType.OldGUTIType = &mapped

	if movingFrom5GSInIdleMode(mappedType) {
		t.Error("a TAU whose Old GUTI is typed mapped is taken as an inter-system change from 5GS")
	}
}

func idleArrivalMME(t *testing.T, peer *fakeFiveGSPeer) (*mme.MME, *mme.UeConn, *captureConn) {
	t.Helper()

	m := newTestMME(t)
	m.FiveGS = peer

	cc := &captureConn{}

	conn := m.NewUeConn(cc, 7)
	conn.ServingTAI = servedAttachTAI

	return m, conn, cc
}

// TS 33.501 §8.5.2 steps 3-6
func TestInterSystemTAURecoversTheContextFromTheAMF(t *testing.T) {
	peer := &fakeFiveGSPeer{Response: arrivingEPSContext(t, interworking.EPSNASAlgorithms{
		Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES,
	})}

	m, conn, _ := idleArrivalMME(t, peer)

	pdu := interSystemTAU(t, nil)

	if got := dispositionForNAS(context.Background(), m, conn, pdu); got.Reason == nasreply.ReasonNoContext {
		t.Fatalf("the update was dropped: %+v", got)
	}

	if len(peer.Requests) != 1 {
		t.Fatalf("context requests = %d, want 1", len(peer.Requests))
	}

	if got := peer.Requests[0].Mapped5GGUTI.TMSI; got != [4]byte{0x00, 0x00, 0xde, 0xad} {
		t.Errorf("the AMF was asked about 5G-TMSI %x, want the one the Old GUTI maps back to", got)
	}

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound, so the UE would be told to re-attach")
	}

	if !ue.Secured() {
		t.Error("the mapped EPS security context was not installed")
	}

	if !peer.Acked {
		t.Fatal("the AMF was never acknowledged, so it keeps serving a UE that has left")
	}

	if len(peer.Transferred) != 1 || peer.Transferred[0] != 3 {
		t.Errorf("acknowledged sessions = %v, want the adopted PDU session 3", peer.Transferred)
	}

	if p := m.LookupPDN(ue, 6); p == nil {
		t.Error("the arriving PDU session was not published as a PDN connection on EBI 6")
	}
}

// TS 24.301 §5.5.3.2.5
func TestInterSystemTAUWithoutARecoverableContext(t *testing.T) {
	peer := &fakeFiveGSPeer{Err: interworking.ErrUnknownUEContext}
	m, conn, _ := idleArrivalMME(t, peer)

	got := dispositionForNAS(context.Background(), m, conn, interSystemTAU(t, nil))
	if got.Reason != nasreply.ReasonNoContext {
		t.Fatalf("disposition = %+v, want the connection left bare", got)
	}

	if conn.UeContext() != nil {
		t.Error("a context was minted for an update no peer vouched for")
	}
}

func TestInterSystemTAUWithAFailedIntegrityCheck(t *testing.T) {
	peer := &fakeFiveGSPeer{Err: interworking.ErrIntegrityCheckFailed}
	m, conn, _ := idleArrivalMME(t, peer)

	if got := dispositionForNAS(context.Background(), m, conn, interSystemTAU(t, nil)); got.Reason != nasreply.ReasonNoContext {
		t.Fatalf("disposition = %+v, want the connection left bare", got)
	}

	if conn.UeContext() != nil {
		t.Error("a context was minted for an update the AMF could not verify")
	}
}

func TestOrdinaryTAUOnABareConnectionMintsNothing(t *testing.T) {
	peer := &fakeFiveGSPeer{Response: arrivingEPSContext(t, interworking.EPSNASAlgorithms{
		Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES,
	})}

	m, conn, _ := idleArrivalMME(t, peer)

	pdu := interSystemTAU(t, func(req *eps.TrackingAreaUpdateRequest) {
		req.UEStatus = nil
	})

	if got := dispositionForNAS(context.Background(), m, conn, pdu); got.Reason != nasreply.ReasonNoContext {
		t.Fatalf("disposition = %+v, want the connection left bare", got)
	}

	if len(peer.Requests) != 0 {
		t.Error("the AMF was asked about an update that is not an inter-system change")
	}
}

// TS 33.501 §8.5.2 steps 7-10
func TestInterSystemTAURekeysOnAnAlgorithmChange(t *testing.T) {
	peer := &fakeFiveGSPeer{Response: arrivingEPSContext(t, interworking.EPSNASAlgorithms{
		Ciphering: nas.CipheringNull, Integrity: nas.IntegritySNOW3G,
	})}

	m, conn, cc := idleArrivalMME(t, peer)

	dispositionForNAS(context.Background(), m, conn, interSystemTAU(t, nil))

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound")
	}

	if len(cc.sent) == 0 {
		t.Fatal("the MME sent nothing, want a SECURITY MODE COMMAND before the update is answered")
	}

	if ue.RegStep() != mme.RegStepSecurityMode {
		t.Errorf("registration step = %v, want the security mode sub-phase", ue.RegStep())
	}
}
