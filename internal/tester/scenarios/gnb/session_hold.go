// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const sessionHoldIMSI = "001017271246590"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/session_hold",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runSessionHold(ctx, env)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(sessionHoldIMSI, ""),
				},
			}
		},
	})
}

// runSessionHold holds the PDU session and its IP lease open until ctx is
// cancelled so external tests can observe the BGP route advertisement before tear-down.
func runSessionHold(ctx context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newDefaultUE(gNodeB, sessionHoldIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
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

	logger.Logger.Info("session established, holding until cancelled",
		zap.String("IMSI", sessionHoldIMSI),
		zap.String("UE IP", session.UEIPv4),
	)

	<-ctx.Done()

	logger.Logger.Info("context cancelled, tearing down session",
		zap.String("IMSI", sessionHoldIMSI),
	)

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		logger.Logger.Warn("deregistration failed during teardown", zap.Error(err))
	}

	return nil
}
