// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

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

func (amf *AMF) DecideIMSVoPS(ctx context.Context, ue *UeContext) bool {
	supported := false

	if supi := ue.Supi(); amf.IMSVoice != nil && supi.IsIMSI() {
		var err error

		supported, err = amf.IMSVoice.VoiceSupported(ctx, supi.IMSI())
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("could not decide whether IMS voice is supported for the UE", logger.SUPI(supi.String()), zap.Error(err))

			supported = false
		}
	}

	if ue.imsVoPS.Swap(supported) != supported {
		logger.From(ctx, logger.AmfLog).Info("IMS voice over PS indication changed", logger.SUPI(ue.Supi().String()), zap.Bool("supported", supported))
	}

	return supported
}
