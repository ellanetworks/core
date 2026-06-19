// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// trackingAreaUpdateNAS builds a protected TRACKING AREA UPDATE REQUEST at the
// UE's current uplink NAS COUNT, optionally carrying an EPS bearer context status
// IE (IEI 0x57) when bearerStatus is non-nil.
func trackingAreaUpdateNAS(t *testing.T, ue *UeContext, activeFlag bool, bearerStatus *uint16) []byte {
	t.Helper()

	updateType := uint8(0)
	if activeFlag {
		updateType |= 0x08
	}

	plain := []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), updateType}

	if bearerStatus != nil {
		plain = append(plain, 0x57, 0x02, byte(*bearerStatus), byte(*bearerStatus>>8))
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.ulCount)),
		nascommon.DirectionUplink, ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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

	m.handleNAS(context.Background(), ue, trackingAreaUpdateNAS(t, ue, false, nil))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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

	if ue.emmState != EMMRegistered || ue.ecmState != ECMConnected {
		t.Fatal("UE should remain registered and connected after a periodic TAU")
	}
}

// TestTrackingAreaUpdateReconcilesBearerContextStatus checks that when the UE
// reports its EPS bearer context status, the MME deactivates locally the bearers
// the UE marks inactive and mirrors its resulting active set in the TAU Accept
// (TS 24.301 §5.5.3.2.4). Default bearer EBI 5 stays; additional bearer EBI 6 is
// released.
func TestTrackingAreaUpdateReconcilesBearerContextStatus(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m) // ECM-CONNECTED, secured, EMM-REGISTERED

	m.addDefaultPDN(ue) // EBI 5
	ue.ensurePDN(6)     // an additional PDN connection

	status := uint16(1 << 5) // the UE reports only EBI 5 active
	m.handleNAS(context.Background(), ue, trackingAreaUpdateNAS(t, ue, false, &status))

	if _, ok := ue.pdns[6]; ok {
		t.Fatal("EBI 6 should be released locally after the UE reports it inactive")
	}

	if _, ok := ue.pdns[5]; !ok {
		t.Fatal("EBI 5 should remain active")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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
	m.onTrackingAreaUpdate(context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x02})

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	accept, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(accept)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.EMMCause == nil || *parsed.EMMCause != emmCauseCSDomainNotAvailable {
		t.Fatalf("EMM cause = %v, want #%d (CS domain not available)", parsed.EMMCause, emmCauseCSDomainNotAvailable)
	}
}

