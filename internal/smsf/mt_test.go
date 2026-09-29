// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/smsf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

var (
	deliverTPDU = []byte{0x04, 0x0b, 0x91, 0x51, 0x55, 0x21, 0x03, 0x00, 0xf1, 0x00, 0x00, 0x62, 0x90, 0x92, 0x21, 0x00, 0x00, 0x00, 0x02, 0xe8, 0x34}
	deliverRpt  = []byte{0x00, 0x00}
)

type forwarded struct {
	answer sgd.Answer
	err    error
}

func forwardAsync(t *testing.T, e *env) <-chan forwarded {
	t.Helper()

	out := make(chan forwarded, 1)

	go func() {
		ans, err := e.smsc.forward(t, deliverTPDU)
		if err != nil {
			out <- forwarded{err: err}
			return
		}

		a, err := sgd.ParseMTForwardShortMessageAnswer(ans)
		out <- forwarded{answer: a, err: err}
	}()

	return out
}

func awaitTFA(t *testing.T, ch <-chan forwarded) forwarded {
	t.Helper()

	select {
	case f := <-ch:
		return f
	case <-time.After(waitTimeout):
		t.Fatal("timed out waiting for the TFA")
		return forwarded{}
	}
}

func receiveRPData(t *testing.T, e *env) (*sms.CPData, *sms.RPData) {
	t.Helper()

	data, rp := e.ue.nextData(t)

	rpData, ok := rp.(*sms.RPData)
	if !ok {
		t.Fatalf("expected RP-DATA, got %T", rp)
	}

	return data, rpData
}

func answerMT(t *testing.T, e *env, data *sms.CPData, rp sms.RPMessage) {
	t.Helper()

	ti := data.TransactionIdentifier.Peer()

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPAck{TransactionIdentifier: ti}))
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: ti, UserData: encode(t, rp)}))

	expectCPAck(t, e, data.TransactionIdentifier)
}

func failure(t *testing.T, f forwarded) *sgd.ResultError {
	t.Helper()

	var re *sgd.ResultError
	if !errors.As(f.err, &re) {
		t.Fatalf("TFA error = %v, want a result error", f.err)
	}

	return re
}

func TestMobileTerminatedSMSIsDelivered(t *testing.T) {
	e := newEnv(t)
	tfa := forwardAsync(t, e)

	data, rp := receiveRPData(t, e)
	if data.TransactionIdentifier.Flag || rp.Originator == nil || rp.Originator.Digits != serviceCentre || !bytes.Equal(rp.UserData, deliverTPDU) {
		t.Fatalf("CP-DATA %s carrying %+v", data.TransactionIdentifier, rp)
	}

	answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference, UserData: deliverRpt})

	f := awaitTFA(t, tfa)
	if f.err != nil || !bytes.Equal(f.answer.SMRPUI, deliverRpt) {
		t.Fatalf("TFA = %+v %v, want success carrying the delivery report", f.answer, f.err)
	}
}

func TestMemoryFullUEIsReportedThenAlertedOnMemoryAvailable(t *testing.T) {
	e := newEnv(t)
	tfa := forwardAsync(t, e)

	data, rp := receiveRPData(t, e)
	answerMT(t, e, data, &sms.RPError{Direction: nas.DirectionUplink, Reference: rp.Reference, Cause: sms.RPCauseMemoryCapacityExceeded, UserData: deliverRpt})

	re := failure(t, awaitTFA(t, tfa))
	if !re.IsExperimental(tgpp.ResultErrorSMDeliveryFailure) || re.DeliveryFailureCause == nil ||
		*re.DeliveryFailureCause != sgd.CauseMemoryCapacityExceeded || !bytes.Equal(re.DiagnosticInfo, deliverRpt) {
		t.Fatalf("TFA = %+v, want SM delivery failure (memory capacity exceeded)", re)
	}

	e.smsf.UEReachable(context.Background(), imsi)
	time.Sleep(200 * time.Millisecond)

	if n := len(e.smsc.alerted()); n != 0 {
		t.Fatalf("SMSC alerted on reachability while the UE memory is full")
	}

	smma := &sms.RPSMMA{Reference: 1}
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: moTI(0), UserData: encode(t, smma)}))
	expectCPAck(t, e, moTI(0).Peer())

	if _, ok := expectReport(t, e, moTI(0).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK to RP-SMMA")
	}

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })

	alert := e.smsc.alerted()[0]
	if alert.User.MSISDN != msisdn || alert.User.IMSI != "" || alert.ServiceCentreAddress != serviceCentre {
		t.Fatalf("ALR = %+v", alert)
	}

	eventually(t, "the waiting flag to clear", func() bool { return !e.smsf.Waiting(imsi) })
}

