// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type OwnedLease struct {
	PoolID    string
	PoolType  string
	IMSI      string
	SessionID int
}

type OwnedLeaseStore interface {
	ListOwnedLeases(ctx context.Context) ([]OwnedLease, error)
	ReleaseOwnedLease(ctx context.Context, lease OwnedLease) (netip.Addr, error)
}

var ErrLeaseStoreUnavailable = errors.New("lease store unavailable")

const StaleLeaseReleaseInterval = 30 * time.Second

func (s *SMF) RunStaleLeaseReleaser(ctx context.Context, store OwnedLeaseStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		s.ReleaseStaleLeases(ctx, store)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *SMF) ReleaseStaleLeases(ctx context.Context, store OwnedLeaseStore) int {
	leases, err := store.ListOwnedLeases(ctx)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("list UE address leases bound to this node failed", zap.Error(err))
		return 0
	}

	released := 0

	for _, lease := range leases {
		if ctx.Err() != nil {
			return released
		}

		key, ok := s.claimStaleLease(lease.IMSI, lease.SessionID)
		if !ok {
			continue
		}

		addr, err := store.ReleaseOwnedLease(ctx, lease)

		s.finishStaleLease(key)

		if err != nil {
			logger.From(ctx, logger.SmfLog).Warn("release of a UE address lease no session holds failed; will retry",
				logger.SUPIFromIMSI(lease.IMSI), zap.Int("session_id", lease.SessionID), zap.String("pool_type", lease.PoolType), zap.Error(err))

			if errors.Is(err, ErrLeaseStoreUnavailable) {
				return released
			}

			continue
		}

		released++

		logger.From(ctx, logger.SmfLog).Info("released a UE address lease no session holds",
			logger.SUPIFromIMSI(lease.IMSI), zap.Int("session_id", lease.SessionID), zap.String("pool_type", lease.PoolType), zap.Stringer("address", addr))
	}

	return released
}

func (s *SMF) claimStaleLease(imsi string, sessionID int) (string, bool) {
	if sessionID <= 0 || sessionID > math.MaxUint8 {
		return "", false
	}

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return "", false
	}

	key := canonicalName(supi, uint8(sessionID))

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byKey[key] != nil {
		return "", false
	}

	if _, busy := s.staleReleases[key]; busy {
		return "", false
	}

	for _, kept := range s.orphanedLeases {
		if canonicalName(kept.supi, kept.sessionKey) == key {
			return "", false
		}
	}

	s.staleReleases[key] = make(chan struct{})

	return key, true
}

func (s *SMF) finishStaleLease(key string) {
	s.mu.Lock()
	done := s.staleReleases[key]
	delete(s.staleReleases, key)
	s.mu.Unlock()

	if done != nil {
		close(done)
	}
}

func (s *SMF) awaitStaleLeaseRelease(ctx context.Context, supi etsi.SUPI, sessionKey uint8) error {
	s.mu.RLock()
	done := s.staleReleases[canonicalName(supi, sessionKey)]
	s.mu.RUnlock()

	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
