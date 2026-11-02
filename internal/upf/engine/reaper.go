// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func (conn *SessionEngine) pendingCleanupSEIDs() []uint64 {
	var seids []uint64

	conn.eachSession(func(session *Session) bool {
		if !session.opMu.TryLock() {
			return true
		}

		if session.deleted {
			seids = append(seids, session.SEID)
		}

		session.opMu.Unlock()

		return true
	})

	slices.Sort(seids)

	return seids
}

func (conn *SessionEngine) ReapDeletedSessions(ctx context.Context) (cleaned []uint64, pending int) {
	for _, seid := range conn.pendingCleanupSEIDs() {
		resp, err := conn.DeleteSession(ctx, &models.DeleteRequest{SEID: seid})
		if errors.Is(err, models.ErrSessionNotFound) {
			continue
		}

		if resp != nil {
			if uplink, downlink := models.UsageVolume(resp.Usage); uplink+downlink > 0 {
				logger.From(ctx, logger.UpfLog).Error("usage bytes lost: a session cleaned up after a failed operation had measured traffic",
					logger.SEID(seid), zap.Uint64("uplink_bytes", uplink), zap.Uint64("downlink_bytes", downlink))
			}
		}

		if err != nil {
			pending++

			logger.From(ctx, logger.UpfLog).Warn("session cleanup retry failed",
				logger.SEID(seid), zap.Error(err))

			continue
		}

		cleaned = append(cleaned, seid)

		logger.From(ctx, logger.UpfLog).Info("cleaned up a session left by a failed operation",
			logger.SEID(seid))
	}

	return cleaned, pending
}
