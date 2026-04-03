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

	slices, err := amf.DBInstance.ListAllNetworkSlices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list network slices: %w", err)
	}

	var snssai *models.Snssai

	if len(slices) > 0 {
		sd := ""
		if slices[0].Sd != nil {
			sd = *slices[0].Sd
		}

		snssai = &models.Snssai{
			Sst: slices[0].Sst,
			Sd:  sd,
		}
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
			SNssai: snssai,
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

func (amf *AMF) GetSubscriberDnn(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai) (string, error) {
	if snssai == nil {
		return "", fmt.Errorf("snssai is nil")
	}

	ctx, span := tracer.Start(ctx, "amf/get_subscriber_dnn",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
			attribute.Int("sst", int(snssai.Sst)),
			attribute.String("sd", snssai.Sd),
		),
	)
	defer span.End()

	imsi := supi.IMSI()

	subscriber, err := amf.DBInstance.GetSubscriber(ctx, imsi)
	if err != nil {
		return "", fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}

	slices, err := amf.DBInstance.ListAllNetworkSlices(ctx)
	if err != nil {
		return "", fmt.Errorf("couldn't list network slices: %v", err)
	}

	sliceID := -1

	for _, s := range slices {
		if s.Sst != snssai.Sst {
			continue
		}

		sliceSd := ""
		if s.Sd != nil {
			sliceSd = *s.Sd
		}

		if sliceSd == snssai.Sd {
			sliceID = s.ID

			break
		}
	}

	if sliceID < 0 {
		return "", fmt.Errorf("no slice matching sst=%d sd=%q", snssai.Sst, snssai.Sd)
	}

	policy, err := amf.DBInstance.GetPolicyByProfileAndSlice(ctx, subscriber.ProfileID, sliceID)
	if err != nil {
		return "", fmt.Errorf("couldn't get policy for profile %d slice %d: %v", subscriber.ProfileID, sliceID, err)
	}

	dataNetwork, err := amf.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return "", fmt.Errorf("couldn't get data network %d: %v", policy.DataNetworkID, err)
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

	subscriber, err := amf.DBInstance.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}

	profile, err := amf.DBInstance.GetProfileByID(ctx, subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
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
