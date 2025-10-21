// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"context"
	"fmt"
	"strings"
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
	SessionPolicy map[string]*SessionPolicy // dnn is key
}

type PcfSubscriberPolicyData struct {
	PccPolicy map[string]*PccPolicy // sst+sd is key
	Supi      string
}

// Allocate PCF Ue with supi and add to pcf Context and returns allocated ue
func (c *PCFContext) NewPCFUe(Supi string) (*UeContext, error) {
	newUeContext := &UeContext{}
	newUeContext.AMPolicyData = make(map[string]*UeAMPolicyData)
	newUeContext.PolAssociationIDGenerator = 1
	newUeContext.Supi = Supi
	c.UePool.Store(Supi, newUeContext)
	return newUeContext, nil
}

// Find PcfUe which the policyId belongs to
func (c *PCFContext) PCFUeFindByPolicyID(PolicyID string) (*UeContext, error) {
	index := strings.LastIndex(PolicyID, "-")
	if index == -1 {
		return nil, fmt.Errorf("invalid policy ID format: %s", PolicyID)
	}
	supi := PolicyID[:index]
	if value, ok := c.UePool.Load(supi); ok {
		return value.(*UeContext), nil
	}
	return nil, fmt.Errorf("ue not found in PCF for policy association ID: %s", PolicyID)
}

func GetSubscriberPolicy(ctx context.Context, imsi string) (*PcfSubscriberPolicyData, error) {
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

	operator, err := pcfCtx.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %w", err)
	}

	subscriberPolicies := &PcfSubscriberPolicyData{
		Supi:      imsi,
		PccPolicy: make(map[string]*PccPolicy),
	}

	pccPolicyID := fmt.Sprintf("%d%s", operator.Sst, operator.GetHexSd())

	if _, exists := subscriberPolicies.PccPolicy[pccPolicyID]; !exists {
		subscriberPolicies.PccPolicy[pccPolicyID] = &PccPolicy{
			SessionPolicy: make(map[string]*SessionPolicy),
		}
	}

	if _, exists := subscriberPolicies.PccPolicy[pccPolicyID].SessionPolicy[dataNetwork.Name]; !exists {
		subscriberPolicies.PccPolicy[pccPolicyID].SessionPolicy[dataNetwork.Name] = &SessionPolicy{
			SessionRule: &models.SessionRule{},
		}
	}

	// Create QoS data
	qosData := &models.QosData{
		Var5qi:               policy.Var5qi,
		Arp:                  &models.Arp{PriorityLevel: policy.Arp},
		DefQosFlowIndication: true,
	}
	subscriberPolicies.PccPolicy[pccPolicyID].QosDecs = qosData

	// Add session rule
	subscriberPolicies.PccPolicy[pccPolicyID].SessionPolicy[dataNetwork.Name].SessionRule = &models.SessionRule{
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: qosData.Var5qi,
			Arp:    qosData.Arp,
		},
		AuthSessAmbr: &models.Ambr{
			Uplink:   policy.BitrateUplink,
			Downlink: policy.BitrateDownlink,
		},
	}
	return subscriberPolicies, nil
}
