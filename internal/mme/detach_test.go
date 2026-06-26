// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func TestDetachSubscriberNetworkInitiated(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	// A network-originating Detach Request (protected downlink NAS).
	if len(cc.sent) != 1 {
		t.Fatalf("expected network Detach Request, got %d", len(cc.sent))
	}

	if ue.emmState.load() != EMMDeregistered {
		t.Fatal("UE not EMM-DEREGISTERED after network-initiated detach")
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("Detach Request failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachRequestNetwork(plain); err != nil {
		t.Fatalf("not a network-originating Detach Request: %v", err)
	}

	// UE acknowledges → release + delete.
	m.onDetachAccept(context.Background(), ue)
	parseUEContextReleaseCommand(t, cc.sent[1])

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: 7}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.lookupUeByIMSI(ue.imsi); ok {
		t.Fatal("UE context not deleted after network-initiated detach")
	}
}

// TestDetachSubscriberUnansweredReleases confirms a network-initiated detach
// whose Detach Accept never arrives is retransmitted and then releases the UE
// context, so a silent UE cannot leak it (TS 24.301: T3422).
func TestDetachSubscriberUnansweredReleases(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardTimeout = 5 * time.Millisecond
	m.nasGuardMaxRetransmit = 2

	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	// Initial Detach Request + 2 retransmissions + the UE Context Release Command.
	eventually(t, time.Second, func() bool {
		return cc.count() >= 4
	})

	if !ue.s1.releasing {
		t.Fatal("UE not released after an unanswered network-initiated detach")
	}
}

func TestDetachSubscriberNotAttachedNoop(t *testing.T) {
	m := newTestMME(t)
	// No UE attached for this IMSI: must be a no-op (no panic, nothing sent).
	m.DetachSubscriber(context.Background(), "001010000000999")
}

// TestForgedMessageIgnoredForSecuredUE checks that once the secure exchange of
// NAS messages is established, a message that fails the integrity check (here a
// forged DETACH REQUEST) is discarded, not processed. TS 24.301 §4.4.4.3
// recovery applies only before that point (no usable context in the network),
// so an attacker cannot tear down an authenticated UE with an unverifiable
// message.
func TestForgedMessageIgnoredForSecuredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: false, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Integrity-protected envelope (SHT=1) with a MAC the MME cannot reproduce.
	forged := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, byte(ue.ulCount)}, plain...)

	m.handleNAS(context.Background(), ue, forged)

	if cc.count() != 0 {
		t.Fatalf("forged DETACH against a secured UE was acted on: %d downlink(s) sent", cc.count())
	}

	if _, ok := m.lookupUe(ue.s1.MMEUES1APID); !ok || ue.emmState.load() != EMMRegistered || !ue.secured {
		t.Fatal("secured UE was disrupted by a forged, unverifiable DETACH")
	}
}

// securedUE returns a UE context in the post-security-mode state (keys derived,
// registered), as it would be after a completed attach.
func securedUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := m.newUe(cc, 7)

	kasme := make([]byte, 32)
	for i := range kasme {
		kasme[i] = byte(i + 1)
	}

	ue.kasme = kasme
	ue.eea, ue.eia = 2, 2

	var err error
	if ue.knasEnc, err = deriveKNASEnc(kasme, 2); err != nil {
		t.Fatal(err)
	}

	if ue.knasInt, err = deriveKNASInt(kasme, 2); err != nil {
		t.Fatal(err)
	}

	ue.secured = true
	ue.emmState.store(EMMRegistered)
	registerTestUE(m, ue, testSubscriber.IMSI)

	return ue, cc
}

// registerTestUE sets a UE's IMSI and indexes it in the persistent registry, as a
// completed attach would. Re-registering a UE under a new IMSI moves its index.
func registerTestUE(m *MME, ue *UeContext, imsi string) {
	m.mu.Lock()
	if ue.imsi != "" && m.ues[ue.imsi] == ue {
		delete(m.ues, ue.imsi)
	}

	ue.imsi = imsi
	m.ues[imsi] = ue
	m.mu.Unlock()
}

func parseUEContextReleaseCommand(t *testing.T, pdu []byte) *s1ap.UEContextReleaseCommand {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command, got %T", msg)
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(im.Value)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	return cmd
}

func detachRequest(t *testing.T, ue *UeContext, switchOff bool) []byte {
	t.Helper()

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: switchOff, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.ulCount)),
		nascommon.DirectionUplink, ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

func TestDetachSwitchOff(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.handleNAS(context.Background(), ue, detachRequest(t, ue, true))

	// Switch-off: no Detach Accept, just the UE Context Release Command.
	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 S1AP message (UE Context Release Command), got %d", len(cc.sent))
	}

	cmd := parseUEContextReleaseCommand(t, cc.sent[0])
	if !cmd.UES1APIDs.Pair || cmd.UES1APIDs.MMEUES1APID != ue.s1.MMEUES1APID || cmd.UES1APIDs.ENBUES1APID != 7 {
		t.Fatalf("unexpected release command IDs: %+v", cmd.UES1APIDs)
	}

	// eNB confirms release → context deleted.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: 7}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.lookupUeByIMSI(ue.imsi); ok {
		t.Fatal("UE context not deleted after release complete")
	}
}

