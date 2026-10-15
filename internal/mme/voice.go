// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type IMSVoice interface {
	VoiceSupported(ctx context.Context, imsi string) (bool, error)
}

func (ue *UeContext) IMSVoPS() bool {
	return ue.imsVoPS.Load()
}

func (m *MME) DecideIMSVoPS(ctx context.Context, ue *UeContext) bool {
	supported := false

	if imsi := ue.imsiOrEmpty(); m.IMSVoice != nil && imsi != "" {
		var err error

		supported, err = m.IMSVoice.VoiceSupported(ctx, imsi)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("could not decide whether IMS voice is supported for the UE", logger.SUPIFromIMSI(imsi), zap.Error(err))

			supported = false
		}
	}

	if ue.imsVoPS.Swap(supported) != supported {
		logger.From(ctx, logger.MmeLog).Info("IMS voice over PS indication changed", logger.SUPIFromIMSI(ue.imsiOrEmpty()), zap.Bool("supported", supported))
	}

	return supported
}
