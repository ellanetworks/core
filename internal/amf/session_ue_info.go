// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func (amf *AMF) SessionUEInfo(ctx context.Context, ue *UeContext) models.SessionUEInfo {
	loc := ue.GetUserLocation()

	info := models.SessionUEInfo{
		PEI:         ue.Imei().String(),
		Location:    loc,
		RATType:     models.RATTypeOf(loc),
		TimeZone:    models.LocalTimeZone(time.Now()),
		ServingNode: models.ServingNode{Functionality: models.ServingNodeAMF},
	}

	op, err := amf.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("serving AMF identity unavailable for the session", zap.Error(err))

		return info
	}

	if op.Guami != nil {
		guami := *op.Guami
		info.ServingNode.GUAMI = &guami

		if guami.PlmnID != nil {
			info.ServingNetwork = *guami.PlmnID
		}
	}

	return info
}

func (ue *UeContext) ReleaseUEInfo() models.SessionUEInfoUpdate {
	return models.LocationUpdate(ue.GetUserLocation(), time.Now())
}