// TestTrackingAreaUpdateReallocatesGUTI checks that a TAU reallocates the GUTI:
// the accept carries a new GUTI, both old and new M-TMSIs resolve during the
// window, and TAU Complete commits the new one and frees the old (TS 24.301
// §5.5.3.2.4).
func TestTrackingAreaUpdateReallocatesGUTI(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plmn, err := m.operatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.mmeIdentity()
	m.assignGUTI(ue, plmn, group, code)
	oldMTMSI := ue.mtmsi

	m.onTrackingAreaUpdate(context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x03}) // periodic

	if ue.oldMTMSI != oldMTMSI || ue.mtmsi == oldMTMSI {
		t.Fatalf("GUTI not reallocated: mtmsi=%d oldMTMSI=%d (was %d)", ue.mtmsi, ue.oldMTMSI, oldMTMSI)
	}

	if _, ok := m.lookupUeByMTMSI(oldMTMSI); !ok {
		t.Fatal("old M-TMSI must stay resolvable until TAU Complete")
	}

	if _, ok := m.lookupUeByMTMSI(ue.mtmsi); !ok {
		t.Fatal("new M-TMSI not resolvable")
	}

	// The accept on the wire carries the reallocated GUTI.
	dl := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(dl, nascommon.NASCount(0, dl[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect TAU Accept: %v", err)
	}

	parsed, err := eps.ParseTrackingAreaUpdateAccept(plain)
	if err != nil {
		t.Fatalf("parse TAU Accept: %v", err)
	}

	if parsed.GUTI == nil || parsed.GUTI.MTMSI != ue.mtmsi {
		t.Fatalf("TAU Accept GUTI = %+v, want M-TMSI %d", parsed.GUTI, ue.mtmsi)
	}

	// TAU Complete commits the new GUTI and frees the old M-TMSI.
	m.onTrackingAreaUpdateComplete(context.Background(), ue)

	if ue.oldMTMSI != 0 {
		t.Fatal("reallocation not committed after TAU Complete")
	}

	if _, ok := m.lookupUeByMTMSI(oldMTMSI); ok {
		t.Fatal("old M-TMSI still resolvable after TAU Complete")
	}

	if _, ok := m.lookupUeByMTMSI(ue.mtmsi); !ok {
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
	ue.mtmsi = 1 // a GUTI to reallocate
	ue.ecmState = ECMIdle

	m.onTrackingAreaUpdate(context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x00})

	// Only the TAU Accept goes out; the release waits for TAU Complete.
	if len(cc.sent) != 1 {
		t.Fatalf("expected only a TAU Accept before TAU Complete, got %d", len(cc.sent))
	}

	// The UE is ECM-CONNECTED for the exchange so its TAU Complete resolves on the
	// re-established connection (would be dropped as "no active connection"
	// otherwise, TS 36.413 §10.6).
	if ue.ecmState != ECMConnected {
		t.Fatal("UE not ECM-CONNECTED for the TAU exchange; TAU Complete would be rejected")
	}

	if ue.oldMTMSI == 0 {
		t.Fatal("GUTI reallocation not pending after TAU Accept")
	}

	// UE Completes: the new GUTI commits and the UE is released to ECM-IDLE.
	m.onTrackingAreaUpdateComplete(context.Background(), ue)

	if ue.oldMTMSI != 0 {
		t.Fatal("old M-TMSI not freed after TAU Complete")
	}

	if len(cc.sent) != 2 {
		t.Fatalf("expected a UE Context Release Command after TAU Complete, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[1])

	if ue.emmState != EMMRegistered {
		t.Fatal("UE should remain EMM-REGISTERED after a periodic TAU")
	}
}

// TestTrackingAreaUpdateIdleActiveFlagReestablishes checks that a TAU from an
// idle UE with the active flag re-establishes the radio bearer via the Initial
// Context Setup and moves the UE to ECM-CONNECTED (TS 24.301 §5.5.3.2.4).
func TestTrackingAreaUpdateIdleActiveFlagReestablishes(t *testing.T) {
	m := newTestMME(t)
	ue, _ := idleRegisteredUE(t, m)
	cc := ue.conn.(*captureConn)

	m.onTrackingAreaUpdate(context.Background(), ue, []byte{0x07, byte(eps.MsgTrackingAreaUpdateRequest), 0x08})

	if ue.ecmState != ECMConnected {
		t.Fatal("UE not ECM-CONNECTED after an active-flag TAU")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected Initial Context Setup Request, got %d S1AP messages", len(cc.sent))
	}

	parseInitialContextSetup(t, cc.sent[0])
}

// TestTrackingAreaUpdateRecovery checks that an integrity-protected TRACKING
// AREA UPDATE REQUEST the MME cannot verify (no security context, e.g. after an
// MME restart, TS 24.301 §4.4.4.3) is answered with TAU REJECT #9 rather than
// dropped, and that the transient UE context is discarded so the UE re-attaches.
func TestTrackingAreaUpdateRecovery(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 7)

	// Security-protected NAS: SHT=integrity-protected | PD=EMM, a MAC the MME
	// cannot reproduce (no context), sequence 1, and an inner plain TAU REQUEST.
	nas := []byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x01, 0x07, byte(eps.MsgTrackingAreaUpdateRequest)}

	m.handleNAS(context.Background(), ue, nas)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Reject), got %d", len(cc.sent))
	}

	rej, err := eps.ParseTrackingAreaUpdateReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not a TAU Reject: %v", err)
	}

	if rej.Cause != emmCauseUEIdentityUnderivable {
		t.Fatalf("TAU Reject cause = %d, want %d", rej.Cause, emmCauseUEIdentityUnderivable)
	}

	if _, ok := m.lookupUe(ue.MMEUES1APID); ok {
		t.Fatal("transient UE context was not discarded after the TAU Reject")
	}
}