func TestAbsentUEIsReportedThenAlertedWhenReachable(t *testing.T) {
	e := newEnv(t)
	e.ue.fail(&smsf.AbsentError{Diagnostic: tgpp.AbsentUserIMSIDetached})

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.AbsentUserDiagnostic == nil || *re.AbsentUserDiagnostic != tgpp.AbsentUserIMSIDetached {
		t.Fatalf("TFA = %+v, want absent user (IMSI detached)", re)
	}

	if !e.smsf.Waiting(imsi) {
		t.Fatal("no waiting flag after an absent-user failure")
	}

	e.ue.fail(nil)
	e.smsf.UEReachable(context.Background(), imsi)

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
	eventually(t, "the waiting flag to clear", func() bool { return !e.smsf.Waiting(imsi) })

	e.smsf.UEReachable(context.Background(), imsi)
	time.Sleep(200 * time.Millisecond)

	if n := len(e.smsc.alerted()); n != 1 {
		t.Fatalf("SMSC alerted %d times, want 1", n)
	}
}

func TestUnreachedUEIsAbsentWithNoPagingResponse(t *testing.T) {
	e := newEnv(t)
	e.ue.fail(context.DeadlineExceeded)

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.AbsentUserDiagnostic == nil || *re.AbsentUserDiagnostic != tgpp.AbsentUserNoPagingResponseMSC {
		t.Fatalf("TFA = %+v, want absent user (no paging response)", re)
	}
}

func TestMobileTerminatedSMSToAUEServedByAnotherNode(t *testing.T) {
	e := newEnv(t)
	e.store.register(db.UERegistrationTypeMME, remoteNode, false)
	e.ue.fail(smsf.ErrUserUnknown)

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorUserUnknown) {
		t.Fatalf("TFA = %+v, want user unknown", re)
	}

	if e.smsf.Waiting(imsi) {
		t.Fatal("waiting flag set for a UE another node serves")
	}
}

func TestSwitchedOffUEIsAlertedWhenItComesBack(t *testing.T) {
	e := newEnv(t)
	e.store.register(db.UERegistrationTypeMME, localNode, true)
	e.ue.fail(smsf.ErrUserUnknown)

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.AbsentUserDiagnostic == nil || *re.AbsentUserDiagnostic != tgpp.AbsentUserIMSIDetached {
		t.Fatalf("TFA = %+v, want absent user (IMSI detached)", re)
	}

	e.ue.fail(nil)
	e.smsf.UEReachable(context.Background(), imsi)

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
}

func TestMobileTerminatedSMSToAnIMSIThatIsNotASubscriber(t *testing.T) {
	e := newEnv(t)

	e.store.mu.Lock()
	delete(e.store.subscribers, imsi)
	e.store.mu.Unlock()

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorUserUnknown) {
		t.Fatalf("TFA = %+v, want user unknown", re)
	}

	if n := e.smsf.TrackedUEs(); n != 0 {
		t.Fatalf("SMSF tracks %d UEs after a TFR for an unknown IMSI", n)
	}
}

