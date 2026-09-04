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

// runBufferedDownlink asserts that downlink datagrams sent to an idle UE are
// buffered and delivered after the UE answers the page. Requires local switching.
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

	if err := gNodeB.ReleaseContext(ueB, ranUENGAPID_B, []uint8{scenarios.DefaultPDUSessionID}, gnb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release UE-B to idle: %w", err)
	}

	time.Sleep(bufferedSettle)

	if err := probe.SendUDPOneWay(ctx, tunA, regB.UEIPv4, bufferedDstPort, []byte("first datagram to an idle ue")); err != nil {
		return fmt.Errorf("send first datagram from UE-A: %w", err)
	}

	time.Sleep(bufferedSettle)

	if err := probe.SendUDPOneWay(ctx, tunA, regB.UEIPv4, bufferedDstPort, []byte("second datagram while paging")); err != nil {
		return fmt.Errorf("send second datagram from UE-A: %w", err)
	}

	time.Sleep(bufferedSettle)

	serviceRequest, err := gNodeB.ServiceRequest(ueB, ranUENGAPID_B, scenarios.DefaultPDUSessionID, registrationTimeout, &gnb.ServiceRequestOpts{DLTEID: regB.DLTEID})
	if err != nil {
		return fmt.Errorf("service request from idle: %w", err)
	}

	sessionB := serviceRequest.Session

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

		rxBefore = 0
		dlTEID = sessionB.DLTEID
	}

	if err := probe.SendUDPOneWay(ctx, tunA, regB.UEIPv4, bufferedDstPort, []byte("third datagram after resume")); err != nil {
		return fmt.Errorf("send third datagram from UE-A: %w", err)
	}

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
