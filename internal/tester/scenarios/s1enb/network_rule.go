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
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	numNetRuleParallel = 5

	netRuleAllowedIMSI     = "001017271246620"
	netRuleBlockedIMSI     = "001017271246625"
	netRuleAllowedIPv6IMSI = "001017271246630"
	netRuleBlockedIPv6IMSI = "001017271246635"
)

func netRuleFixture(startIMSI string) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numNetRuleParallel)

	for i := range numNetRuleParallel {
		subs[i] = scenarios.DefaultSubscriberWith(nthIMSI(startIMSI, i), "")
	}

	return scenarios.FixtureSpec{Subscribers: subs}
}

type probeParams struct {
	Protocol       string
	SourcePortBase int
}

func bindProbeFlags(fs *pflag.FlagSet) any {
	p := &probeParams{Protocol: string(probe.ICMP)}
	fs.StringVar(&p.Protocol, "protocol", p.Protocol, "probe protocol: icmp|tcp|udp")
	fs.IntVar(&p.SourcePortBase, "probe-source-port-base", 0, "first TCP source port to probe from; 0 uses ephemeral ports")

	return p
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_allowed",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleAllowedIMSI, "s1enbnra", true, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return netRuleFixture(netRuleAllowedIMSI)
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_blocked",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleBlockedIMSI, "s1enbnrb", false, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return netRuleFixture(netRuleBlockedIMSI)
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_allowed_ipv6",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleAllowedIPv6IMSI, "s1enbnrav6", true, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return netRuleFixture(netRuleAllowedIPv6IMSI)
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_blocked_ipv6",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleBlockedIPv6IMSI, "s1enbnrbv6", false, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return netRuleFixture(netRuleBlockedIPv6IMSI)
		},
	})
}

func runS1ENBNetworkRule(ctx context.Context, env scenarios.Env, params *probeParams, startIMSI, tunIfacePrefix string, expectAllowed, ipv6 bool) error {
	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	eg := errgroup.Group{}

	for i := range numNetRuleParallel {
		eg.Go(func() error {
			return runS1ENBNetworkRuleUE(
				ctx, e, params,
				nthIMSI(startIMSI, i),
				fmt.Sprintf("%s%d", tunIfacePrefix, i),
				netRuleSourcePorts(params.SourcePortBase, i),
				expectAllowed, ipv6,
			)
		})
	}

	return eg.Wait()
}

func netRuleSourcePorts(base, ue int) int {
	if base == 0 {
		return 0
	}

	return base + ue*probe.AttemptCount
}

func runS1ENBNetworkRuleUE(ctx context.Context, e *s1enb.ENB, params *probeParams, imsi, tunIface string, srcPortBase int, expectAllowed, ipv6 bool) error {
	proto, err := probe.ParseProtocol(params.Protocol)
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(imsi, k, opc)
	if ipv6 {
		ue.RequestPDNType(uint8(eps.PDNTypeIPv6))
	}

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	tun := &s1enb.TunnelOpts{
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: tunIface,
	}

	dst := scenarios.DefaultPingDestination

	if ipv6 {
		if res.UEIPv6 == "" {
			return fmt.Errorf("IPv6 attach assigned no IPv6 interface identifier")
		}

		tun.UEIPv6 = res.UEIPv6 + "/64"
		dst = scenarios.DefaultPingDestinationV6
	} else {
		if res.UEIPv4 == "" {
			return fmt.Errorf("attach assigned no IPv4 address")
		}

		tun.UEIPv4 = res.UEIPv4 + "/16"
	}

	if err := e.AddTunnel(tun); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	if ipv6 {
		if err := s1enb.WaitForULAAddr(tunIface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
			return fmt.Errorf("await SLAAC address: %w", err)
		}
	}

	awaitDownlinkReady()

	probeErr := probe.RunFromSourcePorts(ctx, proto, tunIface, dst, scenarios.DefaultProbePort, ipv6, srcPortBase)

	if expectAllowed && probeErr != nil {
		return fmt.Errorf("%s probe to %s was blocked but expected to be allowed: %w", proto, dst, probeErr)
	}

	if !expectAllowed && probeErr == nil {
		return fmt.Errorf("%s probe to %s succeeded but expected to be blocked (deny rule should be in force)", proto, dst)
	}

	return nil
}
