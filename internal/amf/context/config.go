// Copyright 2024 Ella Networks
package context

import (
	"context"
	"fmt"

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

func GetSupportTaiList(ctx context.Context) ([]models.Tai, error) {
	ctx, span := tracer.Start(ctx, "AMF GetSupportTaiList")
	defer span.End()

	amfSelf := AMFSelf()

	dbNetwork, err := amfSelf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}

	tais := make([]models.Tai, 0)

	supportedTacs := dbNetwork.GetSupportedTacs()

	for _, tac := range supportedTacs {
		tai := models.Tai{
			PlmnID: &models.PlmnID{
				Mcc: dbNetwork.Mcc,
				Mnc: dbNetwork.Mnc,
			},
			Tac: tac,
		}
		tais = append(tais, tai)
	}
	return tais, nil
}

func GetServedGuami(ctx context.Context) (*models.Guami, error) {
	ctx, span := tracer.Start(ctx, "AMF GetServedGuami")
	defer span.End()

	amfSelf := AMFSelf()

	dbNetwork, err := amfSelf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}

	return &models.Guami{
		PlmnID: &models.PlmnID{
			Mcc: dbNetwork.Mcc,
			Mnc: dbNetwork.Mnc,
		},
		AmfID: "cafe00", // To edit
	}, nil
}

func GetSupportedPlmn(ctx context.Context) (*PlmnSupportItem, error) {
	ctx, span := tracer.Start(ctx, "AMF GetSupportedPlmn")
	defer span.End()

	amfSelf := AMFSelf()

	operator, err := amfSelf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get operator: %v", err)
	}

	plmnSupportItem := &PlmnSupportItem{
		PlmnID: models.PlmnID{
			Mcc: operator.Mcc,
			Mnc: operator.Mnc,
		},
		SNssai: models.Snssai{
			Sst: operator.Sst,
			Sd:  operator.GetHexSd(),
		},
	}
	return plmnSupportItem, nil
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
