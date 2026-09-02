// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	bufferedS1StartIMSI      = "001017271248101"
	bufferedS1TunIfacePrefix = "s1buftun"

	bufferedS1PollInterval = 500 * time.Millisecond
	bufferedS1PollDeadline = 10 * time.Second
	bufferedS1Settle       = 500 * time.Millisecond
	bufferedS1DstPort      = 59998
	bufferedS1PageDeadline = 5 * time.Second
	// bufferedS1DrainPaging bounds each attempt to consume a stale Paging;
	// the frames are already recorded, so this only has to be non-zero.
	bufferedS1DrainPaging = 10 * time.Millisecond
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/buffered_downlink",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runS1ENBBufferedDownlink(ctx, env)
		},
		Fixture: fixtureS1ENBBufferedDownlink,
	})
}

func fixtureS1ENBBufferedDownlink(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(bufferedS1StartIMSI, ""),
			scenarios.DefaultSubscriberWith(nthIMSI(bufferedS1StartIMSI, 1), ""),
		},
	}
}

// runS1ENBBufferedDownlink is the EPS counterpart of gnb/buffered_downlink:
// downlink datagrams arriving while the receiving UE is in ECM-IDLE — starting
// with the first, whose loss is unrecoverable for single-shot traffic — are
// delivered after the UE answers the page. It covers what the 5G scenario
// cannot: that the MME actually pages, and that the re-injected packets are
// encapsulated for S1-U, i.e. as plain G-PDUs with no PDU Session Container
// (that extension header is N3/N9-only, TS 38.415).
//
// The sender is another UE, so the traffic takes the local switch path;
// delivery is asserted with the eNB's G-PDU counter on UE-B's downlink tunnel,
// because both UE addresses are local to the tester and the kernel stack
// cannot tell the two ends apart. Requires local switching enabled.
func runS1ENBBufferedDownlink(ctx context.Context, env scenarios.Env) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENBWithDatapath(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	imsiA := bufferedS1StartIMSI
	imsiB := nthIMSI(bufferedS1StartIMSI, 1)
	tunA := fmt.Sprintf("%s%d", bufferedS1TunIfacePrefix, 0)
	tunB := fmt.Sprintf("%s%d", bufferedS1TunIfacePrefix, 1)

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

	// Keep UE-A active: its session may have gone idle during UE-B's attach,
	// and the buffered datagrams have to be sent from a connected UE.
	if err := probe.Run(ctx, probe.ICMP, tunA, env.PingDestination(), scenarios.DefaultProbePort, false); err != nil {
		logger.Logger.Debug("keepalive ping from UE-A to N6 failed (session may be idle)", zap.Error(err))
	}

	rxBefore := e.TunnelRXCount(resB.DLTEID)

	// Drop UE-B to ECM-IDLE: its downlink FAR flips to BUFF|NOCP.
	if err := e.ReleaseContext(resB.mmeUES1APID, resB.enbUES1APID, s1enb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release UE-B to ECM-IDLE: %w", err)
	}

	time.Sleep(bufferedS1Settle)

	// Consume any Paging recorded earlier, so the wait below can only be
	// satisfied by the page this scenario's own datagram triggers.
	for {
		if _, err := e.WaitForMessage(s1enb.NoUEID, s1enb.Initiating, s1ap.ProcPaging, bufferedS1DrainPaging); err != nil {
			break
		}
	}

	// Two datagrams of the idle window: the first is the single-shot case
	// whose loss is unrecoverable, the second covers the queue beyond depth
	// one while the page is in flight.
	if err := probe.SendUDPOneWay(ctx, tunA, resB.UEIPv4, bufferedS1DstPort, []byte("first datagram to an idle ue")); err != nil {
		return fmt.Errorf("send first datagram from UE-A: %w", err)
	}

	// The first buffered packet must make the MME page UE-B (TS 23.401
	// §5.3.4.3): without a page there is nothing for the UE to answer, and
	// the queue would only ever be drained by the TTL sweeper.
	if _, err := e.WaitForMessage(s1enb.NoUEID, s1enb.Initiating, s1ap.ProcPaging, bufferedS1PageDeadline); err != nil {
		return fmt.Errorf("await S1AP Paging for the buffered downlink packet: %w", err)
	}

	if err := probe.SendUDPOneWay(ctx, tunA, resB.UEIPv4, bufferedS1DstPort, []byte("second datagram while paging")); err != nil {
		return fmt.Errorf("send second datagram from UE-A: %w", err)
	}

	time.Sleep(bufferedS1Settle)

	// Neither datagram may reach the eNB yet. The tunnel is deliberately kept
	// across the idle period (see the pinned TEID below), so a packet the UPF
	// forwarded instead of buffering would land on this same counter and make
	// the delivery check below pass for the wrong reason.
	if got := e.TunnelRXCount(resB.DLTEID); got != rxBefore {
		return fmt.Errorf("eNB received %d G-PDUs on DL TEID %d while UE-B was ECM-IDLE (was %d): the UPF forwarded to a suspended bearer instead of buffering", got, resB.DLTEID, rxBefore)
	}

	// UE-B answers the page: the FAR flips to FORW and the buffered
	// datagrams are re-injected through the downlink pipeline. Pin the
	// downlink TEID to the one the existing tunnel was built with, so the
	// re-injected datagrams cannot arrive before a replacement tunnel is
	// registered — the core drains as soon as it has processed the Initial
	// Context Setup Response, which races tearing the tunnel down and
	// building a new one here.
	sr, err := e.ServiceRequest(resB.ue, resB.guti, releaseTimeout, &s1enb.ServiceRequestOpts{DLTEID: resB.DLTEID})
	if err != nil {
		return fmt.Errorf("service request from ECM-IDLE: %w", err)
	}

	// A third datagram after the resume: it must flow directly, proving the
	// live path works alongside the drained queue.
	if err := probe.SendUDPOneWay(ctx, tunA, resB.UEIPv4, bufferedS1DstPort, []byte("third datagram after resume")); err != nil {
		return fmt.Errorf("send third datagram from UE-A: %w", err)
	}

	// Expect the two buffered datagrams plus the live one on UE-B's
	// downlink tunnel.
	if err := awaitS1TunnelRX(e, resB.DLTEID, rxBefore+3, bufferedS1PollDeadline); err != nil {
		return err
	}

	// S1-U carries no PDU Session Container, so the re-injected packets must
	// arrive as plain G-PDUs. The eNB tolerates extension headers, so this
	// is asserted rather than implied by the counter above.
	if got := e.TunnelRXExtHeaderCount(resB.DLTEID); got != 0 {
		return fmt.Errorf("%d G-PDUs on UE-B's S1-U tunnel carried a GTP-U extension header, want 0: the UPF encapsulated a 4G bearer as if it were N3", got)
	}

	logger.Logger.Info("buffered downlink scenario completed: datagrams buffered while ECM-IDLE delivered after the page",
		zap.String("ue_a", resA.UEIPv4),
		zap.String("ue_b", resB.UEIPv4),
	)

	if err := e.Detach(resA.ue, resA.mmeUES1APID, resA.enbUES1APID, releaseTimeout); err != nil {
		return fmt.Errorf("UE-A detach: %w", err)
	}

	if err := e.Detach(resB.ue, sr.MMEUES1APID, sr.ENBUES1APID, releaseTimeout); err != nil {
		return fmt.Errorf("UE-B detach: %w", err)
	}

	return nil
}

// awaitS1TunnelRX polls the eNB's G-PDU receive counter for a DL TEID until it
// reaches want, or the deadline passes.
func awaitS1TunnelRX(e *s1enb.ENB, dlteid uint32, want uint64, deadline time.Duration) error {
	for deadline > 0 {
		if e.TunnelRXCount(dlteid) >= want {
			return nil
		}

		sleep := bufferedS1PollInterval
		if deadline < sleep {
			sleep = deadline
		}

		time.Sleep(sleep)
		deadline -= sleep
	}

	if got := e.TunnelRXCount(dlteid); got < want {
		return fmt.Errorf("eNB received %d G-PDUs on DL TEID %d, want at least %d: buffered datagrams were not delivered", got, dlteid, want)
	}

	return nil
}
