// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// TestTrackingAreaUpdateTrackingAreaNotAllowed checks a TAU from a serving cell outside
// the served area is rejected with TAU REJECT #12 (TS 24.301 §5.5.3.2.5).
func TestTrackingAreaUpdateTrackingAreaNotAllowed(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	// Served PLMN 001/01 but TAC 2, which the operator does not serve (it serves TAC 1).
	ue.Conn().ServingTAI = s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 2}

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) == 0 {
		t.Fatal("expected a TAU Reject, got no downlink")
	}

	rej, err := eps.ParseTrackingAreaUpdateReject(decodeProtectedDownlink(t, ue, cc.sent[0]))
	if err != nil {
		t.Fatalf("not a TAU Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseTrackingAreaNotAllowed {
		t.Fatalf("TAU Reject cause = %d, want %d", rej.Cause, eps.EMMCauseTrackingAreaNotAllowed)
	}
}

// tauGUTI is the Old GUTI every TAU request carries.
var tauGUTI = eps.GUTIIdentity(eps.GUTI{
	PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 0, TMSI: [4]byte{0, 0, 0, 1},
})

// tauRequest is a TRACKING AREA UPDATE REQUEST of the given EPS update type.
func tauRequest(updateType eps.EPSUpdateType) *eps.TrackingAreaUpdateRequest {
	return &eps.TrackingAreaUpdateRequest{EPSUpdateType: updateType, OldGUTI: tauGUTI}
}

// tauRequestActive is a TRACKING AREA UPDATE REQUEST with the active flag set,
// which asks the network to re-establish the radio bearers (TS 24.301 §9.9.3.14).
func tauRequestActive(updateType eps.EPSUpdateType) *eps.TrackingAreaUpdateRequest {
	return &eps.TrackingAreaUpdateRequest{EPSUpdateType: updateType, ActiveFlag: true, OldGUTI: tauGUTI}
}

// handleTAU dispatches a TRACKING AREA UPDATE REQUEST together with the octets it
// encodes to, which the handler keeps as the duplicate-detection oracle
// (TS 24.301 §5.5.3.2.7 case d).
func handleTAU(t *testing.T, m *mme.MME, ue *mme.UeContext, req *eps.TrackingAreaUpdateRequest) {
	t.Helper()

	plain, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("encode TAU Request: %v", err)
	}

	handleTrackingAreaUpdate(context.Background(), m, ue, ue.Conn(), req, plain)
}

// tauOldGUTI is the mandatory Old GUTI (EPS mobile identity, LV) every TAU
// REQUEST carries right after the update-type octet (TS 24.301 table 8.2.29.1).
var tauOldGUTI = []byte{0x0b, 0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}

// trackingAreaUpdateNAS builds a protected TRACKING AREA UPDATE REQUEST at the
// UE's current uplink NAS COUNT, optionally carrying an EPS bearer context status
// IE (IEI 0x57) when bearerStatus is non-nil.
func trackingAreaUpdateNAS(t *testing.T, ue *mme.UeContext, bearerStatus *uint16) []byte {
	t.Helper()

	plain := []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x00}
	plain = append(plain, tauOldGUTI...)

	if bearerStatus != nil {
		plain = append(plain, 0x57, 0x02, byte(*bearerStatus), byte(*bearerStatus>>8))
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

// TestTrackingAreaUpdateConnectedAccepted checks that a periodic TAU from a
// connected UE is accepted over Downlink NAS Transport and the UE stays
// registered and connected (TS 24.301 §5.5.3.2.4).
func TestTrackingAreaUpdateConnectedAccepted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	if mt, err := eps.PeekMessageType(accept); err != nil || mt != eps.MsgTrackingAreaUpdateAccept {
		t.Fatalf("downlink message = %#x (err %v), want TAU Accept", mt, err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.TAIList == nil {
		t.Fatal("TAU Accept is missing the TAI list (TS 24.301 §5.5.3.2.4)")
	}

	if parsed.Cause != nil {
		t.Fatalf("EPS-only TAU Accept carries EMM cause #%d, want none", *parsed.Cause)
	}

	if ue.EMMState() != mme.EMMRegistered || !ue.Connected() {
		t.Fatal("UE should remain registered and connected after a periodic TAU")
	}
}

// An identical TAU retransmission resends the stored accept byte-for-byte, so no
// second GUTI is reallocated (TS 24.301 §5.5.3.2.7 case d).
func TestTrackingAreaUpdateDuplicateResendsAccept(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m) // ECM-CONNECTED, secured, EMM-REGISTERED

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))
	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) != 2 {
		t.Fatalf("expected two downlinks (TAU Accept and its resend), got %d", len(cc.sent))
	}

	var plains [2][]byte

	for i := range plains {
		dl := decodeDownlinkNAS(t, cc.sent[i])

		plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink,
			mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
		if err != nil {
			t.Fatalf("unprotect downlink %d: %v", i, err)
		}

		if mt, err := eps.PeekMessageType(plain); err != nil || mt != eps.MsgTrackingAreaUpdateAccept {
			t.Fatalf("downlink %d message = %#x (err %v), want TAU Accept", i, mt, err)
		}

		plains[i] = plain
	}

	if !bytes.Equal(plains[0], plains[1]) {
		t.Fatal("resent TAU Accept differs from the original; a duplicate TAU REQUEST must resend the stored accept")
	}
}

