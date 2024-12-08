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
	"github.com/yeastengine/ella/internal/udr/producer"
)

func HandleDeletePoliciesPolAssoId(request *httpwrapper.Request) *httpwrapper.Response {
	logger.AMpolicylog.Infof("Handle AM Policy Association Delete")

	polAssoId := request.Params["polAssoId"]

	err := DeleteAMPolicy(polAssoId)
	if err != nil {
		problemDetails := util.GetProblemDetail(err.Error(), util.ERROR_REQUEST_PARAMETERS)
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
}

func DeleteAMPolicy(polAssoId string) error {
	ue := pcf_context.PCF_Self().PCFUeFindByPolicyId(polAssoId)
	if ue == nil || ue.AMPolicyData[polAssoId] == nil {
		return fmt.Errorf("polAssoId not found  in PCF")
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

	response, err := UpdateAMPolicy(polAssoId, policyAssociationUpdateRequest)
	if err != nil {
		problemDetails := util.GetProblemDetail(err.Error(), util.ERROR_REQUEST_PARAMETERS)
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return httpwrapper.NewResponse(http.StatusOK, nil, response)
}

func UpdateAMPolicy(polAssoId string, policyAssociationUpdateRequest models.PolicyAssociationUpdateRequest) (*models.PolicyUpdate, error) {
	ue := pcf_context.PCF_Self().PCFUeFindByPolicyId(polAssoId)
	if ue == nil || ue.AMPolicyData[polAssoId] == nil {
		return nil, fmt.Errorf("polAssoId not found  in PCF")
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
				return nil, fmt.Errorf("UserLoc doesn't exist in Policy Association Requset Update while Triggers include LOC_CH")
			}
			amPolicyData.UserLoc = policyAssociationUpdateRequest.UserLoc
			logger.AMpolicylog.Infof("Ue[%s] UserLocation %+v", ue.Supi, amPolicyData.UserLoc)
		case models.RequestTrigger_PRA_CH:
			if policyAssociationUpdateRequest.PraStatuses == nil {
				return nil, fmt.Errorf("PraStatuses doesn't exist in Policy Association")
			}
			for praId, praInfo := range policyAssociationUpdateRequest.PraStatuses {
				// TODO: report to AF subscriber
				logger.AMpolicylog.Infof("Policy Association Presence Id[%s] change state to %s", praId, praInfo.PresenceState)
			}
		case models.RequestTrigger_SERV_AREA_CH:
			if policyAssociationUpdateRequest.ServAreaRes == nil {
				return nil, fmt.Errorf("ServAreaRes doesn't exist in Policy Association Requset Update while Triggers include SERV_AREA_CH")
			} else {
				amPolicyData.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
				response.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
			}
		case models.RequestTrigger_RFSP_CH:
			if policyAssociationUpdateRequest.Rfsp == 0 {
				return nil, fmt.Errorf("Rfsp doesn't exist in Policy Association Requset Update while Triggers include RFSP_CH")
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

	policyAssociationRequest := request.Body.(models.PolicyAssociationRequest)

	response, locationHeader, err := CreateAMPolicy(policyAssociationRequest)
	headers := http.Header{
		"Location": {locationHeader},
	}
	if err != nil {
		problemDetails := util.GetProblemDetail(err.Error(), util.ERROR_REQUEST_PARAMETERS)
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	return httpwrapper.NewResponse(http.StatusCreated, headers, response)
}

func CreateAMPolicy(policyAssociationRequest models.PolicyAssociationRequest) (*models.PolicyAssociation, string, error) {
	var response models.PolicyAssociation
	pcfSelf := pcf_context.PCF_Self()
	var ue *pcf_context.UeContext
	if val, ok := pcfSelf.UePool.Load(policyAssociationRequest.Supi); ok {
		ue = val.(*pcf_context.UeContext)
	}
	if ue == nil {
		if newUe, err := pcfSelf.NewPCFUe(policyAssociationRequest.Supi); err != nil {
			return nil, "", fmt.Errorf("supi Format Error: %s", err.Error())
		} else {
			ue = newUe
		}
	}
	response.Request = deepcopy.Copy(&policyAssociationRequest).(*models.PolicyAssociationRequest)
	assolId := fmt.Sprintf("%s-%d", ue.Supi, ue.PolAssociationIDGenerator)
	amPolicy := ue.AMPolicyData[assolId]

	if amPolicy == nil || amPolicy.AmPolicyData == nil {
		amData, err := producer.GetAmPolicyData(ue.Supi)
		if err != nil {
			return nil, "", fmt.Errorf("can't find UE[%s] AM Policy Data in UDR", ue.Supi)
		}
		if amPolicy == nil {
			amPolicy = ue.NewUeAMPolicyData(assolId, policyAssociationRequest)
		}
		amPolicy.AmPolicyData = amData
	}

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
	logger.AMpolicylog.Debugf("AMPolicy association Id[%s] Create", assolId)

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
			problemDetails, err := consumer.AmfStatusChangeSubscribe(pcfSelf.AmfUri, []models.Guami{*policyAssociationRequest.Guami})
			if err != nil {
				logger.AMpolicylog.Errorf("Subscribe AMF status change error[%+v]", err)
			} else if problemDetails != nil {
				logger.AMpolicylog.Errorf("Subscribe AMF status change failed[%+v]", problemDetails)
			} else {
				amPolicy.Guami = policyAssociationRequest.Guami
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
