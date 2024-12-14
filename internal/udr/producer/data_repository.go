package producer

import (
	"fmt"
	"strconv"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/udr/context"
)

const (
	AuthenticationManagementField = "8000"
	EncryptionAlgorithm           = 0
	EncryptionKey                 = 0
	OpValue                       = ""
)

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

var AllowedSessionTypes = []models.PduSessionType{models.PduSessionType_IPV4}

// This function is defined twice, here and in the NMS. We should move it to a common place.
func convertDbAmDataToModel(sd string, sst int32, bitrateDownlink string, bitrateUplink string) *models.AccessAndMobilitySubscriptionData {
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
		Sd:  sd,
		Sst: sst,
	})
	amData.Nssai.SingleNssais = append(amData.Nssai.SingleNssais, models.Snssai{
		Sd:  sd,
		Sst: sst,
	})
	return amData
}

func GetAmData(ueId string) (*models.AccessAndMobilitySubscriptionData, error) {
	udrSelf := context.UDR_Self()
	subscriber, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	amData := convertDbAmDataToModel(subscriber.Sd, subscriber.Sst, subscriber.BitRateDownlink, subscriber.BitRateUplink)
	return amData, nil
}

func EditAuthenticationSubscription(ueId string, sequenceNumber string) error {
	udrSelf := context.UDR_Self()
	subscriber, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	subscriber.SequenceNumber = sequenceNumber
	err = udrSelf.DbInstance.CreateSubscriber(subscriber)
	if err != nil {
		return fmt.Errorf("couldn't update subscriber %s: %v", ueId, err)
	}
	return nil
}

func convertDbAuthSubsDataToModel(opc string, key string, sequenceNumber string) *models.AuthenticationSubscription {
	authSubsData := &models.AuthenticationSubscription{}
	authSubsData.AuthenticationManagementField = AuthenticationManagementField
	authSubsData.AuthenticationMethod = models.AuthMethod__5_G_AKA
	authSubsData.Milenage = &models.Milenage{
		Op: &models.Op{
			EncryptionAlgorithm: EncryptionAlgorithm,
			EncryptionKey:       EncryptionKey,
			OpValue:             OpValue,
		},
	}
	authSubsData.Opc = &models.Opc{
		EncryptionAlgorithm: EncryptionAlgorithm,
		EncryptionKey:       EncryptionKey,
		OpcValue:            opc,
	}
	authSubsData.PermanentKey = &models.PermanentKey{
		EncryptionAlgorithm: EncryptionAlgorithm,
		EncryptionKey:       EncryptionKey,
		PermanentKeyValue:   key,
	}
	authSubsData.SequenceNumber = sequenceNumber

	return authSubsData
}

func GetAuthSubsData(ueId string) (*models.AuthenticationSubscription, error) {
	udrSelf := context.UDR_Self()
	subscriber, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	authSubsData := convertDbAuthSubsDataToModel(subscriber.OpcValue, subscriber.PermanentKeyValue, subscriber.SequenceNumber)
	return authSubsData, nil
}

func GetAmPolicyData(ueId string) (*models.AmPolicyData, error) {
	udrSelf := context.UDR_Self()
	_, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	amPolicyData := &models.AmPolicyData{}
	return amPolicyData, nil
}

// We have this function twice, here and in the NMS. We should move it to a common place.
func convertDbSmPolicyDataToModel(sst int32, sd string, dnn string) *models.SmPolicyData {
	smPolicyData := &models.SmPolicyData{
		SmPolicySnssaiData: make(map[string]models.SmPolicySnssaiData),
	}
	snssai := fmt.Sprintf("%d%s", sst, sd)
	smPolicyData.SmPolicySnssaiData[snssai] = models.SmPolicySnssaiData{
		Snssai: &models.Snssai{
			Sd:  sd,
			Sst: sst,
		},
		SmPolicyDnnData: make(map[string]models.SmPolicyDnnData),
	}
	smPolicySnssaiData := smPolicyData.SmPolicySnssaiData[snssai]
	smPolicySnssaiData.SmPolicyDnnData[dnn] = models.SmPolicyDnnData{
		Dnn: dnn,
	}
	smPolicyData.SmPolicySnssaiData[snssai] = smPolicySnssaiData
	return smPolicyData
}

