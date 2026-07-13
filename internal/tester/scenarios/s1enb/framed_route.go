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
	framedRouteIMSI    = "001017271246810"
	framedRouteIMSIv6  = "001017271246811"
	framedTunIface     = "s1enbtunfr0"
	framedSubnet       = "192.168.60.0/24"
	framedHost         = "192.168.60.9"
	unframedHost       = "192.168.99.9"
	framedSubnetV6     = "fd00:60::/64"
	framedHostV6       = "fd00:60::9"
	unframedHostV6     = "fd00:99::9"
	framedProgramDelay = 500 * time.Millisecond
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/framed_route",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runS1ENBFramedRoute(ctx, env, framedRouteIMSI, framedSubnet, framedHost, unframedHost, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return s1enbFramedRouteFixture(framedRouteIMSI, []string{framedSubnet}, nil)
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/framed_route_ipv6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runS1ENBFramedRoute(ctx, env, framedRouteIMSIv6, framedSubnetV6, framedHostV6, unframedHostV6, true)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return s1enbFramedRouteFixture(framedRouteIMSIv6, nil, []string{framedSubnetV6})
		},
	})
}

func s1enbFramedRouteFixture(imsi string, ipv4, ipv6 []string) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(imsi, "")},
		FramedRoutes: []scenarios.FramedRouteSpec{{
			IMSI:        imsi,
			DataNetwork: scenarios.DefaultDNN,
			IPv4:        ipv4,
			IPv6:        ipv6,
		}},
		AssertUsageForIMSIs: []string{imsi},
	}
}

// runS1ENBFramedRoute attaches a 4G UE whose subscriber owns a framed route,
// then verifies that a host behind the UE (an address inside the framed subnet)
// reaches the network and its reply returns down the framed-route downlink,
// while a host outside any framed route does not (TS 23.501 §5.6.14).
func runS1ENBFramedRoute(ctx context.Context, env scenarios.Env, imsi, subnet, host, offRouteHost string, ipv6 bool) error {
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

	mask := "/24"
	hostMask := "/32"
	ueCIDR := res.UEIPv4 + "/16"

	if ipv6 {
		mask = "/64"
		hostMask = "/128"
		ueCIDR = res.UEIPv6 + "/64"
	}

	dst := env.PingDestination()

	tun := &s1enb.TunnelOpts{
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: framedTunIface,
		ExtraAddrs:       []string{host + mask, offRouteHost + mask},
		ExtraRoutes:      []string{dst + hostMask},
	}

	if ipv6 {
		tun.UEIPv6 = ueCIDR
	} else {
		tun.UEIPv4 = ueCIDR
	}

	if err := e.AddTunnel(tun); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	if ipv6 {
		if err := s1enb.WaitForULAAddr(framedTunIface, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
			return fmt.Errorf("await SLAAC address: %w", err)
		}
	} else {
		// Let the UPF program the downlink endpoint before probing.
		time.Sleep(framedProgramDelay)
	}

	if err := probe.RunFromAddr(ctx, host, dst, ipv6); err != nil {
		return fmt.Errorf("framed-route host %s (subnet %s) could not reach %s: %w", host, subnet, dst, err)
	}

	if err := probe.RunFromAddr(ctx, offRouteHost, dst, ipv6); err == nil {
		return fmt.Errorf("off-route host %s reached %s, but should not have", offRouteHost, dst)
	}

	return e.Detach(ue, res.MMEUES1APID, res.ENBUES1APID, 10*time.Second)
}
