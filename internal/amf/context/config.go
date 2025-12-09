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

func ListAmfRan(page int, perPage int) (int, []AmfRan) {
	amfSelf := AMFSelf()

	ranList := amfSelf.ListAmfRan()

	total := len(ranList)

	startIndex, endIndex := getPaginateIndexes(page, perPage, total)

	ranListPage := ranList[startIndex:endIndex]

	return total, ranListPage
}

type OperatorInfo struct {
	Tais          []models.Tai
	Guami         *models.Guami
	SupportedPLMN *PlmnSupportItem
}

func GetOperatorInfo(ctx context.Context) (*OperatorInfo, error) {
	ctx, span := tracer.Start(ctx, "AMF GetOperatorInfo")
	defer span.End()

	amfSelf := AMFSelf()

	operator, err := amfSelf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}

	operatorInfo := &OperatorInfo{
		Tais: getSupportedTAIs(operator),
		Guami: &models.Guami{
			PlmnID: &models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			AmfID: "cafe00", // To edit
		},
		SupportedPLMN: &PlmnSupportItem{
			PlmnID: models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			SNssai: models.Snssai{
				Sst: operator.Sst,
				Sd:  operator.GetHexSd(),
			},
		},
	}

	return operatorInfo, nil
}

func getSupportedTAIs(operator *db.Operator) []models.Tai {
	tais := make([]models.Tai, 0)

	supportedTacs := operator.GetSupportedTacs()

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

	return tais
}

func SubscriberExists(ctx context.Context, ueID string) bool {
	ctx, span := tracer.Start(ctx, "AMF SubscriberExists",
		trace.WithAttributes(
			attribute.String("supi", ueID),
		),
	)
	defer span.End()

	amfSelf := AMFSelf()

	_, err := amfSelf.DBInstance.GetSubscriber(ctx, ueID)
	return err == nil
}

func GetSubscriberData(ctx context.Context, ueID string) (*models.AmbrRm, string, error) {
	ctx, span := tracer.Start(ctx, "AMF GetSubscriberData",
		trace.WithAttributes(
			attribute.String("supi", ueID),
		),
	)
	defer span.End()

	amfSelf := AMFSelf()

	subscriber, err := amfSelf.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, "", fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}

	policy, err := amfSelf.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, "", fmt.Errorf("couldn't get policy %d: %v", subscriber.PolicyID, err)
	}

	dataNetwork, err := amfSelf.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return nil, "", fmt.Errorf("couldn't get data network %d: %v", policy.DataNetworkID, err)
	}

	bitRate := &models.AmbrRm{
		Downlink: policy.BitrateDownlink,
		Uplink:   policy.BitrateUplink,
	}

	return bitRate, dataNetwork.Name, nil
}
