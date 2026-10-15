// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/nrppa"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "ue/location_measurement_quantities",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &locationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address (e.g. http://10.3.0.2:5002)")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runLocationMeasurementQuantitiesTest(ctx, env, params.(*locationParams))
		},
		Fixture: fixtureLocation,
	})
}

func runLocationMeasurementQuantitiesTest(ctx context.Context, env scenarios.Env, p *locationParams) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

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

	gNodeB.SupportNRPPaMeasurementQuantities(
		nrppa.MeasSSRSRP,
		nrppa.MeasSSRSRQ,
		nrppa.MeasAngleOfArrivalNR,
		nrppa.MeasTimingAdvanceNR,
	)

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout); err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	supi := newUE.UeSecurity.Supi

	defaultResult, err := common.GetLocation(ctx, cl, supi, "ecid", "network_based")
	if err == nil && networkECIDSucceeded(defaultResult) {
		return fmt.Errorf("expected the gNB to reject the default quantities, got %+v", defaultResult.PositioningDataList)
	}

	logger.Logger.Info("default E-CID quantities rejected by the gNB as expected", zap.Error(err))

	narrowed, err := common.GetECIDLocationWithQuantities(ctx, cl, supi, "network_based",
		[]string{"sS-RSRP", "sS-RSRQ", "timingAdvanceNR"}, nil)
	if err != nil {
		return fmt.Errorf("E-CID with supported quantities failed: %v", err)
	}

	if !networkECIDSucceeded(narrowed) {
		return fmt.Errorf("expected a successful network-based NR_ECID result, got %+v", narrowed.PositioningDataList)
	}

	logger.Logger.Info("E-CID with supported quantities succeeded",
		zap.Int("network_measurements", common.CountMeasurements(narrowed, "NETWORK")),
	)

	return nil
}

func networkECIDSucceeded(result *common.LocationData) bool {
	return common.HasPositioning(result, "NR_ECID", "CONVENTIONAL", "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION") &&
		common.CountMeasurements(result, "NETWORK") > 0
}
