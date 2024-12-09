package producer

import (
	"fmt"
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	gmm_message "github.com/yeastengine/ella/internal/amf/gmm/message"
	ngap_message "github.com/yeastengine/ella/internal/amf/ngap/message"
	"github.com/yeastengine/ella/internal/logger"
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
					logger.AmfLog.Errorf("Build Configuration Update Command Failed : %s", err.Error())
					return
				}

				ue.ConfigurationUpdateMessage = message
				ue.SetOnGoing(models.AccessType__3_GPP_ACCESS, &context.OnGoingProcedureWithPrio{
					Procedure: context.OnGoingProcedurePaging,
				})

				pkg, err := ngap_message.BuildPaging(ue, nil, false)
				if err != nil {
					logger.AmfLog.Errorf("Build Paging failed : %s", err.Error())
					return
				}
				ngap_message.SendPaging(ue, pkg)
			}
		}()
	}
	return nil
}
