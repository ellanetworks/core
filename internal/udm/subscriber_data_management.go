// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/openapi/models"
)

var AllowedSessionTypes = []coreModels.PduSessionType{coreModels.PduSessionType_IPV4}

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

type subsID = string

type UESubsData struct {
	SdmSubscriptions map[subsID]*models.SdmSubscription
}

func GetAmData(ueId string) (*models.AccessAndMobilitySubscriptionData, error) {
	subscriber, err := udmContext.DbInstance.GetSubscriber(ueId)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	profile, err := udmContext.DbInstance.GetProfileByID(subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}
	operator, err := udmContext.DbInstance.GetOperator()
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}
	amData := &models.AccessAndMobilitySubscriptionData{
		Nssai: &models.Nssai{
			DefaultSingleNssais: make([]models.Snssai, 0),
			SingleNssais:        make([]models.Snssai, 0),
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: profile.BitrateDownlink,
			Uplink:   profile.BitrateUplink,
		},
	}
	amData.Nssai.DefaultSingleNssais = append(amData.Nssai.DefaultSingleNssais, models.Snssai{
		Sd:  operator.GetHexSd(),
		Sst: operator.Sst,
	})
	amData.Nssai.SingleNssais = append(amData.Nssai.SingleNssais, models.Snssai{
		Sd:  operator.GetHexSd(),
		Sst: operator.Sst,
	})
	return amData, nil
}

func GetAmDataAndSetAMSubscription(supi string) (
	*models.AccessAndMobilitySubscriptionData, error,
) {
	amData, err := GetAmData(supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func GetSmData(ueId string) ([]coreModels.SessionManagementSubscriptionData, error) {
	subscriber, err := udmContext.DbInstance.GetSubscriber(ueId)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	profile, err := udmContext.DbInstance.GetProfileByID(subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}
	operator, err := udmContext.DbInstance.GetOperator()
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}
	smData := make([]coreModels.SessionManagementSubscriptionData, 0)
	smDataObjModel := coreModels.SessionManagementSubscriptionData{
		SingleNssai: &coreModels.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		DnnConfigurations: make(map[string]coreModels.DnnConfiguration),
	}
	smDataObjModel.DnnConfigurations[config.DNN] = coreModels.DnnConfiguration{
		PduSessionTypes: &coreModels.PduSessionTypes{
			DefaultSessionType:  coreModels.PduSessionType_IPV4,
			AllowedSessionTypes: make([]coreModels.PduSessionType, 0),
		},
		SscModes: &coreModels.SscModes{
			DefaultSscMode:  coreModels.SscMode__1,
			AllowedSscModes: make([]coreModels.SscMode, 0),
		},
		SessionAmbr: &coreModels.Ambr{
			Downlink: profile.BitrateDownlink,
			Uplink:   profile.BitrateUplink,
		},
		Var5gQosProfile: &coreModels.SubscribedDefaultQos{
			Var5qi:        profile.Var5qi,
			Arp:           &coreModels.Arp{PriorityLevel: profile.PriorityLevel},
			PriorityLevel: profile.PriorityLevel,
		},
	}
	smDataObjModel.DnnConfigurations[config.DNN].PduSessionTypes.AllowedSessionTypes = append(smDataObjModel.DnnConfigurations[config.DNN].PduSessionTypes.AllowedSessionTypes, AllowedSessionTypes...)
	for _, sscMode := range AllowedSscModes {
		smDataObjModel.DnnConfigurations[config.DNN].SscModes.AllowedSscModes = append(smDataObjModel.DnnConfigurations[config.DNN].SscModes.AllowedSscModes, coreModels.SscMode(sscMode))
	}
	smData = append(smData, smDataObjModel)
	return smData, nil
}

func GetAndSetSmData(supi string, Dnn string, Snssai string) ([]coreModels.SessionManagementSubscriptionData, error) {
	sessionManagementSubscriptionDataResp, err := GetSmData(supi)
	if err != nil {
		return nil, fmt.Errorf("GetSmData error: %+v", err)
	}

	udmUe := udmContext.NewUdmUe(supi)
	smData := udmContext.ManageSmData(sessionManagementSubscriptionDataResp, Snssai, Dnn)
	udmUe.SetSMSubsData(smData)

	rspSMSubDataList := make([]coreModels.SessionManagementSubscriptionData, 0, 4)

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

func GetSmfSelectData(ueId string) (*coreModels.SmfSelectionSubscriptionData, error) {
	operator, err := udmContext.DbInstance.GetOperator()
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}
	snssai := fmt.Sprintf("%d%s", operator.Sst, operator.GetHexSd())
	smfSelectionData := &coreModels.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: make(map[string]coreModels.SnssaiInfo),
	}
	smfSelectionData.SubscribedSnssaiInfos[snssai] = coreModels.SnssaiInfo{
		DnnInfos: make([]coreModels.DnnInfo, 0),
	}
	snssaiInfo := smfSelectionData.SubscribedSnssaiInfos[snssai]
	snssaiInfo.DnnInfos = append(snssaiInfo.DnnInfos, coreModels.DnnInfo{
		Dnn: config.DNN,
	})
	smfSelectionData.SubscribedSnssaiInfos[snssai] = snssaiInfo
	return smfSelectionData, nil
}

