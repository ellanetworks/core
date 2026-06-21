// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/spf13/pflag"
)

const (
	numSubscribersSequential = 50
	sequentialStartIMSI      = "001017271246546"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/registration_success_50_sequential",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccess50Sequential(ctx, env, params)
		},
		Fixture: fixtureRegistrationSuccess50Sequential,
	})
}

func fixtureRegistrationSuccess50Sequential(env scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numSubscribersSequential)
	for i := range numSubscribersSequential {
		subs[i] = scenarios.DefaultSubscriberWith(incrementIMSI(sequentialStartIMSI, i), "")
	}

	return scenarios.FixtureSpec{Subscribers: subs}
}

func runRegistrationSuccess50Sequential(_ context.Context, env scenarios.Env, _ any) error {
	subs, err := buildSubscribers(numSubscribersSequential, sequentialStartIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
	}

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	network, err := netip.ParsePrefix("10.45.0.0/16")
	if err != nil {
		return fmt.Errorf("failed to parse UE IP subnet: %v", err)
	}

	for i := range numSubscribersSequential {
		ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)

		exp := &validate.ExpectedPDUSessionEstablishmentAccept{
			PDUSessionID:               scenarios.DefaultPDUSessionID,
			PDUSessionType:             env.PDUSessionType(),
			UeIPSubnet:                 network,
			Dnn:                        scenarios.DefaultDNN,
			Sst:                        scenarios.DefaultSST,
			Sd:                         scenarios.DefaultSD,
			MaximumBitRateUplinkMbps:   100,
			MaximumBitRateDownlinkMbps: 100,
			Qfi:                        1,
			FiveQI:                     9,
		}

		err := ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], scenarios.DefaultDNN, exp, env.PDUSessionType())
		if err != nil {
			return fmt.Errorf("UE registration test failed for subscriber %d: %v", i, err)
		}
	}

	return nil
}