func TestMobileTerminatedSMSToAUENotRegisteredForSMSIsAbsent(t *testing.T) {
	e := newEnv(t)
	e.ue.fail(smsf.ErrNotRegisteredForSMS)

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.AbsentUserDiagnostic == nil || *re.AbsentUserDiagnostic != tgpp.AbsentUserIMSIDetached {
		t.Fatalf("TFA = %+v, want absent user (IMSI detached)", re)
	}

	if !e.smsf.Waiting(imsi) {
		t.Fatal("no waiting flag for a UE not registered for SMS")
	}

	e.ue.fail(nil)
	e.smsf.UEReachable(context.Background(), imsi)

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
}

func TestMobileTerminatedSMSToAUENotRegisteredForSMSServedElsewhere(t *testing.T) {
	e := newEnv(t)
	e.ue.fail(smsf.ErrNotRegisteredForSMS)
	e.store.register(db.UERegistrationTypeMME, remoteNode, false)

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorUserUnknown) {
		t.Fatalf("TFA = %+v, want user unknown", re)
	}

	if e.smsf.Waiting(imsi) {
		t.Fatal("waiting flag set on the wrong node")
	}
}

func TestSilentUEGetsRetransmissionsThenProtocolError(t *testing.T) {
	e := newEnv(t)
	tfa := forwardAsync(t, e)

	for i := 0; i <= fastTimers().MaxRetransmissions; i++ {
		receiveRPData(t, e)
	}

	re := failure(t, awaitTFA(t, tfa))
	if !re.IsExperimental(tgpp.ResultErrorSMDeliveryFailure) || re.DeliveryFailureCause == nil || *re.DeliveryFailureCause != sgd.CauseEquipmentProtocolError {
		t.Fatalf("TFA = %+v, want SM delivery failure (equipment protocol error)", re)
	}
}

func TestUEAbortWithCPError(t *testing.T) {
	e := newEnv(t)
	tfa := forwardAsync(t, e)

	data, _ := receiveRPData(t, e)
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPError{TransactionIdentifier: data.TransactionIdentifier.Peer(), Cause: sms.CPCauseProtocolErrorUnspecified}))

	re := failure(t, awaitTFA(t, tfa))
	if *re.DeliveryFailureCause != sgd.CauseEquipmentProtocolError {
		t.Fatalf("TFA = %+v, want equipment protocol error", re)
	}
}

func TestConcurrentMobileTerminatedSMSIsBusy(t *testing.T) {
	e := newEnv(t)
	first := forwardAsync(t, e)

	data, rp := receiveRPData(t, e)

	re := failure(t, awaitTFA(t, forwardAsync(t, e)))
	if !re.IsExperimental(tgpp.ResultErrorUserBusyForMTSMS) {
		t.Fatalf("second TFA = %+v, want user busy for MT SMS", re)
	}

	answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference})

	if f := awaitTFA(t, first); f.err != nil {
		t.Fatalf("first TFA = %v", f.err)
	}
}

func TestReportForUnknownTransactionIsRejected(t *testing.T) {
	e := newEnv(t)

	ti := sms.TransactionIdentifier{Value: 5, Flag: true}
	ack := &sms.RPAck{Direction: nas.DirectionUplink, Reference: 1}
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: ti, UserData: encode(t, ack)}))

	cpErr, ok := e.ue.next(t).(*sms.CPError)
	if !ok || cpErr.Cause != sms.CPCauseInvalidTransactionIdentifier || cpErr.TransactionIdentifier != ti.Peer() {
		t.Fatalf("expected CP-ERROR #81, got %+v", cpErr)
	}
}

func TestSuccessfulDeliveryClearsWaiting(t *testing.T) {
	e := newEnv(t)
	e.ue.fail(&smsf.AbsentError{Diagnostic: tgpp.AbsentUserIMSIDetached})
	_ = failure(t, awaitTFA(t, forwardAsync(t, e)))

	e.ue.fail(nil)
	e.smsc.setAnswer(nil)

	tfa := forwardAsync(t, e)
	data, rp := receiveRPData(t, e)
	answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference})

	if f := awaitTFA(t, tfa); f.err != nil {
		t.Fatalf("TFA = %v", f.err)
	}

	if e.smsf.Waiting(imsi) {
		t.Fatal("waiting flag survived a successful delivery")
	}
}

