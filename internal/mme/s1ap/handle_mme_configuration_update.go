// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func timeToWaitDuration(t s1ap.TimeToWait) time.Duration {
	switch t {
	case s1ap.TimeToWaitV1s:
		return 1 * time.Second
	case s1ap.TimeToWaitV2s:
		return 2 * time.Second
	case s1ap.TimeToWaitV5s:
		return 5 * time.Second
	case s1ap.TimeToWaitV10s:
		return 10 * time.Second
	case s1ap.TimeToWaitV20s:
		return 20 * time.Second
	default:
		return 60 * time.Second
	}
}

func handleMMEConfigurationUpdateAcknowledge(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	ack, err := s1ap.ParseMMEConfigurationUpdateAcknowledge(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode MME Configuration Update Acknowledge", zap.Error(err))
		return
	}

	if ack.CriticalityDiagnostics != nil {
		logger.From(ctx, radio.Log).Warn("eNB reported criticality diagnostics for MME Configuration Update")
	}

	logger.From(ctx, radio.Log).Info("MME Configuration Update acknowledged")

	m.ConfigUpdateAcknowledged(ctx, radio)
}

func handleMMEConfigurationUpdateFailure(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	fail, err := s1ap.ParseMMEConfigurationUpdateFailure(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode MME Configuration Update Failure", zap.Error(err))
		return
	}

	wait := time.Duration(0)
	fields := []zap.Field{}

	if fail.Cause != nil {
		fields = append(fields, zap.String("cause", mme.S1apCauseName(fail.Cause)))
	}

	if fail.TimeToWait != nil {
		wait = timeToWaitDuration(*fail.TimeToWait)
		fields = append(fields, zap.Duration("time-to-wait", wait))
	}

	logger.From(ctx, radio.Log).Warn("eNB rejected MME Configuration Update", fields...)

	m.ConfigUpdateFailed(ctx, radio, wait)
}
