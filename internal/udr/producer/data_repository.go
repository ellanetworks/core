package producer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/mitchellh/mapstructure"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/db"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/logger"
	"github.com/yeastengine/ella/internal/udr/util"
	"go.mongodb.org/mongo-driver/bson"
)

var CurrentResourceUri string

func HandleCreateAccessAndMobilityData(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleDeleteAccessAndMobilityData(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleQueryAccessAndMobilityData(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleQueryAmData(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QueryAmData")

	ueId := request.Params["ueId"]
	servingPlmnId := request.Params["servingPlmnId"]
	response, problemDetails := QueryAmDataProcedure(ueId, servingPlmnId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func QueryAmDataProcedure(ueId string, servingPlmnId string) (*map[string]interface{},
	*models.ProblemDetails,
) {
	filter := bson.M{"ueId": ueId, "servingPlmnId": servingPlmnId}
	accessAndMobilitySubscriptionData, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.AmDataColl, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}
	if accessAndMobilitySubscriptionData != nil {
		return &accessAndMobilitySubscriptionData, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandleAmfContext3gpp(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle AmfContext3gpp")
	patchItem := request.Body.([]models.PatchItem)
	ueId := request.Params["ueId"]

	problemDetails := AmfContext3gppProcedure(ueId, patchItem)
	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func AmfContext3gppProcedure(ueId string, patchItem []models.PatchItem) *models.ProblemDetails {
	filter := bson.M{"ueId": ueId}
	origValue, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	patchJSON, err := json.Marshal(patchItem)
	if err != nil {
		logger.DataRepoLog.Error(err)
	}
	failure := db.CommonDBClient.RestfulAPIJSONPatch(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter, patchJSON)

	if failure == nil {
		newValue, errGetOneNew := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter)
		if errGetOneNew != nil {
			logger.DataRepoLog.Warnln(errGetOneNew)
		}
		PreHandleOnDataChangeNotify(ueId, CurrentResourceUri, patchItem, origValue, newValue)
		return nil
	} else {
		return util.ProblemDetailsModifyNotAllowed("")
	}
}

func HandleCreateAmfContext3gpp(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle CreateAmfContext3gpp")

	Amf3GppAccessRegistration := request.Body.(models.Amf3GppAccessRegistration)
	ueId := request.Params["ueId"]

	CreateAmfContext3gppProcedure(ueId, Amf3GppAccessRegistration)

	return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
}

func CreateAmfContext3gppProcedure(ueId string,
	Amf3GppAccessRegistration models.Amf3GppAccessRegistration,
) {
	filter := bson.M{"ueId": ueId}
	putData := util.ToBsonM(Amf3GppAccessRegistration)
	putData["ueId"] = ueId

	_, errPutOne := db.CommonDBClient.RestfulAPIPutOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter, putData)
	if errPutOne != nil {
		logger.DataRepoLog.Warnln(errPutOne)
	}
}

func HandleQueryAmfContext3gpp(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QueryAmfContext3gpp")

	ueId := request.Params["ueId"]

	response, problemDetails := QueryAmfContext3gppProcedure(ueId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QueryAmfContext3gppProcedure(ueId string) (*map[string]interface{}, *models.ProblemDetails) {
	filter := bson.M{"ueId": ueId}
	amf3GppAccessRegistration, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_CTXDATA_AMF_3GPPACCESS, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	if amf3GppAccessRegistration != nil {
		return &amf3GppAccessRegistration, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandleModifyAuthentication(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle ModifyAuthentication")

	ueId := request.Params["ueId"]
	patchItem := request.Body.([]models.PatchItem)

	problemDetails := ModifyAuthenticationProcedure(ueId, patchItem)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func ModifyAuthenticationProcedure(ueId string, patchItem []models.PatchItem) *models.ProblemDetails {
	filter := bson.M{"ueId": ueId}
	origValue, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.AuthSubsDataColl, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	patchJSON, err := json.Marshal(patchItem)
	if err != nil {
		logger.DataRepoLog.Error(err)
	}
	failure := db.CommonDBClient.RestfulAPIJSONPatch(db.AuthSubsDataColl, filter, patchJSON)

	if failure == nil {
		newValue, errGetOneNew := db.CommonDBClient.RestfulAPIGetOne(db.AuthSubsDataColl, filter)
		if errGetOneNew != nil {
			logger.DataRepoLog.Warnln(errGetOneNew)
		}
		PreHandleOnDataChangeNotify(ueId, CurrentResourceUri, patchItem, origValue, newValue)
		return nil
	} else {
		return util.ProblemDetailsModifyNotAllowed("")
	}
}

func HandleQueryAuthSubsData(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QueryAuthSubsData")

	ueId := request.Params["ueId"]

	response, problemDetails := QueryAuthSubsDataProcedure(ueId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QueryAuthSubsDataProcedure(ueId string) (map[string]interface{}, *models.ProblemDetails) {
	filter := bson.M{"ueId": ueId}

	authenticationSubscription, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.AuthSubsDataColl, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	if authenticationSubscription != nil {
		return authenticationSubscription, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandleCreateAuthenticationStatus(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle CreateAuthenticationStatus")

	putData := util.ToBsonM(request.Body)
	ueId := request.Params["ueId"]

	CreateAuthenticationStatusProcedure(ueId, putData)

	return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
}

func CreateAuthenticationStatusProcedure(ueId string, putData bson.M) {
	filter := bson.M{"ueId": ueId}
	putData["ueId"] = ueId

	_, errPutOne := db.CommonDBClient.RestfulAPIPutOne(db.SUBSCDATA_AUT_AUTHSTATUS, filter, putData)
	if errPutOne != nil {
		logger.DataRepoLog.Warnln(errPutOne)
	}
}

func HandleQueryAuthenticationStatus(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QueryAuthenticationStatus")

	ueId := request.Params["ueId"]

	response, problemDetails := QueryAuthenticationStatusProcedure(ueId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QueryAuthenticationStatusProcedure(ueId string) (*map[string]interface{},
	*models.ProblemDetails,
) {
	filter := bson.M{"ueId": ueId}

	authEvent, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SUBSCDATA_AUT_AUTHSTATUS, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	if authEvent != nil {
		return &authEvent, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandleExposureDataSubsToNotifyPost(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleExposureDataSubsToNotifySubIdDelete(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleExposureDataSubsToNotifySubIdPut(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandlePolicyDataSubsToNotifyPost(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle PolicyDataSubsToNotifyPost")

	PolicyDataSubscription := request.Body.(models.PolicyDataSubscription)

	locationHeader := PolicyDataSubsToNotifyPostProcedure(PolicyDataSubscription)

	headers := http.Header{}
	headers.Set("Location", locationHeader)
	return httpwrapper.NewResponse(http.StatusCreated, headers, PolicyDataSubscription)
}

func PolicyDataSubsToNotifyPostProcedure(PolicyDataSubscription models.PolicyDataSubscription) string {
	udrSelf := udr_context.UDR_Self()

	newSubscriptionID := strconv.Itoa(udrSelf.PolicyDataSubscriptionIDGenerator)
	udrSelf.PolicyDataSubscriptions[newSubscriptionID] = &PolicyDataSubscription
	udrSelf.PolicyDataSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/subs-to-notify/{subsId} */
	locationHeader := fmt.Sprintf("%s/policy-data/subs-to-notify/%s", udrSelf.GetIPv4GroupUri(udr_context.NUDR_DR),
		newSubscriptionID)

	return locationHeader
}

func HandlePolicyDataSubsToNotifySubsIdDelete(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle PolicyDataSubsToNotifySubsIdDelete")

	subsId := request.Params["subsId"]

	problemDetails := PolicyDataSubsToNotifySubsIdDeleteProcedure(subsId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func PolicyDataSubsToNotifySubsIdDeleteProcedure(subsId string) (problemDetails *models.ProblemDetails) {
	udrSelf := udr_context.UDR_Self()
	_, ok := udrSelf.PolicyDataSubscriptions[subsId]
	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(udrSelf.PolicyDataSubscriptions, subsId)

	return nil
}

func HandlePolicyDataSubsToNotifySubsIdPut(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle PolicyDataSubsToNotifySubsIdPut")

	subsId := request.Params["subsId"]
	policyDataSubscription := request.Body.(models.PolicyDataSubscription)

	response, problemDetails := PolicyDataSubsToNotifySubsIdPutProcedure(subsId, policyDataSubscription)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func PolicyDataSubsToNotifySubsIdPutProcedure(subsId string,
	policyDataSubscription models.PolicyDataSubscription,
) (*models.PolicyDataSubscription, *models.ProblemDetails) {
	udrSelf := udr_context.UDR_Self()
	_, ok := udrSelf.PolicyDataSubscriptions[subsId]
	if !ok {
		return nil, util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	udrSelf.PolicyDataSubscriptions[subsId] = &policyDataSubscription

	return &policyDataSubscription, nil
}

func HandlePolicyDataUesUeIdAmDataGet(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle PolicyDataUesUeIdAmDataGet")

	ueId := request.Params["ueId"]

	response, problemDetails := PolicyDataUesUeIdAmDataGetProcedure(ueId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func PolicyDataUesUeIdAmDataGetProcedure(ueId string) (*map[string]interface{}, *models.ProblemDetails) {
	filter := bson.M{"ueId": ueId}

	amPolicyData, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.AmPolicyDataColl, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	if amPolicyData != nil {
		return &amPolicyData, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandlePolicyDataUesUeIdSmDataGet(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle PolicyDataUesUeIdSmDataGet")
	ueId := request.Params["ueId"]
	sNssai := models.Snssai{}
	sNssaiQuery := request.Query.Get("snssai")
	err := json.Unmarshal([]byte(sNssaiQuery), &sNssai)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}
	dnn := request.Query.Get("dnn")

	response, problemDetails := PolicyDataUesUeIdSmDataGetProcedure(ueId, sNssai, dnn)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func PolicyDataUesUeIdSmDataGetProcedure(ueId string, snssai models.Snssai,
	dnn string,
) (*models.SmPolicyData, *models.ProblemDetails) {
	filter := bson.M{"ueId": ueId}

	if !reflect.DeepEqual(snssai, models.Snssai{}) {
		filter["smPolicySnssaiData."+util.SnssaiModelsToHex(snssai)] = bson.M{"$exists": true}
	}
	if !reflect.DeepEqual(snssai, models.Snssai{}) && dnn != "" {
		filter["smPolicySnssaiData."+util.SnssaiModelsToHex(snssai)+".smPolicyDnnData."+dnn] = bson.M{"$exists": true}
	}

	smPolicyData, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SmPolicyDataColl, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}
	if smPolicyData != nil {
		var smPolicyDataResp models.SmPolicyData
		err := json.Unmarshal(util.MapToByte(smPolicyData), &smPolicyDataResp)
		if err != nil {
			logger.DataRepoLog.Warnln(err)
		}
		return &smPolicyDataResp, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandleCreateAMFSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle CreateAMFSubscriptions")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]
	AmfSubscriptionInfo := request.Body.([]models.AmfSubscriptionInfo)

	problemDetails := CreateAMFSubscriptionsProcedure(subsId, ueId, AmfSubscriptionInfo)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func CreateAMFSubscriptionsProcedure(subsId string, ueId string,
	AmfSubscriptionInfo []models.AmfSubscriptionInfo,
) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
	UESubsData := value.(*udr_context.UESubsData)

	_, ok = UESubsData.EeSubscriptionCollection[subsId]
	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos = AmfSubscriptionInfo
	return nil
}

func HandleRemoveAmfSubscriptionsInfo(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle RemoveAmfSubscriptionsInfo")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]

	problemDetails := RemoveAmfSubscriptionsInfoProcedure(subsId, ueId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func RemoveAmfSubscriptionsInfoProcedure(subsId string, ueId string) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
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

func HandleModifyAmfSubscriptionInfo(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle ModifyAmfSubscriptionInfo")

	patchItem := request.Body.([]models.PatchItem)
	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]

	problemDetails := ModifyAmfSubscriptionInfoProcedure(ueId, subsId, patchItem)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func ModifyAmfSubscriptionInfoProcedure(ueId string, subsId string,
	patchItem []models.PatchItem,
) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
	UESubsData := value.(*udr_context.UESubsData)

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

func HandleGetAmfSubscriptionInfo(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle GetAmfSubscriptionInfo")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]

	response, problemDetails := GetAmfSubscriptionInfoProcedure(subsId, ueId)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func GetAmfSubscriptionInfoProcedure(subsId string, ueId string) (*[]models.AmfSubscriptionInfo,
	*models.ProblemDetails,
) {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return nil, util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}

	if UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos == nil {
		return nil, util.ProblemDetailsNotFound("AMFSUBSCRIPTION_NOT_FOUND")
	}
	return &UESubsData.EeSubscriptionCollection[subsId].AmfSubscriptionInfos, nil
}

func HandleRemoveEeGroupSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle RemoveEeGroupSubscriptions")

	ueGroupId := request.Params["ueGroupId"]
	subsId := request.Params["subsId"]

	problemDetails := RemoveEeGroupSubscriptionsProcedure(ueGroupId, subsId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func RemoveEeGroupSubscriptionsProcedure(ueGroupId string, subsId string) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UEGroupSubsData := value.(*udr_context.UEGroupSubsData)
	_, ok = UEGroupSubsData.EeSubscriptions[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(UEGroupSubsData.EeSubscriptions, subsId)

	return nil
}

func HandleUpdateEeGroupSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle UpdateEeGroupSubscriptions")

	ueGroupId := request.Params["ueGroupId"]
	subsId := request.Params["subsId"]
	EeSubscription := request.Body.(models.EeSubscription)

	problemDetails := UpdateEeGroupSubscriptionsProcedure(ueGroupId, subsId, EeSubscription)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func UpdateEeGroupSubscriptionsProcedure(ueGroupId string, subsId string,
	EeSubscription models.EeSubscription,
) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UEGroupSubsData := value.(*udr_context.UEGroupSubsData)
	_, ok = UEGroupSubsData.EeSubscriptions[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	UEGroupSubsData.EeSubscriptions[subsId] = &EeSubscription

	return nil
}

func HandleCreateEeGroupSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle CreateEeGroupSubscriptions")

	ueGroupId := request.Params["ueGroupId"]
	EeSubscription := request.Body.(models.EeSubscription)

	locationHeader := CreateEeGroupSubscriptionsProcedure(ueGroupId, EeSubscription)

	headers := http.Header{}
	headers.Set("Location", locationHeader)
	return httpwrapper.NewResponse(http.StatusCreated, headers, EeSubscription)
}

func CreateEeGroupSubscriptionsProcedure(ueGroupId string, EeSubscription models.EeSubscription) string {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		udrSelf.UEGroupCollection.Store(ueGroupId, new(udr_context.UEGroupSubsData))
		value, _ = udrSelf.UEGroupCollection.Load(ueGroupId)
	}
	UEGroupSubsData := value.(*udr_context.UEGroupSubsData)
	if UEGroupSubsData.EeSubscriptions == nil {
		UEGroupSubsData.EeSubscriptions = make(map[string]*models.EeSubscription)
	}

	newSubscriptionID := strconv.Itoa(udrSelf.EeSubscriptionIDGenerator)
	UEGroupSubsData.EeSubscriptions[newSubscriptionID] = &EeSubscription
	udrSelf.EeSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/nudr-dr/v1/subscription-data/group-data/{ueGroupId}/ee-subscriptions */
	locationHeader := fmt.Sprintf("%s/nudr-dr/v1/subscription-data/group-data/%s/ee-subscriptions/%s",
		udrSelf.GetIPv4GroupUri(udr_context.NUDR_DR), ueGroupId, newSubscriptionID)

	return locationHeader
}

func HandleQueryEeGroupSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QueryEeGroupSubscriptions")

	ueGroupId := request.Params["ueGroupId"]

	response, problemDetails := QueryEeGroupSubscriptionsProcedure(ueGroupId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QueryEeGroupSubscriptionsProcedure(ueGroupId string) ([]models.EeSubscription, *models.ProblemDetails) {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UEGroupCollection.Load(ueGroupId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UEGroupSubsData := value.(*udr_context.UEGroupSubsData)
	var eeSubscriptionSlice []models.EeSubscription

	for _, v := range UEGroupSubsData.EeSubscriptions {
		eeSubscriptionSlice = append(eeSubscriptionSlice, *v)
	}
	return eeSubscriptionSlice, nil
}

func HandleRemoveeeSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle RemoveeeSubscriptions")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]

	problemDetails := RemoveeeSubscriptionsProcedure(ueId, subsId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func RemoveeeSubscriptionsProcedure(ueId string, subsId string) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(UESubsData.EeSubscriptionCollection, subsId)
	return nil
}

func HandleUpdateEesubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle UpdateEesubscriptions")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]
	EeSubscription := request.Body.(models.EeSubscription)

	problemDetails := UpdateEesubscriptionsProcedure(ueId, subsId, EeSubscription)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func UpdateEesubscriptionsProcedure(ueId string, subsId string,
	EeSubscription models.EeSubscription,
) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	_, ok = UESubsData.EeSubscriptionCollection[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	UESubsData.EeSubscriptionCollection[subsId].EeSubscriptions = &EeSubscription

	return nil
}

func HandleCreateEeSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle CreateEeSubscriptions")

	ueId := request.Params["ueId"]
	EeSubscription := request.Body.(models.EeSubscription)

	locationHeader := CreateEeSubscriptionsProcedure(ueId, EeSubscription)

	headers := http.Header{}
	headers.Set("Location", locationHeader)
	return httpwrapper.NewResponse(http.StatusCreated, headers, EeSubscription)
}

func CreateEeSubscriptionsProcedure(ueId string, EeSubscription models.EeSubscription) string {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		udrSelf.UESubsCollection.Store(ueId, new(udr_context.UESubsData))
		value, _ = udrSelf.UESubsCollection.Load(ueId)
	}
	UESubsData := value.(*udr_context.UESubsData)
	if UESubsData.EeSubscriptionCollection == nil {
		UESubsData.EeSubscriptionCollection = make(map[string]*udr_context.EeSubscriptionCollection)
	}

	newSubscriptionID := strconv.Itoa(udrSelf.EeSubscriptionIDGenerator)
	UESubsData.EeSubscriptionCollection[newSubscriptionID] = new(udr_context.EeSubscriptionCollection)
	UESubsData.EeSubscriptionCollection[newSubscriptionID].EeSubscriptions = &EeSubscription
	udrSelf.EeSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/{ueId}/context-data/ee-subscriptions/{subsId} */
	locationHeader := fmt.Sprintf("%s/subscription-data/%s/context-data/ee-subscriptions/%s",
		udrSelf.GetIPv4GroupUri(udr_context.NUDR_DR), ueId, newSubscriptionID)

	return locationHeader
}

func HandleQueryeesubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle Queryeesubscriptions")

	ueId := request.Params["ueId"]

	response, problemDetails := QueryeesubscriptionsProcedure(ueId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QueryeesubscriptionsProcedure(ueId string) ([]models.EeSubscription, *models.ProblemDetails) {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	var eeSubscriptionSlice []models.EeSubscription

	for _, v := range UESubsData.EeSubscriptionCollection {
		eeSubscriptionSlice = append(eeSubscriptionSlice, *v.EeSubscriptions)
	}
	return eeSubscriptionSlice, nil
}

func HandleCreateSessionManagementData(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleDeleteSessionManagementData(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleQuerySessionManagementData(request *httpwrapper.Request) *httpwrapper.Response {
	return httpwrapper.NewResponse(http.StatusOK, nil, map[string]interface{}{})
}

func HandleQueryProvisionedData(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QueryProvisionedData")

	var provisionedDataSets models.ProvisionedDataSets
	ueId := request.Params["ueId"]
	servingPlmnId := request.Params["servingPlmnId"]

	response, problemDetails := QueryProvisionedDataProcedure(ueId, servingPlmnId, provisionedDataSets)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QueryProvisionedDataProcedure(ueId string, servingPlmnId string,
	provisionedDataSets models.ProvisionedDataSets,
) (*models.ProvisionedDataSets, *models.ProblemDetails) {
	{
		filter := bson.M{"ueId": ueId, "servingPlmnId": servingPlmnId}
		accessAndMobilitySubscriptionData, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.AmDataColl, filter)
		if errGetOne != nil {
			logger.DataRepoLog.Warnln(errGetOne)
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
		filter := bson.M{"ueId": ueId, "servingPlmnId": servingPlmnId}
		smfSelectionSubscriptionData, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SmfSelDataColl, filter)
		if errGetOne != nil {
			logger.DataRepoLog.Warnln(errGetOne)
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
		filter := bson.M{"ueId": ueId, "servingPlmnId": servingPlmnId}
		sessionManagementSubscriptionDatas, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.SmDataColl, filter)
		if errGetMany != nil {
			logger.DataRepoLog.Warnln(errGetMany)
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

func HandleRemovesdmSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle RemovesdmSubscriptions")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]

	problemDetails := RemovesdmSubscriptionsProcedure(ueId, subsId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func RemovesdmSubscriptionsProcedure(ueId string, subsId string) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	_, ok = UESubsData.SdmSubscriptions[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(UESubsData.SdmSubscriptions, subsId)

	return nil
}

func HandleUpdatesdmsubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle Updatesdmsubscriptions")

	ueId := request.Params["ueId"]
	subsId := request.Params["subsId"]
	SdmSubscription := request.Body.(models.SdmSubscription)

	problemDetails := UpdatesdmsubscriptionsProcedure(ueId, subsId, SdmSubscription)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func UpdatesdmsubscriptionsProcedure(ueId string, subsId string,
	SdmSubscription models.SdmSubscription,
) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	_, ok = UESubsData.SdmSubscriptions[subsId]

	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	SdmSubscription.SubscriptionId = subsId
	UESubsData.SdmSubscriptions[subsId] = &SdmSubscription

	return nil
}

func HandleCreateSdmSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle CreateSdmSubscriptions")

	SdmSubscription := request.Body.(models.SdmSubscription)
	ueId := request.Params["ueId"]

	locationHeader, SdmSubscription := CreateSdmSubscriptionsProcedure(SdmSubscription, ueId)

	headers := http.Header{}
	headers.Set("Location", locationHeader)
	return httpwrapper.NewResponse(http.StatusCreated, headers, SdmSubscription)
}

func CreateSdmSubscriptionsProcedure(SdmSubscription models.SdmSubscription,
	ueId string,
) (string, models.SdmSubscription) {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		udrSelf.UESubsCollection.Store(ueId, new(udr_context.UESubsData))
		value, _ = udrSelf.UESubsCollection.Load(ueId)
	}
	UESubsData := value.(*udr_context.UESubsData)
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
		udrSelf.GetIPv4GroupUri(udr_context.NUDR_DR), ueId, newSubscriptionID)

	return locationHeader, SdmSubscription
}

func HandleQuerysdmsubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle Querysdmsubscriptions")

	ueId := request.Params["ueId"]

	response, problemDetails := QuerysdmsubscriptionsProcedure(ueId)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	pd := util.ProblemDetailsUpspecified("")
	return httpwrapper.NewResponse(int(pd.Status), nil, pd)
}

func QuerysdmsubscriptionsProcedure(ueId string) (*[]models.SdmSubscription, *models.ProblemDetails) {
	udrSelf := udr_context.UDR_Self()

	value, ok := udrSelf.UESubsCollection.Load(ueId)
	if !ok {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}

	UESubsData := value.(*udr_context.UESubsData)
	var sdmSubscriptionSlice []models.SdmSubscription

	for _, v := range UESubsData.SdmSubscriptions {
		sdmSubscriptionSlice = append(sdmSubscriptionSlice, *v)
	}
	return &sdmSubscriptionSlice, nil
}

func HandleQuerySmData(request *httpwrapper.Request) *httpwrapper.Response {
	ueId := request.Params["ueId"]
	servingPlmnId := request.Params["servingPlmnId"]
	singleNssai := models.Snssai{}
	singleNssaiQuery := request.Query.Get("single-nssai")
	err := json.Unmarshal([]byte(singleNssaiQuery), &singleNssai)
	if err != nil {
		logger.DataRepoLog.Warnln(err)
	}

	dnn := request.Query.Get("dnn")
	response := QuerySmDataProcedure(ueId, servingPlmnId, singleNssai, dnn)

	return httpwrapper.NewResponse(http.StatusOK, nil, response)
}

func QuerySmDataProcedure(ueId string, servingPlmnId string,
	singleNssai models.Snssai, dnn string,
) *[]map[string]interface{} {
	filter := bson.M{"ueId": ueId, "servingPlmnId": servingPlmnId}

	if !reflect.DeepEqual(singleNssai, models.Snssai{}) {
		if singleNssai.Sd == "" {
			filter["singleNssai.sst"] = singleNssai.Sst
		} else {
			filter["singleNssai.sst"] = singleNssai.Sst
			filter["singleNssai.sd"] = singleNssai.Sd
		}
	}

	if dnn != "" {
		filter["dnnConfigurations."+dnn] = bson.M{"$exists": true}
	}

	sessionManagementSubscriptionDatas, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.SmDataColl, filter)
	if errGetMany != nil {
		logger.DataRepoLog.Warnln(errGetMany)
	}

	return &sessionManagementSubscriptionDatas
}

func HandleQuerySmfSelectData(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle QuerySmfSelectData")

	ueId := request.Params["ueId"]
	servingPlmnId := request.Params["servingPlmnId"]
	response, problemDetails := QuerySmfSelectDataProcedure(ueId, servingPlmnId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func QuerySmfSelectDataProcedure(ueId string, servingPlmnId string) (*map[string]interface{}, *models.ProblemDetails) {
	filter := bson.M{"ueId": ueId, "servingPlmnId": servingPlmnId}
	smfSelectionSubscriptionData, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SmfSelDataColl, filter)
	if errGetOne != nil {
		logger.DataRepoLog.Warnln(errGetOne)
	}

	if smfSelectionSubscriptionData != nil {
		return &smfSelectionSubscriptionData, nil
	} else {
		return nil, util.ProblemDetailsNotFound("USER_NOT_FOUND")
	}
}

func HandlePostSubscriptionDataSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle PostSubscriptionDataSubscriptions")

	SubscriptionDataSubscriptions := request.Body.(models.SubscriptionDataSubscriptions)

	locationHeader := PostSubscriptionDataSubscriptionsProcedure(SubscriptionDataSubscriptions)

	headers := http.Header{}
	headers.Set("Location", locationHeader)
	return httpwrapper.NewResponse(http.StatusCreated, headers, SubscriptionDataSubscriptions)
}

func PostSubscriptionDataSubscriptionsProcedure(
	SubscriptionDataSubscriptions models.SubscriptionDataSubscriptions,
) string {
	udrSelf := udr_context.UDR_Self()

	newSubscriptionID := strconv.Itoa(udrSelf.SubscriptionDataSubscriptionIDGenerator)
	udrSelf.SubscriptionDataSubscriptions[newSubscriptionID] = &SubscriptionDataSubscriptions
	udrSelf.SubscriptionDataSubscriptionIDGenerator++

	/* Contains the URI of the newly created resource, according
	   to the structure: {apiRoot}/subscription-data/subs-to-notify/{subsId} */
	locationHeader := fmt.Sprintf("%s/subscription-data/subs-to-notify/%s",
		udrSelf.GetIPv4GroupUri(udr_context.NUDR_DR), newSubscriptionID)

	return locationHeader
}

func HandleRemovesubscriptionDataSubscriptions(request *httpwrapper.Request) *httpwrapper.Response {
	logger.DataRepoLog.Infof("Handle RemovesubscriptionDataSubscriptions")

	subsId := request.Params["subsId"]

	problemDetails := RemovesubscriptionDataSubscriptionsProcedure(subsId)

	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, map[string]interface{}{})
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func RemovesubscriptionDataSubscriptionsProcedure(subsId string) *models.ProblemDetails {
	udrSelf := udr_context.UDR_Self()
	_, ok := udrSelf.SubscriptionDataSubscriptions[subsId]
	if !ok {
		return util.ProblemDetailsNotFound("SUBSCRIPTION_NOT_FOUND")
	}
	delete(udrSelf.SubscriptionDataSubscriptions, subsId)
	return nil
}
