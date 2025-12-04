// Copyright 2024 Ella Networks
package context

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// This file contains calls to db to get configuration data

func getPaginateIndexes(page int, perPage int, total int) (int, int) {
	startIndex := (page - 1) * perPage

	endIndex := startIndex + perPage

	if startIndex > total {
		return 0, 0
	}

	if endIndex > total {
		endIndex = total
	}

	return startIndex, endIndex
}

func ListAmfRan(page int, perPage int) (int, []AmfRan) {
	amfSelf := AMFSelf()

	ranList := amfSelf.ListAmfRan()

	total := len(ranList)

	startIndex, endIndex := getPaginateIndexes(page, perPage, total)

	ranListPage := ranList[startIndex:endIndex]

	return total, ranListPage
}

func GetSupportTaiList(ctx context.Context) []models.Tai {
	amfSelf := AMFSelf()
	tais := make([]models.Tai, 0)
	dbNetwork, err := amfSelf.DBInstance.GetOperator(ctx)
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

func GetServedGuami(ctx context.Context) *models.Guami {
	amfSelf := AMFSelf()

	dbNetwork, err := amfSelf.DBInstance.GetOperator(ctx)
	if err != nil {
		logger.AmfLog.Warn("Failed to get operator", zap.Error(err))
		return nil
	}

	return &models.Guami{
		PlmnID: &models.PlmnID{
			Mcc: dbNetwork.Mcc,
			Mnc: dbNetwork.Mnc,
		},
		AmfID: "cafe00", // To edit
	}
}

func GetSupportedPlmn(ctx context.Context) *PlmnSupportItem {
	amfSelf := AMFSelf()
	operator, err := amfSelf.DBInstance.GetOperator(ctx)
	if err != nil {
		logger.AmfLog.Warn("Failed to get operator", zap.Error(err))
		return nil
	}
	plmnSupportItem := &PlmnSupportItem{
		PlmnID: models.PlmnID{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		SNssai: models.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
	}
	return plmnSupportItem
}
