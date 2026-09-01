// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/service_request/signalling",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runServiceRequestSignalling(ctx, env, params)
		},
		Fixture: fixtureServiceRequestSignalling,
	})
}

func fixtureServiceRequestSignalling(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runServiceRequestSignalling(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	_, err = gNodeB.Register(newUE, int64(scenarios.DefaultRANUENGAPID), scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	pduSessionStatus := []uint8{scenarios.DefaultPDUSessionID}

	err = gNodeB.ReleaseContext(newUE, int64(scenarios.DefaultRANUENGAPID), pduSessionStatus, gnb.CauseUserInactivity, releaseTimeout)
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	err = gNodeB.ServiceRequestSignalling(newUE, int64(scenarios.DefaultRANUENGAPID), registrationTimeout)
	if err != nil {
		return fmt.Errorf("signalling service request procedure failed: %v", err)
	}

	err = gNodeB.ReleaseContext(newUE, int64(scenarios.DefaultRANUENGAPID), pduSessionStatus, gnb.CauseUserInactivity, releaseTimeout)
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure after the signalling service request failed: %v", err)
	}

	_, err = gNodeB.ServiceRequest(newUE, int64(scenarios.DefaultRANUENGAPID), scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("the PDU session did not survive the signalling service request: %v", err)
	}

	err = gNodeB.Deregister(newUE, int64(scenarios.DefaultRANUENGAPID), releaseTimeout)
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
