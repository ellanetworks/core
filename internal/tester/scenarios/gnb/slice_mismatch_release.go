// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const sliceMismatchIMSI = "001017271246547"

type sliceMismatchParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
	SliceSST       int
	SliceSD        string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/slice-mismatch-release",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &sliceMismatchParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")
			fs.IntVar(&p.SliceSST, "slice-sst", 2, "SST for the new slice that will cause mismatch")
			fs.StringVar(&p.SliceSD, "slice-sd", "abcdef", "SD for the new slice that will cause mismatch")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			p := params.(*sliceMismatchParams)
			return runSliceMismatchRelease(ctx, env, p)
		},
		Fixture: fixtureSliceMismatchRelease,
	})
}

func fixtureSliceMismatchRelease(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: "alternate", UeAmbrUplink: scenarios.DefaultProfileUeAmbrUplink, UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink},
		},
		Slices: []scenarios.SliceSpec{
			{Name: "alternate", SST: 2, SD: "abcdef"},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "alternate",
				ProfileName:         "alternate",
				SliceName:           "alternate",
				DataNetworkName:     scenarios.DefaultDNN,
				SessionAmbrUplink:   scenarios.DefaultPolicySessionAmbrUplink,
				SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
				Var5qi:              9,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(sliceMismatchIMSI, scenarios.DefaultProfileName)},
	}
}

func runSliceMismatchRelease(ctx context.Context, env scenarios.Env, p *sliceMismatchParams) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           sliceMismatchIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	logger.Logger.Info(
		"PDU session established, proceeding to trigger slice mismatch release",
		zap.String("IMSI", sub.IMSI),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	// Switch to a profile without the default slice so the SMF detects the
	// session's Snssai matches no configured slice.
	logger.Logger.Info("Updating subscriber profile to remove default slice",
		zap.String("IMSI", sub.IMSI),
	)

	err = cl.UpdateSubscriber(ctx, sub.IMSI, &client.UpdateSubscriberOptions{
		ProfileName: "alternate",
	})
	if err != nil {
		return fmt.Errorf("failed to update subscriber profile: %v", err)
	}

	// On slice mismatch the SMF releases with cause #39 "reactivation requested".
	_, err = gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand,
		15*time.Second,
	)
	if err != nil {
		return fmt.Errorf("gNB did not receive PDUSessionResourceReleaseCommand: %v", err)
	}

	logger.Logger.Info("gNB received PDUSessionResourceReleaseCommand")

	releaseCmd, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionReleaseCommand, 15*time.Second)
	if err != nil {
		return fmt.Errorf("UE did not receive PDU Session Release Command: %v", err)
	}

	logger.Logger.Info("UE received PDU Session Release Command")

	if releaseCmd.PDUSessionReleaseCommand == nil {
		return fmt.Errorf("PDU Session Release Command message is nil")
	}

	cause := releaseCmd.PDUSessionReleaseCommand.GetCauseValue()
	if cause != nasMessage.Cause5GSMReactivationRequested {
		return fmt.Errorf("expected 5GSM cause #39 (reactivation requested), got %d", cause)
	}

	logger.Logger.Info("PDU Session Release Command validated: cause = reactivation requested",
		zap.Uint8("cause", cause),
	)

	_ = cl.UpdateSubscriber(ctx, sub.IMSI, &client.UpdateSubscriberOptions{
		ProfileName: scenarios.DefaultProfileName,
	})

	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
	if err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	logger.Logger.Info("Slice mismatch release scenario completed successfully")

	return nil
}
