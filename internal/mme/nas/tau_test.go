// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// trackingAreaUpdateNAS builds a protected TRACKING AREA UPDATE REQUEST at the
// UE's current uplink NAS COUNT, optionally carrying an EPS bearer context status
// IE (IEI 0x57) when bearerStatus is non-nil.
func trackingAreaUpdateNAS(t *testing.T, ue *mme.UeContext, activeFlag bool, bearerStatus *uint16) []byte {
	t.Helper()

	updateType := uint8(0)
	if activeFlag {
		updateType |= 0x08
	}

	plain := []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), updateType}

	if bearerStatus != nil {
		plain = append(plain, 0x57, 0x02, byte(*bearerStatus), byte(*bearerStatus>>8))
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.ULCount())),
		nascommon.DirectionUplink, ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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
	ue, cc := securedUE(t, m) // ECM-CONNECTED, secured, EMM-REGISTERED

	HandleNAS(m, context.Background(), ue.Conn(), trackingAreaUpdateNAS(t, ue, false, nil))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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

	if len(parsed.TAIList) == 0 {
		t.Fatal("TAU Accept is missing the TAI list (TS 24.301 §5.5.3.2.4)")
	}

	if parsed.EMMCause != nil {
		t.Fatalf("EPS-only TAU Accept carries EMM cause #%d, want none", *parsed.EMMCause)
	}

	if ue.EMMState() != mme.EMMRegistered || !ue.Connected() {
		t.Fatal("UE should remain registered and connected after a periodic TAU")
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
	HandleNAS(m, context.Background(), ue.Conn(), trackingAreaUpdateNAS(t, ue, false, &status))

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

	accept, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.EPSBearerContextStatus == nil || *parsed.EPSBearerContextStatus != uint16(1<<5) {
		t.Fatalf("TAU Accept bearer status = %v, want only EBI 5 (%#x)", parsed.EPSBearerContextStatus, uint16(1<<5))
	}
}

// TestTrackingAreaUpdateCombinedSignalsCSDomainUnavailable checks that a
// combined TAU (the UE also requesting CS-domain registration) is accepted for
// EPS services only with EMM cause #18, so the UE stops attempting CS
// registration (TS 24.301 §8.2.26.8, §5.5.3.3.4.3).
func TestTrackingAreaUpdateCombinedSignalsCSDomainUnavailable(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m) // ECM-CONNECTED, secured, EMM-REGISTERED

	// EPS update type 2 = combined TA/LA updating with IMSI attach.
	handleTrackingAreaUpdate(m, context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x02})

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.EMMCause == nil || *parsed.EMMCause != mme.EmmCauseCSDomainNotAvailable {
		t.Fatalf("EMM cause = %v, want #%d (CS domain not available)", parsed.EMMCause, mme.EmmCauseCSDomainNotAvailable)
	}
}

// TestTrackingAreaUpdateReallocatesGUTI checks that a TAU reallocates the GUTI:
// the accept carries a new GUTI, both old and new M-TMSIs resolve during the
// window, and TAU Complete commits the new one and frees the old (TS 24.301
// §5.5.3.2.4).
func TestTrackingAreaUpdateReallocatesGUTI(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.MmeIdentity()
	m.ReallocateGUTI(ue, plmn, group, code)
	oldMTMSI := ue.TmsiForTest()

	handleTrackingAreaUpdate(m, context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x03}) // periodic

	if ue.OldTmsiForTest() != oldMTMSI || ue.TmsiForTest() == oldMTMSI {
		t.Fatalf("GUTI not reallocated: mtmsi=%d oldMTMSI=%d (was %d)", ue.TmsiForTest(), ue.OldTmsiForTest(), oldMTMSI)
	}

	if _, ok := m.LookupUeByMTMSI(oldMTMSI); !ok {
		t.Fatal("old M-TMSI must stay resolvable until TAU Complete")
	}

	if _, ok := m.LookupUeByMTMSI(ue.TmsiForTest()); !ok {
		t.Fatal("new M-TMSI not resolvable")
	}

	// The accept on the wire carries the reallocated GUTI.
	dl := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(plain)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.GUTI == nil || parsed.GUTI.MTMSI != ue.TmsiForTest() {
		t.Fatalf("TAU Accept GUTI = %+v, want M-TMSI %d", parsed.GUTI, ue.TmsiForTest())
	}

	// TAU Complete commits the new GUTI and frees the old M-TMSI.
	handleTrackingAreaUpdateComplete(m, context.Background(), ue)

	if ue.OldTmsiForTest() != 0 {
		t.Fatal("reallocation not committed after TAU Complete")
	}

	if _, ok := m.LookupUeByMTMSI(oldMTMSI); ok {
		t.Fatal("old M-TMSI still resolvable after TAU Complete")
	}

	if _, ok := m.LookupUeByMTMSI(ue.TmsiForTest()); !ok {
		t.Fatal("new M-TMSI lost after TAU Complete")
	}
}

