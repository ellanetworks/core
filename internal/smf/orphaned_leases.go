// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type orphanedLease struct {
	supi       etsi.SUPI
	sessionKey uint8
	dnn        string
	ipv4, ipv6 bool
}

func (s *SMF) recordOrphanedLease(smContext *SMContext) {
	if smContext.PFCPContext == nil || (smContext.PDUIPV4Address == nil && smContext.PDUIPV6Prefix == nil) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.orphanedLeases[smContext.PFCPContext.SEID] = orphanedLease{
		supi:       smContext.Supi,
		sessionKey: smContext.sessionKey(),
		dnn:        smContext.Dnn,
		ipv4:       smContext.PDUIPV4Address != nil,
		ipv6:       smContext.PDUIPV6Prefix != nil,
	}
}

func (s *SMF) forgetOrphanedLease(seid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.orphanedLeases, seid)
}

func (s *SMF) UserPlaneSessionCleaned(ctx context.Context, seid uint64) {
	s.mu.RLock()
	lease, ok := s.orphanedLeases[seid]
	s.mu.RUnlock()

	if !ok {
		return
	}

	dn, err := s.store.ResolveDNN(ctx, lease.dnn)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("resolve data network to release a kept UE address lease failed",
			logger.SUPI(lease.supi.String()), logger.DNN(lease.dnn), logger.SEID(seid), zap.Error(err))

		s.forgetOrphanedLease(seid)

		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.orphanedLeases, seid)

	if held := s.byKey[canonicalName(lease.supi, lease.sessionKey)]; held != nil {
		logger.From(ctx, logger.SmfLog).Info("kept UE address lease is now held by a current session; not releasing it",
			logger.SUPI(lease.supi.String()), logger.SEID(seid), logger.SMContextRef(held.Ref))

		return
	}

	imsi := lease.supi.IMSI()

	if lease.ipv4 {
		if _, err := dn.ReleaseIP(ctx, imsi, lease.sessionKey); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("failed to release a kept IPv4 lease",
				logger.SUPI(lease.supi.String()), logger.SEID(seid), zap.Error(err))
		}
	}

	if lease.ipv6 {
		if _, err := dn.ReleaseIPv6(ctx, imsi, lease.sessionKey); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("failed to release a kept IPv6 lease",
				logger.SUPI(lease.supi.String()), logger.SEID(seid), zap.Error(err))
		}
	}

	logger.From(ctx, logger.SmfLog).Info("released the UE address lease kept after a failed user-plane teardown",
		logger.SUPI(lease.supi.String()), logger.DNN(lease.dnn), logger.SEID(seid))
}
