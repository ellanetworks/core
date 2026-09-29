// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

var submitTPDU = []byte{0x01, 0x00, 0x0b, 0x91, 0x51, 0x55, 0x21, 0x03, 0x00, 0xf2, 0x00, 0x00, 0x02, 0xe8, 0x34}

func moTI(value uint8) sms.TransactionIdentifier {
	return sms.TransactionIdentifier{Value: value}
}

func sendRPData(t *testing.T, e *env, ti sms.TransactionIdentifier, reference uint8) {
	t.Helper()

	rp := &sms.RPData{
		Direction:   nas.DirectionUplink,
		Reference:   reference,
		Destination: sms.E164Address(serviceCentre),
		UserData:    submitTPDU,
	}

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: ti, UserData: encode(t, rp)}))
}

func expectCPAck(t *testing.T, e *env, ti sms.TransactionIdentifier) {
	t.Helper()

	ack, ok := e.ue.next(t).(*sms.CPAck)
	if !ok || ack.TransactionIdentifier != ti {
		t.Fatalf("expected CP-ACK on %s, got %+v", ti, ack)
	}
}

func expectReport(t *testing.T, e *env, ti sms.TransactionIdentifier) sms.RPMessage {
	t.Helper()

	data, rp := e.ue.nextData(t)
	if data.TransactionIdentifier != ti {
		t.Fatalf("report on TI %s, want %s", data.TransactionIdentifier, ti)
	}

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPAck{TransactionIdentifier: ti.Peer()}))

	return rp
}

func TestMobileOriginatedSMSIsForwardedToTheSMSC(t *testing.T) {
	e := newEnv(t)

	sendRPData(t, e, moTI(1), 7)

	expectCPAck(t, e, moTI(1).Peer())

	ack, ok := expectReport(t, e, moTI(1).Peer()).(*sms.RPAck)
	if !ok || ack.Reference != 7 || !bytes.Equal(ack.UserData, []byte{0x01, 0x00}) {
		t.Fatalf("expected RP-ACK 7 carrying the SMSC report, got %+v", ack)
	}

	ofrs := e.smsc.submitted()
	if len(ofrs) != 1 {
		t.Fatalf("SMSC received %d OFRs, want 1", len(ofrs))
	}

	ofr := ofrs[0]
	if ofr.ServiceCentreAddress != serviceCentre || ofr.User.IMSI != imsi || ofr.User.MSISDN != msisdn || !bytes.Equal(ofr.SMRPUI, submitTPDU) {
		t.Fatalf("OFR = %+v", ofr)
	}

	e.ue.expectNothing(t, 3*fastTimers().TC1)
}

func TestMobileOriginatedSMSMapsSMSCFailures(t *testing.T) {
	cases := []struct {
		name   string
		answer func(req *diameter.Message, id diameter.Identity) *diameter.Message
		cause  sms.RPCause
		report []byte
	}{
		{
			name: "unknown service centre",
			answer: func(req *diameter.Message, id diameter.Identity) *diameter.Message {
				ans, _ := sgd.NewDeliveryFailureAnswer(req, id, sgd.CauseUnknownServiceCentre, []byte{0x01, 0xc1})
				return ans
			},
			cause:  sms.RPCauseUnassignedNumber,
			report: []byte{0x01, 0xc1},
		},
		{
			name: "SC congestion",
			answer: func(req *diameter.Message, id diameter.Identity) *diameter.Message {
				ans, _ := sgd.NewDeliveryFailureAnswer(req, id, sgd.CauseSCCongestion, nil)
				return ans
			},
			cause: sms.RPCauseCongestion,
		},
		{
			name: "invalid SME address",
			answer: func(req *diameter.Message, id diameter.Identity) *diameter.Message {
				ans, _ := sgd.NewDeliveryFailureAnswer(req, id, sgd.CauseInvalidSMEAddress, nil)
				return ans
			},
			cause: sms.RPCauseShortMessageTransferRejected,
		},
		{
			name: "not an SC user",
			answer: func(req *diameter.Message, id diameter.Identity) *diameter.Message {
				ans, _ := sgd.NewDeliveryFailureAnswer(req, id, sgd.CauseUserNotSCUser, nil)
				return ans
			},
			cause: sms.RPCauseUnidentifiedSubscriber,
		},
		{
			name: "facility not supported",
			answer: func(req *diameter.Message, id diameter.Identity) *diameter.Message {
				return tgpp.NewExperimentalAnswer(req, id, tgpp.ResultErrorFacilityNotSupported)
			},
			cause: sms.RPCauseRequestedFacilityNotImplemented,
		},
		{
			name: "unable to comply",
			answer: func(req *diameter.Message, id diameter.Identity) *diameter.Message {
				return tgpp.NewAnswer(req, id, diameter.ResultUnableToComply)
			},
			cause: sms.RPCauseNetworkOutOfOrder,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.smsc.setAnswer(tc.answer)

			sendRPData(t, e, moTI(2), 9)
			expectCPAck(t, e, moTI(2).Peer())

			rpErr, ok := expectReport(t, e, moTI(2).Peer()).(*sms.RPError)
			if !ok || rpErr.Reference != 9 || rpErr.Cause != tc.cause || !bytes.Equal(rpErr.UserData, tc.report) {
				t.Fatalf("RP-ERROR = %+v, want cause %s with report %x", rpErr, tc.cause, tc.report)
			}
		})
	}
}

