// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/registration/no_follow_on_request",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationNoFollowOnRequest(ctx, env, params)
		},
		Fixture: fixtureRegistrationNoFollowOnRequest,
	})
}

func fixtureRegistrationNoFollowOnRequest(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

// runRegistrationNoFollowOnRequest registers a UE that clears the Follow-On
// Request bit and establishes no PDU session. With nothing pending the AMF
// releases the NAS signalling connection on its own once the Registration
// Complete lands (TS 24.501 §5.5.1.2.4), which is the only path that produces a
// UE CONTEXT RELEASE COMMAND with an NGAP cause from the NAS group: every other
// release this simulator sees is one the gNB asked for.
func runRegistrationNoFollowOnRequest(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newSignallingOnlyUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	gNodeB.AddUE(ranUENGAPID, newUE)

	if err := gNodeB.RegisterWithoutSession(newUE, ranUENGAPID, registrationTimeout); err != nil {
		return fmt.Errorf("registration without a PDU session failed: %v", err)
	}

	fr, err := gNodeB.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, releaseTimeout)
	if err != nil {
		return fmt.Errorf("the AMF did not release the UE context of a registration with no follow-on request: %w", err)
	}

	if err := validateUEContextReleaseCommand(fr, gnb.CauseNASNormalRelease); err != nil {
		return fmt.Errorf("UEContextRelease validation failed: %v", err)
	}

	logger.Logger.Info("AMF released the UE context after a registration with no follow-on request",
		zap.String("cause", gnb.CauseNASNormalRelease.String()),
	)

	if err := newUE.WaitForRRCRelease(releaseTimeout); err != nil {
		return fmt.Errorf("the UE was not released to CM-IDLE: %w", err)
	}

	if ids := newUE.ActivePDUSessionIDs(); len(ids) != 0 {
		return fmt.Errorf("the UE holds PDU sessions %v, but it registered without establishing one", ids)
	}

	return nil
}