// TestTrackingAreaUpdateReconcilesBearerContextStatus checks that when the UE
// reports its EPS bearer context status, the MME deactivates locally the bearers
// the UE marks inactive and reflects the resulting active set in the TAU Accept
// (TS 24.301 §5.5.3.2.4). Default bearer EBI 5 stays; additional bearer EBI 6 is
// released.
func TestTrackingAreaUpdateReconcilesBearerContextStatus(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m) // ECM-CONNECTED, secured, EMM-REGISTERED

	m.AddDefaultPDN(ue) // EBI 5
	ue.EnsurePDN(6)     // an additional PDN connection

	status := uint16(1 << 5) // the UE reports only EBI 5 active
	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, &status))

	if _, ok := ue.Pdns[6]; ok {
		t.Fatal("EBI 6 should be released locally after the UE reports it inactive")
	}

	if _, ok := ue.Pdns[5]; !ok {
		t.Fatal("EBI 5 should remain active")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	var wantStatus nas.EPSBearerContextStatus

	wantStatus.Active[5] = true

	if parsed.EPSBearerContextStatus == nil || *parsed.EPSBearerContextStatus != wantStatus {
		t.Fatalf("TAU Accept bearer status = %v, want only EBI 5", parsed.EPSBearerContextStatus)
	}
}

// TS 24.301 §5.5.3.2.4
func TestTrackingAreaUpdateOmitsTheBearerStatusWithNoBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	status := uint16(1 << 5)
	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, &status))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	if parsed := parseTAUAccept(t, ue, cc.sent[0]); parsed.EPSBearerContextStatus != nil {
		t.Fatalf("TAU Accept bearer status = %v, want the IE omitted for a UE with no bearer", parsed.EPSBearerContextStatus)
	}
}

func TestTrackingAreaUpdateOmitsTheBearerStatusWhenNoneWasAsked(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.AddDefaultPDN(ue)

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	if parsed := parseTAUAccept(t, ue, cc.sent[0]); parsed.EPSBearerContextStatus != nil {
		t.Fatalf("TAU Accept bearer status = %v, want the IE omitted: the request carried none and nothing was deactivated",
			parsed.EPSBearerContextStatus)
	}
}

// TS 24.301 §5.5.3.2.4
func TestTrackingAreaUpdateReportsALocalDeactivation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.AddDefaultPDN(ue)

	dropped := ue.EnsurePDN(6)
	m.ReleasePDN(context.Background(), ue, dropped)

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	var want nas.EPSBearerContextStatus

	want.Active[5] = true

	parsed := parseTAUAccept(t, ue, cc.sent[0])
	if parsed.EPSBearerContextStatus == nil {
		t.Fatal("TAU Accept carries no bearer status, so the UE keeps sending on a bearer the MME dropped")
	}

	if *parsed.EPSBearerContextStatus != want {
		t.Fatalf("TAU Accept bearer status = %v, want only EBI 5", parsed.EPSBearerContextStatus)
	}
}

