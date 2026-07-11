// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	staticIPv6IMSI = "001017271246703"
	staticIPv6Pin  = "fd45:0:0:11::"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/static_ip_ipv6",
		BindFlags: bindStaticIPFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBStaticIP(ctx, env, params.(*staticIPParams), staticIPv6IMSI, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return staticIPFixture(staticIPv6IMSI, staticIPv6Pin)
		},
	})
}
