// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	errorIndS1StartIMSI      = "001017271248301"
	errorIndS1TunIfacePrefix = "s1eitun"

	errorIndS1DstPort         = 59997
	errorIndS1Settle          = 500 * time.Millisecond
	errorIndS1ReleaseDeadline = 10 * time.Second
	errorIndS1PollDeadline    = 10 * time.Second
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/error_indication",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runS1ENBErrorIndication(ctx, env)
		},
		Fixture: fixtureS1ENBErrorIndication,
	})
}

func fixtureS1ENBErrorIndication(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(errorIndS1StartIMSI, ""),
			scenarios.DefaultSubscriberWith(nthIMSI(errorIndS1StartIMSI, 1), ""),
		},
	}
}

func runS1ENBErrorIndication(ctx context.Context, env scenarios.Env) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	imsiA := errorIndS1StartIMSI
	imsiB := nthIMSI(errorIndS1StartIMSI, 1)
	tunA := fmt.Sprintf("%s%d", errorIndS1TunIfacePrefix, 0)
	tunB := fmt.Sprintf("%s%d", errorIndS1TunIfacePrefix, 1)

	resA, err := attachAndTunnelS1(e, imsiA, k, opc, tunA)
	if err != nil {
		return fmt.Errorf("UE-A attach: %w", err)
	}

	defer e.CloseTunnel(resA.DLTEID)

	resB, err := attachAndTunnelS1(e, imsiB, k, opc, tunB)
	if err != nil {
		return fmt.Errorf("UE-B attach: %w", err)
	}

	defer e.CloseTunnel(resB.DLTEID)

	if resB.guti == nil {
		return fmt.Errorf("UE-B attached without a GUTI, cannot service-request")
	}

	sgw, err := netip.ParseAddr(resB.UpfAddress)
	if err != nil {
		return fmt.Errorf("parse the S-GW S1-U address %q: %w", resB.UpfAddress, err)
	}

	if err := scenarios.AwaitDownlinkDelivery(
		func() error {
			return probe.SendUDPOneWay(ctx, tunA, resB.UEIPv4, errorIndS1DstPort, []byte("datagram before the error indication"))
		},
		func() uint64 { return e.TunnelRXCount(resB.DLTEID) },
		"UE-to-UE downlink before the Error Indication",
	); err != nil {
		return err
	}

	if err := e.SendGTPUErrorIndication(resB.DLTEID, sgw, e.N3Address()); err != nil {
		return fmt.Errorf("send a GTP-U Error Indication for TEID %#x: %w", resB.DLTEID, err)
	}

	cmd, err := e.WaitForUEContextReleaseCommand(resB.enbUES1APID, errorIndS1ReleaseDeadline)
	if err != nil {
		return fmt.Errorf("the MME did not release S1 after the Error Indication: %w", err)
	}

	if err := e.SendUEContextReleaseComplete(int64(cmd.UES1APIDs.MMEUES1APID), int64(cmd.UES1APIDs.ENBUES1APID)); err != nil {
		return fmt.Errorf("acknowledge the UE Context Release: %w", err)
	}

	rxBefore := e.TunnelRXCount(resB.DLTEID)

	sr, err := e.ServiceRequest(resB.ue, resB.guti, errorIndS1ReleaseDeadline, &s1enb.ServiceRequestOpts{DLTEID: resB.DLTEID})
	if err != nil {
		return fmt.Errorf("service request after the error indication: %w", err)
	}

	if err := probe.SendUDPOneWay(ctx, tunA, resB.UEIPv4, errorIndS1DstPort, []byte("datagram after the tunnel was re-established")); err != nil {
		return fmt.Errorf("send the second datagram from UE-A: %w", err)
	}

	if err := awaitS1TunnelRX(e, resB.DLTEID, rxBefore+1, errorIndS1PollDeadline); err != nil {
		return err
	}

	logger.Logger.Info("error indication scenario completed: the MME released S1 and the bearer was re-established",
		zap.Uint32("dl_teid", resB.DLTEID), zap.Int64("enb_ue_s1ap_id", sr.ENBUES1APID))

	return nil
}
