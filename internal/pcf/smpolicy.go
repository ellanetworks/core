// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/models"
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

func GetSmPolicyData() (*models.SmPolicyData, error) {
	operator, err := pcfCtx.DBInstance.GetOperator()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}
	smPolicyData := &models.SmPolicyData{
		SmPolicySnssaiData: make(map[string]models.SmPolicySnssaiData),
	}
	snssai := fmt.Sprintf("%d%s", operator.Sst, operator.GetHexSd())
	smPolicyData.SmPolicySnssaiData[snssai] = models.SmPolicySnssaiData{
		Snssai: &models.Snssai{
			Sd:  operator.GetHexSd(),
			Sst: operator.Sst,
		},
		SmPolicyDnnData: make(map[string]models.SmPolicyDnnData),
	}
	smPolicySnssaiData := smPolicyData.SmPolicySnssaiData[snssai]
	smPolicySnssaiData.SmPolicyDnnData[config.DNN] = models.SmPolicyDnnData{
		Dnn: config.DNN,
	}
	smPolicyData.SmPolicySnssaiData[snssai] = smPolicySnssaiData
	return smPolicyData, nil
}

func CreateSMPolicy(request models.SmPolicyContextData) (*models.SmPolicyDecision, error) {
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
	var err error
	smPolicyID := fmt.Sprintf("%s-%d", ue.Supi, request.PduSessionID)
	smPolicyData := ue.SmPolicyData[smPolicyID]
	if smPolicyData == nil || smPolicyData.SmPolicyData == nil {
		smData, err = GetSmPolicyData()
		if err != nil {
			return nil, fmt.Errorf("can't find UE SM Policy Data in UDR: %s", ue.Supi)
		}
	} else {
		smData = smPolicyData.SmPolicyData
	}
	amPolicy := ue.FindAMPolicy(request.AccessType, request.ServingNetwork)
	if amPolicy == nil {
		return nil, fmt.Errorf("can't find corresponding AM Policy")
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
	_ = ue.NewUeSmPolicyData(smPolicyID, request, smData)
	decision := &models.SmPolicyDecision{
		SessRules:     make(map[string]*models.SessionRule),
		PccRules:      make(map[string]*models.PccRule),
		QosDecs:       make(map[string]*models.QosData),
		TraffContDecs: make(map[string]*models.TrafficControlData),
	}

	sstStr := strconv.Itoa(int(request.SliceInfo.Sst))
	sliceid := sstStr + request.SliceInfo.Sd
	subscriberPolicy, err := GetSubscriberPolicy(ue.Supi)
	if err != nil {
		return nil, fmt.Errorf("can't find subscriber policy for subscriber %s: %s", ue.Supi, err)
	}
	if subscriberPolicy == nil {
		return nil, fmt.Errorf("subscriber policy is nil for subscriber %s", ue.Supi)
	}
	PccPolicy, ok := subscriberPolicy.PccPolicy[sliceid]
	if !ok {
		return nil, fmt.Errorf("can't find PCC policy for slice %s", sliceid)
	}
	sessPolicy, exist := PccPolicy.SessionPolicy[request.Dnn]
	if !exist {
		return nil, fmt.Errorf("can't find session policy for dnn %s", request.Dnn)
	}
	for _, sessRule := range sessPolicy.SessionRules {
		decision.SessRules[sessRule.SessRuleID] = deepCopySessionRule(sessRule)
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

	dnnData, err := GetSMPolicyDnnData(*smData, request.SliceInfo, request.Dnn)
	if err != nil {
		return nil, fmt.Errorf("error finding SM Policy DNN Data for dnn %s", request.Dnn)
	}
	if dnnData == nil {
		return nil, fmt.Errorf("SM Policy DNN Data is empty for dnn %s", request.Dnn)
	}
	if dnnData.GbrDl != "" {
		_, err := ConvertBitRateToKbps(dnnData.GbrDl)
		if err != nil {
			return nil, fmt.Errorf("can't convert GBR DL to Kbps: %s", err)
		}
	}
	if dnnData.GbrUl != "" {
		_, err := ConvertBitRateToKbps(dnnData.GbrUl)
		if err != nil {
			return nil, fmt.Errorf("can't convert GBR UL to Kbps: %s", err)
		}
	}
	return decision, nil
}

func DeleteSMPolicy(smPolicyID string) error {
	ue, err := pcfCtx.PCFUeFindByPolicyID(smPolicyID)
	if err != nil {
		return fmt.Errorf("ue not found in PCF for smPolicyID: %s", smPolicyID)
	}
	if ue == nil {
		return fmt.Errorf("ue not found in PCF for smPolicyID: %s", smPolicyID)
	}
	if ue.SmPolicyData[smPolicyID] == nil {
		return fmt.Errorf("smPolicyID not found in PCF for smPolicyID: %s", smPolicyID)
	}
	delete(ue.SmPolicyData, smPolicyID)
	return nil
}
