// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	errorIndicationStartIMSI      = "001017271247301"
	errorIndicationDstPort        = 59998
	errorIndicationReleaseTimeout = 10 * time.Second
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/error_indication",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runErrorIndication(ctx, env)
		},
		Fixture: fixtureErrorIndication,
	})
}

func fixtureErrorIndication(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(errorIndicationStartIMSI, ""),
			scenarios.DefaultSubscriberWith(incrementIMSI(errorIndicationStartIMSI, 1), ""),
		},
	}
}

func runErrorIndication(ctx context.Context, env scenarios.Env) error {
	subs, err := buildSubscribers(2, errorIndicationStartIMSI)
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

	_, _, err = registerAndTunnel(gNodeB, subs[0], ranUENGAPID_A, tunA, pduSessionType)
	if err != nil {
		return fmt.Errorf("UE-A registration: %w", err)
	}

	regB, ueB, err := registerAndTunnel(gNodeB, subs[1], ranUENGAPID_B, tunB, pduSessionType)
	if err != nil {
		return fmt.Errorf("UE-B registration: %w", err)
	}

	upf, err := netip.ParseAddr(regB.UpfAddress)
	if err != nil {
		return fmt.Errorf("parse the UPF N3 address %q: %w", regB.UpfAddress, err)
	}

	if err := scenarios.AwaitDownlinkDelivery(
		func() error {
			return probe.SendUDPOneWay(ctx, tunA, regB.UEIPv4, errorIndicationDstPort, []byte("datagram before the error indication"))
		},
		func() uint64 { return gNodeB.TunnelRXCount(regB.DLTEID) },
		"UE-to-UE downlink before the Error Indication",
	); err != nil {
		return err
	}

	if err := gNodeB.SendGTPUErrorIndication(regB.DLTEID, upf, gNodeB.N3Address); err != nil {
		return fmt.Errorf("send a GTP-U Error Indication for TEID %#x: %w", regB.DLTEID, err)
	}

	if err := gNodeB.AwaitPDUSessionRelease(ranUENGAPID_B, scenarios.DefaultPDUSessionID, errorIndicationReleaseTimeout); err != nil {
		return fmt.Errorf("the core did not release the access resources of the broken tunnel: %w", err)
	}

	rxBefore := gNodeB.TunnelRXCount(regB.DLTEID)

	serviceRequest, err := gNodeB.ServiceRequest(ueB, ranUENGAPID_B, scenarios.DefaultPDUSessionID, registrationTimeout, &gnb.ServiceRequestOpts{DLTEID: regB.DLTEID})
	if err != nil {
		return fmt.Errorf("service request after the error indication: %w", err)
	}

	session := serviceRequest.Session
	dlTEID := regB.DLTEID

	if session.DLTEID != regB.DLTEID {
		gNodeB.CloseTunnel(regB.DLTEID)

		if err := gNodeB.AddTunnel(&gnb.TunnelOpts{
			UEIPv4:           regB.UEIPv4 + "/16",
			UpfAddress:       session.UpfAddress,
			TunInterfaceName: tunB,
			ULTEID:           session.ULTEID,
			DLTEID:           session.DLTEID,
			MTU:              session.MTU,
			QFI:              session.QFI,
		}); err != nil {
			return fmt.Errorf("recreate the UE-B tunnel after the service request: %w", err)
		}

		rxBefore = 0
		dlTEID = session.DLTEID
	}

	if err := probe.SendUDPOneWay(ctx, tunA, regB.UEIPv4, errorIndicationDstPort, []byte("datagram after the tunnel was re-established")); err != nil {
		return fmt.Errorf("send the second datagram from UE-A: %w", err)
	}

	if err := awaitTunnelRX(gNodeB, dlTEID, rxBefore+1, bufferedPollDeadline); err != nil {
		return err
	}

	logger.Logger.Info("error indication scenario completed: the core released the dead tunnel and re-established it",
		zap.Uint32("dl_teid", dlTEID))

	return nil
}
