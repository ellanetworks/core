// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

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

func (amf *AMF) DecideIMSVoPS(ctx context.Context, ue *UeContext) bool {
	supported := false

	if amf.IMSVoice != nil && !ue.IsVisitor() {
		var err error

		supported, err = amf.IMSVoice.AdvertiseIMSVoice(ctx)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("could not read whether to advertise IMS voice over PS sessions", logger.SUPI(ue.Supi().String()), zap.Error(err))

			return ue.imsVoPS.Load()
		}
	}

	if ue.imsVoPS.Swap(supported) != supported {
		logger.From(ctx, logger.AmfLog).Info("IMS voice over PS indication changed", logger.SUPI(ue.Supi().String()), zap.Bool("supported", supported))
	}

	return supported
}
