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
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// idleLocationMethods are the positioning methods exercised from CM-IDLE. A-GNSS buffers an
// LPP message for the UE (TS 23.273 §6.11.1), E-CID an NRPPa message for the RAN (§6.11.2),
// so together they cover both halves of the buffer-and-page path.
var idleLocationMethods = []string{"agnss_ue_assisted", "ecid"}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/location_idle",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &locationIdleParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address (e.g. http://10.3.0.2:5002)")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")
			fs.StringVar(&p.Method, "location-method", "", "Restrict to one positioning method (agnss_ue_assisted or ecid); default exercises both")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			p := params.(*locationIdleParams)

			return runLocationIdle(ctx, env, p)
		},
		Fixture: fixtureLocationIdle,
	})
}

type locationIdleParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
	Method         string
}

func fixtureLocationIdle(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runLocationIdle(ctx context.Context, env scenarios.Env, p *locationIdleParams) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required")
	}

	methods := idleLocationMethods
	if p.Method != "" {
		methods = []string{p.Method}
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("create API client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}
	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	sub := subscriber{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	supi := newUE.UeSecurity.Supi

	logger.Logger.Info("UE registered", zap.String("IMSI", sub.IMSI), zap.Int64("RAN UE NGAP ID", ranUENGAPID))

	for _, method := range methods {
		if err := locateIdleUE(ctx, cl, gNodeB, newUE, ranUENGAPID, supi, method, scenarios.DefaultPDUSessionID); err != nil {
			return fmt.Errorf("%s from CM-IDLE: %w", method, err)
		}
	}

	// Leave no context behind: the suite shares this IMSI with other scenarios.
	if err := releaseToIdle(gNodeB, newUE, ranUENGAPID, scenarios.DefaultPDUSessionID); err != nil {
		return fmt.Errorf("final UE context release: %w", err)
	}

	logger.Logger.Info("Idle location scenario completed successfully")

	return nil
}

// locateIdleUE drives one positioning method against a UE it first puts in CM-IDLE: the
// location request pages the UE, the UE answers, and the request completes.
func locateIdleUE(
	ctx context.Context,
	cl *client.Client,
	gNodeB *gnb.GnodeB,
	newUE *ue.UE,
	ranUENGAPID int64,
	supi string,
	method string,
	pduSessionID uint8,
) error {
	logger.Logger.Info("=== Putting UE in CM-IDLE ===", zap.String("method", method))

	if err := releaseToIdle(gNodeB, newUE, ranUENGAPID, pduSessionID); err != nil {
		return fmt.Errorf("UE context release: %w", err)
	}

	logger.Logger.Info("=== Location request (triggers paging) ===", zap.String("method", method))

	locCh := make(chan *common.LocationData, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := common.GetLocation(ctx, cl, supi, method)
		locCh <- result

		errCh <- err
	}()

	if _, err := gNodeB.WaitForMessage(gnb.Initiating, ngap.ProcPaging, 15*time.Second); err != nil {
		return fmt.Errorf("did not receive paging within 15s: %w", err)
	}

	logger.Logger.Info("Paging received from AMF")

	// A UE answering a page sets the service type to "mobile terminated services"
	// (TS 24.501 §5.6.1.2.1 for case a of §5.6.1.1), which is also the branch that
	// delivers the downlink message the paging was triggered for.
	var pduSessionStatus [16]bool

	pduSessionStatus[pduSessionID] = true

	if err := newUE.SendServiceRequest(ranUENGAPID, pduSessionStatus, uint8(fgs.ServiceTypeMobileTerminatedServices)); err != nil {
		return fmt.Errorf("send service request: %w", err)
	}

	if _, err := newUE.WaitForNASGMMMessage(uint8(fgs.MsgServiceAccept), 5*time.Second); err != nil {
		return fmt.Errorf("did not receive Service Accept: %w", err)
	}

	// TS 24.501 §5.4.4.1: the AMF reallocates the 5G-GUTI after a Service Request
	// triggered by paging, in a Configuration Update Command.
	if _, err := newUE.WaitForNASGMMMessage(uint8(fgs.MsgConfigurationUpdateCommand), 5*time.Second); err != nil {
		return fmt.Errorf("did not receive Configuration Update Command: %w", err)
	}

	logger.Logger.Info("Service Request completed, UE is CM-CONNECTED")

	select {
	case result := <-locCh:
		if err := <-errCh; err != nil {
			return fmt.Errorf("location request failed: %w", err)
		}

		return validateIdleResult(result, method)
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timed out waiting for location result after paging")
	}
}

// releaseToIdle takes the UE to CM-IDLE, keeping its PDU session so the location
// request that follows has something to page for.
func releaseToIdle(gNodeB *gnb.GnodeB, newUE *ue.UE, ranUENGAPID int64, pduSessionID uint8) error {
	return gNodeB.ReleaseContext(newUE, ranUENGAPID, []uint8{pduSessionID}, gnb.CauseUserInactivity, releaseTimeout)
}

func validateIdleResult(result *common.LocationData, method string) error {
	if result == nil {
		return fmt.Errorf("nil location result")
	}

	if result.LocationEstimate == nil || result.LocationEstimate.Point == nil {
		return fmt.Errorf("location result missing locationEstimate point")
	}

	switch method {
	case "agnss_ue_assisted", "agnss_ue_based":
		// A-GNSS: coordinates from UE tester (45.0N, 21.45E).
		if m := common.PositioningMethod(result); m != "GNSS" {
			return fmt.Errorf("expected GNSS method, got %q", m)
		}

		if lat := result.LocationEstimate.Point.Lat; lat < 44.99 || lat > 45.01 {
			return fmt.Errorf("A-GNSS lat mismatch: expected ~45.0, got %f", lat)
		}

		if lon := result.LocationEstimate.Point.Lon; lon < 21.44 || lon > 21.46 {
			return fmt.Errorf("A-GNSS lon mismatch: expected ~21.45, got %f", lon)
		}

		logger.Logger.Info("A-GNSS idle test passed",
			zap.Float64("lat", result.LocationEstimate.Point.Lat),
			zap.Float64("lon", result.LocationEstimate.Point.Lon))

	case "ecid":
		// A degradation to Cell-ID means the NRPPa exchange did not complete inside the
		// LMF's measurement timeout, so the method check is the real assertion here.
		if m := common.PositioningMethod(result); m != "ECID" && m != "NR_ECID" {
			return fmt.Errorf("expected E-CID method, got %q", m)
		}

		if result.Ncgi == nil {
			return fmt.Errorf("E-CID result missing NCGI")
		}

		logger.Logger.Info("E-CID idle test passed",
			zap.String("cellId", result.Ncgi.NrCellID))

	default:
		return fmt.Errorf("unsupported method: %s", method)
	}

	return nil
}
