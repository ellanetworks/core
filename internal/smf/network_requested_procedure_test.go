// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

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
