// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/nas"
	"github.com/spf13/pflag"
)

const (
	framedReconcileAddIMSI    = "001017271246820"
	framedReconcileRemoveIMSI = "001017271246821"
	framedReconcileAddPrefix  = "192.168.70.0/24"
	framedReconcileRmPrefix   = "192.168.71.0/24"
)

type framedReconcileParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func bindFramedReconcileFlags(fs *pflag.FlagSet) any {
	p := &framedReconcileParams{}
	fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
	fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

	return p
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/framed_route_add_live",
		BindFlags: bindFramedReconcileFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runFramedRouteReconcile(ctx, env, params.(*framedReconcileParams), framedReconcileAddIMSI, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(framedReconcileAddIMSI, "")},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/framed_route_remove_live",
		BindFlags: bindFramedReconcileFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runFramedRouteReconcile(ctx, env, params.(*framedReconcileParams), framedReconcileRemoveIMSI, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(framedReconcileRemoveIMSI, "")},
				FramedRoutes: []scenarios.FramedRouteSpec{{
					IMSI:        framedReconcileRemoveIMSI,
					DataNetwork: scenarios.DefaultDNN,
					IPv4:        []string{framedReconcileRmPrefix},
				}},
			}
		},
	})
}

// runFramedRouteReconcile establishes a PDU session, then adds (add=true) or
// removes (add=false) a framed route on the subscriber via the API while the
// session is live, and asserts the SMF releases it with cause #39 "reactivation
// requested" so the UE re-establishes with the new routes (TS 23.501 §5.6.14,
// TS 24.501). A framed-route change cannot be applied in place because the routes
// live in the session's downlink PDRs.
func runFramedRouteReconcile(ctx context.Context, env scenarios.Env, p *framedReconcileParams, imsi string, add bool) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(gNodeB, imsi[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	if add {
		if err := cl.CreateDataNetworkFramedRoute(ctx, scenarios.DefaultDNN, &client.CreateFramedRouteOptions{
			IMSI: imsi,
			IPv4: []string{framedReconcileAddPrefix},
		}); err != nil {
			return fmt.Errorf("add framed route on live session: %w", err)
		}
	} else {
		if err := cl.DeleteDataNetworkFramedRoute(ctx, scenarios.DefaultDNN, imsi); err != nil {
			return fmt.Errorf("remove framed route on live session: %w", err)
		}
	}

	releaseCmd, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionReleaseCommand, 15*time.Second)
	if err != nil {
		return fmt.Errorf("UE did not receive PDU Session Release Command after framed-route change: %w", err)
	}

	if releaseCmd.PDUSessionReleaseCommand == nil {
		return fmt.Errorf("PDUSessionReleaseCommand is nil")
	}

	if cause := releaseCmd.PDUSessionReleaseCommand.GetCauseValue(); cause != 39 {
		return fmt.Errorf("expected cause #39 (reactivation requested) after framed-route change, got %d", cause)
	}

	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	return procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
}