// TestTrackingAreaUpdateIdleNoActiveFlagReleases checks that a TAU from an idle
// UE without the active flag is accepted (reallocating the GUTI), and that the
// S1 release back to ECM-IDLE is deferred until the UE acknowledges the new GUTI
// with TAU Complete (TS 24.301 §5.5.3.2.4).
func TestTrackingAreaUpdateIdleNoActiveFlagReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.SetTmsiForTest(1) // a GUTI to reallocate

	handleTrackingAreaUpdate(m, context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x00})

	// Only the TAU Accept goes out; the release waits for TAU Complete.
	if len(cc.sent) != 1 {
		t.Fatalf("expected only a TAU Accept before TAU Complete, got %d", len(cc.sent))
	}

	// The UE is ECM-CONNECTED for the exchange so its TAU Complete resolves on the
	// re-established connection (would be dropped as "no active connection"
	// otherwise, TS 36.413 §10.6).
	if !ue.Connected() {
		t.Fatal("UE not ECM-CONNECTED for the TAU exchange; TAU Complete would be rejected")
	}

	if ue.OldTmsiForTest() == 0 {
		t.Fatal("GUTI reallocation not pending after TAU Accept")
	}

	// UE Completes: the new GUTI commits and the UE is released to ECM-IDLE.
	handleTrackingAreaUpdateComplete(m, context.Background(), ue)

	if ue.OldTmsiForTest() != 0 {
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

// TestTrackingAreaUpdateIdleActiveFlagReestablishes checks that a TAU from an
// idle UE with the active flag re-establishes the radio bearer via the Initial
// Context Setup and moves the UE to ECM-CONNECTED (TS 24.301 §5.5.3.2.4).
func TestTrackingAreaUpdateIdleActiveFlagReestablishes(t *testing.T) {
	m := newTestMME(t)
	ue, _ := idleRegisteredUE(t, m)
	cc := &captureConn{}
	establishResumeForTest(m, ue, cc, 9) // the resume re-binds the connection

	handleTrackingAreaUpdate(m, context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x08})

	if !ue.Connected() {
		t.Fatal("UE not ECM-CONNECTED after an active-flag TAU")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected Initial Context Setup Request, got %d S1AP messages", len(cc.sent))
	}

	parseInitialContextSetup(t, cc.sent[0])
}

// TestTrackingAreaUpdateRecovery checks that an integrity-protected TRACKING AREA
// UPDATE REQUEST arriving as an Initial UE Message that the MME cannot resolve (no
// security context, e.g. after an MME restart, TS 24.301 §5.5.3.2.5) is answered
// with TAU REJECT #9 over the bare connection rather than dropped, and that no UE
// context or connection is left behind, so the UE re-attaches at once.
func TestTrackingAreaUpdateRecovery(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	// Security-protected NAS: SHT=integrity-protected | PD=EMM, a MAC the MME
	// cannot reproduce (no context), sequence 1, and an inner plain TAU REQUEST.
	nas := []byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x01, 0x07, byte(eps.MsgTrackingAreaUpdateRequest)}

	mmes1ap.HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(cc), initiatingValue(t, initialUEMessagePDU(t, 7, nas)))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Reject), got %d", len(cc.sent))
	}

	rej, err := eps.ParseTrackingAreaUpdateReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not a TAU Reject: %v", err)
	}

	if rej.Cause != mme.EmmCauseUEIdentityUnderivable {
		t.Fatalf("TAU Reject cause = %d, want %d", rej.Cause, mme.EmmCauseUEIdentityUnderivable)
	}

	if m.ConnCountForTest() != 0 {
		t.Fatalf("bare connection not released after the TAU Reject: %d remain", m.ConnCountForTest())
	}
}
