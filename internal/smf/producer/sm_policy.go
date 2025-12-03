package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf/context"
)

// SendSMPolicyAssociationCreate creates the SM Policy Decision
func SendSMPolicyAssociationCreate(ctx ctxt.Context, smContext *context.SMContext) (*models.SmPolicyDecision, error) {
	nilval := models.DnnConfiguration{}
	if smContext.DnnConfiguration == nilval {
		return nil, fmt.Errorf("DNN configuration is not set for SMContext")
	}

	smPolicyData := models.SmPolicyContextData{}
	smPolicyData.Supi = smContext.Supi
	smPolicyData.Dnn = smContext.Dnn
	smPolicyData.AccessType = smContext.AnType

	smPolicyData.SliceInfo = &models.Snssai{
		Sst: smContext.Snssai.Sst,
		Sd:  smContext.Snssai.Sd,
	}
	smPolicyData.ServingNetwork = &models.PlmnID{
		Mcc: smContext.ServingNetwork.Mcc,
		Mnc: smContext.ServingNetwork.Mnc,
	}
	smPolicyDecision, err := pcf.CreateSMPolicy(ctx, smPolicyData)
	if err != nil {
		return nil, fmt.Errorf("failed to create sm policy decision: %s", err.Error())
	}
	err = validateSmPolicyDecision(smPolicyDecision)
	if err != nil {
		return nil, fmt.Errorf("failed to validate sm policy decision: %s", err.Error())
	}
	return smPolicyDecision, nil
}

func validateSmPolicyDecision(smPolicy *models.SmPolicyDecision) error {
	// Validate just presence of important IEs as of now
	if smPolicy == nil {
		return fmt.Errorf("sm policy decision is nil")
	}

	if smPolicy.SessRule == nil {
		return fmt.Errorf("session rule missing")
	}

	if smPolicy.SessRule.AuthSessAmbr == nil {
		return fmt.Errorf("authorised session ambr missing")
	}

	if smPolicy.SessRule.AuthDefQos == nil {
		return fmt.Errorf("authorised default qos missing")
	}

	return nil
}