func TestTrackingAreaUpdateReportsALocalDeactivationOnce(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	ue.Conn().ICS = mme.ICSCompleted // stay connected across both updates

	m.AddDefaultPDN(ue)

	dropped := ue.EnsurePDN(6)
	m.ReleasePDN(context.Background(), ue, dropped)

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) != 2 {
		t.Fatalf("expected a second TAU Accept, got %d downlinks", len(cc.sent))
	}

	if parsed := parseTAUAccept(t, ue, cc.sent[1]); parsed.EPSBearerContextStatus != nil {
		t.Fatalf("TAU Accept bearer status = %v, want the IE omitted once the deactivation was reported",
			parsed.EPSBearerContextStatus)
	}
}

func parseTAUAccept(t *testing.T, ue *mme.UeContext, sent []byte) *eps.TrackingAreaUpdateAccept {
	t.Helper()

	dl := decodeDownlinkNAS(t, sent)

	accept, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink,
		mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	return parsed
}

// TS 24.301 §8.2.26.8, §5.5.3.3.4.3
func TestTrackingAreaUpdateCombinedSignalsCSDomainUnavailable(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m) // ECM-CONNECTED, secured, EMM-REGISTERED

	handleTAU(t, m, ue, tauRequest(2))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.Cause == nil || *parsed.Cause != eps.EMMCauseCSDomainNotAvailable {
		t.Fatalf("EMM cause = %v, want #%d (CS domain not available)", parsed.Cause, eps.EMMCauseCSDomainNotAvailable)
	}
}

