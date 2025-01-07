// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

func GetSmData(supi string, Dnn string, Snssai string) ([]models.SessionManagementSubscriptionData, error) {
	sessionManagementSubscriptionDataResp, err := GetSmData2(supi)
	if err != nil {
		return nil, fmt.Errorf("GetSmData error: %+v", err)
	}

	udmUe := udmContext.NewUdmUe(supi)
	smData := udmContext.ManageSmData(sessionManagementSubscriptionDataResp, Snssai, Dnn)
	udmUe.SetSMSubsData(smData)

	rspSMSubDataList := make([]models.SessionManagementSubscriptionData, 0, 4)

	udmUe.SmSubsDataLock.RLock()
	for _, eachSMSubData := range udmUe.SessionManagementSubsData {
		rspSMSubDataList = append(rspSMSubDataList, eachSMSubData)
	}
	udmUe.SmSubsDataLock.RUnlock()
	return rspSMSubDataList, nil
}

func GetNssai(supi string) (*models.Nssai, error) {
	accessAndMobilitySubscriptionDataResp, err := GetAmData(supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func GetSmfSelectData(supi string) (
	*models.SmfSelectionSubscriptionData, error,
) {
	var body models.SmfSelectionSubscriptionData
	udmContext.CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := GetSmfSelectData2(supi)
	if err != nil {
		logger.UdmLog.Errorf("GetSmfSelectData error: %+v", err)
		return nil, fmt.Errorf("GetSmfSelectData error: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelectionSubscriptionDataResp)
	return udmUe.SmfSelSubsData, nil
}

func GetUeContextInSmfData(supi string) (*models.UeContextInSmfData, error) {
	var body models.UeContextInSmfData
	udmContext.CreateUeContextInSmfDataforUe(supi, body)
	pdusess, err := GetSMFRegistrations2(supi)
	if err != nil {
		return nil, fmt.Errorf("GetSMFRegistrations error: %+v", err)
	}
	pduSessionMap := make(map[string]models.PduSession)
	for _, element := range pdusess {
		var pduSession models.PduSession
		pduSession.Dnn = element.Dnn
		pduSession.SmfInstanceId = element.SmfInstanceId
		pduSession.PlmnId = element.PlmnId
		pduSessionMap[strconv.Itoa(int(element.PduSessionId))] = pduSession
	}
	var ueContextInSmfData models.UeContextInSmfData
	ueContextInSmfData.PduSessions = pduSessionMap
	var pgwInfoArray []models.PgwInfo
	for _, element := range pdusess {
		var pgwInfo models.PgwInfo
		pgwInfo.Dnn = element.Dnn
		pgwInfo.PgwFqdn = element.PgwFqdn
		pgwInfo.PlmnId = element.PlmnId
		pgwInfoArray = append(pgwInfoArray, pgwInfo)
	}

	ueContextInSmfData.PgwInfo = pgwInfoArray

	udmUe := udmContext.NewUdmUe(supi)
	udmUe.UeCtxtInSmfData = &ueContextInSmfData
	return udmUe.UeCtxtInSmfData, nil
}
