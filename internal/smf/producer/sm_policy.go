package producer

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/util"
)

// SendSMPolicyAssociationCreate creates the SM Policy Decision
func SendSMPolicyAssociationCreate(smContext *context.SMContext) (*models.SmPolicyDecision, error) {
	smPolicyData := models.SmPolicyContextData{}
	smPolicyData.Supi = smContext.Supi
	smPolicyData.PduSessionID = smContext.PDUSessionID
	smPolicyData.Dnn = smContext.Dnn
	smPolicyData.PduSessionType = util.PDUSessionTypeToModels(smContext.SelectedPDUSessionType)
	smPolicyData.AccessType = smContext.AnType
	smPolicyData.RatType = smContext.RatType
	smPolicyData.IPv4Address = smContext.PDUAddress.IP.To4().String()
	smPolicyData.SubsSessAmbr = &models.Ambr{
		Uplink:   smContext.DnnConfiguration.SessionAmbr.Uplink,
		Downlink: smContext.DnnConfiguration.SessionAmbr.Downlink,
	}
	smPolicyData.SubsDefQos = &models.SubscribedDefaultQos{
		Arp: &models.Arp{
			PriorityLevel: smContext.DnnConfiguration.Var5gQosProfile.Arp.PriorityLevel,
			PreemptCap:    smContext.DnnConfiguration.Var5gQosProfile.Arp.PreemptCap,
			PreemptVuln:   smContext.DnnConfiguration.Var5gQosProfile.Arp.PreemptVuln,
		},
	}
	smPolicyData.SliceInfo = &models.Snssai{
		Sst: smContext.Snssai.Sst,
		Sd:  smContext.Snssai.Sd,
	}
	smPolicyData.ServingNetwork = &models.PlmnID{
		Mcc: smContext.ServingNetwork.Mcc,
		Mnc: smContext.ServingNetwork.Mnc,
	}

	smPolicyDecision, err := pcf.CreateSMPolicy(smPolicyData)
	if err != nil {
		return nil, fmt.Errorf("failed to create sm policy decision: %s", err.Error())
	}
	err = validateSmPolicyDecision(smPolicyDecision)
	if err != nil {
		return nil, fmt.Errorf("failed to validate sm policy decision: %s", err.Error())
	}
	return smPolicyDecision, nil
}

func SendSMPolicyAssociationDelete(supi string, pduSessionID int32) error {
	smPolicyID := fmt.Sprintf("%s-%d", supi, pduSessionID)
	err := pcf.DeleteSMPolicy(smPolicyID)
	if err != nil {
		return fmt.Errorf("smf policy delete failed, [%v] ", err.Error())
	}
	return nil
}

func validateSmPolicyDecision(smPolicy *models.SmPolicyDecision) error {
	// Validate just presence of important IEs as of now
	// Sess Rules
	for _, rule := range smPolicy.SessRules {
		if rule.AuthSessAmbr == nil {
			return fmt.Errorf("authorised session ambr missing")
		}

		if rule.AuthDefQos == nil {
			return fmt.Errorf("authorised default qos missing")
		}
	}
	return nil
}
