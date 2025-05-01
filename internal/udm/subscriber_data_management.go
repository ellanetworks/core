// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/models"
)

var AllowedSessionTypes = []models.PduSessionType{models.PduSessionTypeIPv4}

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

type subsID = string

type UESubsData struct {
	SdmSubscriptions map[subsID]*models.SdmSubscription
}

func GetAmData(ueID string, ctx context.Context) (*models.AccessAndMobilitySubscriptionData, error) {
	subscriber, err := udmContext.DBInstance.GetSubscriber(ueID, ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}
	profile, err := udmContext.DBInstance.GetProfileByID(subscriber.ProfileID, ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}
	operator, err := udmContext.DBInstance.GetOperator(ctx)
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

func GetAmDataAndSetAMSubscription(supi string, ctx context.Context) (*models.AccessAndMobilitySubscriptionData, error) {
	amData, err := GetAmData(supi, ctx)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func GetSmData(ueID string, ctx context.Context) ([]models.SessionManagementSubscriptionData, error) {
	subscriber, err := udmContext.DBInstance.GetSubscriber(ueID, ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}
	profile, err := udmContext.DBInstance.GetProfileByID(subscriber.ProfileID, ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}
	operator, err := udmContext.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}
	smData := make([]models.SessionManagementSubscriptionData, 0)
	smDataObjModel := models.SessionManagementSubscriptionData{
		SingleNssai: &models.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
		DnnConfigurations: make(map[string]models.DnnConfiguration),
	}
	smDataObjModel.DnnConfigurations[config.DNN] = models.DnnConfiguration{
		PduSessionTypes: &models.PduSessionTypes{
			DefaultSessionType:  models.PduSessionTypeIPv4,
			AllowedSessionTypes: make([]models.PduSessionType, 0),
		},
		SscModes: &models.SscModes{
			DefaultSscMode:  models.SscMode1,
			AllowedSscModes: make([]models.SscMode, 0),
		},
		SessionAmbr: &models.Ambr{
			Downlink: profile.BitrateDownlink,
			Uplink:   profile.BitrateUplink,
		},
		Var5gQosProfile: &models.SubscribedDefaultQos{
			Var5qi:        profile.Var5qi,
			Arp:           &models.Arp{PriorityLevel: profile.PriorityLevel},
			PriorityLevel: profile.PriorityLevel,
		},
	}
	smDataObjModel.DnnConfigurations[config.DNN].PduSessionTypes.AllowedSessionTypes = append(smDataObjModel.DnnConfigurations[config.DNN].PduSessionTypes.AllowedSessionTypes, AllowedSessionTypes...)
	for _, sscMode := range AllowedSscModes {
		smDataObjModel.DnnConfigurations[config.DNN].SscModes.AllowedSscModes = append(smDataObjModel.DnnConfigurations[config.DNN].SscModes.AllowedSscModes, models.SscMode(sscMode))
	}
	smData = append(smData, smDataObjModel)
	return smData, nil
}

func GetAndSetSmData(supi string, Dnn string, Snssai string, ctx context.Context) ([]models.SessionManagementSubscriptionData, error) {
	sessionManagementSubscriptionDataResp, err := GetSmData(supi, ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting sm data: %+v", err)
	}

	udmUe := udmContext.NewUdmUe(supi)
	smData, err := udmContext.ManageSmData(sessionManagementSubscriptionDataResp, Snssai, Dnn)
	if err != nil {
		return nil, fmt.Errorf("error managing sm data: %+v", err)
	}
	udmUe.SetSMSubsData(smData)

	rspSMSubDataList := make([]models.SessionManagementSubscriptionData, 0, 4)

	udmUe.SmSubsDataLock.RLock()
	for _, eachSMSubData := range udmUe.SessionManagementSubsData {
		rspSMSubDataList = append(rspSMSubDataList, eachSMSubData)
	}
	udmUe.SmSubsDataLock.RUnlock()
	return rspSMSubDataList, nil
}

func GetNssai(supi string, ctx context.Context) (*models.Nssai, error) {
	accessAndMobilitySubscriptionDataResp, err := GetAmData(supi, ctx)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func GetSmfSelectData(ueID string, ctx context.Context) (*models.SmfSelectionSubscriptionData, error) {
	operator, err := udmContext.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}
	snssai := fmt.Sprintf("%d%s", operator.Sst, operator.GetHexSd())
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

func GetAndSetSmfSelectData(supi string, ctx context.Context) (*models.SmfSelectionSubscriptionData, error) {
	var body models.SmfSelectionSubscriptionData
	udmContext.CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := GetSmfSelectData(supi, ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting smf selection data: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelectionSubscriptionDataResp)
	return udmUe.SmfSelSubsData, nil
}

func CreateSdmSubscriptions(SdmSubscription models.SdmSubscription, ueID string) models.SdmSubscription {
	value, ok := udmContext.UESubsCollection.Load(ueID)
	if !ok {
		udmContext.UESubsCollection.Store(ueID, new(UESubsData))
		value, _ = udmContext.UESubsCollection.Load(ueID)
	}
	UESubsData := value.(*UESubsData)
	if UESubsData.SdmSubscriptions == nil {
		UESubsData.SdmSubscriptions = make(map[string]*models.SdmSubscription)
	}

	newSubscriptionID := strconv.Itoa(udmContext.SdmSubscriptionIDGenerator)
	SdmSubscription.SubscriptionID = newSubscriptionID
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
	udmUe.CreateSubscriptiontoNotifChange(sdmSubscriptionResp.SubscriptionID, &sdmSubscriptionResp)
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
		pduSession.SmfInstanceID = element.SmfInstanceID
		pduSession.PlmnID = element.PlmnID
		pduSessionMap[strconv.Itoa(int(element.PduSessionID))] = pduSession
	}
	var ueContextInSmfData models.UeContextInSmfData
	ueContextInSmfData.PduSessions = pduSessionMap
	var pgwInfoArray []models.PgwInfo
	for _, element := range pdusess {
		var pgwInfo models.PgwInfo
		pgwInfo.Dnn = element.Dnn
		pgwInfo.PgwFqdn = element.PgwFqdn
		pgwInfo.PlmnID = element.PlmnID
		pgwInfoArray = append(pgwInfoArray, pgwInfo)
	}

	ueContextInSmfData.PgwInfo = pgwInfoArray

	udmUe := udmContext.NewUdmUe(supi)
	udmUe.UeCtxtInSmfData = &ueContextInSmfData
	return udmUe.UeCtxtInSmfData, nil
}
