package producer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antihax/optional"
	"github.com/google/uuid"
	"github.com/mohae/deepcopy"
	"github.com/omec-project/openapi/Nudr_DataRepository"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	pcf_context "github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/pcf/util"
)

func HandleGetBDTPolicyContextRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.Bdtpolicylog.Infof("Handle GetBDTPolicyContext")

	// step 2: retrieve request
	bdtPolicyID := request.Params["bdtPolicyId"]

	// step 3: handle the message
	response, problemDetails := getBDTPolicyContextProcedure(bdtPolicyID)

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

func getBDTPolicyContextProcedure(bdtPolicyID string) (
	response *models.BdtPolicy, problemDetails *models.ProblemDetails,
) {
	logger.Bdtpolicylog.Debugln("Handle BDT Policy GET")
	// check bdtPolicyID from pcfUeContext
	if value, ok := pcf_context.PCF_Self().BdtPolicyPool.Load(bdtPolicyID); ok {
		bdtPolicy := value.(*models.BdtPolicy)
		return bdtPolicy, nil
	} else {
		// not found
		problemDetail := util.GetProblemDetail("Can't find bdtPolicyID related resource", util.CONTEXT_NOT_FOUND)
		logger.Bdtpolicylog.Warnf(problemDetail.Detail)
		return nil, &problemDetail
	}
}

