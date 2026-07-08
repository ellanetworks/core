// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

const procedureTimerInterval = 20 * time.Millisecond

func buildPDUSessionModificationComplete(pduSessionID, pti uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionModificationComplete)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationComplete = nasMessage.NewPDUSessionModificationComplete(0)
	m.PDUSessionModificationComplete.SetMessageType(nas.MsgTypePDUSessionModificationComplete)
	m.PDUSessionModificationComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationComplete.SetPDUSessionID(pduSessionID)
	m.PDUSessionModificationComplete.SetPTI(pti)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Modification Complete: %v", err))
	}

	return buf
}

func buildPDUSessionModificationCommandReject(pduSessionID, pti uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionModificationCommandReject)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationCommandReject = nasMessage.NewPDUSessionModificationCommandReject(0)
	m.PDUSessionModificationCommandReject.SetMessageType(nas.MsgTypePDUSessionModificationCommandReject)
	m.PDUSessionModificationCommandReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationCommandReject.SetPDUSessionID(pduSessionID)
	m.PDUSessionModificationCommandReject.SetPTI(pti)
	m.PDUSessionModificationCommandReject.SetCauseValue(nasMessage.Cause5GSMRequestRejectedUnspecified)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Modification Command Reject: %v", err))
	}

	return buf
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

func reconcileAmbrChange(t *testing.T, s *smf.SMF, ref string) {
	t.Helper()

	err := s.ReconcileSmContext(context.Background(), &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy: &models.SessionPolicyDelta{
			SessionAmbrUplink:   "500 Mbps",
			SessionAmbrDownlink: "600 Mbps",
			Var5qi:              9,
			Arp:                 1,
		},
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}
}

func releaseCallCount(amfCb *fakeAMF) int {
	amfCb.mu.Lock()
	defer amfCb.mu.Unlock()

	return len(amfCb.releaseCalls)
}

func modifyCallCount(amfCb *fakeAMF) int {
	amfCb.mu.Lock()
	defer amfCb.mu.Unlock()

	return len(amfCb.modifyCalls)
}

func ptiInUse(t *testing.T, smCtx *smf.SMContext, pti uint8) bool {
	t.Helper()

	smCtx.Mutex.Lock()
	defer smCtx.Mutex.Unlock()

	return smCtx.IsPTIInUse(pti)
}

// TestReconcileSkipsIdleSession verifies that a reconcile for a CM-IDLE session (the
// downlink FAR buffering after DeactivateSmContext) touches no enforcement point —
// no UPF push, no N1N2, no policy commit. The change is applied when the UE
// reactivates (item 8; mirrors the MME deferring idle UEs to its ICS-Response
// reconcile).
func TestReconcileSkipsIdleSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	// Move the session to CM-IDLE (downlink FAR → buffering).
	if err := s.DeactivateSmContext(context.Background(), ref); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	// Ignore the PFCP modify from the deactivation itself.
	upf.mu.Lock()
	upf.modifyCalls = nil
	upf.mu.Unlock()

	reconcileAmbrChange(t, s, ref)

	upf.mu.Lock()
	upfModifies := len(upf.modifyCalls)
	upf.mu.Unlock()

	if upfModifies != 0 {
		t.Fatalf("an idle session must not be pushed to the UPF, got %d modify calls", upfModifies)
	}

	if got := modifyCallCount(amfCb); got != 0 {
		t.Fatalf("an idle session must not be signalled, got %d N1N2 modify calls", got)
	}

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if dl != "200 Mbps" {
		t.Fatalf("an idle session's policy must not be committed, got %q", dl)
	}
}

// TestReconcileSkippedWhileProcedureInFlight verifies that a reconcile arriving
// while a network-requested modification is outstanding (T3591 running) is skipped
// rather than re-sending the command and resetting the counter (item 8; mirrors the
// MME guarding on Modifying/Deactivating).
func TestReconcileSkippedWhileProcedureInFlight(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb) // default (long) T3591: it will not fire during the test

	_, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, ref)

	if got := modifyCallCount(amfCb); got != 1 {
		t.Fatalf("expected 1 modify call, got %d", got)
	}

	reconcileAmbrChange(t, s, ref)

	if got := modifyCallCount(amfCb); got != 1 {
		t.Fatalf("a reconcile during an in-flight modification must be skipped, got %d modify calls", got)
	}
}

