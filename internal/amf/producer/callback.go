package producer

import (
	"fmt"
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	gmm_message "github.com/yeastengine/ella/internal/amf/gmm/message"
	"github.com/yeastengine/ella/internal/amf/logger"
	ngap_message "github.com/yeastengine/ella/internal/amf/ngap/message"
)

func AmPolicyControlUpdateNotifyUpdateProcedure(polAssoID string,
	policyUpdate models.PolicyUpdate,
) *models.ProblemDetails {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByPolicyAssociationID(polAssoID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Policy Association ID[%s] Not Found", polAssoID),
		}
		return problemDetails
	}

	ue.AmPolicyAssociation.Triggers = policyUpdate.Triggers
	ue.RequestTriggerLocationChange = false

	for _, trigger := range policyUpdate.Triggers {
		if trigger == models.RequestTrigger_LOC_CH {
			ue.RequestTriggerLocationChange = true
		}
		//if trigger == models.RequestTrigger_PRA_CH {
		// TODO: Presence Reporting Area handling (TS 23.503 6.1.2.5, TS 23.501 5.6.11)
		//}
	}

	if policyUpdate.ServAreaRes != nil {
		ue.AmPolicyAssociation.ServAreaRes = policyUpdate.ServAreaRes
	}

	if policyUpdate.Rfsp != 0 {
		ue.AmPolicyAssociation.Rfsp = policyUpdate.Rfsp
	}

	if ue != nil {
		// use go routine to write response first to ensure the order of the procedure
		go func() {
			// UE is CM-Connected State
			if ue.CmConnect(models.AccessType__3_GPP_ACCESS) {
				gmm_message.SendConfigurationUpdateCommand(ue, models.AccessType__3_GPP_ACCESS, nil)
				// UE is CM-IDLE => paging
			} else {
				message, err := gmm_message.BuildConfigurationUpdateCommand(ue, models.AccessType__3_GPP_ACCESS, nil)
				if err != nil {
					logger.GmmLog.Errorf("Build Configuration Update Command Failed : %s", err.Error())
					return
				}

				ue.ConfigurationUpdateMessage = message
				ue.SetOnGoing(models.AccessType__3_GPP_ACCESS, &context.OnGoingProcedureWithPrio{
					Procedure: context.OnGoingProcedurePaging,
				})

				pkg, err := ngap_message.BuildPaging(ue, nil, false)
				if err != nil {
					logger.NgapLog.Errorf("Build Paging failed : %s", err.Error())
					return
				}
				ngap_message.SendPaging(ue, pkg)
			}
		}()
	}
	return nil
}

// func HandleNfSubscriptionStatusNotify(request *httpwrapper.Request) *httpwrapper.Response {
// 	logger.ProducerLog.Debugln("[AMF] Handle NF Status Notify")

// 	notificationData := request.Body.(models.NotificationData)

// 	problemDetails := NfSubscriptionStatusNotifyProcedure(notificationData)
// 	if problemDetails != nil {
// 		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
// 	} else {
// 		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
// 	}
// }

// // To delete (?)
// func NfSubscriptionStatusNotifyProcedure(notificationData models.NotificationData) *models.ProblemDetails {
// 	logger.ProducerLog.Debugf("NfSubscriptionStatusNotify: %+v", notificationData)

// 	if notificationData.Event == "" || notificationData.NfInstanceUri == "" {
// 		problemDetails := &models.ProblemDetails{
// 			Status: http.StatusBadRequest,
// 			Cause:  "MANDATORY_IE_MISSING", // Defined in TS 29.510 6.1.6.2.17
// 			Detail: "Missing IE [Event]/[NfInstanceUri] in NotificationData",
// 		}
// 		return problemDetails
// 	}
// 	return nil
// }

// func HandleDeregistrationNotification(request *httpwrapper.Request) *httpwrapper.Response {
// 	logger.ProducerLog.Infoln("Handle Deregistration Notification")
// 	deregistrationData := request.Body.(models.DeregistrationData)

// 	switch deregistrationData.DeregReason {
// 	case "SUBSCRIPTION_WITHDRAWN":
// 		amfSelf := context.AMF_Self()
// 		if supi, exists := request.Params["supi"]; exists {
// 			reqUri := request.URL.RequestURI()
// 			if ue, ok := amfSelf.AmfUeFindBySupi(supi); ok {
// 				logger.ProducerLog.Debugln("amf ue found: ", ue.Supi)
// 				sbiMsg := context.SbiMsg{
// 					UeContextId: ue.Supi,
// 					ReqUri:      reqUri,
// 					Msg:         nil,
// 					Result:      make(chan context.SbiResponseMsg, 10),
// 				}
// 				ue.EventChannel.UpdateSbiHandler(HandleOAMPurgeUEContextRequest)
// 				ue.EventChannel.SubmitMessage(sbiMsg)
// 				msg := <-sbiMsg.Result
// 				if msg.ProblemDetails != nil {
// 					return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
// 				} else {
// 					return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
// 				}
// 			} else {
// 				return httpwrapper.NewResponse(http.StatusNotFound, nil, nil)
// 			}
// 		}

// 	case "":
// 		problemDetails := &models.ProblemDetails{
// 			Status: http.StatusBadRequest,
// 			Cause:  "MANDATORY_IE_MISSING", // Defined in TS 29.503 6.2.5.2
// 			Detail: "Missing IE [DeregReason] in DeregistrationData",
// 		}
// 		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)

// 	default:
// 		problemDetails := &models.ProblemDetails{
// 			Status: http.StatusNotImplemented,
// 			Cause:  "NOT_IMPLEMENTED", // Defined in TS 29.503
// 			Detail: "Unsupported [DeregReason] in DeregistrationData",
// 		}
// 		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
// 	}
// 	return nil
// }
