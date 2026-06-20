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
	"golang.org/x/sync/errgroup"
)

const (
	numSubscribersParallel = 150
	parallelStartIMSI      = "001017271246000"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/registration_success_150_parallel",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRegistrationSuccess150Parallel(ctx, env, params)
		},
		Fixture: fixtureRegistrationSuccess150Parallel,
	})
}

func fixtureRegistrationSuccess150Parallel(env scenarios.Env) scenarios.FixtureSpec {
	subs := make([]scenarios.SubscriberSpec, numSubscribersParallel)
	for i := range numSubscribersParallel {
		subs[i] = scenarios.DefaultSubscriberWith(incrementIMSI(parallelStartIMSI, i), "")
	}

	return scenarios.FixtureSpec{Subscribers: subs}
}

func runRegistrationSuccess150Parallel(_ context.Context, env scenarios.Env, _ any) error {
	subs, err := buildSubscribers(numSubscribersParallel, parallelStartIMSI)
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

	eg := errgroup.Group{}

	for i := range numSubscribersParallel {
		func() {
			eg.Go(func() error {
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

				return ueRegistrationTest(ranUENGAPID, gNodeB, subs[i], scenarios.DefaultDNN, exp, env.PDUSessionType())
			})
		}()
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("error during UE registrations: %v", err)
	}

	return nil
}
