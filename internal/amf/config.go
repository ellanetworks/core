// Copyright 2024 Ella Networks
package amf

import (
	"context"
	"fmt"
	"sort"

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
	Tais  []models.Tai
	Guami *models.Guami
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

	// 3GPP TS 23.003 §2.10.1: AMF Identifier = <AMF Region ID><AMF Set ID><AMF Pointer>
	amfID := fmt.Sprintf("%06x", (operator.AmfRegionID<<16)|(operator.AmfSetID<<6)|amf.DBInstance.NodeID())

	operatorInfo := &OperatorInfo{
		Tais: supportedTAIs,
		Guami: &models.Guami{
			PlmnID: &models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			AmfID: amfID,
		},
	}

	return operatorInfo, nil
}

func (amf *AMF) ListOperatorSnssai(ctx context.Context) ([]models.Snssai, error) {
	ctx, span := tracer.Start(ctx, "amf/list_operator_snssai")
	defer span.End()

	slices, err := amf.DBInstance.ListAllNetworkSlices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list network slices: %w", err)
	}

	snssaiList := make([]models.Snssai, 0, len(slices))
	for _, s := range slices {
		sd := ""
		if s.Sd != nil {
			sd = *s.Sd
		}

		snssaiList = append(snssaiList, models.Snssai{
			Sst: s.Sst,
			Sd:  sd,
		})
	}

	return snssaiList, nil
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

// SubscriberProfile holds the per-subscriber session configuration
// derived from the subscriber's profile: allowed network slices and bitrate.
type SubscriberProfile struct {
	AllowedNssai []models.Snssai
	Ambr         *models.Ambr
}

func (amf *AMF) GetSubscriberProfile(ctx context.Context, supi etsi.SUPI) (*SubscriberProfile, error) {
	ctx, span := tracer.Start(ctx, "amf/get_subscriber_profile",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	imsi := supi.IMSI()

	subscriber, err := amf.DBInstance.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %w", imsi, err)
	}

	// Derive allowed NSSAI from the subscriber's profile policies.
	policies, err := amf.DBInstance.ListPoliciesByProfile(ctx, subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't list policies for profile %d: %w", subscriber.ProfileID, err)
	}

	// Collect unique slice IDs and batch-fetch.
	sliceIDSet := make(map[int]struct{})
	for _, p := range policies {
		sliceIDSet[p.SliceID] = struct{}{}
	}

	sliceIDs := make([]int, 0, len(sliceIDSet))
	for id := range sliceIDSet {
		sliceIDs = append(sliceIDs, id)
	}

	sort.Ints(sliceIDs)

	slices, err := amf.DBInstance.ListNetworkSlicesByIDs(ctx, sliceIDs)
	if err != nil {
		return nil, fmt.Errorf("couldn't list slices by IDs: %w", err)
	}

	var allowedNssai []models.Snssai

	for _, slice := range slices {
		sd := ""
		if slice.Sd != nil {
			sd = *slice.Sd
		}

		allowedNssai = append(allowedNssai, models.Snssai{
			Sst: slice.Sst,
			Sd:  sd,
		})
	}

	// Derive bitrate from the subscriber's profile.
	profile, err := amf.DBInstance.GetProfileByID(ctx, subscriber.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get profile %d: %v", subscriber.ProfileID, err)
	}

	return &SubscriberProfile{
		AllowedNssai: allowedNssai,
		Ambr: &models.Ambr{
			Downlink: profile.UeAmbrDownlink,
			Uplink:   profile.UeAmbrUplink,
		},
	}, nil
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

	policies, err := amf.DBInstance.ListPoliciesByProfile(ctx, subscriber.ProfileID)
	if err != nil {
		return "", fmt.Errorf("couldn't list policies for profile %d: %v", subscriber.ProfileID, err)
	}

	// Batch-fetch all referenced network slices.
	sliceIDSet := make(map[int]struct{})
	for _, p := range policies {
		sliceIDSet[p.SliceID] = struct{}{}
	}

	sliceIDs := make([]int, 0, len(sliceIDSet))
	for id := range sliceIDSet {
		sliceIDs = append(sliceIDs, id)
	}

	sort.Ints(sliceIDs)

	sliceList, err := amf.DBInstance.ListNetworkSlicesByIDs(ctx, sliceIDs)
	if err != nil {
		return "", fmt.Errorf("couldn't list slices by IDs: %v", err)
	}

	sliceMap := make(map[int]db.NetworkSlice, len(sliceList))
	for _, s := range sliceList {
		sliceMap[s.ID] = s
	}

	for _, p := range policies {
		slice, ok := sliceMap[p.SliceID]
		if !ok {
			continue
		}

		sliceSd := ""
		if slice.Sd != nil {
			sliceSd = *slice.Sd
		}

		if slice.Sst == snssai.Sst && sliceSd == snssai.Sd {
			dataNetwork, err := amf.DBInstance.GetDataNetworkByID(ctx, p.DataNetworkID)
			if err != nil {
				return "", fmt.Errorf("couldn't get data network %d: %v", p.DataNetworkID, err)
			}

			return dataNetwork.Name, nil
		}
	}

	return "", fmt.Errorf("no policy matching sst=%d sd=%q for profile %d", snssai.Sst, snssai.Sd, subscriber.ProfileID)
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
