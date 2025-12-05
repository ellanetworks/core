// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("ella-core/udm")

var AllowedSessionTypes = []models.PduSessionType{models.PduSessionTypeIPv4}

var AllowedSscModes = []string{
	"SSC_MODE_2",
	"SSC_MODE_3",
}

func GetSubscriberData(ctx context.Context, ueID string) (*models.SubscriberData, error) {
	ctx, span := tracer.Start(ctx, "UDM GetSubscriberData")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", ueID),
	)

	subscriber, err := udmContext.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := udmContext.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	dataNetwork, err := udmContext.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get data network %d: %v", policy.DataNetworkID, err)
	}

	amData := &models.AccessAndMobilitySubscriptionData{
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: policy.BitrateDownlink,
			Uplink:   policy.BitrateUplink,
		},
	}

	subscriberData := &models.SubscriberData{
		AccessAndMobilitySubscriptionData: amData,
		Dnn:                               dataNetwork.Name,
	}

	return subscriberData, nil
}

func GetDnnConfig(ctx context.Context, ueID string) (*models.DnnConfiguration, error) {
	ctx, span := tracer.Start(ctx, "UDM GetSmData")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", ueID),
	)

	subscriber, err := udmContext.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := udmContext.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	dnnConfig := &models.DnnConfiguration{
		PduSessionTypes: &models.PduSessionTypes{
			DefaultSessionType:  models.PduSessionTypeIPv4,
			AllowedSessionTypes: make([]models.PduSessionType, 0),
		},
		SscModes: &models.SscModes{
			DefaultSscMode:  models.SscMode1,
			AllowedSscModes: make([]models.SscMode, 0),
		},
		SessionAmbr: &models.Ambr{
			Downlink: policy.BitrateDownlink,
			Uplink:   policy.BitrateUplink,
		},
		Var5gQosProfile: &models.SubscribedDefaultQos{
			Var5qi: policy.Var5qi,
			Arp:    &models.Arp{PriorityLevel: policy.Arp},
		},
	}

	dnnConfig.PduSessionTypes.AllowedSessionTypes = append(dnnConfig.PduSessionTypes.AllowedSessionTypes, AllowedSessionTypes...)
	for _, sscMode := range AllowedSscModes {
		dnnConfig.SscModes.AllowedSscModes = append(dnnConfig.SscModes.AllowedSscModes, models.SscMode(sscMode))
	}

	return dnnConfig, nil
}

func GetSnssai(ctx context.Context) (*models.Snssai, error) {
	ctx, span := tracer.Start(ctx, "UDM GetNssai")
	defer span.End()

	operator, err := udmContext.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}

	snssai := &models.Snssai{
		Sd:  operator.GetHexSd(),
		Sst: operator.Sst,
	}

	return snssai, nil
}