func GetAndSetSmfSelectData(supi string) (
	*coreModels.SmfSelectionSubscriptionData, error,
) {
	var body coreModels.SmfSelectionSubscriptionData
	udmContext.CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := GetSmfSelectData(supi)
	if err != nil {
		logger.UdmLog.Errorf("GetSmfSelectData error: %+v", err)
		return nil, fmt.Errorf("GetSmfSelectData error: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelectionSubscriptionDataResp)
	return udmUe.SmfSelSubsData, nil
}

func CreateSdmSubscriptions(SdmSubscription models.SdmSubscription, ueId string) models.SdmSubscription {
	value, ok := udmContext.UESubsCollection.Load(ueId)
	if !ok {
		udmContext.UESubsCollection.Store(ueId, new(UESubsData))
		value, _ = udmContext.UESubsCollection.Load(ueId)
	}
	UESubsData := value.(*UESubsData)
	if UESubsData.SdmSubscriptions == nil {
		UESubsData.SdmSubscriptions = make(map[string]*models.SdmSubscription)
	}

	newSubscriptionID := strconv.Itoa(udmContext.SdmSubscriptionIDGenerator)
	SdmSubscription.SubscriptionId = newSubscriptionID
	UESubsData.SdmSubscriptions[newSubscriptionID] = &SdmSubscription
	udmContext.SdmSubscriptionIDGenerator++

	return SdmSubscription
}

func CreateSubscription(sdmSubscription *models.SdmSubscription, supi string) error {
	sdmSubscriptionResp := CreateSdmSubscriptions(*sdmSubscription, supi)
	udmUe, _ := udmContext.UdmUeFindBySupi(supi)
	if udmUe == nil {
		udmUe = udmContext.NewUdmUe(supi)
	}
	udmUe.CreateSubscriptiontoNotifChange(sdmSubscriptionResp.SubscriptionId, &sdmSubscriptionResp)
	return nil
}

func GetUeContextInSmfData(supi string) (*coreModels.UeContextInSmfData, error) {
	var body coreModels.UeContextInSmfData
	udmContext.CreateUeContextInSmfDataforUe(supi, body)
	pdusess := []*coreModels.SmfRegistration{}
	pduSessionMap := make(map[string]coreModels.PduSession)
	for _, element := range pdusess {
		var pduSession coreModels.PduSession
		pduSession.Dnn = element.Dnn
		pduSession.SmfInstanceId = element.SmfInstanceId
		pduSession.PlmnId = element.PlmnId
		pduSessionMap[strconv.Itoa(int(element.PduSessionId))] = pduSession
	}
	var ueContextInSmfData coreModels.UeContextInSmfData
	ueContextInSmfData.PduSessions = pduSessionMap
	var pgwInfoArray []coreModels.PgwInfo
	for _, element := range pdusess {
		var pgwInfo coreModels.PgwInfo
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
