// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

// This file adapts the shared internal/tester/probe package to the names the ue
// scenarios use, so the probe implementation has a single source shared with the
// 4G (s1enb) scenarios.

const (
	probeAttemptCount   = probe.AttemptCount
	probeAttemptTimeout = probe.AttemptTimeout
)

// makeProbePayload returns a deterministic payload of exactly n bytes.
func makeProbePayload(n int) []byte { return probe.MakePayload(n) }

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

func sendTCPProbe(ctx context.Context, tun, dst string, port, count int, perAttemptTimeout time.Duration, payload []byte) error {
	return probe.SendTCP(ctx, tun, dst, port, count, perAttemptTimeout, payload)
}

func sendUDPProbe(ctx context.Context, tun, dst string, port, count int, perAttemptTimeout time.Duration, payload []byte) error {
	return probe.SendUDP(ctx, tun, dst, port, count, perAttemptTimeout, payload)
}
