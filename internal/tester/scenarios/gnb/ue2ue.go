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
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	ue2ueStartIMSI = "001017271247001"
)

type ue2ueParams struct {
	ExpectSuccess bool `json:"expectSuccess"`
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/ue2ue",
		BindFlags: func(fs *pflag.FlagSet) any {
			var p ue2ueParams
			fs.BoolVar(&p.ExpectSuccess, "expect-success", true, "expect the UDP probe to succeed")

			return &p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runUE2UE(ctx, env, params)
		},
		Fixture: fixtureUE2UE,
	})
}

func fixtureUE2UE(env scenarios.Env) scenarios.FixtureSpec {
	imsiA := ue2ueStartIMSI
	imsiB := incrementIMSI(ue2ueStartIMSI, 1)

	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(imsiA, ""),
			scenarios.DefaultSubscriberWith(imsiB, ""),
		},
		AssertUsageForIMSIs: []string{imsiA, imsiB},
	}
}

func runUE2UE(ctx context.Context, env scenarios.Env, params any) error {
	var expectSuccess bool
	if p, ok := params.(*ue2ueParams); ok {
		expectSuccess = p.ExpectSuccess
	} else {
		expectSuccess = true
	}

	subs, err := buildSubscribers(2, ue2ueStartIMSI)
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

	ueBIP := regB.UEIPv4
	if ueBIP == "" {
		return fmt.Errorf("UE-B was not assigned an IPv4 address")
	}

	logger.Logger.Debug("sending UDP from UE-A to UE-B",
		zap.String("ueA", regA.UEIPv4),
		zap.String("ueB", ueBIP),
		zap.String("tunA", tunA),
		zap.String("tunB", tunB),
	)

	probeErr := probe.UE2UE(ctx, tunA, tunB, ueBIP, scenarios.DefaultProbePort)

	gNodeB.CloseTunnel(regA.DLTEID)
	gNodeB.CloseTunnel(regB.DLTEID)

	if err := gNodeB.Deregister(ueA, ranUENGAPID_A, releaseTimeout); err != nil {
		return fmt.Errorf("UE-A deregistration: %w", err)
	}

	if err := gNodeB.Deregister(ueB, ranUENGAPID_B, releaseTimeout); err != nil {
		return fmt.Errorf("UE-B deregistration: %w", err)
	}

	if expectSuccess && probeErr != nil {
		return fmt.Errorf("udp UE-A (%s) -> UE-B (%s): expected success but failed: %w", regA.UEIPv4, ueBIP, probeErr)
	}

	if !expectSuccess && probeErr == nil {
		return fmt.Errorf("udp UE-A (%s) -> UE-B (%s): expected failure but probe succeeded", regA.UEIPv4, ueBIP)
	}

	if expectSuccess {
		logger.Logger.Debug("UE-to-UE UDP probe successful",
			zap.String("ueA", regA.UEIPv4),
			zap.String("ueB", ueBIP),
		)
	} else {
		logger.Logger.Debug("UE-to-UE UDP probe failed as expected",
			zap.String("ueA", regA.UEIPv4),
			zap.String("ueB", ueBIP),
		)
	}

	return nil
}

type ue2ueSession struct {
	UEIPv4 string
	DLTEID uint32
}

func registerAndTunnel(g *gnb.GnodeB, sub subscriber, ranUENGAPID int64, tunName string, pduSessionType uint8) (*ue2ueSession, *ue.UE, error) {
	newUE, err := newDefaultUE(g, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, pduSessionType)
	if err != nil {
		return nil, nil, fmt.Errorf("create UE: %w", err)
	}

	g.AddUE(ranUENGAPID, newUE)

	registration, err := g.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return nil, nil, fmt.Errorf("registration: %w", err)
	}

	session := registration.Session

	ueIP := session.UEIPv4 + "/16"

	err = g.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           ueIP,
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: tunName,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create GTP tunnel %s: %w", tunName, err)
	}

	time.Sleep(500 * time.Millisecond)

	return &ue2ueSession{
		UEIPv4: session.UEIPv4,
		DLTEID: session.DLTEID,
	}, newUE, nil
}
