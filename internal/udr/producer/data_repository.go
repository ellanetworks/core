package producer

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/mitchellh/mapstructure"
	"github.com/omec-project/openapi/models"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/logger"
	"github.com/yeastengine/ella/internal/udr/util"
)

var CurrentResourceUri string

// This function is defined twice, here and in the NMS. We should move it to a common place.
func convertDbAmDataToModel(dbAmData *dbModels.AccessAndMobilitySubscriptionData) *models.AccessAndMobilitySubscriptionData {
	if dbAmData == nil {
		return &models.AccessAndMobilitySubscriptionData{}
	}
	amData := &models.AccessAndMobilitySubscriptionData{
		Gpsis: dbAmData.Gpsis,
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
	dbAmData, err := queries.GetAmData(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	if dbAmData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	amData := convertDbAmDataToModel(dbAmData)
	return amData, nil
}

func PatchAmfContext3gppProcedure(ueId string, patchItem []models.PatchItem) error {
	origValue, err := queries.GetAmf3GPP(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}

	dbPatchItem := make([]dbModels.PatchItem, 0)
	for _, item := range patchItem {
		dbPatchItem = append(dbPatchItem, dbModels.PatchItem{
			Op:    item.Op,
			Path:  item.Path,
			From:  item.From,
			Value: item.Value,
		})
	}
	err = queries.PatchAmf3GPP(ueId, dbPatchItem)
	if err != nil {
		return fmt.Errorf("ModifyNotAllowed")
	}

	newValue, err := queries.GetAmf3GPP(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	PreHandleOnDataChangeNotify(ueId, CurrentResourceUri, patchItem, origValue, newValue)
	return nil
}

func CreateAmfContext3gppProcedure(ueId string, Amf3GppAccessRegistration models.Amf3GppAccessRegistration) error {
	dbAmfData := &dbModels.Amf3GppAccessRegistration{
		InitialRegistrationInd: Amf3GppAccessRegistration.InitialRegistrationInd,
		Guami: &dbModels.Guami{
			PlmnId: &dbModels.PlmnId{
				Mcc: Amf3GppAccessRegistration.Guami.PlmnId.Mcc,
				Mnc: Amf3GppAccessRegistration.Guami.PlmnId.Mnc,
			},
			AmfId: Amf3GppAccessRegistration.Guami.AmfId,
		},
		RatType:          dbModels.RatType(Amf3GppAccessRegistration.RatType),
		AmfInstanceId:    Amf3GppAccessRegistration.AmfInstanceId,
		ImsVoPs:          dbModels.ImsVoPs(Amf3GppAccessRegistration.ImsVoPs),
		DeregCallbackUri: Amf3GppAccessRegistration.DeregCallbackUri,
	}
	err := queries.EditAmf3GPP(ueId, dbAmfData)
	return err
}

func convertDbAmf3GppAccessRegistrationToModel(dbAmf3Gpp *dbModels.Amf3GppAccessRegistration) *models.Amf3GppAccessRegistration {
	if dbAmf3Gpp == nil {
		return &models.Amf3GppAccessRegistration{}
	}
	amf3Gpp := &models.Amf3GppAccessRegistration{
		InitialRegistrationInd: dbAmf3Gpp.InitialRegistrationInd,
		Guami: &models.Guami{
			PlmnId: &models.PlmnId{
				Mcc: dbAmf3Gpp.Guami.PlmnId.Mcc,
				Mnc: dbAmf3Gpp.Guami.PlmnId.Mnc,
			},
			AmfId: dbAmf3Gpp.Guami.AmfId,
		},
		RatType:          models.RatType(dbAmf3Gpp.RatType),
		AmfInstanceId:    dbAmf3Gpp.AmfInstanceId,
		ImsVoPs:          models.ImsVoPs(dbAmf3Gpp.ImsVoPs),
		DeregCallbackUri: dbAmf3Gpp.DeregCallbackUri,
	}
	return amf3Gpp
}

func QueryAmfContext3gppProcedure(ueId string) (*models.Amf3GppAccessRegistration, error) {
	dbAmf3Gpp, err := queries.GetAmf3GPP(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	if dbAmf3Gpp == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	amf3Gpp := convertDbAmf3GppAccessRegistrationToModel(dbAmf3Gpp)
	return amf3Gpp, nil
}

func EditAuthenticationSubscription(ueId string, patchItem []models.PatchItem) error {
	origValue, err := queries.GetAuthenticationSubscription(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}

	dbPatchItem := make([]dbModels.PatchItem, 0)
	for _, item := range patchItem {
		dbPatchItem = append(dbPatchItem, dbModels.PatchItem{
			Op:    item.Op,
			Path:  item.Path,
			From:  item.From,
			Value: item.Value,
		})
	}
	err = queries.PatchAuthenticationSubscription(ueId, dbPatchItem)

	if err == nil {
		newValue, err := queries.GetAuthenticationSubscription(ueId)
		if err != nil {
			logger.DataRepoLog.Warnln(err)
		}
		PreHandleOnDataChangeNotify(ueId, CurrentResourceUri, patchItem, origValue, newValue)
		return nil
	} else {
		return err
	}
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
	dbAuthSubs, err := queries.GetAuthenticationSubscription(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	if dbAuthSubs == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	authSubs := convertDbAuthSubsDataToModel(dbAuthSubs)
	return authSubs, nil
}

func EditAuthenticationStatus(ueID string, authStatus models.AuthEvent) error {
	dbAuthStatus := &dbModels.AuthEvent{
		NfInstanceId:       authStatus.NfInstanceId,
		Success:            authStatus.Success,
		TimeStamp:          authStatus.TimeStamp,
		AuthType:           dbModels.AuthType(authStatus.AuthType),
		ServingNetworkName: authStatus.ServingNetworkName,
	}

	err := queries.EditAuthenticationStatus(ueID, dbAuthStatus)
	return err
}

func QueryAuthenticationStatusProcedure(ueId string) (*dbModels.AuthEvent,
	*models.ProblemDetails,
) {
	authEvent, err := queries.GetAuthenticationStatus(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}

	if authEvent != nil {
		return authEvent, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func PolicyDataSubsToNotifyPostProcedure(PolicyDataSubscription models.PolicyDataSubscription) string {
	udrSelf := context.UDR_Self()

	newSubscriptionID := strconv.Itoa(udrSelf.PolicyDataSubscriptionIDGenerator)
	udrSelf.PolicyDataSubscriptions[newSubscriptionID] = &PolicyDataSubscription
	udrSelf.PolicyDataSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/subs-to-notify/{subsId} */
	locationHeader := fmt.Sprintf("%s/policy-data/subs-to-notify/%s", udrSelf.GetIPv4GroupUri(context.NUDR_DR),
		newSubscriptionID)

	return locationHeader
}

func PolicyDataSubsToNotifySubsIdDeleteProcedure(subsId string) (problemDetails *models.ProblemDetails) {
	udrSelf := context.UDR_Self()
	_, ok := udrSelf.PolicyDataSubscriptions[subsId]
	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(udrSelf.PolicyDataSubscriptions, subsId)

	return nil
}

func PolicyDataSubsToNotifySubsIdPutProcedure(subsId string,
	policyDataSubscription models.PolicyDataSubscription,
) (*models.PolicyDataSubscription, *models.ProblemDetails) {
	udrSelf := context.UDR_Self()
	_, ok := udrSelf.PolicyDataSubscriptions[subsId]
	if !ok {
		return nil, util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	udrSelf.PolicyDataSubscriptions[subsId] = &policyDataSubscription

	return &policyDataSubscription, nil
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

func PolicyDataUesUeIdAmDataGetProcedure(ueId string) (*models.AmPolicyData, error) {
	dbAmPolicyData, err := queries.GetAmPolicyData(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
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

func PolicyDataUesUeIdSmDataGetProcedure(ueId string) (*models.SmPolicyData, error) {
	dbSmPolicyData, err := queries.GetSmPolicyData(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	if dbSmPolicyData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	smPolicyData := convertDbSmPolicyDataToModel(dbSmPolicyData)
	return smPolicyData, nil
}

func CreateAMFSubscriptionsProcedure(subsId string, ueId string,
	AmfSubscriptionInfo []models.AmfSubscriptionInfo,
) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
	UESubsData := value.(*context.UESubsData)

	_, ok = UESubsData.EeSubscriptionCollection[subsId]
	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos = AmfSubscriptionInfo
	return nil
}

func RemoveAmfSubscriptionsInfoProcedure(subsId string, ueId string) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	if UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos == nil {
		return util.ProblemDetailsNotFound("AMFSUBSCRIPTION_NOT_FOUND")
	}

	UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos = nil

	return nil
}

func ModifyAmfSubscriptionInfoProcedure(ueId string, subsId string,
	patchItem []models.PatchItem,
) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
	UESubsData := value.(*context.UESubsData)

	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	if UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos == nil {
		return util.ProblemDetailsNotFound("AMFSUBSCRIPTION_NOT_FOUND")
	}
	var patchJSON []byte
	if patchJSONtemp, err := json.Marshal(patchItem); err != nil {
		logger.DataRepoLog.Errorln(err)
	} else {
		patchJSON = patchJSONtemp
	}
	var patch jsonpatch.Patch
	if patchtemp, err := jsonpatch.DecodePatch(patchJSON); err != nil {
		logger.DataRepoLog.Errorln(err)
		return util.ProblemDetailsModifyNotAllowed("PatchItem attributes are invalid")
	} else {
		patch = patchtemp
	}
	original, err := json.Marshal((UESubsData.EeSubscriptionCollection[subsId]).AmfSubscriptionInfos)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}

	modified, err := patch.Apply(original)
	if err != nil {
		return util.ProblemDetailsModifyNotAllowed("Occur error when applying PatchItem")
	}
	var modifiedData []models.AmfSubscriptionInfo
	err = json.Unmarshal(modified, &modifiedData)
	if err != nil {
		logger.DataRepoLog.Error(err)
	}

	UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos = modifiedData
	return nil
}

func GetAmfSubscriptionInfoProcedure(subsId string, ueId string) (*[]models.AmfSubscriptionInfo,
	*models.ProblemDetails,
) {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return nil, util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	if UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos == nil {
		return nil, util.ProblemDetailsNotFound("AMFSUBSCRIPTION_NOT_FOUND")
	}
	return &UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos, nil
}

func RemoveEeGroupSubscriptionsProcedure(ueGroupId string, subsId string) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UEGroupSubsData := value.(*context.UEGroupSubsData)
	_, ok = UEGroupSubsData.EeSubscriptions[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(UEGroupSubsData.EeSubscriptions, subsId)

	return nil
}

func UpdateEeGroupSubscriptionsProcedure(ueGroupId string, subsId string,
	EeSubscription models.EeSubscription,
) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UEGroupSubsData := value.(*context.UEGroupSubsData)
	_, ok = UEGroupSubsData.EeSubscriptions[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	UEGroupSubsData.EeSubscriptions[subsId] = &EeSubscription

	return nil
}

func CreateEeGroupSubscriptionsProcedure(ueGroupId string, EeSubscription models.EeSubscription) string {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		udrSelf.UEGroupCollection.Store(ueGroupId, new(context.UEGroupSubsData))
		value, _ = udrSelf.UEGroupCollection.Load(ueGroupId)
	}
	UEGroupSubsData := value.(*context.UEGroupSubsData)
	if UEGroupSubsData.EeSubscriptions == nil {
		UEGroupSubsData.EeSubscriptions = make(map[string]*models.EeSubscription)
	}

	newSubscriptionID := strconv.Itoa(udrSelf.EeSubscriptionIDGenerator)
	UEGroupSubsData.EeSubscriptions[newSubscriptionID] = &EeSubscription
	udrSelf.EeSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/nudr-dr/v1/subscription-data/group-data/{ueGroupId}/ee-subscriptions */
	locationHeader := fmt.Sprintf("%s/nudr-dr/v1/subscription-data/group-data/%s/ee-subscriptions/%s",
		udrSelf.GetIPv4GroupUri(context.NUDR_DR), ueGroupId, newSubscriptionID)

	return locationHeader
}

func QueryEeGroupSubscriptionsProcedure(ueGroupId string) ([]models.EeSubscription, *models.ProblemDetails) {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UEGroupSubsData := value.(*context.UEGroupSubsData)
	var eeSubscriptionSlice []models.EeSubscription

	for _, v := range UEGroupSubsData.EeSubscriptions {
		eeSubscriptionSlice = append(eeSubscriptionSlice, *v)
	}
	return eeSubscriptionSlice, nil
}

func RemoveeeSubscriptionsProcedure(ueId string, subsId string) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(UESubsData.EeSubscriptionCollection, subsId)
	return nil
}

func UpdateEesubscriptionsProcedure(ueId string, subsId string,
	EeSubscription models.EeSubscription,
) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	UESubsData.EeSubscriptionCollection[subsId].EeSubscriptions = &EeSubscription

	return nil
}

func CreateEeSubscriptionsProcedure(ueId string, EeSubscription models.EeSubscription) string {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		udrSelf.UESubsCollection.Store(ueId, new(context.UESubsData))
		value, _ = udrSelf.UESubsCollection.Load(ueId)
	}
	UESubsData := value.(*context.UESubsData)
	if UESubsData.EeSubscriptionCollection == nil {
		UESubsData.EeSubscriptionCollection = make(map[string]*context.EeSubscriptionCollection)
	}

	newSubscriptionID := strconv.Itoa(udrSelf.EeSubscriptionIDGenerator)
	UESubsData.EeSubscriptionCollection[newSubscriptionID] = new(context.EeSubscriptionCollection)
	UESubsData.EeSubscriptionCollection[newSubscriptionID].EeSubscriptions = &EeSubscription
	udrSelf.EeSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/{ueId}/context-data/ee-subscriptions/{subsId} */
	locationHeader := fmt.Sprintf("%s/subscription-data/%s/context-data/ee-subscriptions/%s",
		udrSelf.GetIPv4GroupUri(context.NUDR_DR), ueId, newSubscriptionID)

	return locationHeader
}

func QueryeesubscriptionsProcedure(ueId string) ([]models.EeSubscription, *models.ProblemDetails) {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	var eeSubscriptionSlice []models.EeSubscription

	for _, v := range UESubsData.EeSubscriptionCollection {
		eeSubscriptionSlice = append(eeSubscriptionSlice, *v.EeSubscriptions)
	}
	return eeSubscriptionSlice, nil
}

func QueryProvisionedDataProcedure(ueId string, provisionedDataSets models.ProvisionedDataSets) (*models.ProvisionedDataSets, *models.ProblemDetails) {
	{
		accessAndMobilitySubscriptionData, err := queries.GetAmData(ueId)
		if err != nil {
			logger.DataRepoLog.Warnln(err)
		}
		if accessAndMobilitySubscriptionData != nil {
			var tmp models.AccessAndMobilitySubscriptionData
			err := mapstructure.Decode(accessAndMobilitySubscriptionData, &tmp)
			if err != nil {
				panic(err)
			}
			provisionedDataSets.AmData = &tmp
		}
	}

	{
		smfSelectionSubscriptionData, err := queries.GetSmfSelectionSubscriptionData(ueId)
		if err != nil {
			logger.DataRepoLog.Warnln(err)
		}
		if smfSelectionSubscriptionData != nil {
			var tmp models.SmfSelectionSubscriptionData
			err := mapstructure.Decode(smfSelectionSubscriptionData, &tmp)
			if err != nil {
				panic(err)
			}
			provisionedDataSets.SmfSelData = &tmp
		}
	}

	{
		sessionManagementSubscriptionDatas, err := queries.ListSmData(ueId)
		if err != nil {
			logger.DataRepoLog.Warnln(err)
		}
		if sessionManagementSubscriptionDatas != nil {
			var tmp []models.SessionManagementSubscriptionData
			err := mapstructure.Decode(sessionManagementSubscriptionDatas, &tmp)
			if err != nil {
				panic(err)
			}
			provisionedDataSets.SmData = tmp
		}
	}

	if !reflect.DeepEqual(provisionedDataSets, models.ProvisionedDataSets{}) {
		return &provisionedDataSets, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
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

func CreateSdmSubscriptions(SdmSubscription models.SdmSubscription, ueId string) (string, models.SdmSubscription) {
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

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/{ueId}/context-data/sdm-subscriptions/{subsId}' */
	locationHeader := fmt.Sprintf("%s/subscription-data/%s/context-data/sdm-subscriptions/%s",
		udrSelf.GetIPv4GroupUri(context.NUDR_DR), ueId, newSubscriptionID)

	return locationHeader, SdmSubscription
}

func QuerysdmsubscriptionsProcedure(ueId string) (*[]models.SdmSubscription, *models.ProblemDetails) {
	udrSelf := context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*context.UESubsData)
	var sdmSubscriptionSlice []models.SdmSubscription

	for _, v := range UESubsData.SdmSubscriptions {
		sdmSubscriptionSlice = append(sdmSubscriptionSlice, *v)
	}
	return &sdmSubscriptionSlice, nil
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
	dbSessionManagementData, err := queries.ListSmData(ueId)
	if err != nil {
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
	dbSmfSelectionSubscriptionData, err := queries.GetSmfSelectionSubscriptionData(ueId)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	if dbSmfSelectionSubscriptionData == nil {
		return nil, fmt.Errorf("USER_NOT_FOUND")
	}
	smfSelectionSubscriptionData := convertDbSmfSelectionDataToModel(dbSmfSelectionSubscriptionData)
	return smfSelectionSubscriptionData, nil
}

func PostSubscriptionDataSubscriptionsProcedure(
	SubscriptionDataSubscriptions models.SubscriptionDataSubscriptions,
) string {
	udrSelf := context.UDR_Self()

	newSubscriptionID := strconv.Itoa(udrSelf.SubscriptionDataSubscriptionIDGenerator)
	udrSelf.SubscriptionDataSubscriptions[newSubscriptionID] = &SubscriptionDataSubscriptions
	udrSelf.SubscriptionDataSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/subs-to-notify/{subsId} */
	locationHeader := fmt.Sprintf("%s/subscription-data/subs-to-notify/%s",
		udrSelf.GetIPv4GroupUri(context.NUDR_DR), newSubscriptionID)

	return locationHeader
}

func RemovesubscriptionDataSubscriptionsProcedure(subsId string) *models.ProblemDetails {
	udrSelf := context.UDR_Self()
	_, ok := udrSelf.SubscriptionDataSubscriptions[subsId]
	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(udrSelf.SubscriptionDataSubscriptions, subsId)
	return nil
}
