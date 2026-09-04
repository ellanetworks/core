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
		Name:      "gnb/mobility_registration/no_ue_context_request",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runMobilityRegistrationNoUEContextRequest(ctx, env, params)
		},
		Fixture: fixtureServiceRequestData,
	})
}

func runMobilityRegistrationNoUEContextRequest(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	gNodeB.OmitUEContextRequest = true

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	if _, err := gNodeB.Register(newUE, int64(scenarios.DefaultRANUENGAPID), scenarios.DefaultPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	pduSessionStatus := []uint8{scenarios.DefaultPDUSessionID}

	if err := gNodeB.ReleaseContext(newUE, int64(scenarios.DefaultRANUENGAPID), pduSessionStatus, gnb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	if _, err := gNodeB.MobilityRegistrationUpdate(newUE, int64(scenarios.DefaultRANUENGAPID), scenarios.DefaultPDUSessionID, nil, registrationTimeout); err != nil {
		return fmt.Errorf("mobility registration update procedure failed: %v", err)
	}

	if err := gNodeB.Deregister(newUE, int64(scenarios.DefaultRANUENGAPID), releaseTimeout); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}
