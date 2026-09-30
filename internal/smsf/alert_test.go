// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/sms"
)

const otherServiceCentre = "15550000009"

func TestWaitingDataRecordedByAnotherNodeAlerts(t *testing.T) {
	e := newEnv(t)

	if err := e.store.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre}); err != nil {
		t.Fatal(err)
	}

	e.smsf.UEReachable(context.Background(), imsi)

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
	eventually(t, "the waiting data to clear", func() bool { return !e.smsf.Waiting(imsi) })
}

func TestEveryWaitingServiceCentreIsAlerted(t *testing.T) {
	e := newEnv(t)

	for _, sc := range []string{serviceCentre, otherServiceCentre} {
		if err := e.store.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: sc}); err != nil {
			t.Fatal(err)
		}
	}

	e.smsf.UEReachable(context.Background(), imsi)

	eventually(t, "both SMSC alerts", func() bool { return len(e.smsc.alerted()) == 2 })

	var got []string
	for _, a := range e.smsc.alerted() {
		got = append(got, a.ServiceCentreAddress)
	}

	slices.Sort(got)

	if want := []string{serviceCentre, otherServiceCentre}; !slices.Equal(got, want) {
		t.Fatalf("alerted %v, want %v", got, want)
	}

	eventually(t, "the waiting data to clear", func() bool { return !e.smsf.Waiting(imsi) })
}

func TestAFailedMemoryAvailableAlertIsRetriedOnReachability(t *testing.T) {
	e := newEnv(t)

	if err := e.store.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre, MemoryFull: true}); err != nil {
		t.Fatal(err)
	}

	e.smsc.setFailAlerts(true)

	smma := &sms.RPSMMA{Reference: 1}
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: moTI(0), UserData: encode(t, smma)}))
	expectCPAck(t, e, moTI(0).Peer())

	if _, ok := expectReport(t, e, moTI(0).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK to RP-SMMA")
	}

	eventually(t, "the failed SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
	time.Sleep(100 * time.Millisecond)

	if !e.smsf.Waiting(imsi) {
		t.Fatal("a failed alert cleared the waiting data")
	}

	e.smsc.setFailAlerts(false)
	e.smsf.UEReachable(context.Background(), imsi)

	eventually(t, "the retried SMSC alert", func() bool { return len(e.smsc.alerted()) == 2 })
	eventually(t, "the waiting data to clear", func() bool { return !e.smsf.Waiting(imsi) })
}

func sendSMMA(t *testing.T, e *env) sms.RPMessage {
	t.Helper()

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: moTI(0), UserData: encode(t, &sms.RPSMMA{Reference: 1})}))
	expectCPAck(t, e, moTI(0).Peer())

	return expectReport(t, e, moTI(0).Peer())
}

func TestMemoryAvailableDuringAnAlertInFlightIsAlerted(t *testing.T) {
	e := newEnv(t)

	if err := e.store.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre, MemoryFull: true}); err != nil {
		t.Fatal(err)
	}

	release := e.smsf.HoldAlertForTest(imsi)

	if _, ok := sendSMMA(t, e).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK to RP-SMMA")
	}

	if w, err := e.store.GetSMSWaiting(context.Background(), imsi); err != nil || w.MemoryFull {
		t.Fatalf("waiting = %+v (%v), want the memory-full flag cleared before the RP-ACK", w, err)
	}

	release()

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
}

func TestMemoryAvailableThatCannotBeRecordedIsRetriedByTheUE(t *testing.T) {
	e := newEnv(t)

	if err := e.store.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre, MemoryFull: true}); err != nil {
		t.Fatal(err)
	}

	e.store.mu.Lock()
	e.store.clearErr = errors.New("leader changed")
	e.store.mu.Unlock()

	rpErr, ok := sendSMMA(t, e).(*sms.RPError)
	if !ok || rpErr.Cause != sms.RPCauseTemporaryFailure {
		t.Fatalf("report = %+v, want RP-ERROR #41 so the UE retries after TRAM", rpErr)
	}

	if w, err := e.store.GetSMSWaiting(context.Background(), imsi); err != nil || !w.MemoryFull {
		t.Fatalf("waiting = %+v (%v), want the memory-full flag kept", w, err)
	}
}
