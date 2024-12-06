package producer

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/antihax/optional"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	Nudr "github.com/omec-project/openapi/Nudr_DataRepository"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	udm_context "github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udr/producer"
)

func createUDMClientToUDR() *Nudr.APIClient {
	uri := udm_context.UDM_Self().UdrUri
	cfg := Nudr.NewConfiguration()
	cfg.SetBasePath(uri)
	clientAPI := Nudr.NewAPIClient(cfg)
	return clientAPI
}

func HandleGetAmDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetAmData")
	supi := request.Params["supi"]
	response, problemDetails := getAmDataProcedure(supi)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func getAmDataProcedure(supi string) (
	response *models.AccessAndMobilitySubscriptionData, problemDetails *models.ProblemDetails,
) {
	amData, err := producer.GetAmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetAmData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func HandleGetSupiRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.SdmLog.Infof("Handle GetSupiRequest")

	// step 2: retrieve request
	supi := request.Params["supi"]
	supportedFeatures := request.Query.Get("supported-features")

	// step 3: handle the message
	response, problemDetails := getSupiProcedure(supi, supportedFeatures)

	// step 4: process the return value from step 3
	if response != nil {
		// status code is based on SPEC, and option headers
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func getSupiProcedure(supi string, supportedFeatures string) (
	response *models.SubscriptionDataSets, problemDetails *models.ProblemDetails,
) {
	var subscriptionDataSets, subsDataSetBody models.SubscriptionDataSets
	var ueContextInSmfDataResp models.UeContextInSmfData
	pduSessionMap := make(map[string]models.PduSession)
	var pgwInfoArray []models.PgwInfo

	udm_context.UDM_Self().CreateSubsDataSetsForUe(supi, subsDataSetBody)

	amData, err := producer.GetAmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetAmData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	subscriptionDataSets.AmData = amData

	smfSelData, err := producer.GetSmfSelectData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmfSelectData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	udmUe = udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelData)
	subscriptionDataSets.SmfSelData = smfSelData

	sessionManagementSubscriptionData, err := producer.GetSmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	udmUe = udm_context.UDM_Self().NewUdmUe(supi)
	smData, _, _, _ := udm_context.UDM_Self().ManageSmData(sessionManagementSubscriptionData, "", "")
	udmUe.SetSMSubsData(smData)
	subscriptionDataSets.SmData = sessionManagementSubscriptionData

	clientAPI := createUDMClientToUDR()
	var UeContextInSmfbody models.UeContextInSmfData
	var querySmfRegListParamOpts Nudr.QuerySmfRegListParamOpts
	querySmfRegListParamOpts.SupportedFeatures = optional.NewString(supportedFeatures)
	udm_context.UDM_Self().CreateUeContextInSmfDataforUe(supi, UeContextInSmfbody)
	pdusess, res, err := clientAPI.SMFRegistrationsCollectionApi.QuerySmfRegList(
		context.Background(), supi, &querySmfRegListParamOpts)
	if err != nil {
		if res == nil {
			fmt.Println(err.Error())
		} else if err.Error() != res.Status {
			fmt.Println(err.Error())
		} else {
			problemDetails = &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}

			return nil, problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("QuerySmfRegList response body cannot close: %+v", rspCloseErr)
		}
	}()

	for _, element := range pdusess {
		var pduSession models.PduSession
		pduSession.Dnn = element.Dnn
		pduSession.SmfInstanceId = element.SmfInstanceId
		pduSession.PlmnId = element.PlmnId
		pduSessionMap[strconv.Itoa(int(element.PduSessionId))] = pduSession
	}
	ueContextInSmfDataResp.PduSessions = pduSessionMap

	for _, element := range pdusess {
		var pgwInfo models.PgwInfo
		pgwInfo.Dnn = element.Dnn
		pgwInfo.PgwFqdn = element.PgwFqdn
		pgwInfo.PlmnId = element.PlmnId
		pgwInfoArray = append(pgwInfoArray, pgwInfo)
	}
	ueContextInSmfDataResp.PgwInfo = pgwInfoArray

	if res.StatusCode == http.StatusOK {
		udmUe := udm_context.UDM_Self().NewUdmUe(supi)
		udmUe.UeCtxtInSmfData = &ueContextInSmfDataResp
	} else {
		var problemDetails models.ProblemDetails
		problemDetails.Cause = "DATA_NOT_FOUND"
		fmt.Printf(problemDetails.Cause)
	}

	if (res.StatusCode == http.StatusOK) && (amData != nil) &&
		(smfSelData != nil) &&
		(sessionManagementSubscriptionData != nil) {
		subscriptionDataSets.UecSmfData = &ueContextInSmfDataResp
		return &subscriptionDataSets, nil
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
		}

		return nil, problemDetails
	}
}

func HandleGetSmDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	supi := request.Params["supi"]
	Dnn := request.Query.Get("dnn")
	Snssai := request.Query.Get("single-nssai")

	response, problemDetails := getSmDataProcedure(supi, Dnn, Snssai)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func getSmDataProcedure(supi string, Dnn string, Snssai string) (
	response interface{}, problemDetails *models.ProblemDetails,
) {
	sessionManagementSubscriptionDataResp, err := producer.GetSmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}

	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	smData, snssaikey, AllDnnConfigsbyDnn, AllDnns := udm_context.UDM_Self().ManageSmData(
		sessionManagementSubscriptionDataResp, Snssai, Dnn)
	udmUe.SetSMSubsData(smData)

	rspSMSubDataList := make([]models.SessionManagementSubscriptionData, 0, 4)

	udmUe.SmSubsDataLock.RLock()
	for _, eachSMSubData := range udmUe.SessionManagementSubsData {
		rspSMSubDataList = append(rspSMSubDataList, eachSMSubData)
	}
	udmUe.SmSubsDataLock.RUnlock()

	switch {
	case Snssai == "" && Dnn == "":
		return AllDnns, nil
	case Snssai != "" && Dnn == "":
		udmUe.SmSubsDataLock.RLock()
		defer udmUe.SmSubsDataLock.RUnlock()
		return udmUe.SessionManagementSubsData[snssaikey].DnnConfigurations, nil
	case Snssai == "" && Dnn != "":
		return AllDnnConfigsbyDnn, nil
	case Snssai != "" && Dnn != "":
		return rspSMSubDataList, nil
	default:
		udmUe.SmSubsDataLock.RLock()
		defer udmUe.SmSubsDataLock.RUnlock()
		return udmUe.SessionManagementSubsData, nil
	}
}

func HandleGetNssaiRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetNssai")
	supi := request.Params["supi"]
	response, problemDetails := getNssaiProcedure(supi)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func getNssaiProcedure(supi string) (
	*models.Nssai, *models.ProblemDetails,
) {
	accessAndMobilitySubscriptionDataResp, err := producer.GetAmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetAmData error: %+v", err)
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func HandleGetSmfSelectDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetSmfSelectData")
	supi := request.Params["supi"]
	response, problemDetails := getSmfSelectDataProcedure(supi)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func getSmfSelectDataProcedure(supi string) (
	response *models.SmfSelectionSubscriptionData, problemDetails *models.ProblemDetails,
) {
	var body models.SmfSelectionSubscriptionData
	udm_context.UDM_Self().CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := producer.GetSmfSelectData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmfSelectData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return nil, problemDetails
	}
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelectionSubscriptionDataResp)
	return udmUe.SmfSelSubsData, nil
}

func HandleSubscribeToSharedDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.SdmLog.Infof("Handle SubscribeToSharedData")

	// step 2: retrieve request
	sdmSubscription := request.Body.(models.SdmSubscription)

	// step 3: handle the message
	header, response, problemDetails := subscribeToSharedDataProcedure(&sdmSubscription)

	// step 4: process the return value from step 3
	if response != nil {
		// status code is based on SPEC, and option headers
		return httpwrapper.NewResponse(http.StatusCreated, header, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusNotFound, nil, nil)
	}
}

func subscribeToSharedDataProcedure(sdmSubscription *models.SdmSubscription) (
	header http.Header, response *models.SdmSubscription, problemDetails *models.ProblemDetails,
) {
	cfg := Nudm_SubscriberDataManagement.NewConfiguration()
	udmClientAPI := Nudm_SubscriberDataManagement.NewAPIClient(cfg)

	sdmSubscriptionResp, res, err := udmClientAPI.SubscriptionCreationForSharedDataApi.SubscribeToSharedData(
		context.Background(), *sdmSubscription)
	if err != nil {
		if res == nil {
			logger.SdmLog.Warnln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Warnln(err)
		} else {
			problemDetails = &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}
			return nil, nil, problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("SubscribeToSharedData response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res.StatusCode == http.StatusCreated {
		header = make(http.Header)
		udm_context.UDM_Self().CreateSubstoNotifSharedData(sdmSubscriptionResp.SubscriptionId, &sdmSubscriptionResp)
		reourceUri := udm_context.UDM_Self().GetSDMUri() + "//shared-data-subscriptions/" + sdmSubscriptionResp.SubscriptionId
		header.Set("Location", reourceUri)
		return header, &sdmSubscriptionResp, nil
	} else if res.StatusCode == http.StatusNotFound {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
		}

		return nil, nil, problemDetails
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotImplemented,
			Cause:  "UNSUPPORTED_RESOURCE_URI",
		}

		return nil, nil, problemDetails
	}
}

