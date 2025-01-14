// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

var AllowedSessionTypes = []models.PduSessionType{models.PduSessionType_IPV4}

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

type subsID = string

type UESubsData struct {
	SdmSubscriptions map[subsID]*models.SdmSubscription
}

// This function is defined twice, here and in the NMS. We should move it to a common place.
func convertDbAmDataToModel(bitrateDownlink string, bitrateUplink string) *models.AccessAndMobilitySubscriptionData {
	amData := &models.AccessAndMobilitySubscriptionData{
		Nssai: &models.Nssai{
			DefaultSingleNssais: make([]models.Snssai, 0),
			SingleNssais:        make([]models.Snssai, 0),
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: bitrateDownlink,
			Uplink:   bitrateUplink,
		},
	}
	amData.Nssai.DefaultSingleNssais = append(amData.Nssai.DefaultSingleNssais, models.Snssai{
		Sd:  config.Sd,
		Sst: config.Sst,
	})
	amData.Nssai.SingleNssais = append(amData.Nssai.SingleNssais, models.Snssai{
		Sd:  config.Sd,
		Sst: config.Sst,
	})
	return amData
}

func GetAmData2(ueId string) (*models.AccessAndMobilitySubscriptionData, error) {
	subscriber, err := udmContext.DbInstance.GetSubscriber(ueId)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	profile, err := udmContext.DbInstance.GetProfileByID(subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}
	amData := convertDbAmDataToModel(profile.BitrateDownlink, profile.BitrateUplink)
	return amData, nil
}

func GetAmDataAndSetAMSubscription(supi string) (
	*models.AccessAndMobilitySubscriptionData, error,
) {
	amData, err := GetAmData2(supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func convertDbSessionManagementDataToModel(
	bitrateDownlink string,
	bitrateUplink string,
	var5qi int32,
	priorityLevel int32,
) []models.SessionManagementSubscriptionData {
	smData := make([]models.SessionManagementSubscriptionData, 0)
	smDataObjModel := models.SessionManagementSubscriptionData{
		SingleNssai: &models.Snssai{
			Sst: config.Sst,
			Sd:  config.Sd,
		},
		DnnConfigurations: make(map[string]models.DnnConfiguration),
	}
	smDataObjModel.DnnConfigurations[config.DNN] = models.DnnConfiguration{
		PduSessionTypes: &models.PduSessionTypes{
			DefaultSessionType:  models.PduSessionType_IPV4,
			AllowedSessionTypes: make([]models.PduSessionType, 0),
		},
		SscModes: &models.SscModes{
			DefaultSscMode:  models.SscMode__1,
			AllowedSscModes: make([]models.SscMode, 0),
		},
		SessionAmbr: &models.Ambr{
			Downlink: bitrateDownlink,
			Uplink:   bitrateUplink,
		},
		Var5gQosProfile: &models.SubscribedDefaultQos{
			Var5qi:        var5qi,
			Arp:           &models.Arp{PriorityLevel: priorityLevel},
			PriorityLevel: priorityLevel,
		},
	}
	smDataObjModel.DnnConfigurations[config.DNN].PduSessionTypes.AllowedSessionTypes = append(smDataObjModel.DnnConfigurations[config.DNN].PduSessionTypes.AllowedSessionTypes, AllowedSessionTypes...)
	for _, sscMode := range AllowedSscModes {
		smDataObjModel.DnnConfigurations[config.DNN].SscModes.AllowedSscModes = append(smDataObjModel.DnnConfigurations[config.DNN].SscModes.AllowedSscModes, models.SscMode(sscMode))
	}
	smData = append(smData, smDataObjModel)
	return smData
}

func GetSmData(ueId string) ([]models.SessionManagementSubscriptionData, error) {
	subscriber, err := udmContext.DbInstance.GetSubscriber(ueId)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	profile, err := udmContext.DbInstance.GetProfileByID(subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}
	sessionManagementData := convertDbSessionManagementDataToModel(profile.BitrateDownlink, profile.BitrateUplink, profile.Var5qi, profile.PriorityLevel)
	return sessionManagementData, nil
}

func GetAndSetSmData(supi string, Dnn string, Snssai string) ([]models.SessionManagementSubscriptionData, error) {
	sessionManagementSubscriptionDataResp, err := GetSmData(supi)
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
	accessAndMobilitySubscriptionDataResp, err := GetAmData2(supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func GetSmfSelectData(ueId string) (*models.SmfSelectionSubscriptionData, error) {
	snssai := fmt.Sprintf("%d%s", config.Sst, config.Sd)
	smfSelectionData := &models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: make(map[string]models.SnssaiInfo),
	}
	smfSelectionData.SubscribedSnssaiInfos[snssai] = models.SnssaiInfo{
		DnnInfos: make([]models.DnnInfo, 0),
	}
	snssaiInfo := smfSelectionData.SubscribedSnssaiInfos[snssai]
	snssaiInfo.DnnInfos = append(snssaiInfo.DnnInfos, models.DnnInfo{
		Dnn: config.DNN,
	})
	smfSelectionData.SubscribedSnssaiInfos[snssai] = snssaiInfo
	return smfSelectionData, nil
}

func GetAndSetSmfSelectData(supi string) (
	*models.SmfSelectionSubscriptionData, error,
) {
	var body models.SmfSelectionSubscriptionData
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

func GetUeContextInSmfData(supi string) (*models.UeContextInSmfData, error) {
	var body models.UeContextInSmfData
	udmContext.CreateUeContextInSmfDataforUe(supi, body)
	pdusess := []*models.SmfRegistration{}
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
