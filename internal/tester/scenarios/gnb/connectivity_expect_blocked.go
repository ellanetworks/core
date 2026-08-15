// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/connectivity_expect_blocked",
		BindFlags: func(fs *pflag.FlagSet) any {
			return bindConnectivityProbeFlags(fs)
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityExpectBlocked(ctx, env, params.(*connectivityProbeParams))
		},
		Fixture: fixtureConnectivityExpectBlocked,
	})
}

func fixtureConnectivityExpectBlocked(env scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numConnectivityParallel)

	for i := range numConnectivityParallel {
		imsi := incrementIMSI(connectivityStartIMSI, i)
		subs[i] = scenarios.DefaultSubscriberWith(imsi, "")
	}

	return scenarios.FixtureSpec{
		Subscribers: subs,
	}
}

func runConnectivityExpectBlocked(ctx context.Context, env scenarios.Env, params *connectivityProbeParams) error {
	protocol, err := parseConnectivityProbeProtocol(params.Protocol)
	if err != nil {
		return err
	}

	subs, err := buildSubscribers(numConnectivityParallel, connectivityStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	eg := errgroup.Group{}

	for i := range numConnectivityParallel {
		func() {
			eg.Go(func() error {
				ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)
				tunInterfaceName := fmt.Sprintf(gtpInterfaceNamePrefix+"%d", i)

				return runConnectivityExpectBlockedTest(
					ctx,
					ranUENGAPID,
					gNodeB,
					subs[i],
					tunInterfaceName,
					env.PingDestination(),
					env.PDUSessionType(),
					protocol,
					probeSourcePorts(params.SourcePortBase, i),
				)
			})
		}()
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error during connectivity_expect_blocked test: %v", err)
	}

	return nil
}

func runConnectivityExpectBlockedTest(
	ctx context.Context,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	sub subscriber,
	tunInterfaceName string,
	pingDestination string,
	pduSessionType uint8,
	protocol connectivityProbeProtocol,
	srcPortBase int,
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

	ueIP := session.UEIPv4 + "/16"

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

	if err := runConnectivityProbe(ctx, protocol, tunInterfaceName, pingDestination, false, srcPortBase); err == nil {
		return fmt.Errorf("%s probe to %s via %s succeeded, but was expected to fail (deny rule should be in force)", protocol, pingDestination, tunInterfaceName)
	}

	logger.Logger.Debug(
		"Probe failed as expected (traffic blocked by network rule)",
		zap.String("protocol", string(protocol)),
		zap.String("interface", tunInterfaceName),
		zap.String("destination", pingDestination),
	)

	gNodeB.CloseTunnel(session.DLTEID)

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
