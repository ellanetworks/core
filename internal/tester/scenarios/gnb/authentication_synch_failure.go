// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// aheadSequenceNumber puts the UE's sequence number well past the one the
// subscriber carries in the core, so the first AUTN the network sends is stale
// and the UE has to ask for a resynchronisation.
const aheadSequenceNumber = "000000000099"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/authentication/synch_failure",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runAuthenticationSynchFailure(ctx, env, params)
		},
		Fixture: fixtureAuthenticationSynchFailure,
	})
}

func fixtureAuthenticationSynchFailure(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

// runAuthenticationSynchFailure drives the sequence-number resynchronisation of
// TS 33.501 §6.1.3.2.1, the one authentication outcome the reference network
// produces that is not a plain success: the UE rejects the first AUTHENTICATION
// REQUEST with 5GMM cause #21 and an AUTS, the network resynchronises against
// it and authenticates again, and the registration completes.
func runAuthenticationSynchFailure(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, aheadSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("registration did not survive the authentication resynchronisation: %v", err)
	}

	// One request for the stale AUTN the UE refused, one for the vector the
	// network built from the AUTS.
	if got := newUE.ReceivedNASGMMCount(uint8(fgs.MsgAuthenticationRequest)); got != 2 {
		return fmt.Errorf("the UE saw %d Authentication Requests, want 2: the network did not resynchronise and authenticate again", got)
	}

	logger.Logger.Info("Authentication resynchronised after a synch failure",
		zap.Int("authentication requests", 2),
	)

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
