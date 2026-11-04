// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	sessionTrafficHoldIMSI   = "001017271246591"
	sessionTrafficHoldTunnel = "ellatesterhold"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/session_traffic_hold",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runSessionTrafficHold(ctx, env)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(sessionTrafficHoldIMSI, ""),
				},
			}
		},
	})
}

func runSessionTrafficHold(ctx context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newDefaultUE(gNodeB, sessionTrafficHoldIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, scenarios.DefaultPDUSessionTypeIPv4)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	registration, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	session := registration.Session

	if err := gNodeB.AddTunnel(&gnb.TunnelOpts{
		UEIPv4:           session.UEIPv4 + "/16",
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: sessionTrafficHoldTunnel,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	}); err != nil {
		return fmt.Errorf("could not create GTP tunnel: %v", err)
	}

	logger.Logger.Info("session established, sending traffic until cancelled",
		zap.String("IMSI", sessionTrafficHoldIMSI),
		zap.String("UE IP", session.UEIPv4),
	)

	for ctx.Err() == nil {
		if err := probe.Run(ctx, probe.ICMP, sessionTrafficHoldTunnel, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil && ctx.Err() == nil {
			logger.Logger.Warn("ping failed", zap.Error(err))
		}
	}

	logger.Logger.Info("context cancelled, tearing down session", zap.String("IMSI", sessionTrafficHoldIMSI))

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		logger.Logger.Warn("deregistration failed during teardown", zap.Error(err))
	}

	return nil
}
