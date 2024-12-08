package producer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	udm_context "github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udr/producer"
)

func HandleGetAmDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetAmData")
	supi := request.Params["supi"]
	response, err := GetAmData(supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusOK, nil, response)
}

func GetAmData(supi string) (
	*models.AccessAndMobilitySubscriptionData, error,
) {
	amData, err := producer.GetAmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetAmData error: %+v", err)
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)
	return amData, nil
}

func HandleGetSupiRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetSupiRequest")
	supi := request.Params["supi"]
	problemDetails := getSupiProcedure(supi)
	return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
}

func getSupiProcedure(supi string) (
	problemDetails *models.ProblemDetails,
) {
	var subsDataSetBody models.SubscriptionDataSets

	udm_context.UDM_Self().CreateSubsDataSetsForUe(supi, subsDataSetBody)

	amData, err := producer.GetAmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetAmData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return problemDetails
	}
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetAMSubsriptionData(amData)

	smfSelData, err := producer.GetSmfSelectData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmfSelectData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return problemDetails
	}
	udmUe = udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.SetSmfSelectionSubsData(smfSelData)

	sessionManagementSubscriptionData, err := producer.GetSmData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmData error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return problemDetails
	}
	udmUe = udm_context.UDM_Self().NewUdmUe(supi)
	smData, _, _, _ := udm_context.UDM_Self().ManageSmData(sessionManagementSubscriptionData, "", "")
	udmUe.SetSMSubsData(smData)
	var UeContextInSmfbody models.UeContextInSmfData
	udm_context.UDM_Self().CreateUeContextInSmfDataforUe(supi, UeContextInSmfbody)
	problemDetails = &models.ProblemDetails{
		Status: http.StatusNotFound,
		Cause:  "DATA_NOT_FOUND",
	}

	return problemDetails
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
	response, err := GetNssai(supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusOK, nil, response)
}

func GetNssai(supi string) (*models.Nssai, error) {
	accessAndMobilitySubscriptionDataResp, err := producer.GetAmData(supi)
	if err != nil {
		return nil, fmt.Errorf("GetAmData error: %+v", err)
	}
	nssaiResp := *accessAndMobilitySubscriptionDataResp.Nssai
	udmUe := udm_context.UDM_Self().NewUdmUe(supi)
	udmUe.Nssai = &nssaiResp
	return udmUe.Nssai, nil
}

func HandleGetSmfSelectDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetSmfSelectData")
	supi := request.Params["supi"]
	response, err := GetSmfSelectData(supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusOK, nil, response)
}

func GetSmfSelectData(supi string) (
	*models.SmfSelectionSubscriptionData, error,
) {
	var body models.SmfSelectionSubscriptionData
	udm_context.UDM_Self().CreateSmfSelectionSubsDataforUe(supi, body)
	smfSelectionSubscriptionDataResp, err := producer.GetSmfSelectData(supi)
	if err != nil {
		logger.SdmLog.Errorf("GetSmfSelectData error: %+v", err)
		return nil, fmt.Errorf("GetSmfSelectData error: %+v", err)
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
	err := CreateSubscription(&sdmSubscription, supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "USER_NOT_FOUND",
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusCreated, nil, nil)
}

func CreateSubscription(sdmSubscription *models.SdmSubscription, supi string) error {
	sdmSubscriptionResp := producer.CreateSdmSubscriptions(*sdmSubscription, supi)
	header := make(http.Header)
	udmUe, _ := udm_context.UDM_Self().UdmUeFindBySupi(supi)
	if udmUe == nil {
		udmUe = udm_context.UDM_Self().NewUdmUe(supi)
	}
	udmUe.CreateSubscriptiontoNotifChange(sdmSubscriptionResp.SubscriptionId, &sdmSubscriptionResp)
	header.Set("Location", udmUe.GetLocationURI2(udm_context.LocationUriSdmSubscription, supi))
	return nil
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
	err := producer.RemovesdmSubscriptions("====", subscriptionID)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "USER_NOT_FOUND",
		}
		return problemDetails
	}
	return nil
}

func HandleModifyRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle Modify")
	supi := request.Params["supi"]
	subscriptionID := request.Params["subscriptionId"]
	problemDetails := modifyProcedure(supi, subscriptionID)
	return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
}

func modifyProcedure(supi string, subscriptionID string) (
	problemDetails *models.ProblemDetails,
) {
	sdmSubscription := models.SdmSubscription{}
	err := producer.Updatesdmsubscriptions(supi, subscriptionID, sdmSubscription)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "USER_NOT_FOUND",
		}
		return problemDetails
	}
	return nil
}

func HandleGetUeContextInSmfDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SdmLog.Infof("Handle GetUeContextInSmfData")
	supi := request.Params["supi"]
	_, err := GetUeContextInSmfData(supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "DATA_NOT_FOUND",
			Detail: err.Error(),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusOK, nil, nil)
}

// Something does not seem right here. The function signature is not matching with the one in the generated code.
func GetUeContextInSmfData(supi string) (*models.UeContextInSmfData, error) {
	var ueContext models.UeContextInSmfData
	udm_context.UDM_Self().CreateUeContextInSmfDataforUe(supi, ueContext)
	logger.ProducerLog.Errorf("UeContext: %v", ueContext)
	return nil, nil
}
