// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	ipv6StartIMSI = "001017271246550"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/connectivity_ipv6",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runConnectivityIPv6(ctx, env, params)
		},
		Fixture: fixtureConnectivityIPv6,
	})
}

func fixtureConnectivityIPv6(env scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numConnectivityParallel)
	imsis := make([]string, numConnectivityParallel)

	for i := range numConnectivityParallel {
		imsi := incrementIMSI(ipv6StartIMSI, i)
		subs[i] = scenarios.DefaultSubscriberWith(imsi, "")
		imsis[i] = imsi
	}

	return scenarios.FixtureSpec{
		Subscribers:         subs,
		AssertUsageForIMSIs: imsis,
	}
}

func runConnectivityIPv6(ctx context.Context, env scenarios.Env, _ any) error {
	subs, err := buildSubscribers(numConnectivityParallel, ipv6StartIMSI)
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
				tunInterfaceName := fmt.Sprintf(gtpInterfaceNamePrefix+"v6%d", i)

				return runConnectivityIPv6Test(
					ctx,
					ranUENGAPID,
					gNodeB,
					subs[i],
					tunInterfaceName,
					env.PDUSessionType(),
				)
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during IPv6 connectivity test: %v", err)
	}

	return nil
}

func runConnectivityIPv6Test(
	ctx context.Context,
	ranUENGAPID int64,
	gNodeB *gnb.GnodeB,
	sub subscriber,
	tunInterfaceName string,
	pduSessionType uint8,
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

	ueAmbr := gNodeB.GetUEAmbr(ranUENGAPID)

	err = validate.UEAmbr(ueAmbr, &validate.ExpectedUEAmbr{
		UplinkBps:   100_000_000,
		DownlinkBps: 100_000_000,
	})
	if err != nil {
		return fmt.Errorf("UE AMBR validation failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration Procedure (IPv6)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

	session := registration.Session

	ueIP := session.UEIPv6 + "/64"

	err = validate.PDUSessionInformation(session, &validate.ExpectedPDUSessionInformation{
		FiveQI: 9,
		ARP:    15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("NGAP QoS validation failed: %v", err)
	}

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP,
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, session.DLTEID, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session (IPv6)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", session.UpfAddress),
		zap.Uint32("UL TEID", session.ULTEID),
		zap.Uint32("DL TEID", session.DLTEID),
	)

	err = gnb.WaitForULAAddr(tunInterfaceName, scenarios.DefaultUEIPv6Pool, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for ULA address on %s: %v", tunInterfaceName, err)
	}

	if err := probe.Run(ctx, probe.ICMP, tunInterfaceName, scenarios.DefaultPingDestinationV6, scenarios.DefaultProbePort, true); err != nil {
		return fmt.Errorf("ping6 via %s failed after initial registration: %w", tunInterfaceName, err)
	}

	logger.Logger.Debug(
		"Ping6 successful (IPv6)",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestinationV6),
	)

	pduSessionStatus := []uint8{scenarios.DefaultPDUSessionID}

	err = gNodeB.ReleaseContext(newUE, ranUENGAPID, pduSessionStatus, gnb.CauseUserInactivity, releaseTimeout)
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed UE Context Release Procedure (IPv6)",
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	if err := probe.Run(ctx, probe.ICMP, tunInterfaceName, scenarios.DefaultPingDestinationV6, scenarios.DefaultProbePort, true); err == nil {
		return fmt.Errorf("ping6 via %s succeeded, but was expected to fail after UE Context Release", tunInterfaceName)
	}

	logger.Logger.Debug(
		"Ping6 failed as expected after UE Context Release (IPv6)",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestinationV6),
	)

	serviceRequest, err := gNodeB.ServiceRequest(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("service request procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Service Request Procedure (IPv6)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(ranUENGAPID)),
	)

	gNodeB.CloseTunnel(session.DLTEID)

	session = serviceRequest.Session

	err = gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP,
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel after service request (name: %s, DL TEID: %d): %v", tunInterfaceName, session.DLTEID, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session after Service Request (IPv6)",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", session.UpfAddress),
		zap.Uint32("UL TEID", session.ULTEID),
		zap.Uint32("DL TEID", session.DLTEID),
	)

	err = gnb.WaitForULAAddr(tunInterfaceName, scenarios.DefaultUEIPv6Pool, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for ULA address on %s after service request: %v", tunInterfaceName, err)
	}

	if err := probe.Run(ctx, probe.ICMP, tunInterfaceName, scenarios.DefaultPingDestinationV6, scenarios.DefaultProbePort, true); err != nil {
		return fmt.Errorf("ping6 via %s failed after service request: %w", tunInterfaceName, err)
	}

	logger.Logger.Debug(
		"Ping6 successful after Service Request (IPv6)",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestinationV6),
	)

	logger.Logger.Debug("client-side usage assertion skipped; integration test will verify",
		zap.String("IMSI", sub.IMSI),
	)

	gNodeB.CloseTunnel(session.DLTEID)

	logger.Logger.Debug(
		"Closed GTP tunnel (IPv6)",
		zap.String("interface", tunInterfaceName),
	)

	err = gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout)
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
