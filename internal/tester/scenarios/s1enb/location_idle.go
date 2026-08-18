// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const s1enbLocationIdleIMSI = "001017271246613"

type locationIdleParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "s1enb/location_idle",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &locationIdleParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBLocationIdle(ctx, env, params.(*locationIdleParams))
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1enbLocationIdleIMSI, "")},
			}
		},
	})
}

func runS1ENBLocationIdle(ctx context.Context, env scenarios.Env, p *locationIdleParams) error {
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

	ue := e.NewUE(s1enbLocationIdleIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI")
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, 10*time.Second); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	supi := "imsi-" + s1enbLocationIdleIMSI

	logger.Logger.Info("UE released to ECM-IDLE, requesting E-CID location", zap.String("supi", supi))

	locCh := make(chan *common.LocationData, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := common.GetLocation(ctx, cl, supi, "ecid")
		locCh <- result

		errCh <- err
	}()

	if _, err := e.WaitForMessage(s1enb.NoUEID, s1enb.Initiating, s1ap.ProcPaging, 15*time.Second); err != nil {
		return fmt.Errorf("did not receive S1AP Paging within 15s: %w", err)
	}

	logger.Logger.Info("S1AP Paging received from MME")

	sr, err := e.ServiceRequest(ue, res.GUTI, 10*time.Second)
	if err != nil {
		return fmt.Errorf("service request answering the page: %w", err)
	}

	logger.Logger.Info("Service Request completed, UE is ECM-CONNECTED")

	select {
	case result := <-locCh:
		if err := <-errCh; err != nil {
			return fmt.Errorf("E-CID location failed: %w", err)
		}

		if result == nil {
			return fmt.Errorf("nil location result")
		}

		if result.LocationEstimate == nil || result.LocationEstimate.Point == nil {
			return fmt.Errorf("E-CID result missing locationEstimate point")
		}

		if result.Ecgi == nil {
			return fmt.Errorf("E-CID result missing ecgi")
		}

		if m := common.PositioningMethod(result); m != "ECID" {
			return fmt.Errorf("expected ECID positioning method, got %q", m)
		}

		logger.Logger.Info("E-CID idle location validated",
			zap.String("shape", result.LocationEstimate.Shape),
			zap.Float64("lat", result.LocationEstimate.Point.Lat),
			zap.Float64("lon", result.LocationEstimate.Point.Lon))

	case <-time.After(60 * time.Second):
		return fmt.Errorf("timed out waiting for E-CID location result after paging")
	}

	if err := e.ReleaseContext(sr.MMEUES1APID, sr.ENBUES1APID, s1enb.CauseUserInactivity, 10*time.Second); err != nil {
		return fmt.Errorf("final UE context release: %w", err)
	}

	return nil
}
