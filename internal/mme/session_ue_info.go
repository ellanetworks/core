// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func (m *MME) SessionUEInfo(ctx context.Context, ue *UeContext) models.SessionUEInfo {
	ue.mu.Lock()
	pei, loc := ue.Imei.String(), ue.Location
	ue.mu.Unlock()

	info := models.SessionUEInfo{
		PEI:         pei,
		Location:    loc,
		RATType:     models.RATTypeEUTRA,
		TimeZone:    models.LocalTimeZone(time.Now()),
		ServingNode: models.ServingNode{Functionality: models.ServingNodeSGW},
	}

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("serving network unavailable for the session", zap.Error(err))

		return info
	}

	info.ServingNetwork = plmn

	return info
}

func (ue *UeContext) ReleaseUEInfo() models.SessionUEInfoUpdate {
	return models.LocationUpdate(ue.GetUserLocation(), time.Now())
}

func (m *MME) reportLocationChange(ctx context.Context, ue *UeContext) {
	if m == nil || m.Session == nil || !ue.Supi().IsValid() {
		return
	}

	update := models.LocationUpdate(ue.GetUserLocation(), time.Now())
	if update.Location == nil {
		return
	}

	m.Session.UpdateEPSUEInfo(ctx, ue.Supi(), update)
}

func (m *MME) ReportPEI(ctx context.Context, ue *UeContext) {
	if m == nil || m.Session == nil || !ue.Supi().IsValid() {
		return
	}

	pei := ue.IMEI().String()
	m.Session.UpdateEPSUEInfo(ctx, ue.Supi(), models.SessionUEInfoUpdate{PEI: &pei})
}
