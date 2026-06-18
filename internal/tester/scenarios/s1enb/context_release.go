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

const ctxReleaseIMSI = "001017271246603"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/context_release",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBContextRelease,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(ctxReleaseIMSI, "")},
			}
		},
	})
}

// runS1ENBContextRelease attaches a UE, then has the eNB request an S1 context
// release (as on inactivity) and completes it, dropping the UE to ECM-IDLE
// (TS 36.413 §8.3.2).
func runS1ENBContextRelease(_ context.Context, env scenarios.Env, _ any) error {
	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(ctxReleaseIMSI, k, opc)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, 10*time.Second); err != nil {
		return fmt.Errorf("context release: %w", err)
	}

	return nil
}
