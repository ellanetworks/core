// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	payload, err := encodeReport(s.relay(ctx, imsi, rpdu))
	if err != nil {
		logger.From(ctx, s.logger).Error("Could not encode an RP report", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	deadline := s.timers.TC1 * (1 + time.Duration(s.timers.MaxRetransmissions))

	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	if err := s.sendReliably(ctx, imsi, &sms.CPData{TransactionIdentifier: ti.Peer(), UserData: payload}, &t.cpTxn); err != nil {
		logger.From(ctx, s.logger).Info("UE did not acknowledge an SMS report", zap.String("imsi", imsi), zap.Stringer("ti", ti), zap.Error(err))
	}
}

func encodeReport(report sms.RPMessage) ([]byte, error) {
	b, err := report.MarshalBinary()
	if err == nil {
		return b, nil
	}

	switch m := report.(type) {
	case *sms.RPAck:
		m.UserData = nil
	case *sms.RPError:
		m.UserData = nil
	default:
		return nil, err
	}

	return report.MarshalBinary()
}

func userData(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}

	return b
}

func (s *SMSF) relay(ctx context.Context, imsi string, rpdu []byte) sms.RPMessage {
	msg, err := sms.ParseRP(rpdu, nas.DirectionUplink)
	if msg == nil {
		return rpErrorFor(rpdu, err)
	}

	if err != nil {
		logger.From(ctx, s.logger).Debug("Ignored non-imperative errors in an RP message", zap.String("imsi", imsi), zap.Error(err))
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
	log := logger.From(ctx, s.logger,
		zap.String("imsi", imsi),
		zap.Uint8("rp_reference", m.Reference),
		zap.String("service_centre", m.Destination.Digits),
		zap.Int("tpdu_length", len(m.UserData)),
	)

	reject := func(level zapcore.Level, cause sms.RPCause, diagnostic []byte, reason string, err error) sms.RPMessage {
		log.Log(level, "Rejected a mobile-originated SMS", zap.String("reason", reason), zap.Stringer("rp_cause", cause), zap.Error(err))

		return &sms.RPError{Direction: nas.DirectionDownlink, Reference: m.Reference, Cause: cause, UserData: userData(diagnostic)}
	}

	settings, err := s.store.GetSMSSettings(ctx)
	if err != nil {
		return reject(zapcore.WarnLevel, sms.RPCauseNetworkOutOfOrder, nil, "SMS settings unavailable", err)
	}

	if !settings.Enabled {
		return reject(zapcore.InfoLevel, sms.RPCauseRequestedFacilityNotImplemented, nil, "SMS is disabled", nil)
	}

	sub, err := s.store.GetSubscriber(ctx, imsi)
	if err != nil {
		return reject(zapcore.WarnLevel, sms.RPCauseNetworkOutOfOrder, nil, "subscriber unavailable", err)
	}

	if sub.Msisdn == "" {
		return reject(zapcore.InfoLevel, sms.RPCauseRequestedFacilityNotSubscribed, nil, "subscriber has no MSISDN", nil)
	}

	envelope, err := s.smsc.Envelope()
	if err != nil {
		return reject(zapcore.WarnLevel, sms.RPCauseNetworkOutOfOrder, nil, "SMSC not connected", err)
	}

	req, err := sgd.NewMOForwardShortMessageRequest(envelope, sgd.MOForwardShortMessage{
		ServiceCentreAddress: m.Destination.Digits,
		User:                 tgpp.UserIdentifier{IMSI: imsi, MSISDN: sub.Msisdn},
		SMRPUI:               m.UserData,
	})
	if err != nil {
		return reject(zapcore.InfoLevel, sms.RPCauseInvalidMandatoryInformation, nil, "RP-DATA cannot be forwarded", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timers.TR2N)
	defer cancel()

	ans, err := s.smsc.Do(ctx, req)
	if err != nil {
		return reject(zapcore.WarnLevel, sms.RPCauseNetworkOutOfOrder, nil, "SMSC did not answer", err)
	}

	res, err := sgd.ParseMOForwardShortMessageAnswer(ans)

	var failure *sgd.ResultError

	switch {
	case err == nil:
		log.Info("Forwarded a mobile-originated SMS")

		return &sms.RPAck{Direction: nas.DirectionDownlink, Reference: m.Reference, UserData: userData(res.SMRPUI)}
	case errors.As(err, &failure):
		return reject(zapcore.WarnLevel, moFailureCause(failure), failure.DiagnosticInfo, "SMSC rejected the message", err)
	default:
		return reject(zapcore.WarnLevel, sms.RPCauseNetworkOutOfOrder, nil, "invalid answer from the SMSC", err)
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
