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
	netRuleAllowedIMSI     = "001017271246610"
	netRuleBlockedIMSI     = "001017271246611"
	netRuleAllowedIPv6IMSI = "001017271246612"
	netRuleBlockedIPv6IMSI = "001017271246613"
)

// probeParams carries the --protocol flag for the network-rule scenarios.
type probeParams struct {
	Protocol string
}

func bindProbeFlags(fs *pflag.FlagSet) any {
	p := &probeParams{Protocol: string(probe.ICMP)}
	fs.StringVar(&p.Protocol, "protocol", p.Protocol, "probe protocol: icmp|tcp|udp")

	return p
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_allowed",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleAllowedIMSI, "s1enbnra0", true, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(netRuleAllowedIMSI, "")},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_blocked",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleBlockedIMSI, "s1enbnrb0", false, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(netRuleBlockedIMSI, "")},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_allowed_ipv6",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleAllowedIPv6IMSI, "s1enbnra6", true, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(netRuleAllowedIPv6IMSI, "")},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_expect_blocked_ipv6",
		BindFlags: bindProbeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNetworkRule(ctx, env, params.(*probeParams), netRuleBlockedIPv6IMSI, "s1enbnrb6", false, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(netRuleBlockedIPv6IMSI, "")},
			}
		},
	})
}

// runS1ENBNetworkRule attaches a 4G UE, builds a GTP-U tunnel, and probes the N6
// destination, asserting the probe is allowed or blocked according to the network
// rule the driving test installed on the policy — the 4G counterpart of
// gnb/connectivity_expect_{allowed,blocked}. ipv6 selects an IPv6 PDN + probe.
func runS1ENBNetworkRule(ctx context.Context, env scenarios.Env, params *probeParams, imsi, tunIface string, expectAllowed, ipv6 bool) error {
	proto, err := probe.ParseProtocol(params.Protocol)
	if err != nil {
		return err
	}

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

	ue := e.NewUE(imsi, k, opc)
	if ipv6 {
		ue.RequestPDNType(eps.PDNTypeIPv6)
	}

	res, err := e.Attach(ue, 15*time.Second)
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
		// Wait for the UPF Router Advertisement to give the TUN a global IPv6 address.
		if err := s1enb.WaitForULAAddr(tunIface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
			return fmt.Errorf("await SLAAC address: %w", err)
		}
	} else {
		// Let the UPF program the downlink endpoint before probing.
		time.Sleep(500 * time.Millisecond)
	}

	probeErr := probe.Run(ctx, proto, tunIface, dst, scenarios.DefaultProbePort, ipv6)

	if expectAllowed && probeErr != nil {
		return fmt.Errorf("%s probe to %s was blocked but expected to be allowed: %w", proto, dst, probeErr)
	}

	if !expectAllowed && probeErr == nil {
		return fmt.Errorf("%s probe to %s succeeded but expected to be blocked (deny rule should be in force)", proto, dst)
	}

	return nil
}
