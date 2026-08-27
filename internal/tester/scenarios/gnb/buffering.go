// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	bufferedStartIMSI = "001017271247101"

	bufferedPollInterval = 500 * time.Millisecond
	bufferedPollDeadline = 10 * time.Second
	bufferedSettle       = 500 * time.Millisecond
	bufferedDstPort      = 59999
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/buffered_downlink",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runBufferedDownlink(ctx, env)
		},
		Fixture: fixtureBufferedDownlink,
	})
}

func fixtureBufferedDownlink(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(bufferedStartIMSI, ""),
			scenarios.DefaultSubscriberWith(incrementIMSI(bufferedStartIMSI, 1), ""),
		},
	}
}

// runBufferedDownlink asserts the 3GPP buffering behaviour for an idle UE
// (TS 23.501 §5.8.2.2.1): downlink datagrams arriving while the receiving
// UE is in CM-IDLE — starting with the first, whose loss is unrecoverable
// for single-shot traffic — are delivered after the UE answers the page.
// The sender is another UE, so the traffic takes the local switch path.
// Delivery is asserted with the gNB's G-PDU counter on UE-B's downlink
// tunnel: a UDP listener does not work here, because both UE addresses are
// local to the tester and the kernel stack cannot tell the two ends apart.
// Requires local switching enabled.
func runBufferedDownlink(ctx context.Context, env scenarios.Env) error {
	subs, err := buildSubscribers(2, bufferedStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	pduSessionType := env.PDUSessionType()

	ranUENGAPID_A := int64(scenarios.DefaultRANUENGAPID)
	ranUENGAPID_B := int64(scenarios.DefaultRANUENGAPID) + 1
	tunA := fmt.Sprintf(gtpInterfaceNamePrefix+"%d", 0)
	tunB := fmt.Sprintf(gtpInterfaceNamePrefix+"%d", 1)

	regA, ueA, err := registerAndTunnel(gNodeB, subs[0], ranUENGAPID_A, tunA, pduSessionType)
	if err != nil {
		return fmt.Errorf("UE-A registration: %w", err)
	}

	regB, ueB, err := registerAndTunnel(gNodeB, subs[1], ranUENGAPID_B, tunB, pduSessionType)
	if err != nil {
		return fmt.Errorf("UE-B registration: %w", err)
	}

	if regB.UEIPv4 == "" {
		return fmt.Errorf("UE-B was not assigned an IPv4 address")
	}

	if err := probe.Run(ctx, probe.ICMP, tunA, env.PingDestination(), scenarios.DefaultProbePort, false); err != nil {
		logger.Logger.Debug("keepalive ping from UE-A to N6 failed (session may be idle)", zap.Error(err))
	}

	rxBefore := gNodeB.TunnelRXCount(regB.DLTEID)

	// Drop UE-B to CM-IDLE: its downlink FAR flips to BUFF|NOCP.
	if err := gNodeB.ReleaseContext(ueB, ranUENGAPID_B, []uint8{scenarios.DefaultPDUSessionID}, releaseTimeout); err != nil {
		return fmt.Errorf("release UE-B to idle: %w", err)
	}

	time.Sleep(bufferedSettle)

	// Two datagrams of the idle window: the first is the single-shot case
	// whose loss is unrecoverable, the second covers the queue beyond
	// depth one while the page is in flight.
	if err := udpSendFromTun(ctx, tunA, regB.UEIPv4, []byte("first datagram to an idle ue")); err != nil {
		return fmt.Errorf("send first datagram from UE-A: %w", err)
	}

	time.Sleep(bufferedSettle)

	if err := udpSendFromTun(ctx, tunA, regB.UEIPv4, []byte("second datagram while paging")); err != nil {
		return fmt.Errorf("send second datagram from UE-A: %w", err)
	}

	time.Sleep(bufferedSettle)

	// UE-B answers the page: the FAR flips to FORW and the buffered
	// datagrams are re-injected through the downlink pipeline.
	serviceRequest, err := gNodeB.ServiceRequest(ueB, ranUENGAPID_B, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("service request from idle: %w", err)
	}

	sessionB := serviceRequest.Session

	// A service request normally keeps the AN tunnel; reprogram the gNB
	// side only when the DL TEID actually changed, so the existing tunnel
	// keeps receiving the re-injected datagrams without a gap.
	dlTEID := regB.DLTEID

	if sessionB.DLTEID != regB.DLTEID {
		gNodeB.CloseTunnel(regB.DLTEID)

		if err := gNodeB.AddTunnel(&gnb.TunnelOpts{
			UEIPv4:           regB.UEIPv4 + "/16",
			UpfAddress:       sessionB.UpfAddress,
			TunInterfaceName: tunB,
			ULTEID:           sessionB.ULTEID,
			DLTEID:           sessionB.DLTEID,
			MTU:              sessionB.MTU,
			QFI:              sessionB.QFI,
		}); err != nil {
			return fmt.Errorf("recreate UE-B tunnel after service request: %w", err)
		}

		// A fresh tunnel counts from zero.
		rxBefore = 0
		dlTEID = sessionB.DLTEID
	}

	// A third datagram after the resume: it must flow directly, proving
	// the live path works alongside the drained queue.
	if err := udpSendFromTun(ctx, tunA, regB.UEIPv4, []byte("third datagram after resume")); err != nil {
		return fmt.Errorf("send third datagram from UE-A: %w", err)
	}

	// Expect the two buffered datagrams plus the live one on UE-B's
	// downlink tunnel.
	if err := awaitTunnelRX(gNodeB, dlTEID, rxBefore+3, bufferedPollDeadline); err != nil {
		return err
	}

	logger.Logger.Info("buffered downlink scenario completed: datagrams buffered while idle delivered after the page",
		zap.String("ue_a", regA.UEIPv4),
		zap.String("ue_b", regB.UEIPv4),
	)

	gNodeB.CloseTunnel(dlTEID)
	gNodeB.CloseTunnel(regA.DLTEID)

	if err := gNodeB.Deregister(ueA, ranUENGAPID_A, releaseTimeout); err != nil {
		return fmt.Errorf("UE-A deregistration: %w", err)
	}

	if err := gNodeB.Deregister(ueB, ranUENGAPID_B, releaseTimeout); err != nil {
		return fmt.Errorf("UE-B deregistration: %w", err)
	}

	return nil
}

// awaitTunnelRX polls the gNB's G-PDU receive counter for a DL TEID until it
// reaches want, or the deadline passes.
func awaitTunnelRX(gNodeB *gnb.GnodeB, dlteid uint32, want uint64, deadline time.Duration) error {
	for deadline > 0 {
		if gNodeB.TunnelRXCount(dlteid) >= want {
			return nil
		}

		sleep := bufferedPollInterval
		if deadline < sleep {
			sleep = deadline
		}

		time.Sleep(sleep)
		deadline -= sleep
	}

	if got := gNodeB.TunnelRXCount(dlteid); got < want {
		return fmt.Errorf("gNB received %d G-PDUs on DL TEID %d, want at least %d: buffered datagrams were not delivered", got, dlteid, want)
	}

	return nil
}

// udpSendFromTun sends one datagram from a UE's TUN device, so the packet
// egresses the gNB tunnel as that UE's uplink. One-way on purpose: no echo
// is expected while the receiver is idle.
func udpSendFromTun(ctx context.Context, tun, dstIP string, payload []byte) error {
	dialer := net.Dialer{
		Control: func(_, _ string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, tun)
			})
		},
	}

	conn, err := dialer.DialContext(ctx, "udp4", net.JoinHostPort(dstIP, strconv.Itoa(bufferedDstPort)))
	if err != nil {
		return err
	}

	defer func() {
		_ = conn.Close()
	}()

	_, err = conn.Write(payload)

	return err
}
