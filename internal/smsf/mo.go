// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

func (s *SMSF) mobileOriginated(ctx context.Context, imsi string, ti sms.TransactionIdentifier, t *moTransaction, rpdu []byte) {
	defer func() {
		s.mu.Lock()
		if u, ok := s.ues[imsi]; ok && u.mo[ti.Value] == t {
			delete(u.mo, ti.Value)
		}
		s.mu.Unlock()

		s.transactionEnded(context.WithoutCancel(ctx), imsi)
	}()

	report := s.relay(ctx, imsi, rpdu)

	payload, err := report.MarshalBinary()
	if err != nil {
		s.logger.Error("Could not encode an RP report", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	deadline := s.timers.TC1 * (1 + time.Duration(s.timers.MaxRetransmissions))

	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	s.mu.Lock()
	t.reportSent = true
	s.mu.Unlock()

	s.transactionEnded(context.WithoutCancel(ctx), imsi)

	if err := s.sendReliably(ctx, imsi, &sms.CPData{TransactionIdentifier: ti.Peer(), UserData: payload}, t.ack, t.aborted, t.retry); err != nil {
		s.logger.Info("UE did not acknowledge an SMS report", zap.String("imsi", imsi), zap.Stringer("ti", ti), zap.Error(err))
	}
}

func (s *SMSF) relay(ctx context.Context, imsi string, rpdu []byte) sms.RPMessage {
	msg, err := sms.ParseRP(rpdu, nas.DirectionUplink)
	if err != nil {
		return rpErrorFor(rpdu, err)
	}

	switch m := msg.(type) {
	case *sms.RPData:
		return s.submit(ctx, imsi, m)
	case *sms.RPSMMA:
		return s.memoryAvailable(ctx, imsi, m)
	default:
		return &sms.RPError{Direction: nas.DirectionDownlink, Reference: msg.MessageReference(), Cause: sms.RPCauseMessageNotCompatibleWithState}
	}
}

func rpErrorFor(rpdu []byte, err error) *sms.RPError {
	var reference uint8
	if len(rpdu) > 1 {
		reference = rpdu[1]
	}

	cause := sms.RPCauseInvalidMandatoryInformation
	if errors.Is(err, sms.ErrUnknownMessageType) {
		cause = sms.RPCauseMessageTypeNonExistent
	}

	return &sms.RPError{Direction: nas.DirectionDownlink, Reference: reference, Cause: cause}
}

func (s *SMSF) submit(ctx context.Context, imsi string, m *sms.RPData) sms.RPMessage {
	log := s.logger.With(zap.String("imsi", imsi), zap.Uint8("rp_reference", m.Reference))

	reject := func(cause sms.RPCause, diagnostic []byte, reason string, err error) sms.RPMessage {
		log.Info("Rejected a mobile-originated SMS", zap.String("reason", reason), zap.Stringer("rp_cause", cause), zap.Error(err))

		return &sms.RPError{Direction: nas.DirectionDownlink, Reference: m.Reference, Cause: cause, UserData: diagnostic}
	}

	settings, err := s.store.GetSMSSettings(ctx)
	if err != nil {
		return reject(sms.RPCauseNetworkOutOfOrder, nil, "SMS settings unavailable", err)
	}

	if !settings.Enabled() {
		return reject(sms.RPCauseRequestedFacilityNotImplemented, nil, "SMS is disabled", nil)
	}

	sub, err := s.store.GetSubscriber(ctx, imsi)
	if err != nil {
		return reject(sms.RPCauseNetworkOutOfOrder, nil, "subscriber unavailable", err)
	}

	if sub.Msisdn == "" {
		return reject(sms.RPCauseRequestedFacilityNotSubscribed, nil, "subscriber has no MSISDN", nil)
	}

	smsc, err := s.smsc()
	if err != nil {
		return reject(sms.RPCauseNetworkOutOfOrder, nil, "SMSC not connected", err)
	}

	req, err := sgd.NewMOForwardShortMessageRequest(smsc.envelope(), sgd.MOForwardShortMessage{
		ServiceCentreAddress: m.Destination.Digits,
		User:                 tgpp.UserIdentifier{IMSI: imsi, MSISDN: sub.Msisdn},
		SMRPUI:               m.UserData,
	})
	if err != nil {
		return reject(sms.RPCauseInvalidMandatoryInformation, nil, "RP-DATA cannot be forwarded", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timers.TR2N)
	defer cancel()

	ans, err := smsc.node.Do(ctx, PeerRoleSMSC, req)
	if err != nil {
		return reject(sms.RPCauseNetworkOutOfOrder, nil, "SMSC did not answer", err)
	}

	res, err := sgd.ParseMOForwardShortMessageAnswer(ans)

	var failure *sgd.ResultError

	switch {
	case err == nil:
		log.Info("Forwarded a mobile-originated SMS", zap.String("service_centre", m.Destination.Digits))

		return &sms.RPAck{Direction: nas.DirectionDownlink, Reference: m.Reference, UserData: res.SMRPUI}
	case errors.As(err, &failure):
		return reject(moFailureCause(failure), failure.DiagnosticInfo, "SMSC rejected the message", err)
	default:
		return reject(sms.RPCauseNetworkOutOfOrder, nil, "invalid answer from the SMSC", err)
	}
}

func moFailureCause(e *sgd.ResultError) sms.RPCause {
	switch {
	case e.IsExperimental(tgpp.ResultErrorSMDeliveryFailure) && e.DeliveryFailureCause != nil:
		switch *e.DeliveryFailureCause {
		case sgd.CauseUnknownServiceCentre:
			return sms.RPCauseUnassignedNumber
		case sgd.CauseSCCongestion:
			return sms.RPCauseCongestion
		case sgd.CauseInvalidSMEAddress:
			return sms.RPCauseShortMessageTransferRejected
		case sgd.CauseUserNotSCUser:
			return sms.RPCauseUnidentifiedSubscriber
		}
	case e.IsExperimental(tgpp.ResultErrorFacilityNotSupported):
		return sms.RPCauseRequestedFacilityNotImplemented
	}

	return sms.RPCauseNetworkOutOfOrder
}
