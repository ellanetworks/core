package producer

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/mohae/deepcopy"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/pcf/consumer"
	pcf_context "github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/pcf/util"
)

func HandleDeletePoliciesPolAssoId(request *httpwrapper.Request) *httpwrapper.Response {
	logger.AMpolicylog.Infof("Handle AM Policy Association Delete")

	polAssoId := request.Params["polAssoId"]

	problemDetails := DeletePoliciesPolAssoIdProcedure(polAssoId)
	if problemDetails == nil {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	} else {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
}

func DeletePoliciesPolAssoIdProcedure(polAssoId string) *models.ProblemDetails {
	ue := pcf_context.PCF_Self().PCFUeFindByPolicyId(polAssoId)
	if ue == nil || ue.AMPolicyData[polAssoId] == nil {
		problemDetails := util.GetProblemDetail("polAssoId not found  in PCF", util.CONTEXT_NOT_FOUND)
		return &problemDetails
	}
	delete(ue.AMPolicyData, polAssoId)
	return nil
}

// PoliciesPolAssoIdGet -
func HandleGetPoliciesPolAssoId(request *httpwrapper.Request) *httpwrapper.Response {
	logger.AMpolicylog.Infof("Handle AM Policy Association Get")

	polAssoId := request.Params["polAssoId"]

	response, problemDetails := GetPoliciesPolAssoIdProcedure(polAssoId)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
}

func GetPoliciesPolAssoIdProcedure(polAssoId string) (*models.PolicyAssociation, *models.ProblemDetails) {
	ue := pcf_context.PCF_Self().PCFUeFindByPolicyId(polAssoId)
	if ue == nil || ue.AMPolicyData[polAssoId] == nil {
		problemDetails := util.GetProblemDetail("polAssoId not found  in PCF", util.CONTEXT_NOT_FOUND)
		return nil, &problemDetails
	}
	amPolicyData := ue.AMPolicyData[polAssoId]
	rsp := models.PolicyAssociation{
		SuppFeat: amPolicyData.SuppFeat,
	}
	if amPolicyData.Rfsp != 0 {
		rsp.Rfsp = amPolicyData.Rfsp
	}
	if amPolicyData.ServAreaRes != nil {
		rsp.ServAreaRes = amPolicyData.ServAreaRes
	}
	if amPolicyData.Triggers != nil {
		rsp.Triggers = amPolicyData.Triggers
		for _, trigger := range amPolicyData.Triggers {
			if trigger == models.RequestTrigger_PRA_CH {
				rsp.Pras = amPolicyData.Pras
				break
			}
		}
	}
	return &rsp, nil
}

func HandleUpdatePostPoliciesPolAssoId(request *httpwrapper.Request) *httpwrapper.Response {
	logger.AMpolicylog.Infof("Handle AM Policy Association Update")

	polAssoId := request.Params["polAssoId"]
	policyAssociationUpdateRequest := request.Body.(models.PolicyAssociationUpdateRequest)

	response, problemDetails := UpdatePostPoliciesPolAssoIdProcedure(polAssoId, policyAssociationUpdateRequest)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
}

func UpdatePostPoliciesPolAssoIdProcedure(polAssoId string,
	policyAssociationUpdateRequest models.PolicyAssociationUpdateRequest,
) (*models.PolicyUpdate, *models.ProblemDetails) {
	ue := pcf_context.PCF_Self().PCFUeFindByPolicyId(polAssoId)
	if ue == nil || ue.AMPolicyData[polAssoId] == nil {
		problemDetails := util.GetProblemDetail("polAssoId not found  in PCF", util.CONTEXT_NOT_FOUND)
		return nil, &problemDetails
	}

	amPolicyData := ue.AMPolicyData[polAssoId]
	var response models.PolicyUpdate
	if policyAssociationUpdateRequest.NotificationUri != "" {
		amPolicyData.NotificationUri = policyAssociationUpdateRequest.NotificationUri
	}
	if policyAssociationUpdateRequest.AltNotifIpv4Addrs != nil {
		amPolicyData.AltNotifIpv4Addrs = policyAssociationUpdateRequest.AltNotifIpv4Addrs
	}
	if policyAssociationUpdateRequest.AltNotifIpv6Addrs != nil {
		amPolicyData.AltNotifIpv6Addrs = policyAssociationUpdateRequest.AltNotifIpv6Addrs
	}
	for _, trigger := range policyAssociationUpdateRequest.Triggers {
		// TODO: Modify the value according to policies
		switch trigger {
		case models.RequestTrigger_LOC_CH:
			// TODO: report to AF subscriber
			if policyAssociationUpdateRequest.UserLoc == nil {
				problemDetail := util.GetProblemDetail("UserLoc are nli", util.ERROR_REQUEST_PARAMETERS)
				logger.AMpolicylog.Warnln(
					"UserLoc doesn't exist in Policy Association Requset Update while Triggers include LOC_CH")
				return nil, &problemDetail
			}
			amPolicyData.UserLoc = policyAssociationUpdateRequest.UserLoc
			logger.AMpolicylog.Infof("Ue[%s] UserLocation %+v", ue.Supi, amPolicyData.UserLoc)
		case models.RequestTrigger_PRA_CH:
			if policyAssociationUpdateRequest.PraStatuses == nil {
				problemDetail := util.GetProblemDetail("PraStatuses are nli", util.ERROR_REQUEST_PARAMETERS)
				logger.AMpolicylog.Warnln("PraStatuses doesn't exist in Policy Association",
					"Requset Update while Triggers include PRA_CH")
				return nil, &problemDetail
			}
			for praId, praInfo := range policyAssociationUpdateRequest.PraStatuses {
				// TODO: report to AF subscriber
				logger.AMpolicylog.Infof("Policy Association Presence Id[%s] change state to %s", praId, praInfo.PresenceState)
			}
		case models.RequestTrigger_SERV_AREA_CH:
			if policyAssociationUpdateRequest.ServAreaRes == nil {
				problemDetail := util.GetProblemDetail("ServAreaRes are nli", util.ERROR_REQUEST_PARAMETERS)
				logger.AMpolicylog.Warnln("ServAreaRes doesn't exist in Policy Association",
					"Requset Update while Triggers include SERV_AREA_CH")
				return nil, &problemDetail
			} else {
				amPolicyData.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
				response.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
			}
		case models.RequestTrigger_RFSP_CH:
			if policyAssociationUpdateRequest.Rfsp == 0 {
				problemDetail := util.GetProblemDetail("Rfsp are nli", util.ERROR_REQUEST_PARAMETERS)
				logger.AMpolicylog.Warnln("Rfsp doesn't exist in Policy Association Requset Update while Triggers include RFSP_CH")
				return nil, &problemDetail
			} else {
				amPolicyData.Rfsp = policyAssociationUpdateRequest.Rfsp
				response.Rfsp = policyAssociationUpdateRequest.Rfsp
			}
		}
	}
	// TODO: handle TraceReq
	// TODO: Change Request Trigger Policies if needed
	response.Triggers = amPolicyData.Triggers
	// TODO: Change Policies if needed
	// rsp.Pras
	return &response, nil
}

// Create AM Policy
func HandlePostPolicies(request *httpwrapper.Request) *httpwrapper.Response {
	logger.AMpolicylog.Infof("Handle AM Policy Create Request")

	polAssoId := request.Params["polAssoId"]
	policyAssociationRequest := request.Body.(models.PolicyAssociationRequest)

	response, locationHeader, problemDetails := PostPoliciesProcedure(polAssoId, policyAssociationRequest)
	headers := http.Header{
		"Location": {locationHeader},
	}
	if response != nil {
		return httpwrapper.NewResponse(http.StatusCreated, headers, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
}

func PostPoliciesProcedure(polAssoId string,
	policyAssociationRequest models.PolicyAssociationRequest,
) (*models.PolicyAssociation, string, *models.ProblemDetails) {
	var response models.PolicyAssociation
	pcfSelf := pcf_context.PCF_Self()
	var ue *pcf_context.UeContext
	if val, ok := pcfSelf.UePool.Load(policyAssociationRequest.Supi); ok {
		ue = val.(*pcf_context.UeContext)
	}
	if ue == nil {
		if newUe, err := pcfSelf.NewPCFUe(policyAssociationRequest.Supi); err != nil {
			// supi format dose not match "imsi-..."
			problemDetail := util.GetProblemDetail("Supi Format Error", util.ERROR_REQUEST_PARAMETERS)
			logger.AMpolicylog.Errorln(err.Error())
			return nil, "", &problemDetail
		} else {
			ue = newUe
		}
	}
	udrUri := getUdrUri(ue)
	if udrUri == "" {
		// Can't find any UDR support this Ue
		pcfSelf.UePool.Delete(ue.Supi)
		problemDetail := util.GetProblemDetail("Ue is not supported in PCF", util.USER_UNKNOWN)
		logger.AMpolicylog.Errorf("Ue[%s] is not supported in PCF", ue.Supi)
		return nil, "", &problemDetail
	}
	ue.UdrUri = udrUri

	response.Request = deepcopy.Copy(&policyAssociationRequest).(*models.PolicyAssociationRequest)
	assolId := fmt.Sprintf("%s-%d", ue.Supi, ue.PolAssociationIDGenerator)
	amPolicy := ue.AMPolicyData[assolId]

	if amPolicy == nil || amPolicy.AmPolicyData == nil {
		client := util.GetNudrClient(udrUri)
		var response *http.Response
		amData, response, err := client.DefaultApi.PolicyDataUesUeIdAmDataGet(context.Background(), ue.Supi)
		if err != nil || response == nil || response.StatusCode != http.StatusOK {
			problemDetail := util.GetProblemDetail("Can't find UE AM Policy Data in UDR", util.USER_UNKNOWN)
			logger.AMpolicylog.Errorf("Can't find UE[%s] AM Policy Data in UDR", ue.Supi)
			return nil, "", &problemDetail
		}
		defer func() {
			if rspCloseErr := response.Body.Close(); rspCloseErr != nil {
				logger.AMpolicylog.Errorf("PolicyDataUesUeIdAmDataGet response cannot close: %+v", rspCloseErr)
			}
		}()
		if amPolicy == nil {
			amPolicy = ue.NewUeAMPolicyData(assolId, policyAssociationRequest)
		}
		amPolicy.AmPolicyData = &amData
	}

	// TODO: according to PCF Policy to determine ServAreaRes, Rfsp, SuppFeat
	// amPolicy.ServAreaRes =
	// amPolicy.Rfsp =
	var requestSuppFeat openapi.SupportedFeature
	if suppFeat, err := openapi.NewSupportedFeature(policyAssociationRequest.SuppFeat); err != nil {
		logger.AMpolicylog.Warnln(err)
	} else {
		requestSuppFeat = suppFeat
	}
	amPolicy.SuppFeat = pcfSelf.PcfSuppFeats[models.
		ServiceName_NPCF_AM_POLICY_CONTROL].NegotiateWith(
		requestSuppFeat).String()
	if amPolicy.Rfsp != 0 {
		response.Rfsp = amPolicy.Rfsp
	}
	response.SuppFeat = amPolicy.SuppFeat
	// TODO: add Reports
	// rsp.Triggers
	// rsp.Pras
	ue.PolAssociationIDGenerator++
	// Create location header for update, delete, get
	locationHeader := util.GetResourceUri(models.ServiceName_NPCF_AM_POLICY_CONTROL, assolId)
	logger.AMpolicylog.Tracef("AMPolicy association Id[%s] Create", assolId)

	// if consumer is AMF then subscribe this AMF Status
	if policyAssociationRequest.Guami != nil {
		// if policyAssociationRequest.Guami has been subscribed, then no need to subscribe again
		needSubscribe := true
		pcfSelf.AMFStatusSubsData.Range(func(key, value interface{}) bool {
			data := value.(pcf_context.AMFStatusSubscriptionData)
			for _, guami := range data.GuamiList {
				if reflect.DeepEqual(guami, *policyAssociationRequest.Guami) {
					needSubscribe = false
					break
				}
			}
			// if no need to subscribe => stop iteration
			return needSubscribe
		})

		if needSubscribe {
			logger.AMpolicylog.Debugf("Subscribe AMF status change[GUAMI: %+v]", *policyAssociationRequest.Guami)
			amfUri := consumer.SendNFIntancesAMF(pcfSelf.NrfUri, *policyAssociationRequest.Guami, models.ServiceName_NAMF_COMM)
			if amfUri != "" {
				problemDetails, err := consumer.AmfStatusChangeSubscribe(amfUri, []models.Guami{*policyAssociationRequest.Guami})
				if err != nil {
					logger.AMpolicylog.Errorf("Subscribe AMF status change error[%+v]", err)
				} else if problemDetails != nil {
					logger.AMpolicylog.Errorf("Subscribe AMF status change failed[%+v]", problemDetails)
				} else {
					amPolicy.Guami = policyAssociationRequest.Guami
				}
			}
		} else {
			logger.AMpolicylog.Debugf("AMF status[GUAMI: %+v] has been subscribed", *policyAssociationRequest.Guami)
		}
	}
	return &response, locationHeader, nil
}

// Send AM Policy Update to AMF if policy has changed
func SendAMPolicyUpdateNotification(ue *pcf_context.UeContext, PolId string, request models.PolicyUpdate) {
	if ue == nil {
		logger.AMpolicylog.Warnln("Policy Update Notification Error[Ue is nil]")
		return
	}
	amPolicyData := ue.AMPolicyData[PolId]
	if amPolicyData == nil {
		logger.AMpolicylog.Warnf("Policy Update Notification Error[Can't find polAssoId[%s] in UE(%s)]", PolId, ue.Supi)
		return
	}
	client := util.GetNpcfAMPolicyCallbackClient()
	uri := amPolicyData.NotificationUri
	for uri != "" {
		rsp, err := client.DefaultCallbackApi.PolicyUpdateNotification(context.Background(), uri, request)
		if err != nil {
			if rsp != nil && rsp.StatusCode != http.StatusNoContent {
				logger.AMpolicylog.Warnf("Policy Update Notification Error[%s]", rsp.Status)
			} else {
				logger.AMpolicylog.Warnf("Policy Update Notification Failed[%s]", err.Error())
			}
			return
		} else if rsp == nil {
			logger.AMpolicylog.Warnln("Policy Update Notification Failed[HTTP Response is nil]")
			return
		}
		defer func() {
			if rspCloseErr := rsp.Body.Close(); rspCloseErr != nil {
				logger.AMpolicylog.Errorf("PolicyUpdateNotification response cannot close: %+v", rspCloseErr)
			}
		}()
		if rsp.StatusCode == http.StatusTemporaryRedirect {
			// for redirect case, resend the notification to redirect target
			uRI, err := rsp.Location()
			if err != nil {
				logger.AMpolicylog.Warnln("Policy Update Notification Redirect Need Supply URI")
				return
			}
			uri = uRI.String()
			continue
		}

		logger.AMpolicylog.Infoln("Policy Update Notification Success")
		return
	}
}

// Send AM Policy Update to AMF if policy has been terminated
func SendAMPolicyTerminationRequestNotification(ue *pcf_context.UeContext,
	PolId string, request models.TerminationNotification,
) {
	if ue == nil {
		logger.AMpolicylog.Warnln("Policy Assocition Termination Request Notification Error[Ue is nil]")
		return
	}
	amPolicyData := ue.AMPolicyData[PolId]
	if amPolicyData == nil {
		logger.AMpolicylog.Warnf(
			"Policy Assocition Termination Request Notification Error[Can't find polAssoId[%s] in UE(%s)]", PolId, ue.Supi)
		return
	}
	client := util.GetNpcfAMPolicyCallbackClient()
	uri := amPolicyData.NotificationUri
	for uri != "" {
		rsp, err := client.DefaultCallbackApi.PolicyAssocitionTerminationRequestNotification(
			context.Background(), uri, request)
		if err != nil {
			if rsp != nil && rsp.StatusCode != http.StatusNoContent {
				logger.AMpolicylog.Warnf("Policy Assocition Termination Request Notification Error[%s]", rsp.Status)
			} else {
				logger.AMpolicylog.Warnf("Policy Assocition Termination Request Notification Failed[%s]", err.Error())
			}
			return
		} else if rsp == nil {
			logger.AMpolicylog.Warnln("Policy Assocition Termination Request Notification Failed[HTTP Response is nil]")
			return
		}
		defer func() {
			if rspCloseErr := rsp.Body.Close(); rspCloseErr != nil {
				logger.AMpolicylog.Errorf(
					"PolicyAssociationTerminationRequestNotification response body cannot close: %+v",
					rspCloseErr)
			}
		}()
		if rsp.StatusCode == http.StatusTemporaryRedirect {
			// for redirect case, resend the notification to redirect target
			uRI, err := rsp.Location()
			if err != nil {
				logger.AMpolicylog.Warnln("Policy Assocition Termination Request Notification Redirect Need Supply URI")
				return
			}
			uri = uRI.String()
			continue
		}
		return
	}
}

// returns UDR Uri of Ue, if ue.UdrUri dose not exist, query NRF to get supported Udr Uri
func getUdrUri(ue *pcf_context.UeContext) string {
	return consumer.SendNFIntancesUDR(pcf_context.PCF_Self().NrfUri, ue.Supi)
}
