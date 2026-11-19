// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	haN2HandoverIMSI = "001017271246660"
	haXnHandoverIMSI = "001017271246661"
)

type handoverConnectivityOpts struct {
	imsi       string
	connectAll bool
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/n2_handover_5g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return n2HandoverConnectivity(ctx, env, handoverConnectivityOpts{imsi: haN2HandoverIMSI, connectAll: true})
		},
		Fixture: haHandoverFixture(haN2HandoverIMSI),
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "ha/xn_handover_5g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return xnHandoverConnectivity(ctx, env, handoverConnectivityOpts{imsi: haXnHandoverIMSI, connectAll: true})
		},
		Fixture: haHandoverFixture(haXnHandoverIMSI),
	})
}

func haHandoverFixture(imsi string) func(scenarios.Env) scenarios.FixtureSpec {
	return func(_ scenarios.Env) scenarios.FixtureSpec {
		return scenarios.FixtureSpec{
			Subscribers:         []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(imsi, "")},
			AssertUsageForIMSIs: []string{imsi},
		}
	}
}

func (o handoverConnectivityOpts) finish(ctx context.Context, env scenarios.Env, sourceAMF, targetAMF, ueIP string, targetRANUENGAPID int64) error {
	if !o.connectAll {
		return nil
	}

	return scenarios.HAHandoverDone(ctx, o.imsi, env.FirstCore(), sourceAMF, targetAMF, ueIP, targetRANUENGAPID)
}
