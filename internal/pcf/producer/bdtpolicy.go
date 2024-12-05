package producer

import (
	"net/http"

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
