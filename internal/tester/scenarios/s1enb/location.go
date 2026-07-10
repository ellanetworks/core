// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
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

	if _, err := e.Attach(ue, 15*time.Second); err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	supi := "imsi-" + s1enbLocationIMSI

	logger.Logger.Info("UE attached, proceeding to location tests", zap.String("supi", supi))

	// E-CID is anchored by the eNB's access-point position (in the LPPa response),
	// so it yields a coordinate without any provisioning.
	ecid, err := common.GetLocation(ctx, cl, supi, "ecid")
	if err != nil {
		return fmt.Errorf("E-CID location failed: %w", err)
	}

	if ecid.LocationEstimate == nil || ecid.LocationEstimate.Point == nil {
		return fmt.Errorf("E-CID result missing locationEstimate point")
	}

	if ecid.Ecgi == nil {
		return fmt.Errorf("E-CID result missing ecgi")
	}

	if m := common.PositioningMethod(ecid); m != "ECID" {
		return fmt.Errorf("expected ECID positioning method, got %q", m)
	}

	logger.Logger.Info("E-CID location validated",
		zap.String("shape", ecid.LocationEstimate.Shape),
		zap.Float64("lat", ecid.LocationEstimate.Point.Lat),
		zap.Float64("lon", ecid.LocationEstimate.Point.Lon))

	if err := common.ProvisionCellPosition(ctx, cl, "eutra", ecid.Ecgi.PlmnID.Mcc, ecid.Ecgi.PlmnID.Mnc, ecid.Ecgi.EutraCellID); err != nil {
		logger.Logger.Warn("cell position provisioning returned an error (may already exist)", zap.Error(err))
	}

	cellID, err := common.GetLocation(ctx, cl, supi, "cell_id")
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

	return nil
}
