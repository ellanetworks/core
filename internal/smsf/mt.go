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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const deliveryTimerMargin = 2 * time.Second

type mtOutcome struct {
	result     uint32
	cause      uint32
	report     []byte
	diagnostic *uint32
	reason     string
	err        error
}

func (o mtOutcome) because(reason string, err error) mtOutcome {
	o.reason = reason
	o.err = err

	return o
}

func (o mtOutcome) resultName() string {
	if o.result == diameter.ResultSuccess {
		return diameter.ResultName(o.result)
	}

	return tgpp.Experimental(o.result).Name()
}

func (o mtOutcome) logFields() []zap.Field {
	fields := []zap.Field{
		zap.Uint32("result", o.result),
		zap.String("result_name", o.resultName()),
		zap.String("reason", o.reason),
	}

	switch o.result {
	case tgpp.ResultErrorSMDeliveryFailure:
		fields = append(fields, zap.Uint32("delivery_failure_cause", o.cause))
	case tgpp.ResultErrorAbsentUser:
		if o.diagnostic != nil {
			fields = append(fields, zap.Uint32("absent_user_diagnostic", *o.diagnostic))
		}
	}

	if o.err != nil {
		fields = append(fields, zap.Error(o.err))
	}

	return fields
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

	trace.SpanFromContext(ctx).SetAttributes(attrs.SUPIFromIMSI(m.IMSI))

	deadline := time.Now().Add(s.timers.Paging + s.timers.TR1N)

	if m.DeliveryTimer > 0 {
		start := m.DeliveryStartTime
		if start.IsZero() {
			start = time.Now()
		}

		if d := start.Add(m.DeliveryTimer - deliveryTimerMargin); d.Before(deadline) {
			deadline = d
		}
	}

	ctx, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	defer cancel()

	outcome := s.mobileTerminated(ctx, m.IMSI, m.ServiceCentreAddress, m.SMRPUI, m.MoreMessagesToSend)

	log := logger.From(ctx, s.logger,
		zap.String("imsi", m.IMSI),
		zap.String("service_centre", m.ServiceCentreAddress),
		zap.Int("tpdu_length", len(m.SMRPUI)),
		zap.Bool("more_messages_to_send", m.MoreMessagesToSend),
	)

	if outcome.result == diameter.ResultSuccess {
		log.Info("Delivered a mobile-terminated SMS")
	} else {
		log.Warn("Could not deliver a mobile-terminated SMS", outcome.logFields()...)
	}

	return outcome.answer(req, id)
}

func (s *SMSF) mobileTerminated(ctx context.Context, imsi, serviceCentre string, tpdu []byte, moreMessages bool) (outcome mtOutcome) {
	if _, err := s.store.GetSubscriber(ctx, imsi); errors.Is(err, db.ErrNotFound) {
		return experimental(tgpp.ResultErrorUserUnknown).because("IMSI is not a subscriber", nil)
	}

	s.mu.Lock()
	u := s.ue(imsi)

	if u.mt != nil {
		s.mu.Unlock()
		return experimental(tgpp.ResultErrorUserBusyForMTSMS).because("another mobile-terminated SMS is in progress", nil)
	}

	t := &mtTransaction{
		cpTxn:         newCPTxn(),
		ti:            sms.TransactionIdentifier{Value: u.nextTI},
		reference:     u.nextRef,
		report:        make(chan sms.RPMessage, 1),
		serviceCentre: serviceCentre,
		span:          trace.SpanFromContext(ctx),
	}

	u.mt = t
	u.stopHoldingForMoreMessages()
	u.nextTI = (u.nextTI + 1) % (maxTransactionValue + 1)
	u.nextRef++
	s.mu.Unlock()

	defer func() {
		var alert bool

		s.mu.Lock()
		if u, ok := s.ues[imsi]; ok && u.mt == t {
			u.mt = nil
			alert, u.deferredAlert = u.deferredAlert, false

			if moreMessages && outcome.result == diameter.ResultSuccess {
				s.holdForMoreMessagesLocked(ctx, imsi, u)
			}
		}
		s.mu.Unlock()

		s.transactionEnded(context.WithoutCancel(ctx), imsi)

		if alert {
			s.startAlert(context.WithoutCancel(ctx), imsi)
		}
	}()

	rpdu, err := (&sms.RPData{
		Direction:  nas.DirectionDownlink,
		Reference:  t.reference,
		Originator: sms.E164Address(serviceCentre),
		UserData:   tpdu,
	}).MarshalBinary()
	if err != nil {
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("RP-DATA cannot be encoded", err)
	}

	err = s.sendReliably(ctx, imsi, &sms.CPData{TransactionIdentifier: t.ti, UserData: rpdu}, &t.cpTxn)

	if outcome, failed := s.transferFailure(ctx, imsi, serviceCentre, err); failed {
		return outcome
	}

	s.mu.Lock()
	tr1n := time.NewTimer(time.Until(t.sentAt.Add(s.timers.TR1N)))
	s.mu.Unlock()

	defer tr1n.Stop()

	select {
	case rp := <-t.report:
		return reportOutcome(rp)
	case <-t.aborted:
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("UE aborted the transaction with CP-ERROR", nil)
	case <-tr1n.C:
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("UE sent no RP report before TR1N expired", nil)
	case <-ctx.Done():
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("delivery deadline expired before the UE sent an RP report", ctx.Err())
	}
}

func (s *SMSF) transferFailure(ctx context.Context, imsi, serviceCentre string, err error) (mtOutcome, bool) {
	switch {
	case err == nil:
		return mtOutcome{}, false
	case errors.Is(err, errAborted):
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("UE aborted the transaction with CP-ERROR", err), true
	case errors.Is(err, ErrNotRegisteredForSMS) && s.servedElsewhere(context.WithoutCancel(ctx), imsi):
		return experimental(tgpp.ResultErrorUserUnknown).because("UE is served by another node", err), true
	case errors.Is(err, ErrNotRegisteredForSMS):
		s.markWaiting(ctx, imsi, serviceCentre)
		return absent(tgpp.AbsentUserIMSIDetached).because("UE is not registered for SMS", err), true
	case errors.Is(err, errNoCPAck):
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("UE did not acknowledge the CP-DATA", err), true
	case errors.Is(err, ErrUnreachable), errors.Is(err, context.DeadlineExceeded):
		s.markWaiting(ctx, imsi, serviceCentre)
		return absent(tgpp.AbsentUserNoPagingResponseMSC).because("UE did not respond to paging", err), true
	default:
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("could not send the SMS to the UE", err), true
	}
}

func reportOutcome(rp sms.RPMessage) mtOutcome {
	switch m := rp.(type) {
	case *sms.RPAck:
		return delivered(m.UserData)
	case *sms.RPError:
		reason := "UE rejected the message with RP-ERROR (" + m.Cause.String() + ")"

		if m.Cause == sms.RPCauseMemoryCapacityExceeded {
			return deliveryFailure(sgd.CauseMemoryCapacityExceeded, m.UserData).because(reason, nil)
		}

		return deliveryFailure(sgd.CauseEquipmentProtocolError, m.UserData).because(reason, nil)
	default:
		return deliveryFailure(sgd.CauseEquipmentProtocolError, nil).because("UE sent an unexpected RP report", nil)
	}
}
