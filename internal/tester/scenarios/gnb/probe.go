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
	Protocol       string
	SourcePortBase int
}

func bindConnectivityProbeFlags(fs *pflag.FlagSet) *connectivityProbeParams {
	p := &connectivityProbeParams{Protocol: string(probe.ICMP)}
	fs.StringVar(&p.Protocol, "protocol", p.Protocol, "probe protocol: icmp|tcp|udp")
	fs.IntVar(&p.SourcePortBase, "probe-source-port-base", 0, "first TCP source port to probe from; 0 uses ephemeral ports")

	return p
}

func probeSourcePorts(base, ue int) int {
	if base == 0 {
		return 0
	}

	return base + ue*probe.AttemptCount
}

func parseConnectivityProbeProtocol(s string) (connectivityProbeProtocol, error) {
	return probe.ParseProtocol(s)
}

func runConnectivityProbe(ctx context.Context, protocol connectivityProbeProtocol, tun, dst string, ipv6 bool, srcPortBase int) error {
	return probe.RunFromSourcePorts(ctx, protocol, tun, dst, scenarios.DefaultProbePort, ipv6, srcPortBase)
}