func HandleSubscribeRequest(request *httpwrapper.Request) *httpwrapper.Response {
	sdmSubscription := request.Body.(models.SdmSubscription)
	supi := request.Params["supi"]
	header, response, problemDetails := subscribeProcedure(&sdmSubscription, supi)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusCreated, header, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusNotFound, nil, nil)
	}
}

func subscribeProcedure(sdmSubscription *models.SdmSubscription, supi string) (
	header http.Header, response *models.SdmSubscription, problemDetails *models.ProblemDetails,
) {
	clientAPI := createUDMClientToUDR()

	sdmSubscriptionResp, res, err := clientAPI.SDMSubscriptionsCollectionApi.CreateSdmSubscriptions(
		context.Background(), supi, *sdmSubscription)
	if err != nil {
		if res == nil {
			logger.SdmLog.Warnln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Warnln(err)
		} else {
			logger.SdmLog.Warnln(err)
			problemDetails = &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}
			return nil, nil, problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("CreateSdmSubscriptions response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res.StatusCode == http.StatusCreated {
		header = make(http.Header)
		udmUe, _ := udm_context.UDM_Self().UdmUeFindBySupi(supi)
		if udmUe == nil {
			udmUe = udm_context.UDM_Self().NewUdmUe(supi)
		}
		udmUe.CreateSubscriptiontoNotifChange(sdmSubscriptionResp.SubscriptionId, &sdmSubscriptionResp)
		header.Set("Location", udmUe.GetLocationURI2(udm_context.LocationUriSdmSubscription, supi))
		return header, &sdmSubscriptionResp, nil
	} else if res.StatusCode == http.StatusNotFound {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
		}
		return nil, nil, problemDetails
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotImplemented,
			Cause:  "UNSUPPORTED_RESOURCE_URI",
		}
		return nil, nil, problemDetails
	}
}

func HandleUnsubscribeForSharedDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle UnsubscribeForSharedData")

	// step 2: retrieve request
	subscriptionID := request.Params["subscriptionId"]
	// step 3: handle the message
	problemDetails := unsubscribeForSharedDataProcedure(subscriptionID)

	// step 4: process the return value from step 3
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
}

func unsubscribeForSharedDataProcedure(subscriptionID string) *models.ProblemDetails {
	cfg := Nudm_SubscriberDataManagement.NewConfiguration()
	udmClientAPI := Nudm_SubscriberDataManagement.NewAPIClient(cfg)

	res, err := udmClientAPI.SubscriptionDeletionForSharedDataApi.UnsubscribeForSharedData(
		context.Background(), subscriptionID)
	if err != nil {
		if res == nil {
			logger.SdmLog.Warnln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Warnln(err)
		} else {
			logger.SdmLog.Warnln(err)
			problemDetails := &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}
			return problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("UnsubscribeForSharedData response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res.StatusCode == http.StatusNoContent {
		return nil
	} else {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
		}
		return problemDetails
	}
}

func HandleUnsubscribeRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle Unsubscribe")

	// step 2: retrieve request
	subscriptionID := request.Params["subscriptionId"]

	// step 3: handle the message
	problemDetails := unsubscribeProcedure(subscriptionID)

	// step 4: process the return value from step 3
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
}

func unsubscribeProcedure(subscriptionID string) *models.ProblemDetails {
	clientAPI := createUDMClientToUDR()

	res, err := clientAPI.SDMSubscriptionDocumentApi.RemovesdmSubscriptions(context.Background(), "====", subscriptionID)
	if err != nil {
		if res == nil {
			logger.SdmLog.Warnln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Warnln(err)
		} else {
			logger.SdmLog.Warnln(err)
			problemDetails := &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}
			return problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("RemovesdmSubscriptions response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res.StatusCode == http.StatusNoContent {
		return nil
	} else {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "USER_NOT_FOUND",
		}
		return problemDetails
	}
}

func HandleModifyRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.SdmLog.Infof("Handle Modify")

	// step 2: retrieve request
	supi := request.Params["supi"]
	subscriptionID := request.Params["subscriptionId"]

	// step 3: handle the message
	response, problemDetails := modifyProcedure(supi, subscriptionID)

	// step 4: process the return value from step 3
	if response != nil {
		// status code is based on SPEC, and option headers
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func modifyProcedure(supi string, subscriptionID string) (
	response *models.SdmSubscription, problemDetails *models.ProblemDetails,
) {
	clientAPI := createUDMClientToUDR()

	sdmSubscription := models.SdmSubscription{}
	body := Nudr.UpdatesdmsubscriptionsParamOpts{
		SdmSubscription: optional.NewInterface(sdmSubscription),
	}
	res, err := clientAPI.SDMSubscriptionDocumentApi.Updatesdmsubscriptions(
		context.Background(), supi, subscriptionID, &body)
	if err != nil {
		if res == nil {
			logger.SdmLog.Warnln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Warnln(err)
		} else {
			problemDetails = &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}
			return nil, problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("Updatesdmsubscriptions response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res.StatusCode == http.StatusOK {
		return &sdmSubscription, nil
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "USER_NOT_FOUND",
		}

		return nil, problemDetails
	}
}

func HandleModifyForSharedDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.SdmLog.Infof("Handle ModifyForSharedData")

	// step 2: retrieve request
	supi := request.Params["supi"]
	subscriptionID := request.Params["subscriptionId"]

	// step 3: handle the message
	response, problemDetails := modifyForSharedDataProcedure(supi, subscriptionID)

	// step 4: process the return value from step 3
	if response != nil {
		// status code is based on SPEC, and option headers
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func modifyForSharedDataProcedure(supi string, subscriptionID string) (response *models.SdmSubscription, problemDetails *models.ProblemDetails) {
	var sdmSubscription models.SdmSubscription
	sdmSubs := models.SdmSubscription{}
	body := Nudr.UpdatesdmsubscriptionsParamOpts{
		SdmSubscription: optional.NewInterface(sdmSubs),
	}
	clientAPI := createUDMClientToUDR()
	res, err := clientAPI.SDMSubscriptionDocumentApi.Updatesdmsubscriptions(
		context.Background(), supi, subscriptionID, &body)
	if err != nil {
		if res == nil {
			logger.SdmLog.Warnln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Warnln(err)
		} else {
			problemDetails = &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}
			return nil, problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("Updatesdmsubscriptions response body cannot close: %+v", rspCloseErr)
		}
	}()

	if res.StatusCode == http.StatusOK {
		return &sdmSubscription, nil
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "USER_NOT_FOUND",
		}

		return nil, problemDetails
	}
}

func HandleGetUeContextInSmfDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.SdmLog.Infof("Handle GetUeContextInSmfData")

	// step 2: retrieve request
	supi := request.Params["supi"]
	supportedFeatures := request.Query.Get("supported-features")

	// step 3: handle the message
	response, problemDetails := getUeContextInSmfDataProcedure(supi, supportedFeatures)

	// step 4: process the return value from step 3
	if response != nil {
		// status code is based on SPEC, and option headers
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func getUeContextInSmfDataProcedure(supi string, supportedFeatures string) (
	response *models.UeContextInSmfData, problemDetails *models.ProblemDetails,
) {
	var body models.UeContextInSmfData
	var ueContextInSmfData models.UeContextInSmfData
	var pgwInfoArray []models.PgwInfo
	var querySmfRegListParamOpts Nudr.QuerySmfRegListParamOpts
	querySmfRegListParamOpts.SupportedFeatures = optional.NewString(supportedFeatures)

	clientAPI := createUDMClientToUDR()
	pduSessionMap := make(map[string]models.PduSession)
	udm_context.UDM_Self().CreateUeContextInSmfDataforUe(supi, body)

	pdusess, res, err := clientAPI.SMFRegistrationsCollectionApi.QuerySmfRegList(
		context.Background(), supi, &querySmfRegListParamOpts)
	if err != nil {
		if res == nil {
			logger.SdmLog.Infoln(err)
		} else if err.Error() != res.Status {
			logger.SdmLog.Infoln(err)
		} else {
			logger.SdmLog.Infoln(err)
			problemDetails = &models.ProblemDetails{
				Status: int32(res.StatusCode),
				Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
				Detail: err.Error(),
			}

			return nil, problemDetails
		}
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("QuerySmfRegList response body cannot close: %+v", rspCloseErr)
		}
	}()

	for _, element := range pdusess {
		var pduSession models.PduSession
		pduSession.Dnn = element.Dnn
		pduSession.SmfInstanceId = element.SmfInstanceId
		pduSession.PlmnId = element.PlmnId
		pduSessionMap[strconv.Itoa(int(element.PduSessionId))] = pduSession
	}
	ueContextInSmfData.PduSessions = pduSessionMap

	for _, element := range pdusess {
		var pgwInfo models.PgwInfo
		pgwInfo.Dnn = element.Dnn
		pgwInfo.PgwFqdn = element.PgwFqdn
		pgwInfo.PlmnId = element.PlmnId
		pgwInfoArray = append(pgwInfoArray, pgwInfo)
	}
	ueContextInSmfData.PgwInfo = pgwInfoArray

	if res.StatusCode == http.StatusOK {
		udmUe := udm_context.UDM_Self().NewUdmUe(supi)
		udmUe.UeCtxtInSmfData = &ueContextInSmfData
		return udmUe.UeCtxtInSmfData, nil
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
		}
		return nil, problemDetails
	}
}