// TestModificationRejectKeepsPreviousPolicy verifies that a PDU SESSION MODIFICATION
// COMMAND REJECT discards the uncommitted policy, leaving the previous configuration
// in place (item 8; TS 24.501 §6.3.2.4, §6.3.2.5).
func TestModificationRejectKeepsPreviousPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, ref) // new AMBR 500/600 Mbps, held pending

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionModificationCommandReject(smCtx.PDUSessionID, 0)); err != nil {
		t.Fatalf("modification command reject: %v", err)
	}

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if dl != "200 Mbps" {
		t.Fatalf("a rejected modification must keep the previous AMBR (200 Mbps), got %q", dl)
	}
}

// TestT3592RetransmitsThenReleasesLocally verifies that an unacknowledged release
// command is retransmitted on each of the first four T3592 expiries and that the
// session is released locally on the fifth (TS 24.501 §6.3.3 abnormal case a).
func TestT3592RetransmitsThenReleasesLocally(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := smf.New(pcf, store, upf, amfCb, smf.WithT3592(procedureTimerInterval))

	smCtx, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	waitFor(t, "session local release after T3592 expiry", func() bool { return s.GetSession(ref) == nil })

	if got := releaseCallCount(amfCb); got != 5 {
		t.Errorf("ReleaseSession calls = %d, want 5 (1 initial + 4 retransmissions)", got)
	}

	// The user plane, held through the release window, is torn down on the T3592
	// abort (item 8).
	store.mu.Lock()
	releasedIPs := len(store.releasedIPs)
	store.mu.Unlock()

	if releasedIPs == 0 {
		t.Error("expected IP released on T3592 abort")
	}

	upf.mu.Lock()
	deleteCalls := len(upf.deleteCalls)
	upf.mu.Unlock()

	if deleteCalls == 0 {
		t.Error("expected PFCP session deleted on T3592 abort")
	}
}

// TestT3592StopsOnReleaseComplete verifies that the PDU Session Release Complete
// stops T3592 and removes the session with no retransmission (TS 24.501 §6.3.3.3).
func TestT3592StopsOnReleaseComplete(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := smf.New(pcf, store, upf, amfCb, smf.WithT3592(procedureTimerInterval))

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 5

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseRequest(smCtx.PDUSessionID, pti)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseComplete(smCtx.PDUSessionID, pti)); err != nil {
		t.Fatalf("release complete: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("expected session removed after Release Complete")
	}

	time.Sleep(5 * procedureTimerInterval)

	if got := releaseCallCount(amfCb); got != 1 {
		t.Errorf("ReleaseSession calls = %d, want 1 (no retransmission after Release Complete)", got)
	}
}

// TestT3591RetransmitsThenAborts verifies that an unacknowledged modification
// command is retransmitted on each of the first four T3591 expiries and that the
// procedure is aborted on the fifth, leaving the session active (TS 24.501
// §6.3.2.5).
func TestT3591RetransmitsThenAborts(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := smf.New(pcf, store, upf, amfCb, smf.WithT3591(procedureTimerInterval))

	_, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, ref)

	waitFor(t, "T3591 abort clearing the PTI", func() bool {
		sc := s.GetSession(ref)
		return sc != nil && !ptiInUse(t, sc, 0)
	})

	if got := modifyCallCount(amfCb); got != 5 {
		t.Errorf("ModifyN1N2 calls = %d, want 5 (1 initial + 4 retransmissions)", got)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("expected session to remain active after modification abort")
	}
}

// TestT3591StopsOnModificationComplete verifies that the PDU Session Modification
// Complete stops T3591 with no retransmission (TS 24.501 §6.3.2.3).
func TestT3591StopsOnModificationComplete(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := smf.New(pcf, store, upf, amfCb, smf.WithT3591(procedureTimerInterval))

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, ref)

	// A network-requested modification carries PTI 0 (TS 24.501 §7.3.1).
	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionModificationComplete(smCtx.PDUSessionID, 0)); err != nil {
		t.Fatalf("modification complete: %v", err)
	}

	time.Sleep(5 * procedureTimerInterval)

	if got := modifyCallCount(amfCb); got != 1 {
		t.Errorf("ModifyN1N2 calls = %d, want 1 (no retransmission after Modification Complete)", got)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("expected session to remain active after modification complete")
	}

	if ptiInUse(t, smCtx, 0) {
		t.Error("expected PTI 0 cleared after Modification Complete")
	}
}
