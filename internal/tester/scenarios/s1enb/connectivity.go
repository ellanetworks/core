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
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	connStartIMSI           = "001017271246604"
	connTunIfacePrefix      = "s1enbtun"
	numConnectivityParallel = 5
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBConnectivity,
		Fixture:   fixtureS1ENBConnectivity,
	})
}

func fixtureS1ENBConnectivity(_ scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numConnectivityParallel)
	imsis := make([]string, numConnectivityParallel)

	for i := range numConnectivityParallel {
		imsi := nthIMSI(connStartIMSI, i)
		subs[i] = scenarios.DefaultSubscriberWith(imsi, "")
		imsis[i] = imsi
	}

	return scenarios.FixtureSpec{
		Subscribers:         subs,
		AssertUsageForIMSIs: imsis,
	}
}

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
		Name: s1enbName, CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	eg := errgroup.Group{}

	for i := range numConnectivityParallel {
		imsi := nthIMSI(connStartIMSI, i)
		tunIface := fmt.Sprintf("%s%d", connTunIfacePrefix, i)

		eg.Go(func() error {
			if err := runS1ENBConnectivityUE(ctx, e, imsi, k, opc, tunIface); err != nil {
				return fmt.Errorf("connectivity for %s: %w", imsi, err)
			}

			return nil
		})
	}

	return eg.Wait()
}

// runS1ENBConnectivityUE attaches a UE and pings the N6 destination, drops the UE
// to ECM-IDLE with an S1 release (the ping must then fail), re-establishes the
// bearer with a service request, and pings again (TS 24.301 §5.6.1).
func runS1ENBConnectivityUE(ctx context.Context, e *s1enb.ENB, imsi string, k, opc [16]byte, tunIface string) error {
	ue := e.NewUE(imsi, k, opc)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("attach assigned no IPv4 address")
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: tunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	// Let the UPF program the downlink endpoint before pinging.
	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, tunIface, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return err
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, 10*time.Second); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	// Let the UPF tear down the downlink path before the negative ping.
	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, tunIface, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err == nil {
		return fmt.Errorf("ping via %s succeeded after S1 release but the bearer should be suspended", tunIface)
	}

	e.CloseTunnel(res.DLTEID)

	sr, err := e.ServiceRequest(ue, res.GUTI, 10*time.Second)
	if err != nil {
		return fmt.Errorf("service request: %w", err)
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       sr.UpfAddress,
		ULTEID:           sr.ULTEID,
		DLTEID:           sr.DLTEID,
		TunInterfaceName: tunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel after service request: %w", err)
	}

	defer e.CloseTunnel(sr.DLTEID)

	// Let the UPF program the downlink endpoint before pinging.
	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, tunIface, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping after service request: %w", err)
	}

	return e.Detach(ue, sr.MMEUES1APID, sr.ENBUES1APID, 10*time.Second)
}
