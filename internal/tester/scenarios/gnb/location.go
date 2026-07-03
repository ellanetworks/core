// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "ue/location",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &locationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address (e.g. http://10.3.0.2:5002)")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			p := params.(*locationParams)

			return runLocationTest(ctx, env, p)
		},
		Fixture: fixtureLocation,
	})
}

type locationParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func fixtureLocation(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runLocationTest(ctx context.Context, env scenarios.Env, p *locationParams) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required for location scenario")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required for location scenario")
	}

	// Build Ella API client.
	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

	// Start gNB.
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %v", err)
	}

	// Create UE and register.
	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	logger.Logger.Info(
		"UE registered, proceeding to location tests",
		zap.String("IMSI", sub.IMSI),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	// Extract SUPI from UE security context.
	supi := newUE.UeSecurity.Supi

	// --- Phase 1: E-CID location ---
	// E-CID is anchored by the gNB's NG-RAN Access Point Position, so it yields
	// a coordinate without any provisioning. Its response also carries the
	// canonical serving NCGI we use to provision the cell-position table.
	logger.Logger.Info("=== Testing E-CID location ===")

	ecidResult, err := getLocation(ctx, cl, supi, "ecid")
	if err != nil {
		return fmt.Errorf("E-CID location failed: %v", err)
	}

	if ecidResult.LocationEstimate == nil || ecidResult.LocationEstimate.Point == nil {
		return fmt.Errorf("E-CID result missing locationEstimate point")
	}

	if ecidResult.Ncgi == nil {
		return fmt.Errorf("E-CID result missing ncgi")
	}

	if m := positioningMethod(ecidResult); m != "ECID" && m != "NR_ECID" {
		return fmt.Errorf("expected E-CID positioning method, got %q", m)
	}

	logger.Logger.Info("E-CID location validated successfully",
		zap.String("shape", ecidResult.LocationEstimate.Shape),
		zap.Float64("lat", ecidResult.LocationEstimate.Point.Lat),
		zap.Float64("lon", ecidResult.LocationEstimate.Point.Lon),
	)

	// --- Phase 2: provision the serving cell's position ---
	logger.Logger.Info("=== Provisioning cell position ===",
		zap.String("nrCellId", ecidResult.Ncgi.NrCellID))

	if err := provisionCellPosition(ctx, cl, ecidResult.Ncgi.PlmnID.Mcc, ecidResult.Ncgi.PlmnID.Mnc, ecidResult.Ncgi.NrCellID); err != nil {
		logger.Logger.Warn("cell position provisioning returned an error (may already exist)", zap.Error(err))
	}

	// --- Phase 3: Cell ID location ---
	// With the cell position provisioned, Cell-ID resolves a coordinate from
	// the table.
	logger.Logger.Info("=== Testing Cell ID location ===")

	cellIDResult, err := getLocation(ctx, cl, supi, "cell_id")
	if err != nil {
		return fmt.Errorf("cell ID location failed: %v", err)
	}

	if cellIDResult.LocationEstimate == nil || cellIDResult.LocationEstimate.Point == nil {
		return fmt.Errorf("cell ID result missing locationEstimate point (is the cell provisioned?)")
	}

	if m := positioningMethod(cellIDResult); m != "CELLID" {
		return fmt.Errorf("expected CELLID positioning method, got %q", m)
	}

	if cellIDResult.Ncgi == nil {
		return fmt.Errorf("cell ID result missing NCGI")
	}

	logger.Logger.Info("Cell ID location validated successfully",
		zap.String("shape", cellIDResult.LocationEstimate.Shape),
		zap.Float64("lat", cellIDResult.LocationEstimate.Point.Lat),
	)

	// --- Phase 4: A-GNSS location ---
	logger.Logger.Info("=== Testing A-GNSS location ===")

	agnssResult, err := getLocation(ctx, cl, supi, "agnss_ue_assisted")
	if err != nil {
		return fmt.Errorf("A-GNSS location failed: %v", err)
	}

	if agnssResult.LocationEstimate == nil || agnssResult.LocationEstimate.Point == nil {
		return fmt.Errorf("A-GNSS result missing locationEstimate point")
	}

	if m := positioningMethod(agnssResult); m != "GNSS" {
		return fmt.Errorf("expected GNSS positioning method, got %q", m)
	}

	// A-GNSS coordinates come from the UE tester: 45.0°N, 21.45°E.
	if lat := agnssResult.LocationEstimate.Point.Lat; lat < 44.99 || lat > 45.01 {
		return fmt.Errorf("A-GNSS latitude mismatch: expected ~45.0, got %f", lat)
	}

	if lon := agnssResult.LocationEstimate.Point.Lon; lon < 21.44 || lon > 21.46 {
		return fmt.Errorf("A-GNSS longitude mismatch: expected ~21.45, got %f", lon)
	}

	logger.Logger.Info("A-GNSS location validated successfully")

	// --- Cleanup: Deregister UE ---
	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
	if err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	logger.Logger.Info("Location scenario completed successfully")

	return nil
}

