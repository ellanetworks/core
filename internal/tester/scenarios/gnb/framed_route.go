// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/spf13/pflag"
)

const (
	framedRouteIMSI = "001017271246800"
	// unframedHost's subnet has no framed route and no N6 return route, so its
	// downlink is not delivered (negative check).
	framedSubnet = "192.168.60.0/24"
	framedHost   = "192.168.60.9"
	unframedHost = "192.168.99.9"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/framed_route",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runFramedRoute(ctx, env, framedRouteIMSI, framedSubnet, framedHost, unframedHost, false)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return framedRouteFixture(framedRouteIMSI, []string{framedSubnet}, nil)
		},
	})
}

func framedRouteFixture(imsi string, ipv4, ipv6 []string) scenarios.FixtureSpec {
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

// runFramedRoute establishes a session for a subscriber with a framed route,
// then verifies that a host behind the UE (an address inside the framed subnet)
// reaches the network and its reply returns down the framed-route downlink,
// while a host outside any framed route does not (TS 23.501 §5.6.14).
func runFramedRoute(ctx context.Context, env scenarios.Env, imsi, subnet, host, offRouteHost string, ipv6 bool) error {
	subs, err := buildSubscribers(1, imsi)
	if err != nil {
		return fmt.Errorf("build subscriber: %v", err)
	}

	sub := subs[0]

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	tunInterfaceName := gtpInterfaceNamePrefix + "0"

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %v", err)
	}

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait for PDU session: %v", err)
	}

	ueSess := newUE.GetPDUSession(scenarios.DefaultPDUSessionID)

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait for gNB PDU session: %v", err)
	}

	// Both hosts sit on the UE TUN; only the framed subnet has an N6 return route.
	mask := "/24"
	hostMask := "/32"
	ueCIDR := ueSess.UEIP + "/16"

	if ipv6 {
		mask = "/64"
		hostMask = "/128"
		ueCIDR = ueSess.UEIPV6 + "/64"
	}

	dst := env.PingDestination()

	if _, err := gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueCIDR,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              ueSess.QFI,
		ExtraAddrs:       []string{host + mask, offRouteHost + mask},
		ExtraRoutes:      []string{dst + hostMask},
	}); err != nil {
		return fmt.Errorf("create GTP tunnel: %v", err)
	}

	if ipv6 {
		if err := gnb.WaitForULAAddr(tunInterfaceName, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
			return fmt.Errorf("await SLAAC address: %w", err)
		}
	}

	if err := probe.RunFromAddr(ctx, host, dst, ipv6); err != nil {
		return fmt.Errorf("framed-route host %s (subnet %s) could not reach %s: %w", host, subnet, dst, err)
	}

	if err := probe.RunFromAddr(ctx, offRouteHost, dst, ipv6); err == nil {
		return fmt.Errorf("off-route host %s reached %s, but should not have", offRouteHost, dst)
	}

	return procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	})
}
