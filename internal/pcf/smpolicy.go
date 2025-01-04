// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/udr"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

func deepCopySessionRule(src *models.SessionRule) *models.SessionRule {
	if src == nil {
		return nil
	}
	copiedSessionRule := *src
	return &copiedSessionRule
}

func deepCopyPccRule(src *models.PccRule) *models.PccRule {
	if src == nil {
		return nil
	}
	copiedPccRule := *src
	return &copiedPccRule
}

func deepCopyQosData(src *models.QosData) *models.QosData {
	if src == nil {
		return nil
	}
	copiedQosData := *src
	return &copiedQosData
}

func deepCopyTrafficControlData(src *models.TrafficControlData) *models.TrafficControlData {
	if src == nil {
		return nil
	}
	copiedTrafficControlData := *src
	return &copiedTrafficControlData
}

func CreateSMPolicy(request models.SmPolicyContextData) (
	response *models.SmPolicyDecision, err1 error,
) {
	var err error
	logger.PcfLog.Debugf("Handle Create SM Policy Request")

	if request.Supi == "" || request.SliceInfo == nil || len(request.SliceInfo.Sd) != 6 {
		return nil, fmt.Errorf("Errorneous/Missing Mandotory IE")
	}

	var ue *UeContext
	if val, exist := pcfCtx.UePool.Load(request.Supi); exist {
		ue = val.(*UeContext)
	}

	if ue == nil {
		return nil, fmt.Errorf("supi is not supported in PCF")
	}
	var smData *models.SmPolicyData
	smPolicyID := fmt.Sprintf("%s-%d", ue.Supi, request.PduSessionId)
	smPolicyData := ue.SmPolicyData[smPolicyID]
	if smPolicyData == nil || smPolicyData.SmPolicyData == nil {
		smData, err = udr.GetSmPolicyData(ue.Supi)
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
	subscriberPolicy := GetSubscriberPolicy(ue.Supi)
	if subscriberPolicy == nil {
		return nil, fmt.Errorf("can't find subscriber policy")
	}
	logger.PcfLog.Infof("Found an existing policy for subscriber [%s]", ue.Supi)
	if PccPolicy, ok1 := subscriberPolicy.PccPolicy[sliceid]; ok1 {
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

	dnnData := GetSMPolicyDnnData(*smData, request.SliceInfo, request.Dnn)
	if dnnData != nil {
		decision.Online = dnnData.Online
		decision.Offline = dnnData.Offline
		decision.Ipv4Index = dnnData.Ipv4Index
		decision.Ipv6Index = dnnData.Ipv6Index
		// Set Aggregate GBR if exist
		if dnnData.GbrDl != "" {
			var gbrDL float64
			gbrDL, err = ConvertBitRateToKbps(dnnData.GbrDl)
			if err != nil {
				logger.PcfLog.Warnf(err.Error())
			} else {
				smPolicyData.RemainGbrDL = &gbrDL
				logger.PcfLog.Debugf("SM Policy Dnn[%s] Data Aggregate DL GBR[%.2f Kbps]", request.Dnn, gbrDL)
			}
		}
		if dnnData.GbrUl != "" {
			var gbrUL float64
			gbrUL, err = ConvertBitRateToKbps(dnnData.GbrUl)
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
	decision.SuppFeat = pcfCtx.PcfSuppFeats[models.ServiceName_NPCF_SMPOLICYCONTROL].NegotiateWith(requestSuppFeat).String()
	decision.QosFlowUsage = request.QosFlowUsage
	decision.PolicyCtrlReqTriggers = PolicyControlReqTrigToArray(0x40780f)
	smPolicyData.PolicyDecision = &decision
	locationHeader := GetResourceUri(models.ServiceName_NPCF_SMPOLICYCONTROL, smPolicyID)
	logger.PcfLog.Infof("Location Header: %s", locationHeader)
	return &decision, nil
}

func DeleteSMPolicy(smPolicyID string) error {
	ue := pcfCtx.PCFUeFindByPolicyId(smPolicyID)
	if ue == nil || ue.SmPolicyData[smPolicyID] == nil {
		return fmt.Errorf("smPolicyID not found in PCF")
	}

	smPolicy := ue.SmPolicyData[smPolicyID]

	// Unsubscrice UDR
	delete(ue.SmPolicyData, smPolicyID)
	logger.PcfLog.Debugf("SMPolicy smPolicyID[%s] DELETE", smPolicyID)

	// Release related App Session
	for appSessionID := range smPolicy.AppSessions {
		if _, exist := pcfCtx.AppSessionPool.Load(appSessionID); exist {
			pcfCtx.AppSessionPool.Delete(appSessionID)
			logger.PcfLog.Debugf("SMPolicy[%s] DELETE Related AppSession[%s]", smPolicyID, appSessionID)
		}
	}
	return nil
}
