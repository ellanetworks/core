// Copyright 2024 Ella Networks
package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/security"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/amf")

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

func (amf *AMF) ListAmfRan(page int, perPage int) (int, []*Radio) {
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
	ctx, span := tracer.Start(ctx, "amf/get_operator_info")
	defer span.End()

	operator, err := amf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}

	supportedTAIs, err := getSupportedTAIs(operator)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported TAIs: %w", err)
	}

	slices, err := amf.DBInstance.ListNetworkSlices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list network slices: %w", err)
	}

	if len(slices) == 0 {
		return nil, fmt.Errorf("no network slices configured")
	}

	slice := slices[0]

	var sd string
	if slice.Sd != nil {
		sd = *slice.Sd
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
				Sst: slice.Sst,
				Sd:  sd,
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

func (amf *AMF) GetSubscriberDnn(ctx context.Context, supi etsi.SUPI) (string, error) {
	ctx, span := tracer.Start(ctx, "amf/get_subscriber_dnn",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	imsi := supi.IMSI()

	subscriber, err := amf.DBInstance.GetSubscriber(ctx, imsi)
	if err != nil {
		return "", fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}

	configs, err := amf.DBInstance.ListProfileNetworkConfigs(ctx, subscriber.ProfileID)
	if err != nil {
		return "", fmt.Errorf("couldn't get profile network configs for profile %d: %v", subscriber.ProfileID, err)
	}

	if len(configs) == 0 {
		return "", fmt.Errorf("no network configs found for profile %d", subscriber.ProfileID)
	}

	dataNetwork, err := amf.DBInstance.GetDataNetworkByID(ctx, configs[0].DataNetworkID)
	if err != nil {
		return "", fmt.Errorf("couldn't get data network %d: %v", configs[0].DataNetworkID, err)
	}

	return dataNetwork.Name, nil
}

func (amf *AMF) GetSubscriberBitrate(ctx context.Context, supi etsi.SUPI) (*models.Ambr, error) {
	ctx, span := tracer.Start(ctx, "amf/get_subscriber_bitrate",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	imsi := supi.IMSI()

	profile, err := amf.DBInstance.GetSubscriberProfile(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile for subscriber %s: %v", imsi, err)
	}

	return &models.Ambr{
		Downlink: profile.UeAmbrDownlink,
		Uplink:   profile.UeAmbrUplink,
	}, nil
}

var cipheringNameToAlg = map[string]uint8{
	"NEA0": security.AlgCiphering128NEA0,
	"NEA1": security.AlgCiphering128NEA1,
	"NEA2": security.AlgCiphering128NEA2,
}

var integrityNameToAlg = map[string]uint8{
	"NIA0": security.AlgIntegrity128NIA0,
	"NIA1": security.AlgIntegrity128NIA1,
	"NIA2": security.AlgIntegrity128NIA2,
}

// GetSecurityAlgorithms loads the configured NAS security algorithm preference
// order from the database and returns them as uint8 slices ready for
// SelectSecurityAlg.
func (amf *AMF) GetSecurityAlgorithms(ctx context.Context) ([]uint8, []uint8, error) {
	ctx, span := tracer.Start(ctx, "amf/get_security_algorithms")
	defer span.End()

	operator, err := amf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get operator: %w", err)
	}

	cipherNames, err := operator.GetCiphering()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ciphering order: %w", err)
	}

	integrityNames, err := operator.GetIntegrity()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse integrity order: %w", err)
	}

	encOrder := make([]uint8, 0, len(cipherNames))
	for _, name := range cipherNames {
		alg, ok := cipheringNameToAlg[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown ciphering algorithm: %s", name)
		}

		encOrder = append(encOrder, alg)
	}

	intOrder := make([]uint8, 0, len(integrityNames))
	for _, name := range integrityNames {
		alg, ok := integrityNameToAlg[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown integrity algorithm: %s", name)
		}

		intOrder = append(intOrder, alg)
	}

	return intOrder, encOrder, nil
}