// UpdateBDTPolicy - Update an Individual BDT policy (choose policy data)
func HandleUpdateBDTPolicyContextProcedure(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.Bdtpolicylog.Infof("Handle UpdateBDTPolicyContext")

	// step 2: retrieve request
	requestDataType := request.Body.(models.BdtPolicyDataPatch)
	bdtPolicyID := request.Params["bdtPolicyId"]

	// step 3: handle the message
	response, problemDetails := updateBDTPolicyContextProcedure(requestDataType, bdtPolicyID)

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

func updateBDTPolicyContextProcedure(request models.BdtPolicyDataPatch, bdtPolicyID string) (
	response *models.BdtPolicy, problemDetails *models.ProblemDetails,
) {
	logger.Bdtpolicylog.Infoln("Handle BDTPolicyUpdate")
	// check bdtPolicyID from pcfUeContext
	pcfSelf := pcf_context.PCF_Self()

	var bdtPolicy *models.BdtPolicy
	if value, ok := pcf_context.PCF_Self().BdtPolicyPool.Load(bdtPolicyID); ok {
		bdtPolicy = value.(*models.BdtPolicy)
	} else {
		// not found
		problemDetail := util.GetProblemDetail("Can't find bdtPolicyID related resource", util.CONTEXT_NOT_FOUND)
		logger.Bdtpolicylog.Warnf(problemDetail.Detail)
		return nil, &problemDetail
	}

	for _, policy := range bdtPolicy.BdtPolData.TransfPolicies {
		if policy.TransPolicyId == request.SelTransPolicyId {
			polData := bdtPolicy.BdtPolData
			polReq := bdtPolicy.BdtReqData
			polData.SelTransPolicyId = request.SelTransPolicyId
			bdtData := models.BdtData{
				AspId:       polReq.AspId,
				TransPolicy: policy,
				BdtRefId:    polData.BdtRefId,
			}
			if polReq.NwAreaInfo != nil {
				bdtData.NwAreaInfo = *polReq.NwAreaInfo
			}
			param := Nudr_DataRepository.PolicyDataBdtDataBdtReferenceIdPutParamOpts{
				BdtData: optional.NewInterface(bdtData),
			}
			client := util.GetNudrClient(pcfSelf.UdrUri)
			rsp, err := client.DefaultApi.PolicyDataBdtDataBdtReferenceIdPut(context.Background(), bdtData.BdtRefId, &param)
			if err != nil {
				logger.Bdtpolicylog.Warnf("UDR Put BdtDate error[%s]", err.Error())
			}
			defer func() {
				if rspCloseErr := rsp.Body.Close(); rspCloseErr != nil {
					logger.Bdtpolicylog.Errorf("PolicyDataBdtDataBdtReferenceIdPut response body cannot close: %+v", rspCloseErr)
				}
			}()
			logger.Bdtpolicylog.Debugln("bdtPolicyID[%s] has Updated with SelTransPolicyId[%d]",
				bdtPolicyID, request.SelTransPolicyId)
			return bdtPolicy, nil
		}
	}
	problemDetail := util.GetProblemDetail(
		fmt.Sprintf("Can't find TransPolicyId[%d] in TransfPolicies with bdtPolicyID[%s]",
			request.SelTransPolicyId, bdtPolicyID),
		util.CONTEXT_NOT_FOUND)
	logger.Bdtpolicylog.Warnf(problemDetail.Detail)
	return nil, &problemDetail
}

// CreateBDTPolicy - Create a new Individual BDT policy
func HandleCreateBDTPolicyContextRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.Bdtpolicylog.Infof("Handle CreateBDTPolicyContext")

	// step 2: retrieve request
	requestMsg := request.Body.(models.BdtReqData)

	// step 3: handle the message
	header, response, problemDetails := createBDTPolicyContextProcedure(&requestMsg)

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

func createBDTPolicyContextProcedure(request *models.BdtReqData) (
	header http.Header, response *models.BdtPolicy, problemDetails *models.ProblemDetails,
) {
	response = &models.BdtPolicy{}
	logger.Bdtpolicylog.Debugln("Handle BDT Policy Create")

	pcfSelf := pcf_context.PCF_Self()

	// Query BDT DATA array from UDR
	client := util.GetNudrClient(pcfSelf.UdrUri)
	bdtDatas, httpResponse, err := client.DefaultApi.PolicyDataBdtDataGet(context.Background())
	if err != nil || httpResponse == nil || httpResponse.StatusCode != http.StatusOK {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusServiceUnavailable,
			Detail: "Query to UDR failed",
		}
		logger.Bdtpolicylog.Warnf("Query to UDR failed")
		return nil, nil, problemDetails
	}
	defer func() {
		if rspCloseErr := httpResponse.Body.Close(); rspCloseErr != nil {
			logger.Bdtpolicylog.Errorf("PolicyDataBdtDataGet response body cannot close: %+v", rspCloseErr)
		}
	}()
	// TODO: decide BDT Policy from other bdt policy data
	response.BdtReqData = deepcopy.Copy(&request).(*models.BdtReqData)
	var bdtData *models.BdtData
	var bdtPolicyData models.BdtPolicyData
	for _, data := range bdtDatas {
		// If ASP has exist, use its background data policy
		if request.AspId == data.AspId {
			bdtData = &data
			break
		}
	}
	// Only support one bdt policy, TODO: more policy for decision
	if bdtData != nil {
		// found
		// modify policy according to new request
		bdtData.TransPolicy.RecTimeInt = request.DesTimeInt
	} else {
		// use default bdt policy, TODO: decide bdt transfer data policy
		bdtData = &models.BdtData{
			AspId:       request.AspId,
			BdtRefId:    uuid.New().String(),
			TransPolicy: getDefaultTransferPolicy(1, *request.DesTimeInt),
		}
	}
	if request.NwAreaInfo != nil {
		bdtData.NwAreaInfo = *request.NwAreaInfo
	}
	bdtPolicyData.SelTransPolicyId = bdtData.TransPolicy.TransPolicyId
	// no support feature in subclause 5.8 of TS29554
	bdtPolicyData.BdtRefId = bdtData.BdtRefId
	bdtPolicyData.TransfPolicies = append(bdtPolicyData.TransfPolicies, bdtData.TransPolicy)
	response.BdtPolData = &bdtPolicyData
	bdtPolicyID, err := pcfSelf.AllocBdtPolicyID()
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusServiceUnavailable,
			Detail: "Allocate bdtPolicyID failed",
		}
		logger.Bdtpolicylog.Warnf("Allocate bdtPolicyID failed")
		return nil, nil, problemDetails
	}

	pcfSelf.BdtPolicyPool.Store(bdtPolicyID, response)

	// Update UDR BDT Data(PUT)
	param := Nudr_DataRepository.PolicyDataBdtDataBdtReferenceIdPutParamOpts{
		BdtData: optional.NewInterface(*bdtData),
	}

	var updateRsp *http.Response
	if rsp, rspErr := client.DefaultApi.PolicyDataBdtDataBdtReferenceIdPut(context.Background(),
		bdtPolicyData.BdtRefId, &param); rspErr != nil {
		logger.Bdtpolicylog.Warnf("UDR Put BdtDate error[%s]", rspErr.Error())
	} else {
		updateRsp = rsp
	}
	defer func() {
		if rspCloseErr := updateRsp.Body.Close(); rspCloseErr != nil {
			logger.Bdtpolicylog.Errorf("PolicyDataBdtDataBdtReferenceIdPut response body cannot close: %+v", rspCloseErr)
		}
	}()

	locationHeader := util.GetResourceUri(models.ServiceName_NPCF_BDTPOLICYCONTROL, bdtPolicyID)
	header = http.Header{
		"Location": {locationHeader},
	}
	logger.Bdtpolicylog.Debugln("BDT Policy Id[%s] Create", bdtPolicyID)
	return header, response, problemDetails
}

// get default background data transfer policy
func getDefaultTransferPolicy(transferPolicyId int32, timeWindow models.TimeWindow) models.TransferPolicy {
	return models.TransferPolicy{
		TransPolicyId: transferPolicyId,
		RecTimeInt:    &timeWindow,
		RatingGroup:   1,
	}
}
