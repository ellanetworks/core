// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const serviceRequestIMSI = "001017271246612"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/service_request/data",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBServiceRequest,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(serviceRequestIMSI, "")},
			}
		},
	})
}

// runS1ENBServiceRequest attaches a UE, drops it to ECM-IDLE with an S1 release,
// then has the UE return with a mobile-originated SERVICE REQUEST and verifies the
// MME re-establishes the bearer (TS 24.301 §5.6.1).
func runS1ENBServiceRequest(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(serviceRequestIMSI, k, opc)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI, cannot service-request")
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, 10*time.Second); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	mmeUEID, _, err := e.ServiceRequest(ue, res.GUTI, 10*time.Second)
	if err != nil {
		return fmt.Errorf("service request: %w", err)
	}

	if mmeUEID == res.MMEUES1APID {
		return fmt.Errorf("MME reused the released MME-UE-S1AP-ID %d; expected a fresh one", mmeUEID)
	}

	return nil
}
