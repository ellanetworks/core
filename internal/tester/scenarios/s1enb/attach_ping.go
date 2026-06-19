// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// attachAndPing attaches ue on the already-started eNB, builds a GTP-U tunnel
// for its default bearer on tunIface, pings the N6 destination through it, then
// tears the tunnel down. Shared by the connectivity and HA-failover scenarios.
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

	// Let the UPF program the downlink endpoint, then ping through the tunnel.
	time.Sleep(500 * time.Millisecond)

	cmd := exec.CommandContext(ctx, "ping", "-I", tunIface, scenarios.DefaultPingDestination, "-c", "3", "-W", "2") // #nosec G204 -- fixed ping; interface and destination are test config
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ping %s via %s failed: %v\n%s", scenarios.DefaultPingDestination, tunIface, err, string(out))
	}

	return nil
}
