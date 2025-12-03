// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"context"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

var pcfCtx *PCFContext

type PCFContext struct {
	UePool     sync.Map
	DBInstance *db.Database
}

type SessionPolicy struct {
	SessionRule *models.SessionRule
}

type PccPolicy struct {
	QosDecs       *models.QosData
	SessionPolicy *SessionPolicy
}

type PcfSubscriberPolicyData struct {
	PccPolicy *PccPolicy
	Supi      string
}

// Allocate PCF Ue with supi and add to pcf Context and returns allocated ue
func (c *PCFContext) NewPCFUe(Supi string) (*UeContext, error) {
	newUeContext := &UeContext{}
	newUeContext.Supi = Supi
	c.UePool.Store(Supi, newUeContext)
	return newUeContext, nil
}

// Find PcfUe which the policyId belongs to
func (c *PCFContext) FindUEBySUPI(supi string) (*UeContext, error) {
	if value, ok := c.UePool.Load(supi); ok {
		return value.(*UeContext), nil
	}

	return nil, fmt.Errorf("ue not found in PCF for supi: %s", supi)
}

func GetSubscriberPolicy(ctx context.Context, imsi string, sst int32, sd string, dnn string) (*PcfSubscriberPolicyData, error) {
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

	subscriberPolicies := &PcfSubscriberPolicyData{
		Supi: imsi,
		PccPolicy: &PccPolicy{
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
		},
	}

	return subscriberPolicies, nil
}