func TestTFRWithoutMandatoryAVPsIsRejected(t *testing.T) {
	e := newEnv(t)

	req := &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   sgd.CommandMTForwardShortMessage,
		ApplicationID: sgd.ApplicationID,
		AVPs: tgpp.Envelope{
			SessionID: e.smsc.node.NewSessionID(), Origin: e.smsc.node.Identity(),
			DestinationHost: localIdent.Host, DestinationRealm: localIdent.Realm,
		}.AVPs(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	ans, err := e.smsc.node.DoHost(ctx, localIdent.Host, req)
	if err != nil {
		t.Fatalf("TFR: %v", err)
	}

	result, err := tgpp.ParseResult(ans)
	if err != nil || result.Code != diameter.ResultMissingAVP {
		t.Fatalf("result = %v %v, want missing AVP", result, err)
	}
}

func TestSuccessiveMobileTerminatedSMSUseNewIdentifiers(t *testing.T) {
	e := newEnv(t)

	seen := map[sms.TransactionIdentifier]bool{}
	refs := map[uint8]bool{}

	for i := 0; i < 3; i++ {
		tfa := forwardAsync(t, e)

		data, rp := receiveRPData(t, e)
		if seen[data.TransactionIdentifier] || refs[rp.Reference] {
			t.Fatalf("transfer %d reused TI %s or RP reference %d", i, data.TransactionIdentifier, rp.Reference)
		}

		seen[data.TransactionIdentifier] = true
		refs[rp.Reference] = true

		answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference})

		if f := awaitTFA(t, tfa); f.err != nil {
			t.Fatalf("TFA %d = %v", i, f.err)
		}
	}
}

func TestCPTimeoutAfterReachingTheUEIsAProtocolError(t *testing.T) {
	timers := fastTimers()
	timers.TR1N = 300 * time.Millisecond

	e := newEnvWithTimers(t, timers)
	tfa := forwardAsync(t, e)

	receiveRPData(t, e)

	re := failure(t, awaitTFA(t, tfa))
	if !re.IsExperimental(tgpp.ResultErrorSMDeliveryFailure) || *re.DeliveryFailureCause != sgd.CauseEquipmentProtocolError {
		t.Fatalf("TFA = %+v, want equipment protocol error rather than absent user", re)
	}

	if e.smsf.Waiting(imsi) {
		t.Fatal("waiting flag set although the UE was reached")
	}
}

func TestReportWithAnotherReferenceIsIgnored(t *testing.T) {
	e := newEnv(t)
	tfa := forwardAsync(t, e)

	data, rp := receiveRPData(t, e)
	answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference + 1})

	ti := data.TransactionIdentifier.Peer()
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: ti, UserData: encode(t, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference})}))
	expectCPAck(t, e, data.TransactionIdentifier)

	if f := awaitTFA(t, tfa); f.err != nil {
		t.Fatalf("TFA = %v, want success from the matching report", f.err)
	}
}

func TestNonDeliveryRetransmitsAtOnce(t *testing.T) {
	timers := fastTimers()
	timers.TC1 = time.Hour
	e := newEnvWithTimers(t, timers)
	tfa := forwardAsync(t, e)

	first, _ := receiveRPData(t, e)

	e.smsf.DeliveryFailed(imsi)

	again, rp := receiveRPData(t, e)
	if again.TransactionIdentifier != first.TransactionIdentifier {
		t.Fatalf("retransmission on %s, want %s", again.TransactionIdentifier, first.TransactionIdentifier)
	}

	answerMT(t, e, again, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference, UserData: deliverRpt})

	if f := awaitTFA(t, tfa); f.err != nil {
		t.Fatalf("TFA: %v", f.err)
	}
}

