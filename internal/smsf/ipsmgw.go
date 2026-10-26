// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const relayTimeout = 10 * time.Second

func (s *SMSF) RegisterIPSMGW(ctx context.Context, r db.IPSMGWRegistration) error {
	if err := s.store.PutIPSMGWRegistration(ctx, r); err != nil {
		return err
	}

	logger.From(ctx, s.logger).Info("Registered an IP-SM-GW", zap.String("imsi", r.IMSI), zap.String("number", r.Number), zap.String("diameter_host", r.DiameterHost))

	s.startAlert(ctx, r.IMSI)

	return nil
}

func (s *SMSF) DeregisterIPSMGW(ctx context.Context, imsi string) error {
	if err := s.store.DeleteIPSMGWRegistration(ctx, imsi); err != nil {
		return err
	}

	logger.From(ctx, s.logger).Info("Deregistered the IP-SM-GW", zap.String("imsi", imsi))

	return nil
}

func (s *SMSF) ipSMGW(ctx context.Context, imsi string) (*db.IPSMGWRegistration, error) {
	reg, err := s.store.GetIPSMGWRegistration(ctx, imsi)
	if errors.Is(err, db.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		logger.From(ctx, s.logger).Warn("Could not look up the IP-SM-GW of a subscriber", zap.String("imsi", imsi), zap.Error(err))
		return nil, err
	}

	return reg, nil
}

func sentBy(req *diameter.Message, reg *db.IPSMGWRegistration) bool {
	return strings.EqualFold(tgpp.ParseEnvelope(req).Origin.OriginHost, reg.DiameterHost)
}

func (s *SMSF) relayToIPSMGW(ctx context.Context, id diameter.Identity, from string, req *diameter.Message, imsi string, reg *db.IPSMGWRegistration) *diameter.Message {
	log := logger.From(ctx, s.logger, zap.String("imsi", imsi), zap.String("ip_sm_gw", reg.DiameterHost))

	ctx, cancel := context.WithTimeout(ctx, relayTimeout)
	defer cancel()

	ans, err := s.peers.Relay(ctx, req, from, reg.DiameterHost, reg.DiameterRealm)

	switch {
	case err == nil:
		log.Debug("Forwarded a routing request to the IP-SM-GW")
		return ans
	case errors.Is(err, diameter.ErrNotProxiable):
		log.Info("Answered a routing request that cannot be proxied to the IP-SM-GW")
		return nil
	case errors.Is(err, diameter.ErrLoopDetected):
		return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultLoopDetected})
	}

	log.Info("Could not forward a routing request to the IP-SM-GW", zap.Error(err))

	return s6c.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToDeliver})
}
