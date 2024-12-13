package producer

import (
	"fmt"
	"strconv"

	"github.com/omec-project/openapi/models"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/udr/context"
)

var CurrentResourceUri string

// This function is defined twice, here and in the NMS. We should move it to a common place.
func convertDbAmDataToModel(dbAmData *dbModels.AccessAndMobilitySubscriptionData) *models.AccessAndMobilitySubscriptionData {
	if dbAmData == nil {
		return &models.AccessAndMobilitySubscriptionData{}
	}
	amData := &models.AccessAndMobilitySubscriptionData{
		Nssai: &models.Nssai{
			DefaultSingleNssais: make([]models.Snssai, 0),
			SingleNssais:        make([]models.Snssai, 0),
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: dbAmData.SubscribedUeAmbr.Downlink,
			Uplink:   dbAmData.SubscribedUeAmbr.Uplink,
		},
	}
	for _, snssai := range dbAmData.Nssai.DefaultSingleNssais {
		amData.Nssai.DefaultSingleNssais = append(amData.Nssai.DefaultSingleNssais, models.Snssai{
			Sd:  snssai.Sd,
			Sst: snssai.Sst,
		})
	}
	for _, snssai := range dbAmData.Nssai.SingleNssais {
		amData.Nssai.SingleNssais = append(amData.Nssai.SingleNssais, models.Snssai{
			Sd:  snssai.Sd,
			Sst: snssai.Sst,
		})
	}
	return amData
}

func GetAmData(ueId string) (*models.AccessAndMobilitySubscriptionData, error) {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	dbAmData := subscriber.AccessAndMobilitySubscriptionData
	if subscriber.AccessAndMobilitySubscriptionData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	amData := convertDbAmDataToModel(dbAmData)
	return amData, nil
}

func EditAuthenticationSubscription(ueId string, sequenceNumber string) error {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	if subscriber.AuthenticationSubscription == nil {
		return fmt.Errorf("USER_NOT_FOUND")
	}
	subscriber.AuthenticationSubscription.SequenceNumber = sequenceNumber
	err = queries.CreateSubscriber(subscriber)
	if err != nil {
		return fmt.Errorf("couldn't update subscriber %s: %v", ueId, err)
	}
	return nil
}

func convertDbAuthSubsDataToModel(dbAuthSubsData *dbModels.AuthenticationSubscription) *models.AuthenticationSubscription {
	if dbAuthSubsData == nil {
		return &models.AuthenticationSubscription{}
	}
	authSubsData := &models.AuthenticationSubscription{}
	authSubsData.AuthenticationManagementField = dbAuthSubsData.AuthenticationManagementField
	authSubsData.AuthenticationMethod = models.AuthMethod(dbAuthSubsData.AuthenticationMethod)
	if dbAuthSubsData.Milenage != nil {
		authSubsData.Milenage = &models.Milenage{
			Op: &models.Op{
				EncryptionAlgorithm: dbAuthSubsData.Milenage.Op.EncryptionAlgorithm,
				EncryptionKey:       dbAuthSubsData.Milenage.Op.EncryptionKey,
				OpValue:             dbAuthSubsData.Milenage.Op.OpValue,
			},
		}
	}
	if dbAuthSubsData.Opc != nil {
		authSubsData.Opc = &models.Opc{
			EncryptionAlgorithm: dbAuthSubsData.Opc.EncryptionAlgorithm,
			EncryptionKey:       dbAuthSubsData.Opc.EncryptionKey,
			OpcValue:            dbAuthSubsData.Opc.OpcValue,
		}
	}
	if dbAuthSubsData.PermanentKey != nil {
		authSubsData.PermanentKey = &models.PermanentKey{
			EncryptionAlgorithm: dbAuthSubsData.PermanentKey.EncryptionAlgorithm,
			EncryptionKey:       dbAuthSubsData.PermanentKey.EncryptionKey,
			PermanentKeyValue:   dbAuthSubsData.PermanentKey.PermanentKeyValue,
		}
	}
	authSubsData.SequenceNumber = dbAuthSubsData.SequenceNumber

	return authSubsData
}