func TestAnAlertDoesNotRaceADeliveryInProgress(t *testing.T) {
	e := newEnv(t)
	e.ue.fail(&smsf.AbsentError{Diagnostic: tgpp.AbsentUserIMSIDetached})
	awaitTFA(t, forwardAsync(t, e))
	e.ue.fail(nil)

	if !e.smsf.Waiting(imsi) {
		t.Fatal("precondition: waiting flag")
	}

	gate := e.ue.holdReachability()
	tfa := forwardAsync(t, e)

	eventually(t, "the redelivery to start", func() bool { return e.ue.reachability() == 2 })

	e.smsf.UEReachable(context.Background(), imsi)
	close(gate)

	data, rp := receiveRPData(t, e)
	answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference, UserData: deliverRpt})

	if f := awaitTFA(t, tfa); f.err != nil {
		t.Fatalf("TFA: %v", f.err)
	}

	time.Sleep(200 * time.Millisecond)

	if n := len(e.smsc.alerted()); n != 0 {
		t.Fatalf("SMSC alerted %d times during a delivery that succeeded", n)
	}
}

func TestNonDeliveryBeforeTheCPDataIsSentDoesNotRetransmit(t *testing.T) {
	timers := fastTimers()
	timers.TC1 = time.Hour
	e := newEnvWithTimers(t, timers)

	gate := e.ue.holdReachability()
	tfa := forwardAsync(t, e)

	eventually(t, "the UE to be reached", func() bool { return e.ue.reachability() == 1 })

	e.smsf.DeliveryFailed(imsi)
	close(gate)

	data, rp := receiveRPData(t, e)

	select {
	case m := <-e.ue.downlink:
		t.Fatalf("spurious retransmission %s", m.MessageType())
	case <-time.After(200 * time.Millisecond):
	}

	answerMT(t, e, data, &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference, UserData: deliverRpt})

	if f := awaitTFA(t, tfa); f.err != nil {
		t.Fatalf("TFA: %v", f.err)
	}
}

func TestMemoryAvailableRightAfterAMemoryFullReportAlerts(t *testing.T) {
	for range 20 {
		e := newEnv(t)
		tfa := forwardAsync(t, e)

		data, rp := receiveRPData(t, e)
		answerMT(t, e, data, &sms.RPError{Direction: nas.DirectionUplink, Reference: rp.Reference, Cause: sms.RPCauseMemoryCapacityExceeded})

		e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: moTI(0), UserData: encode(t, &sms.RPSMMA{Reference: 1})}))
		expectCPAck(t, e, moTI(0).Peer())

		if _, ok := expectReport(t, e, moTI(0).Peer()).(*sms.RPAck); !ok {
			t.Fatal("expected RP-ACK to RP-SMMA")
		}

		awaitTFA(t, tfa)

		eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
		eventually(t, "the waiting flag to clear", func() bool { return !e.smsf.Waiting(imsi) })
	}
}

func TestAnAbsentAttemptKeepsTheMemoryFullFlag(t *testing.T) {
	e := newEnv(t)
	tfa := forwardAsync(t, e)

	data, rp := receiveRPData(t, e)
	answerMT(t, e, data, &sms.RPError{Direction: nas.DirectionUplink, Reference: rp.Reference, Cause: sms.RPCauseMemoryCapacityExceeded})
	awaitTFA(t, tfa)

	e.ue.fail(&smsf.AbsentError{Diagnostic: tgpp.AbsentUserNoPagingResponseMSC})
	awaitTFA(t, forwardAsync(t, e))
	e.ue.fail(nil)

	e.smsf.UEReachable(context.Background(), imsi)
	time.Sleep(200 * time.Millisecond)

	if n := len(e.smsc.alerted()); n != 0 {
		t.Fatal("SMSC alerted on reachability while the UE memory is still full")
	}
}
