// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/s1ap"
)

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

	if !ue.S1.releasing {
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
func securedUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	kasme := make([]byte, 32)
	for i := range kasme {
		kasme[i] = byte(i + 1)
	}

	ue.kasme = kasme
	ue.eea, ue.eia = 2, 2

	var err error
	if ue.knasEnc, err = DeriveKNASEnc(kasme, 2); err != nil {
		t.Fatal(err)
	}

	if ue.knasInt, err = DeriveKNASInt(kasme, 2); err != nil {
		t.Fatal(err)
	}

	ue.secured = true
	ue.S1.secureExchangeEstablished = true
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

func TestECMIdleBuffersSession(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).Apn = "internet"

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.S1.MMEUES1APID, ENBUES1APID: 7}
	b, _ := complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	m.HandleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.Connected() {
		t.Fatal("UE not ECM-IDLE after release complete")
	}

	if !m.Session.(*fakeSessionManager).deactivated {
		t.Fatal("EPS session not deactivated (buffered) for paging on ECM-IDLE")
	}
}

func TestUEContextReleaseRequestFromENB(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req := &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.S1.MMEUES1APID, ENBUES1APID: 7,
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
	m.ReleaseUEContext(context.Background(), ue, CauseNASDetach)

	if len(cc.sent) != 1 {
		t.Fatalf("release not idempotent: %d commands sent", len(cc.sent))
	}

	// Completing an eNB-initiated release moves the UE to ECM-IDLE; the EMM
	// context is retained, not deleted.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.S1.MMEUES1APID, ENBUES1APID: 7}

	b, _ = complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	m.HandleUEContextReleaseComplete(cc, cpdu.(*s1ap.SuccessfulOutcome).Value)

	got, ok := m.LookupUeByIMSI(ue.imsi)
	if !ok {
		t.Fatal("EMM context deleted on an inactivity release; expected ECM-IDLE retention")
	}

	if got.Connected() {
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
		MMEUES1APID: ue.S1.MMEUES1APID, ENBUES1APID: ue.S1.ENBUES1APID,
		Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}

	b, _ := req.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	foreign := &captureConn{}
	m.handleUEContextReleaseRequest(context.Background(), foreign, pdu.(*s1ap.InitiatingMessage).Value)

	if len(cc.sent) != 0 {
		t.Fatalf("foreign eNB released a UE on another association: %d S1AP messages on the owning association", len(cc.sent))
	}

	if ue.S1.releasing {
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
	testPDN(ue).Apn = "internet"
	m.FreeS1Conn(ue) // ECM-IDLE: no S1 connection

	m.DetachSubscriber(context.Background(), ue.imsi)

	if _, ok := m.LookupUeByIMSI(ue.imsi); ok {
		t.Fatal("idle UE context not removed on subscriber deletion")
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released on subscriber deletion")
	}
}

// TestReleaseUEContextIdleNoPanic checks releaseUEContext on a UE whose connection
// was freed in the gap before it took the lock returns without dereferencing nil.
func TestReleaseUEContextIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	m.FreeS1Conn(ue)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)
}
