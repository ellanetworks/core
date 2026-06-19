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
	"github.com/ellanetworks/core/internal/lmf/models"
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

	// --- Phase 1: Cell ID location ---
	logger.Logger.Info("=== Testing Cell ID location ===")

	cellIDResult, err := getLocationCellID(ctx, cl, supi)
	if err != nil {
		return fmt.Errorf("cell ID location failed: %v", err)
	}

	logger.Logger.Info("Cell ID location retrieved",
		zap.Int("shape", int(cellIDResult.Shape)),
		zap.String("access_type", cellIDResult.AccessType),
	)

	// Validate Cell ID result has expected fields.
	if cellIDResult.Shape != models.GADCellID {
		return fmt.Errorf("expected Cell ID shape %d, got %d", models.GADCellID, cellIDResult.Shape)
	}

	if cellIDResult.AccessType != "NR" {
		return fmt.Errorf("expected access type NR, got %s", cellIDResult.AccessType)
	}

	if cellIDResult.TAI == nil {
		return fmt.Errorf("cell ID result missing TAI")
	}

	if cellIDResult.NCGI == nil {
		return fmt.Errorf("cell ID result missing NCGI")
	}

	logger.Logger.Info("Cell ID location validated successfully")

	// --- Phase 2: E-CID location ---
	logger.Logger.Info("=== Testing E-CID location ===")

	ecidResult, err := getLocationECID(ctx, cl, supi, scenarios.DefaultAMF)
	if err != nil {
		return fmt.Errorf("E-CID location failed: %v", err)
	}

	logger.Logger.Info("E-CID location retrieved",
		zap.Int("shape", int(ecidResult.Shape)),
		zap.Any("rsrp", ecidResult.RSRP),
		zap.Any("ta", ecidResult.TA),
		zap.Any("distance_m", ecidResult.Distance),
	)

	// Validate E-CID result.
	if ecidResult.Shape != models.GADECID {
		return fmt.Errorf("expected E-CID shape %d, got %d", models.GADECID, ecidResult.Shape)
	}

	// E-CID should have radio measurements from gNB tester.
	// RSRP is optional (gNB tester may not report it).
	if ecidResult.TA == nil {
		return fmt.Errorf("E-CID result missing Timing Advance measurement")
	}

	if ecidResult.Distance == nil {
		return fmt.Errorf("E-CID result missing distance estimate")
	}

	logger.Logger.Info("E-CID location validated successfully")

	// --- Phase 3: A-GNSS location ---
	logger.Logger.Info("=== Testing A-GNSS location ===")

	agnssResult, err := getLocationAGNSS(ctx, cl, supi, scenarios.DefaultAMF)
	if err != nil {
		return fmt.Errorf("A-GNSS location failed: %v", err)
	}

	logger.Logger.Info("A-GNSS location retrieved",
		zap.Int("shape", int(agnssResult.Shape)),
		zap.Int32("latitude", agnssResult.Latitude),
		zap.Int32("longitude", agnssResult.Longitude),
		zap.Int32("altitude", agnssResult.Altitude),
		zap.Uint32("horizontal_accuracy", agnssResult.HorizontalAccuracy),
	)

	// Validate A-GNSS result.
	if agnssResult.Shape != models.GADEllipsoidalPoint {
		return fmt.Errorf("expected A-GNSS shape %d, got %d", models.GADEllipsoidalPoint, agnssResult.Shape)
	}

	// A-GNSS should have coordinates from UE tester.
	if agnssResult.Latitude == 0 {
		return fmt.Errorf("A-GNSS result has zero latitude")
	}

	if agnssResult.Longitude == 0 {
		return fmt.Errorf("A-GNSS result has zero longitude")
	}

	// Expected coordinates: 45.0°N, 21.45°E (stored as 1e-7 degrees).
	expectedLat := int32(45.0 * 1e7)
	expectedLon := int32(21.45 * 1e7)

	if agnssResult.Latitude != expectedLat {
		return fmt.Errorf("A-GNSS latitude mismatch: expected %d, got %d", expectedLat, agnssResult.Latitude)
	}

	if agnssResult.Longitude != expectedLon {
		return fmt.Errorf("A-GNSS longitude mismatch: expected %d, got %d", expectedLon, agnssResult.Longitude)
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

// getLocationCellID calls POST /api/beta/location with request_type "immediate"
// to retrieve the UE's location using the Cell ID method.
func getLocationCellID(ctx context.Context, cl *client.Client, supi string) (*models.LocationResult, error) {
	reqBody := map[string]string{
		"supi":         supi,
		"request_type": "immediate",
		"method":       "cell_id",
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

	var result models.LocationResult
	if err := resp.DecodeResult(&result); err != nil {
		return nil, fmt.Errorf("decode location response: %w", err)
	}

	return &result, nil
}

// getLocationECID calls POST /api/beta/location with request_type "immediate"
// and method "ecid". The LMF internally creates a session for the E-CID procedure.
func getLocationECID(ctx context.Context, cl *client.Client, supi string, amfID string) (*models.LocationResult, error) {
	sessionID, err := createLocationSession(ctx, cl, supi, amfID, "ecid", "immediate")
	if err != nil {
		return nil, fmt.Errorf("create E-CID session: %w", err)
	}

	result, err := waitForSessionResult(ctx, cl, sessionID, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait for E-CID session: %w", err)
	}

	return result, nil
}

// getLocationAGNSS calls POST /api/beta/location with request_type "immediate"
// and method "agnss_ue_assisted". The LMF internally creates an LPP session.
func getLocationAGNSS(ctx context.Context, cl *client.Client, supi string, amfID string) (*models.LocationResult, error) {
	sessionID, err := createLocationSession(ctx, cl, supi, amfID, "agnss_ue_assisted", "immediate")
	if err != nil {
		return nil, fmt.Errorf("create A-GNSS session: %w", err)
	}

	result, err := waitForSessionResult(ctx, cl, sessionID, 45*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait for A-GNSS session: %w", err)
	}

	return result, nil
}

// createLocationSession creates a location session via the unified endpoint and
// returns the session ID. For immediate requests, the LMF creates the session
// internally and returns the ID for tracking.
func createLocationSession(ctx context.Context, cl *client.Client, supi, amfID, method, requestType string) (string, error) {
	params := map[string]string{
		"supi":         supi,
		"amf_id":       amfID,
		"method":       method,
		"request_type": requestType,
	}

	body, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
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
		return "", fmt.Errorf("POST location request failed: %w", err)
	}

	var result map[string]string
	if err := resp.DecodeResult(&result); err != nil {
		return "", fmt.Errorf("decode location response: %w", err)
	}

	sessionID, ok := result["id"]
	if !ok {
		return "", fmt.Errorf("location response missing 'id' field")
	}

	logger.Logger.Info("Location session created",
		zap.String("session_id", sessionID),
		zap.String("method", method),
		zap.String("request_type", requestType),
	)

	return sessionID, nil
}

// waitForSessionResult polls the session status until it completes or times out.
func waitForSessionResult(ctx context.Context, cl *client.Client, sessionID string, timeout time.Duration) (*models.LocationResult, error) {
	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for session: %w", ctx.Err())
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for session %s to complete", sessionID)
			}

			result, status, err := getSessionResult(ctx, cl, sessionID)
			if err != nil {
				return nil, fmt.Errorf("get session %s status: %w", sessionID, err)
			}

			switch status {
			case 1: // SessionStatusCompleted
				return result, nil
			case 2: // SessionStatusFailed
				return nil, fmt.Errorf("session %s failed", sessionID)
			default:
				logger.Logger.Debug("Session still active",
					zap.String("session_id", sessionID),
					zap.Int("status", status),
				)
			}
		}
	}
}

// getSessionResult fetches the session detail and returns the location result
// along with the session status code.
func getSessionResult(ctx context.Context, cl *client.Client, sessionID string) (*models.LocationResult, int, error) {
	resp, err := cl.Requester.Do(ctx, &client.RequestOptions{
		Type:   client.SyncRequest,
		Method: http.MethodGet,
		Path:   fmt.Sprintf("/api/beta/positioning/sessions/%s", sessionID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GET session request failed: %w", err)
	}

	var detail struct {
		Status     int             `json:"status"`
		LastResult json.RawMessage `json:"last_result,omitempty"`
	}

	if err := resp.DecodeResult(&detail); err != nil {
		return nil, 0, fmt.Errorf("decode session detail: %w", err)
	}

	if detail.LastResult == nil {
		return nil, detail.Status, nil
	}

	var result models.LocationResult
	if err := json.Unmarshal(detail.LastResult, &result); err != nil {
		return nil, 0, fmt.Errorf("unmarshal location result: %w", err)
	}

	return &result, detail.Status, nil
}
