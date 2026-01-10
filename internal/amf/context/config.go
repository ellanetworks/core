// Copyright 2024 Ella Networks
package context

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/amf/context")

// This file contains calls to db to get configuration data

func getPaginateIndexes(page int, perPage int, total int) (int, int) {
	startIndex := (page - 1) * perPage

	endIndex := startIndex + perPage

	if startIndex > total {
		return 0, 0
	}

	if endIndex > total {
		endIndex = total
	}

	return startIndex, endIndex
}

func (amf *AMF) ListAmfRan(page int, perPage int) (int, []Radio) {
	radios := amf.ListRadios()

	total := len(radios)

	startIndex, endIndex := getPaginateIndexes(page, perPage, total)

	radioListPage := radios[startIndex:endIndex]

	return total, radioListPage
}

type OperatorInfo struct {
	Tais          []models.Tai
	Guami         *models.Guami
	SupportedPLMN *models.PlmnSupportItem
}

func (amf *AMF) GetOperatorInfo(ctx context.Context) (*OperatorInfo, error) {
	ctx, span := tracer.Start(ctx, "AMF GetOperatorInfo")
	defer span.End()

	operator, err := amf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}

	supportedTAIs, err := getSupportedTAIs(operator)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported TAIs: %w", err)
	}

	operatorInfo := &OperatorInfo{
		Tais: supportedTAIs,
		Guami: &models.Guami{
			PlmnID: &models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			AmfID: "cafe00", // To edit
		},
		SupportedPLMN: &models.PlmnSupportItem{
			PlmnID: models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			SNssai: &models.Snssai{
				Sst: operator.Sst,
				Sd:  operator.GetHexSd(),
			},
		},
	}

	return operatorInfo, nil
}

func getSupportedTAIs(operator *db.Operator) ([]models.Tai, error) {
	supportedTacs, err := operator.GetSupportedTacs()
	if err != nil {
		return nil, fmt.Errorf("failed to get supported TACs: %w", err)
	}

	tais := make([]models.Tai, 0)

	for _, tac := range supportedTacs {
		tai := models.Tai{
			PlmnID: &models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			Tac: tac,
		}
		tais = append(tais, tai)
	}

	return tais, nil
}

func (amf *AMF) GetSubscriberDnn(ctx context.Context, ueID string) (string, error) {
	ctx, span := tracer.Start(ctx, "AMF GetSubscriberData",
		trace.WithAttributes(
			attribute.String("supi", ueID),
		),
	)
	defer span.End()

	subscriber, err := amf.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return "", fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := amf.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return "", fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	dataNetwork, err := amf.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return "", fmt.Errorf("couldn't get data network %d: %v", policy.DataNetworkID, err)
	}

	return dataNetwork.Name, nil
}

func (amf *AMF) GetSubscriberBitrate(ctx context.Context, ueID string) (*models.Ambr, error) {
	ctx, span := tracer.Start(ctx, "AMF GetSubscriberBitrate",
		trace.WithAttributes(
			attribute.String("supi", ueID),
		),
	)
	defer span.End()

	subscriber, err := amf.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := amf.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	bitRate := &models.Ambr{
		Downlink: policy.BitrateDownlink,
		Uplink:   policy.BitrateUplink,
	}

	return bitRate, nil
}