// A switch-off DETACH REQUEST that fails the integrity check is ignored once a
// security context exists (TS 24.301 §4.4.4.3).
func TestDetachSwitchOffUnverifiableIgnoredForSecuredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Null algorithms: zero MAC, unciphered payload — unverifiable without the keys.
	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, 0, nascommon.DirectionUplink,
		[16]byte{}, [16]byte{}, nascommon.NullIntegrity{}, nascommon.NullCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(context.Background(), ue, wire)

	if len(cc.sent) != 0 {
		t.Fatalf("S1AP messages sent = %d, want 0", len(cc.sent))
	}

	if _, ok := m.lookupUe(ue.s1.MMEUES1APID); !ok || ue.emmState.load() != EMMRegistered || !ue.secured {
		t.Fatal("secured UE state changed by an unverifiable switch-off detach")
	}
}

// Before a security context exists, an unverifiable switch-off detach is honoured
// (TS 24.301 §4.4.4.3).
func TestDetachSwitchOffUnsecuredAccepted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.secured = false

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, 0, nascommon.DirectionUplink,
		[16]byte{}, [16]byte{}, nascommon.NullIntegrity{}, nascommon.NullCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(context.Background(), ue, wire)

	if len(cc.sent) != 1 {
		t.Fatalf("expected UE Context Release Command, got %d S1AP messages", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

func TestDetachNotSwitchOff(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.handleNAS(context.Background(), ue, detachRequest(t, ue, false))

	// Not switch-off: Detach Accept (downlink NAS), then UE Context Release Command.
	if len(cc.sent) != 2 {
		t.Fatalf("expected Detach Accept + Release Command, got %d", len(cc.sent))
	}

	acceptWire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(acceptWire, nascommon.NASCount(0, acceptWire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("Detach Accept failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachAccept(plain); err != nil {
		t.Fatalf("not a Detach Accept: %v", err)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TestECMIdleBuffersSession checks that when a registered UE goes ECM-IDLE, the
// MME buffers its EPS session so downlink data triggers paging (TS 23.401
// §5.3.4.3) rather than being forwarded to the released eNB tunnel.
func TestECMIdleBuffersSession(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).apn = "internet"

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: 7}
	b, _ := complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.connected() {
		t.Fatal("UE not ECM-IDLE after release complete")
	}

	if !m.session.(*fakeSessionManager).deactivated {
		t.Fatal("EPS session not deactivated (buffered) for paging on ECM-IDLE")
	}
}

func TestUEContextReleaseRequestFromENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: 7,
		Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseRequest(context.Background(), cc, pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 UE Context Release Command, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])

	// A second release attempt must not emit another command (idempotent).
	m.releaseUEContext(context.Background(), ue, causeNASDetach)

	if len(cc.sent) != 1 {
		t.Fatalf("release not idempotent: %d commands sent", len(cc.sent))
	}

	// Completing an eNB-initiated release moves the UE to ECM-IDLE; the EMM
	// context is retained, not deleted.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: 7}

	b, _ = complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	got, ok := m.lookupUeByIMSI(ue.imsi)
	if !ok {
		t.Fatal("EMM context deleted on an inactivity release; expected ECM-IDLE retention")
	}

	if got.connected() {
		t.Fatal("UE not marked ECM-IDLE after eNB release")
	}

	// The released MME-UE-S1AP-ID no longer identifies an active S1 connection.
	// A repeat UE Context Release Request on the same association is answered
	// with an Error Indication, not re-actioned with another release command
	// (TS 36.413).
	m.handleUEContextReleaseRequest(context.Background(), cc, pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 2 {
		t.Fatalf("expected an Error Indication for the released AP ID, got %d S1AP messages", len(cc.sent))
	}

	ind := parseOutboundErrorIndication(t, cc.sent[1])
	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}

// TestUEContextReleaseRequestFromForeignENB checks that a UE-associated message
// arriving on an S1 association other than the UE's own is rejected with an
// Error Indication, not acted upon: the global MME-UE-S1AP-ID map is shared
// across eNBs, so without this an eNB could release a UE attached through
// another by presenting its AP-ID pair (TS 36.413).
func TestUEContextReleaseRequestFromForeignENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: ue.s1.ENBUES1APID,
		Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	foreign := &captureConn{}
	m.handleUEContextReleaseRequest(context.Background(), foreign, pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 0 {
		t.Fatalf("foreign eNB released a UE on another association: %d S1AP messages on the owning association", len(cc.sent))
	}

	if ue.s1.releasing {
		t.Fatal("UE marked releasing by a message from a foreign association")
	}

	if len(foreign.sent) != 1 {
		t.Fatalf("expected one Error Indication to the foreign association, got %d", len(foreign.sent))
	}

	ind := parseOutboundErrorIndication(t, foreign.sent[0])
	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}

// TestDetachSubscriberIdleReleasesLocally checks that deleting a subscriber whose
// UE is in ECM-IDLE releases its sessions and removes the context locally, without
// dereferencing the freed S1 connection.
func TestDetachSubscriberIdleReleasesLocally(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).apn = "internet"
	m.freeS1Conn(ue) // ECM-IDLE: no S1 connection

	m.DetachSubscriber(context.Background(), ue.imsi)

	if _, ok := m.lookupUeByIMSI(ue.imsi); ok {
		t.Fatal("idle UE context not removed on subscriber deletion")
	}

	if !m.session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on subscriber deletion")
	}
}

// TestReleaseUEContextIdleNoPanic checks releaseUEContext on a UE whose connection
// was freed in the gap before it took the lock returns without dereferencing nil.
func TestReleaseUEContextIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	m.freeS1Conn(ue)

	m.releaseUEContext(context.Background(), ue, causeNASNormalRelease)
}
