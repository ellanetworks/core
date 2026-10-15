// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package amf

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/amf")

type OperatorInfo struct {
	Tais  []models.Tai
	Guami *models.Guami
}

func (amf *AMF) OperatorInfo(ctx context.Context) (*OperatorInfo, error) {
	ctx, span := tracer.Start(ctx, "amf/get_operator_info")
	defer span.End()

	operator, err := amf.DBInstance.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator: %s", err)
	}

	return amf.operatorInfoFrom(operator)
}

type OperatorConfig struct {
	op   *db.Operator
	info *OperatorInfo
}

func (amf *AMF) Operator(ctx context.Context) (OperatorConfig, error) {
	ctx, span := tracer.Start(ctx, "amf/get_operator")
	defer span.End()

	operator, err := amf.DBInstance.GetOperator(ctx)
	if err != nil {
		return OperatorConfig{}, fmt.Errorf("failed to get operator: %w", err)
	}

	info, err := amf.operatorInfoFrom(operator)
	if err != nil {
		return OperatorConfig{}, err
	}

	return OperatorConfig{op: operator, info: info}, nil
}

func (o OperatorConfig) Info() *OperatorInfo {
	return o.info
}

func (amf *AMF) operatorInfoFrom(operator *db.Operator) (*OperatorInfo, error) {
	supportedTAIs, err := getSupportedTAIs(operator)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported TAIs: %w", err)
	}

	pointer := amf.DBInstance.AMFPointer()
	if pointer < 1 {
		return nil, fmt.Errorf("this node has no AMF Pointer yet; the leader allocates it into cluster_members on join")
	}

	amfID := util.AMFIDToModels(
		ngap.AMFRegionID(operator.GUAMIRegionID()),
		ngap.AMFSetID(operator.AmfSetID),
		ngap.AMFPointer(pointer),
	)

	return &OperatorInfo{
		Tais: supportedTAIs,
		Guami: &models.Guami{
			PlmnID: &models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			AmfID: amfID,
		},
	}, nil
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

	tais := make([]models.Tai, 0, len(supportedTacs))

	for _, tac := range supportedTacs {
		// Canonicalise the TAC to a single lowercase 6-hex form so it compares equal
		// to the RAN's hex.EncodeToString(TAC) and encodes to NAS; the DB accepts any
		// even/odd-case hex (TS 23.003).
		n, err := strconv.ParseUint(tac, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid TAC %q: %w", tac, err)
		}

		tais = append(tais, models.Tai{
			PlmnID: &models.PlmnID{
				Mcc: operator.Mcc,
				Mnc: operator.Mnc,
			},
			Tac: fmt.Sprintf("%06x", n),
		})
	}

	return tais, nil
}

// SubscriberProfile holds the per-subscriber session configuration
// derived from the subscriber's profile: allowed network slices and bitrate.
type SubscriberProfile struct {
	AllowedNssai []models.Snssai
	Ambr         *models.Ambr
	Allow5G      bool
	Allow4G      bool
}

func (amf *AMF) subscriptions() *udm.Subscriptions {
	return udm.NewSubscriptions(amf.DBInstance, nil)
}

func (amf *AMF) SubscriberProfile(ctx context.Context, supi etsi.SUPI) (*SubscriberProfile, error) {
	ctx, span := tracer.Start(ctx, "amf/get_subscriber_profile",
		trace.WithAttributes(
			attrs.SUPI(supi.String()),
		),
	)
	defer span.End()

	am, err := amf.subscriptions().AccessAndMobility(ctx, supi.IMSI())
	if err != nil {
		return nil, err
	}

	ambr := am.UEAMBR

	return &SubscriberProfile{
		AllowedNssai: am.SubscribedSNSSAIs,
		Ambr:         &ambr,
		Allow5G:      am.Allow5G,
		Allow4G:      am.Allow4G,
	}, nil
}

func (amf *AMF) SubscribedUEAMBR(ctx context.Context, supi etsi.SUPI) (*models.Ambr, error) {
	ambr, err := amf.subscriptions().UEAMBR(ctx, supi.IMSI())
	if err != nil {
		return nil, err
	}

	return &ambr, nil
}

func (amf *AMF) SubscriberDnn(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai) (string, error) {
	if snssai == nil {
		return "", fmt.Errorf("snssai is nil")
	}

	ctx, span := tracer.Start(ctx, "amf/get_subscriber_dnn",
		trace.WithAttributes(
			attrs.SUPI(supi.String()),
			attrs.SST(snssai.Sst),
			attrs.SD(snssai.Sd),
		),
	)
	defer span.End()

	sm, err := amf.subscriptions().SessionManagement(ctx, supi.IMSI())
	if err != nil {
		return "", err
	}

	dnn, ok := sm.DefaultDNN(*snssai)
	if !ok {
		return "", fmt.Errorf("no DNN subscribed on sst=%d sd=%q", snssai.Sst, snssai.Sd)
	}

	return dnn, nil
}

// NAS security algorithms are stored as RAT-neutral identities shared by EPS and
// 5G (TS 24.301 ≡ TS 24.501): NULL(0), SNOW3G(1), AES(2).
var cipheringNameToAlg = map[string]nas.CipheringAlgorithm{
	"NULL":   nas.CipheringNull,
	"SNOW3G": nas.CipheringSNOW3G,
	"AES":    nas.CipheringAES,
}

var integrityNameToAlg = map[string]nas.IntegrityAlgorithm{
	"NULL":   nas.IntegrityNull,
	"SNOW3G": nas.IntegritySNOW3G,
	"AES":    nas.IntegrityAES,
}

// SecurityAlgorithms loads the configured NAS security algorithm preference
// order from the database and returns them as uint8 slices ready for
// SelectSecurityAlg.
func (amf *AMF) SecurityAlgorithms(ctx context.Context) ([]nas.IntegrityAlgorithm, []nas.CipheringAlgorithm, error) {
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

	encOrder := make([]nas.CipheringAlgorithm, 0, len(cipherNames))
	for _, name := range cipherNames {
		alg, ok := cipheringNameToAlg[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown ciphering algorithm: %s", name)
		}

		encOrder = append(encOrder, alg)
	}

	intOrder := make([]nas.IntegrityAlgorithm, 0, len(integrityNames))
	for _, name := range integrityNames {
		alg, ok := integrityNameToAlg[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown integrity algorithm: %s", name)
		}

		intOrder = append(intOrder, alg)
	}

	return intOrder, encOrder, nil
}
