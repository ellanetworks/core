// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type IMSVoice interface {
	AdvertiseIMSVoice(ctx context.Context) (bool, error)
}

func (ue *UeContext) IMSVoPS() bool {
	return ue.imsVoPS.Load()
}

func (m *MME) DecideIMSVoPS(ctx context.Context, ue *UeContext) bool {
	supported := false

	if m.IMSVoice != nil && !ue.IsVisitor() {
		var err error

		supported, err = m.IMSVoice.AdvertiseIMSVoice(ctx)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("could not read whether to advertise IMS voice over PS sessions", logger.SUPIFromIMSI(ue.imsiOrEmpty()), zap.Error(err))

			return ue.imsVoPS.Load()
		}
	}

	if ue.imsVoPS.Swap(supported) != supported {
		logger.From(ctx, logger.MmeLog).Info("IMS voice over PS indication changed", logger.SUPIFromIMSI(ue.imsiOrEmpty()), zap.Bool("supported", supported))
	}

	return supported
}
