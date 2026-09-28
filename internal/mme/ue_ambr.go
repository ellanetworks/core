// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme/procedure"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func S1APUEAMBR(ambr models.Ambr) s1ap.UEAggregateMaximumBitRate {
	return s1ap.UEAggregateMaximumBitRate{
		DL: s1ap.BitRate(ambr.Downlink.Bps()),
		UL: s1ap.BitRate(ambr.Uplink.Bps()),
	}
}

func (c *UeConn) HeldUEAMBR() (models.Ambr, bool) {
	if c == nil {
		return models.Ambr{}, false
	}

	held := c.ranUEAMBR.Load()
	if held == nil {
		return models.Ambr{}, false
	}

	return *held, true
}

func (c *UeConn) holdUEAMBR(ambr *s1ap.UEAggregateMaximumBitRate) {
	if c == nil || ambr == nil {
		return
	}

	c.ranUEAMBR.Store(&models.Ambr{
		Downlink: models.BitRateFromBps(uint64(ambr.DL)),
		Uplink:   models.BitRateFromBps(uint64(ambr.UL)),
	})
}

func (c *UeConn) ForgetUEAMBR() {
	if c != nil {
		c.ranUEAMBR.Store(nil)
	}
}

func (c *UeConn) SendUEContextModification(ctx context.Context, ambr models.Ambr) error {
	if c == nil {
		return nil
	}

	req := &s1ap.UEContextModificationRequest{
		MMEUES1APID:               c.MMEUES1APID,
		ENBUES1APID:               c.ENBUES1APID(),
		UEAggregateMaximumBitRate: new(S1APUEAMBR(ambr)),
	}

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("marshal UE Context Modification Request: %w", err)
	}

	c.holdUEAMBR(req.UEAggregateMaximumBitRate)

	_ = c.SendS1AP(ctx, S1APProcedureUEContextModRequest, b)

	return nil
}

func (m *MME) ueAMBRTarget(ue *UeContext) (*UeConn, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn := ue.Conn()
	if ue.EMMState() != EMMRegistered || conn == nil || conn.releasing || ue.handover != nil || conn.ICS() != ICSCompleted {
		return nil, false
	}

	if ue.procedures != nil && ue.procedures.Active(procedure.PathSwitch) {
		return nil, false
	}

	return conn, true
}

func (m *MME) SyncUEAMBR(ctx context.Context, ue *UeContext) {
	conn, ok := m.ueAMBRTarget(ue)
	if !ok {
		return
	}

	ul, dl := ue.AmbrRates()
	if ul.IsZero() && dl.IsZero() {
		return
	}

	want := ue.RANUEAMBR()

	if held, ok := conn.HeldUEAMBR(); ok && held.Uplink.Equal(want.Uplink) && held.Downlink.Equal(want.Downlink) {
		return
	}

	conn.Log(ctx).Info("UE-AMBR changed; sending UE Context Modification Request",
		zap.Stringer("ue_ambr_uplink", want.Uplink), zap.Stringer("ue_ambr_downlink", want.Downlink))

	if err := conn.SendUEContextModification(ctx, want); err != nil {
		conn.Log(ctx).Error("failed to send UE Context Modification Request", zap.Error(err))
	}
}

func (m *MME) ReconcileUEAMBR(ctx context.Context) {
	seen := make(map[*UeContext]struct{})

	for _, ue := range m.ConnectedUEs() {
		if _, dup := seen[ue]; dup {
			continue
		}

		seen[ue] = struct{}{}

		if ue.EMMState() != EMMRegistered {
			continue
		}

		subscribed, err := SubscribedUEAMBR(ctx, m, ue.IMSI())
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("UE-AMBR reconcile: failed to resolve the subscribed UE-AMBR",
				logger.SUPI(ue.Supi().String()), zap.Error(err))

			continue
		}

		ue.SetSubscribedUEAMBR(subscribed)
		m.SyncUEAMBR(ctx, ue)
	}
}
