// Copyright 2024 Ella Networks
package context

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// This file contains calls to db to get configuration data

func ListAmfRan() []AmfRan {
	amfSelf := AMFSelf()
	return amfSelf.ListAmfRan()
}

func GetSupportTaiList() []models.Tai {
	amfSelf := AMFSelf()
	tais := make([]models.Tai, 0)
	dbNetwork, err := amfSelf.DBInstance.GetOperator(context.Background())
	if err != nil {
		logger.AmfLog.Warn("Failed to get operator", zap.Error(err))
		return tais
	}
	plmnID := models.PlmnID{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	supportedTacs := dbNetwork.GetSupportedTacs()
	for _, tac := range supportedTacs {
		tai := models.Tai{
			PlmnID: &plmnID,
			Tac:    tac,
		}
		tais = append(tais, tai)
	}
	return tais
}

func GetServedGuamiList() []models.Guami {
	amfSelf := AMFSelf()
	guamis := make([]models.Guami, 0)
	dbNetwork, err := amfSelf.DBInstance.GetOperator(context.Background())
	if err != nil {
		logger.AmfLog.Warn("Failed to get operator", zap.Error(err))
		return guamis
	}
	plmnID := models.PlmnID{
		Mcc: dbNetwork.Mcc,
		Mnc: dbNetwork.Mnc,
	}
	guami := models.Guami{
		PlmnID: &plmnID,
		AmfID:  "cafe00", // To edit
	}
	guamis = append(guamis, guami)
	return guamis
}

func GetSupportedPlmn() *PlmnSupportItem {
	amfSelf := AMFSelf()
	operator, err := amfSelf.DBInstance.GetOperator(context.Background())
	if err != nil {
		logger.AmfLog.Warn("Failed to get operator", zap.Error(err))
		return nil
	}
	plmnSupportItem := &PlmnSupportItem{
		PlmnID: models.PlmnID{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		SNssaiList: []models.Snssai{
			{
				Sst: operator.Sst,
				Sd:  operator.GetHexSd(),
			},
		},
	}
	return plmnSupportItem
}