func GetAuthSubsData(ueId string) (*models.AuthenticationSubscription, error) {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	dbAuthSubsData := subscriber.AuthenticationSubscription
	if dbAuthSubsData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	authSubsData := convertDbAuthSubsDataToModel(dbAuthSubsData)
	return authSubsData, nil
}

// We have this function twice, here and in the NMS. We should move it to a common place.
func convertDbAmPolicyDataToModel(dbAmPolicyData *dbModels.AmPolicyData) *models.AmPolicyData {
	if dbAmPolicyData == nil {
		return &models.AmPolicyData{}
	}
	amPolicyData := &models.AmPolicyData{
		SubscCats: dbAmPolicyData.SubscCats,
	}
	return amPolicyData
}

func GetAmPolicyData(ueId string) (*models.AmPolicyData, error) {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	dbAmPolicyData := subscriber.AmPolicyData
	if dbAmPolicyData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	amPolicyData := convertDbAmPolicyDataToModel(dbAmPolicyData)
	return amPolicyData, nil
}

// We have this function twice, here and in the NMS. We should move it to a common place.
func convertDbSmPolicyDataToModel(dbSmPolicyData *dbModels.SmPolicyData) *models.SmPolicyData {
	if dbSmPolicyData == nil {
		return &models.SmPolicyData{}
	}
	smPolicyData := &models.SmPolicyData{
		SmPolicySnssaiData: make(map[string]models.SmPolicySnssaiData),
	}
	for snssai, dbSmPolicySnssaiData := range dbSmPolicyData.SmPolicySnssaiData {
		smPolicyData.SmPolicySnssaiData[snssai] = models.SmPolicySnssaiData{
			Snssai: &models.Snssai{
				Sd:  dbSmPolicySnssaiData.Snssai.Sd,
				Sst: dbSmPolicySnssaiData.Snssai.Sst,
			},
			SmPolicyDnnData: make(map[string]models.SmPolicyDnnData),
		}
		smPolicySnssaiData := smPolicyData.SmPolicySnssaiData[snssai]
		for dnn, dbSmPolicyDnnData := range dbSmPolicySnssaiData.SmPolicyDnnData {
			smPolicySnssaiData.SmPolicyDnnData[dnn] = models.SmPolicyDnnData{
				Dnn: dbSmPolicyDnnData.Dnn,
			}
		}
		smPolicyData.SmPolicySnssaiData[snssai] = smPolicySnssaiData
	}
	return smPolicyData
}

func GetSmPolicyData(ueId string) (*models.SmPolicyData, error) {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	dbSmPolicyData := subscriber.SmPolicyData
	if dbSmPolicyData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	smPolicyData := convertDbSmPolicyDataToModel(dbSmPolicyData)
	return smPolicyData, nil
}

// I'm not sure whether we need this function or not. It's used but
// the fact that it returns an empty list and the e2e tests
// still pass makes me think that it's not used.
func GetSMFRegistrations(supi string) ([]*models.SmfRegistration, error) {
	// return empty list
	return []*models.SmfRegistration{}, nil
}

