// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

const (
	connDualStackIMSI     = "001017271246606"
	connDualStackTunIface = "s1enbtunds0"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_dualstack",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBConnectivityDualStack,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(connDualStackIMSI, "")},
			}
		},
	})
}

// runS1ENBConnectivityDualStack attaches a UE with PDN type IPv4v6, builds a
// dual-stack GTP-U tunnel (IPv4 plus an IPv6 link-local that SLAAC promotes to a
// global address via the UPF Router Advertisement), and verifies user-plane
// connectivity by pinging the N6 destination over both families — the 4G
// counterpart of ue/connectivity_dualstack.
func runS1ENBConnectivityDualStack(ctx context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: s1enbName, CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(connDualStackIMSI, k, opc)
	ue.RequestPDNType(eps.PDNTypeIPv4v6)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.PDNType != eps.PDNTypeIPv4v6 {
		return fmt.Errorf("expected negotiated PDN type IPv4v6 (%d), got %d", eps.PDNTypeIPv4v6, res.PDNType)
	}

	if res.UEIPv4 == "" || res.UEIPv6 == "" {
		return fmt.Errorf("dual-stack attach missing an address (v4=%q v6=%q)", res.UEIPv4, res.UEIPv6)
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UEIPv6:           res.UEIPv6 + "/64",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: connDualStackTunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	// Wait for the UPF Router Advertisement to give the TUN a global IPv6 address.
	if err := s1enb.WaitForULAAddr(connDualStackTunIface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
		return fmt.Errorf("await SLAAC address: %w", err)
	}

	v4 := exec.CommandContext(ctx, "ping", "-I", connDualStackTunIface, scenarios.DefaultPingDestination, "-c", "3", "-W", "2") // #nosec G204 -- fixed test constants
	if out, err := v4.CombinedOutput(); err != nil {
		return fmt.Errorf("ping %s (IPv4) via %s failed: %v\n%s", scenarios.DefaultPingDestination, connDualStackTunIface, err, string(out))
	}

	v6 := exec.CommandContext(ctx, "ping6", "-I", connDualStackTunIface, scenarios.DefaultPingDestinationV6, "-c", "3", "-W", "2") // #nosec G204 -- fixed test constants
	if out, err := v6.CombinedOutput(); err != nil {
		return fmt.Errorf("ping6 %s (IPv6) via %s failed: %v\n%s", scenarios.DefaultPingDestinationV6, connDualStackTunIface, err, string(out))
	}

	return nil
}
