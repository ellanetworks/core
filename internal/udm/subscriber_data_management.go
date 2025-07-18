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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("ella-core/udm")

var AllowedSessionTypes = []models.PduSessionType{models.PduSessionTypeIPv4}

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

type subsID = string

type UESubsData struct {
	SdmSubscriptions map[subsID]*models.SdmSubscription
}

func GetAmData(ctx context.Context, ueID string) (*models.AccessAndMobilitySubscriptionData, error) {
	subscriber, err := udmContext.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}
	profile, err := udmContext.DBInstance.GetProfileByID(ctx, subscriber.ProfileID)
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

func GetAmDataAndSetAMSubscription(ctx context.Context, supi string) (*models.AccessAndMobilitySubscriptionData, error) {
	ctx, span := tracer.Start(ctx, "UDM GetAmDataAndSetAMSubscription")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
	amData, err := GetAmData(ctx, supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func GetSmData(ctx context.Context, ueID string) ([]models.SessionManagementSubscriptionData, error) {
	subscriber, err := udmContext.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}
	profile, err := udmContext.DBInstance.GetProfileByID(ctx, subscriber.ProfileID)
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

func GetAndSetSmData(ctx context.Context, supi string, Dnn string, Snssai string) ([]models.SessionManagementSubscriptionData, error) {
	sessionManagementSubscriptionDataResp, err := GetSmData(ctx, supi)
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

func GetNssai(ctx context.Context, supi string) (*models.Nssai, error) {
	ctx, span := tracer.Start(ctx, "UDM GetNssai")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
	accessAndMobilitySubscriptionDataResp, err := GetAmData(ctx, supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := udmContext.NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func GetSmfSelectData(ctx context.Context, ueID string) (*models.SmfSelectionSubscriptionData, error) {
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

func GetAndSetSmfSelectData(ctx context.Context, supi string) (*models.SmfSelectionSubscriptionData, error) {
	ctx, span := tracer.Start(ctx, "UDM SetSmfSelectData")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
	var body models.SmfSelectionSubscriptionData
	udmContext.CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := GetSmfSelectData(ctx, supi)
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

func CreateSubscription(ctx context.Context, sdmSubscription *models.SdmSubscription, supi string) error {
	_, span := tracer.Start(ctx, "UDM CreateSubscription")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
	sdmSubscriptionResp := CreateSdmSubscriptions(*sdmSubscription, supi)
	udmUe, _ := udmContext.UdmUeFindBySupi(supi)
	if udmUe == nil {
		udmUe = udmContext.NewUdmUe(supi)
	}
	udmUe.CreateSubscriptiontoNotifChange(sdmSubscriptionResp.SubscriptionID, &sdmSubscriptionResp)
	return nil
}

func GetUeContextInSmfData(ctx context.Context, supi string) (*models.UeContextInSmfData, error) {
	_, span := tracer.Start(ctx, "UDM GetUeContextInSmfData")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
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
