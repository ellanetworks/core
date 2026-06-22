// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

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

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: 7}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.lookupUe(ue.MMEUES1APID); ok {
		t.Fatal("UE context not deleted after network-initiated detach")
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

	if _, ok := m.lookupUe(ue.MMEUES1APID); !ok || ue.emmState.load() != EMMRegistered || !ue.secured {
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
	ue.imsi = testSubscriber.IMSI

	return ue, cc
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
	if !cmd.UES1APIDs.Pair || cmd.UES1APIDs.MMEUES1APID != ue.MMEUES1APID || cmd.UES1APIDs.ENBUES1APID != 7 {
		t.Fatalf("unexpected release command IDs: %+v", cmd.UES1APIDs)
	}

	// eNB confirms release → context deleted.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: 7}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.lookupUe(ue.MMEUES1APID); ok {
		t.Fatal("UE context not deleted after release complete")
	}
}

// TestDetachSwitchOffNullSecurity covers srsUE's behaviour: a switch-off Detach
// Request sent with a null MAC and an unciphered payload, which the MME must
// accept despite the failed integrity check (TS 24.301 §5.5.2.2.2).
func TestDetachSwitchOffNullSecurity(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Wrap as security-protected (SHT integrity+ciphered) but with null
	// algorithms: zero MAC and unciphered payload, as srsUE does.
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

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: 7}
	b, _ := complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.ecmState.load() != ECMIdle {
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
		MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: 7,
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
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: 7}

	b, _ = complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	m.handleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	got, ok := m.lookupUe(ue.MMEUES1APID)
	if !ok {
		t.Fatal("EMM context deleted on an inactivity release; expected ECM-IDLE retention")
	}

	if got.ecmState.load() != ECMIdle {
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
		MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: ue.ENBUES1APID,
		Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	foreign := &captureConn{}
	m.handleUEContextReleaseRequest(context.Background(), foreign, pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 0 {
		t.Fatalf("foreign eNB released a UE on another association: %d S1AP messages on the owning association", len(cc.sent))
	}

	if ue.releasing {
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
