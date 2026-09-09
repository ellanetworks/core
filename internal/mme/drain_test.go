// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
)

func trackTestRadio(m *MME, conn S1APWriter, id string) *Radio {
	r := &Radio{Conn: conn, m: m, id: id, Log: logger.MmeLog}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.reg.Track(conn, r)

	return r
}

func parseConfigUpdate(t *testing.T, pdu []byte) *s1ap.MMEConfigurationUpdate {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcMMEConfigurationUpdate {
		t.Fatalf("got %T procedureCode %v, want MME Configuration Update", msg, im)
	}

	out, err := s1ap.ParseMMEConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return out
}

func TestDrainAdvertisesAZeroWeightFactorToEveryENB(t *testing.T) {
	m := newTestMME(t)

	a, b := &captureConn{}, &captureConn{}
	ra := trackTestRadio(m, a, "00f110-1")
	rb := trackTestRadio(m, b, "00f110-2")

	if got := m.RelativeCapacity(); got != DefaultRelativeCapacity {
		t.Fatalf("capacity before drain = %d, want %d", got, DefaultRelativeCapacity)
	}

	if notified := m.SetEligible(context.Background(), false); notified != 2 {
		t.Fatalf("notified %d eNBs, want 2", notified)
	}

	t.Cleanup(func() {
		m.ConfigUpdateAcknowledged(context.Background(), ra)
		m.ConfigUpdateAcknowledged(context.Background(), rb)
		m.SetEligible(context.Background(), true)
	})

	if got := m.RelativeCapacity(); got != DrainedRelativeCapacity {
		t.Fatalf("capacity after drain = %d, want 0", got)
	}

	for _, cc := range []*captureConn{a, b} {
		sent := cc.snapshot()
		if len(sent) != 1 {
			t.Fatalf("eNB received %d messages, want 1", len(sent))
		}

		update := parseConfigUpdate(t, sent[0])
		if update.RelativeMMECapacity == nil || *update.RelativeMMECapacity != 0 {
			t.Fatalf("RelativeMMECapacity = %v, want 0", update.RelativeMMECapacity)
		}

		if update.MMEName != nil || len(update.ServedGUMMEIs) != 0 {
			t.Fatalf("drain update carried extra IEs: %+v", update)
		}
	}
}

func TestDrainedCapacityIsAdvertisedToLateJoiners(t *testing.T) {
	m := newTestMME(t)
	m.SetEligible(context.Background(), false)

	t.Cleanup(func() { m.SetEligible(context.Background(), true) })

	if got := m.RelativeCapacity(); got != DrainedRelativeCapacity {
		t.Fatalf("capacity = %d, want 0", got)
	}
}

func TestResumeRestoresTheWeightFactor(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	radio := trackTestRadio(m, cc, "00f110-1")

	m.SetEligible(context.Background(), false)
	m.ConfigUpdateAcknowledged(context.Background(), radio)

	if notified := m.SetEligible(context.Background(), true); notified != 1 {
		t.Fatalf("set-eligible notified %d eNBs, want 1", notified)
	}

	if got := m.RelativeCapacity(); got != DefaultRelativeCapacity {
		t.Fatalf("capacity after resume = %d, want %d", got, DefaultRelativeCapacity)
	}

	sent := cc.snapshot()
	if len(sent) != 2 {
		t.Fatalf("eNB received %d messages, want 2 (drain + resume)", len(sent))
	}

	update := parseConfigUpdate(t, sent[1])
	if update.RelativeMMECapacity == nil || *update.RelativeMMECapacity != DefaultRelativeCapacity {
		t.Fatalf("resume advertised %v, want %d", update.RelativeMMECapacity, DefaultRelativeCapacity)
	}
}

