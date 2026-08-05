// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/context/release",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runUEContextRelease(ctx, env, params)
		},
		Fixture: fixtureUEContextRelease,
	})
}

func fixtureUEContextRelease(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func runUEContextRelease(_ context.Context, env scenarios.Env, _ any) error {
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
		return fmt.Errorf("InitialRegistrationProcedure failed: %v", err)
	}

	pduSessionStatus := [16]bool{}
	pduSessionStatus[scenarios.DefaultPDUSessionID] = true

	err = gNodeB.SendUEContextReleaseRequest(&gnb.UEContextReleaseRequestOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID:   int64(scenarios.DefaultRANUENGAPID),
		PDUSessionIDs: pduSessionStatus,
		Cause:         ngaplib.Cause{Group: ngaplib.CauseGroupRadioNetwork, Value: ngaplib.CauseRadioNetworkReleaseDueToNGRANGeneratedReason},
	})
	if err != nil {
		return fmt.Errorf("could not send UEContextReleaseComplete: %v", err)
	}

	logger.Logger.Debug(
		"Sent UE Context Release Request",
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID))),
		zap.Int64("RAN UE NGAP ID", int64(scenarios.DefaultRANUENGAPID)),
		zap.String("Cause", "ReleaseDueToNgranGeneratedReason"),
	)

	fr, err := gNodeB.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	err = validateUEContextReleaseCommand(fr, ngaplib.Cause{
		Group: ngaplib.CauseGroupRadioNetwork,
		Value: ngaplib.CauseRadioNetworkReleaseDueToNGRANGeneratedReason,
	})
	if err != nil {
		return fmt.Errorf("UEContextRelease validation failed: %v", err)
	}

	return nil
}

func validateUEContextReleaseCommand(fr gnb.SCTPFrame, want ngaplib.Cause) error {
	err := testutil.ValidateSCTP(fr.Info, 60, 1)
	if err != nil {
		return fmt.Errorf("SCTP validation failed: %v", err)
	}

	pdu, err := ngaplib.Unmarshal(fr.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	im, ok := pdu.(*ngaplib.InitiatingMessage)
	if !ok {
		return fmt.Errorf("NGAP PDU is not an InitiatingMessage")
	}

	if im.ProcedureCode != ngaplib.ProcUEContextRelease {
		return fmt.Errorf("NGAP ProcedureCode is not UEContextRelease (%d), received %d", ngaplib.ProcUEContextRelease, im.ProcedureCode)
	}

	cmd, err := ngaplib.ParseUEContextReleaseCommand(im.Value)
	if err != nil {
		return fmt.Errorf("could not parse UE Context Release Command: %v", err)
	}

	if cmd.Cause == nil {
		return fmt.Errorf("cause is absent")
	}

	if *cmd.Cause != want {
		return fmt.Errorf("unexpected Cause: got %+v, want %+v", *cmd.Cause, want)
	}

	// The AMF addresses a UE it has a RAN UE NGAP ID for by the pair
	// (TS 38.413 §9.2.2.5).
	if !cmd.UENGAPIDs.Pair {
		return fmt.Errorf("UE-NGAP-IDs carries no RAN UE NGAP ID")
	}

	return nil
}