// locationData is a minimal view of the spec-shaped LocationData response
// (TS 29.572) returned by POST /api/beta/location.
type locationData struct {
	LocationEstimate    *locGeoArea      `json:"locationEstimate"`
	PositioningDataList []locMethodUsage `json:"positioningDataList"`
	Ncgi                *locNcgi         `json:"ncgi"`
}

type locGeoArea struct {
	Shape       string       `json:"shape"`
	Point       *locGeoPoint `json:"point"`
	Uncertainty *float64     `json:"uncertainty"`
}

type locGeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type locMethodUsage struct {
	Method string `json:"method"`
	Mode   string `json:"mode"`
	Usage  string `json:"usage"`
}

type locPlmn struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type locNcgi struct {
	PlmnID   locPlmn `json:"plmnId"`
	NrCellID string  `json:"nrCellId"`
}

// positioningMethod returns the first reported positioning method, or "".
func positioningMethod(d *locationData) string {
	if len(d.PositioningDataList) == 0 {
		return ""
	}

	return d.PositioningDataList[0].Method
}

// getLocation calls POST /api/beta/location for the given method and decodes the
// spec-shaped LocationData response.
func getLocation(ctx context.Context, cl *client.Client, supi, method string) (*locationData, error) {
	reqBody := map[string]string{
		"supi":         supi,
		"request_type": "immediate",
		"method":       method,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	resp, err := cl.Requester.Do(ctx, &client.RequestOptions{
		Type:   client.SyncRequest,
		Method: http.MethodPost,
		Path:   "/api/beta/location",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bytes.NewReader(body),
	})
	if err != nil {
		return nil, fmt.Errorf("POST location request failed: %w", err)
	}

	var result locationData
	if err := resp.DecodeResult(&result); err != nil {
		return nil, fmt.Errorf("decode location response: %w", err)
	}

	return &result, nil
}

// provisionCellPosition provisions an antenna coordinate for the given NR cell
// so Cell-ID / E-CID can anchor a location estimate.
func provisionCellPosition(ctx context.Context, cl *client.Client, mcc, mnc, nrCellID string) error {
	reqBody := map[string]any{
		"rat":                    "nr",
		"mcc":                    mcc,
		"mnc":                    mnc,
		"cell_identity":          nrCellID,
		"latitude":               45.0,
		"longitude":              21.45,
		"uncertainty_semi_major": 150.0,
		"uncertainty_semi_minor": 150.0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	_, err = cl.Requester.Do(ctx, &client.RequestOptions{
		Type:   client.SyncRequest,
		Method: http.MethodPost,
		Path:   "/api/beta/cell-positions",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bytes.NewReader(body),
	})
	if err != nil {
		return fmt.Errorf("POST cell-positions request failed: %w", err)
	}

	return nil
}
