// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"

	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

type waiting struct {
	serviceCentre string
	memoryFull    bool
	alerting      bool
}

func (s *SMSF) markWaiting(imsi, serviceCentre string, memoryFull bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.waiting[imsi] = waiting{serviceCentre: serviceCentre, memoryFull: memoryFull}
}

func (s *SMSF) clearWaiting(imsi string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.waiting, imsi)
}

func (s *SMSF) Waiting(imsi string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.waiting[imsi]

	return ok
}

func (s *SMSF) UEReachable(ctx context.Context, imsi string) {
	s.startAlert(ctx, imsi, false)
}

func (s *SMSF) memoryAvailable(ctx context.Context, imsi string, m *sms.RPSMMA) sms.RPMessage {
	s.startAlert(ctx, imsi, true)

	return &sms.RPAck{Direction: nas.DirectionDownlink, Reference: m.Reference}
}

func (s *SMSF) startAlert(ctx context.Context, imsi string, memoryAvailable bool) {
	s.mu.Lock()
	w, ok := s.waiting[imsi]

	if !ok || w.alerting || (w.memoryFull && !memoryAvailable) {
		s.mu.Unlock()
		return
	}

	w.alerting = true
	s.waiting[imsi] = w
	s.mu.Unlock()

	go s.alert(context.WithoutCancel(ctx), imsi, w)
}

func (s *SMSF) alert(ctx context.Context, imsi string, w waiting) {
	alerted := false

	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		cur, ok := s.waiting[imsi]
		if !ok || cur.serviceCentre != w.serviceCentre || cur.memoryFull != w.memoryFull {
			return
		}

		if alerted {
			delete(s.waiting, imsi)
			return
		}

		cur.alerting = false
		s.waiting[imsi] = cur
	}()

	log := s.logger.With(zap.String("imsi", imsi), zap.String("service_centre", w.serviceCentre))

	sub, err := s.store.GetSubscriber(ctx, imsi)
	if err != nil || sub.Msisdn == "" {
		log.Info("Dropped a pending SMSC alert for a subscriber without an MSISDN", zap.Error(err))

		alerted = true

		return
	}

	smsc, err := s.smsc()
	if err != nil {
		log.Info("Could not alert the SMSC; will retry when the UE is next reachable", zap.Error(err))
		return
	}

	req, err := s6c.NewHSSAlertServiceCentreRequest(smsc.envelope(), s6c.Alert{
		ServiceCentreAddress: w.serviceCentre,
		User:                 tgpp.UserIdentifier{MSISDN: sub.Msisdn},
	})
	if err != nil {
		log.Warn("Could not build an Alert-Service-Centre request", zap.Error(err))

		alerted = true

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

	alerted = true

	log.Info("Alerted the SMSC that the UE can receive SMS again")
}
