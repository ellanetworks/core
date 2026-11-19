// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	haS1HandoverIMSI = "001017271246662"
	haX2HandoverIMSI = "001017271246663"
)

type handoverConnectivityOpts struct {
	imsi       string
	connectAll bool
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/s1_handover_4g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return s1HandoverConnectivity(ctx, env, handoverConnectivityOpts{imsi: haS1HandoverIMSI, connectAll: true})
		},
		Fixture: haHandoverFixture(haS1HandoverIMSI),
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "ha/x2_handover_4g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return x2HandoverConnectivity(ctx, env, handoverConnectivityOpts{imsi: haX2HandoverIMSI, connectAll: true})
		},
		Fixture: haHandoverFixture(haX2HandoverIMSI),
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

func (o handoverConnectivityOpts) mmeAddresses(env scenarios.Env) ([]string, error) {
	cores := env.CoreN2Addresses
	if !o.connectAll {
		cores = []string{env.FirstCore()}
	}

	addresses := make([]string, 0, len(cores))

	for _, core := range cores {
		addr, err := s1mmeAddress(core)
		if err != nil {
			return nil, err
		}

		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func (o handoverConnectivityOpts) finish(ctx context.Context, env scenarios.Env, sourceMME, targetMME, ueIP string, targetENBUEID int64) error {
	if !o.connectAll {
		return nil
	}

	want, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	return scenarios.HAHandoverDone(ctx, o.imsi, want, sourceMME, targetMME, ueIP, targetENBUEID)
}
