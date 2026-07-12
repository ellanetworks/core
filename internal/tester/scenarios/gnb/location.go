// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
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

	ecidResult, err := common.GetLocation(ctx, cl, supi, "ecid")
	if err != nil {
		return fmt.Errorf("E-CID location failed: %v", err)
	}

	if ecidResult.LocationEstimate == nil || ecidResult.LocationEstimate.Point == nil {
		return fmt.Errorf("E-CID result missing locationEstimate point")
	}

	if ecidResult.Ncgi == nil {
		return fmt.Errorf("E-CID result missing ncgi")
	}

	if m := common.PositioningMethod(ecidResult); m != "ECID" && m != "NR_ECID" {
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

	if err := common.ProvisionCellPosition(ctx, cl, "nr", ecidResult.Ncgi.PlmnID.Mcc, ecidResult.Ncgi.PlmnID.Mnc, ecidResult.Ncgi.NrCellID); err != nil {
		logger.Logger.Warn("cell position provisioning returned an error (may already exist)", zap.Error(err))
	}

	// --- Phase 3: Cell ID location ---
	// With the cell position provisioned, Cell-ID resolves a coordinate from
	// the table.
	logger.Logger.Info("=== Testing Cell ID location ===")

	cellIDResult, err := common.GetLocation(ctx, cl, supi, "cell_id")
	if err != nil {
		return fmt.Errorf("cell ID location failed: %v", err)
	}

	if cellIDResult.LocationEstimate == nil || cellIDResult.LocationEstimate.Point == nil {
		return fmt.Errorf("cell ID result missing locationEstimate point (is the cell provisioned?)")
	}

	if m := common.PositioningMethod(cellIDResult); m != "CELLID" {
		return fmt.Errorf("expected CELLID positioning method, got %q", m)
	}

	if cellIDResult.Ncgi == nil {
		return fmt.Errorf("cell ID result missing NCGI")
	}

	logger.Logger.Info("Cell ID location validated successfully",
		zap.String("shape", cellIDResult.LocationEstimate.Shape),
		zap.Float64("lat", cellIDResult.LocationEstimate.Point.Lat),
	)

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
