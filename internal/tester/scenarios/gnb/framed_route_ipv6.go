// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	framedRouteIMSIv6 = "001017271246801"
	framedSubnetV6    = "fd00:60::/64"
	framedHostV6      = "fd00:60::9"
	unframedHostV6    = "fd00:99::9"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/framed_route_ipv6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runFramedRoute(ctx, env, framedRouteIMSIv6, framedSubnetV6, framedHostV6, unframedHostV6, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return framedRouteFixture(framedRouteIMSIv6, nil, []string{framedSubnetV6})
		},
	})
}