func GetSmPolicyData(ueId string) (*models.SmPolicyData, error) {
	udrSelf := context.UDR_Self()
	subscriber, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	smPolicyData := convertDbSmPolicyDataToModel(subscriber.Sst, subscriber.Sd, subscriber.Dnn)
	return smPolicyData, nil
}

// I'm not sure whether we need this function or not. It's used but
// the fact that it returns an empty list and the e2e tests
// still pass makes me think that it's not used.
func GetSMFRegistrations(supi string) ([]*models.SmfRegistration, error) {
	// return empty list
	return []*models.SmfRegistration{}, nil
}

func CreateSdmSubscriptions(SdmSubscription models.SdmSubscription, ueId string) models.SdmSubscription {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		udrSelf.UESubsCollection.Store(ueId, new(context.UESubsData))
		value, _ = udrSelf.UESubsCollection.Load(ueId)
	}
	UESubsData := value.(*context.UESubsData)
	if UESubsData.SdmSubscriptions == nil {
		UESubsData.SdmSubscriptions = make(map[string]*models.SdmSubscription)
	}

	newSubscriptionID := strconv.Itoa(udrSelf.SdmSubscriptionIDGenerator)
	SdmSubscription.SubscriptionId = newSubscriptionID
	UESubsData.SdmSubscriptions[newSubscriptionID] = &SdmSubscription
	udrSelf.SdmSubscriptionIDGenerator++

	return SdmSubscription
}

func convertDbSessionManagementDataToModel(
	dnn string,
	sst int32,
	sd string,
	bitrateDownlink string,
	bitrateUplink string,
	var5qi int32,
	priorityLevel int32,
) []models.SessionManagementSubscriptionData {
	smData := make([]models.SessionManagementSubscriptionData, 0)
	smDataObjModel := models.SessionManagementSubscriptionData{
		SingleNssai: &models.Snssai{
			Sst: sst,
			Sd:  sd,
		},
		DnnConfigurations: make(map[string]models.DnnConfiguration),
	}
	smDataObjModel.DnnConfigurations[dnn] = models.DnnConfiguration{
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
	for _, sessionType := range AllowedSessionTypes {
		smDataObjModel.DnnConfigurations[dnn].PduSessionTypes.AllowedSessionTypes = append(smDataObjModel.DnnConfigurations[dnn].PduSessionTypes.AllowedSessionTypes, sessionType)
	}
	for _, sscMode := range AllowedSscModes {
		smDataObjModel.DnnConfigurations[dnn].SscModes.AllowedSscModes = append(smDataObjModel.DnnConfigurations[dnn].SscModes.AllowedSscModes, models.SscMode(sscMode))
	}
	smData = append(smData, smDataObjModel)
	return smData
}

func GetSmData(ueId string) ([]models.SessionManagementSubscriptionData, error) {
	udrSelf := context.UDR_Self()
	subscriber, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}

	sessionManagementData := convertDbSessionManagementDataToModel(subscriber.Dnn, subscriber.Sst, subscriber.Sd, subscriber.BitRateDownlink, subscriber.BitRateUplink, subscriber.Var5qi, subscriber.PriorityLevel)
	return sessionManagementData, nil
}

// We have this function twice, here and in the NMS. We should move it to a common place.
func convertDbSmfSelectionDataToModel(snssai, dnn string) *models.SmfSelectionSubscriptionData {
	smfSelectionData := &models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: make(map[string]models.SnssaiInfo),
	}
	smfSelectionData.SubscribedSnssaiInfos[snssai] = models.SnssaiInfo{
		DnnInfos: make([]models.DnnInfo, 0),
	}
	snssaiInfo := smfSelectionData.SubscribedSnssaiInfos[snssai]
	snssaiInfo.DnnInfos = append(snssaiInfo.DnnInfos, models.DnnInfo{
		Dnn: dnn,
	})
	smfSelectionData.SubscribedSnssaiInfos[snssai] = snssaiInfo
	return smfSelectionData
}

func GetSmfSelectData(ueId string) (*models.SmfSelectionSubscriptionData, error) {
	udrSelf := context.UDR_Self()
	subscriber, err := udrSelf.DbInstance.GetSubscriberByUeID(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	snssai := fmt.Sprintf("%d%s", subscriber.Sst, subscriber.Sd)
	smfSelectionSubscriptionData := convertDbSmfSelectionDataToModel(snssai, subscriber.Dnn)
	return smfSelectionSubscriptionData, nil
}
