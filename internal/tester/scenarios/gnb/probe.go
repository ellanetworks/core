// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

type connectivityProbeProtocol = probe.Protocol

type connectivityProbeParams struct {
	Protocol string
}

func bindConnectivityProbeFlags(fs *pflag.FlagSet) *connectivityProbeParams {
	p := &connectivityProbeParams{Protocol: string(probe.ICMP)}
	fs.StringVar(&p.Protocol, "protocol", p.Protocol, "probe protocol: icmp|tcp|udp")

	return p
}

func parseConnectivityProbeProtocol(s string) (connectivityProbeProtocol, error) {
	return probe.ParseProtocol(s)
}

func runConnectivityProbe(ctx context.Context, protocol connectivityProbeProtocol, tun, dst string, ipv6 bool) error {
	return probe.Run(ctx, protocol, tun, dst, scenarios.DefaultProbePort, ipv6)
}
