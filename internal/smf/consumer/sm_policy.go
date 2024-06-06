package consumer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/openapi/models"
	"github.com/pkg/errors"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
)

// SendSMPolicyAssociationCreate create the session management association to the PCF
func SendSMPolicyAssociationCreate(smContext *smf_context.SMContext) (*models.SmPolicyDecision, int, error) {
	httpRspStatusCode := http.StatusInternalServerError
	if smContext.SMPolicyClient == nil {
		return nil, httpRspStatusCode, errors.Errorf("smContext not selected PCF")
	}

	smPolicyData := models.SmPolicyContextData{}

	smPolicyData.Supi = smContext.Supi
	smPolicyData.PduSessionId = smContext.PDUSessionID
	smPolicyData.NotificationUri = fmt.Sprintf("%s://%s:%d/nsmf-callback/sm-policies/%s",
		smf_context.SMF_Self().URIScheme,
		smf_context.SMF_Self().RegisterIPv4,
		smf_context.SMF_Self().SBIPort,
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

	var smPolicyDecision *models.SmPolicyDecision
	if smPolicyDecisionFromPCF, httpRsp, err := smContext.SMPolicyClient.
		DefaultApi.SmPoliciesPost(context.Background(), smPolicyData); err != nil {
		if httpRsp != nil {
			httpRspStatusCode = httpRsp.StatusCode
		}
		return nil, httpRspStatusCode, fmt.Errorf("setup sm policy association failed: %s", err.Error())
	} else {
		httpRspStatusCode = http.StatusCreated
		smPolicyDecision = &smPolicyDecisionFromPCF
	}

	if err := validateSmPolicyDecision(smPolicyDecision); err != nil {
		return nil, httpRspStatusCode, fmt.Errorf("setup sm policy association failed: %s", err.Error())
	}

	return smPolicyDecision, httpRspStatusCode, nil
}

func SendSMPolicyAssociationModify(smContext *smf_context.SMContext) {
	// TODO
}

func SendSMPolicyAssociationDelete(smContext *smf_context.SMContext, smDelReq *models.ReleaseSmContextRequest) (int, error) {
	smPolicyDelData := models.SmPolicyDeleteData{}

	// Populate Policy delete data
	// Network Id
	smPolicyDelData.ServingNetwork = &models.NetworkId{
		Mcc: smContext.ServingNetwork.Mcc,
		Mnc: smContext.ServingNetwork.Mnc,
	}

	// User location info
	if smDelReq.JsonData.UeLocation != nil {
		smPolicyDelData.UserLocationInfo = smDelReq.JsonData.UeLocation
	} else if smDelReq.JsonData.AddUeLocation != nil {
		smPolicyDelData.UserLocationInfo = smDelReq.JsonData.AddUeLocation
	}

	// UE Time Zone
	if smDelReq.JsonData.UeTimeZone != "" {
		smPolicyDelData.UeTimeZone = smDelReq.JsonData.UeTimeZone
	}

	// RAN/NAS Release Cause
	ranNasRelCause := models.RanNasRelCause{}
	if smDelReq.JsonData.NgApCause != nil {
		ranNasRelCause.NgApCause = smDelReq.JsonData.NgApCause
	}
	// MM cause
	ranNasRelCause.Var5gMmCause = smDelReq.JsonData.Var5gMmCauseValue

	// SM Cause ?
	// ranNasRelCause.Var5gSmCause =

	smPolicyDelData.RanNasRelCauses = []models.RanNasRelCause{ranNasRelCause}

	// Policy Id (supi-pduSessId)
	smPolicyID := fmt.Sprintf("%s-%d", smContext.Supi, smContext.PDUSessionID)

	// Send to  PCF
	if httpRsp, err := smContext.SMPolicyClient.
		DefaultApi.SmPoliciesSmPolicyIdDeletePost(context.Background(), smPolicyID, smPolicyDelData); err != nil {
		logger.ConsumerLog.Warnf("smf policy delete failed, [%v] ", err.Error())
		return 0, err
	} else {
		return httpRsp.StatusCode, nil
	}
}

func validateSmPolicyDecision(smPolicy *models.SmPolicyDecision) error {
	// Validate just presence of important IEs as of now
	// Sess Rules
	for name, rule := range smPolicy.SessRules {
		if rule.AuthSessAmbr == nil {
			logger.ConsumerLog.Errorf("SM policy decision rule [%s] validation failure, authorised session ambr missing", name)
			return fmt.Errorf("authorised session ambr missing")
		}

		if rule.AuthDefQos == nil {
			logger.ConsumerLog.Errorf("SM policy decision rule [%s] validation failure, authorised default qos missing", name)
			return fmt.Errorf("authorised default qos missing")
		}
	}
	return nil
}
