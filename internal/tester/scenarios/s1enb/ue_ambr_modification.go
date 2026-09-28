// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	ueAMBRModIMSI    = "001017271246687"
	ueAMBRModProfile = "s1enb-ue-ambr-profile"
	ueAMBRModPolicy  = "s1enb-ue-ambr-policy"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/ue-ambr-modification",
		BindFlags: bindSessionModFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runUEAMBRModification(ctx, env, params.(*sessionModParams))
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

func runUEAMBRModification(ctx context.Context, env scenarios.Env, p *sessionModParams) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(ueAMBRModIMSI, k, opc)

	attach, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	time.Sleep(2 * time.Second)

	if err := cl.UpdateProfile(ctx, ueAMBRModProfile, &client.UpdateProfileOptions{
		UeAmbrUplink:   "40 Mbps",
		UeAmbrDownlink: "60 Mbps",
	}); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	req, err := e.AcceptUEContextModification(attach.ENBUES1APID, 20*time.Second)
	if err != nil {
		return err
	}

	ambr := req.UEAggregateMaximumBitRate
	if ambr == nil {
		return fmt.Errorf("UE Context Modification Request without a UE-AMBR")
	}

	if uint64(ambr.DL) != 60*mbpsToBps || uint64(ambr.UL) != 40*mbpsToBps {
		return fmt.Errorf("UE-AMBR = %d/%d bit/s (DL/UL), want %d/%d", ambr.DL, ambr.UL, 60*mbpsToBps, 40*mbpsToBps)
	}

	return e.Detach(ue, attach.MMEUES1APID, attach.ENBUES1APID, releaseTimeout)
}
