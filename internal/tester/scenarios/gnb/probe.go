// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

// connectivityProbeProtocol selects the wire protocol used by runConnectivityProbe.
type connectivityProbeProtocol = probe.Protocol

// connectivityProbeParams holds flags shared by every scenario that drives a
// single round of connectivity validation.
type connectivityProbeParams struct {
	Protocol string
}

// bindConnectivityProbeFlags registers the --protocol flag and returns a params
// struct populated with its default.
func bindConnectivityProbeFlags(fs *pflag.FlagSet) *connectivityProbeParams {
	p := &connectivityProbeParams{Protocol: string(probe.ICMP)}
	fs.StringVar(&p.Protocol, "protocol", p.Protocol, "probe protocol: icmp|tcp|udp")

	return p
}

func parseConnectivityProbeProtocol(s string) (connectivityProbeProtocol, error) {
	return probe.ParseProtocol(s)
}

// runConnectivityProbe issues a probe of the given protocol from tun to dst on
// the default probe port. Returns nil on success.
func runConnectivityProbe(ctx context.Context, protocol connectivityProbeProtocol, tun, dst string, ipv6 bool) error {
	return probe.Run(ctx, protocol, tun, dst, scenarios.DefaultProbePort, ipv6)
}