func TestMobileOriginatedSMSWithoutMSISDNIsRejected(t *testing.T) {
	e := newEnv(t)
	e.store.setMSISDN("")

	sendRPData(t, e, moTI(0), 1)
	expectCPAck(t, e, moTI(0).Peer())

	rpErr, ok := expectReport(t, e, moTI(0).Peer()).(*sms.RPError)
	if !ok || rpErr.Cause != sms.RPCauseRequestedFacilityNotSubscribed {
		t.Fatalf("RP-ERROR = %+v, want cause 50", rpErr)
	}

	if n := len(e.smsc.submitted()); n != 0 {
		t.Fatalf("SMSC received %d OFRs", n)
	}
}

func TestMobileOriginatedSMSWhileSMSCUnreachableIsTemporaryFailure(t *testing.T) {
	e := newEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	if err := e.smsc.node.Shutdown(ctx); err != nil {
		t.Fatalf("stop SMSC: %v", err)
	}

	sendRPData(t, e, moTI(3), 4)
	expectCPAck(t, e, moTI(3).Peer())

	rpErr, ok := expectReport(t, e, moTI(3).Peer()).(*sms.RPError)
	if !ok || rpErr.Cause != sms.RPCauseNetworkOutOfOrder {
		t.Fatalf("RP-ERROR = %+v, want cause 38", rpErr)
	}
}

func TestRetransmittedCPDataIsForwardedOnce(t *testing.T) {
	e := newEnv(t)

	e.smsc.setAnswer(func(req *diameter.Message, id diameter.Identity) *diameter.Message {
		sendRPData(t, e, moTI(4), 5)

		ans, _ := sgd.NewMOForwardShortMessageAnswer(req, id, nil)

		return ans
	})

	sendRPData(t, e, moTI(4), 5)
	expectCPAck(t, e, moTI(4).Peer())
	expectCPAck(t, e, moTI(4).Peer())

	if _, ok := expectReport(t, e, moTI(4).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK")
	}

	if n := len(e.smsc.submitted()); n != 1 {
		t.Fatalf("SMSC received %d OFRs, want 1", n)
	}
}

func TestReportIsRetransmittedUntilAcknowledged(t *testing.T) {
	e := newEnv(t)

	sendRPData(t, e, moTI(5), 6)
	expectCPAck(t, e, moTI(5).Peer())

	first, _ := e.ue.nextData(t)
	second, _ := e.ue.nextData(t)

	if first.TransactionIdentifier != second.TransactionIdentifier {
		t.Fatalf("retransmission changed the TI: %s then %s", first.TransactionIdentifier, second.TransactionIdentifier)
	}

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPAck{TransactionIdentifier: moTI(5)}))
	e.ue.expectNothing(t, 3*fastTimers().TC1)
}

func TestInvalidRPMessageIsAnsweredWithRPError(t *testing.T) {
	e := newEnv(t)

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: moTI(6), UserData: []byte{0x07, 0x21}}))
	expectCPAck(t, e, moTI(6).Peer())

	rpErr, ok := expectReport(t, e, moTI(6).Peer()).(*sms.RPError)
	if !ok || rpErr.Reference != 0x21 || rpErr.Cause != sms.RPCauseMessageTypeNonExistent {
		t.Fatalf("RP-ERROR = %+v, want reference 0x21 cause 97", rpErr)
	}
}

func TestMemoryAvailableIsAcknowledged(t *testing.T) {
	e := newEnv(t)

	smma := &sms.RPSMMA{Reference: 3}
	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPData{TransactionIdentifier: moTI(0), UserData: encode(t, smma)}))
	expectCPAck(t, e, moTI(0).Peer())

	ack, ok := expectReport(t, e, moTI(0).Peer()).(*sms.RPAck)
	if !ok || ack.Reference != 3 {
		t.Fatalf("expected RP-ACK 3, got %+v", ack)
	}

	if n := len(e.smsc.alerted()); n != 0 {
		t.Fatalf("SMSC alerted %d times with nothing waiting", n)
	}
}

