// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/attribute"
)

func deepCopySessionRule(src *models.SessionRule) *models.SessionRule {
	if src == nil {
		return nil
	}
	copiedSessionRule := *src
	return &copiedSessionRule
}

func deepCopyQosData(src *models.QosData) *models.QosData {
	if src == nil {
		return nil
	}
	copiedQosData := *src
	return &copiedQosData
}

func GetSmPolicyData(ctx context.Context, supi string) (*models.SmPolicyData, error) {
	operator, err := pcfCtx.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}
	subscriber, err := pcfCtx.DBInstance.GetSubscriber(ctx, supi)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %s", err)
	}
	policy, err := pcfCtx.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %s", err)
	}
	dataNetwork, err := pcfCtx.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get data network: %s", err)
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
	smPolicySnssaiData.SmPolicyDnnData[dataNetwork.Name] = models.SmPolicyDnnData{
		Dnn: dataNetwork.Name,
	}
	smPolicyData.SmPolicySnssaiData[snssai] = smPolicySnssaiData
	return smPolicyData, nil
}

func CreateSMPolicy(ctx context.Context, request models.SmPolicyContextData) (*models.SmPolicyDecision, error) {
	ctx, span := tracer.Start(ctx, "PCF Create SMPolicy")
	span.SetAttributes(
		attribute.String("ue.supi", request.Supi),
	)

	if request.Supi == "" || request.SliceInfo == nil {
		return nil, fmt.Errorf("Errorneous/Missing Mandotory IE")
	}

	var ue *UeContext
	if val, exist := pcfCtx.UePool.Load(request.Supi); exist {
		ue = val.(*UeContext)
	}

	if ue == nil {
		return nil, fmt.Errorf("supi is not supported in PCF")
	}

	smData, err := GetSmPolicyData(ctx, request.Supi)
	if err != nil {
		return nil, fmt.Errorf("can't find UE SM Policy Data in UDR: %s", ue.Supi)
	}

	amPolicy := ue.FindAMPolicy(request.AccessType, request.ServingNetwork)
	if amPolicy == nil {
		return nil, fmt.Errorf("can't find corresponding AM Policy")
	}

	subscriberPolicy, err := GetSubscriberPolicy(ctx, ue.Supi, request.SliceInfo.Sst, request.SliceInfo.Sd, request.Dnn)
	if err != nil {
		return nil, fmt.Errorf("can't find subscriber policy for subscriber %s: %s", ue.Supi, err)
	}

	if subscriberPolicy == nil {
		return nil, fmt.Errorf("subscriber policy is nil for subscriber %s", ue.Supi)
	}

	dnnData, err := GetSMPolicyDnnData(*smData, request.SliceInfo, request.Dnn)
	if err != nil {
		return nil, fmt.Errorf("error finding SM Policy DNN Data for dnn %s: %s", request.Dnn, err)
	}

	if dnnData == nil {
		return nil, fmt.Errorf("SM Policy DNN Data is empty for dnn %s", request.Dnn)
	}

	decision := &models.SmPolicyDecision{
		SessRule: deepCopySessionRule(subscriberPolicy.PccPolicy.SessionPolicy.SessionRule),
		QosDecs:  deepCopyQosData(subscriberPolicy.PccPolicy.QosDecs),
	}

	return decision, nil
}
