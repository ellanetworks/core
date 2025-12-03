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

type SessionPolicy struct {
	SessionRule *models.SessionRule
}

type PccPolicy struct {
	QosDecs       *models.QosData
	SessionPolicy *SessionPolicy
}

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

func CreateSMPolicy(ctx context.Context, request models.SmPolicyContextData) (*models.SmPolicyDecision, error) {
	ctx, span := tracer.Start(ctx, "PCF Create SMPolicy")
	span.SetAttributes(
		attribute.String("ue.supi", request.Supi),
	)

	if request.Supi == "" || request.SliceInfo == nil || request.Dnn == "" {
		return nil, fmt.Errorf("Errorneous/Missing Mandotory IE")
	}

	var ue *UeContext

	if val, exist := pcfCtx.UePool.Load(request.Supi); exist {
		ue = val.(*UeContext)
	}

	if ue == nil {
		return nil, fmt.Errorf("supi is not supported in PCF")
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

	decision := &models.SmPolicyDecision{
		SessRule: deepCopySessionRule(subscriberPolicy.SessionPolicy.SessionRule),
		QosDecs:  deepCopyQosData(subscriberPolicy.QosDecs),
	}

	return decision, nil
}

func GetSubscriberPolicy(ctx context.Context, imsi string, sst int32, sd string, dnn string) (*PccPolicy, error) {
	subscriber, err := pcfCtx.DBInstance.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber %s: %w", imsi, err)
	}

	policy, err := pcfCtx.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy %d: %w", subscriber.PolicyID, err)
	}

	dataNetwork, err := pcfCtx.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get data network %d: %w", policy.DataNetworkID, err)
	}

	if dataNetwork.Name != dnn {
		return nil, fmt.Errorf("subscriber %s has no policy for dnn %s", imsi, dnn)
	}

	operator, err := pcfCtx.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %w", err)
	}

	if operator.Sst != sst || operator.GetHexSd() != sd {
		return nil, fmt.Errorf("subscriber %s has no policy for slice sst: %d sd: %s", imsi, sst, sd)
	}

	subscriberPolicy := &PccPolicy{
		SessionPolicy: &SessionPolicy{
			SessionRule: &models.SessionRule{
				AuthDefQos: &models.AuthorizedDefaultQos{
					Var5qi: policy.Var5qi,
					Arp:    &models.Arp{PriorityLevel: policy.Arp},
				},
				AuthSessAmbr: &models.Ambr{
					Uplink:   policy.BitrateUplink,
					Downlink: policy.BitrateDownlink,
				},
			},
		},
		QosDecs: &models.QosData{
			Var5qi:               policy.Var5qi,
			Arp:                  &models.Arp{PriorityLevel: policy.Arp},
			DefQosFlowIndication: true,
		},
	}

	return subscriberPolicy, nil
}
