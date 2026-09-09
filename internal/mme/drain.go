// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

const (
	DefaultRelativeCapacity uint8 = 0xff
	DrainedRelativeCapacity uint8 = 0
)

const configUpdateGuardTimeout = 5 * time.Second

var causeLoadBalancingTAURequired = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkLoadBalancingTAURequired}

func (m *MME) RelativeCapacity() uint8 {
	return uint8(m.relativeCapacity.Load())
}

func (m *MME) setRelativeCapacity(v uint8) {
	m.relativeCapacity.Store(uint32(v))
}

func (m *MME) SetEligible(ctx context.Context, eligible bool) int {
	capacity := DrainedRelativeCapacity
	if eligible {
		capacity = DefaultRelativeCapacity
	}

	m.setRelativeCapacity(capacity)

	return m.notifyRelativeCapacity(ctx)
}

func (m *MME) notifyRelativeCapacity(ctx context.Context) int {
	capacity := m.RelativeCapacity()

	notified := 0

	for _, radio := range m.ConnectedRadios() {
		if !radio.SetupComplete() {
			continue
		}

		if m.beginConfigUpdate(ctx, radio, capacity) {
			notified++
		}
	}

	if notified > 0 {
		logger.From(ctx, logger.MmeLog).Info("advertised relative MME capacity",
			zap.Uint8("relative-capacity", capacity), zap.Int("radios", notified))
	}

	return notified
}

func (m *MME) beginConfigUpdate(ctx context.Context, radio *Radio, capacity uint8) bool {
	m.mu.Lock()

	if radio.configUpdateOutstanding {
		m.mu.Unlock()
		return false
	}

	if radio.advertisedCapacity != nil && *radio.advertisedCapacity == capacity {
		m.mu.Unlock()
		return false
	}

	if time.Now().Before(radio.retryNotBefore) {
		m.mu.Unlock()
		return false
	}

	radio.configUpdateOutstanding = true

	m.mu.Unlock()

	return m.emitConfigUpdate(ctx, radio, capacity)
}

func (m *MME) emitConfigUpdate(ctx context.Context, radio *Radio, capacity uint8) bool {
	update := &s1ap.MMEConfigurationUpdate{RelativeMMECapacity: &capacity}

	b, err := update.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal MME Configuration Update", zap.Error(err))
		m.finishConfigUpdate(ctx, radio)

		return false
	}

	m.mu.Lock()
	radio.advertisedCapacity = &capacity
	m.mu.Unlock()

	m.SendToRadio(ctx, radio.Conn, S1APProcedureMMEConfigUpdate, b)

	guarded := context.WithoutCancel(ctx)

	radio.configUpdateGuard.ArmOnce(configUpdateGuardTimeout, func() {
		logger.From(guarded, radio.Log).Warn("MME Configuration Update went unanswered")
		m.forgetAdvertisedCapacity(radio)
		m.finishConfigUpdate(guarded, radio)
	})

	return true
}

func (m *MME) ConfigUpdateAcknowledged(ctx context.Context, radio *Radio) {
	m.finishConfigUpdate(ctx, radio)
}

func (m *MME) ConfigUpdateFailed(ctx context.Context, radio *Radio, wait time.Duration) {
	m.mu.Lock()
	radio.advertisedCapacity = nil
	radio.retryNotBefore = time.Now().Add(wait)
	m.mu.Unlock()

	m.finishConfigUpdate(ctx, radio)
}

func (m *MME) forgetAdvertisedCapacity(radio *Radio) {
	m.mu.Lock()
	radio.advertisedCapacity = nil
	m.mu.Unlock()
}

func (m *MME) finishConfigUpdate(_ context.Context, radio *Radio) {
	radio.configUpdateGuard.Stop()

	m.mu.Lock()
	radio.configUpdateOutstanding = false
	m.mu.Unlock()
}

func (m *MME) Offload(ctx context.Context, batch int) int {
	releaseCtx := context.WithoutCancel(ctx)

	released := 0

	for _, ue := range m.offloadCandidates(batch) {
		m.ReleaseUEContext(releaseCtx, ue, causeLoadBalancingTAURequired)

		released++
	}

	if released > 0 {
		logger.From(ctx, logger.MmeLog).Info("off-loaded UEs for drain", zap.Int("count", released))
	}

	return released
}

func (m *MME) RemainingOffloadable() int {
	return len(m.offloadCandidates(0))
}

func (m *MME) offloadCandidates(batch int) []*UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*UeContext, 0, max(batch, 0))

	for supi, ue := range m.UEs {
		if batch > 0 && len(out) >= batch {
			break
		}

		if ue.EMMState() != EMMRegistered || ue.releasing {
			continue
		}

		if _, relocating := m.relocating[supi]; relocating {
			continue
		}

		if ue.Conn() == nil {
			continue
		}

		out = append(out, ue)
	}

	return out
}
