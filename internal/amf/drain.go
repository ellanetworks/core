// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

const (
	DefaultRelativeCapacity uint8 = 0xff
	DrainedRelativeCapacity uint8 = 0
)

func (amf *AMF) RelativeCapacity() uint8 {
	return uint8(amf.relativeCapacity.Load())
}

func (amf *AMF) setRelativeCapacity(v uint8) {
	amf.relativeCapacity.Store(uint32(v))
}

func (amf *AMF) SetEligible(ctx context.Context, eligible bool) int {
	capacity := DrainedRelativeCapacity
	if eligible {
		capacity = DefaultRelativeCapacity
	}

	amf.setRelativeCapacity(capacity)

	var served ngap.ServedGUAMIList

	if eligible {
		served = amf.servedGUAMIList(ctx)

		amf.clearGUAMIUnavailable()
	}

	notified := amf.notifyRelativeCapacity(ctx, served)

	if !eligible {
		amf.notifyGUAMIUnavailable(ctx)
	}

	return notified
}

func (amf *AMF) clearGUAMIUnavailable() {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	for _, ran := range amf.reg.Connected() {
		ran.guamiUnavailableSent = false
	}
}

func (amf *AMF) claimGUAMIUnavailable(radio *Radio) bool {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	if radio.guamiUnavailableSent {
		return false
	}

	radio.guamiUnavailableSent = true

	return true
}

func (amf *AMF) forgetGUAMIUnavailable(radio *Radio) {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	radio.guamiUnavailableSent = false
}

func (amf *AMF) servedGUAMIList(ctx context.Context) ngap.ServedGUAMIList {
	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not get operator info for resume", zap.Error(err))
		return nil
	}

	if operatorInfo.Guami == nil {
		logger.From(ctx, logger.AmfLog).Warn("operator has no GUAMI; resuming without re-advertising it")
		return nil
	}

	g, err := util.GUAMIToNGAP(*operatorInfo.Guami)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not encode GUAMI for resume", zap.Error(err))
		return nil
	}

	return ngap.ServedGUAMIList{{GUAMI: g}}
}

func (amf *AMF) notifyRelativeCapacity(ctx context.Context, served ngap.ServedGUAMIList) int {
	capacity := amf.RelativeCapacity()

	notified := 0

	for _, ran := range amf.SetupCompleteRadios() {
		if amf.beginConfigUpdate(ctx, ran, capacity, served) {
			notified++
		}
	}

	if notified > 0 {
		logger.From(ctx, logger.AmfLog).Info("advertised relative AMF capacity",
			zap.Uint8("relative-capacity", capacity), zap.Int("radios", notified))
	}

	return notified
}

func (amf *AMF) beginConfigUpdate(ctx context.Context, radio *Radio, capacity uint8, served ngap.ServedGUAMIList) bool {
	amf.mu.Lock()

	if radio.advertisedCapacity != nil && *radio.advertisedCapacity == capacity {
		amf.mu.Unlock()
		return false
	}

	if time.Now().Before(radio.retryNotBefore) {
		amf.mu.Unlock()
		return false
	}

	radio.advertisedCapacity = &capacity

	amf.mu.Unlock()

	update := &ngap.AMFConfigurationUpdate{ServedGUAMIList: served, RelativeAMFCapacity: &capacity}

	b, err := update.Marshal()
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to marshal AMF Configuration Update", zap.Error(err))
		amf.forgetAdvertisedCapacity(radio)

		return false
	}

	if err := amf.SendToRadio(ctx, radio.Conn, NGAPProcedureAMFConfigurationUpdate, b); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send AMF Configuration Update", zap.Error(err))
		amf.forgetAdvertisedCapacity(radio)

		return false
	}

	return true
}

func (amf *AMF) ConfigUpdateFailed(_ context.Context, radio *Radio, wait time.Duration) {
	amf.mu.Lock()
	radio.advertisedCapacity = nil
	radio.retryNotBefore = time.Now().Add(wait)
	amf.mu.Unlock()
}

func (amf *AMF) forgetAdvertisedCapacity(radio *Radio) {
	amf.mu.Lock()
	radio.advertisedCapacity = nil
	amf.mu.Unlock()
}

func (amf *AMF) notifyGUAMIUnavailable(ctx context.Context) int {
	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not get operator info for drain", zap.Error(err))
		return 0
	}

	pkt, err := BuildAMFStatusIndication(operatorInfo.Guami)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to build AMF Status Indication", zap.Error(err))
		return 0
	}

	notified := 0

	for _, ran := range amf.SetupCompleteRadios() {
		if !amf.claimGUAMIUnavailable(ran) {
			continue
		}

		if err := amf.SendToRadio(ctx, ran.Conn, NGAPProcedureAMFStatusIndication, pkt); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to send AMF Status Indication", zap.Error(err))
			amf.forgetGUAMIUnavailable(ran)

			continue
		}

		notified++
	}

	return notified
}
