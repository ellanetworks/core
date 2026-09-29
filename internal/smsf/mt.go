// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

const (
	deliveryTimerMargin = 2 * time.Second
	maxPendingReports   = 4
)

type mtOutcome struct {
	result     uint32
	cause      uint32
	report     []byte
	diagnostic *uint32
}

func delivered(report []byte) mtOutcome {
	return mtOutcome{result: diameter.ResultSuccess, report: report}
}

func deliveryFailure(cause uint32, diagnostic []byte) mtOutcome {
	return mtOutcome{result: tgpp.ResultErrorSMDeliveryFailure, cause: cause, report: diagnostic}
}

func absent(diagnostic uint32) mtOutcome {
	return mtOutcome{result: tgpp.ResultErrorAbsentUser, diagnostic: &diagnostic}
}

func experimental(code uint32) mtOutcome { return mtOutcome{result: code} }

func (o mtOutcome) answer(req *diameter.Message, id diameter.Identity) *diameter.Message {
	switch o.result {
	case diameter.ResultSuccess:
		ans, err := sgd.NewMTForwardShortMessageAnswer(req, id, o.report)
		if err != nil {
			ans, _ = sgd.NewMTForwardShortMessageAnswer(req, id, nil)
		}

		return ans
	case tgpp.ResultErrorSMDeliveryFailure:
		ans, err := sgd.NewDeliveryFailureAnswer(req, id, o.cause, o.report)
		if err != nil {
			ans, _ = sgd.NewDeliveryFailureAnswer(req, id, o.cause, nil)
		}

		return ans
	case tgpp.ResultErrorAbsentUser:
		return sgd.NewAbsentUserAnswer(req, id, o.diagnostic)
	default:
		return tgpp.NewExperimentalAnswer(req, id, o.result)
	}
}

func (s *SMSF) MTForwardShortMessage(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	m, err := sgd.ParseMTForwardShortMessageRequest(req)
	if err != nil {
		return tgpp.NewErrorAnswer(req, id, err)
	}

	deadline := time.Now().Add(s.timers.TR1N)

	if m.DeliveryTimer > 0 {
		start := m.DeliveryStartTime
		if start.IsZero() {
			start = time.Now()
		}

		if d := start.Add(m.DeliveryTimer - deliveryTimerMargin); d.Before(deadline) {
			deadline = d
		}
	}

	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	outcome := s.mobileTerminated(ctx, m.IMSI, m.ServiceCentreAddress, m.SMRPUI)

	s.logger.Info("Mobile-terminated SMS delivery attempt",
		zap.String("imsi", m.IMSI),
		zap.String("service_centre", m.ServiceCentreAddress),
		zap.Uint32("result", outcome.result),
		zap.Bool("more_messages_to_send", m.MoreMessagesToSend),
	)

	return outcome.answer(req, id)
}

func (s *SMSF) mobileTerminated(ctx context.Context, imsi, serviceCentre string, tpdu []byte) mtOutcome {
	if _, err := s.store.GetSubscriber(ctx, imsi); errors.Is(err, db.ErrNotFound) {
		return experimental(tgpp.ResultErrorUserUnknown)
	}

	s.mu.Lock()
	_, registered := s.registered[imsi]
	s.mu.Unlock()

	if !registered {
		if s.servedElsewhere(context.WithoutCancel(ctx), imsi) {
			return experimental(tgpp.ResultErrorUserUnknown)
		}

		s.markWaiting(imsi, serviceCentre, false)

		return absent(tgpp.AbsentUserIMSIDetached)
	}

	s.mu.Lock()
	u := s.ue(imsi)

	if u.mt != nil {
		s.mu.Unlock()
		return experimental(tgpp.ResultErrorUserBusyForMTSMS)
	}

	t := &mtTransaction{
		ti:        sms.TransactionIdentifier{Value: u.nextTI},
		reference: u.nextRef,
		ack:       make(chan struct{}, 1),
		report:    make(chan sms.RPMessage, maxPendingReports),
		aborted:   make(chan struct{}, 1),
		retry:     make(chan struct{}, 1),
	}

	u.mt = t
	u.nextTI = (u.nextTI + 1) % (maxTransactionValue + 1)
	u.nextRef++
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if u, ok := s.ues[imsi]; ok && u.mt == t {
			u.mt = nil
		}
		s.mu.Unlock()

		s.transactionEnded(context.WithoutCancel(ctx), imsi)
	}()

	rpdu, err := (&sms.RPData{
		Direction:  nas.DirectionDownlink,
		Reference:  t.reference,
		Originator: sms.E164Address(serviceCentre),
		UserData:   tpdu,
	}).MarshalBinary()
	if err != nil {
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil)
	}

	err = s.sendReliably(ctx, imsi, &sms.CPData{TransactionIdentifier: t.ti, UserData: rpdu}, t.ack, t.aborted, t.retry)

	if outcome, failed := s.transferFailure(ctx, imsi, serviceCentre, err); failed {
		return outcome
	}

	for {
		select {
		case rp := <-t.report:
			if rp.MessageReference() != t.reference {
				s.logger.Info("Ignored an RP report for another message reference",
					zap.String("imsi", imsi), zap.Uint8("rp_reference", rp.MessageReference()), zap.Uint8("expected", t.reference))

				continue
			}

			return s.reportOutcome(imsi, serviceCentre, rp)
		case <-t.aborted:
			return deliveryFailure(sgd.CauseEquipmentProtocolError, nil)
		case <-ctx.Done():
			return deliveryFailure(sgd.CauseEquipmentProtocolError, nil)
		}
	}
}

func (s *SMSF) transferFailure(ctx context.Context, imsi, serviceCentre string, err error) (mtOutcome, bool) {
	var absentErr *AbsentError

	switch {
	case err == nil:
		return mtOutcome{}, false
	case errors.Is(err, errNoCPAck), errors.Is(err, errAborted):
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil), true
	case errors.Is(err, ErrUserUnknown) && s.servedElsewhere(context.WithoutCancel(ctx), imsi):
		return experimental(tgpp.ResultErrorUserUnknown), true
	case errors.Is(err, ErrUserUnknown):
		s.markWaiting(imsi, serviceCentre, false)
		return absent(tgpp.AbsentUserIMSIDetached), true
	case errors.As(err, &absentErr):
		s.markWaiting(imsi, serviceCentre, false)
		return absent(absentErr.Diagnostic), true
	case errors.Is(err, context.DeadlineExceeded):
		s.markWaiting(imsi, serviceCentre, false)
		return absent(tgpp.AbsentUserNoPagingResponseMSC), true
	case errors.Is(err, ErrNotRegisteredForSMS):
		s.markWaiting(imsi, serviceCentre, false)
		return absent(tgpp.AbsentUserIMSIDetached), true
	default:
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil), true
	}
}

func (s *SMSF) reportOutcome(imsi, serviceCentre string, rp sms.RPMessage) mtOutcome {
	switch m := rp.(type) {
	case *sms.RPAck:
		s.clearWaiting(imsi)
		return delivered(m.UserData)
	case *sms.RPError:
		if m.Cause == sms.RPCauseMemoryCapacityExceeded {
			s.markWaiting(imsi, serviceCentre, true)
			return deliveryFailure(sgd.CauseMemoryCapacityExceeded, m.UserData)
		}

		return deliveryFailure(sgd.CauseEquipmentProtocolError, m.UserData)
	default:
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil)
	}
}
