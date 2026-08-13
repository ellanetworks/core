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

// TS 24.301 §4.4.4.3: a subscriber maps to exactly one context. Without the
// commit the arriving context is never indexed by SUPI, so the 5GS peer's later
// context acknowledgement finds nothing and this MME keeps PDN connections for a
// UE that has left (TS 23.502 §4.11.1.3.3 step 14).
func TestInterSystemTAUIndexesTheArrivingContextBySubscriber(t *testing.T) {
	peer := &fakeFiveGSPeer{Response: arrivingEPSContext(t, interworking.EPSNASAlgorithms{
		Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES,
	})}

	m, conn, _ := idleArrivalMME(t, peer)

	supi, err := etsi.NewSUPIFromIMSI(testSubscriber.IMSI)
	if err != nil {
		t.Fatal(err)
	}

	stale := mme.NewUeContext()
	stale.SetSupi(supi)
	m.CommitUEIdentity(context.Background(), stale, mme.MintAuthProofForInterworking())

	dispositionForNAS(context.Background(), m, conn, interSystemTAU(t, nil))

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound")
	}

	held, ok := m.LookupUeBySupi(supi)
	if !ok {
		t.Fatal("the subscriber resolves to no context, so the 5GS peer cannot acknowledge a later move back")
	}

	if held == stale {
		t.Error("the subscriber still resolves to the context it held before the move")
	}

	if held != ue {
		t.Errorf("the subscriber resolves to a context other than the one serving the update")
	}

	if err := m.MMContextAck(context.Background(), supi, nil); err != nil {
		t.Errorf("MMContextAck: %v: a move back to 5GS would leave this MME serving PDN connections the UE has left", err)
	}
}

// TS 23.401 §5.3.3.1 step 8, performed by TS 23.502 §4.11.1.3.2 steps 7-14:
// with no bearer context at all the MME rejects the update rather than telling
// the UE its connectivity survived. The 5GS peer keeps what it still holds
// (TS 23.401 §5.3.3.1 step 7).
func TestInterSystemTAUWithNothingToAdoptIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name  string
		conns []interworking.PDNConnection
	}{
		{
			name: "the offered session has no subscription on this MME",
			conns: []interworking.PDNConnection{
				{PDUSessionID: 3, EPSBearerIdentity: 6, APN: "unsubscribed", Snssai: models.Snssai{Sst: 1, Sd: "010203"}},
			},
		},
		{name: "the 5GS peer offered no session at all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := arrivingEPSContext(t, interworking.EPSNASAlgorithms{
				Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES,
			})
			resp.PDNConnections = tc.conns

			peer := &fakeFiveGSPeer{Response: resp}
			m, conn, cc := idleArrivalMME(t, peer)

			dispositionForNAS(context.Background(), m, conn, interSystemTAU(t, nil))

			ue := conn.UeContext()
			if ue == nil {
				t.Fatal("no context was bound")
			}

			if cc.count() == 0 {
				t.Fatal("the MME sent nothing, want a TRACKING AREA UPDATE REJECT")
			}

			plain := decodeProtectedDownlink(t, ue, cc.sent[0])

			rej, err := eps.ParseTrackingAreaUpdateReject(plain)
			if err != nil {
				t.Fatalf("the MME answered something other than a reject, so the UE believes its bearers survived: %v", err)
			}

			if rej.Cause != eps.EMMCauseImplicitlyDetached {
				t.Errorf("reject cause = %d, want #%d so the UE attaches afresh", rej.Cause, eps.EMMCauseImplicitlyDetached)
			}

			if peer.Acked {
				t.Error("the 5GS peer was acknowledged for a transfer that moved nothing, so it released sessions the UE still needs")
			}
		})
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

// TS 24.301 §4.4.3.1, §5.4.3.2; TS 33.401 §6.5. A context mapped from 5GS
// inherits the 5G NAS COUNTs; changing its algorithms re-keys it under the same
// K'ASME, which is not a new context, so neither side restarts the counters. The
// UE keeps its uplink NAS COUNT because the eKSI matches its current mapped
// context (TS 24.301 §5.4.3.3).
func TestInterSystemTAURekeyKeepsTheMappedNASCounts(t *testing.T) {
	peer := &fakeFiveGSPeer{Response: arrivingEPSContext(t, interworking.EPSNASAlgorithms{
		Ciphering: nas.CipheringNull, Integrity: nas.IntegritySNOW3G,
	})}

	m, conn, cc := idleArrivalMME(t, peer)

	dispositionForNAS(context.Background(), m, conn, interSystemTAU(t, nil))

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound")
	}

	if cc.count() == 0 {
		t.Fatal("the MME sent nothing, want a SECURITY MODE COMMAND")
	}

	if got := ue.ULCount(); got != 8 {
		t.Errorf("next expected uplink NAS COUNT = %d, want 8 after the inherited 7: the MME would reject the Security Mode Complete the UE sends", got)
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])
	if len(wire) < 6 {
		t.Fatalf("downlink NAS message is %d octets, too short to be security protected", len(wire))
	}

	if wire[5] != 2 {
		t.Errorf("Security Mode Command rides NAS sequence number %d, want the inherited downlink COUNT 2", wire[5])
	}

	plain := decodeProtectedDownlink(t, ue, cc.sent[0])
	if _, err := eps.ParseSecurityModeCommand(plain); err != nil {
		t.Fatalf("the re-keyed Security Mode Command does not verify under the new algorithms: %v", err)
	}
}

// TS 33.401 §7.2.4.4, TS 24.301 §5.4.3.3. The UE checks the replay against what
// it sent, and §8.2.29.7 makes the UE network capability IE mandatory in every
// non-periodic TRACKING AREA UPDATE REQUEST, so the peer's relayed copy is never
// the one to replay.
func TestInterSystemTAURekeyReplaysTheCapabilitiesTheUEJustSent(t *testing.T) {
	peer := &fakeFiveGSPeer{Response: arrivingEPSContext(t, interworking.EPSNASAlgorithms{
		Ciphering: nas.CipheringNull, Integrity: nas.IntegritySNOW3G,
	})}

	m, conn, cc := idleArrivalMME(t, peer)

	sent := eps.UENetworkCapability{EEA: 0x20, EIA: 0x20, HasUMTS: true, UEA: 0x40, UIA: 0x40}

	ms, err := eps.ParseMSNetworkCapability([]byte{0x80, 0x00})
	if err != nil {
		t.Fatalf("ParseMSNetworkCapability: %v", err)
	}

	pdu := interSystemTAU(t, func(req *eps.TrackingAreaUpdateRequest) {
		req.UENetworkCapability = &sent
		req.MSNetworkCapability = &ms
	})

	dispositionForNAS(context.Background(), m, conn, pdu)

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound")
	}

	if cc.count() == 0 {
		t.Fatal("the MME sent nothing, want a SECURITY MODE COMMAND")
	}

	smc, err := eps.ParseSecurityModeCommand(decodeProtectedDownlink(t, ue, cc.sent[0]))
	if err != nil {
		t.Fatalf("parse Security Mode Command: %v", err)
	}

	want := eps.ReplayedUESecurityCapability(sent, &ms)
	if smc.ReplayedUESecurityCapability != want {
		t.Errorf("replayed UE security capability = %+v, want the %+v derived from the update's own IEs: the UE answers Security Mode Reject on a mismatch",
			smc.ReplayedUESecurityCapability, want)
	}
}
