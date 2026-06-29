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

const periodicTAUIMSI = "001017271246613"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration/periodic/signalling",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBPeriodicTAU,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(periodicTAUIMSI, "")},
			}
		},
	})
}

func runS1ENBPeriodicTAU(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(periodicTAUIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI, cannot tracking-area-update")
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, 10*time.Second); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	if err := e.PeriodicTrackingAreaUpdate(ue, res.GUTI, 10*time.Second); err != nil {
		return fmt.Errorf("periodic tracking area update: %w", err)
	}

	return nil
}
