// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func timeToWaitDuration(t ngap.TimeToWait) time.Duration {
	switch t {
	case ngap.TimeToWaitV1s:
		return 1 * time.Second
	case ngap.TimeToWaitV2s:
		return 2 * time.Second
	case ngap.TimeToWaitV5s:
		return 5 * time.Second
	case ngap.TimeToWaitV10s:
		return 10 * time.Second
	case ngap.TimeToWaitV20s:
		return 20 * time.Second
	default:
		return 60 * time.Second
	}
}

func handleAMFConfigurationUpdateAcknowledge(ctx context.Context, radio *amf.Radio, value []byte) {
	ack, err := ngap.ParseAMFConfigurationUpdateAcknowledge(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode AMF Configuration Update Acknowledge", zap.Error(err))
		return
	}

	if ack.CriticalityDiagnostics != nil {
		logger.From(ctx, radio.Log).Warn("gNB reported criticality diagnostics for AMF Configuration Update")
	}

	logger.From(ctx, radio.Log).Info("AMF Configuration Update acknowledged")
}

func handleAMFConfigurationUpdateFailure(amfInstance *amf.AMF, ctx context.Context, radio *amf.Radio, value []byte) {
	fail, err := ngap.ParseAMFConfigurationUpdateFailure(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode AMF Configuration Update Failure", zap.Error(err))
		return
	}

	wait := time.Duration(0)
	fields := []zap.Field{}

	if fail.Cause != nil {
		fields = append(fields, zap.String("cause", fail.Cause.String()))
	}

	if fail.TimeToWait != nil {
		wait = timeToWaitDuration(*fail.TimeToWait)
		fields = append(fields, zap.Duration("time-to-wait", wait))
	}

	logger.From(ctx, radio.Log).Warn("gNB rejected AMF Configuration Update", fields...)

	amfInstance.ConfigUpdateFailed(ctx, radio, wait)
}