// TS 24.301 §5.5.3.2.4
func TestTrackingAreaUpdateReallocatesGUTI(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code, err := m.MmeIdentity(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.ReallocateGUTI(t.Context(), ue, plmn, group, code); err != nil {
		t.Fatal(err)
	}

	oldMTMSI := ue.TmsiForTest()

	handleTAU(t, m, ue, tauRequest(3)) // periodic

	if ue.OldTmsiForTest() != oldMTMSI || ue.TmsiForTest() == oldMTMSI {
		t.Fatalf("GUTI not reallocated: mtmsi=%d oldMTMSI=%d (was %d)", ue.TmsiForTest(), ue.OldTmsiForTest(), oldMTMSI)
	}

	if _, ok := m.LookupUeByMTMSI(oldMTMSI); !ok {
		t.Fatal("old M-TMSI must stay resolvable until TAU Complete")
	}

	if _, ok := m.LookupUeByMTMSI(ue.TmsiForTest()); !ok {
		t.Fatal("new M-TMSI not resolvable")
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(plain)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.GUTI == nil {
		t.Fatalf("TAU Accept GUTI absent")
	}

	if parsed.GUTI.GUTI == nil || binary.BigEndian.Uint32(parsed.GUTI.GUTI.TMSI[:]) != ue.TmsiForTest() {
		t.Fatalf("TAU Accept GUTI = %+v, want M-TMSI %d", parsed.GUTI, ue.TmsiForTest())
	}

	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())

	if !ue.OldTmsiUnsetForTest() {
		t.Fatal("reallocation not committed after TAU Complete")
	}

	if _, ok := m.LookupUeByMTMSI(oldMTMSI); ok {
		t.Fatal("old M-TMSI still resolvable after TAU Complete")
	}

	if _, ok := m.LookupUeByMTMSI(ue.TmsiForTest()); !ok {
		t.Fatal("new M-TMSI lost after TAU Complete")
	}
}

// TS 24.301 §5.5.3.2.4
func TestTrackingAreaUpdateIdleNoActiveFlagReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.SetTmsiForTest(1) // a GUTI to reallocate

	handleTAU(t, m, ue, tauRequest(0))

	if len(cc.sent) != 1 {
		t.Fatalf("expected only a TAU Accept before TAU Complete, got %d", len(cc.sent))
	}

	if !ue.Connected() {
		t.Fatal("UE not ECM-CONNECTED for the TAU exchange; TAU Complete would be rejected")
	}

	if ue.OldTmsiUnsetForTest() {
		t.Fatal("GUTI reallocation not pending after TAU Accept")
	}

	handleTrackingAreaUpdateComplete(context.Background(), m, ue, ue.Conn())

	if !ue.OldTmsiUnsetForTest() {
		t.Fatal("old M-TMSI not freed after TAU Complete")
	}

	if len(cc.sent) != 2 {
		t.Fatalf("expected a UE Context Release Command after TAU Complete, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[1])

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatal("UE should remain EMM-REGISTERED after a periodic TAU")
	}
}

// TS 24.301 §5.5.3.2.4
func TestTrackingAreaUpdateIdleActiveFlagReestablishes(t *testing.T) {
	m := newTestMME(t)
	ue, _ := idleRegisteredUE(t, m)
	cc := &captureConn{}
	establishResumeForTest(m, ue, cc, 9) // the resume re-binds the connection

	handleTAU(t, m, ue, tauRequestActive(0))

	if !ue.Connected() {
		t.Fatal("UE not ECM-CONNECTED after an active-flag TAU")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected Initial Context Setup Request, got %d S1AP messages", len(cc.sent))
	}

	parseInitialContextSetup(t, cc.sent[0])
}

// TTS 24.301 §5.5.3.2.5
func TestTrackingAreaUpdateRecovery(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	pdu := []byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x01, 0x07, byte(eps.MsgTrackingAreaUpdateRequest)}

	mmes1ap.HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(cc), initiatingValue(t, initialUEMessagePDU(t, 7, pdu)))

	if len(cc.sent) != 2 {
		t.Fatalf("expected a TAU Reject and a UE Context Release Command, got %d messages", len(cc.sent))
	}

	rej, err := eps.ParseTrackingAreaUpdateReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not a TAU Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseUEIdentityCannotBeDerived {
		t.Fatalf("TAU Reject cause = %d, want %d", rej.Cause, eps.EMMCauseUEIdentityCannotBeDerived)
	}

	cmd := parseUEContextReleaseCommandPDU(t, cc.sent[1])
	if cmd.UES1APIDs.ENBUES1APID != 7 {
		t.Fatalf("release command names eNB-UE-S1AP-ID %d, want the rejected connection's 7", cmd.UES1APIDs.ENBUES1APID)
	}

	if m.ConnCountForTest() != 1 {
		t.Fatalf("the MME-UE-S1AP-ID was freed before the Release Complete: %d connections remain", m.ConnCountForTest())
	}

	complete := &s1ap.UEContextReleaseComplete{
		MMEUES1APID: s1ap.Ptr(cmd.UES1APIDs.MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(cmd.UES1APIDs.ENBUES1APID),
	}

	b, err := complete.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	cpdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	mmes1ap.HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), cpdu.(*s1ap.SuccessfulOutcome).Value)

	if m.ConnCountForTest() != 0 {
		t.Fatalf("bare connection not released after the Release Complete: %d remain", m.ConnCountForTest())
	}
}

func parseUEContextReleaseCommandPDU(t *testing.T, pdu []byte) *s1ap.UEContextReleaseCommand {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected a UE Context Release Command, got %T", msg)
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(im.Value)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	return cmd
}