func RemovesdmSubscriptions(ueId string, subsId string) error {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return fmt.Errorf("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	_, ok = UESubsData.SdmSubscriptions[subsId]

	if !ok {
		return fmt.Errorf("SUBSCRIPTION_NOT_FOUND")
	}
	delete(UESubsData.SdmSubscriptions, subsId)

	return nil
}

func Updatesdmsubscriptions(ueId string, subsId string, SdmSubscription models.SdmSubscription) error {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return fmt.Errorf("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	_, ok = UESubsData.SdmSubscriptions[subsId]

	if !ok {
		return fmt.Errorf("SUBSCRIPTION_NOT_FOUND")
	}
	SdmSubscription.SubscriptionId = subsId
	UESubsData.SdmSubscriptions[subsId] = &SdmSubscription

	return nil
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

func convertDbSessionManagementDataToModel(dbSmData []*dbModels.SessionManagementSubscriptionData) []models.SessionManagementSubscriptionData {
	if dbSmData == nil {
		return nil
	}
	smData := make([]models.SessionManagementSubscriptionData, 0)
	for _, smDataObj := range dbSmData {
		smDataObjModel := models.SessionManagementSubscriptionData{
			SingleNssai: &models.Snssai{
				Sst: smDataObj.SingleNssai.Sst,
				Sd:  smDataObj.SingleNssai.Sd,
			},
			DnnConfigurations: make(map[string]models.DnnConfiguration),
		}
		for dnn, dnnConfig := range smDataObj.DnnConfigurations {
			smDataObjModel.DnnConfigurations[dnn] = models.DnnConfiguration{
				PduSessionTypes: &models.PduSessionTypes{
					DefaultSessionType:  models.PduSessionType(dnnConfig.PduSessionTypes.DefaultSessionType),
					AllowedSessionTypes: make([]models.PduSessionType, 0),
				},
				SscModes: &models.SscModes{
					DefaultSscMode:  models.SscMode(dnnConfig.SscModes.DefaultSscMode),
					AllowedSscModes: make([]models.SscMode, 0),
				},
				SessionAmbr: &models.Ambr{
					Downlink: dnnConfig.SessionAmbr.Downlink,
					Uplink:   dnnConfig.SessionAmbr.Uplink,
				},
				Var5gQosProfile: &models.SubscribedDefaultQos{
					Var5qi:        dnnConfig.Var5gQosProfile.Var5qi,
					Arp:           &models.Arp{PriorityLevel: dnnConfig.Var5gQosProfile.Arp.PriorityLevel},
					PriorityLevel: dnnConfig.Var5gQosProfile.PriorityLevel,
				},
			}
			for _, sessionType := range dnnConfig.PduSessionTypes.AllowedSessionTypes {
				smDataObjModel.DnnConfigurations[dnn].PduSessionTypes.AllowedSessionTypes = append(smDataObjModel.DnnConfigurations[dnn].PduSessionTypes.AllowedSessionTypes, models.PduSessionType(sessionType))
			}
			for _, sscMode := range dnnConfig.SscModes.AllowedSscModes {
				smDataObjModel.DnnConfigurations[dnn].SscModes.AllowedSscModes = append(smDataObjModel.DnnConfigurations[dnn].SscModes.AllowedSscModes, models.SscMode(sscMode))
			}
		}
		smData = append(smData, smDataObjModel)
	}
	return smData
}

func GetSmData(ueId string) ([]models.SessionManagementSubscriptionData, error) {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	dbSessionManagementData := subscriber.SessionManagementSubscriptionData
	if dbSessionManagementData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	sessionManagementData := convertDbSessionManagementDataToModel(dbSessionManagementData)
	return sessionManagementData, nil
}

// We have this function twice, here and in the NMS. We should move it to a common place.
func convertDbSmfSelectionDataToModel(dbSmfSelectionData *dbModels.SmfSelectionSubscriptionData) *models.SmfSelectionSubscriptionData {
	if dbSmfSelectionData == nil {
		return &models.SmfSelectionSubscriptionData{}
	}
	smfSelectionData := &models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: make(map[string]models.SnssaiInfo),
	}
	for snssai, dbSnssaiInfo := range dbSmfSelectionData.SubscribedSnssaiInfos {
		smfSelectionData.SubscribedSnssaiInfos[snssai] = models.SnssaiInfo{
			DnnInfos: make([]models.DnnInfo, 0),
		}
		snssaiInfo := smfSelectionData.SubscribedSnssaiInfos[snssai]
		for _, dbDnnInfo := range dbSnssaiInfo.DnnInfos {
			snssaiInfo.DnnInfos = append(snssaiInfo.DnnInfos, models.DnnInfo{
				Dnn: dbDnnInfo.Dnn,
			})
		}
		smfSelectionData.SubscribedSnssaiInfos[snssai] = snssaiInfo
	}
	return smfSelectionData
}

func GetSmfSelectData(ueId string) (*models.SmfSelectionSubscriptionData, error) {
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.UdrLog.Warnln(err)
	}
	dbSmfSelectionSubscriptionData := subscriber.SmfSelectionSubscriptionData
	if dbSmfSelectionSubscriptionData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	smfSelectionSubscriptionData := convertDbSmfSelectionDataToModel(dbSmfSelectionSubscriptionData)
	return smfSelectionSubscriptionData, nil
}
