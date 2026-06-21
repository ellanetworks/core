// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/service_request/data",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runServiceRequestData(ctx, env, params)
		},
		Fixture: fixtureServiceRequestData,
	})
}

func fixtureServiceRequestData(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runServiceRequestData(_ context.Context, env scenarios.Env, _ any) error {
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

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  int64(scenarios.DefaultRANUENGAPID),
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	pduSessionStatus := [16]bool{}
	pduSessionStatus[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID:   int64(scenarios.DefaultRANUENGAPID),
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionStatus,
	})
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	err = runServiceRequestOnUE(int64(scenarios.DefaultRANUENGAPID), pduSessionStatus, newUE)
	if err != nil {
		return fmt.Errorf("service request procedure failed: %v", err)
	}

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID: int64(scenarios.DefaultRANUENGAPID),
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}

func runServiceRequestOnUE(ranUENGAPID int64, pduSessionStatus [16]bool, u *ue.UE) error {
	err := u.SendServiceRequest(ranUENGAPID, pduSessionStatus, nasMessage.ServiceTypeData)
	if err != nil {
		return fmt.Errorf("could not send Service Request NAS message: %v", err)
	}

	_, err = u.WaitForNASGMMMessage(nas.MsgTypeServiceAccept, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Service Accept NAS message: %v", err)
	}

	return nil
}
