// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func init() {
	for _, v := range []struct {
		name           string
		startIMSI      string
		tunIfacePrefix string
		expectAllowed  bool
		ipv6           bool
	}{
		{"gnb/connectivity_expect_allowed", connectivityStartIMSI, gtpInterfaceNamePrefix, true, false},
		{"gnb/connectivity_expect_blocked", connectivityStartIMSI, gtpInterfaceNamePrefix, false, false},
		{"gnb/connectivity_expect_allowed_ipv6", ipv6StartIMSI, gtpInterfaceNamePrefix + "v6", true, true},
		{"gnb/connectivity_expect_blocked_ipv6", ipv6StartIMSI, gtpInterfaceNamePrefix + "v6", false, true},
	} {
		scenarios.Register(scenarios.Scenario{
			Name: v.name,
			BindFlags: func(fs *pflag.FlagSet) any {
				return bindConnectivityProbeFlags(fs)
			},
			Run: func(ctx context.Context, env scenarios.Env, params any) error {
				return runConnectivityExpect(ctx, env, params.(*connectivityProbeParams),
					v.startIMSI, v.tunIfacePrefix, v.expectAllowed, v.ipv6)
			},
			Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
				return connectivityExpectFixture(v.startIMSI)
			},
		})
	}
}

func connectivityExpectFixture(startIMSI string) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numConnectivityParallel)

	for i := range numConnectivityParallel {
		subs[i] = scenarios.DefaultSubscriberWith(incrementIMSI(startIMSI, i), "")
	}

	return scenarios.FixtureSpec{Subscribers: subs}
}

func runConnectivityExpect(
	ctx context.Context,
	env scenarios.Env,
	params *connectivityProbeParams,
	startIMSI, tunIfacePrefix string,
	expectAllowed, ipv6 bool,
) error {
	protocol, err := parseConnectivityProbeProtocol(params.Protocol)
	if err != nil {
		return err
	}

	subs, err := buildSubscribers(numConnectivityParallel, startIMSI)
	if err != nil {
		return err
	}

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	eg := errgroup.Group{}

	for i := range numConnectivityParallel {
		eg.Go(func() error {
			return runConnectivityExpectUE(
				ctx,
				int64(scenarios.DefaultRANUENGAPID)+int64(i),
				gNodeB,
				subs[i],
				fmt.Sprintf("%s%d", tunIfacePrefix, i),
				env.PDUSessionType(),
				protocol,
				probeSourcePorts(params.SourcePortBase, i),
				env.PingDestination(),
				expectAllowed,
				ipv6,
			)
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error during %s test: %v", connectivityExpectLabel(expectAllowed, ipv6), err)
	}

	return nil
}

func connectivityExpectLabel(expectAllowed, ipv6 bool) string {
	name := "connectivity_expect_blocked"
	if expectAllowed {
		name = "connectivity_expect_allowed"
	}

	if ipv6 {
		name += "_ipv6"
	}

	return name
}

func runConnectivityExpectUE(
	ctx context.Context,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	sub subscriber,
	tunInterfaceName string,
	pduSessionType uint8,
	protocol connectivityProbeProtocol,
	srcPortBase int,
	pingDestination string,
	expectAllowed, ipv6 bool,
) error {
	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, pduSessionType)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	registration, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	session := registration.Session

	dst := pingDestination
	ueIP := session.UEIPv4 + "/16"

	if ipv6 {
		dst = scenarios.DefaultPingDestinationV6
		ueIP = session.UEIPv6 + "/64"
	}

	if err := gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP,
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	}); err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, session.DLTEID, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
	)

	if ipv6 {
		if err := gnb.WaitForULAAddr(tunInterfaceName, scenarios.DefaultUEIPv6Pool, 5*time.Second); err != nil {
			return fmt.Errorf("timeout waiting for ULA address on %s: %v", tunInterfaceName, err)
		}
	}

	awaitDownlinkReady()

	probeErr := runConnectivityProbe(ctx, protocol, tunInterfaceName, dst, ipv6, srcPortBase)

	if expectAllowed && probeErr != nil {
		return fmt.Errorf("%s probe to %s via %s failed, but was expected to succeed: %v", protocol, dst, tunInterfaceName, probeErr)
	}

	if !expectAllowed && probeErr == nil {
		return fmt.Errorf("%s probe to %s via %s succeeded, but was expected to fail (deny rule should be in force)", protocol, dst, tunInterfaceName)
	}

	gNodeB.CloseTunnel(session.DLTEID)

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
