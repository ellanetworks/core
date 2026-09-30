// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"slices"

	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

func (s *SMSF) markWaiting(ctx context.Context, imsi, serviceCentre string) {
	s.recordWaiting(ctx, db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre})
}

func (s *SMSF) markMemoryFull(ctx context.Context, imsi, serviceCentre string) {
	s.recordWaiting(ctx, db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre, MemoryFull: true})
}

func (s *SMSF) recordWaiting(ctx context.Context, u db.SMSWaitingUpdate) {
	if u.ServiceCentre == "" {
		return
	}

	if err := s.store.RecordSMSWaiting(context.WithoutCancel(ctx), u); err != nil {
		s.logger.Warn("Could not record message waiting data", zap.String("imsi", u.IMSI), zap.String("service_centre", u.ServiceCentre), zap.Error(err))
	}
}

func (s *SMSF) delivered(ctx context.Context, imsi, serviceCentre string) {
	ctx = context.WithoutCancel(ctx)

	w, err := s.store.GetSMSWaiting(ctx, imsi)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			s.logger.Warn("Could not read message waiting data", zap.String("imsi", imsi), zap.Error(err))
		}

		return
	}

	if err := s.store.RemoveSMSWaitingCentre(ctx, imsi, serviceCentre); err != nil {
		s.logger.Warn("Could not clear message waiting data", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	if slices.ContainsFunc(w.ServiceCentres, func(sc string) bool { return sc != serviceCentre }) {
		s.startAlert(ctx, imsi)
	}
}

func (s *SMSF) UEReachable(ctx context.Context, imsi string) {
	s.startAlert(ctx, imsi)
}

func (s *SMSF) memoryAvailable(ctx context.Context, imsi string, m *sms.RPSMMA) sms.RPMessage {
	if err := s.clearMemoryFull(ctx, imsi); err != nil {
		s.logger.Warn("Could not clear the memory capacity exceeded flag; asking the UE to retry", zap.String("imsi", imsi), zap.Error(err))

		return &sms.RPError{Direction: nas.DirectionDownlink, Reference: m.Reference, Cause: sms.RPCauseTemporaryFailure}
	}

	s.startAlert(ctx, imsi)

	return &sms.RPAck{Direction: nas.DirectionDownlink, Reference: m.Reference}
}

func (s *SMSF) clearMemoryFull(ctx context.Context, imsi string) error {
	w, err := s.store.GetSMSWaiting(ctx, imsi)
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	if !w.MemoryFull {
		return nil
	}

	return s.store.ClearSMSMemoryFull(ctx, imsi)
}

func (s *SMSF) startAlert(ctx context.Context, imsi string) {
	s.mu.Lock()

	if s.deliveringLocked(imsi) {
		s.ues[imsi].deferredAlert = true
		s.mu.Unlock()

		return
	}

	if _, running := s.alerting[imsi]; running {
		s.alerting[imsi] = true
		s.mu.Unlock()

		return
	}

	s.alerting[imsi] = false
	s.mu.Unlock()

	go s.alertUntilSettled(context.WithoutCancel(ctx), imsi)
}

func (s *SMSF) alertUntilSettled(ctx context.Context, imsi string) {
	for {
		s.alert(ctx, imsi)

		if s.alertSettled(imsi) {
			return
		}
	}
}

func (s *SMSF) alertSettled(imsi string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.alerting[imsi] {
		s.alerting[imsi] = false
		return false
	}

	delete(s.alerting, imsi)

	return true
}

func (s *SMSF) alert(ctx context.Context, imsi string) {
	log := s.logger.With(zap.String("imsi", imsi))

	w, err := s.store.GetSMSWaiting(ctx, imsi)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			log.Warn("Could not read message waiting data", zap.Error(err))
		}

		return
	}

	if w.MemoryFull {
		return
	}

	sub, err := s.store.GetSubscriber(ctx, imsi)
	if err != nil || sub.Msisdn == "" {
		log.Info("Dropped the message waiting data of a subscriber without an MSISDN", zap.Error(err))

		if err := s.store.DeleteSMSWaiting(ctx, imsi); err != nil {
			log.Warn("Could not delete message waiting data", zap.Error(err))
		}

		return
	}

	smsc, err := s.smsc()
	if err != nil {
		log.Info("Could not alert the SMSC; will retry when the UE is next reachable", zap.Error(err))
		return
	}

	for _, sc := range w.ServiceCentres {
		s.alertServiceCentre(ctx, log.With(zap.String("service_centre", sc)), smsc, imsi, sub.Msisdn, sc)
	}
}

func (s *SMSF) alertServiceCentre(ctx context.Context, log *zap.Logger, smsc smscEndpoint, imsi, msisdn, serviceCentre string) {
	req, err := s6c.NewHSSAlertServiceCentreRequest(smsc.envelope(), s6c.Alert{
		ServiceCentreAddress: serviceCentre,
		User:                 tgpp.UserIdentifier{MSISDN: msisdn},
	})
	if err != nil {
		log.Warn("Dropped an Alert-Service-Centre that cannot be built", zap.Error(err))
		s.removeWaitingCentre(ctx, log, imsi, serviceCentre)

		return
	}

	if !s.stillWaiting(ctx, imsi, serviceCentre) {
		log.Debug("Dropped an SMSC alert made redundant by a delivery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, s.timers.AlertTimeout)
	defer cancel()

	ans, err := smsc.node.Do(ctx, PeerRoleSMSC, req)
	if err == nil {
		err = s6c.ParseAlertServiceCentreAnswer(ans)
	}

	if err != nil {
		log.Warn("SMSC alert failed; will retry when the UE is next reachable", zap.Error(err))
		return
	}

	s.removeWaitingCentre(context.WithoutCancel(ctx), log, imsi, serviceCentre)

	log.Info("Alerted the SMSC that the UE can receive SMS again")
}

func (s *SMSF) removeWaitingCentre(ctx context.Context, log *zap.Logger, imsi, serviceCentre string) {
	if err := s.store.RemoveSMSWaitingCentre(ctx, imsi, serviceCentre); err != nil {
		log.Warn("Could not clear message waiting data", zap.Error(err))
	}
}

func (s *SMSF) deliveringLocked(imsi string) bool {
	u, ok := s.ues[imsi]

	return ok && u.mt != nil && !u.mt.reported
}

func (s *SMSF) stillWaiting(ctx context.Context, imsi, serviceCentre string) bool {
	s.mu.Lock()
	delivering := s.deliveringLocked(imsi)
	s.mu.Unlock()

	if delivering {
		return false
	}

	w, err := s.store.GetSMSWaiting(ctx, imsi)

	return err == nil && !w.MemoryFull && slices.Contains(w.ServiceCentres, serviceCentre)
}
