package consumer

import (
	"fmt"
	"net/http"

	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/pcf/producer"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
)

// SendSMPolicyAssociationCreate creates the SM Policy Decision
func SendSMPolicyAssociationCreate(smContext *smf_context.SMContext) (*models.SmPolicyDecision, int, error) {
	httpRspStatusCode := http.StatusInternalServerError
	smPolicyData := models.SmPolicyContextData{}
	smPolicyData.Supi = smContext.Supi
	smPolicyData.PduSessionId = smContext.PDUSessionID
	smPolicyData.NotificationUri = fmt.Sprintf("nsmf-callback/sm-policies/%s",
		smContext.Ref,
	)
	smPolicyData.Dnn = smContext.Dnn
	smPolicyData.PduSessionType = nasConvert.PDUSessionTypeToModels(smContext.SelectedPDUSessionType)
	smPolicyData.AccessType = smContext.AnType
	smPolicyData.RatType = smContext.RatType
	smPolicyData.Ipv4Address = smContext.PDUAddress.Ip.To4().String()
	smPolicyData.SubsSessAmbr = smContext.DnnConfiguration.SessionAmbr
	smPolicyData.SubsDefQos = smContext.DnnConfiguration.Var5gQosProfile
	smPolicyData.SliceInfo = smContext.Snssai
	smPolicyData.ServingNetwork = &models.NetworkId{
		Mcc: smContext.ServingNetwork.Mcc,
		Mnc: smContext.ServingNetwork.Mnc,
	}
	smPolicyData.SuppFeat = "F"

	smPolicyDecision, err := producer.CreateSMPolicy(smPolicyData)
	if err != nil {
		return nil, httpRspStatusCode, fmt.Errorf("setup sm policy association failed: %s", err.Error())
	}
	err = validateSmPolicyDecision(smPolicyDecision)
	if err != nil {
		return nil, httpRspStatusCode, fmt.Errorf("setup sm policy association failed: %s", err.Error())
	}
	return smPolicyDecision, http.StatusCreated, nil
}

func SendSMPolicyAssociationModify(smContext *smf_context.SMContext) {}

func SendSMPolicyAssociationDelete(smContext *smf_context.SMContext, smDelReq *models.ReleaseSmContextRequest) (int, error) {
	smPolicyID := fmt.Sprintf("%s-%d", smContext.Supi, smContext.PDUSessionID)
	err := producer.DeleteSMPolicy(smPolicyID)
	if err != nil {
		logger.SmfLog.Warnf("smf policy delete failed, [%v] ", err.Error())
		return http.StatusInternalServerError, err
	}
	return http.StatusAccepted, nil
}

func validateSmPolicyDecision(smPolicy *models.SmPolicyDecision) error {
	// Validate just presence of important IEs as of now
	// Sess Rules
	for name, rule := range smPolicy.SessRules {
		if rule.AuthSessAmbr == nil {
			logger.SmfLog.Errorf("SM policy decision rule [%s] validation failure, authorised session ambr missing", name)
			return fmt.Errorf("authorised session ambr missing")
		}

		if rule.AuthDefQos == nil {
			logger.SmfLog.Errorf("SM policy decision rule [%s] validation failure, authorised default qos missing", name)
			return fmt.Errorf("authorised default qos missing")
		}
	}
	return nil
}