func TestTrackingAreaUpdateStoresReplayedCapabilities(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	replayed := eps.UENetworkCapability{EEA: 0xe0, EIA: 0x60, HasUMTS: true, UEA: 0xc0, UIA: 0x40, Rest: []byte{0x00, 0x80, 0x20}}
	msNetCap := eps.MSNetworkCapability{Rest: []byte{0xe5, 0xe0, 0x00}}

	req := tauRequest(eps.EPSUpdateTypeTA)
	req.UENetworkCapability = &replayed
	req.MSNetworkCapability = &msNetCap

	handleTAU(t, m, ue, req)

	if got := ue.UeNetCap(); got.EEA != replayed.EEA || got.EIA != replayed.EIA {
		t.Errorf("stored UE network capability = EEA %#08b EIA %#08b, want EEA %#08b EIA %#08b",
			uint8(got.EEA), uint8(got.EIA), uint8(replayed.EEA), uint8(replayed.EIA))
	}

	stored := ue.MsNetCap()
	if stored == nil {
		t.Fatal("MS network capability not stored, want the replayed one")
	}

	if !bytes.Equal(stored.Rest, msNetCap.Rest) {
		t.Errorf("stored MS network capability = %x, want %x", stored.Rest, msNetCap.Rest)
	}
}

func TestTrackingAreaUpdateKeepsHeldMSCapability(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	held := eps.MSNetworkCapability{Rest: []byte{0xe5, 0xe0, 0x00}}
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}, &held, mme.MintAuthProofForAttachRequest())

	req := tauRequest(eps.EPSUpdateTypeTA)
	req.UENetworkCapability = &eps.UENetworkCapability{EEA: 0xe0, EIA: 0x60}

	handleTAU(t, m, ue, req)

	stored := ue.MsNetCap()
	if stored == nil {
		t.Fatal("held MS network capability dropped, want it kept")
	}

	if !bytes.Equal(stored.Rest, held.Rest) {
		t.Errorf("stored MS network capability = %x, want the held %x", stored.Rest, held.Rest)
	}
}

func TestTrackingAreaUpdateWithoutCapabilityKeepsStored(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	held := eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}
	ue.SetUESecurityCapability(held, nil, mme.MintAuthProofForAttachRequest())

	handleTAU(t, m, ue, tauRequest(eps.EPSUpdateTypeTA))

	if got := ue.UeNetCap(); got.EEA != held.EEA || got.EIA != held.EIA {
		t.Errorf("stored UE network capability = EEA %#08b EIA %#08b, want the held EEA %#08b EIA %#08b",
			uint8(got.EEA), uint8(got.EIA), uint8(held.EEA), uint8(held.EIA))
	}
}

func TestTrackingAreaUpdateStoresAnMSCapabilityOnlyReplay(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	held := eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}
	ue.SetUESecurityCapability(held, nil, mme.MintAuthProofForAttachRequest())

	replayed := eps.MSNetworkCapability{Rest: []byte{0xe5, 0xe0, 0x00}}

	req := tauRequest(eps.EPSUpdateTypeTA)
	req.MSNetworkCapability = &replayed

	handleTAU(t, m, ue, req)

	stored := ue.MsNetCap()
	if stored == nil {
		t.Fatal("MS network capability not stored, want the replayed one")
	}

	if !bytes.Equal(stored.Rest, replayed.Rest) {
		t.Errorf("stored MS network capability = %x, want %x", stored.Rest, replayed.Rest)
	}

	if got := ue.UeNetCap(); got.EEA != held.EEA || got.EIA != held.EIA {
		t.Errorf("stored UE network capability = EEA %#08b EIA %#08b, want the held EEA %#08b EIA %#08b",
			uint8(got.EEA), uint8(got.EIA), uint8(held.EEA), uint8(held.EIA))
	}
}

// TS 24.301 §5.5.3.2.4
func TestTrackingAreaUpdateKeepsTheLastPDNReportedInactive(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.AddDefaultPDN(ue)

	status := uint16(0)
	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, &status))

	if _, ok := ue.Pdns[5]; !ok {
		t.Error("the UE's only PDN connection was released: this MME does not advertise EMM-REGISTERED without PDN connection, so §5.5.3.2.4 does not permit deactivating the last one")
	}

	if ue.EMMState() != mme.EMMRegistered {
		t.Errorf("EMM state = %v, want EMM-REGISTERED: the UE asked for a tracking area update, not a detach", ue.EMMState())
	}

	if len(cc.sent) != 1 {
		t.Fatalf("sent %d messages, want only the TAU Accept", len(cc.sent))
	}

	decodeDownlinkNAS(t, cc.sent[0])
}
