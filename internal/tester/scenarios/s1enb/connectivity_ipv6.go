// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

const (
	connIPv6IMSI     = "001017271246607"
	connIPv6TunIface = "s1enbtunv60"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_ipv6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBConnectivityIPv6,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(connIPv6IMSI, "")},
			}
		},
	})
}

// runS1ENBConnectivityIPv6 attaches an IPv6 UE and pings the N6 destination. The
// PDN IID's link-local is promoted to a global address by the UPF Router
// Advertisement before the ping.
func runS1ENBConnectivityIPv6(ctx context.Context, env scenarios.Env, _ any) error {
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

	ue := e.NewUE(connIPv6IMSI, k, opc)
	ue.RequestPDNType(eps.PDNTypeIPv6)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.PDNType != eps.PDNTypeIPv6 {
		return fmt.Errorf("expected negotiated PDN type IPv6 (%d), got %d", eps.PDNTypeIPv6, res.PDNType)
	}

	if res.UEIPv6 == "" {
		return fmt.Errorf("IPv6 attach assigned no IPv6 interface identifier")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv6:           res.UEIPv6 + "/64",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: connIPv6TunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	if err := s1enb.WaitForULAAddr(connIPv6TunIface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
		return fmt.Errorf("await SLAAC address: %w", err)
	}

	if err := probe.Run(ctx, probe.ICMP, connIPv6TunIface, scenarios.DefaultPingDestinationV6, scenarios.DefaultProbePort, true); err != nil {
		return fmt.Errorf("ping6 via %s failed: %w", connIPv6TunIface, err)
	}

	return nil
}