func TestNextMobileOriginatedSMSImplicitlyAcknowledgesThePreviousReport(t *testing.T) {
	e := newEnv(t)

	sendRPData(t, e, moTI(1), 1)
	expectCPAck(t, e, moTI(1).Peer())

	if data, _ := e.ue.nextData(t); data.TransactionIdentifier != moTI(1).Peer() {
		t.Fatalf("report on %s", data.TransactionIdentifier)
	}

	sendRPData(t, e, moTI(2), 2)
	expectCPAck(t, e, moTI(2).Peer())

	if _, ok := expectReport(t, e, moTI(2).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK for the second message")
	}

	select {
	case m := <-e.ue.downlink:
		t.Fatalf("the first report was retransmitted after the next CP-DATA: %s on %s", m.MessageType(), m.TI())
	case <-time.After(3 * fastTimers().TC1):
	}
}

func TestOnlyTheReportWaitsForTheUEToBeReachable(t *testing.T) {
	e := newEnv(t)

	reachedBeforeReport := make(chan int, 1)

	e.smsc.setAnswer(func(req *diameter.Message, id diameter.Identity) *diameter.Message {
		reachedBeforeReport <- e.ue.reachability()

		ans, _ := sgd.NewMOForwardShortMessageAnswer(req, id, nil)

		return ans
	})

	sendRPData(t, e, moTI(1), 7)
	expectCPAck(t, e, moTI(1).Peer())

	if n := <-reachedBeforeReport; n != 0 {
		t.Fatalf("the CP-ACK asked for UE reachability %d times", n)
	}

	if _, ok := expectReport(t, e, moTI(1).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK")
	}

	if n := e.ue.reachability(); n != 1 {
		t.Fatalf("the report asked for UE reachability %d times, want 1", n)
	}
}

func TestAPendingTransactionHoldsTheConnectionUntilItEnds(t *testing.T) {
	e := newEnv(t)

	answered := make(chan struct{})

	e.smsc.setAnswer(func(req *diameter.Message, id diameter.Identity) *diameter.Message {
		<-answered

		ans, _ := sgd.NewMOForwardShortMessageAnswer(req, id, nil)

		return ans
	})

	sendRPData(t, e, moTI(1), 7)
	expectCPAck(t, e, moTI(1).Peer())

	if !e.smsf.TransactionPending(imsi) {
		t.Fatal("no transaction pending while the SMSC has not answered")
	}

	if n := e.ue.settlements(); n != 0 {
		t.Fatalf("signalling settled %d times with a transaction open", n)
	}

	close(answered)

	if _, ok := expectReport(t, e, moTI(1).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK")
	}

	eventually(t, "the transaction to end", func() bool { return !e.smsf.TransactionPending(imsi) })
	eventually(t, "the signalling to settle", func() bool { return e.ue.settlements() == 1 })
}

func TestTheConnectionIsReleasableOnceTheReportIsSent(t *testing.T) {
	e := newEnv(t)

	sendRPData(t, e, moTI(1), 7)
	expectCPAck(t, e, moTI(1).Peer())

	if data, _ := e.ue.nextData(t); data.TransactionIdentifier != moTI(1).Peer() {
		t.Fatalf("report on %s", data.TransactionIdentifier)
	}

	eventually(t, "the connection to be releasable", func() bool { return !e.smsf.TransactionPending(imsi) })

	if n := e.ue.settlements(); n != 1 {
		t.Fatalf("signalling settled %d times, want once", n)
	}

	e.smsf.Uplink(context.Background(), imsi, encode(t, &sms.CPAck{TransactionIdentifier: moTI(1)}))
}

func TestTheReportIsSentBeforeTheConnectionIsReleasable(t *testing.T) {
	e := newEnv(t)

	sendRPData(t, e, moTI(1), 7)
	expectCPAck(t, e, moTI(1).Peer())

	if _, ok := expectReport(t, e, moTI(1).Peer()).(*sms.RPAck); !ok {
		t.Fatal("expected RP-ACK")
	}

	eventually(t, "the signalling to settle", func() bool { return e.ue.settlements() >= 1 })

	events := e.ue.events()
	report, settled := -1, slices.Index(events, "settled")

	for i, ev := range events {
		if ev == sms.CPMessageTypeData.String() {
			report = i
		}
	}

	if report < 0 || settled < report {
		t.Fatalf("events = %v: the report must reach the core before the connection may be released", events)
	}
}
