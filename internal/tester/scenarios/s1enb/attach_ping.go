// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// attachAndPing attaches ue, builds a GTP-U tunnel for its default bearer, pings
// the N6 destination through it, then tears the tunnel down.
func attachAndPing(ctx context.Context, e *s1enb.ENB, ue *s1enb.UE, tunIface string) error {
	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("attach assigned no IPv4 address")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: tunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	// Let the UPF program the downlink endpoint before pinging.
	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, tunIface, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping via %s failed: %w", tunIface, err)
	}

	return nil
}
