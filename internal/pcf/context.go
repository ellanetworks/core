// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/idgenerator"
)

var pcfCtx *PCFContext

type PCFContext struct {
	UePool                 sync.Map
	SessionRuleIDGenerator *idgenerator.IDGenerator
	QoSDataIDGenerator     *idgenerator.IDGenerator
	DBInstance             *db.Database
}

type SessionPolicy struct {
	SessionRules map[string]*models.SessionRule
}

type PccPolicy struct {
	PccRules      map[string]*models.PccRule
	QosDecs       map[string]*models.QosData
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
	profile, err := pcfCtx.DBInstance.GetProfileByID(ctx, subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile %d: %w", subscriber.ProfileID, err)
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
			PccRules:      make(map[string]*models.PccRule),
			QosDecs:       make(map[string]*models.QosData),
		}
	}

	if _, exists := subscriberPolicies.PccPolicy[pccPolicyID].SessionPolicy[config.DNN]; !exists {
		subscriberPolicies.PccPolicy[pccPolicyID].SessionPolicy[config.DNN] = &SessionPolicy{
			SessionRules: make(map[string]*models.SessionRule),
		}
	}

	// Generate IDs using ID generators
	qosID, _ := pcfCtx.QoSDataIDGenerator.Allocate()
	sessionRuleID, _ := pcfCtx.SessionRuleIDGenerator.Allocate()

	// Create QoS data
	qosData := &models.QosData{
		QosID:                strconv.FormatInt(qosID, 10),
		Var5qi:               profile.Var5qi,
		MaxbrUl:              profile.BitrateUplink,
		MaxbrDl:              profile.BitrateDownlink,
		Arp:                  &models.Arp{PriorityLevel: profile.PriorityLevel},
		DefQosFlowIndication: true,
	}
	subscriberPolicies.PccPolicy[pccPolicyID].QosDecs[qosData.QosID] = qosData

	// Add session rule
	subscriberPolicies.PccPolicy[pccPolicyID].SessionPolicy[config.DNN].SessionRules[strconv.FormatInt(sessionRuleID, 10)] = &models.SessionRule{
		SessRuleID: strconv.FormatInt(sessionRuleID, 10),
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: qosData.Var5qi,
			Arp:    qosData.Arp,
		},
		AuthSessAmbr: &models.Ambr{
			Uplink:   profile.BitrateUplink,
			Downlink: profile.BitrateDownlink,
		},
	}
	return subscriberPolicies, nil
}
