// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
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

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	logger.Logger.Info("session established, holding until cancelled",
		zap.String("IMSI", sessionHoldIMSI),
		zap.String("UE IP", uePDUSession.UEIP),
	)

	<-ctx.Done()

	logger.Logger.Info("context cancelled, tearing down session",
		zap.String("IMSI", sessionHoldIMSI),
	)

	if err := procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID: ranUENGAPID,
	}); err != nil {
		logger.Logger.Warn("deregistration failed during teardown", zap.Error(err))
	}

	return nil
}
