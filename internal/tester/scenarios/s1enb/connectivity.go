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
	"github.com/spf13/pflag"
)

const (
	connIMSI     = "001017271246604"
	connTunIface = "s1enbtun0"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBConnectivity,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(connIMSI, "")},
			}
		},
	})
}

// runS1ENBConnectivity attaches a UE, builds a GTP-U tunnel for its default
// bearer, and verifies user-plane connectivity by pinging the N6 destination
// through the tunnel — the 4G counterpart of ue/connectivity.
func runS1ENBConnectivity(ctx context.Context, env scenarios.Env, _ any) error {
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
		Name: "Ella-Core-Tester-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(connIMSI, k, opc)

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
		TunInterfaceName: connTunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	// Let the UPF program the downlink endpoint, then ping through the tunnel.
	time.Sleep(500 * time.Millisecond)

	cmd := exec.CommandContext(ctx, "ping", "-I", connTunIface, scenarios.DefaultPingDestination, "-c", "3", "-W", "2") // #nosec G204 -- fixed ping; interface and destination are test config
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ping %s via %s failed: %v\n%s", scenarios.DefaultPingDestination, connTunIface, err, string(out))
	}

	return nil
}
