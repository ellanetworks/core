package producer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/util"
	"github.com/yeastengine/ella/internal/udr/producer"
)

func deepCopySessionRule(src *models.SessionRule) *models.SessionRule {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}

func deepCopyPccRule(src *models.PccRule) *models.PccRule {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}

func deepCopyQosData(src *models.QosData) *models.QosData {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}

func deepCopyTrafficControlData(src *models.TrafficControlData) *models.TrafficControlData {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}

func CreateSMPolicy(request models.SmPolicyContextData) (
	response *models.SmPolicyDecision, err1 error,
) {
	var err error
	logger.PcfLog.Debugf("Handle Create SM Policy Request")

	if request.Supi == "" || request.SliceInfo == nil || len(request.SliceInfo.Sd) != 6 {
		return nil, fmt.Errorf("Errorneous/Missing Mandotory IE")
	}

	pcfSelf := context.PCF_Self()
	var ue *context.UeContext
	if val, exist := pcfSelf.UePool.Load(request.Supi); exist {
		ue = val.(*context.UeContext)
	}

	if ue == nil {
		return nil, fmt.Errorf("supi is not supported in PCF")
	}
	var smData *models.SmPolicyData
	smPolicyID := fmt.Sprintf("%s-%d", ue.Supi, request.PduSessionId)
	smPolicyData := ue.SmPolicyData[smPolicyID]
	if smPolicyData == nil || smPolicyData.SmPolicyData == nil {
		smData, err = producer.GetSmPolicyData(ue.Supi)
		if err != nil {
			return nil, fmt.Errorf("Can't find UE SM Policy Data in UDR: %s", ue.Supi)
		}
	} else {
		smData = smPolicyData.SmPolicyData
	}
	amPolicy := ue.FindAMPolicy(request.AccessType, request.ServingNetwork)
	if amPolicy == nil {
		return nil, fmt.Errorf("Can't find corresponding AM Policy")
	}
	// TODO: check service restrict
	if ue.Gpsi == "" {
		ue.Gpsi = request.Gpsi
	}
	if ue.Pei == "" {
		ue.Pei = request.Pei
	}
	if smPolicyData != nil {
		delete(ue.SmPolicyData, smPolicyID)
	}
	smPolicyData = ue.NewUeSmPolicyData(smPolicyID, request, smData)
	// Policy Decision
	decision := models.SmPolicyDecision{
		SessRules:     make(map[string]*models.SessionRule),
		PccRules:      make(map[string]*models.PccRule),
		QosDecs:       make(map[string]*models.QosData),
		TraffContDecs: make(map[string]*models.TrafficControlData),
	}

	sstStr := strconv.Itoa(int(request.SliceInfo.Sst))
	sliceid := sstStr + request.SliceInfo.Sd
	imsi := strings.TrimPrefix(ue.Supi, "imsi-")
	subscriberPolicies := context.GetSubscriberPolicies()
	if subsPolicyData, ok := subscriberPolicies[imsi]; ok {
		logger.PcfLog.Infof("Found an existing policy for subscriber [%s]", imsi)
		if PccPolicy, ok1 := subsPolicyData.PccPolicy[sliceid]; ok1 {
			if sessPolicy, exist := PccPolicy.SessionPolicy[request.Dnn]; exist {
				for _, sessRule := range sessPolicy.SessionRules {
					decision.SessRules[sessRule.SessRuleId] = deepCopySessionRule(sessRule)
				}
			} else {
				return nil, fmt.Errorf("can't find local policy")
			}

			for key, pccRule := range PccPolicy.PccRules {
				decision.PccRules[key] = deepCopyPccRule(pccRule)
			}

			for key, qosData := range PccPolicy.QosDecs {
				decision.QosDecs[key] = deepCopyQosData(qosData)
			}
			for key, trafficData := range PccPolicy.TraffContDecs {
				decision.TraffContDecs[key] = deepCopyTrafficControlData(trafficData)
			}
		} else {
			return nil, fmt.Errorf("can't find local policy")
		}
	} else {
		return nil, fmt.Errorf("can't find UE in local policy: %s", ue.Supi)
	}

	dnnData := util.GetSMPolicyDnnData(*smData, request.SliceInfo, request.Dnn)
	if dnnData != nil {
		decision.Online = dnnData.Online
		decision.Offline = dnnData.Offline
		decision.Ipv4Index = dnnData.Ipv4Index
		decision.Ipv6Index = dnnData.Ipv6Index
		// Set Aggregate GBR if exist
		if dnnData.GbrDl != "" {
			var gbrDL float64
			gbrDL, err = context.ConvertBitRateToKbps(dnnData.GbrDl)
			if err != nil {
				logger.PcfLog.Warnf(err.Error())
			} else {
				smPolicyData.RemainGbrDL = &gbrDL
				logger.PcfLog.Debugf("SM Policy Dnn[%s] Data Aggregate DL GBR[%.2f Kbps]", request.Dnn, gbrDL)
			}
		}
		if dnnData.GbrUl != "" {
			var gbrUL float64
			gbrUL, err = context.ConvertBitRateToKbps(dnnData.GbrUl)
			if err != nil {
				logger.PcfLog.Warnf(err.Error())
			} else {
				smPolicyData.RemainGbrUL = &gbrUL
				logger.PcfLog.Debugf("SM Policy Dnn[%s] Data Aggregate UL GBR[%.2f Kbps]", request.Dnn, gbrUL)
			}
		}
	} else {
		logger.PcfLog.Warnf(
			"Policy Subscription Info: SMPolicyDnnData is null for dnn[%s] in UE[%s]", request.Dnn, ue.Supi)
		decision.Online = request.Online
		decision.Offline = request.Offline
	}

	requestSuppFeat, err := openapi.NewSupportedFeature(request.SuppFeat)
	if err != nil {
		logger.PcfLog.Errorf("openapi NewSupportedFeature error: %+v", err)
	}
	decision.SuppFeat = pcfSelf.PcfSuppFeats[models.ServiceName_NPCF_SMPOLICYCONTROL].NegotiateWith(requestSuppFeat).String()
	decision.QosFlowUsage = request.QosFlowUsage
	// TODO: Trigger about UMC, ADC, NetLoc,...
	decision.PolicyCtrlReqTriggers = util.PolicyControlReqTrigToArray(0x40780f)
	smPolicyData.PolicyDecision = &decision
	// TODO: PCC rule, PraInfo ...
	locationHeader := util.GetResourceUri(models.ServiceName_NPCF_SMPOLICYCONTROL, smPolicyID)
	logger.PcfLog.Infof("Location Header: %s", locationHeader)
	return &decision, nil
}

func DeleteSMPolicy(smPolicyID string) error {
	ue := context.PCF_Self().PCFUeFindByPolicyId(smPolicyID)
	if ue == nil || ue.SmPolicyData[smPolicyID] == nil {
		return fmt.Errorf("smPolicyID not found in PCF")
	}

	pcfSelf := context.PCF_Self()
	smPolicy := ue.SmPolicyData[smPolicyID]

	// Unsubscrice UDR
	delete(ue.SmPolicyData, smPolicyID)
	logger.PcfLog.Debugf("SMPolicy smPolicyID[%s] DELETE", smPolicyID)

	// Release related App Session
	terminationInfo := models.TerminationInfo{
		TermCause: models.TerminationCause_PDU_SESSION_TERMINATION,
	}
	for appSessionID := range smPolicy.AppSessions {
		if val, exist := pcfSelf.AppSessionPool.Load(appSessionID); exist {
			appSession := val.(*context.AppSessionData)
			SendAppSessionTermination(appSession, terminationInfo)
			pcfSelf.AppSessionPool.Delete(appSessionID)
			logger.PcfLog.Debugf("SMPolicy[%s] DELETE Related AppSession[%s]", smPolicyID, appSessionID)
		}
	}
	return nil
}
