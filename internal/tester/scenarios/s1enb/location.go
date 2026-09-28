// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// s1enbLocationIMSI is dedicated to this scenario so it does not race the other
// s1enb scenarios that reuse s1enbIMSI.
const s1enbLocationIMSI = "001017271246601"

type locationParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "s1enb/location",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &locationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBLocation(ctx, env, params.(*locationParams))
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1enbLocationIMSI, "")},
			}
		},
	})
}

func runS1ENBLocation(ctx context.Context, env scenarios.Env, p *locationParams) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(s1enbLocationIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	attached, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	supi := "imsi-" + s1enbLocationIMSI

	logger.Logger.Info("UE attached, proceeding to location tests", zap.String("supi", supi))

	// E-CID is anchored by the eNB's access-point position (in the LPPa response),
	// so it yields a coordinate without any provisioning.
	ecid, err := common.GetLocation(ctx, cl, supi, "ecid", "network_based")
	if err != nil {
		return fmt.Errorf("E-CID location failed: %w", err)
	}

	if ecid.LocationEstimate == nil || ecid.LocationEstimate.Point == nil {
		return fmt.Errorf("E-CID result missing locationEstimate point")
	}

	if ecid.Ecgi == nil {
		return fmt.Errorf("E-CID result missing ecgi")
	}

	if !common.HasPositioning(ecid, "ECID", "CONVENTIONAL", "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION") {
		return fmt.Errorf("expected a network-based ECID result that generated the location, got %+v", ecid.PositioningDataList)
	}

	if n := common.CountMeasurements(ecid, "NETWORK"); n == 0 {
		return fmt.Errorf("network-based E-CID returned no eNB measurements")
	}

	logger.Logger.Info("E-CID location validated",
		zap.String("shape", ecid.LocationEstimate.Shape),
		zap.Float64("lat", ecid.LocationEstimate.Point.Lat),
		zap.Float64("lon", ecid.LocationEstimate.Point.Lon))

	if err := common.ProvisionCellPosition(ctx, cl, "eutra", ecid.Ecgi.PlmnID.Mcc, ecid.Ecgi.PlmnID.Mnc, ecid.Ecgi.EutraCellID); err != nil {
		logger.Logger.Warn("cell position provisioning returned an error (may already exist)", zap.Error(err))
	}

	cellID, err := common.GetLocation(ctx, cl, supi, "cell_id", "")
	if err != nil {
		return fmt.Errorf("cell ID location failed: %w", err)
	}

	if cellID.LocationEstimate == nil || cellID.LocationEstimate.Point == nil {
		return fmt.Errorf("cell ID result missing locationEstimate point (is the cell provisioned?)")
	}

	if m := common.PositioningMethod(cellID); m != "CELLID" {
		return fmt.Errorf("expected CELLID positioning method, got %q", m)
	}

	if cellID.Ecgi == nil {
		return fmt.Errorf("cell ID result missing ecgi")
	}

	logger.Logger.Info("Cell ID location validated",
		zap.String("shape", cellID.LocationEstimate.Shape),
		zap.Float64("lat", cellID.LocationEstimate.Point.Lat))

	type locationOutcome struct {
		result *common.LocationData
		err    error
	}

	fix := s1enb.LPPFix{Latitude: 450000000, Longitude: 214500000, Altitude: 10000, HorizontalAccuracy: 10, VerticalAccuracy: 15}

	ueECIDDone := make(chan locationOutcome, 1)

	go func() {
		result, err := common.GetLocation(ctx, cl, supi, "ecid", "ue_assisted")
		ueECIDDone <- locationOutcome{result: result, err: err}
	}()

	if err := e.AnswerLPP(ue, attached.ENBUES1APID, fix, lppTimeout); err != nil {
		return fmt.Errorf("answer LPP E-CID: %w", err)
	}

	ueECID := <-ueECIDDone
	if ueECID.err != nil {
		return fmt.Errorf("UE-assisted E-CID location failed: %w", ueECID.err)
	}

	if !common.HasPositioning(ueECID.result, "ECID", "UE_ASSISTED", "SUCCESS_RESULTS_NOT_USED") {
		return fmt.Errorf("expected a successful UE-assisted ECID result, got %+v", ueECID.result.PositioningDataList)
	}

	if !common.HasPositioning(ueECID.result, "CELLID", "CONVENTIONAL", "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION") {
		return fmt.Errorf("expected the provisioned cell to generate the location, got %+v", ueECID.result.PositioningDataList)
	}

	if n := common.CountMeasurements(ueECID.result, "UE"); n != 2 {
		return fmt.Errorf("expected 2 UE-reported cells, got %d", n)
	}

	logger.Logger.Info("UE-assisted E-CID location validated")

	agnssDone := make(chan locationOutcome, 1)

	go func() {
		result, err := common.GetLocation(ctx, cl, supi, "gnss", "")
		agnssDone <- locationOutcome{result: result, err: err}
	}()

	if err := e.AnswerLPP(ue, attached.ENBUES1APID, fix, lppTimeout); err != nil {
		return fmt.Errorf("answer LPP: %w", err)
	}

	agnss := <-agnssDone
	if agnss.err != nil {
		return fmt.Errorf("A-GNSS location failed: %w", agnss.err)
	}

	if agnss.result.LocationEstimate == nil || agnss.result.LocationEstimate.Point == nil {
		return fmt.Errorf("A-GNSS result missing locationEstimate point")
	}

	if g := common.GNSSPositioning(agnss.result); g != "GPS" {
		return fmt.Errorf("expected a GPS positioning result, got %q", g)
	}

	if lat := agnss.result.LocationEstimate.Point.Lat; lat < 44.99 || lat > 45.01 {
		return fmt.Errorf("A-GNSS latitude mismatch: expected ~45.0, got %f", lat)
	}

	if lon := agnss.result.LocationEstimate.Point.Lon; lon < 21.44 || lon > 21.46 {
		return fmt.Errorf("A-GNSS longitude mismatch: expected ~21.45, got %f", lon)
	}

	logger.Logger.Info("A-GNSS location validated",
		zap.Float64("lat", agnss.result.LocationEstimate.Point.Lat),
		zap.Float64("lon", agnss.result.LocationEstimate.Point.Lon))

	return nil
}