func TestOffloadReleasesRegisteredUEsWithLoadBalancingTAURequired(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	if released := m.Offload(context.Background(), 16); released != 1 {
		t.Fatalf("off-loaded %d UEs, want 1", released)
	}

	sent := cc.snapshot()
	if len(sent) != 1 {
		t.Fatalf("UE received %d messages, want 1 UE Context Release Command", len(sent))
	}

	msg, err := s1ap.Unmarshal(sent[0])
	if err != nil {
		t.Fatal(err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("got %T, want UE Context Release Command", msg)
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if cmd.Cause == nil || cmd.Cause.Group != s1ap.CauseGroupRadioNetwork ||
		cmd.Cause.Value != s1ap.CauseRadioNetworkLoadBalancingTAURequired {
		t.Fatalf("cause = %+v, want radioNetwork/load-balancing-tau-required", cmd.Cause)
	}
}

func TestOffloadSkipsUEsThatAreNotYetRegistered(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.TransitionTo(EMMRegistrationInitiated)

	if released := m.Offload(context.Background(), 16); released != 0 {
		t.Fatalf("off-loaded %d UEs, want 0", released)
	}

	if got := cc.count(); got != 0 {
		t.Fatalf("mid-attach UE received %d messages, want 0", got)
	}
}

func TestOffloadIsBounded(t *testing.T) {
	m := newTestMME(t)

	conns := make([]*captureConn, 0, 5)

	for i := range 5 {
		ue, cc := securedUE(t, m)
		registerTestUE(m, ue, "00101000000000"+string(rune('0'+i)))
		ue.TransitionTo(EMMRegistered)

		conns = append(conns, cc)
	}

	if released := m.Offload(context.Background(), 2); released != 2 {
		t.Fatalf("off-loaded %d UEs in one sweep, want the batch size 2", released)
	}

	commanded := 0

	for _, cc := range conns {
		commanded += cc.count()
	}

	if commanded != 2 {
		t.Fatalf("%d UEs were commanded to release, want 2: the sweep is not bounded", commanded)
	}
}

func TestConfigUpdateIsSerializedPerENB(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	radio := trackTestRadio(m, cc, "00f110-1")

	m.SetEligible(context.Background(), false)

	if got := cc.count(); got != 1 {
		t.Fatalf("drain sent %d updates, want 1", got)
	}

	if notified := m.SetEligible(context.Background(), true); notified != 0 {
		t.Fatal("a second update went out with one already outstanding")
	}

	if got := cc.count(); got != 1 {
		t.Fatalf("%d updates on the wire, want 1: the procedure was not serialized", got)
	}

	m.ConfigUpdateAcknowledged(context.Background(), radio)
	m.SetEligible(context.Background(), true)

	sent := cc.snapshot()
	if len(sent) != 2 {
		t.Fatalf("%d updates after the ack, want 2: the blocked change was not re-sent", len(sent))
	}

	update := parseConfigUpdate(t, sent[1])
	if update.RelativeMMECapacity == nil || *update.RelativeMMECapacity != DefaultRelativeCapacity {
		t.Fatalf("re-sent update carried %v, want %d", update.RelativeMMECapacity, DefaultRelativeCapacity)
	}
}

func TestQueuedConfigUpdatesCoalesceToTheNetChange(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	radio := trackTestRadio(m, cc, "00f110-1")

	m.SetEligible(context.Background(), false)
	m.SetEligible(context.Background(), true)
	m.SetEligible(context.Background(), false)

	m.ConfigUpdateAcknowledged(context.Background(), radio)

	sent := cc.snapshot()
	if len(sent) != 1 {
		t.Fatalf("%d updates on the wire, want 1: the queued updates did not coalesce to the net change", len(sent))
	}

	update := parseConfigUpdate(t, sent[0])
	if update.RelativeMMECapacity == nil || *update.RelativeMMECapacity != DrainedRelativeCapacity {
		t.Fatalf("update carried %v, want the net capacity 0", update.RelativeMMECapacity)
	}
}

func TestUnchangedCapacityIsNotReadvertised(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	radio := trackTestRadio(m, cc, "00f110-1")

	m.SetEligible(context.Background(), false)
	m.ConfigUpdateAcknowledged(context.Background(), radio)

	for range 5 {
		m.SetEligible(context.Background(), false)
	}

	if got := cc.count(); got != 1 {
		t.Fatalf("%d updates for one capacity change, want 1: reconciling re-advertises an unchanged capacity", got)
	}
}

func TestFailureClearsTheAdvertisedCapacitySoTheBackstopRetries(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	radio := trackTestRadio(m, cc, "00f110-1")

	m.SetEligible(context.Background(), false)
	m.ConfigUpdateFailed(context.Background(), radio, 0)

	m.SetEligible(context.Background(), false)

	if got := cc.count(); got != 2 {
		t.Fatalf("%d updates, want 2: a rejected update was not re-sent on the next reconcile", got)
	}
}

func TestTimeToWaitHoldsOffTheNextUpdate(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	radio := trackTestRadio(m, cc, "00f110-1")

	m.SetEligible(context.Background(), false)
	m.ConfigUpdateFailed(context.Background(), radio, time.Minute)

	m.SetEligible(context.Background(), false)

	if got := cc.count(); got != 1 {
		t.Fatalf("%d updates, want 1: the MME reinitiated inside the Time To Wait", got)
	}
}

func TestConfigUpdateStateClearsOnDisconnect(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	radio := m.RadioForConn(conn)

	m.mu.Lock()
	radio.configUpdateOutstanding = true
	m.mu.Unlock()

	m.DisconnectRadio(conn)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if radio.configUpdateOutstanding {
		t.Fatal("a dropped association left its configuration-update procedure outstanding")
	}
}

func TestOffloadWithoutABoundReleasesEverything(t *testing.T) {
	m := newTestMME(t)

	for i := range 5 {
		ue, _ := securedUE(t, m)
		registerTestUE(m, ue, "00101000000000"+string(rune('0'+i)))
	}

	want := m.CountRegisteredSubscribers()

	if got := m.Offload(context.Background(), 0); got != want {
		t.Fatalf("Offload(batch=0) released %d, want all %d", got, want)
	}
}

func TestIdleUEsAreNotOffloadable(t *testing.T) {
	m := newTestMME(t)

	ue, _ := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)
	m.FreeUeConn(ue)

	if got := m.Offload(context.Background(), 0); got != 0 {
		t.Fatalf("released %d idle UEs, want 0: an idle UE has no S1 connection to release", got)
	}

	if got := m.RemainingOffloadable(); got != 0 {
		t.Fatalf("RemainingOffloadable = %d, want 0: an idle UE has no S1 connection to release, so it cannot block the drain", got)
	}
}

func TestDrainSkipsAssociationsWithoutS1Setup(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	trackTestRadio(m, cc, "")

	if notified := m.SetEligible(context.Background(), false); notified != 0 {
		t.Fatalf("notified %d eNBs, want 0", notified)
	}

	if got := cc.count(); got != 0 {
		t.Fatalf("sent %d messages before S1 Setup completed; TS 36.413 §8.7.3.1 makes S1 Setup the first S1AP procedure on the association", got)
	}
}
