// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	ueAMBRModIMSI    = "001017271246591"
	ueAMBRModProfile = "gnb-ue-ambr-profile"
	ueAMBRModPolicy  = "gnb-ue-ambr-policy"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/ue-ambr-modification",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &sessionModificationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runUEAMBRModification(ctx, env, params.(*sessionModificationParams))
		},
		Fixture: fixtureUEAMBRModification,
	})
}

func fixtureUEAMBRModification(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: ueAMBRModProfile, UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name: ueAMBRModPolicy, ProfileName: ueAMBRModProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps",
				Var5qi: 9, Arp: 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(ueAMBRModIMSI, ueAMBRModProfile)},
	}
}

func runUEAMBRModification(ctx context.Context, env scenarios.Env, p *sessionModificationParams) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(gNodeB, ueAMBRModIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %w", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	time.Sleep(2 * time.Second)

	if err := cl.UpdateProfile(ctx, ueAMBRModProfile, &client.UpdateProfileOptions{
		UeAmbrUplink:   "40 Mbps",
		UeAmbrDownlink: "60 Mbps",
	}); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	req, err := gNodeB.WaitForUEContextModificationRequest(20 * time.Second)
	if err != nil {
		return fmt.Errorf("gNB did not receive a UE Context Modification Request: %w", err)
	}

	ambr := req.UEAggregateMaximumBitRate
	if ambr == nil {
		return fmt.Errorf("UE Context Modification Request without a UE-AMBR")
	}

	if uint64(ambr.DL) != 60_000_000 || uint64(ambr.UL) != 40_000_000 {
		return fmt.Errorf("UE-AMBR = %d/%d bit/s (DL/UL), want 60000000/40000000", ambr.DL, ambr.UL)
	}

	return gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout)
}
